package http

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"ride-hail/pkg/logger"
	"ride-hail/services/driver_location_service/internal/domain"
	"ride-hail/services/driver_location_service/internal/ports"
)

// Handler contains the HTTP handlers for the driver location service
type Handler struct {
	service ports.DriverService
	logger  logger.Logger
}

// HandlerConfig holds configuration for the handler
type HandlerConfig struct {
	Service ports.DriverService
	Logger  logger.Logger
}

// NewHandler creates a new HTTP handler
func NewHandler(config HandlerConfig) *Handler {
	return &Handler{
		service: config.Service,
		logger:  config.Logger,
	}
}

// HealthCheck handles health check requests
func (h *Handler) HealthCheck(w http.ResponseWriter, r *http.Request) {
	h.respondJSON(w, http.StatusOK, map[string]string{
		"status":  "healthy",
		"service": "driver-location-service",
		"time":    time.Now().Format(time.RFC3339),
	})
}

// GoOnline handles driver going online
// POST /drivers/{driver_id}/online
func (h *Handler) GoOnline(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract driver ID from JWT token
	driverID, err := h.extractDriverID(r)
	if err != nil {
		h.logger.Error("handler.go_online.extract_driver_id", err)
		h.respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Parse request body
	var req domain.OnlineRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("handler.go_online.decode_request", err)
		h.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if err := h.validateOnlineRequest(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Call service
	response, err := h.service.GoOnline(ctx, driverID, &req)
	if err != nil {
		h.logger.Error("handler.go_online.service", err)
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, response)
}

// GoOffline handles driver going offline
// POST /drivers/{driver_id}/offline
func (h *Handler) GoOffline(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract driver ID from JWT token
	driverID, err := h.extractDriverID(r)
	if err != nil {
		h.logger.Error("handler.go_offline.extract_driver_id", err)
		h.respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Call service
	response, err := h.service.GoOffline(ctx, driverID)
	if err != nil {
		h.logger.Error("handler.go_offline.service", err)
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, response)
}

// UpdateLocation handles driver location updates
// POST /drivers/{driver_id}/location
func (h *Handler) UpdateLocation(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract driver ID from JWT token
	driverID, err := h.extractDriverID(r)
	if err != nil {
		h.logger.Error("handler.update_location.extract_driver_id", err)
		h.respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Parse request body
	var req domain.UpdateLocationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("handler.update_location.decode_request", err)
		h.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if err := h.validateLocationRequest(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Call service
	response, err := h.service.UpdateLocation(ctx, driverID, &req)
	if err != nil {
		h.logger.Error("handler.update_location.service", err)

		// Check for rate limit error
		if strings.Contains(err.Error(), "rate limit") {
			h.respondError(w, http.StatusTooManyRequests, err.Error())
			return
		}

		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, response)
}

// StartRide handles starting a ride
// POST /drivers/{driver_id}/start
func (h *Handler) StartRide(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract driver ID from JWT token
	driverID, err := h.extractDriverID(r)
	if err != nil {
		h.logger.Error("handler.start_ride.extract_driver_id", err)
		h.respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Parse request body
	var req domain.StartRideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("handler.start_ride.decode_request", err)
		h.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if req.RideID == "" {
		h.respondError(w, http.StatusBadRequest, "ride_id is required")
		return
	}

	if err := h.validateLocation(&req.DriverLocation); err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Call service
	response, err := h.service.StartRide(ctx, driverID, &req)
	if err != nil {
		h.logger.Error("handler.start_ride.service", err)
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, response)
}

// CompleteRide handles completing a ride
// POST /drivers/{driver_id}/complete
func (h *Handler) CompleteRide(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract driver ID from JWT token
	driverID, err := h.extractDriverID(r)
	if err != nil {
		h.logger.Error("handler.complete_ride.extract_driver_id", err)
		h.respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Parse request body
	var req domain.CompleteRideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Error("handler.complete_ride.decode_request", err)
		h.respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Validate request
	if req.RideID == "" {
		h.respondError(w, http.StatusBadRequest, "ride_id is required")
		return
	}

	if err := h.validateLocation(&req.FinalLocation); err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if req.ActualDistanceKm <= 0 {
		h.respondError(w, http.StatusBadRequest, "actual_distance_km must be greater than 0")
		return
	}

	if req.ActualDurationMinutes <= 0 {
		h.respondError(w, http.StatusBadRequest, "actual_duration_minutes must be greater than 0")
		return
	}

	// Call service
	response, err := h.service.CompleteRide(ctx, driverID, &req)
	if err != nil {
		h.logger.Error("handler.complete_ride.service", err)
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, response)
}

// GetDriver handles getting driver information
// GET /drivers/{driver_id}
func (h *Handler) GetDriver(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract driver ID from JWT token
	driverID, err := h.extractDriverID(r)
	if err != nil {
		h.logger.Error("handler.get_driver.extract_driver_id", err)
		h.respondError(w, http.StatusUnauthorized, "Unauthorized")
		return
	}

	// Call service
	driver, err := h.service.GetDriver(ctx, driverID)
	if err != nil {
		h.logger.Error("handler.get_driver.service", err)
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, driver)
}

// GetNearbyDrivers handles finding nearby drivers (internal endpoint)
// GET /internal/drivers/nearby?latitude=X&longitude=Y&radius=Z&vehicle_type=TYPE&limit=N
func (h *Handler) GetNearbyDrivers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	lat, err := h.parseFloatParam(r, "latitude")
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid latitude")
		return
	}

	lng, err := h.parseFloatParam(r, "longitude")
	if err != nil {
		h.respondError(w, http.StatusBadRequest, "Invalid longitude")
		return
	}

	radius, err := h.parseFloatParam(r, "radius")
	if err != nil || radius <= 0 {
		radius = 5.0 // Default 5km radius
	}

	vehicleType := r.URL.Query().Get("vehicle_type")
	if vehicleType == "" {
		vehicleType = "ECONOMY" // Default vehicle type
	}

	limit, err := h.parseIntParam(r, "limit")
	if err != nil || limit <= 0 {
		limit = 10 // Default limit
	}

	// Call service
	drivers, err := h.service.GetNearbyDrivers(ctx, lat, lng, radius, vehicleType, limit)
	if err != nil {
		h.logger.Error("handler.get_nearby_drivers.service", err)
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{
		"drivers": drivers,
		"count":   len(drivers),
	})
}

// Helper methods

// extractDriverID extracts the driver ID from the JWT token
func (h *Handler) extractDriverID(r *http.Request) (string, error) {
	// Get Authorization header
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return "", ErrUnauthorized
	}

	// Check Bearer token format
	parts := strings.Split(authHeader, " ")
	if len(parts) != 2 || parts[0] != "Bearer" {
		return "", ErrInvalidToken
	}

	// In a real implementation, you would:
	// 1. Parse and validate the JWT token
	// 2. Extract the driver ID from the token claims
	// 3. Verify token signature and expiration

	// For now, extract from path as fallback
	// In production, this should come from JWT claims
	path := r.URL.Path
	segments := strings.Split(strings.Trim(path, "/"), "/")

	if len(segments) >= 2 && segments[0] == "drivers" {
		return segments[1], nil
	}

	return "", ErrInvalidToken
}

// validateOnlineRequest validates the online request
func (h *Handler) validateOnlineRequest(req *domain.OnlineRequest) error {
	if req.Latitude < -90 || req.Latitude > 90 {
		return ErrInvalidLatitude
	}
	if req.Longitude < -180 || req.Longitude > 180 {
		return ErrInvalidLongitude
	}
	return nil
}

// validateLocationRequest validates the location update request
func (h *Handler) validateLocationRequest(req *domain.UpdateLocationRequest) error {
	if req.Latitude < -90 || req.Latitude > 90 {
		return ErrInvalidLatitude
	}
	if req.Longitude < -180 || req.Longitude > 180 {
		return ErrInvalidLongitude
	}
	if req.AccuracyMeters != nil && *req.AccuracyMeters < 0 {
		return ErrInvalidAccuracy
	}
	if req.SpeedKmh != nil && *req.SpeedKmh < 0 {
		return ErrInvalidSpeed
	}
	if req.HeadingDegrees != nil && (*req.HeadingDegrees < 0 || *req.HeadingDegrees > 360) {
		return ErrInvalidHeading
	}
	return nil
}

// validateLocation validates a location object
func (h *Handler) validateLocation(loc *domain.Location) error {
	if loc.Latitude < -90 || loc.Latitude > 90 {
		return ErrInvalidLatitude
	}
	if loc.Longitude < -180 || loc.Longitude > 180 {
		return ErrInvalidLongitude
	}
	return nil
}

// parseFloatParam parses a float query parameter
func (h *Handler) parseFloatParam(r *http.Request, key string) (float64, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return 0, ErrMissingParameter
	}

	var result float64
	_, err := fmt.Sscanf(value, "%f", &result)
	return result, err
}

// parseIntParam parses an int query parameter
func (h *Handler) parseIntParam(r *http.Request, key string) (int, error) {
	value := r.URL.Query().Get(key)
	if value == "" {
		return 0, ErrMissingParameter
	}

	var result int
	_, err := fmt.Sscanf(value, "%d", &result)
	return result, err
}

// respondJSON sends a JSON response
func (h *Handler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(data); err != nil {
		h.logger.Error("handler.respond_json", err)
	}
}

// respondError sends an error response
func (h *Handler) respondError(w http.ResponseWriter, status int, message string) {
	h.respondJSON(w, status, domain.ErrorResponse{
		Error: message,
	})
}

// Error definitions
var (
	ErrUnauthorized     = &HandlerError{Code: "UNAUTHORIZED", Message: "Unauthorized"}
	ErrInvalidToken     = &HandlerError{Code: "INVALID_TOKEN", Message: "Invalid token"}
	ErrInvalidLatitude  = &HandlerError{Code: "INVALID_LATITUDE", Message: "Latitude must be between -90 and 90"}
	ErrInvalidLongitude = &HandlerError{Code: "INVALID_LONGITUDE", Message: "Longitude must be between -180 and 180"}
	ErrInvalidAccuracy  = &HandlerError{Code: "INVALID_ACCURACY", Message: "Accuracy meters must be non-negative"}
	ErrInvalidSpeed     = &HandlerError{Code: "INVALID_SPEED", Message: "Speed must be non-negative"}
	ErrInvalidHeading   = &HandlerError{Code: "INVALID_HEADING", Message: "Heading must be between 0 and 360"}
	ErrMissingParameter = &HandlerError{Code: "MISSING_PARAMETER", Message: "Required parameter is missing"}
)

// HandlerError represents a handler error
type HandlerError struct {
	Code    string
	Message string
}

func (e *HandlerError) Error() string {
	return e.Message
}
