package http

import (
	"encoding/json"
	"net/http"

	"ride-hail/internal/ride-service/application"
	"ride-hail/pkg/auth"
	"ride-hail/pkg/logger"
)

// RideHandler handles HTTP requests for rides using clean architecture
type RideHandler struct {
	createRideUseCase *application.CreateRideUseCase
	cancelRideUseCase *application.CancelRideUseCase
	logger            logger.Logger
}

// NewRideHandler creates a new ride handler
func NewRideHandler(
	createRideUseCase *application.CreateRideUseCase,
	cancelRideUseCase *application.CancelRideUseCase,
	logger logger.Logger,
) *RideHandler {
	return &RideHandler{
		createRideUseCase: createRideUseCase,
		cancelRideUseCase: cancelRideUseCase,
		logger:            logger,
	}
}

// CreateRideRequest represents the HTTP request for creating a ride
type CreateRideRequest struct {
	PickupLatitude       float64 `json:"pickup_latitude"`
	PickupLongitude      float64 `json:"pickup_longitude"`
	PickupAddress        string  `json:"pickup_address,omitempty"`
	DestinationLatitude  float64 `json:"destination_latitude"`
	DestinationLongitude float64 `json:"destination_longitude"`
	DestinationAddress   string  `json:"destination_address,omitempty"`
	RideType             string  `json:"ride_type"`
}

// CreateRideResponse represents the HTTP response for creating a ride
type CreateRideResponse struct {
	RideID        string  `json:"ride_id"`
	RideNumber    string  `json:"ride_number"`
	Status        string  `json:"status"`
	EstimatedFare float64 `json:"estimated_fare"`
}

// CreateRide handles POST /rides
func (h *RideHandler) CreateRide(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Parse HTTP request
	var req CreateRideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.WithFields(logger.LogFields{
			"error": err.Error(),
		}).Error("parse_request_failed", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.logger.WithFields(logger.LogFields{
		"pickup_lat": req.PickupLatitude,
		"pickup_lng": req.PickupLongitude,
		"dest_lat":   req.DestinationLatitude,
		"dest_lng":   req.DestinationLongitude,
		"ride_type":  req.RideType,
	}).Info("create_ride_request", "Received create ride request")

	// 2. Extract passenger ID from JWT context
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		h.logger.Error("missing_claims", nil)
		http.Error(w, "Unauthorized - missing claims", http.StatusUnauthorized)
		return
	}

	// Verify the user is a passenger
	if claims.Role != auth.RolePassenger {
		h.logger.Error("invalid_role", nil)
		http.Error(w, "Only passengers can create rides", http.StatusForbidden)
		return
	}

	passengerID := claims.UserID

	// 3. Convert HTTP request to application command
	cmd := application.CreateRideCommand{
		PassengerID:          passengerID,
		PickupLatitude:       req.PickupLatitude,
		PickupLongitude:      req.PickupLongitude,
		PickupAddress:        req.PickupAddress,
		DestinationLatitude:  req.DestinationLatitude,
		DestinationLongitude: req.DestinationLongitude,
		DestinationAddress:   req.DestinationAddress,
		RideType:             req.RideType,
	}
	// 4. Execute use case (business logic is here)
	result, err := h.createRideUseCase.Execute(r.Context(), cmd)
	if err != nil {
		h.logger.WithFields(logger.LogFields{
			"error":        err.Error(),
			"passenger_id": passengerID,
		}).Error("create_ride_failed", err)

		// Map domain errors to HTTP status codes
		statusCode := mapErrorToStatusCode(err)
		http.Error(w, err.Error(), statusCode)
		return
	}

	// 5. Send HTTP response
	response := CreateRideResponse{
		RideID:        result.ID,
		RideNumber:    result.RideNumber,
		Status:        result.Status,
		EstimatedFare: result.EstimatedFare,
	}

	h.logger.WithFields(logger.LogFields{
		"ride_id":      result.ID,
		"passenger_id": passengerID,
	}).Info("ride_created", "Ride created successfully")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(response)
}

// CancelRideRequest represents the HTTP request for cancelling a ride
type CancelRideRequest struct {
	Reason string `json:"reason,omitempty"`
}

// CancelRide handles POST /rides/{ride_id}/cancel
func (h *RideHandler) CancelRide(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Get ride ID from URL path
	rideID := r.PathValue("ride_id")
	if rideID == "" {
		http.Error(w, "Ride ID is required", http.StatusBadRequest)
		return
	}

	// 2. Parse request body (optional reason)
	var req CancelRideRequest
	if r.Body != nil && r.ContentLength > 0 {
		json.NewDecoder(r.Body).Decode(&req)
	}
	if req.Reason == "" {
		req.Reason = "Cancelled by passenger"
	}

	// 3. Extract passenger ID from JWT context
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		http.Error(w, "Unauthorized - missing claims", http.StatusUnauthorized)
		return
	}

	passengerID := claims.UserID

	h.logger.WithFields(logger.LogFields{
		"ride_id":      rideID,
		"passenger_id": passengerID,
		"reason":       req.Reason,
	}).Info("cancel_ride_request", "Received cancel ride request")

	// 4. Convert to application command
	cmd := application.CancelRideCommand{
		RideID:      rideID,
		PassengerID: passengerID,
		Reason:      req.Reason,
	}

	// 5. Execute use case
	if err := h.cancelRideUseCase.Execute(r.Context(), cmd); err != nil {
		h.logger.WithFields(logger.LogFields{
			"error":   err.Error(),
			"ride_id": rideID,
		}).Error("cancel_ride_failed", err)

		statusCode := mapErrorToStatusCode(err)
		http.Error(w, err.Error(), statusCode)
		return
	}

	h.logger.WithFields(logger.LogFields{
		"ride_id":      rideID,
		"passenger_id": passengerID,
	}).Info("ride_cancelled", "Ride cancelled successfully")

	// 6. Send response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Ride cancelled successfully",
		"ride_id": rideID,
	})
}

// Health returns health check status
func (h *RideHandler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// mapErrorToStatusCode maps domain errors to HTTP status codes
func mapErrorToStatusCode(err error) int {
	errMsg := err.Error()

	// Check for common domain errors
	switch {
	case contains(errMsg, "invalid"):
		return http.StatusBadRequest
	case contains(errMsg, "not found"):
		return http.StatusNotFound
	case contains(errMsg, "cannot cancel"):
		return http.StatusConflict
	case contains(errMsg, "unauthorized"):
		return http.StatusUnauthorized
	default:
		return http.StatusInternalServerError
	}
}

// contains checks if s contains substr (case insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsIgnoreCase(s, substr))
}

func containsIgnoreCase(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFold(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalFold(s, t string) bool {
	if len(s) != len(t) {
		return false
	}
	for i := 0; i < len(s); i++ {
		sr := s[i]
		tr := t[i]
		if sr >= 'A' && sr <= 'Z' {
			sr = sr + 32
		}
		if tr >= 'A' && tr <= 'Z' {
			tr = tr + 32
		}
		if sr != tr {
			return false
		}
	}
	return true
}
