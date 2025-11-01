package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ride-hail/internal/ride-service/handler"
	"ride-hail/pkg/auth"
	"ride-hail/pkg/config"
	"ride-hail/pkg/db"
	"ride-hail/pkg/logger"
	"ride-hail/pkg/rabbitmq"
)

func main() {
	// Load config
	cfg, err := config.LoadConfig(".env")
	if err != nil {
		panic(fmt.Sprintf("Failed to load config: %v", err))
	}

	// Initialize logger
	log := logger.NewLogger("ride-service")
	log.Info("service_starting", "Ride Service starting on port 3000")

	// Connect to database
	dbConn, err := db.NewConnection(cfg, log)
	if err != nil {
		log.Error("db_connect_failed", err)
		os.Exit(1)
	}
	defer dbConn.Close()

	// Connect to RabbitMQ
	rabbit, err := rabbitmq.NewConnection(cfg, log)
	if err != nil {
		log.Error("rabbitmq_connect_failed", err)
		os.Exit(1)
	}
	defer rabbit.Close()

	// Initialize JWT manager
	jwtManager := auth.NewJWTManager("someone", 1*time.Hour)

	// Initialize handler
	h := handler.New(dbConn, rabbit, log)

	// Setup routes
	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.Health)

	// Public endpoints - User Management
	mux.HandleFunc("POST /users", h.CreateUser)             // Register new user
	mux.HandleFunc("GET /users", h.ListUsers)               // List all users
	mux.HandleFunc("GET /users/{user_id}", h.GetUser)       // Get user by ID
	mux.HandleFunc("DELETE /users/{user_id}", h.DeleteUser) // Delete user

	// Public endpoint for testing - generates tokens (remove in production!)
	mux.HandleFunc("POST /auth/token", func(w http.ResponseWriter, r *http.Request) {
		h.GenerateTestToken(w, r, jwtManager)
	})

	// Protected endpoints - require JWT authentication
	mux.Handle("POST /rides", jwtManager.AuthMiddleware(http.HandlerFunc(h.CreateRide)))
	mux.Handle("POST /rides/{ride_id}/cancel", jwtManager.AuthMiddleware(http.HandlerFunc(h.CancelRide)))

	// Start server
	srv := &http.Server{
		Addr:    ":3000",
		Handler: mux,
	}

	// Graceful shutdown
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server_failed", err)
			os.Exit(1)
		}
	}()

	log.Info("server_running", "Ride Service running on :3000")

	// Wait for interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("server_shutdown", "Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	log.Info("server_stopped", "Server stopped gracefully")
}
