package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"ride-hail/internal/ride-service/repository"
	"ride-hail/internal/ride-service/service"
	"ride-hail/pkg/auth"
	"ride-hail/pkg/logger"
	"ride-hail/pkg/rabbitmq"
	"ride-hail/pkg/uuid"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Handler struct {
	repo   *repository.RideRepository
	rabbit *rabbitmq.Connection
	log    logger.Logger
	db     *pgxpool.Pool
}

func New(db *pgxpool.Pool, rabbit *rabbitmq.Connection, log logger.Logger) *Handler {
	return &Handler{
		repo:   repository.New(db),
		rabbit: rabbit,
		log:    log,
		db:     db,
	}
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

type CreateRideRequest struct {
	PickupLatitude       float64 `json:"pickup_latitude"`
	PickupLongitude      float64 `json:"pickup_longitude"`
	PickupAddress        string  `json:"pickup_address,omitempty"`
	DestinationLatitude  float64 `json:"destination_latitude"`
	DestinationLongitude float64 `json:"destination_longitude"`
	DestinationAddress   string  `json:"destination_address,omitempty"`
	RideType             string  `json:"ride_type"`
}

type CreateRideResponse struct {
	RideID        string  `json:"ride_id"`
	Status        string  `json:"status"`
	EstimatedFare float64 `json:"estimated_fare"`
	RideNumber    string  `json:"ride_number"`
}

func (h *Handler) CreateRide(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request
	var req CreateRideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.log.WithFields(logger.LogFields{
			"error": err.Error(),
		}).Error("parse_request_failed", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	h.log.WithFields(logger.LogFields{
		"pickup_lat": req.PickupLatitude,
		"pickup_lng": req.PickupLongitude,
		"dest_lat":   req.DestinationLatitude,
		"dest_lng":   req.DestinationLongitude,
		"ride_type":  req.RideType,
	}).Info("create_ride_request", "Received create ride request")

	// Validate that coordinates are provided (not zero values)
	if req.PickupLatitude == 0.0 || req.PickupLongitude == 0.0 {
		http.Error(w, "Pickup location is required", http.StatusBadRequest)
		return
	}
	if req.DestinationLatitude == 0.0 || req.DestinationLongitude == 0.0 {
		http.Error(w, "Destination location is required", http.StatusBadRequest)
		return
	}

	// Validate coordinate ranges
	if !isValidCoordinate(req.PickupLatitude, req.PickupLongitude) {
		http.Error(w, "Invalid pickup coordinates", http.StatusBadRequest)
		return
	}
	if !isValidCoordinate(req.DestinationLatitude, req.DestinationLongitude) {
		http.Error(w, "Invalid destination coordinates", http.StatusBadRequest)
		return
	}

	// Validate ride_type
	validTypes := map[string]bool{"ECONOMY": true, "PREMIUM": true, "XL": true}
	if !validTypes[req.RideType] {
		h.log.Error("invalid_ride_type", fmt.Errorf("invalid type: %s", req.RideType))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Invalid ride_type"})
		return
	}

	// Extract passenger_id from JWT token
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		h.log.Error("missing_claims", fmt.Errorf("no claims in context"))
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	h.log.WithFields(logger.LogFields{
		"user_id": claims.UserID,
		"role":    claims.Role,
	}).Info("auth_claims_extracted", "JWT claims extracted successfully")

	// Verify the user is a passenger
	if claims.Role != auth.RolePassenger {
		h.log.WithFields(logger.LogFields{
			"user_id": claims.UserID,
			"role":    claims.Role,
		}).Error("invalid_role", fmt.Errorf("role %s not allowed", claims.Role))
		http.Error(w, "Only passengers can create rides", http.StatusForbidden)
		return
	}

	passengerID := claims.UserID

	h.log.WithFields(logger.LogFields{
		"passenger_id": passengerID,
	}).Info("passenger_verified", "Passenger authentication verified") // Generate ride ID
	rideID := uuid.MustNewV4().String()
	correlationID := fmt.Sprintf("req_%d", time.Now().Unix())

	// Calculate fare
	fare := service.CalculateFare(
		req.PickupLatitude,
		req.PickupLongitude,
		req.DestinationLatitude,
		req.DestinationLongitude,
		req.RideType,
	)

	// Create ride in database
	ride := repository.Ride{
		RideID:        rideID,
		PassengerID:   passengerID,
		Status:        "REQUESTED",
		RideType:      req.RideType,
		EstimatedFare: fare,
		RequestedAt:   time.Now(),
	}

	pickup := repository.Coordinate{
		RideID:    rideID,
		Type:      "PICKUP",
		Latitude:  req.PickupLatitude,
		Longitude: req.PickupLongitude,
		IsCurrent: true,
	}

	dest := repository.Coordinate{
		RideID:    rideID,
		Type:      "DESTINATION",
		Latitude:  req.DestinationLatitude,
		Longitude: req.DestinationLongitude,
		IsCurrent: true,
	}

	h.log.WithFields(logger.LogFields{
		"ride_id":        rideID,
		"passenger_id":   passengerID,
		"ride_type":      req.RideType,
		"estimated_fare": fare,
	}).Info("creating_ride", "Creating ride in database")

	if err := h.repo.CreateRide(r.Context(), ride, pickup, dest); err != nil {
		h.log.WithFields(logger.LogFields{
			"ride_id":      rideID,
			"passenger_id": passengerID,
			"error":        err.Error(),
		}).Error("create_ride_failed", err)
		http.Error(w, "Failed to create ride", http.StatusInternalServerError)
		return
	}

	logWithFields := h.log.WithFields(logger.LogFields{
		"ride_id":        rideID,
		"correlation_id": correlationID,
	})
	logWithFields.Info("ride_created", "Ride request created successfully")

	// Publish to RabbitMQ
	message := map[string]interface{}{
		"ride_id":      rideID,
		"passenger_id": passengerID,
		"ride_type":    req.RideType,
		"pickup_location": map[string]interface{}{
			"latitude":  req.PickupLatitude,
			"longitude": req.PickupLongitude,
			"address":   req.PickupAddress,
		},
		"destination_location": map[string]interface{}{
			"latitude":  req.DestinationLatitude,
			"longitude": req.DestinationLongitude,
			"address":   req.DestinationAddress,
		},
		"estimated_fare": fare,
		"requested_at":   time.Now(),
		"correlation_id": correlationID,
	}

	messageBytes, err := json.Marshal(message)
	if err != nil {
		h.log.Error("marshal_message_failed", err)
	} else {
		routingKey := fmt.Sprintf("ride.request.%s", req.RideType)
		if err := h.rabbit.Publish(r.Context(), "ride_topic", routingKey, messageBytes); err != nil {
			h.log.Error("publish_message_failed", err)
			// Continue anyway - ride is already created
		} else {
			logWithFields.Info("ride_request_published", "Message published to RabbitMQ")
		}
	}

	// Return response
	rideNumber := fmt.Sprintf("RIDE_%s_%s", time.Now().Format("20060102"), rideID[:8])
	resp := CreateRideResponse{
		RideID:        rideID,
		Status:        "REQUESTED",
		EstimatedFare: fare,
		RideNumber:    rideNumber,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp)
}

func isValidCoordinate(lat, lng float64) bool {
	return lat >= -90 && lat <= 90 && lng >= -180 && lng <= 180
}

type CancelRideRequest struct {
	Reason string `json:"reason"` // Optional cancellation reason
}

// CancelRide handles ride cancellation by passengers
func (h *Handler) CancelRide(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Extract ride_id from URL path (e.g., /rides/{ride_id}/cancel)
	rideID := r.PathValue("ride_id")
	if rideID == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "ride_id required"})
		return
	}

	// Parse request body (reason is optional)
	var req CancelRideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// If body is empty or invalid JSON, continue with empty reason
		req.Reason = ""
	}

	// Extract passenger_id from JWT token
	claims, ok := auth.GetClaims(r.Context())
	if !ok {
		h.log.Error("missing_claims", fmt.Errorf("no claims in context"))
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Verify the user is a passenger
	if claims.Role != auth.RolePassenger {
		h.log.WithFields(logger.LogFields{
			"user_id": claims.UserID,
			"role":    claims.Role,
		}).Error("invalid_role", fmt.Errorf("role %s not allowed", claims.Role))
		http.Error(w, "Only passengers can cancel rides", http.StatusForbidden)
		return
	}

	passengerID := claims.UserID

	// Verify the ride belongs to the passenger
	ride, err := h.repo.GetRideByPassenger(r.Context(), rideID, passengerID)
	if err != nil {
		h.log.Error("get_ride_for_cancellation", err)
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "Ride not found"})
		return
	}

	if ride.Status == "IN_PROGRESS" || ride.Status == "COMPLETED" || ride.Status == "CANCELLED" {
		h.log.Error("invalid_status_for_cancellation", fmt.Errorf("status: %s", ride.Status))
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{"error": "Ride cannot be cancelled"})
		return
	}

	// Cancel the ride with reason
	if err := h.repo.CancelRide(r.Context(), rideID, req.Reason); err != nil {
		h.log.Error("cancel_ride", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": "Failed to cancel ride"})
		return
	}

	// Publish cancellation message to RabbitMQ if the ride was matched
	if ride.Status == "MATCHED" {
		message := map[string]interface{}{
			"ride_id":      rideID,
			"passenger_id": passengerID,
			"status":       "CANCELLED",
			"cancelled_at": time.Now(),
			"reason":       req.Reason,
		}

		messageBytes, _ := json.Marshal(message)
		if err := h.rabbit.Publish(r.Context(), "ride_topic", "ride.cancelled."+rideID, messageBytes); err != nil {
			h.log.Error("publish_cancellation", err)
		}
	}

	h.log.WithFields(logger.LogFields{
		"ride_id": rideID,
		"reason":  req.Reason,
	}).Info("ride_cancelled", "Ride cancelled by passenger")

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"ride_id":      rideID,
		"status":       "CANCELLED",
		"cancelled_at": time.Now().Format(time.RFC3339),
		"message":      "Ride cancelled successfully",
	})
}

// GenerateTestToken is a helper endpoint for testing authentication
// WARNING: This should be removed or secured in production!
type TokenRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"` // PASSANGER, DRIVER, or ADMIN
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
		req.Role = "PASSANGER"
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
		http.Error(w, "Invalid role. Must be PASSANGER, DRIVER, or ADMIN", http.StatusBadRequest)
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
