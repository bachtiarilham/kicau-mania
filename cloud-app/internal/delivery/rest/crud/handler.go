// Package crud adapts the shared resource HTTP contract to domain-owned usecases.
package crud

import (
	"context"

	"github.com/gin-gonic/gin"

	"kicau.cloud/control/internal/core/model"
	"kicau.cloud/control/internal/delivery/rest/request"
	"kicau.cloud/control/internal/delivery/rest/response"
)

type UseCase interface {
	List(context.Context, string, string, string, string, int) ([]model.Record, error)
	Get(context.Context, string, string, string, string) (model.Record, error)
	Create(context.Context, string, string, string, model.Record) (model.Record, error)
	Update(context.Context, string, string, string, string, model.Record) (model.Record, error)
	Delete(context.Context, string, string, string, string) error
}

func Routes(g *gin.RouterGroup, path, resource string, u UseCase) {
	idParam := "id"
	if resource == "arenas" {
		idParam = "arena"
	}
	arena := func(c *gin.Context) string {
		if v := c.Param("arena"); v != "" && resource != "arenas" {
			return v
		}
		return c.Query("arena_id")
	}
	g.GET(path, func(c *gin.Context) {
		cursor, limit, ok := request.Page(c)
		if !ok {
			return
		}
		rows, e := u.List(c.Request.Context(), request.Actor(c), resource, arena(c), cursor, limit)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Records retrieved", request.PageData(rows, limit))
	})
	g.POST(path, func(c *gin.Context) {
		var in model.Record
		if !request.Bind(c, &in) {
			return
		}
		if in == nil {
			response.BadRequest(c, "An object is required", nil)
			return
		}
		row, e := u.Create(c.Request.Context(), request.Actor(c), resource, arena(c), in)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Created(c, "Record created", row)
	})
	g.GET(path+"/:"+idParam, func(c *gin.Context) {
		row, e := u.Get(c.Request.Context(), request.Actor(c), resource, c.Param(idParam), arena(c))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Record retrieved", row)
	})
	g.PATCH(path+"/:"+idParam, func(c *gin.Context) {
		var in model.Record
		if !request.Bind(c, &in) {
			return
		}
		if in == nil {
			response.BadRequest(c, "An object is required", nil)
			return
		}
		row, e := u.Update(c.Request.Context(), request.Actor(c), resource, c.Param(idParam), arena(c), in)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Record updated", row)
	})
	g.DELETE(path+"/:"+idParam, func(c *gin.Context) {
		if e := u.Delete(c.Request.Context(), request.Actor(c), resource, c.Param(idParam), arena(c)); e != nil {
			response.WriteError(c, e)
			return
		}
		response.NoContent(c)
	})
}
