package delivery

import (
	"context"
	"strconv"

	"github.com/gin-gonic/gin"

	"kicau.cloud/control/internal/asset/model"
	"kicau.cloud/control/internal/delivery/rest/request"
	"kicau.cloud/control/internal/delivery/rest/response"
)

type AssetBusiness interface {
	GetArena(ctx context.Context, id string) (*model.ArenaView, error)
	ListArenas(ctx context.Context, cursor *string, limit int) ([]model.ArenaView, error)
	CreateArena(ctx context.Context, actorID string, req *model.CreateArenaRequest) (*model.ArenaView, error)
	UpdateArena(ctx context.Context, id string, name, address, timezone *string, isActive *bool) (*model.ArenaView, error)
	DeleteArena(ctx context.Context, id string) error

	GetGantangan(ctx context.Context, arenaID, id string) (*model.GantanganView, error)
	ListGantangan(ctx context.Context, arenaID string, cursor *string, limit int) ([]model.GantanganView, error)
	CreateGantangan(ctx context.Context, arenaID string, number int, desc *string, baris, kolom int) (*model.GantanganView, error)
	UpdateGantangan(ctx context.Context, arenaID, id string, number *int, desc *string, baris, kolom *int) (*model.GantanganView, error)
	DeleteGantangan(ctx context.Context, arenaID, id string) error

	GetSpecies(ctx context.Context, id string) (*model.BirdSpeciesView, error)
	ListSpecies(ctx context.Context, cursor *string, limit int) ([]model.BirdSpeciesView, error)
	CreateSpecies(ctx context.Context, name string) (*model.BirdSpeciesView, error)
	UpdateSpecies(ctx context.Context, id, name string) (*model.BirdSpeciesView, error)
	DeleteSpecies(ctx context.Context, id string) error

	GetBird(ctx context.Context, id string) (*model.BirdView, error)
	ListBirds(ctx context.Context, ownerID, search, cursor *string, limit int) ([]model.BirdView, error)
	CreateBird(ctx context.Context, actorID string, req *model.CreateBirdRequest) (*model.BirdView, error)
	UpdateBird(ctx context.Context, actorID, id string, req *model.UpdateBirdRequest) (*model.BirdView, error)
	DeleteBird(ctx context.Context, actorID, id string, expectedVersion int) error
	TransferBird(ctx context.Context, actorID, id string, req *model.TransferBirdRequest) error
	BirdOwnership(ctx context.Context, birdID string) ([]model.BirdOwnershipView, error)
	BirdAwards(ctx context.Context, birdID string, cursor *string, limit int) ([]model.AwardView, error)
	BirdAward(ctx context.Context, birdID, awardID string) (*model.AwardView, error)

	GetCategory(ctx context.Context, arenaID, id string) (*model.CategoryView, error)
	ListCategories(ctx context.Context, arenaID string, cursor *string, limit int) ([]model.CategoryView, error)
	CreateCategory(ctx context.Context, arenaID, code, name string) (*model.CategoryView, error)
	UpdateCategory(ctx context.Context, arenaID, id string, code, name *string) (*model.CategoryView, error)
	DeleteCategory(ctx context.Context, arenaID, id string) error

	ListCriteria(ctx context.Context, arenaID string) ([]model.ScoreCriteriaView, error)
	CreateCriteria(ctx context.Context, arenaID, name string) (*model.ScoreCriteriaView, error)
	UpdateCriteria(ctx context.Context, arenaID, id, name string) (*model.ScoreCriteriaView, error)
	DeleteCriteria(ctx context.Context, arenaID, id string) error
}

type Handler struct {
	Asset AssetBusiness
}

func result(c *gin.Context, data any, err error) {
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, "success", data)
}

func (h *Handler) Routes(g *gin.RouterGroup) {
	// Arenas
	g.GET("/arenas", func(c *gin.Context) {
		cursor, limit, ok := request.Page(c)
		if !ok {
			return
		}
		var curPtr *string
		if cursor != "" {
			curPtr = &cursor
		}
		items, err := h.Asset.ListArenas(c.Request.Context(), curPtr, limit)
		if err != nil {
			result(c, nil, err)
			return
		}
		result(c, response.Paginate(items, limit, func(a model.ArenaView) string { return a.ID }), nil)
	})

	g.GET("/arenas/:arena", func(c *gin.Context) {
		v, err := h.Asset.GetArena(c.Request.Context(), c.Param("arena"))
		result(c, v, err)
	})

	g.POST("/arenas", func(c *gin.Context) {
		var req model.CreateArenaRequest
		if !request.Bind(c, &req) {
			return
		}
		v, err := h.Asset.CreateArena(c.Request.Context(), request.Actor(c), &req)
		result(c, v, err)
	})

	for _, method := range []string{"PATCH", "PUT"} {
		g.Handle(method, "/arenas/:arena", func(c *gin.Context) {
			var body struct {
				Name     *string `json:"name"`
				Address  *string `json:"address"`
				Timezone *string `json:"timezone"`
				IsActive *bool   `json:"is_active"`
			}
			if !request.Bind(c, &body) {
				return
			}
			v, err := h.Asset.UpdateArena(c.Request.Context(), c.Param("arena"), body.Name, body.Address, body.Timezone, body.IsActive)
			result(c, v, err)
		})
	}

	g.DELETE("/arenas/:arena", func(c *gin.Context) {
		err := h.Asset.DeleteArena(c.Request.Context(), c.Param("arena"))
		result(c, nil, err)
	})

	// Gantangan
	g.GET("/arenas/:arena/gantangan", func(c *gin.Context) {
		cursor, limit, ok := request.Page(c)
		if !ok {
			return
		}
		var curPtr *string
		if cursor != "" {
			curPtr = &cursor
		}
		items, err := h.Asset.ListGantangan(c.Request.Context(), c.Param("arena"), curPtr, limit)
		if err != nil {
			result(c, nil, err)
			return
		}
		result(c, response.Paginate(items, limit, func(gt model.GantanganView) string { return gt.ID }), nil)
	})

	g.GET("/arenas/:arena/gantangan/:id", func(c *gin.Context) {
		v, err := h.Asset.GetGantangan(c.Request.Context(), c.Param("arena"), c.Param("id"))
		result(c, v, err)
	})

	g.POST("/arenas/:arena/gantangan", func(c *gin.Context) {
		var req struct {
			Number      int     `json:"number"`
			Description *string `json:"description"`
			Baris       int     `json:"baris"`
			Kolom       int     `json:"kolom"`
		}
		if !request.Bind(c, &req) {
			return
		}
		v, err := h.Asset.CreateGantangan(c.Request.Context(), c.Param("arena"), req.Number, req.Description, req.Baris, req.Kolom)
		result(c, v, err)
	})

	for _, method := range []string{"PATCH", "PUT"} {
		g.Handle(method, "/arenas/:arena/gantangan/:id", func(c *gin.Context) {
			var req struct {
				Number      *int    `json:"number"`
				Description *string `json:"description"`
				Baris       *int    `json:"baris"`
				Kolom       *int    `json:"kolom"`
			}
			if !request.Bind(c, &req) {
				return
			}
			v, err := h.Asset.UpdateGantangan(c.Request.Context(), c.Param("arena"), c.Param("id"), req.Number, req.Description, req.Baris, req.Kolom)
			result(c, v, err)
		})
	}

	g.DELETE("/arenas/:arena/gantangan/:id", func(c *gin.Context) {
		err := h.Asset.DeleteGantangan(c.Request.Context(), c.Param("arena"), c.Param("id"))
		result(c, nil, err)
	})

	// Bird Species
	g.GET("/bird-species", func(c *gin.Context) {
		cursor, limit, ok := request.Page(c)
		if !ok {
			return
		}
		var curPtr *string
		if cursor != "" {
			curPtr = &cursor
		}
		items, err := h.Asset.ListSpecies(c.Request.Context(), curPtr, limit)
		if err != nil {
			result(c, nil, err)
			return
		}
		result(c, response.Paginate(items, limit, func(s model.BirdSpeciesView) string { return s.ID }), nil)
	})

	g.GET("/bird-species/:id", func(c *gin.Context) {
		v, err := h.Asset.GetSpecies(c.Request.Context(), c.Param("id"))
		result(c, v, err)
	})

	g.POST("/bird-species", func(c *gin.Context) {
		var req struct {
			Name string `json:"name"`
		}
		if !request.Bind(c, &req) {
			return
		}
		v, err := h.Asset.CreateSpecies(c.Request.Context(), req.Name)
		result(c, v, err)
	})

	for _, method := range []string{"PATCH", "PUT"} {
		g.Handle(method, "/bird-species/:id", func(c *gin.Context) {
			var req struct {
				Name string `json:"name"`
			}
			if !request.Bind(c, &req) {
				return
			}
			v, err := h.Asset.UpdateSpecies(c.Request.Context(), c.Param("id"), req.Name)
			result(c, v, err)
		})
	}

	g.DELETE("/bird-species/:id", func(c *gin.Context) {
		err := h.Asset.DeleteSpecies(c.Request.Context(), c.Param("id"))
		result(c, nil, err)
	})

	// Birds
	g.GET("/birds", func(c *gin.Context) {
		cursor, limit, ok := request.Page(c)
		if !ok {
			return
		}
		var curPtr, ownerPtr, searchPtr *string
		if cursor != "" {
			curPtr = &cursor
		}
		if o := c.Query("owner_id"); o != "" {
			ownerPtr = &o
		}
		if s := c.Query("q"); s != "" {
			searchPtr = &s
		}
		items, err := h.Asset.ListBirds(c.Request.Context(), ownerPtr, searchPtr, curPtr, limit)
		if err != nil {
			result(c, nil, err)
			return
		}
		result(c, response.Paginate(items, limit, func(b model.BirdView) string { return b.ID }), nil)
	})

	g.GET("/birds/:id", func(c *gin.Context) {
		v, err := h.Asset.GetBird(c.Request.Context(), c.Param("id"))
		result(c, v, err)
	})

	g.POST("/birds", func(c *gin.Context) {
		var req model.CreateBirdRequest
		if !request.Bind(c, &req) {
			return
		}
		v, err := h.Asset.CreateBird(c.Request.Context(), request.Actor(c), &req)
		result(c, v, err)
	})

	for _, method := range []string{"PATCH", "PUT"} {
		g.Handle(method, "/birds/:id", func(c *gin.Context) {
			var req model.UpdateBirdRequest
			if !request.Bind(c, &req) {
				return
			}
			v, err := h.Asset.UpdateBird(c.Request.Context(), request.Actor(c), c.Param("id"), &req)
			result(c, v, err)
		})
	}

	g.DELETE("/birds/:id", func(c *gin.Context) {
		expectedVersion, _ := strconv.Atoi(c.Query("expected_version"))
		err := h.Asset.DeleteBird(c.Request.Context(), request.Actor(c), c.Param("id"), expectedVersion)
		result(c, nil, err)
	})

	g.POST("/birds/:id/transfer", func(c *gin.Context) {
		var req model.TransferBirdRequest
		if !request.Bind(c, &req) {
			return
		}
		err := h.Asset.TransferBird(c.Request.Context(), request.Actor(c), c.Param("id"), &req)
		result(c, nil, err)
	})

	g.GET("/birds/:id/ownership", func(c *gin.Context) {
		v, err := h.Asset.BirdOwnership(c.Request.Context(), c.Param("id"))
		result(c, v, err)
	})

	g.GET("/birds/:id/awards", func(c *gin.Context) {
		cursor, limit, ok := request.Page(c)
		if !ok {
			return
		}
		var curPtr *string
		if cursor != "" {
			curPtr = &cursor
		}
		v, err := h.Asset.BirdAwards(c.Request.Context(), c.Param("id"), curPtr, limit)
		if err != nil {
			result(c, nil, err)
			return
		}
		result(c, response.Paginate(v, limit, func(a model.AwardView) string { return a.ID }), nil)
	})

	g.GET("/birds/:id/awards/:award", func(c *gin.Context) {
		v, err := h.Asset.BirdAward(c.Request.Context(), c.Param("id"), c.Param("award"))
		result(c, v, err)
	})

	// Categories
	g.GET("/arenas/:arena/categories", func(c *gin.Context) {
		cursor, limit, ok := request.Page(c)
		if !ok {
			return
		}
		var curPtr *string
		if cursor != "" {
			curPtr = &cursor
		}
		items, err := h.Asset.ListCategories(c.Request.Context(), c.Param("arena"), curPtr, limit)
		if err != nil {
			result(c, nil, err)
			return
		}
		result(c, response.Paginate(items, limit, func(cat model.CategoryView) string { return cat.ID }), nil)
	})

	g.GET("/arenas/:arena/categories/:id", func(c *gin.Context) {
		v, err := h.Asset.GetCategory(c.Request.Context(), c.Param("arena"), c.Param("id"))
		result(c, v, err)
	})

	g.POST("/arenas/:arena/categories", func(c *gin.Context) {
		var req struct {
			Code string `json:"code"`
			Name string `json:"name"`
		}
		if !request.Bind(c, &req) {
			return
		}
		v, err := h.Asset.CreateCategory(c.Request.Context(), c.Param("arena"), req.Code, req.Name)
		result(c, v, err)
	})

	for _, method := range []string{"PATCH", "PUT"} {
		g.Handle(method, "/arenas/:arena/categories/:id", func(c *gin.Context) {
			var req struct {
				Code *string `json:"code"`
				Name *string `json:"name"`
			}
			if !request.Bind(c, &req) {
				return
			}
			v, err := h.Asset.UpdateCategory(c.Request.Context(), c.Param("arena"), c.Param("id"), req.Code, req.Name)
			result(c, v, err)
		})
	}

	g.DELETE("/arenas/:arena/categories/:id", func(c *gin.Context) {
		err := h.Asset.DeleteCategory(c.Request.Context(), c.Param("arena"), c.Param("id"))
		result(c, nil, err)
	})

	// Criteria
	g.GET("/arenas/:arena/criteria", func(c *gin.Context) {
		v, err := h.Asset.ListCriteria(c.Request.Context(), c.Param("arena"))
		result(c, v, err)
	})

	g.POST("/arenas/:arena/criteria", func(c *gin.Context) {
		var req struct {
			CriteriaName string `json:"criteria_name"`
		}
		if !request.Bind(c, &req) {
			return
		}
		v, err := h.Asset.CreateCriteria(c.Request.Context(), c.Param("arena"), req.CriteriaName)
		result(c, v, err)
	})

	for _, method := range []string{"PATCH", "PUT"} {
		g.Handle(method, "/arenas/:arena/criteria/:id", func(c *gin.Context) {
			var req struct {
				CriteriaName string `json:"criteria_name"`
			}
			if !request.Bind(c, &req) {
				return
			}
			v, err := h.Asset.UpdateCriteria(c.Request.Context(), c.Param("arena"), c.Param("id"), req.CriteriaName)
			result(c, v, err)
		})
	}

	g.DELETE("/arenas/:arena/criteria/:id", func(c *gin.Context) {
		err := h.Asset.DeleteCriteria(c.Request.Context(), c.Param("arena"), c.Param("id"))
		result(c, nil, err)
	})
}
