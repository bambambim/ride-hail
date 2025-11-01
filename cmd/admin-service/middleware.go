package adminservice

import (
	"errors"
	"fmt"
	"net/http"
	"ride-hail/pkg/auth"
	"ride-hail/pkg/logger"
)

func adminOnly(log logger.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := auth.GetClaims(r.Context())
		if !ok {
			log.Error("admin_middleware", errors.New("could not retrieve claims from context"))
			writeError(w, http.StatusInternalServerError, "Error processing request")
			return
		}

		if claims.Role != auth.RoleAdmin {
			log.Error("admin_middleware", fmt.Errorf("Unauthorized access attempt: UserID=%s Role=%s ", claims.UserID, auth.RoleAdmin))
			writeError(w, http.StatusUnauthorized, "You do not have permission to access this resource")
			return
		}

		next.ServeHTTP(w, r)

	})
}
