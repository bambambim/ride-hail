package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
	"net/http"
	"strings"
	"time"
)

type Role string

const (
	RolePassanger Role = "PASSANGER"
	RoleDriver    Role = "DRIVER"
	RoleAdmin     Role = "ADMIN"
)

type contextKey string

const claimsKey = contextKey("claims")

type AppClaims struct {
	UserID string `json:"user_id"`
	Role   Role   `json:"role"`
	jwt.RegisteredClaims
}

// JWTManager handles generating and verifying JWT tokens.
type JWTManager struct {
	secretKey     []byte
	tokenDuration time.Duration
}

func NewJWTManager(secretKey string, tokenDuration time.Duration) *JWTManager {
	return &JWTManager{[]byte(secretKey), tokenDuration}
}

func (m *JWTManager) GenerateToken(userID string, role Role) (string, error) {
	claims := AppClaims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.tokenDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "ride-hail-system",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secretKey)
}

// ParseToken checks the token's validity and returns the claims
func (m *JWTManager) ParseToken(tokenString string) (*AppClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &AppClaims{}, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secretKey, nil
	},
	)

	if err != nil {
		return nil, fmt.Errorf("failed toparse token: %w", err)
	}

	if claims, ok := token.Claims.(*AppClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, fmt.Errorf("invalid token")
}

// AuthMiddleware is an HTTP middleware that verifies the JWT token.
func (m *JWTManager) AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			writeError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			writeError(w, http.StatusUnauthorized, "invalid authorization header")
			return
		}

		claims, err := m.ParseToken(parts[1])
		if err != nil {
			writeError(w, http.StatusUnauthorized, fmt.Sprintf("invalid token: %w", err))
			return
		}

		ctx := context.WithValue(r.Context(), claimsKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))

	})
}

// GetClaims retrieves the AppClaims from the request context.
// This is used by handlers *after* the AuthMiddleware.
func GetClaims(ctx context.Context) (*AppClaims, bool) {
	claims, ok := ctx.Value(claimsKey).(*AppClaims)
	return claims, ok
}
func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   http.StatusText(code),
		"message": msg,
	})

}
