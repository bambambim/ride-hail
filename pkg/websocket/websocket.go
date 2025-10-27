package websocket

import (
	"encoding/json"
	"errors"
	"github.com/gorilla/websocket"
	"net/http"
	"ride-hail/pkg/auth"
	"ride-hail/pkg/logger"
	"strings"
	"sync"
	"time"
)

const (
	// Time allowed to write message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Period of sending Ping messages
	pingPeriod = (pongWait * 9) / 10

	// Max message size
	maxMessageSize = 512

	// Time allowed to send auth message
	authTime = 5 * time.Second
)

type wsErrorResponse struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

type authRequest struct {
	Type  string `json:"type"`
	Token string `json:"message"`
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Connection struct {
	conn       *websocket.Conn
	log        logger.Logger
	send       chan []byte
	done       chan []byte
	writeMutex sync.Mutex
	Claims     *auth.AppClaims
}

func newConnection(conn *websocket.Conn, log logger.Logger, claims *auth.AppClaims) *Connection {
	return &Connection{
		conn:       conn,
		log:        log,
		send:       make(chan []byte, 256),
		done:       make(chan []byte, 256),
		writeMutex: sync.Mutex{},
		Claims:     claims,
	}
}

func (c *Connection) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				c.write(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.write(websocket.TextMessage, message); err != nil {
				c.log.Error("websocket_write:", err)
				return
			}

		case <-ticker.C:
			if err := c.write(websocket.PingMessage, []byte{}); err != nil {
				c.log.Error("websocket_ping:", err)
				return
			}
		case <-c.done:
			return
		}
	}

}

func (c *Connection) write(mt int, payload []byte) error {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()
	c.conn.SetWriteDeadline(time.Now().Add(writeWait))
	return c.conn.WriteMessage(mt, payload)
}

func (c *Connection) WriteJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}

	select {
	case c.send <- data:
		return nil
	case <-c.done:
		return errors.New("connection closed")
	default:
		c.log.WithFields(logger.LogFields{"user_id": c.Claims.UserID}).Error("websocket_send_buffer_full", errors.New("dropping message"))
		return errors.New("send buffer full")
	}
}

func (c *Connection) ReadPump(onMessage func(msgType int, p []byte), onDisconnect func()) {
	defer func() {
		onDisconnect()
		c.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		msgType, msg, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				c.log.Error("websocket_read_error", err)
			} else {
				c.log.WithFields(logger.LogFields{"error": err.Error()}).Info("websocket_disconnect", "Client disconnected")
			}
			break

		}

		onMessage(msgType, msg)
	}
}

func (c *Connection) Close() {
	c.writeMutex.Lock()
	defer c.writeMutex.Unlock()

	select {
	case <-c.done:
		return
	default:
		close(c.done)
		close(c.send)
		c.conn.Close()
	}
}

type Handler struct {
	log          logger.Logger
	jwtManager   *auth.JWTManager
	onConnect    func(conn *Connection)
	expectedRole auth.Role
}

func NewHandler(log logger.Logger, jwtManager *auth.JWTManager, onConnect func(conn *Connection), expectedRole auth.Role) *Handler {
	return &Handler{
		log:          log,
		jwtManager:   jwtManager,
		onConnect:    onConnect,
		expectedRole: expectedRole,
	}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Error("websocket_upgrade_failed", err)
		return
	}

	conn.SetReadDeadline(time.Now().Add(authTime))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		h.log.Error("websocket_auth_timeout", err)
		sendErrorAndClose(conn, "Authentication timeout")
		return
	}

	var req authRequest
	if err := json.Unmarshal(msg, &req); err != nil {
		h.log.Error("websocket_auth_format_error", err)
		sendErrorAndClose(conn, "Invalid authentication request format")
		return
	}
	if req.Type != "auth" || req.Token == "" {
		h.log.Error("websocket_auth_format_error", errors.New("invalid auth message format"))
		sendErrorAndClose(conn, "Invalid authentication request format")
		return
	}

	tokenString := strings.TrimPrefix(req.Token, "Bearer ")
	claims, err := h.jwtManager.ParseToken(tokenString)
	if err != nil {
		h.log.Error("websocket_auth_token_invalid", err)
		sendErrorAndClose(conn, "Invalid or expired token")
		return
	}

	if claims.Role != h.expectedRole {
		h.log.WithFields(logger.LogFields{
			"user_id":  claims.UserID,
			"got_role": claims.Role,
			"expected": h.expectedRole,
		}).Error("websocket_auth_role_mismatch", errors.New("invalid role"))
		sendErrorAndClose(conn, "Invalid or expired token")
		return
	}

	h.log.WithFields(logger.LogFields{"user_id": claims.UserID}).Info("websocket_auth_success", "Client authenticated")
	wsConn := newConnection(conn, h.log, claims)
	go wsConn.writePump()
	go h.onConnect(wsConn)

}
func sendErrorAndClose(conn *websocket.Conn, msg string) {
	conn.SetWriteDeadline(time.Now().Add(writeWait))
	conn.WriteJSON(wsErrorResponse{
		Type:    "error",
		Message: msg,
	})
	conn.Close()
}
