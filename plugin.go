package websocket

import (
	"net/http"
	"sync"

	dreego "github.com/dreego-stack/dreego/core"
)

type Options struct {
	Path string
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*Conn]struct{}
}

func newHub() *Hub {
	return &Hub{clients: map[*Conn]struct{}{}}
}

func (h *Hub) register(c *Conn) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) unregister(c *Conn) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

func (h *Hub) Broadcast(data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		c.WriteText(data)
	}
}

func (h *Hub) clientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

var (
	globalHub *Hub
	once      sync.Once
)

func HubInstance() *Hub {
	once.Do(func() {
		globalHub = newHub()
	})
	return globalHub
}

func Register(app *dreego.App, options Options) error {
	if options.Path == "" {
		options.Path = "/ws"
	}
	hub := HubInstance()
	return app.Register(http.MethodGet, options.Path, wsHandler(hub))
}

func wsHandler(hub *Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrade(w, r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		hub.register(conn)
		defer hub.unregister(conn)
		for {
			opcode, payload, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if opcode == 0x8 {
				return
			}
			conn.WriteText(payload)
		}
	}
}