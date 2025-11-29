package adminservice

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ride-hail/pkg/auth"
	"ride-hail/pkg/config"
	"ride-hail/pkg/db"
	"ride-hail/pkg/logger"
)

func AdminService() {
	log := logger.NewLogger("admin-service")
	log.Info("startup", "Starting admin service")

	cfg, err := config.LoadConfig(".env")
	if err != nil {
		log.Error("startup", fmt.Errorf("Failed to load config: %w", err))
		os.Exit(1)
	}
	log.Info("config_loaded", "Configuration loaded successfully: "+cfg.TestVariable)

	pool, err := db.NewConnection(cfg, log)
	if err != nil {
		log.Error("startup", fmt.Errorf("Failed to connect to database: %w", err))
		os.Exit(1)
	}
	defer pool.Close()

	sKey := os.Getenv("JWT_SECRET_KEY")
	if sKey == "" {
		log.Error("startup", fmt.Errorf("JWT_SECRET_KEY environment variable not set"))
		sKey = "someone"
	}
	jwtManager := auth.NewJWTManager(sKey, 1*time.Hour)

	mux := http.NewServeMux()
	adminHandler := NewAdminHandler(log, pool)

	overviewHandler := jwtManager.AuthMiddleware(adminOnly(log, http.HandlerFunc(adminHandler.getOverviewMetrics)))
	activeRidesHandler := jwtManager.AuthMiddleware(adminOnly(log, http.HandlerFunc(adminHandler.getActiveRides)))

	mux.Handle("GET /admin/overview", overviewHandler)
	mux.Handle("GET /admin/rides/active", activeRidesHandler)

	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.Services.AdminService),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	serverErrors := make(chan error, 1)

	go func() {
		log.Info("startup", fmt.Sprintf("admin service listening on port %d", cfg.Services.AdminService))
		serverErrors <- server.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Error("shutdown", fmt.Errorf("server error: %w", err))
		}
	case <-stop:
		log.Info("shutdown", "Shutdown signal received. Starting graceful shutdown...")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Error("shutdown", fmt.Errorf("failed to gracefully shutdown: %w", err))
	}

	log.Info("shutdown", "Admin service shutdown complete")
}
