// Package apidocs serves generated API documentation without media dependencies.
package apidocs

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "kicau.local/edge/docs/generated" // Register the generated specification.
)

// Routes is available even while the engine is waiting for setup.
func Routes(r *gin.Engine) {
	r.GET("/api/docs", func(c *gin.Context) { c.Redirect(http.StatusFound, "/api/docs/index.html") })
	// Setup can replace the engine router while requests are still in flight.
	// Give each router its own mutable WebDAV prefix; the assets remain shared.
	assets := *swaggerFiles.Handler
	ui := ginSwagger.WrapHandler(&assets,
		ginSwagger.URL("/api/docs/doc.json"),
		ginSwagger.PersistAuthorization(false),
		ginSwagger.DocExpansion("none"),
	)
	r.GET("/api/docs/*any", func(c *gin.Context) {
		if c.Param("any") == "/" {
			c.Redirect(http.StatusFound, "/api/docs/index.html")
			return
		}
		ui(c)
	})
}
