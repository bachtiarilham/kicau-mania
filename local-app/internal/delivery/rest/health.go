// Package rest handles JSON REST endpoints, independently of protocol routing.
package rest

import (
	"github.com/gin-gonic/gin"
	"kicau.local/edge/internal/delivery/rest/response"
)

func Health(c *gin.Context) {
	response.Success(c, "Service is alive", gin.H{"status": "UP"})
}
