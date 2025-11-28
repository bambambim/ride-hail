package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	// Clean Architecture imports
	"ride-hail/internal/ride-service/application"
	"ride-hail/internal/ride-service/domain"
	"ride-hail/internal/ride-service/infrastructure/messaging"
	"ride-hail/internal/ride-service/infrastructure/repository"
	ridehttp "ride-hail/internal/ride-service/interface/http"

	// Legacy imports (still needed for consumers, users, websocket)
	"ride-hail/internal/ride-service/handler"
	"ride-hail/internal/ride-service/infrastructure/consumer"
	"ride-hail/pkg/auth"
	"ride-hail/pkg/config"
	"ride-hail/pkg/db"
	"ride-hail/pkg/logger"
	"ride-hail/pkg/rabbitmq"
	"ride-hail/pkg/websocket"
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
	sKey := os.Getenv("JWT_SECRET_KEY")

	if sKey == "" {
		log.Error("startup", fmt.Errorf("JWT_SECRET_KEY environment variable not set"))
		sKey = "someone"
	}
	jwtManager := auth.NewJWTManager(sKey, 1*time.Hour)
	// Initialize WebSocket manager
	wsManager := websocket.NewManager(log)

	// Initialize old handler (still needed for users, websocket, and token generation)
	h := handler.New(dbConn, rabbit, log)

	// ========================================
	// 🆕 Clean Architecture Setup
	// ========================================

	// 1. Create Infrastructure (Adapters)
	rideRepo := repository.NewPostgresRideRepository(dbConn)
	eventPublisher := messaging.NewRabbitMQEventPublisher(rabbit, log)

	// 2. Create Domain Services
	fareCalculator := domain.NewFareCalculator()

	// 3. Create Application Use Cases
	createRideUseCase := application.NewCreateRideUseCase(
		rideRepo,
		eventPublisher,
		fareCalculator,
		log,
	)
	cancelRideUseCase := application.NewCancelRideUseCase(
		rideRepo,
		eventPublisher,
		log,
	)

	// 4. Create HTTP Handlers (Clean Architecture)
	rideHandler := ridehttp.NewRideHandler(
		createRideUseCase,
		cancelRideUseCase,
		log,
	)

	log.Info("clean_architecture_initialized", "Clean Architecture components initialized")

	// ========================================
	// End Clean Architecture Setup
	// ========================================

	// Initialize and start message consumers (using new repository)
	messageConsumer := consumer.New(rabbit, log, wsManager, rideRepo)
	ctx := context.Background()
	if err := messageConsumer.StartConsuming(ctx); err != nil {
		log.Error("consumer_start_failed", err)
		os.Exit(1)
	}

	// Setup routes
	mux := http.NewServeMux()

	// CORS middleware function
	corsHandler := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Allow all origins for development (restrict in production)
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

			// Handle preflight requests
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}

	mux.Handle("/health", corsHandler(http.HandlerFunc(h.Health)))

	// Public endpoints - User Management
	// mux.Handle("POST /users", corsHandler(http.HandlerFunc(h.CreateUser)))             // Register new user
	// mux.Handle("GET /users", corsHandler(http.HandlerFunc(h.ListUsers)))               // List all users
	// mux.Handle("GET /users/{user_id}", corsHandler(http.HandlerFunc(h.GetUser)))       // Get user by ID
	// mux.Handle("DELETE /users/{user_id}", corsHandler(http.HandlerFunc(h.DeleteUser))) // Delete user

	// Public endpoint for testing - generates tokens (remove in production!)
	mux.Handle("POST /auth/token", corsHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.GenerateTestToken(w, r, jwtManager)
	})))

	// Protected endpoints - require JWT authentication
	// Using Clean Architecture handlers for rides
	mux.Handle("POST /rides", corsHandler(jwtManager.AuthMiddleware(http.HandlerFunc(rideHandler.CreateRide))))
	mux.Handle("POST /rides/{ride_id}/cancel", corsHandler(jwtManager.AuthMiddleware(http.HandlerFunc(rideHandler.CancelRide))))

	// WebSocket endpoint for passengers with passenger_id in path
	mux.HandleFunc("GET /ws/passengers/{passenger_id}", func(w http.ResponseWriter, r *http.Request) {
		passengerID := r.PathValue("passenger_id")

		if passengerID == "" {
			log.Error("websocket_missing_passenger_id", fmt.Errorf("passenger_id is required"))
			http.Error(w, "passenger_id is required", http.StatusBadRequest)
			return
		}

		// Create WebSocket handler with passenger_id validation
		passengerWsHandler := websocket.NewHandler(
			log,
			jwtManager,
			func(conn *websocket.Connection) {
				// Verify that JWT user_id matches the URL passenger_id
				if conn.Claims.UserID != passengerID {
					log.WithFields(logger.LogFields{
						"url_passenger_id": passengerID,
						"jwt_user_id":      conn.Claims.UserID,
					}).Error("websocket_passenger_id_mismatch", fmt.Errorf("passenger_id mismatch"))
					conn.Close()
					return
				}

				wsManager.AddConnection(passengerID, conn)

				log.WithFields(logger.LogFields{
					"passenger_id": passengerID,
				}).Info("websocket_passenger_connected", "Passenger WebSocket connected")

				// Start reading messages (optional - passengers mostly receive)
				conn.ReadPump(
					func(msgType int, p []byte) {
						log.WithFields(logger.LogFields{
							"passenger_id": passengerID,
							"message":      string(p),
						}).Debug("passenger_ws_message", "Message from passenger")
					},
					func() {
						wsManager.RemoveConnection(passengerID)
						log.WithFields(logger.LogFields{
							"passenger_id": passengerID,
						}).Info("websocket_passenger_disconnected", "Passenger WebSocket disconnected")
					},
				)
			},
			auth.RolePassenger,
		)

		passengerWsHandler.ServeHTTP(w, r)
	})

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
