// Package delivery assembles Gin protocol routes and shared transport middleware.
package delivery

import (
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/delivery/middleware"
	"kicau.cloud/control/internal/delivery/rest"
	"kicau.cloud/control/internal/delivery/rest/apidocs"
	"kicau.cloud/control/internal/delivery/rest/response"
	workflow "kicau.cloud/control/internal/delivery/rest/workflow"
)

type Handlers struct {
	Health   *rest.HealthHandler
	Workflow *workflow.Handler
	Verifier middleware.UserVerifier
}
type Options struct {
	AllowedOrigins, TrustedProxies []string
	RequestTimeout                 time.Duration
}

func Router(opts Options, h Handlers) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.RedirectTrailingSlash = false
	r.RedirectFixedPath = false
	r.HandleMethodNotAllowed = true
	_ = r.SetTrustedProxies(opts.TrustedProxies)
	r.Use(middleware.SecurityHeaders(), middleware.AccessLog(), middleware.Recover(), middleware.CanonicalParams(), middleware.Limits(opts.RequestTimeout), middleware.CORS(opts.AllowedOrigins))
	r.GET("/healthz", h.Health.Live)
	r.HEAD("/healthz", h.Health.Live)
	r.GET("/readyz", h.Health.Ready)
	r.HEAD("/readyz", h.Health.Ready)
	apidocs.Routes(r)
	h.Workflow.Public(r.Group("/api/v1/auth"))
	g := r.Group("/api/v1", middleware.RequireUser(h.Verifier))
	h.Workflow.Routes(g)
	r.NoRoute(func(c *gin.Context) { response.WriteError(c, apperror.ErrNotFound) })
	r.NoMethod(func(c *gin.Context) {
		methods := []string{}
		for _, route := range r.Routes() {
			if matches(route.Path, c.Request.URL.Path) {
				methods = append(methods, route.Method)
			}
		}
		sort.Strings(methods)
		c.Header("Allow", strings.Join(methods, ", "))
		response.JSON(c, 405, false, "method not allowed", nil, nil)
	})
	return r
}

func matches(pattern, path string) bool {
	p, s := strings.Split(pattern, "/"), strings.Split(path, "/")
	if len(p) != len(s) {
		return false
	}
	for i, part := range p {
		if strings.HasPrefix(part, ":") {
			if s[i] == "" {
				return false
			}
			continue
		}
		if part != s[i] {
			return false
		}
	}
	return true
}
