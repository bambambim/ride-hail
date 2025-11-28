package rest

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"
)

func driverIDFromRequest(r *http.Request) (string, error) {
	if v := r.PathValue("driver_id"); v != "" {
		return v, nil
	}

	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 2 || parts[0] != "drivers" {
		return "", errors.New("invalid driver path")
	}
	return parts[1], nil
}

func extractBearerToken(r *http.Request) (string, error) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return "", errors.New("missing bearer token")
	}
	return strings.TrimSpace(strings.TrimPrefix(auth, "Bearer ")), nil
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{
		"error":   http.StatusText(status),
		"message": msg,
	})
}

func validateCoordinates(lat, lng float64) bool {
	return lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180
}

func nowISO() string {
	return time.Now().UTC().Format(time.RFC3339)
}
