// Package apidocs serves the OpenAPI contract and Swagger UI without business dependencies.
package apidocs

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"kicau.cloud/control/docs"
)

// Routes keeps documentation under /api so the existing gateway forwards it to Go.
func Routes(r *gin.Engine) {
	for route, asset := range map[string]struct{ path, contentType string }{
		"/api/docs":                      {"swagger/index.html", "text/html; charset=utf-8"},
		"/api/docs/":                     {"swagger/index.html", "text/html; charset=utf-8"},
		"/api/docs/openapi.json":         {"openapi.json", "application/json; charset=utf-8"},
		"/api/docs/swagger-ui-bundle.js": {"swagger/swagger-ui-bundle.js", "text/javascript; charset=utf-8"},
		"/api/docs/swagger-ui.css":       {"swagger/swagger-ui.css", "text/css; charset=utf-8"},
		"/api/docs/initializer.js":       {"swagger/initializer.js", "text/javascript; charset=utf-8"},
	} {
		data, err := docs.Files.ReadFile(asset.path)
		if err != nil {
			panic("missing embedded documentation: " + asset.path)
		}
		handler := func(c *gin.Context) { c.Data(http.StatusOK, asset.contentType, data) }
		r.GET(route, handler)
		r.HEAD(route, handler)
	}
}
