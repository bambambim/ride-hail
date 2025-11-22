package rest

import (
	"encoding/json"
	"net/http"
	"ride-hail/services/driver_location_service/internal/domain"
	"strings"
)

// Handler holds dependencies for REST endpoints.
type Handler struct {
	driverLocationService domain.DriverLocationService
}

// NewHandler creates a new REST handler bound to the given port implementation.
func NewHandler(dls domain.DriverLocationService) *Handler {
	return &Handler{driverLocationService: dls}
}

// RegisterRoutes mounts REST routes on the given router.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /drivers/{driver_id}/online", h.HandleGoOnline)
	// mux.HandleFunc("/drivers/offline", h.HandleOfflineDriver)
	// mux.HandleFunc("/drivers/update_location", h.HandleUpdateDriverLocation)
	// mux.HandleFunc("/drivers/start_ride", h.HandleStartRide)
	// mux.HandleFunc("/drivers/end_ride", h.HandleEndRide)
	// mux.HandleFunc("/drivers/ws_connect", h.HandleWebsocketConnect)
}

type onlinePayload struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type onlineResponse struct {
	Status    string `json:"status"`
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

// Rewrite using actual service and using helper functions
func (h *Handler) HandleGoOnline(w http.ResponseWriter, r *http.Request) {
	// Parse URL: /drivers/{id}/online
	// parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	// driverID := parts[1]

	// Parse body
	var p onlinePayload
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&p); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Basic coordinate validation
	if p.Latitude < -90 || p.Latitude > 90 || p.Longitude < -180 || p.Longitude > 180 {
		http.Error(w, "invalid coordinates", http.StatusBadRequest)
		return
	}

	// Extract token: Authorization: Bearer <token>
	authHeader := r.Header.Get("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		http.Error(w, "missing or invalid Authorization header", http.StatusUnauthorized)
		return
	}
	// token := strings.TrimPrefix(authHeader, "Bearer ")

	// Call domain service
	// sessionID, err := h.driverLocationService.DriverOnline(
	// 	r.Context(),
	// 	driverID,
	// 	token,
	// 	p.Latitude,
	// 	p.Longitude,
	// )
	// if err != nil {
	// 	http.Error(w, err.Error(), http.StatusBadRequest)
	// 	return
	// }

	resp := onlineResponse{
		Status:    "AVAILABLE",
		// SessionID: sessionID,
		Message:   "You are now online and ready to accept rides",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type offlineSessionSummary struct {
	DurationHours  float64 `json:"duration_hours"`
	RidesCompleted int     `json:"rides_completed"`
	Earnings       float64 `json:"earnings"`
}

type offlineResponse struct {
	Status         string                 `json:"status"`
	SessionID      string                 `json:"session_id"`
	SessionSummary offlineSessionSummary  `json:"session_summary"`
	Message        string                 `json:"message"`
}

func (h *Handler) HandleGoOffline(w http.ResponseWriter, r *http.Request) {
	// parse driver id
	if _, err := parseDriverID(r); err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	// ensure token present
	if _, err := extractBearerToken(r); err != nil {
		http.Error(w, "missing or invalid Authorization header", http.StatusUnauthorized)
		return
	}

	// In a real implementation we'd call domain service to end the session and compute summary.
	resp := offlineResponse{
		Status:    "OFFLINE",
		SessionID: "660e8400-e29b-41d4-a716-446655440001",
		SessionSummary: offlineSessionSummary{
			DurationHours:  5.5,
			RidesCompleted: 12,
			Earnings:       18500.0,
		},
		Message: "You are now offline",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type updateLocationPayload struct {
	Latitude       float64 `json:"latitude"`
	Longitude      float64 `json:"longitude"`
	AccuracyMeters float64 `json:"accuracy_meters"`
	SpeedKmh       float64 `json:"speed_kmh"`
	HeadingDegrees float64 `json:"heading_degrees"`
}

type updateLocationResponse struct {
	CoordinateID string `json:"coordinate_id"`
	UpdatedAt    string `json:"updated_at"`
}

func (h *Handler) HandleUpdateLocation(w http.ResponseWriter, r *http.Request) {
	if _, err := parseDriverID(r); err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	if _, err := extractBearerToken(r); err != nil {
		http.Error(w, "missing or invalid Authorization header", http.StatusUnauthorized)
		return
	}

	var p updateLocationPayload
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Basic coordinate validation
	if p.Latitude < -90 || p.Latitude > 90 || p.Longitude < -180 || p.Longitude > 180 {
		http.Error(w, "invalid coordinates", http.StatusBadRequest)
		return
	}

	// Would normally persist coordinate and return created ID and timestamp.
	resp := updateLocationResponse{
		CoordinateID: "770e8400-e29b-41d4-a716-446655440002",
		UpdatedAt:    "2024-12-16T10:30:00Z",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type startRidePayload struct {
	RideID         string `json:"ride_id"`
	DriverLocation struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"driver_location"`
}

type startRideResponse struct {
	RideID   string `json:"ride_id"`
	Status   string `json:"status"`
	StartedAt string `json:"started_at"`
	Message  string `json:"message"`
}

func (h *Handler) HandleStartRide(w http.ResponseWriter, r *http.Request) {
	if _, err := parseDriverID(r); err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	if _, err := extractBearerToken(r); err != nil {
		http.Error(w, "missing or invalid Authorization header", http.StatusUnauthorized)
		return
	}

	var p startRidePayload
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate coordinates
	lat := p.DriverLocation.Latitude
	lng := p.DriverLocation.Longitude
	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		http.Error(w, "invalid coordinates", http.StatusBadRequest)
		return
	}

	resp := startRideResponse{
		RideID:    p.RideID,
		Status:    "BUSY",
		StartedAt: "2024-12-16T10:35:00Z",
		Message:   "Ride started successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

type completeRidePayload struct {
	RideID               string `json:"ride_id"`
	FinalLocation        struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"final_location"`
	ActualDistanceKm     float64 `json:"actual_distance_km"`
	ActualDurationMinutes int    `json:"actual_duration_minutes"`
}

type completeRideResponse struct {
	RideID         string  `json:"ride_id"`
	Status         string  `json:"status"`
	CompletedAt    string  `json:"completed_at"`
	DriverEarnings float64 `json:"driver_earnings"`
	Message        string  `json:"message"`
}

func (h *Handler) HandleCompleteRide(w http.ResponseWriter, r *http.Request) {
	if _, err := parseDriverID(r); err != nil {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}

	if _, err := extractBearerToken(r); err != nil {
		http.Error(w, "missing or invalid Authorization header", http.StatusUnauthorized)
		return
	}

	var p completeRidePayload
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		http.Error(w, "invalid json: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Validate final location
	lat := p.FinalLocation.Latitude
	lng := p.FinalLocation.Longitude
	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		http.Error(w, "invalid coordinates", http.StatusBadRequest)
		return
	}

	// Would normally compute earnings; here we return a sample value.
	resp := completeRideResponse{
		RideID:         p.RideID,
		Status:         "AVAILABLE",
		CompletedAt:    "2024-12-16T10:51:00Z",
		DriverEarnings: 1216.0,
		Message:        "Ride completed successfully",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}