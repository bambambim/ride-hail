package rest

import (
	"context"
	"errors"
	"net/http"
	"ride-hail/pkg/logger"
	"time"
)

// Server is a simple HTTP server for driver locations.
type Server struct {
	srv *http.Server
	log logger.Logger
}

// New creates a new Server listening on addr (e.g. ":8080").
func New(addr string, register func(mux *http.ServeMux)) *Server {
	mux := http.NewServeMux()

	// call the handler's registration function
	if register != nil {
		register(mux)
	}

	// still register your health endpoint or internal endpoints
	mux.HandleFunc("/health", healthHandler)

	s := &Server{
		srv: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}

	return s
}

// Start runs the server and returns when ctx is cancelled or server fails.
// It will attempt a graceful shutdown with a 5s timeout when ctx is done.
func (s *Server) Start(ctx context.Context) error {
	errCh := make(chan error, 1)

	go func() {
		s.log.Info("http_server_start", "Starting HTTP server on address: " + s.srv.Addr)
		if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		// graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.log.Info("http_server_shutdown", "Shutting down HTTP server")
		return s.srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}
