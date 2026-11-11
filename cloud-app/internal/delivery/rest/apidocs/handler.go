// Package apidocs serves the generated Swagger contract and UI without business dependencies.
package apidocs

import (
	"net/http"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	_ "kicau.cloud/control/docs/generated" // Register the Swaggo-generated specification.
)

// Routes keeps documentation under /api so the existing gateway forwards it to Go.
func Routes(r *gin.Engine) {
	r.GET("/api/docs", func(c *gin.Context) { c.Redirect(http.StatusFound, "/api/docs/index.html") })
	ui := ginSwagger.WrapHandler(swaggerFiles.Handler,
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
