package rest

import (
	"net/http"
	"strings"
)

func parseDriverID(r *http.Request) (string, error) {
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	// Expecting: /drivers/{driver_id}/...
	if len(parts) < 3 || parts[0] != "drivers" {
		return "", http.ErrNotSupported
	}
	return parts[1], nil
}

func extractBearerToken(r *http.Request) (string, error) {
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return "", http.ErrNoCookie
	}
	return strings.TrimPrefix(auth, "Bearer "), nil
}
