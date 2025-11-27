package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ride-hail/pkg/auth"
	"ride-hail/pkg/config"
	"ride-hail/pkg/logger"
	pkgRabbit "ride-hail/pkg/rabbitmq"
	"ride-hail/services/driver_location_service/internal/adapter/db"
	internalRabbit "ride-hail/services/driver_location_service/internal/adapter/rabbitmq"
	"ride-hail/services/driver_location_service/internal/adapter/rest"
	wsadapter "ride-hail/services/driver_location_service/internal/adapter/websocket"
	"ride-hail/services/driver_location_service/internal/app"
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

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "driver_location_service_secret_a"
	}
	jwtMgr := auth.NewJWTManager(jwtSecret, 24*time.Hour)

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
	server := rest.New(
		fmt.Sprintf(":%d", cfg.Services.DriverLocationService),
		log,
		handler.RegisterRoutes,
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

