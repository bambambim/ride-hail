package handler

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"ride-hail/pkg/logger"

	"golang.org/x/crypto/bcrypt"
)

// generateUUID generates a UUID v4 string using crypto/rand
func generateUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	b[6] = (b[6] & 0x0f) | 0x40 // Version 4
	b[8] = (b[8] & 0x3f) | 0x80 // Variant RFC4122
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// CreateUserRequest represents the request to create a new user
type CreateUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"` // PASSENGER, DRIVER, ADMIN
	Phone    string `json:"phone,omitempty"`
	Name     string `json:"name,omitempty"`
}

// CreateUserResponse represents the response after creating a user
type CreateUserResponse struct {
	UserID    string    `json:"user_id"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateUser handles user registration
func (h *Handler) CreateUser(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.Error("parse_user_request_failed", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields
	if req.Email == "" {
		http.Error(w, "Email is required", http.StatusBadRequest)
		return
	}
	if req.Password == "" {
		http.Error(w, "Password is required", http.StatusBadRequest)
		return
	}
	if req.Role == "" {
		req.Role = "PASSENGER" // Default role
	}

	// Validate role
	validRoles := map[string]bool{"PASSENGER": true, "DRIVER": true, "ADMIN": true}
	if !validRoles[req.Role] {
		http.Error(w, "Invalid role. Must be PASSENGER, DRIVER, or ADMIN", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Check if user already exists
	var exists bool
	err := h.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE email = $1)", req.Email).Scan(&exists)
	if err != nil {
		h.log.Error("check_user_exists_failed", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if exists {
		http.Error(w, "User with this email already exists", http.StatusConflict)
		return
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		h.log.Error("password_hash_failed", err)
		http.Error(w, "Failed to process password", http.StatusInternalServerError)
		return
	}

	// Generate user ID
	userID := generateUUID()

	// Build attrs JSON
	attrs := map[string]interface{}{}
	if req.Name != "" {
		attrs["name"] = req.Name
	}
	if req.Phone != "" {
		attrs["phone"] = req.Phone
	}

	attrsJSON, _ := json.Marshal(attrs)

	// Insert user
	now := time.Now()
	_, err = h.db.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, role, status, attrs, created_at, updated_at)
		VALUES ($1, $2, $3, $4, 'ACTIVE', $5, $6, $7)
	`, userID, req.Email, string(hashedPassword), req.Role, attrsJSON, now, now)

	if err != nil {
		h.log.WithFields(logger.LogFields{
			"email": req.Email,
			"error": err.Error(),
		}).Error("user_creation_failed", err)
		http.Error(w, "Failed to create user", http.StatusInternalServerError)
		return
	}

	h.log.WithFields(logger.LogFields{
		"user_id": userID,
		"email":   req.Email,
		"role":    req.Role,
	}).Info("user_created", "User created successfully")

	// Return response
	response := CreateUserResponse{
		UserID:    userID,
		Email:     req.Email,
		Role:      req.Role,
		Status:    "ACTIVE",
		CreatedAt: now,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// GetUser retrieves user information by ID
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	var email, role, status string
	var attrs []byte
	var createdAt, updatedAt time.Time

	err := h.db.QueryRow(ctx, `
		SELECT email, role, status, COALESCE(attrs, '{}'::jsonb), created_at, updated_at
		FROM users
		WHERE id = $1
	`, userID).Scan(&email, &role, &status, &attrs, &createdAt, &updatedAt)

	if err != nil {
		h.log.WithFields(logger.LogFields{
			"user_id": userID,
		}).Error("get_user_failed", err)
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Parse attrs
	var attrsMap map[string]interface{}
	json.Unmarshal(attrs, &attrsMap)

	response := map[string]interface{}{
		"user_id":    userID,
		"email":      email,
		"role":       role,
		"status":     status,
		"attrs":      attrsMap,
		"created_at": createdAt,
		"updated_at": updatedAt,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// ListUsers returns a paginated list of users
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get query parameters
	roleFilter := r.URL.Query().Get("role")
	limit := 50 // Default limit

	query := `
		SELECT id, email, role, status, COALESCE(attrs, '{}'::jsonb), created_at
		FROM users
	`
	args := []interface{}{}

	if roleFilter != "" {
		query += " WHERE role = $1"
		args = append(args, roleFilter)
	}

	query += " ORDER BY created_at DESC LIMIT $" + fmt.Sprintf("%d", len(args)+1)
	args = append(args, limit)

	rows, err := h.db.Query(ctx, query, args...)
	if err != nil {
		h.log.Error("list_users_failed", err)
		http.Error(w, "Failed to list users", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var users []map[string]interface{}
	for rows.Next() {
		var id, email, role, status string
		var attrs []byte
		var createdAt time.Time

		err := rows.Scan(&id, &email, &role, &status, &attrs, &createdAt)
		if err != nil {
			h.log.Error("scan_user_failed", err)
			continue
		}

		var attrsMap map[string]interface{}
		json.Unmarshal(attrs, &attrsMap)

		users = append(users, map[string]interface{}{
			"user_id":    id,
			"email":      email,
			"role":       role,
			"status":     status,
			"attrs":      attrsMap,
			"created_at": createdAt,
		})
	}

	response := map[string]interface{}{
		"count": len(users),
		"users": users,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// DeleteUser soft-deletes a user (sets status to INACTIVE)
func (h *Handler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	userID := r.PathValue("user_id")
	if userID == "" {
		http.Error(w, "user_id is required", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// Check if user exists
	var exists bool
	err := h.db.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id = $1)", userID).Scan(&exists)
	if err != nil {
		h.log.Error("check_user_exists_failed", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	if !exists {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	// Soft delete (set status to INACTIVE)
	_, err = h.db.Exec(ctx, `
		UPDATE users 
		SET status = 'INACTIVE', updated_at = NOW()
		WHERE id = $1
	`, userID)

	if err != nil {
		h.log.WithFields(logger.LogFields{
			"user_id": userID,
		}).Error("delete_user_failed", err)
		http.Error(w, "Failed to delete user", http.StatusInternalServerError)
		return
	}

	h.log.WithFields(logger.LogFields{
		"user_id": userID,
	}).Info("user_deleted", "User deleted successfully")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "User deleted successfully",
		"user_id": userID,
	})
}
