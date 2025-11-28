package rest

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"ride-hail/internal/driver_location_service/domain"
	"ride-hail/pkg/auth"
	"ride-hail/pkg/logger"
)

// Handler hosts REST endpoints for driver operations.
type Handler struct {
	driverLocationService domain.DriverLocationService
	log                   logger.Logger
	jwt                   *auth.JWTManager
}

// NewHandler creates a handler with all required dependencies.
func NewHandler(dls domain.DriverLocationService, jwt *auth.JWTManager, log logger.Logger) *Handler {
	return &Handler{
		driverLocationService: dls,
		log:                   log,
		jwt:                   jwt,
	}
}

// RegisterRoutes mounts REST routes on the given router.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /drivers/{driver_id}/online", h.HandleGoOnline)
	mux.HandleFunc("POST /drivers/{driver_id}/offline", h.HandleGoOffline)
	mux.HandleFunc("POST /drivers/{driver_id}/location", h.HandleUpdateLocation)
	mux.HandleFunc("POST /drivers/{driver_id}/start", h.HandleStartRide)
	mux.HandleFunc("POST /drivers/{driver_id}/complete", h.HandleCompleteRide)
}

type onlinePayload struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Address   string  `json:"address"`
}

type onlineResponse struct {
	Status    string `json:"status"`
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

// HandleGoOnline registers a driver as online and ready to receive rides.
func (h *Handler) HandleGoOnline(w http.ResponseWriter, r *http.Request) {
	driverID, err := driverIDFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid driver path")
		return
	}

	if err := h.authenticateDriver(r, driverID); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var p onlinePayload
	if err := decodeJSON(r, &p); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if !validateCoordinates(p.Latitude, p.Longitude) {
		writeError(w, http.StatusBadRequest, "invalid coordinates")
		return
	}

	sessionID, svcErr := h.driverLocationService.DriverGoOnline(r.Context(), driverID, p.Latitude, p.Longitude, p.Address)
	if svcErr != nil {
		h.log.Error("driver_online_failed", svcErr)
		writeError(w, http.StatusInternalServerError, "failed to bring driver online")
		return
	}

	writeJSON(w, http.StatusOK, onlineResponse{
		Status:    domain.DriverStatusAvailable,
		SessionID: sessionID,
		Message:   "You are now online and ready to accept rides",
	})
}

type offlineSessionSummary struct {
	DurationHours  float64 `json:"duration_hours"`
	RidesCompleted int     `json:"rides_completed"`
	Earnings       float64 `json:"earnings"`
}

type offlineResponse struct {
	Status         string                `json:"status"`
	SessionID      string                `json:"session_id"`
	SessionSummary offlineSessionSummary `json:"session_summary"`
	Message        string                `json:"message"`
}

// HandleGoOffline finalises the driver session and marks the driver unavailable.
func (h *Handler) HandleGoOffline(w http.ResponseWriter, r *http.Request) {
	driverID, err := driverIDFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid driver path")
		return
	}

	if err := h.authenticateDriver(r, driverID); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	session, svcErr := h.driverLocationService.DriverGoOffline(r.Context(), driverID)
	if svcErr != nil {
		h.log.Error("driver_offline_failed", svcErr)
		writeError(w, http.StatusInternalServerError, "failed to go offline")
		return
	}

	var durationHours float64
	if session.EndedAt != nil {
		durationHours = session.EndedAt.Sub(session.StartedAt).Hours()
	} else {
		durationHours = time.Since(session.StartedAt).Hours()
	}

	writeJSON(w, http.StatusOK, offlineResponse{
		Status:    domain.DriverStatusOffline,
		SessionID: session.ID,
		SessionSummary: offlineSessionSummary{
			DurationHours:  durationHours,
			RidesCompleted: session.TotalRides,
			Earnings:       session.TotalEarnings,
		},
		Message: "You are now offline",
	})
}

type updateLocationPayload struct {
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	AccuracyMeters float64 `json:"accuracy_meters"`
	SpeedKmh       float64 `json:"speed_kmh"`
	HeadingDegrees float64 `json:"heading_degrees"`
	Address        string  `json:"address"`
}

type updateLocationResponse struct {
	CoordinateID string `json:"coordinate_id"`
	UpdatedAt    string `json:"updated_at"`
}

// HandleUpdateLocation stores the driver's current location and publishes it downstream.
func (h *Handler) HandleUpdateLocation(w http.ResponseWriter, r *http.Request) {
	driverID, err := driverIDFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid driver path")
		return
	}

	if err := h.authenticateDriver(r, driverID); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var p updateLocationPayload
	if err := decodeJSON(r, &p); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if !validateCoordinates(p.Latitude, p.Longitude) {
		writeError(w, http.StatusBadRequest, "invalid coordinates")
		return
	}

	coordinateID, svcErr := h.driverLocationService.UpdateDriverLocation(
		r.Context(),
		driverID,
		p.Latitude,
		p.Longitude,
		p.AccuracyMeters,
		p.SpeedKmh,
		p.HeadingDegrees,
		p.Address,
	)
	if svcErr != nil {
		h.log.Error("update_location_failed", svcErr)
		writeError(w, http.StatusInternalServerError, "failed to update driver location")
		return
	}

	writeJSON(w, http.StatusOK, updateLocationResponse{
		CoordinateID: coordinateID,
		UpdatedAt:    nowISO(),
	})
}

type startRidePayload struct {
	RideID string `json:"ride_id"`
}

type startRideResponse struct {
	RideID    string `json:"ride_id"`
	Status    string `json:"status"`
	StartedAt string `json:"started_at"`
	Message   string `json:"message"`
}

// HandleStartRide marks the ride as started for a driver.
func (h *Handler) HandleStartRide(w http.ResponseWriter, r *http.Request) {
	driverID, err := driverIDFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid driver path")
		return
	}

	if err := h.authenticateDriver(r, driverID); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var p startRidePayload
	if err := decodeJSON(r, &p); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if p.RideID == "" {
		writeError(w, http.StatusBadRequest, "ride_id is required")
		return
	}

	if svcErr := h.driverLocationService.StartRide(r.Context(), driverID, p.RideID); svcErr != nil {
		h.log.Error("start_ride_failed", svcErr)
		writeError(w, http.StatusInternalServerError, "failed to start ride")
		return
	}

	writeJSON(w, http.StatusOK, startRideResponse{
		RideID:    p.RideID,
		Status:    domain.DriverStatusBusy,
		StartedAt: nowISO(),
		Message:   "Ride started successfully",
	})
}

type completeRidePayload struct {
	RideID                string  `json:"ride_id"`
	ActualDistanceKm      float64 `json:"actual_distance_km"`
	ActualDurationMinutes int     `json:"actual_duration_minutes"`
	FinalLocation         struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"final_location"`
}

type completeRideResponse struct {
	RideID         string  `json:"ride_id"`
	Status         string  `json:"status"`
	CompletedAt    string  `json:"completed_at"`
	DriverEarnings float64 `json:"driver_earnings"`
	Message        string  `json:"message"`
}

// HandleCompleteRide finalises a ride and records metrics.
func (h *Handler) HandleCompleteRide(w http.ResponseWriter, r *http.Request) {
	driverID, err := driverIDFromRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid driver path")
		return
	}

	if err := h.authenticateDriver(r, driverID); err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	var p completeRidePayload
	if err := decodeJSON(r, &p); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if p.RideID == "" {
		writeError(w, http.StatusBadRequest, "ride_id is required")
		return
	}

	if !validateCoordinates(p.FinalLocation.Latitude, p.FinalLocation.Longitude) {
		writeError(w, http.StatusBadRequest, "invalid coordinates")
		return
	}

	earnings, svcErr := h.driverLocationService.CompleteRide(
		r.Context(),
		driverID,
		p.RideID,
		p.ActualDistanceKm,
		p.ActualDurationMinutes,
	)
	if svcErr != nil {
		h.log.Error("complete_ride_failed", svcErr)
		writeError(w, http.StatusInternalServerError, "failed to complete ride")
		return
	}

	writeJSON(w, http.StatusOK, completeRideResponse{
		RideID:         p.RideID,
		Status:         domain.DriverStatusAvailable,
		CompletedAt:    nowISO(),
		DriverEarnings: earnings,
		Message:        "Ride completed successfully",
	})
}

func (h *Handler) authenticateDriver(r *http.Request, driverID string) error {
	token, err := extractBearerToken(r)
	if err != nil {
		return err
	}

	if h.jwt == nil {
		return fmt.Errorf("jwt manager not configured")
	}

	claims, err := h.jwt.ParseToken(token)
	if err != nil {
		return err
	}

	if claims.Role != auth.RoleDriver {
		return fmt.Errorf("token not issued for driver role")
	}

	if claims.UserID != driverID {
		return fmt.Errorf("token does not belong to driver %s", driverID)
	}

	return nil
}

func decodeJSON(r *http.Request, v interface{}) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid json payload: %w", err)
	}
	return nil
}
