package ws

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gorilla/websocket"
)

const (
	authTimeout  = 5 * time.Second
	pingInterval = 30 * time.Second
	pongWait     = 60 * time.Second
)

type Client struct {
	conn          *websocket.Conn
	send          chan []byte
	hub           *Hub
	authenticated bool
	serviceID     string
}

func newClient(conn *websocket.Conn, hub *Hub) *Client {
	return &Client{
		conn: conn,
		send: make(chan []byte, 256),
		hub:  hub,
	}
}

func (c *Client) readLoop() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(10_000)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(appData string) error {
		// Extend deadline when pong received
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	authTimer := time.NewTimer(authTimeout)

	for {
		select {
		case <-authTimer.C:
			if !c.authenticated {
				fmt.Println("auth timeout — closing")
				return
			}
		default:
		}

		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		var m Message
		if err := json.Unmarshal(msg, &m); err != nil {
			fmt.Println("invalid message:", err)
			continue
		}

		// AUTH HANDSHAKE
		if m.Type == MsgTypeAuth && !c.authenticated {
			serviceID, err := ValidateToken(m.Token)
			if err != nil {
				fmt.Println("auth failed:", err)
				return
			}
			c.authenticated = true
			c.serviceID = serviceID
			fmt.Println("client authenticated:", serviceID)
			continue
		}

		// Reject other messages until authenticated
		if !c.authenticated {
			fmt.Println("message before auth ignored")
			continue
		}

		// DATA MESSAGES
		switch m.Type {
		case MsgRideResponse:
			c.hub.handleRideResponse(c.serviceID, &m)
		default:
			fmt.Println("unknown message:", m.Type)
		}
	}
}

func (c *Client) writeLoop() {
	ticker := time.NewTicker(pingInterval)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			// Outgoing data
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, nil)
				return
			}
			c.conn.WriteMessage(websocket.TextMessage, msg)

		case <-ticker.C:
			// Server ping
			c.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
