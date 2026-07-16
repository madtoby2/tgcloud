package handler

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type WSClient struct {
	conn *websocket.Conn
	send chan []byte
}

type WSHub struct {
	mu      sync.RWMutex
	clients map[*WSClient]bool
}

func NewWSHub() *WSHub {
	return &WSHub{
		clients: make(map[*WSClient]bool),
	}
}

func (h *WSHub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &WSClient{conn: conn, send: make(chan []byte, 64)}
	h.mu.Lock()
	h.clients[c] = true
	h.mu.Unlock()

	go c.writePump()
	c.readPump(h)
}

func (h *WSHub) Broadcast(event string, data interface{}) {
	msg, _ := json.Marshal(map[string]interface{}{
		"event": event,
		"data":  data,
	})
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		select {
		case c.send <- msg:
		default:
		}
	}
}

func (c *WSClient) readPump(hub *WSHub) {
	defer func() {
		hub.mu.Lock()
		delete(hub.clients, c)
		hub.mu.Unlock()
		c.conn.Close()
	}()
	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (c *WSClient) writePump() {
	defer c.conn.Close()
	for msg := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			return
		}
	}
}
