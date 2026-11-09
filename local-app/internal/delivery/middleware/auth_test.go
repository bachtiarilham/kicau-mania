package middleware

import (
	"encoding/base64"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestAuthAndOrigin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	token := strings.Repeat("a", 32)
	r := gin.New()
	r.Use(SecurityHeaders(), CORS([]string{"http://arena.test"}), Auth(token))
	r.GET("/", func(c *gin.Context) { c.Status(204) })
	for _, test := range []struct {
		auth, protocol, origin string
		status                 int
	}{{"", "", "", 401}, {"Bearer " + token, "", "", 204}, {"", "edge-v1, edge-token." + base64.RawURLEncoding.EncodeToString([]byte(token)), "http://arena.test", 204}, {"Bearer " + token, "", "http://evil.test", 403}, {"Bearer wrong", "", "", 401}} {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Authorization", test.auth)
		req.Header.Set("Sec-WebSocket-Protocol", test.protocol)
		req.Header.Set("Origin", test.origin)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != test.status {
			t.Fatalf("got %d, expected %d", w.Code, test.status)
		}
	}
}
