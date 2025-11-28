package ws

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"ride-hail/pkg/auth"
	"ride-hail/pkg/logger"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // In production, implement proper origin checking
	},
}

// Manager manages WebSocket connections for drivers
type Manager struct {
	clients   map[string]*Client // driverID -> Client
	mu        sync.RWMutex
	hub       *Hub
	jwtMgr    *auth.JWTManager
	log       logger.Logger
	onMessage func(driverID string, msgType string, data interface{}) // Callback for incoming messages
}

// NewManager creates a new WebSocket manager
func NewManager(jwtMgr *auth.JWTManager, log logger.Logger) *Manager {
	hub := NewHub()
	mgr := &Manager{
		clients: make(map[string]*Client),
		hub:     hub,
		jwtMgr:  jwtMgr,
		log:     log,
	}

	hub.SetRideResponseHandler(func(driverID string, msg *Message) {
		if mgr.onMessage != nil {
			mgr.onMessage(driverID, MsgRideResponse, msg.Data)
		}
	})

	go hub.Run()

	return mgr
}

// SetMessageHandler sets the callback for incoming messages
func (m *Manager) SetMessageHandler(handler func(driverID string, msgType string, data interface{})) {
	m.onMessage = handler
}

// HandleWebSocket upgrades HTTP connection to WebSocket for a driver
func (m *Manager) HandleWebSocket(w http.ResponseWriter, r *http.Request, driverID string) error {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		m.log.Error("websocket_upgrade_failed", err)
		return fmt.Errorf("failed to upgrade connection: %w", err)
	}

	client := newClient(conn, m.hub)
	client.serviceID = driverID

	m.mu.Lock()
	// Close existing connection if any
	if existingClient, ok := m.clients[driverID]; ok {
		existingClient.conn.Close()
	}
	m.clients[driverID] = client
	m.mu.Unlock()

	m.hub.register <- client

	// Start read and write loops
	go client.readLoop()
	go client.writeLoop()

	m.log.Info("driver_connected", fmt.Sprintf("Driver %s connected via WebSocket", driverID))

	return nil
}

// SendRideOffer sends a ride offer to a specific driver
func (m *Manager) SendRideOffer(driverID string, offer interface{}) error {
	m.mu.RLock()
	client, ok := m.clients[driverID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("driver %s not connected", driverID)
	}

	if !client.authenticated {
		return fmt.Errorf("driver %s not authenticated", driverID)
	}

	msg := Message{
		Type: MsgRideOffer,
		Data: offer,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal ride offer: %w", err)
	}

	select {
	case client.send <- data:
		m.log.Info("ride_offer_sent", fmt.Sprintf("Sent ride offer to driver %s", driverID))
		return nil
	default:
		return fmt.Errorf("driver %s send buffer full", driverID)
	}
}

// SendRideDetails sends ride details after acceptance
func (m *Manager) SendRideDetails(driverID string, details interface{}) error {
	m.mu.RLock()
	client, ok := m.clients[driverID]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("driver %s not connected", driverID)
	}

	if !client.authenticated {
		return fmt.Errorf("driver %s not authenticated", driverID)
	}

	msg := Message{
		Type: MsgRideDetails,
		Data: details,
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal ride details: %w", err)
	}

	select {
	case client.send <- data:
		m.log.Info("ride_details_sent", fmt.Sprintf("Sent ride details to driver %s", driverID))
		return nil
	default:
		return fmt.Errorf("driver %s send buffer full", driverID)
	}
}

// BroadcastToAll sends a message to all connected and authenticated drivers
func (m *Manager) BroadcastToAll(message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal broadcast message: %w", err)
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, client := range m.clients {
		if client.authenticated {
			select {
			case client.send <- data:
			default:
				m.log.Error("broadcast_failed", fmt.Errorf("failed to send to driver %s", client.serviceID))
			}
		}
	}

	return nil
}

// IsDriverConnected checks if a driver is connected and authenticated
func (m *Manager) IsDriverConnected(driverID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, ok := m.clients[driverID]
	if !ok {
		return false
	}

	return client.authenticated
}

// DisconnectDriver forcefully disconnects a driver
func (m *Manager) DisconnectDriver(driverID string) {
	m.mu.Lock()
	client, ok := m.clients[driverID]
	if ok {
		delete(m.clients, driverID)
	}
	m.mu.Unlock()

	if ok {
		m.hub.unregister <- client
		client.conn.Close()
		m.log.Info("driver_disconnected", fmt.Sprintf("Driver %s disconnected", driverID))
	}
}

// GetConnectedDrivers returns list of all connected driver IDs
func (m *Manager) GetConnectedDrivers() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	drivers := make([]string, 0, len(m.clients))
	for driverID, client := range m.clients {
		if client.authenticated {
			drivers = append(drivers, driverID)
		}
	}

	return drivers
}

// CloseAll closes all connections gracefully
func (m *Manager) CloseAll() {
	m.mu.Lock()
	clients := m.clients
	m.clients = make(map[string]*Client)
	m.mu.Unlock()

	for _, client := range clients {
		client.conn.Close()
	}

	m.log.Info("websocket_manager_closed", "All driver connections closed")
}

// AuthenticateConnection validates JWT token for WebSocket connection
func (m *Manager) AuthenticateConnection(token string) (string, error) {
	// Remove "Bearer " prefix if present
	token = strings.TrimPrefix(token, "Bearer ")

	claims, err := m.jwtMgr.ParseToken(token)
	if err != nil {
		return "", fmt.Errorf("invalid token: %w", err)
	}

	if claims.Role != auth.RoleDriver {
		return "", fmt.Errorf("invalid role: expected DRIVER, got %s", claims.Role)
	}

	return claims.UserID, nil
}

// GetConnectionCount returns the number of connected drivers
func (m *Manager) GetConnectionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.clients)
}
