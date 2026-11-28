package delivery

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"kicau.local/edge/config"
	roundDelivery "kicau.local/edge/internal/competition/delivery"
	roundUsecase "kicau.local/edge/internal/competition/usecase"
)

func TestRoundLANReadAndLocalControl(t *testing.T) {
	nonce := strings.Repeat("a", 64)
	cfg := config.Config{AccessMode: "lan", LocalControlToken: nonce, JudgeReadCIDRs: []string{"192.168.1.58/32"}}
	router := Router(cfg, Handlers{Rounds: roundDelivery.New(roundUsecase.NewRounds(time.Now))})
	request := func(method, path, peer, token, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://localhost/api/v1/local/round"+path, strings.NewReader(body))
		req.RemoteAddr = peer
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Local-Control", token)
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	payload := `{"arenaId":"a","eventId":"e","roundId":"r","startTime":"13:00","endTime":"13:20","participants":[{"gantanganNumber":1,"birdName":"Bintang","ownerName":"Joko","ownerPhone":"private"}]}`
	if res := request(http.MethodPost, "/start", "192.168.1.58:2000", nonce, payload); res.Code != http.StatusForbidden {
		t.Fatalf("judge wrote round: %d", res.Code)
	}
	if res := request(http.MethodPost, "/start", "127.0.0.1:2000", nonce, payload); res.Code != http.StatusOK {
		t.Fatalf("operator could not start: %d %s", res.Code, res.Body.String())
	}
	if res := request(http.MethodGet, "", "192.168.1.57:2000", "", ""); res.Code != http.StatusForbidden {
		t.Fatalf("unexpected viewer: %d", res.Code)
	}
	metrics := httptest.NewRequest(http.MethodGet, "http://localhost/api/v1/metrics", nil)
	metrics.RemoteAddr = "192.168.1.58:2000"
	metricsResult := httptest.NewRecorder()
	router.ServeHTTP(metricsResult, metrics)
	if metricsResult.Code != http.StatusOK {
		t.Fatalf("judge allowlist did not grant media read access: %d", metricsResult.Code)
	}
	res := request(http.MethodGet, "", "192.168.1.58:2000", "", "")
	if res.Code != http.StatusOK || !strings.Contains(res.Body.String(), "Bintang") || strings.Contains(res.Body.String(), "private") || res.Header().Get("Cache-Control") != "no-store" ||
		!strings.Contains(res.Body.String(), `"startTime":"13:00"`) || !strings.Contains(res.Body.String(), `"endTime":"13:20"`) {
		t.Fatalf("invalid judge snapshot: %d %s", res.Code, res.Body.String())
	}
}
