package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
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

// --- Structs for API responses ---

// TokenResponse is the successful login/register response.
type TokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	UserID    string `json:"user_id"`
	Role      string `json:"role"`
}

// --- Structs for API requests ---

// LoginRequest is the body for the /login endpoint.
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// RegisterRequest is the body for the /register endpoint.
type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"` // "PASSENGER" or "DRIVER"
}

// --- JSON Helper Functions ---

// writeJSON is a helper for writing JSON responses.
func writeJSON(w http.ResponseWriter, status int, data interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data == nil {
		return nil
	}
	return json.NewEncoder(w).Encode(data)
}

// writeError is a helper for writing standardized JSON error responses.
func writeError(w http.ResponseWriter, status int, message string) {
	type errorResponse struct {
		Error   string `json:"error"`
		Message string `json:"message"`
	}
	writeJSON(w, status, errorResponse{
		Error:   http.StatusText(status),
		Message: message,
	})
}
func main() {
	log := logger.NewLogger("auth-service")
	log.Info("startup", "Starting auth-service")

	cfg, err := config.LoadConfig(".env")
	if err != nil {
		log.Error("startup", fmt.Errorf("failed to load config: %w", err))
		os.Exit(1)
	}

	pool, err := db.NewConnection(cfg, log)
	if err != nil {
		log.Error("startup", fmt.Errorf("failed to connect to database: %w", err))
		os.Exit(1)
	}
	defer pool.Close()

	// Initialize JWTManager
	jwtManager := auth.NewJWTManager("someone", time.Hour)

	// Setup HTTP Server and Handlers
	mux := http.NewServeMux()
	authHandler := NewHandler(pool, log, jwtManager)

	mux.HandleFunc("POST /register", authHandler.SignUp)
	mux.HandleFunc("POST /login", authHandler.Login)

	// Configure and Start Server
	// We use a different port, e.g., 3005, or get it from config
	authPort := 3005 // You should add this to your config
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", authPort),
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	serverErrors := make(chan error, 1)
	go func() {
		log.Info("startup", fmt.Sprintf("Auth service listening on port %d", authPort))
		serverErrors <- server.ListenAndServe()
	}()

	// Implement Graceful Shutdown
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

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Error("shutdown", fmt.Errorf("graceful shutdown failed: %w", err))
	}

	log.Info("shutdown", "Auth service stopped gracefully")
}

// Handler holds the dependencies for the auth service handlers.
type Handler struct {
	pool    *pgxpool.Pool
	log     logger.Logger
	jwtMng  *auth.JWTManager
	testEnv bool // Flag to bypass password hashing in test
}

// NewHandler creates a new Handler.
func NewHandler(pool *pgxpool.Pool, log logger.Logger, jwtMng *auth.JWTManager) *Handler {
	return &Handler{
		pool:   pool,
		log:    log,
		jwtMng: jwtMng,
	}
}

// SignUp handles new user registration.
func (h *Handler) SignUp(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("signup_decode_error", err)
		writeError(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	// Validate input
	if req.Email == "" || req.Password == "" || req.Role == "" {
		writeError(w, http.StatusBadRequest, "Email, password, and role are required")
		return
	}

	// 1. Validate and convert role
	var role auth.Role
	switch req.Role {
	case "PASSENGER":
		role = auth.RolePassenger
	case "DRIVER":
		role = auth.RoleDriver
	case "ADMIN":
		// Do not allow admin signups via API
		h.log.Error("signup_admin_attempt", fmt.Errorf("attempt to register admin: %s", req.Email))
		writeError(w, http.StatusForbidden, "Admin registration is not allowed")
		return
	default:
		writeError(w, http.StatusBadRequest, "Invalid role. Must be PASSENGER or DRIVER")
		return
	}

	plainTextPassword := req.Password

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		h.log.Error("signup_begin_tx", err)
		writeError(w, http.StatusInternalServerError, "Database error")
		return
	}
	defer tx.Rollback(ctx)

	var userID string
	err = tx.QueryRow(ctx,
		`INSERT INTO users (email, role, password_hash) VALUES ($1, $2, $3) RETURNING id`,
		req.Email, role, plainTextPassword, // Storing plain text
	).Scan(&userID)

	if err != nil {
		// Check for duplicate email
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" { // Unique violation
			h.log.Error("signup_duplicate_email", err)
			writeError(w, http.StatusConflict, "A user with this email already exists")
		} else {
			h.log.Error("signup_insert_user", err)
			writeError(w, http.StatusInternalServerError, "Failed to create user")
		}
		return
	}

	// 5. If DRIVER, also insert into 'drivers' table
	if role == auth.RoleDriver {

		fakeLicense := fmt.Sprintf("FAKE-%d", time.Now().UnixNano())
		_, err := tx.Exec(ctx,
			`INSERT INTO drivers (id, license_number, vehicle_type, status) VALUES ($1, $2, $3, $4)`,
			userID, fakeLicense, "ECONOMY", "OFFLINE",
		)
		if err != nil {
			h.log.WithFields(logger.LogFields{"user_id": userID}).Error("signup_insert_driver", err)
			writeError(w, http.StatusInternalServerError, "Failed to create driver profile")
			return
		}
	}

	// 6. Commit transaction
	if err := tx.Commit(ctx); err != nil {
		h.log.Error("signup_commit_tx", err)
		writeError(w, http.StatusInternalServerError, "Failed to save registration")
		return
	}

	// 7. Generate JWT Token
	token, err := h.jwtMng.GenerateToken(userID, role)
	if err != nil {
		h.log.WithFields(logger.LogFields{"user_id": userID}).Error("startup_generate_token", err)
		writeError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	// 8. Send successful response
	writeJSON(w, http.StatusCreated, TokenResponse{
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		UserID:    userID,
		Role:      string(role),
	})
}

// Login handles authentication for existing users.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("login_decode_error", err)
		writeError(w, http.StatusBadRequest, "Invalid request format")
		return
	}

	if req.Email == "" || req.Password == "" {
		writeError(w, http.StatusBadRequest, "Email and password are required")
		return
	}

	log := h.log.WithFields(logger.LogFields{"email": req.Email})

	// 1. Find user by email
	var userID, storedPassword, userRole string // storedPassword is plain text
	err := h.pool.QueryRow(ctx,
		`SELECT id, password_hash, role FROM users WHERE email = $1 AND status = 'ACTIVE'`,
		req.Email,
	).Scan(&userID, &storedPassword, &userRole)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			log.Error("login_user_not_found", err)
			writeError(w, http.StatusUnauthorized, "Invalid email or password")
		} else {
			log.Error("login_query_user", err)
			writeError(w, http.StatusInternalServerError, "Database error")
		}
		return
	}

	// 2. Compare password (plain text comparison)
	if storedPassword != req.Password {
		log.Error("login_password_mismatch", errors.New("plain text password mismatch"))
		writeError(w, http.StatusUnauthorized, "Invalid email or password")
		return
	}

	// 3. Generate JWT Token
	role := auth.Role(userRole) // Convert string from DB to auth.Role
	token, err := h.jwtMng.GenerateToken(userID, role)
	if err != nil {
		log.WithFields(logger.LogFields{"user_id": userID}).Error("login_generate_token", err)
		writeError(w, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	// 4. Send successful response
	log.Info("login_success", "User authenticated successfully")
	writeJSON(w, http.StatusOK, TokenResponse{
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		UserID:    userID,
		Role:      string(role),
	})
}
