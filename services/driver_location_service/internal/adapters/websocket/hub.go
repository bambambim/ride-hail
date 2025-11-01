package websocket

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"ride-hail/pkg/logger"
	"ride-hail/services/driver_location_service/internal/domain"
	"ride-hail/services/driver_location_service/internal/ports"

	"github.com/gorilla/websocket"
)

// Hub manages WebSocket connections for drivers
type Hub struct {
	// Registered drivers
	drivers map[string]*Client

	// Register requests from drivers
	register chan *Client

	// Unregister requests from drivers
	unregister chan *Client

	// Broadcast messages to all drivers
	broadcast chan *BroadcastMessage

	// Send message to specific driver
	sendToDriver chan *DriverMessage

	// Mutex for thread-safe access to drivers map
	mu sync.RWMutex

	// Logger
	logger logger.Logger
}

// Client represents a WebSocket client (driver)
type Client struct {
	// Driver ID
	driverID string

	// WebSocket connection
	conn *websocket.Conn

	// Buffered channel of outbound messages
	send chan []byte

	// Hub reference
	hub *Hub

	// Last ping time
	lastPing time.Time

	// Mutex for write operations
	writeMu sync.Mutex
}

// BroadcastMessage represents a message to broadcast to all drivers
type BroadcastMessage struct {
	Message interface{}
}

// DriverMessage represents a message to send to a specific driver
type DriverMessage struct {
	DriverID string
	Message  interface{}
}

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512 * 1024 // 512 KB

	// Send buffer size
	sendBufferSize = 256
)

// NewHub creates a new WebSocket hub
func NewHub(log logger.Logger) ports.WebSocketHub {
	hub := &Hub{
		drivers:      make(map[string]*Client),
		register:     make(chan *Client),
		unregister:   make(chan *Client),
		broadcast:    make(chan *BroadcastMessage),
		sendToDriver: make(chan *DriverMessage),
		logger:       log,
	}

	go hub.run()

	return hub
}

// run handles registration, unregistration, and message routing
func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.drivers[client.driverID] = client
			h.mu.Unlock()
			h.logger.Info("websocket.register", fmt.Sprintf("Driver %s connected", client.driverID))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.drivers[client.driverID]; ok {
				delete(h.drivers, client.driverID)
				close(client.send)
			}
			h.mu.Unlock()
			h.logger.Info("websocket.unregister", fmt.Sprintf("Driver %s disconnected", client.driverID))

		case msg := <-h.broadcast:
			h.mu.RLock()
			for _, client := range h.drivers {
				select {
				case client.send <- h.marshalMessage(msg.Message):
				default:
					// Client's send buffer is full, skip
					h.logger.Debug("websocket.broadcast", fmt.Sprintf("Send buffer full for driver %s", client.driverID))
				}
			}
			h.mu.RUnlock()

		case msg := <-h.sendToDriver:
			h.mu.RLock()
			if client, ok := h.drivers[msg.DriverID]; ok {
				select {
				case client.send <- h.marshalMessage(msg.Message):
					h.logger.Debug("websocket.send", fmt.Sprintf("Sent message to driver %s", msg.DriverID))
				default:
					h.logger.Debug("websocket.send", fmt.Sprintf("Send buffer full for driver %s", msg.DriverID))
				}
			} else {
				h.logger.Debug("websocket.send", fmt.Sprintf("Driver %s not connected", msg.DriverID))
			}
			h.mu.RUnlock()
		}
	}
}

// SendRideOffer sends a ride offer to a specific driver
func (h *Hub) SendRideOffer(driverID string, offer *domain.RideOffer) error {
	h.sendToDriver <- &DriverMessage{
		DriverID: driverID,
		Message:  offer,
	}
	return nil
}

// SendRideDetails sends ride details after acceptance
func (h *Hub) SendRideDetails(driverID string, details *domain.RideDetails) error {
	h.sendToDriver <- &DriverMessage{
		DriverID: driverID,
		Message:  details,
	}
	return nil
}

// BroadcastToDriver sends a generic message to a driver
func (h *Hub) BroadcastToDriver(driverID string, message interface{}) error {
	h.sendToDriver <- &DriverMessage{
		DriverID: driverID,
		Message:  message,
	}
	return nil
}

// RegisterDriver registers a driver's WebSocket connection
func (h *Hub) RegisterDriver(driverID string, conn interface{}) error {
	wsConn, ok := conn.(*websocket.Conn)
	if !ok {
		return fmt.Errorf("invalid connection type")
	}

	client := &Client{
		driverID: driverID,
		conn:     wsConn,
		send:     make(chan []byte, sendBufferSize),
		hub:      h,
		lastPing: time.Now(),
	}

	h.register <- client

	// Start read and write pumps
	go client.readPump()
	go client.writePump()

	return nil
}

// UnregisterDriver removes a driver's WebSocket connection
func (h *Hub) UnregisterDriver(driverID string) error {
	h.mu.RLock()
	client, ok := h.drivers[driverID]
	h.mu.RUnlock()

	if ok {
		h.unregister <- client
	}

	return nil
}

// IsDriverConnected checks if a driver is connected via WebSocket
func (h *Hub) IsDriverConnected(driverID string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	_, ok := h.drivers[driverID]
	return ok
}

// GetConnectedDrivers returns the number of connected drivers
func (h *Hub) GetConnectedDrivers() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.drivers)
}

// marshalMessage marshals a message to JSON
func (h *Hub) marshalMessage(msg interface{}) []byte {
	data, err := json.Marshal(msg)
	if err != nil {
		h.logger.Error("websocket.marshal", err)
		return []byte(`{"error":"Failed to marshal message"}`)
	}
	return data
}

// readPump pumps messages from the WebSocket connection to the hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		c.lastPing = time.Now()
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.hub.logger.Error("websocket.read", err)
			}
			break
		}

		// Handle incoming messages from driver
		c.handleMessage(message)
	}
}

// writePump pumps messages from the hub to the WebSocket connection
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to the current websocket message
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// handleMessage processes incoming messages from the driver
func (c *Client) handleMessage(message []byte) {
	var wsMsg domain.WebSocketMessage
	if err := json.Unmarshal(message, &wsMsg); err != nil {
		c.hub.logger.Error("websocket.handle_message.unmarshal", err)
		return
	}

	switch wsMsg.Type {
	case "auth":
		// Handle authentication
		c.hub.logger.Info("websocket.auth", fmt.Sprintf("Driver %s authenticated", c.driverID))

	case "ride_response":
		// Handle ride response
		var response domain.RideResponse
		payload, _ := json.Marshal(wsMsg.Payload)
		if err := json.Unmarshal(payload, &response); err != nil {
			c.hub.logger.Error("websocket.ride_response.unmarshal", err)
			return
		}
		c.hub.logger.Info("websocket.ride_response", fmt.Sprintf("Driver %s responded to ride %s: accepted=%v", c.driverID, response.RideID, response.Accepted))
		// TODO: Process ride response through service layer

	case "location_update":
		// Handle location update
		var location domain.UpdateLocationRequest
		payload, _ := json.Marshal(wsMsg.Payload)
		if err := json.Unmarshal(payload, &location); err != nil {
			c.hub.logger.Error("websocket.location_update.unmarshal", err)
			return
		}
		c.hub.logger.Debug("websocket.location_update", fmt.Sprintf("Received location update from driver %s", c.driverID))
		// TODO: Process location update through service layer

	case "ping":
		// Handle ping
		c.lastPing = time.Now()
		c.sendMessage(map[string]string{"type": "pong"})

	default:
		c.hub.logger.Debug("websocket.unknown_message", fmt.Sprintf("Unknown message type: %s", wsMsg.Type))
	}
}

// sendMessage sends a message to the driver
func (c *Client) sendMessage(msg interface{}) {
	data := c.hub.marshalMessage(msg)
	select {
	case c.send <- data:
	default:
		c.hub.logger.Debug("websocket.send_message", "Send buffer full")
	}
}

// Close closes the client connection
func (c *Client) Close() error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	return c.conn.Close()
}
