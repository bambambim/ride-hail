package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ride-hail/pkg/config"
	"ride-hail/pkg/logger"
	httpAdapter "ride-hail/services/driver_location_service/internal/adapters/http"
	"ride-hail/services/driver_location_service/internal/adapters/messaging"
	"ride-hail/services/driver_location_service/internal/adapters/ratelimit"
	"ride-hail/services/driver_location_service/internal/adapters/repository"
	"ride-hail/services/driver_location_service/internal/adapters/websocket"
	"ride-hail/services/driver_location_service/internal/ports"
	"ride-hail/services/driver_location_service/internal/service"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	// Create logger
	log := logger.NewLogger("driver-location-service")
	log.Info("startup", "Driver Location Service starting...")

	// Load configuration from .env file
	cfg, err := config.LoadConfig("../../../.env")
	if err != nil {
		log.Error("startup.load_config", err)
		// If .env file not found, continue with environment variables
		log.Info("startup.config", "Continuing with environment variables")
		cfg = &config.Config{}
		// Set defaults if env file not found
		cfg.DB.Host = os.Getenv("DB_HOST")
		if cfg.DB.Host == "" {
			cfg.DB.Host = "localhost"
		}
	}

	log.Info("startup.config", "Configuration loaded successfully")

	// Build database URL from config
	databaseURL := buildDatabaseURL(cfg)

	// Initialize database connection
	dbPool, err := initDatabase(databaseURL, log)
	if err != nil {
		log.Error("startup.init_database", err)
		os.Exit(1)
	}
	defer dbPool.Close()

	// Initialize repository
	repo := repository.NewPostgresRepository(dbPool)
	log.Info("startup.repository", "PostgreSQL repository initialized")

	// Build RabbitMQ URL from config
	rabbitmqURL := buildRabbitMQURL(cfg)

	// Initialize RabbitMQ message broker
	messageBroker, err := initMessageBroker(rabbitmqURL, log)
	if err != nil {
		log.Error("startup.init_message_broker", err)
		os.Exit(1)
	}
	defer messageBroker.Close()

	// Initialize WebSocket hub
	wsHub := websocket.NewHub(log)
	log.Info("startup.websocket", "WebSocket hub initialized")

	// Initialize rate limiter (max 1 location update per 3 seconds)
	rateLimiter := ratelimit.NewMemoryRateLimiter(3*time.Second, 1)
	log.Info("startup.rate_limiter", "Rate limiter initialized")

	// Initialize driver service
	driverService := service.NewDriverService(
		repo,
		messageBroker,
		wsHub,
		rateLimiter,
		log,
	)
	log.Info("startup.service", "Driver service initialized")

	// Start consuming messages
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start consuming ride requests
	if err := messageBroker.ConsumeRideRequests(ctx, driverService.HandleRideRequest); err != nil {
		log.Error("startup.consume_ride_requests", err)
		os.Exit(1)
	}

	// Start consuming ride status updates
	if err := messageBroker.ConsumeRideStatusUpdates(ctx, driverService.HandleRideStatusUpdate); err != nil {
		log.Error("startup.consume_ride_status", err)
		os.Exit(1)
	}

	log.Info("startup.messaging", "Message consumers started")

	// Initialize HTTP handler
	handler := httpAdapter.NewHandler(httpAdapter.HandlerConfig{
		Service: driverService,
		Logger:  log,
	})

	// Get server port from config
	serverPort := cfg.Services.DriverLocationService
	if serverPort == 0 {
		serverPort = 8082 // Default port
	}

	// Initialize HTTP server
	serverConfig := httpAdapter.ServerConfig{
		Host:            "0.0.0.0",
		Port:            serverPort,
		ShutdownTimeout: 30 * time.Second,
		Logger:          log,
		Handler:         handler,
	}

	server := httpAdapter.NewServer(serverConfig)

	log.Info("startup.complete", fmt.Sprintf("Service ready on %s:%d", serverConfig.Host, serverPort))

	// Start server in goroutine
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Start()
	}()

	// Wait for interrupt signal or server error
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	select {
	case err := <-serverErrors:
		log.Error("server.error", err)
		os.Exit(1)

	case sig := <-shutdown:
		log.Info("shutdown", fmt.Sprintf("Received signal %v, starting graceful shutdown", sig))

		// Cancel context to stop message consumers
		cancel()

		// Shutdown HTTP server
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Error("shutdown.server", err)
			os.Exit(1)
		}

		log.Info("shutdown.complete", "Service stopped gracefully")
	}
}

// buildDatabaseURL constructs the database URL from config
func buildDatabaseURL(cfg *config.Config) string {
	// Check for DATABASE_URL environment variable first (for Docker/cloud deployments)
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		return dbURL
	}

	// Build from config struct
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.DB.User,
		cfg.DB.Password,
		cfg.DB.Host,
		cfg.DB.Port,
		cfg.DB.Database,
	)
}

// buildRabbitMQURL constructs the RabbitMQ URL from config
func buildRabbitMQURL(cfg *config.Config) string {
	// Check for RABBITMQ_URL environment variable first (for Docker/cloud deployments)
	if rmqURL := os.Getenv("RABBITMQ_URL"); rmqURL != "" {
		return rmqURL
	}

	// Build from config struct
	return fmt.Sprintf("amqp://%s:%s@%s:%d/",
		cfg.RabbitMQ.User,
		cfg.RabbitMQ.Password,
		cfg.RabbitMQ.Host,
		cfg.RabbitMQ.Port,
	)
}

// initDatabase initializes the database connection pool
func initDatabase(databaseURL string, log logger.Logger) (*pgxpool.Pool, error) {
	config, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse database URL: %w", err)
	}

	// Set connection pool settings
	config.MaxConns = 25
	config.MinConns = 5
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute
	config.HealthCheckPeriod = time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info("database.connected", "Successfully connected to database")
	return pool, nil
}

// initMessageBroker initializes the RabbitMQ message broker
func initMessageBroker(rabbitmqURL string, log logger.Logger) (ports.MessageBroker, error) {
	brokerConfig := messaging.RabbitMQConfig{
		URL:    rabbitmqURL,
		Logger: log,
		Exchanges: messaging.ExchangeConfig{
			RideTopic:      "ride_topic",
			DriverTopic:    "driver_topic",
			LocationFanout: "location_fanout",
		},
		Queues: messaging.QueueConfig{
			DriverMatching:   "driver_matching",
			RideStatusUpdate: "ride_status_update",
		},
		MaxRetries: 3,
		RetryDelay: 5 * time.Second,
	}

	broker, err := messaging.NewRabbitMQBroker(brokerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create message broker: %w", err)
	}

	log.Info("messaging.connected", "Successfully connected to RabbitMQ")
	return broker, nil
}
