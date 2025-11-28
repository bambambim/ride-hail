package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"ride-hail/internal/driver_location_service/adapter/db"
	internalRabbit "ride-hail/internal/driver_location_service/adapter/rabbitmq"
	"ride-hail/internal/driver_location_service/adapter/rest"
	wsadapter "ride-hail/internal/driver_location_service/adapter/websocket"
	"ride-hail/internal/driver_location_service/app"
	"ride-hail/pkg/auth"
	"ride-hail/pkg/config"
	"ride-hail/pkg/logger"
	pkgRabbit "ride-hail/pkg/rabbitmq"
)

func main() {
	log := logger.NewLogger("driver-location-service")
	log.Info("service_start", "Driver location service is starting")

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	cfg, err := config.LoadConfig(".env")
	if err != nil {
		log.Error("config_load_failed", err)
		os.Exit(1)
	}

	repo, err := db.NewPostgresDriverLocationRepository(log, cfg)
	if err != nil {
		log.Error("repository_init_failed", err)
		os.Exit(1)
	}
	defer repo.Close()

	rabbitConn, err := pkgRabbit.NewConnection(cfg, log)
	if err != nil {
		log.Error("rabbitmq_init_failed", err)
		os.Exit(1)
	}
	defer rabbitConn.Close()

	publisher := internalRabbit.NewDriverLocationPublisher(rabbitConn)

	sKey := os.Getenv("JWT_SECRET_KEY")
	if sKey == "" {
		log.Error("startup", fmt.Errorf("JWT_SECRET_KEY environment variable not set"))
		sKey = "someone"
	}
	jwtMgr := auth.NewJWTManager(sKey, 1*time.Hour)

	wsMgr := wsadapter.NewManager(jwtMgr, log)
	defer wsMgr.CloseAll()

	service := app.NewDriverLocationService(log, repo, publisher, wsMgr)

	consumer := internalRabbit.NewDriverLocationConsumer(rabbitConn, service, log)
	if err := consumer.ConsumeDriverMatching(ctx); err != nil {
		log.Error("consumer_driver_matching_failed", err)
		os.Exit(1)
	}
	if err := consumer.ConsumeRideStatus(ctx); err != nil {
		log.Error("consumer_ride_status_failed", err)
		os.Exit(1)
	}

	handler := rest.NewHandler(service, jwtMgr, log)

	// register function will mount REST routes and websocket route
	register := func(mux *http.ServeMux) {
		handler.RegisterRoutes(mux)

		// WebSocket route for drivers: /ws/drivers/{driverID}
		mux.HandleFunc("/ws/drivers/", func(w http.ResponseWriter, r *http.Request) {
			// extract driverID from path
			// path expected: /ws/drivers/{driverID}
			driverID := strings.TrimPrefix(r.URL.Path, "/ws/drivers/")
			if driverID == "" {
				http.Error(w, "driver id required", http.StatusBadRequest)
				return
			}

			if err := wsMgr.HandleWebSocket(w, r, driverID); err != nil {
				log.Error("websocket_handle_failed", err)
				// If upgrade failed, the manager already logged; return 400
				http.Error(w, "failed to upgrade websocket", http.StatusBadRequest)
				return
			}
		})
	}

	server := rest.New(
		fmt.Sprintf(":%d", cfg.Services.DriverLocationService),
		log,
		register,
	)

	serverErr := make(chan error, 1)
	go func() {
		if err := server.Start(ctx); err != nil {
			serverErr <- err
		}
	}()

	select {
	case <-ctx.Done():
	case err := <-serverErr:
		if err != nil {
			log.Error("http_server_failed", err)
			cancel()
		}
	}

	log.Info("service_shutdown", "Driver location service stopped")
}
