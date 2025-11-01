package http

import (
	"net/http"
	"strings"
	"time"

	"ride-hail/pkg/logger"
)

// Router handles HTTP routing for the driver location service
type Router struct {
	mux     *http.ServeMux
	handler *Handler
	logger  logger.Logger
}

// RouterConfig holds configuration for the router
type RouterConfig struct {
	Handler *Handler
	Logger  logger.Logger
}

// NewRouter creates a new HTTP router with configured routes
func NewRouter(config RouterConfig) *Router {
	r := &Router{
		mux:     http.NewServeMux(),
		handler: config.Handler,
		logger:  config.Logger,
	}

	r.registerRoutes()
	return r
}

// registerRoutes sets up all the routes for the service
func (r *Router) registerRoutes() {
	// Health check endpoint
	r.mux.HandleFunc("/health", r.handler.HealthCheck)

	// Driver lifecycle endpoints
	r.mux.HandleFunc("/drivers/", r.handleDriverRoutes)

	// Internal endpoints for service-to-service communication
	r.mux.HandleFunc("/internal/drivers/nearby", r.handler.GetNearbyDrivers)
}

// handleDriverRoutes routes driver-specific endpoints
func (r *Router) handleDriverRoutes(w http.ResponseWriter, req *http.Request) {
	// Parse the path to determine the action
	path := strings.TrimPrefix(req.URL.Path, "/drivers/")
	segments := strings.Split(path, "/")

	if len(segments) < 2 {
		http.NotFound(w, req)
		return
	}

	// driverID := segments[0]
	action := segments[1]

	switch action {
	case "online":
		if req.Method != http.MethodPost {
			r.methodNotAllowed(w)
			return
		}
		r.handler.GoOnline(w, req)

	case "offline":
		if req.Method != http.MethodPost {
			r.methodNotAllowed(w)
			return
		}
		r.handler.GoOffline(w, req)

	case "location":
		if req.Method != http.MethodPost {
			r.methodNotAllowed(w)
			return
		}
		r.handler.UpdateLocation(w, req)

	case "start":
		if req.Method != http.MethodPost {
			r.methodNotAllowed(w)
			return
		}
		r.handler.StartRide(w, req)

	case "complete":
		if req.Method != http.MethodPost {
			r.methodNotAllowed(w)
			return
		}
		r.handler.CompleteRide(w, req)

	default:
		// If no action specified, handle GET request for driver info
		if req.Method == http.MethodGet && len(segments) == 1 {
			r.handler.GetDriver(w, req)
			return
		}
		http.NotFound(w, req)
	}
}

// ServeHTTP implements the http.Handler interface
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	// Apply middleware chain
	handler := r.loggingMiddleware(
		r.corsMiddleware(
			r.authMiddleware(
				r.recoveryMiddleware(r.mux),
			),
		),
	)
	handler.ServeHTTP(w, req)
}

// loggingMiddleware logs all incoming requests
func (r *Router) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		start := time.Now()

		// Wrap ResponseWriter to capture status code
		wrappedWriter := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrappedWriter, req)

		duration := time.Since(start)

		logFields := logger.LogFields{
			"method":      req.Method,
			"path":        req.URL.Path,
			"status":      wrappedWriter.statusCode,
			"duration_ms": duration.Milliseconds(),
			"remote_addr": req.RemoteAddr,
		}

		r.logger.WithFields(logFields).Info("http.request",
			formatLogMessage(req.Method, req.URL.Path, wrappedWriter.statusCode, duration))
	})
}

// corsMiddleware adds CORS headers to responses
func (r *Router) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight requests
		if req.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, req)
	})
}

// authMiddleware validates JWT tokens for protected endpoints
func (r *Router) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		// Skip auth for health check and OPTIONS requests
		if req.URL.Path == "/health" || req.Method == http.MethodOptions {
			next.ServeHTTP(w, req)
			return
		}

		// Check if endpoint requires authentication
		if r.requiresAuth(req.URL.Path) {
			authHeader := req.Header.Get("Authorization")
			if authHeader == "" {
				r.unauthorizedResponse(w, "Missing authorization header")
				return
			}

			// Validate Bearer token format
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				r.unauthorizedResponse(w, "Invalid authorization format")
				return
			}

			// In production, validate JWT token here
			// token := parts[1]
			// claims, err := validateJWT(token)
			// if err != nil {
			//     r.unauthorizedResponse(w, "Invalid token")
			//     return
			// }
			//
			// // Add claims to context
			// ctx := context.WithValue(req.Context(), "claims", claims)
			// req = req.WithContext(ctx)
		}

		next.ServeHTTP(w, req)
	})
}

// recoveryMiddleware recovers from panics and logs them
func (r *Router) recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				r.logger.Error("http.panic",
					&panicError{message: formatPanicMessage(err)})

				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(`{"error":"Internal server error"}`))
			}
		}()

		next.ServeHTTP(w, req)
	})
}

// requiresAuth checks if a path requires authentication
func (r *Router) requiresAuth(path string) bool {
	// All driver endpoints require authentication except health check
	publicPaths := []string{
		"/health",
	}

	for _, publicPath := range publicPaths {
		if path == publicPath {
			return false
		}
	}

	// All /drivers/* endpoints require auth
	if strings.HasPrefix(path, "/drivers/") {
		return true
	}

	// Internal endpoints might have different auth requirements
	if strings.HasPrefix(path, "/internal/") {
		// In production, validate service-to-service auth
		return false
	}

	return false
}

// methodNotAllowed sends a 405 Method Not Allowed response
func (r *Router) methodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusMethodNotAllowed)
	w.Write([]byte(`{"error":"Method not allowed"}`))
}

// unauthorizedResponse sends a 401 Unauthorized response
func (r *Router) unauthorizedResponse(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	response := map[string]string{"error": message}

	if err := writeJSON(w, response); err != nil {
		r.logger.Error("router.unauthorized_response", err)
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code
type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Helper functions

func formatLogMessage(method, path string, status int, duration time.Duration) string {
	return strings.Join([]string{method, path, formatStatus(status), duration.String()}, " ")
}

func formatStatus(status int) string {
	return strings.Join([]string{"[", formatInt(status), "]"}, "")
}

func formatInt(n int) string {
	return strings.TrimSpace(strings.Replace(strings.Replace(formatNumber(n), ",", "", -1), " ", "", -1))
}

func formatNumber(n int) string {
	return strings.TrimSpace(formatValue(n))
}

func formatValue(v interface{}) string {
	return strings.TrimSpace(formatAny(v))
}

func formatAny(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case int:
		return formatIntValue(val)
	default:
		return ""
	}
}

func formatIntValue(n int) string {
	if n == 0 {
		return "0"
	}

	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}

func formatPanicMessage(v interface{}) string {
	switch val := v.(type) {
	case error:
		return val.Error()
	case string:
		return val
	default:
		return "Unknown panic"
	}
}

func writeJSON(w http.ResponseWriter, v interface{}) error {
	// Simple JSON marshaling for error responses
	w.Header().Set("Content-Type", "application/json")

	// Manual JSON encoding to avoid imports in simple cases
	switch val := v.(type) {
	case map[string]string:
		result := "{"
		first := true
		for k, v := range val {
			if !first {
				result += ","
			}
			result += `"` + k + `":"` + v + `"`
			first = false
		}
		result += "}"
		_, err := w.Write([]byte(result))
		return err
	default:
		_, err := w.Write([]byte(`{"error":"Internal error"}`))
		return err
	}
}

// panicError wraps panic messages as errors
type panicError struct {
	message string
}

func (e *panicError) Error() string {
	return e.message
}
