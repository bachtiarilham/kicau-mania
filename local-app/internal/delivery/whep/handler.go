// Package whep proxies authorized WebRTC signaling to the local media server.
package whep

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"kicau.local/edge/internal/delivery/rest/response"
)

var sessionID = regexp.MustCompile(`^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{4}-[a-fA-F0-9]{12}$`)

type PlaybackUseCase interface {
	ValidateChannel(string) error
	Authorize(context.Context, string, string) error
}
type Admission interface{ AcquirePlayback(string) (func(), error) }
type upstreamSession struct {
	path, etag string
	generation uint64
}

type Handler struct {
	engine     PlaybackUseCase
	target     *url.URL
	transport  *http.Transport
	admission  Admission
	mu         sync.Mutex
	sessions   map[string]upstreamSession
	uncertain  bool
	generation uint64
}

func New(engine PlaybackUseCase, upstream string, admission Admission) (*Handler, error) {
	target, err := url.Parse(upstream)
	if err != nil {
		return nil, err
	}
	if target.Host == "" {
		return nil, http.ErrNotSupported
	}
	return &Handler{engine: engine, target: target, admission: admission, sessions: map[string]upstreamSession{}, transport: &http.Transport{Proxy: nil, ResponseHeaderTimeout: 10 * time.Second, IdleConnTimeout: 30 * time.Second}}, nil
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
		requiresTicket := true
		if policy, known := p.engine.(interface{ RequiresTicket() bool }); known {
			requiresTicket = policy.RequiresTicket()
		}
		if !ok && (requiresTicket || c.GetHeader("Authorization") != "") {
			c.AbortWithStatus(401)
			return
		}
		if err := p.engine.Authorize(c.Request.Context(), channel, ticket); err != nil {
			response.WriteError(c, err)
			return
		}
		if p.admission == nil {
			c.AbortWithStatus(503)
			return
		}
		release, err := p.admission.AcquirePlayback(channel)
		if err != nil {
			response.WriteError(c, err)
			return
		}
		defer release()
	} else if !sessionID.MatchString(c.Param("session")) {
		c.AbortWithStatus(404)
		return
	}
	if c.Request.Method != http.MethodPost {
		p.mu.Lock()
		_, ok := p.sessions[channel+"/"+c.Param("session")]
		p.mu.Unlock()
		if !ok {
			c.AbortWithStatus(404)
			return
		}
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 128*1024)
	path := "/" + channel + "/whep"
	if c.Param("session") != "" {
		path += "/" + c.Param("session")
	}
	p.mu.Lock()
	generation := p.generation
	p.mu.Unlock()
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
			// The gateway already validates Origin. Internal MediaMTX must not
			// apply a second, single-origin policy to our multiple allowed frontends.
			r.Header.Del("Origin")
			r.Header["X-Forwarded-For"] = nil
			r.Header.Del("X-Real-IP")
			r.Header.Del("Forwarded")
		},
		ModifyResponse: func(r *http.Response) error {
			if c.Request.Method == http.MethodPost && r.StatusCode == 201 && r.Header.Get("Location") == "" {
				p.markUncertain()
				return http.ErrNotSupported
			}
			if location := r.Header.Get("Location"); location != "" {
				parsed, err := url.Parse(location)
				if err != nil {
					p.markUncertain()
					return http.ErrNotSupported
				}
				prefix := "/" + channel + "/whep/"
				secret, ok := strings.CutPrefix(parsed.Path, prefix)
				if !ok || !sessionID.MatchString(secret) || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Host != "" && parsed.Host != p.target.Host {
					p.markUncertain()
					return http.ErrNotSupported
				}
				p.mu.Lock()
				p.sessions[channel+"/"+secret] = upstreamSession{path: parsed.Path, etag: r.Header.Get("ETag"), generation: generation}
				p.mu.Unlock()
				r.Header.Set("Location", "/api/v1/playback/"+channel+"/whep/"+secret)
			}
			if c.Request.Method == http.MethodDelete && (r.StatusCode < 300 || r.StatusCode == 404) {
				p.mu.Lock()
				delete(p.sessions, channel+"/"+c.Param("session"))
				p.mu.Unlock()
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
			if c.Request.Method == http.MethodPost {
				p.markUncertain()
			}
			http.Error(w, "media gateway unavailable", http.StatusBadGateway)
		},
	}
	proxy.ServeHTTP(c.Writer, c.Request)
}

func (p *Handler) markUncertain() { p.mu.Lock(); p.uncertain = true; p.mu.Unlock() }

// CloseSessions runs only after admission is fenced and in-flight POSTs drained.
func (p *Handler) CloseSessions(ctx context.Context) error {
	p.mu.Lock()
	p.generation++
	entries := p.sessions
	p.sessions = map[string]upstreamSession{}
	failed := p.uncertain
	p.uncertain = false
	p.mu.Unlock()
	client := &http.Client{Transport: p.transport, Timeout: 8 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	for _, session := range entries {
		target := *p.target
		target.Path = session.path
		target.RawPath = ""
		target.RawQuery = ""
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, target.String(), nil)
		if err != nil {
			failed = true
			continue
		}
		if session.etag != "" {
			req.Header.Set("If-Match", session.etag)
		}
		res, err := client.Do(req)
		if err != nil {
			failed = true
			continue
		}
		res.Body.Close()
		if res.StatusCode >= 300 && res.StatusCode != 404 {
			failed = true
		}
	}
	if failed {
		return errors.New("upstream playback closure requires media restart")
	}
	return nil
}
