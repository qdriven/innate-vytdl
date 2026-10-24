package server

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
)

// Hub keeps the set of connected WebSocket clients and broadcasts events.
// Message shapes match the web backend exactly:
//
//	queue + download events: {"event": "<name>", "data": <payload>}
//	vtt analysis events:     {"type": "<name>", ...fields}
type Hub struct {
	mu      sync.Mutex
	clients map[*websocket.Conn]struct{}
}

// NewHub creates an empty hub.
func NewHub() *Hub {
	return &Hub{clients: make(map[*websocket.Conn]struct{})}
}

// ServeWS accepts a WebSocket connection and keeps it until the peer goes away.
// The daemon is the only sender; incoming messages are drained (clients may
// send subscribe pings like the web backend tolerated).
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	h.mu.Lock()
	h.clients[conn] = struct{}{}
	h.mu.Unlock()

	defer func() {
		h.mu.Lock()
		delete(h.clients, conn)
		h.mu.Unlock()
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}()

	for {
		if _, _, err := conn.Read(r.Context()); err != nil {
			return
		}
	}
}

func (h *Hub) send(payload any) {
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for conn := range h.clients {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		err := conn.Write(ctx, websocket.MessageText, data)
		cancel()
		if err != nil {
			_ = conn.CloseNow()
			delete(h.clients, conn)
		}
	}
}

// Broadcast sends a {"event","data"} message to every connected client.
func (h *Hub) Broadcast(event string, data any) {
	h.send(map[string]any{"event": event, "data": data})
}

// BroadcastType sends a {"type", ...fields} message (VTT analysis convention).
func (h *Hub) BroadcastType(typ string, fields map[string]any) {
	payload := map[string]any{"type": typ}
	for k, v := range fields {
		payload[k] = v
	}
	h.send(payload)
}

// ClientCount returns the number of connected clients (diagnostics).
func (h *Hub) ClientCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.clients)
}
