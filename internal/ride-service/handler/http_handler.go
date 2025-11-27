package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"ride-hail/pkg/auth"
	"ride-hail/pkg/logger"
	"ride-hail/pkg/rabbitmq"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Handler handles HTTP requests for users and utility endpoints
// NOTE: Ride creation and cancellation now use clean architecture handlers in internal/ride/
type Handler struct {
	rabbit *rabbitmq.Connection
	log    logger.Logger
	db     *pgxpool.Pool
}

func New(db *pgxpool.Pool, rabbit *rabbitmq.Connection, log logger.Logger) *Handler {
	return &Handler{
		rabbit: rabbit,
		log:    log,
		db:     db,
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// GenerateTestToken is a helper endpoint for testing authentication
// WARNING: This should be removed or secured in production!
type TokenRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"` // PASSENGER, DRIVER, or ADMIN
}

type TokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt string `json:"expires_at"`
	UserID    string `json:"user_id"`
	Role      string `json:"role"`
}

func (h *Handler) GenerateTestToken(w http.ResponseWriter, r *http.Request, jwtManager *auth.JWTManager) {
	var req TokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Default values
	if req.UserID == "" {
		req.UserID = "440e8400-e29b-41d4-a716-446655440003"
	}
	if req.Role == "" {
		req.Role = "PASSENGER"
	}

	// Validate role
	var role auth.Role
	switch req.Role {
	case "PASSENGER":
		role = auth.RolePassenger
	case "DRIVER":
		role = auth.RoleDriver
	case "ADMIN":
		role = auth.RoleAdmin
	default:
		http.Error(w, "Invalid role. Must be PASSENGER, DRIVER, or ADMIN", http.StatusBadRequest)
		return
	}

	// Generate token
	token, err := jwtManager.GenerateToken(req.UserID, role)
	if err != nil {
		h.log.Error("generate_token_failed", err)
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(TokenResponse{
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour).Format(time.RFC3339),
		UserID:    req.UserID,
		Role:      req.Role,
	})
}
