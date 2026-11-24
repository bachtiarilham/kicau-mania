package delivery

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	gorillaws "github.com/gorilla/websocket"
)

func TestWebsocketStreamReleasesConnections(t *testing.T) {
	for _, stop := range []string{"shutdown", "source closed", "client disconnected"} {
		t.Run(stop, func(t *testing.T) {
			stream := newWebsocketStream([]string{"http://arena.test"})
			defer stream.Close()
			messages := make(chan []byte)
			returned := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer close(returned)
				stream.serve(w, r, messages, gorillaws.BinaryMessage)
			}))
			defer server.Close()
			dialer := gorillaws.Dialer{Subprotocols: []string{"edge-v1"}}
			conn, _, err := dialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), http.Header{"Origin": []string{"http://arena.test"}})
			if err != nil {
				t.Fatal(err)
			}
			defer conn.Close()
			switch stop {
			case "shutdown":
				stream.Close()
			case "source closed":
				close(messages)
			case "client disconnected":
				conn.Close()
			}
			select {
			case <-returned:
			case <-time.After(2 * time.Second):
				t.Fatal("WebSocket stream did not stop")
			}
			if stop != "client disconnected" {
				conn.SetReadDeadline(time.Now().Add(time.Second))
				if _, _, err := conn.ReadMessage(); err == nil {
					t.Fatal("connection survived stream shutdown")
				}
			}
		})
	}
}
