package http

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Server represents the HTTP server for the driver location service
type Server struct {
	httpServer *http.Server
	router     *Router
	logger     *log.Logger
	config     ServerConfig
}

// ServerConfig holds configuration for the HTTP server
type ServerConfig struct {
	Host                   string
	Port                   int
	ShutdownTimeout        time.Duration
	Logger                 *log.Logger
	Handler                *Handler
}

// DefaultServerConfig returns a server config with sensible defaults
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Host:                   "0.0.0.0",
		Port:                   8080,
		ShutdownTimeout:        30 * time.Second,
		Logger:                 log.Default(),
	}
}

// NewServer creates a new HTTP server with the given configuration
func NewServer(config ServerConfig) *Server {
	// Apply defaults for zero values
	if config.Host == "" {
		config.Host = "0.0.0.0"
	}
	if config.Port == 0 {
		config.Port = 8080
	}
	if config.ShutdownTimeout == 0 {
		config.ShutdownTimeout = 30 * time.Second
	}
	if config.Logger == nil {
		config.Logger = log.Default()
	}

	// Create router with handler
	router := NewRouter(RouterConfig{
		Handler: config.Handler,
		Logger:  config.Logger,
	})

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      router,
		ErrorLog:     config.Logger,
	}

	return &Server{
		httpServer: httpServer,
		router:     router,
		logger:     config.Logger,
		config:     config,
	}
}

// Start starts the HTTP server
func (s *Server) Start() error {
	s.logger.Printf("Starting HTTP server on %s", s.httpServer.Addr)

	return s.startWithGracefulShutdown()
}

// startWithGracefulShutdown starts the server with graceful shutdown support
func (s *Server) startWithGracefulShutdown() error {
	// Channel to listen for errors coming from the listener
	serverErrors := make(chan error, 1)

	// Start the server in a goroutine
	go func() {
		s.logger.Printf("Server listening on %s", s.httpServer.Addr)
		serverErrors <- s.httpServer.ListenAndServe()
	}()

	// Channel to listen for interrupt signals
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM, syscall.SIGINT)

	// Block until we receive a signal or an error
	select {
	case err := <-serverErrors:
		return fmt.Errorf("server error: %w", err)

	case sig := <-shutdown:
		s.logger.Printf("Received signal %v, starting graceful shutdown", sig)

		// Create context with timeout for shutdown
		ctx, cancel := context.WithTimeout(context.Background(), s.config.ShutdownTimeout)
		defer cancel()

		// Attempt graceful shutdown
		if err := s.httpServer.Shutdown(ctx); err != nil {
			// Force close if graceful shutdown fails
			s.logger.Printf("Error during graceful shutdown, forcing close: %v", err)
			if closeErr := s.httpServer.Close(); closeErr != nil {
				return fmt.Errorf("could not force close server: %w", closeErr)
			}
			return fmt.Errorf("could not gracefully shutdown server: %w", err)
		}

		s.logger.Println("Server stopped gracefully")
	}

	return nil
}

// Shutdown gracefully shuts down the server
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Println("Shutting down server...")
	return s.httpServer.Shutdown(ctx)
}

// Close immediately closes the server
func (s *Server) Close() error {
	s.logger.Println("Closing server...")
	return s.httpServer.Close()
}

// GetAddr returns the server address
func (s *Server) GetAddr() string {
	return s.httpServer.Addr
}

// GetRouter returns the server's router (useful for testing)
func (s *Server) GetRouter() *Router {
	return s.router
}
