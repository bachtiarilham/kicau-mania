package middleware

import (
	"crypto/subtle"
	"net"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"kicau.local/edge/internal/apperror"
	"kicau.local/edge/internal/delivery/rest/response"
)

// IsLocalControl uses the actual peer, not forwarded headers or an arbitrary localhost Host.
func IsLocalControl(c *gin.Context, origins ...[]string) bool {
	host, _, e := net.SplitHostPort(c.Request.RemoteAddr)
	ip := net.ParseIP(host)
	if e != nil || ip == nil || !ip.IsLoopback() {
		return false
	}
	u, e := url.Parse("http://" + c.Request.Host)
	if e != nil {
		return false
	}
	name := strings.ToLower(u.Hostname())
	if name != "localhost" && (net.ParseIP(name) == nil || !net.ParseIP(name).IsLoopback()) {
		return false
	}
	explicit := false
	if o := c.GetHeader("Origin"); o != "" {
		v, e := url.Parse(o)
		if e != nil || v.User != nil || v.Path != "" || v.RawQuery != "" || v.Fragment != "" || (v.Scheme != "http" && v.Scheme != "https") {
			return false
		}
		if len(origins) > 0 && (v.Hostname() == "localhost" || net.ParseIP(v.Hostname()) != nil && net.ParseIP(v.Hostname()).IsLoopback()) {
			for _, allowed := range origins[0] {
				if o == allowed {
					explicit = true
				}
			}
		}
		if v.Host != c.Request.Host && !explicit {
			return false
		}
	}
	return c.GetHeader("Sec-Fetch-Site") != "cross-site" || explicit
}
func LocalControl(nonce string, origins ...[]string) gin.HandlerFunc {
	checkNonce := RequireLocalControlNonce(nonce)
	return func(c *gin.Context) {
		if len(nonce) < 32 || !IsLocalControl(c, origins...) {
			c.Abort()
			response.WriteError(c, apperror.ErrForbidden)
			return
		}
		if c.Request.Method != "GET" && c.Request.Method != "HEAD" {
			checkNonce(c)
			return
		}
		c.Next()
	}
}

func RequireLocalControlNonce(nonce string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(nonce) < 32 || subtle.ConstantTimeCompare([]byte(c.GetHeader("X-Local-Control")), []byte(nonce)) != 1 {
			c.Abort()
			response.WriteError(c, apperror.ErrForbidden)
			return
		}
		c.Next()
	}
}
func LANViewer(cidrs []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ip := net.ParseIP(c.ClientIP())
		allowed := ip != nil && ip.IsLoopback()
		for _, v := range cidrs {
			_, sub, e := net.ParseCIDR(v)
			if e == nil && sub.Contains(ip) {
				allowed = true
			}
		}
		if !allowed {
			c.Abort()
			response.WriteError(c, apperror.ErrForbidden)
			return
		}
		c.Next()
	}
}

// Operator keeps service tokens usable and accepts browser control only on loopback with a nonce.
func Operator(token, nonce string, lan bool, origins ...[]string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if lan && IsLocalControl(c, origins...) && c.GetHeader("X-Local-Control") != "" {
			LocalControl(nonce, origins...)(c)
			return
		}
		Auth(token)(c)
	}
}
