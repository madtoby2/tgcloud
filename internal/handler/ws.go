package handler

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type WSHub struct {
	mu      sync.RWMutex
	conns   map[*websocket.Conn]bool
	logger  *zap.Logger
}

func NewWSHub(logger *zap.Logger) *WSHub {
	return &WSHub{
		conns:  make(map[*websocket.Conn]bool),
		logger: logger,
	}
}

func (h *WSHub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Warn("ws upgrade failed", zap.Error(err))
		return
	}

	h.mu.Lock()
	h.conns[conn] = true
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.conns, conn)
		h.mu.Unlock()
		conn.Close()
	}()

	// Keep connection alive, read pings
	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func (h *WSHub) Broadcast(event string, data interface{}) {
	msg, err := json.Marshal(map[string]interface{}{
		"event": event,
		"data":  data,
	})
	if err != nil {
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	for conn := range h.conns {
		conn.WriteMessage(websocket.TextMessage, msg)
	}
}

func (h *WSHub) ConnCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}
