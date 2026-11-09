package websocket

import (
	"context"
	"net/http"
	"sync"
	"time"

	gorillaws "github.com/gorilla/websocket"
)

// websocketStream owns connection tracking, ping/pong, and message transport.
type websocketStream struct {
	upgrade gorillaws.Upgrader
	mu      sync.Mutex
	conns   map[*gorillaws.Conn]struct{}
	closed  bool
	epoch   uint64
}

func newWebsocketStream(origins []string) *websocketStream {
	return &websocketStream{conns: map[*gorillaws.Conn]struct{}{}, upgrade: gorillaws.Upgrader{Subprotocols: []string{"edge-v1"}, CheckOrigin: func(r *http.Request) bool {
		o := r.Header.Get("Origin")
		if o == "" {
			return true
		}
		for _, v := range origins {
			if o == v {
				return true
			}
		}
		return false
	}}}
}
func (h *websocketStream) serve(w http.ResponseWriter, r *http.Request, messages <-chan []byte, messageType int, expected ...uint64) {
	conn, e := h.upgrade.Upgrade(w, r, nil)
	if e != nil {
		return
	}
	h.mu.Lock()
	if h.closed || (len(expected) > 0 && expected[0] != h.epoch) {
		h.mu.Unlock()
		conn.Close()
		return
	}
	h.conns[conn] = struct{}{}
	h.mu.Unlock()
	defer func() { conn.Close(); h.mu.Lock(); delete(h.conns, conn); h.mu.Unlock() }()
	conn.SetReadLimit(1024)
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(60 * time.Second)) })
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, e := conn.ReadMessage(); e != nil {
				return
			}
		}
	}()
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()
	for {
		select {
		case <-done:
			return
		case <-r.Context().Done():
			return
		case b, ok := <-messages:
			if !ok {
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if e = conn.WriteMessage(messageType, b); e != nil {
				return
			}
		case <-ping.C:
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if e = conn.WriteMessage(gorillaws.PingMessage, nil); e != nil {
				return
			}
		}
	}
}

func (h *websocketStream) generation() uint64 { h.mu.Lock(); defer h.mu.Unlock(); return h.epoch }
func (h *Handler) CloseSessions(context.Context) error {
	h.stream.mu.Lock()
	defer h.stream.mu.Unlock()
	h.stream.epoch++
	for c := range h.stream.conns {
		c.Close()
	}
	return nil
}

func (h *websocketStream) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for c := range h.conns {
		c.Close()
	}
}
