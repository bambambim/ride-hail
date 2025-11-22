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

func (h *Handler) HandleGoOnline(w http.ResponseWriter, r *http.Request) {
	// Parse URL: /drivers/{id}/online
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
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
