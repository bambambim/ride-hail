package ws

import "errors"

func ValidateToken(token string) (string, error) {
	// TODO: Validate JWT, return service identity
	// For example: "ride_service"
	if token == "" {
		return "", errors.New("empty token")
	}
	return "ride_service", nil
}
