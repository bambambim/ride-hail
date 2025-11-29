package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"

	"ride-hail/internal/driver_location_service/domain"
	"ride-hail/pkg/auth"
	"ride-hail/pkg/logger"
	pkgws "ride-hail/pkg/websocket"
)

type IncomingMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

type DriverWSAdapter struct {
	manager  *pkgws.Manager
	log      logger.Logger
	jwtMgr   *auth.JWTManager
	service  domain.DriverLocationService
	handlers map[string]func(driverID string, data json.RawMessage)
}

func NewDriverWSAdapter(log logger.Logger, jwtMgr *auth.JWTManager) *DriverWSAdapter {
	return &DriverWSAdapter{
		manager:  pkgws.NewManager(log),
		log:      log,
		jwtMgr:   jwtMgr,
		handlers: make(map[string]func(string, json.RawMessage)),
	}
}

func (a *DriverWSAdapter) SetService(service domain.DriverLocationService) {
	a.service = service
	a.registerDomainHandlers()
}

func (a *DriverWSAdapter) registerDomainHandlers() {
	a.handlers["ride_response"] = a.handleRideResponse
	a.handlers["location_update"] = a.handleLocationUpdate
}

func (a *DriverWSAdapter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Verify route pattern /ws/drivers/{driverID}
	// The ID in the URL is primarily for routing compliance;
	// actual identity is extracted securely from the JWT token.
	if strings.TrimPrefix(r.URL.Path, "/ws/drivers/") == "" {
		http.Error(w, "Driver ID required in URL path", http.StatusBadRequest)
		return
	}

	handler := pkgws.NewHandler(a.log, a.jwtMgr, a.onConnect, auth.RoleDriver)
	handler.ServeHTTP(w, r)
}

func (a *DriverWSAdapter) onConnect(conn *pkgws.Connection) {
	driverID := conn.Claims.UserID
	a.manager.AddConnection(driverID, conn)
	a.log.WithFields(logger.LogFields{"driver_id": driverID}).Info("ws_connect", "Driver connected")

	go conn.ReadPump(func(msgType int, payload []byte) {
		if msgType != websocket.TextMessage {
			return
		}
		a.handleMessage(driverID, payload)
	}, func() {
		a.manager.RemoveConnection(driverID)
		a.log.WithFields(logger.LogFields{"driver_id": driverID}).Info("ws_disconnect", "Driver disconnected")
	})
}

func (a *DriverWSAdapter) handleMessage(driverID string, payload []byte) {
	var msg IncomingMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		a.log.Error("ws_json_error", err)
		return
	}

	if handler, exists := a.handlers[msg.Type]; exists {
		handler(driverID, msg.Data)
	} else {
		a.log.WithFields(logger.LogFields{"type": msg.Type}).Debug("ws_unknown_message", "Received unknown message type")
	}
}

// --- Handlers ---

func (a *DriverWSAdapter) handleRideResponse(driverID string, data json.RawMessage) {
	var req struct {
		OfferID  string `json:"offer_id"`
		RideID   string `json:"ride_id"`
		Accepted bool   `json:"accepted"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		a.log.Error("ws_handler_error", fmt.Errorf("invalid ride_response format: %w", err))
		return
	}

	ctx := context.Background()
	if err := a.service.HandleDriverRideResponse(ctx, driverID, req.OfferID, req.RideID, req.Accepted); err != nil {
		a.log.Error("ws_handler_failed", err)
	}
}

func (a *DriverWSAdapter) handleLocationUpdate(driverID string, data json.RawMessage) {
	var req struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Accuracy  float64 `json:"accuracy_meters"`
		Speed     float64 `json:"speed_kmh"`
		Heading   float64 `json:"heading_degrees"`
	}
	if err := json.Unmarshal(data, &req); err != nil {
		a.log.Error("ws_handler_error", fmt.Errorf("invalid location_update format: %w", err))
		return
	}

	ctx := context.Background()
	// Address is typically not sent in high-frequency updates, defaulting to empty or unknown
	_, err := a.service.UpdateDriverLocation(
		ctx,
		driverID,
		req.Latitude,
		req.Longitude,
		req.Accuracy,
		req.Speed,
		req.Heading,
		"Unknown",
	)
	if err != nil {
		a.log.Debug("ws_location_update_failed", err.Error())
	}
}

// --- WebSocketManager Interface ---

func (a *DriverWSAdapter) SendRideOffer(driverID string, offer interface{}) error {
	msg := map[string]interface{}{
		"type": "ride_offer",
		"data": offer,
	}
	return a.manager.SendToUser(driverID, msg)
}

func (a *DriverWSAdapter) SendRideDetails(driverID string, details interface{}) error {
	msg := map[string]interface{}{
		"type": "ride_details",
		"data": details,
	}
	return a.manager.SendToUser(driverID, msg)
}

func (a *DriverWSAdapter) BroadcastToAll(message interface{}) error {
	a.manager.Broadcast(message)
	return nil
}

func (a *DriverWSAdapter) IsDriverConnected(driverID string) bool {
	return a.manager.IsUserConnected(driverID)
}
