// Package whep proxies authorized WebRTC signaling to the local media server.
package whep

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"kicau.local/edge/internal/delivery/rest/response"
)

var sessionID = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)

type PlaybackUseCase interface {
	ValidateChannel(string) error
	Authorize(context.Context, string, string) error
}

type Handler struct {
	engine    PlaybackUseCase
	target    *url.URL
	transport *http.Transport
}

func New(engine PlaybackUseCase, upstream string) (*Handler, error) {
	target, err := url.Parse(upstream)
	if err != nil {
		return nil, err
	}
	if target.Host == "" {
		return nil, http.ErrNotSupported
	}
	return &Handler{engine: engine, target: target, transport: &http.Transport{Proxy: nil, ResponseHeaderTimeout: 10 * time.Second, IdleConnTimeout: 30 * time.Second}}, nil
}

func (p *Handler) Serve(c *gin.Context) {
	channel := c.Param("channel")
	if err := p.engine.ValidateChannel(channel); err != nil {
		response.WriteError(c, err)
		return
	}
	if c.Request.URL.RawQuery != "" {
		c.AbortWithStatus(400)
		return
	}
	if c.Request.Method == http.MethodPost {
		if c.ContentType() != "application/sdp" {
			c.AbortWithStatus(415)
			return
		}
		ticket, ok := strings.CutPrefix(c.GetHeader("Authorization"), "Bearer ")
		if !ok {
			c.AbortWithStatus(401)
			return
		}
		if err := p.engine.Authorize(c.Request.Context(), channel, ticket); err != nil {
			response.WriteError(c, err)
			return
		}
	} else if !sessionID.MatchString(c.Param("session")) {
		c.AbortWithStatus(404)
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128*1024)
	path := "/" + channel + "/whep"
	if c.Param("session") != "" {
		path += "/" + c.Param("session")
	}
	proxy := &httputil.ReverseProxy{
		Transport: p.transport,
		ErrorLog:  log.New(io.Discard, "", 0),
		Director: func(r *http.Request) {
			r.URL.Scheme = p.target.Scheme
			r.URL.Host = p.target.Host
			r.Host = p.target.Host
			r.URL.Path = path
			r.URL.RawPath = ""
			r.URL.RawQuery = ""
			r.Header.Del("Authorization")
			r.Header.Del("Cookie")
			r.Header["X-Forwarded-For"] = nil
			r.Header.Del("X-Real-IP")
			r.Header.Del("Forwarded")
		},
		ModifyResponse: func(r *http.Response) error {
			if location := r.Header.Get("Location"); location != "" {
				parsed, err := url.Parse(location)
				if err != nil {
					return http.ErrNotSupported
				}
				prefix := "/" + channel + "/whep/"
				secret, ok := strings.CutPrefix(parsed.Path, prefix)
				if !ok || !sessionID.MatchString(secret) || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Host != "" && parsed.Host != p.target.Host {
					return http.ErrNotSupported
				}
				r.Header.Set("Location", "/api/v1/playback/"+channel+"/whep/"+secret)
			}
			// The API origin middleware owns CORS. ReverseProxy appends upstream
			// headers, so retaining these would duplicate Access-Control-Allow-Origin.
			for name := range r.Header {
				if strings.HasPrefix(strings.ToLower(name), "access-control-") {
					r.Header.Del(name)
				}
			}
			r.Header.Del("Vary")
			r.Header.Del("Cache-Control")
			r.Header.Del("X-Content-Type-Options")
			r.Header.Del("X-Request-ID")
			r.Header.Del("Set-Cookie")
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "media gateway unavailable", http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}
