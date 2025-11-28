package ws

import (
	"encoding/json"
)

type Hub struct {
	clients             map[*Client]bool
	register            chan *Client
	unregister          chan *Client
	rideResponseHandler func(driverID string, msg *Message)
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) Run() {
	for {
		select {

		case c := <-h.register:
			h.clients[c] = true

		case c := <-h.unregister:
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
		}
	}
}

// =============== OUTGOING EVENTS ===================

// Publish ride offer to ride service
func (h *Hub) SendRideOffer(data interface{}) {
	h.sendToAll(Message{
		Type: MsgRideOffer,
		Data: data,
	})
}

func (h *Hub) SendRideDetails(data interface{}) {
	h.sendToAll(Message{
		Type: MsgRideDetails,
		Data: data,
	})
}

func (h *Hub) sendToAll(m Message) {
	b, _ := json.Marshal(m)
	for c := range h.clients {
		if c.authenticated {
			c.send <- b
		}
	}
}

// =============== INCOMING EVENTS ===================

func (h *Hub) handleRideResponse(driverID string, m *Message) {
	if h.rideResponseHandler != nil {
		h.rideResponseHandler(driverID, m)
	}
}

func (h *Hub) SetRideResponseHandler(handler func(driverID string, msg *Message)) {
	h.rideResponseHandler = handler
}
