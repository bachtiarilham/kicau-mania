package delivery

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kicau.local/edge/config"
	"kicau.local/edge/internal/delivery/rest/setup"
)

type arenaSettingsFixture struct {
	setup.SystemInfo
}

func (arenaSettingsFixture) Arena(context.Context) (config.ArenaSettings, error) {
	return config.ArenaSettings{Channels: []config.ArenaChannel{
		{ID: "gantangan01", Source: "rtsp://demo:fixture@192.0.2.1/stream"},
	}}, nil
}

func TestArenaCredentialsRequireLoopbackAndNonce(t *testing.T) {
	nonce := strings.Repeat("a", 64)
	router := Router(config.Config{AccessMode: "lan", LocalControlToken: nonce}, Handlers{
		Setup: setup.New(arenaSettingsFixture{}, nonce),
	})
	for _, tc := range []struct {
		name, peer, token string
		code              int
	}{
		{"missing nonce", "127.0.0.1:30123", "", http.StatusForbidden},
		{"incorrect nonce", "127.0.0.1:30123", "incorrect", http.StatusForbidden},
		{"LAN peer", "192.0.2.1:30123", nonce, http.StatusForbidden},
		{"local control", "127.0.0.1:30123", nonce, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/setup/arena", nil)
			req.RemoteAddr = tc.peer
			req.Header.Set("X-Local-Control", tc.token)
			got := httptest.NewRecorder()
			router.ServeHTTP(got, req)
			if got.Code != tc.code || strings.Contains(got.Body.String(), "demo:fixture@") != (tc.code == http.StatusOK) {
				t.Fatalf("unexpected settings access with HTTP %d", got.Code)
			}
			if tc.code == http.StatusOK && got.Header().Get("Cache-Control") != "no-store" {
				t.Fatal("arena settings response could be cached")
			}
		})
	}
}
