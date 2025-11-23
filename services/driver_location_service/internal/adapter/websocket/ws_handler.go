package ws

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Conn is a minimal websocket connection interface the adapter works with.
// It intentionally mirrors the common methods used by gorilla/websocket and
// other websocket implementations so the concrete conn can satisfy this interface.
type Conn interface {
	ReadMessage() (messageType int, p []byte, err error)
	WriteMessage(messageType int, data []byte) error
	Close() error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
}

// Handler is a more method-specific websocket adapter that maintains a set
// of connected clients and provides high-level operations used by application ports.
type Handler struct {
	clients      map[string]*client
	mu           sync.RWMutex
	readTimeout  time.Duration
	writeTimeout time.Duration
	// buffer sizes and channel capacities can be tuned
	sendBuf int
}

type client struct {
	id   string
	conn Conn
	send chan []byte

	closed   chan struct{}
	closeOnce sync.Once
}

var (
	ErrClientExists    = errors.New("client already registered")
	ErrClientNotFound  = errors.New("client not found")
	ErrHandlerShutdown = errors.New("handler is shutdown")
)

// NewHandler creates a new websocket Handler.
func NewHandler(opts ...Option) *Handler {
	h := &Handler{
		clients:      make(map[string]*client),
		readTimeout:  60 * time.Second,
		writeTimeout: 10 * time.Second,
		sendBuf:      32,
	}
	for _, o := range opts {
		o(h)
	}
	return h
}

// Option configures the Handler.
type Option func(*Handler)

// WithReadTimeout sets the read deadline for connections.
func WithReadTimeout(d time.Duration) Option {
	return func(h *Handler) { h.readTimeout = d }
}

// WithWriteTimeout sets the write deadline for connections.
func WithWriteTimeout(d time.Duration) Option {
	return func(h *Handler) { h.writeTimeout = d }
}

// WithSendBuffer sets the per-client send channel buffer size.
func WithSendBuffer(n int) Option {
	return func(h *Handler) { if n > 0 { h.sendBuf = n } }
}

// Register registers a new client and starts read/write pumps.
// Returns ErrClientExists when a client with the same id already exists.
func (h *Handler) Register(ctx context.Context, id string, conn Conn) error {
	h.mu.Lock()
	if _, ok := h.clients[id]; ok {
		h.mu.Unlock()
		return ErrClientExists
	}
	c := &client{
		id:    id,
		conn:  conn,
		send:  make(chan []byte, h.sendBuf),
		closed: make(chan struct{}),
	}
	h.clients[id] = c
	h.mu.Unlock()

	go h.readPump(c)
	go h.writePump(c)

	return nil
}

// Unregister removes a client and closes the underlying connection.
func (h *Handler) Unregister(ctx context.Context, id string) error {
	h.mu.Lock()
	c, ok := h.clients[id]
	if !ok {
		h.mu.Unlock()
		return ErrClientNotFound
	}
	delete(h.clients, id)
	h.mu.Unlock()

	c.close()
	return nil
}

// Broadcast sends msg to all connected clients. Non-blocking per client; if a
// client's send buffer is full the message will be dropped for that client.
func (h *Handler) Broadcast(ctx context.Context, msg []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, c := range h.clients {
		select {
		case c.send <- copyBytes(msg):
		default:
			// drop for slow client
		}
	}
}

// SendToClient sends msg to a specific client. Returns ErrClientNotFound if client is not present.
func (h *Handler) SendToClient(ctx context.Context, id string, msg []byte) error {
	h.mu.RLock()
	c, ok := h.clients[id]
	h.mu.RUnlock()
	if !ok {
		return ErrClientNotFound
	}

	select {
	case c.send <- copyBytes(msg):
		return nil
	default:
		// if buffer full, treat as transient error by closing the client
		c.close()
		return errors.New("client send buffer full; connection closed")
	}
}

// CloseAll gracefully closes all clients.
func (h *Handler) CloseAll(ctx context.Context) {
	h.mu.Lock()
	clients := h.clients
	h.clients = make(map[string]*client)
	h.mu.Unlock()

	for _, c := range clients {
		c.close()
	}
}

// readPump continuously reads messages from the connection and dispatches them to a handler.
// For this generic adapter we expose an incoming channel hook via context. If the caller
// wants to receive messages, they can pass a value with key incomingKey and a chan<- []byte.
func (h *Handler) readPump(c *client) {
	defer c.close()

	incoming := incomingChanFromContext(context.Background()) // default nil
	// Note: If you want to receive incoming messages in your application, pass a context
	// with incoming channel when calling Register (not shown here). This keeps the adapter generic.

	for {
		_ = c.conn.SetReadDeadline(time.Now().Add(h.readTimeout))
		typ, msg, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		// only forward text or binary payloads, otherwise ignore
		if typ == 1 || typ == 2 {
			if incoming != nil {
				select {
				case incoming <- copyBytes(msg):
				default:
					// drop if receiver is slow
				}
			}
		}
	}
}

// writePump flushes outbound messages to the connection until the client is closed.
func (h *Handler) writePump(c *client) {
	defer c.close()

	for {
		select {
		case msg, ok := <-c.send:
			if !ok {
				// channel closed, close underlying conn
				_ = c.conn.SetWriteDeadline(time.Now().Add(h.writeTimeout))
				_ = c.conn.WriteMessage(1, []byte{})
				return
			}
			_ = c.conn.SetWriteDeadline(time.Now().Add(h.writeTimeout))
			if err := c.conn.WriteMessage(1, msg); err != nil {
				return
			}
		case <-c.closed:
			return
		}
	}
}

// close closes client resources; safe to call multiple times.
func (c *client) close() {
	c.closeOnce.Do(func() {
		close(c.closed)
		// drain send channel
		go func() {
			for range c.send {
			}
		}()
		_ = c.conn.Close()
	})
}

// copyBytes returns a copy of b to avoid accidental reuse of buffers.
func copyBytes(b []byte) []byte {
	cp := make([]byte, len(b))
	copy(cp, b)
	return cp
}

// The adapter allows an optional way to receive incoming messages. Consumers can inject a channel
// via context using this unexported key. This is intentionally simple and can be replaced with a
// more feature-rich event dispatching mechanism by the application code.

type contextKey string

var incomingKey contextKey = "websocket_incoming_chan"

// WithIncomingChan returns a context that carries the incoming channel used by the adapter's read pump.
func WithIncomingChan(ctx context.Context, ch chan<- []byte) context.Context {
	return context.WithValue(ctx, incomingKey, ch)
}

func incomingChanFromContext(ctx context.Context) chan<- []byte {
	v := ctx.Value(incomingKey)
	if v == nil {
		return nil
	}
	ch, _ := v.(chan<- []byte)
	return ch
}