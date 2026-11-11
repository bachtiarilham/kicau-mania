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
	// @Summary Daftar arenas
	// @Tags Assets
	// @Description Create/delete superadmin; admin arena boleh mengubah arenanya.
	// @Security BearerAuth
	// @Produce json
	// @Param cursor query string false "ID dari next_cursor halaman sebelumnya."
	// @Param limit query integer false "Jumlah item per halaman." default(25) minimum(1) maximum(100)
	// @Success 200 {object} response.Response{data=response.CursorPage[model.Record]}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas [get]

	// @Summary Daftar owners
	// @Tags Assets
	// @Description Peserta otomatis terhubung akun sendiri. user_id hanya superadmin. Petugas memakai arena_id untuk master manual.
	// @Security BearerAuth
	// @Produce json
	// @Param cursor query string false "ID dari next_cursor halaman sebelumnya."
	// @Param limit query integer false "Jumlah item per halaman." default(25) minimum(1) maximum(100)
	// @Param arena_id query string false "Scope arena; dibatasi hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=response.CursorPage[model.Record]}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/owners [get]

	// @Summary Daftar birds
	// @Tags Assets
	// @Description Hak master dibatasi kepemilikan/penugasan. owner_id diganti melalui aksi transfer.
	// @Security BearerAuth
	// @Produce json
	// @Param cursor query string false "ID dari next_cursor halaman sebelumnya."
	// @Param limit query integer false "Jumlah item per halaman." default(25) minimum(1) maximum(100)
	// @Param arena_id query string false "Scope arena; dibatasi hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=response.CursorPage[model.Record]}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/birds [get]

	// @Summary Daftar gantangan
	// @Tags Assets
	// @Description Petugas arena; nomor unik per arena.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param cursor query string false "ID dari next_cursor halaman sebelumnya."
	// @Param limit query integer false "Jumlah item per halaman." default(25) minimum(1) maximum(100)
	// @Success 200 {object} response.Response{data=response.CursorPage[model.Record]}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/gantangan [get]

	// @Summary Daftar engines
	// @Tags Assets
	// @Description Petugas arena; satu engine aktif per arena.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param cursor query string false "ID dari next_cursor halaman sebelumnya."
	// @Param limit query integer false "Jumlah item per halaman." default(25) minimum(1) maximum(100)
	// @Success 200 {object} response.Response{data=response.CursorPage[model.Record]}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/engines [get]

	// @Summary Daftar devices
	// @Tags Assets
	// @Description Petugas arena. Engine/gantangan harus di arena sama.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param cursor query string false "ID dari next_cursor halaman sebelumnya."
	// @Param limit query integer false "Jumlah item per halaman." default(25) minimum(1) maximum(100)
	// @Success 200 {object} response.Response{data=response.CursorPage[model.Record]}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/devices [get]

	// @Summary Daftar events
	// @Tags Competition
	// @Description Admin arena/operator.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param cursor query string false "ID dari next_cursor halaman sebelumnya."
	// @Param limit query integer false "Jumlah item per halaman." default(25) minimum(1) maximum(100)
	// @Success 200 {object} response.Response{data=response.CursorPage[model.Record]}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/events [get]

	// @Summary Daftar categories
	// @Tags Competition
	// @Description Admin arena; unik per arena.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param cursor query string false "ID dari next_cursor halaman sebelumnya."
	// @Param limit query integer false "Jumlah item per halaman." default(25) minimum(1) maximum(100)
	// @Success 200 {object} response.Response{data=response.CursorPage[model.Record]}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/categories [get]

	// @Summary Daftar rounds
	// @Tags Competition
	// @Description Admin arena/operator. Update hanya PREPARED. event/category/bird_type terkunci setelah ada item. Lifecycle melalui action ronde.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param cursor query string false "ID dari next_cursor halaman sebelumnya."
	// @Param limit query integer false "Jumlah item per halaman." default(25) minimum(1) maximum(100)
	// @Success 200 {object} response.Response{data=response.CursorPage[model.Record]}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/rounds [get]

	// @Summary Daftar criteria
	// @Tags Judging
	// @Description Admin arena; maksimal 100 kriteria. Terkunci setelah pertama digunakan.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param cursor query string false "ID dari next_cursor halaman sebelumnya."
	// @Param limit query integer false "Jumlah item per halaman." default(25) minimum(1) maximum(100)
	// @Success 200 {object} response.Response{data=response.CursorPage[model.Record]}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/criteria [get]
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
	// @Summary Buat arenas
	// @Tags Assets
	// @Description Create/delete superadmin; admin arena boleh mengubah arenanya.
	// @Security BearerAuth
	// @Accept json
	// @Param request body ArenaCreate true "JSON body"
	// @Produce json
	// @Success 201 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas [post]

	// @Summary Buat owners
	// @Tags Assets
	// @Description Peserta otomatis terhubung akun sendiri. user_id hanya superadmin. Petugas memakai arena_id untuk master manual.
	// @Security BearerAuth
	// @Accept json
	// @Param request body OwnerCreate true "JSON body"
	// @Produce json
	// @Param arena_id query string false "Scope arena; dibatasi hak aktor." Format(uuid)
	// @Success 201 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/owners [post]

	// @Summary Buat birds
	// @Tags Assets
	// @Description Hak master dibatasi kepemilikan/penugasan. owner_id diganti melalui aksi transfer.
	// @Security BearerAuth
	// @Accept json
	// @Param request body BirdCreate true "JSON body"
	// @Produce json
	// @Param arena_id query string false "Scope arena; dibatasi hak aktor." Format(uuid)
	// @Success 201 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/birds [post]

	// @Summary Buat gantangan
	// @Tags Assets
	// @Description Petugas arena; nomor unik per arena.
	// @Security BearerAuth
	// @Accept json
	// @Param request body GantanganCreate true "JSON body"
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 201 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/gantangan [post]

	// @Summary Buat engines
	// @Tags Assets
	// @Description Petugas arena; satu engine aktif per arena.
	// @Security BearerAuth
	// @Accept json
	// @Param request body EngineCreate true "JSON body"
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 201 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/engines [post]

	// @Summary Buat devices
	// @Tags Assets
	// @Description Petugas arena. Engine/gantangan harus di arena sama.
	// @Security BearerAuth
	// @Accept json
	// @Param request body DeviceCreate true "JSON body"
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 201 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/devices [post]

	// @Summary Buat events
	// @Tags Competition
	// @Description Admin arena/operator.
	// @Security BearerAuth
	// @Accept json
	// @Param request body EventCreate true "JSON body"
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 201 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/events [post]

	// @Summary Buat categories
	// @Tags Competition
	// @Description Admin arena; unik per arena.
	// @Security BearerAuth
	// @Accept json
	// @Param request body CategoryCreate true "JSON body"
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 201 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/categories [post]

	// @Summary Buat rounds
	// @Tags Competition
	// @Description Admin arena/operator. Update hanya PREPARED. event/category/bird_type terkunci setelah ada item. Lifecycle melalui action ronde.
	// @Security BearerAuth
	// @Accept json
	// @Param request body RoundCreate true "JSON body"
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 201 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/rounds [post]

	// @Summary Buat criteria
	// @Tags Judging
	// @Description Admin arena; maksimal 100 kriteria. Terkunci setelah pertama digunakan.
	// @Security BearerAuth
	// @Accept json
	// @Param request body CriterionCreate true "JSON body"
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 201 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/criteria [post]
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
	// @Summary Detail arenas
	// @Tags Assets
	// @Description Create/delete superadmin; admin arena boleh mengubah arenanya.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena} [get]

	// @Summary Detail owners
	// @Tags Assets
	// @Description Peserta otomatis terhubung akun sendiri. user_id hanya superadmin. Petugas memakai arena_id untuk master manual.
	// @Security BearerAuth
	// @Produce json
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param arena_id query string false "Scope arena; dibatasi hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/owners/{id} [get]

	// @Summary Detail birds
	// @Tags Assets
	// @Description Hak master dibatasi kepemilikan/penugasan. owner_id diganti melalui aksi transfer.
	// @Security BearerAuth
	// @Produce json
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param arena_id query string false "Scope arena; dibatasi hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/birds/{id} [get]

	// @Summary Detail gantangan
	// @Tags Assets
	// @Description Petugas arena; nomor unik per arena.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/gantangan/{id} [get]

	// @Summary Detail engines
	// @Tags Assets
	// @Description Petugas arena; satu engine aktif per arena.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/engines/{id} [get]

	// @Summary Detail devices
	// @Tags Assets
	// @Description Petugas arena. Engine/gantangan harus di arena sama.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/devices/{id} [get]

	// @Summary Detail events
	// @Tags Competition
	// @Description Admin arena/operator.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/events/{id} [get]

	// @Summary Detail categories
	// @Tags Competition
	// @Description Admin arena; unik per arena.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/categories/{id} [get]

	// @Summary Detail rounds
	// @Tags Competition
	// @Description Admin arena/operator. Update hanya PREPARED. event/category/bird_type terkunci setelah ada item. Lifecycle melalui action ronde.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/rounds/{id} [get]

	// @Summary Detail criteria
	// @Tags Judging
	// @Description Admin arena; maksimal 100 kriteria. Terkunci setelah pertama digunakan.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/criteria/{id} [get]
	g.GET(path+"/:"+idParam, func(c *gin.Context) {
		row, e := u.Get(c.Request.Context(), request.Actor(c), resource, c.Param(idParam), arena(c))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Record retrieved", row)
	})
	// @Summary Ubah arenas
	// @Tags Assets
	// @Description Create/delete superadmin; admin arena boleh mengubah arenanya. Kirim subset field; arena tidak dapat diubah.
	// @Security BearerAuth
	// @Accept json
	// @Param request body ArenaPatch true "JSON body"
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena} [patch]

	// @Summary Ubah owners
	// @Tags Assets
	// @Description Peserta otomatis terhubung akun sendiri. user_id hanya superadmin. Petugas memakai arena_id untuk master manual. Kirim subset field; arena tidak dapat diubah.
	// @Security BearerAuth
	// @Accept json
	// @Param request body OwnerPatch true "JSON body"
	// @Produce json
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param arena_id query string false "Scope arena; dibatasi hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/owners/{id} [patch]

	// @Summary Ubah birds
	// @Tags Assets
	// @Description Hak master dibatasi kepemilikan/penugasan. owner_id diganti melalui aksi transfer. Kirim subset field; arena tidak dapat diubah.
	// @Security BearerAuth
	// @Accept json
	// @Param request body BirdPatch true "JSON body"
	// @Produce json
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param arena_id query string false "Scope arena; dibatasi hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/birds/{id} [patch]

	// @Summary Ubah gantangan
	// @Tags Assets
	// @Description Petugas arena; nomor unik per arena. Kirim subset field; arena tidak dapat diubah.
	// @Security BearerAuth
	// @Accept json
	// @Param request body GantanganPatch true "JSON body"
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/gantangan/{id} [patch]

	// @Summary Ubah engines
	// @Tags Assets
	// @Description Petugas arena; satu engine aktif per arena. Kirim subset field; arena tidak dapat diubah.
	// @Security BearerAuth
	// @Accept json
	// @Param request body EnginePatch true "JSON body"
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/engines/{id} [patch]

	// @Summary Ubah devices
	// @Tags Assets
	// @Description Petugas arena. Engine/gantangan harus di arena sama. Kirim subset field; arena tidak dapat diubah.
	// @Security BearerAuth
	// @Accept json
	// @Param request body DevicePatch true "JSON body"
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/devices/{id} [patch]

	// @Summary Ubah events
	// @Tags Competition
	// @Description Admin arena/operator. Kirim subset field; arena tidak dapat diubah.
	// @Security BearerAuth
	// @Accept json
	// @Param request body EventPatch true "JSON body"
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/events/{id} [patch]

	// @Summary Ubah categories
	// @Tags Competition
	// @Description Admin arena; unik per arena. Kirim subset field; arena tidak dapat diubah.
	// @Security BearerAuth
	// @Accept json
	// @Param request body CategoryPatch true "JSON body"
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/categories/{id} [patch]

	// @Summary Ubah rounds
	// @Tags Competition
	// @Description Admin arena/operator. Update hanya PREPARED. event/category/bird_type terkunci setelah ada item. Lifecycle melalui action ronde. Kirim subset field; arena tidak dapat diubah.
	// @Security BearerAuth
	// @Accept json
	// @Param request body RoundPatch true "JSON body"
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/rounds/{id} [patch]

	// @Summary Ubah criteria
	// @Tags Judging
	// @Description Admin arena; maksimal 100 kriteria. Terkunci setelah pertama digunakan. Kirim subset field; arena tidak dapat diubah.
	// @Security BearerAuth
	// @Accept json
	// @Param request body CriterionPatch true "JSON body"
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/criteria/{id} [patch]
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
	// @Summary Hapus arenas
	// @Tags Assets
	// @Description Create/delete superadmin; admin arena boleh mengubah arenanya. Hanya bila belum direferensikan; konflik 409.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 204 "No response body"
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena} [delete]

	// @Summary Hapus owners
	// @Tags Assets
	// @Description Peserta otomatis terhubung akun sendiri. user_id hanya superadmin. Petugas memakai arena_id untuk master manual. Hanya bila belum direferensikan; konflik 409.
	// @Security BearerAuth
	// @Produce json
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param arena_id query string false "Scope arena; dibatasi hak aktor." Format(uuid)
	// @Success 204 "No response body"
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/owners/{id} [delete]

	// @Summary Hapus birds
	// @Tags Assets
	// @Description Hak master dibatasi kepemilikan/penugasan. owner_id diganti melalui aksi transfer. Hanya bila belum direferensikan; konflik 409.
	// @Security BearerAuth
	// @Produce json
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param arena_id query string false "Scope arena; dibatasi hak aktor." Format(uuid)
	// @Success 204 "No response body"
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/birds/{id} [delete]

	// @Summary Hapus gantangan
	// @Tags Assets
	// @Description Petugas arena; nomor unik per arena. Hanya bila belum direferensikan; konflik 409.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 204 "No response body"
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/gantangan/{id} [delete]

	// @Summary Hapus engines
	// @Tags Assets
	// @Description Petugas arena; satu engine aktif per arena. Hanya bila belum direferensikan; konflik 409.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 204 "No response body"
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/engines/{id} [delete]

	// @Summary Hapus devices
	// @Tags Assets
	// @Description Petugas arena. Engine/gantangan harus di arena sama. Hanya bila belum direferensikan; konflik 409.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 204 "No response body"
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/devices/{id} [delete]

	// @Summary Hapus events
	// @Tags Competition
	// @Description Admin arena/operator. Hanya bila belum direferensikan; konflik 409.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 204 "No response body"
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/events/{id} [delete]

	// @Summary Hapus categories
	// @Tags Competition
	// @Description Admin arena; unik per arena. Hanya bila belum direferensikan; konflik 409.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 204 "No response body"
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/categories/{id} [delete]

	// @Summary Hapus rounds
	// @Tags Competition
	// @Description Admin arena/operator. Update hanya PREPARED. event/category/bird_type terkunci setelah ada item. Lifecycle melalui action ronde. Hanya bila belum direferensikan; konflik 409.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 204 "No response body"
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/rounds/{id} [delete]

	// @Summary Hapus criteria
	// @Tags Judging
	// @Description Admin arena; maksimal 100 kriteria. Terkunci setelah pertama digunakan. Hanya bila belum direferensikan; konflik 409.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 204 "No response body"
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/criteria/{id} [delete]
	g.DELETE(path+"/:"+idParam, func(c *gin.Context) {
		if e := u.Delete(c.Request.Context(), request.Actor(c), resource, c.Param(idParam), arena(c)); e != nil {
			response.WriteError(c, e)
			return
		}
		response.NoContent(c)
	})
}
