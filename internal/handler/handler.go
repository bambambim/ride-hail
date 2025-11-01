package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"ride-hail/internal/domain"
	"ride-hail/internal/service"
	"ride-hail/pkg/logger"
)

// RideHandler provides HTTP endpoints for ride management.
type RideHandler struct {
	Service *service.RideService
	Log     logger.Logger
}

func NewHandler(s *service.RideService, log logger.Logger) *RideHandler {
	return &RideHandler{
		Service: s,
		Log:     log,
	}
}

// HandleCreateRide handles POST /rides requests.
func (h *RideHandler) HandleCreateRide(w http.ResponseWriter, r *http.Request) {
	var req domain.RideRequest
	ctx := r.Context()

	// 1. Decode Request
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.Log.Error("decode_ride_request_failed", err)
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// 2. Validate Request
	if err := req.Validate(); err != nil {
		h.Log.Error("ride_request_validation_failed", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// 3. Call Service Logic
	rideID, err := h.Service.CreateRide(ctx, req)
	if err != nil {
		h.Log.Error("create_ride_service_failed", err)
		http.Error(w, "Failed to process ride request", http.StatusInternalServerError)
		return
	}

	// 4. Success Response
	w.WriteHeader(http.StatusCreated)
	w.Header().Set("Content-Type", "application/json")

	// Return the newly created ride ID as a string
	response := map[string]string{
		"ride_id": rideID.String(),
		"status":  "REQUESTED",
	}
	json.NewEncoder(w).Encode(response)

	h.Log.Info("ride_successfully_requested", fmt.Sprintf("Ride %s requested by passenger %s", rideID.String(), req.PassengerID.String()))
}

// RegisterRoutes is a helper to centralize route definitions
func (h *RideHandler) RegisterRoutes(router *http.ServeMux) {
	// Using the standard Go http.ServeMux
	router.HandleFunc("/rides", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			h.HandleCreateRide(w, r)
		} else {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		}
	})
}
