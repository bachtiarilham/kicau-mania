package delivery

import (
	"context"
	"net/http"
	"net/url"
	"sync"
	"time"

	gorillaws "github.com/gorilla/websocket"
)

type websocketStream struct {
	upgrade gorillaws.Upgrader
	mu      sync.Mutex
	conns   map[*gorillaws.Conn]struct{}
	closed  bool
	epoch   uint64
}

func newWebsocketStream(origins []string) *websocketStream {
	return &websocketStream{
		conns: map[*gorillaws.Conn]struct{}{},
		upgrade: gorillaws.Upgrader{
			Subprotocols: []string{"edge-v1"},
			CheckOrigin: func(r *http.Request) bool {
				o := r.Header.Get("Origin")
				if o == "" {
					return true
				}
				if origin, err := url.Parse(o); err == nil && origin.User == nil && origin.Path == "" && origin.RawQuery == "" && origin.Fragment == "" && (origin.Scheme == "http" || origin.Scheme == "https") && origin.Host == r.Host {
					return true
				}
				for _, v := range origins {
					if o == v {
						return true
					}
				}
				return false
			},
		},
	}
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
	defer func() {
		conn.Close()
		h.mu.Lock()
		delete(h.conns, conn)
		h.mu.Unlock()
	}()
	conn.SetReadLimit(1024)
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, e := conn.ReadMessage(); e != nil {
				return
			}
		}
	}()
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case msg, ok := <-messages:
			if !ok {
				return
			}
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if conn.WriteMessage(messageType, msg) != nil {
				return
			}
		case <-ticker.C:
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			if conn.WriteMessage(gorillaws.PingMessage, nil) != nil {
				return
			}
		}
	}
}

func (h *websocketStream) CloseSessions(_ context.Context) error {
	h.mu.Lock()
	h.epoch++
	conns := make([]*gorillaws.Conn, 0, len(h.conns))
	for c := range h.conns {
		conns = append(conns, c)
	}
	h.conns = map[*gorillaws.Conn]struct{}{}
	h.mu.Unlock()
	for _, c := range conns {
		_ = c.WriteControl(gorillaws.CloseMessage, gorillaws.FormatCloseMessage(gorillaws.CloseNormalClosure, "session reset"), time.Now().Add(time.Second))
		_ = c.Close()
	}
	return nil
}

func (h *websocketStream) generation() uint64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.epoch
}

func (h *websocketStream) Close() {
	h.mu.Lock()
	h.closed = true
	conns := make([]*gorillaws.Conn, 0, len(h.conns))
	for c := range h.conns {
		conns = append(conns, c)
	}
	h.conns = map[*gorillaws.Conn]struct{}{}
	h.mu.Unlock()
	for _, c := range conns {
		c.Close()
	}
}
