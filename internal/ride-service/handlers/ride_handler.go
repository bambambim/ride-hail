package http

import (
	"encoding/json"
	"net/http"
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
