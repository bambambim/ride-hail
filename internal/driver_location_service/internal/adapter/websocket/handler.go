package ws

import (
	"net/http"

	"github.com/gorilla/websocket"
)

var simpleUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

func (h *Hub) Handler(w http.ResponseWriter, r *http.Request) {
	conn, err := simpleUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := newClient(conn, h)
	h.register <- client

	go client.writeLoop()
	go client.readLoop()
}
