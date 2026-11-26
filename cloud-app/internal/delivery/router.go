// Package delivery assembles Gin protocol routes and shared transport middleware.
package delivery

import (
	"os"
	"sort"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"

	"kicau.cloud/control/internal/apperror"
	assetDelivery "kicau.cloud/control/internal/asset/delivery"
	competitionDelivery "kicau.cloud/control/internal/competition/delivery"
	"kicau.cloud/control/internal/delivery/middleware"
	"kicau.cloud/control/internal/delivery/rest"
	"kicau.cloud/control/internal/delivery/rest/apidocs"
	mediaDelivery "kicau.cloud/control/internal/delivery/rest/media"
	"kicau.cloud/control/internal/delivery/rest/response"
	identityDelivery "kicau.cloud/control/internal/identity/delivery"
)

type Handlers struct {
	Media       *mediaDelivery.Handler
	Health      *rest.HealthHandler
	Identity    *identityDelivery.Handler
	Asset       *assetDelivery.Handler
	Competition *competitionDelivery.Handler
	Verifier    middleware.UserVerifier
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
	serviceName := os.Getenv("SERVICE_NAME")
	if serviceName == "" {
		serviceName = "cloud-app"
	}
	r.Use(otelgin.Middleware(serviceName), middleware.SecurityHeaders(), middleware.AccessLog(), middleware.PrometheusMiddleware(), middleware.Recover(), middleware.CanonicalParams(), middleware.Limits(opts.RequestTimeout), middleware.CORS(opts.AllowedOrigins))
	r.GET("/metrics", middleware.PrometheusHandler())
	r.GET("/healthz", h.Health.Live)
	r.HEAD("/healthz", h.Health.Live)
	r.GET("/readyz", h.Health.Ready)
	r.HEAD("/readyz", h.Health.Ready)
	apidocs.Routes(r)

	if h.Identity != nil {
		h.Identity.PublicRoutes(r.Group("/api/v1/auth"))
	}
	if h.Media != nil {
		h.Media.PublicRoutes(r)
	}

	g := r.Group("/api/v1", middleware.RequireUser(h.Verifier))

	if h.Identity != nil {
		h.Identity.ProtectedRoutes(g)
	}
	if h.Asset != nil {
		h.Asset.Routes(g)
	}
	if h.Competition != nil {
		h.Competition.Routes(g)
	}
	if h.Media != nil {
		h.Media.Routes(g, r.Group("/api/v1/engine/media"))
	}

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
