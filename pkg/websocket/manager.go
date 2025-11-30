package websocket

import (
	"sync"

	"ride-hail/pkg/logger"
)

// Manager manages WebSocket connections for passengers and drivers
type Manager struct {
	connections map[string]*Connection // user_id -> connection
	mu          sync.RWMutex
	log         logger.Logger
}

// NewManager creates a new WebSocket manager
func NewManager(log logger.Logger) *Manager {
	return &Manager{
		connections: make(map[string]*Connection),
		log:         log,
	}
}

// AddConnection registers a new connection
func (m *Manager) AddConnection(userID string, conn *Connection) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Close existing connection if any
	if existing, ok := m.connections[userID]; ok {
		existing.Close()
		m.log.WithFields(logger.LogFields{
			"user_id": userID,
		}).Info("websocket_replaced", "Replacing existing connection")
	}

	m.connections[userID] = conn
	m.log.WithFields(logger.LogFields{
		"user_id": userID,
		"total":   len(m.connections),
	}).Info("websocket_connected", "New connection added")
}

// RemoveConnection removes a connection
func (m *Manager) RemoveConnection(userID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if conn, ok := m.connections[userID]; ok {
		conn.Close()
		delete(m.connections, userID)
		m.log.WithFields(logger.LogFields{
			"user_id": userID,
			"total":   len(m.connections),
		}).Info("websocket_disconnected", "Connection removed")
	}
}

// SendToUser sends a message to a specific user
func (m *Manager) SendToUser(userID string, message interface{}) error {
	m.mu.RLock()
	conn, ok := m.connections[userID]
	m.mu.RUnlock()

	if !ok {
		m.log.WithFields(logger.LogFields{
			"user_id": userID,
		}).Debug("websocket_user_not_connected", "User not connected")
		return nil // Not an error - user just isn't connected
	}

	if err := conn.WriteJSON(message); err != nil {
		m.log.WithFields(logger.LogFields{
			"user_id": userID,
			"error":   err.Error(),
		}).Error("websocket_send_failed", err)
		// Remove dead connection
		m.RemoveConnection(userID)
		return err
	}

	return nil
}

// Broadcast sends a message to all connected users
func (m *Manager) Broadcast(message interface{}) {
	m.mu.RLock()
	connections := make([]*Connection, 0, len(m.connections))
	for _, conn := range m.connections {
		connections = append(connections, conn)
	}
	m.mu.RUnlock()

	for _, conn := range connections {
		if err := conn.WriteJSON(message); err != nil {
			m.log.Error("websocket_broadcast_failed", err)
		}
	}
}

// GetConnectionCount returns the number of active connections
func (m *Manager) GetConnectionCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.connections)
}

// IsUserConnected checks if a user is connected
func (m *Manager) IsUserConnected(userID string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.connections[userID]
	return ok
}
