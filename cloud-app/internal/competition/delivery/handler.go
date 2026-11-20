package delivery

import (
	"bytes"
	"context"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	"kicau.cloud/control/internal/apperror"
	"kicau.cloud/control/internal/competition/model"
	"kicau.cloud/control/internal/delivery/rest/request"
	"kicau.cloud/control/internal/delivery/rest/response"
)

type CompetitionBusiness interface {
	GetEvent(ctx context.Context, arenaID, id string) (*model.EventView, error)
	ListEvents(ctx context.Context, arenaID string, cursor *string, limit int) ([]model.EventView, error)
	CreateEvent(ctx context.Context, arenaID string, req *model.CreateEventRequest) (*model.EventView, error)
	UpdateEvent(ctx context.Context, arenaID, id string, req *model.CreateEventRequest) (*model.EventView, error)
	DeleteEvent(ctx context.Context, arenaID, id string) error

	GetRound(ctx context.Context, arenaID, id string) (*model.RoundView, error)
	ListRounds(ctx context.Context, arenaID string, cursor *string, limit int) ([]model.RoundView, error)
	CreateRound(ctx context.Context, arenaID string, req *model.CreateRoundRequest) (*model.RoundView, error)
	UpdateRound(ctx context.Context, arenaID, id string, req *model.CreateRoundRequest) (*model.RoundView, error)
	DeleteRound(ctx context.Context, arenaID, id string) error
	RoundAction(ctx context.Context, arenaID, id, action, expectedStatus string) (*model.RoundView, error)

	GetRegistration(ctx context.Context, arenaID, id string) (*model.RegistrationView, error)
	ListRegistrations(ctx context.Context, arenaID string, roundID, cursor *string, limit int) ([]model.RegistrationView, error)
	CreateRegistration(ctx context.Context, arenaID string, req *model.CreateRegistrationRequest) (*model.RegistrationView, error)
	UpdateRegistration(ctx context.Context, arenaID, id string, req *model.CreateRegistrationRequest) (*model.RegistrationView, error)
	TicketAction(ctx context.Context, arenaID, id, action, reason string) (*model.RegistrationView, error)
	UploadProof(ctx context.Context, arenaID, id string, bytes []byte, mime string) error
	ReadProof(ctx context.Context, arenaID, id string) ([]byte, string, error)
	RoundParticipants(ctx context.Context, arenaID, roundID string) ([]model.RegistrationView, error)

	RoundJudges(ctx context.Context, arenaID, roundID string) ([]model.RoundJudgeView, error)
	AssignJudge(ctx context.Context, arenaID, roundID, judgeUserID string, remove bool) error
	SaveScores(ctx context.Context, arenaID, roundID, registrationID, judgeUserID string, scores []model.ScoreItemRequest) error
	ReadScores(ctx context.Context, arenaID, roundID, registrationID string) ([]model.ScoreView, error)
	RoundRecap(ctx context.Context, arenaID, roundID string) ([]model.ScoreSummaryView, error)
}

type Handler struct {
	Comp CompetitionBusiness
}

func result(c *gin.Context, data any, err error) {
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, "success", data)
}

func (h *Handler) Routes(g *gin.RouterGroup) {
	// Events
	g.GET("/arenas/:arena/events", func(c *gin.Context) {
		cursor, limit, ok := request.Page(c)
		if !ok {
			return
		}
		var curPtr *string
		if cursor != "" {
			curPtr = &cursor
		}
		items, err := h.Comp.ListEvents(c.Request.Context(), c.Param("arena"), curPtr, limit)
		if err != nil {
			result(c, nil, err)
			return
		}
		result(c, response.Paginate(items, limit, func(e model.EventView) string { return e.ID }), nil)
	})

	g.GET("/arenas/:arena/events/:id", func(c *gin.Context) {
		v, err := h.Comp.GetEvent(c.Request.Context(), c.Param("arena"), c.Param("id"))
		result(c, v, err)
	})

	g.POST("/arenas/:arena/events", func(c *gin.Context) {
		var req model.CreateEventRequest
		if !request.Bind(c, &req) {
			return
		}
		v, err := h.Comp.CreateEvent(c.Request.Context(), c.Param("arena"), &req)
		result(c, v, err)
	})

	for _, method := range []string{"PATCH", "PUT"} {
		g.Handle(method, "/arenas/:arena/events/:id", func(c *gin.Context) {
			var req model.CreateEventRequest
			if !request.Bind(c, &req) {
				return
			}
			v, err := h.Comp.UpdateEvent(c.Request.Context(), c.Param("arena"), c.Param("id"), &req)
			result(c, v, err)
		})
	}

	g.DELETE("/arenas/:arena/events/:id", func(c *gin.Context) {
		err := h.Comp.DeleteEvent(c.Request.Context(), c.Param("arena"), c.Param("id"))
		result(c, nil, err)
	})

	// Rounds
	g.GET("/arenas/:arena/rounds", func(c *gin.Context) {
		cursor, limit, ok := request.Page(c)
		if !ok {
			return
		}
		var curPtr *string
		if cursor != "" {
			curPtr = &cursor
		}
		items, err := h.Comp.ListRounds(c.Request.Context(), c.Param("arena"), curPtr, limit)
		if err != nil {
			result(c, nil, err)
			return
		}
		result(c, response.Paginate(items, limit, func(r model.RoundView) string { return r.ID }), nil)
	})

	g.GET("/arenas/:arena/rounds/:id", func(c *gin.Context) {
		v, err := h.Comp.GetRound(c.Request.Context(), c.Param("arena"), c.Param("id"))
		result(c, v, err)
	})

	g.POST("/arenas/:arena/rounds", func(c *gin.Context) {
		var req model.CreateRoundRequest
		if !request.Bind(c, &req) {
			return
		}
		v, err := h.Comp.CreateRound(c.Request.Context(), c.Param("arena"), &req)
		result(c, v, err)
	})

	for _, method := range []string{"PATCH", "PUT"} {
		g.Handle(method, "/arenas/:arena/rounds/:id", func(c *gin.Context) {
			var req model.CreateRoundRequest
			if !request.Bind(c, &req) {
				return
			}
			v, err := h.Comp.UpdateRound(c.Request.Context(), c.Param("arena"), c.Param("id"), &req)
			result(c, v, err)
		})
	}

	g.DELETE("/arenas/:arena/rounds/:id", func(c *gin.Context) {
		err := h.Comp.DeleteRound(c.Request.Context(), c.Param("arena"), c.Param("id"))
		result(c, nil, err)
	})

	base := "/arenas/:arena/rounds/:id"
	for _, action := range []string{"start", "begin-judging", "close", "cancel"} {
		g.POST(base+"/"+action, func(c *gin.Context) {
			var body struct {
				Status string `json:"expected_status"`
			}
			if !request.Bind(c, &body) {
				return
			}
			v, err := h.Comp.RoundAction(c.Request.Context(), c.Param("arena"), c.Param("id"), action, body.Status)
			result(c, v, err)
		})
	}

	g.GET(base+"/participants", func(c *gin.Context) {
		v, err := h.Comp.RoundParticipants(c.Request.Context(), c.Param("arena"), c.Param("id"))
		result(c, v, err)
	})

	g.GET(base+"/recap", func(c *gin.Context) {
		v, err := h.Comp.RoundRecap(c.Request.Context(), c.Param("arena"), c.Param("id"))
		result(c, v, err)
	})

	g.GET(base+"/judges", func(c *gin.Context) {
		v, err := h.Comp.RoundJudges(c.Request.Context(), c.Param("arena"), c.Param("id"))
		result(c, v, err)
	})

	for _, method := range []string{"POST", "DELETE"} {
		g.Handle(method, base+"/judges/:judge", func(c *gin.Context) {
			err := h.Comp.AssignJudge(c.Request.Context(), c.Param("arena"), c.Param("id"), c.Param("judge"), c.Request.Method == "DELETE")
			result(c, nil, err)
		})
	}

	g.PUT(base+"/scores/:ticket", func(c *gin.Context) {
		var req struct {
			Scores []model.ScoreItemRequest `json:"scores"`
		}
		if !request.Bind(c, &req) {
			return
		}
		err := h.Comp.SaveScores(c.Request.Context(), c.Param("arena"), c.Param("id"), c.Param("ticket"), request.Actor(c), req.Scores)
		result(c, nil, err)
	})

	g.GET(base+"/scores/:ticket", func(c *gin.Context) {
		v, err := h.Comp.ReadScores(c.Request.Context(), c.Param("arena"), c.Param("id"), c.Param("ticket"))
		result(c, v, err)
	})

	// Registrations
	g.GET("/registrations", func(c *gin.Context) {
		cursor, limit, ok := request.Page(c)
		if !ok {
			return
		}
		var curPtr, roundPtr *string
		if cursor != "" {
			curPtr = &cursor
		}
		if r := c.Query("round_id"); r != "" {
			roundPtr = &r
		}
		arenaID := c.Query("arena_id")
		items, err := h.Comp.ListRegistrations(c.Request.Context(), arenaID, roundPtr, curPtr, limit)
		if err != nil {
			result(c, nil, err)
			return
		}
		result(c, response.Paginate(items, limit, func(reg model.RegistrationView) string { return reg.ID }), nil)
	})

	g.GET("/registrations/:id", func(c *gin.Context) {
		v, err := h.Comp.GetRegistration(c.Request.Context(), c.Query("arena_id"), c.Param("id"))
		result(c, v, err)
	})

	g.POST("/registrations", func(c *gin.Context) {
		var req model.CreateRegistrationRequest
		if !request.Bind(c, &req) {
			return
		}
		arenaID := c.Query("arena_id")
		v, err := h.Comp.CreateRegistration(c.Request.Context(), arenaID, &req)
		result(c, v, err)
	})

	for _, method := range []string{"PATCH", "PUT"} {
		g.Handle(method, "/registrations/:id", func(c *gin.Context) {
			var req model.CreateRegistrationRequest
			if !request.Bind(c, &req) {
				return
			}
			arenaID := c.Query("arena_id")
			v, err := h.Comp.UpdateRegistration(c.Request.Context(), arenaID, c.Param("id"), &req)
			result(c, v, err)
		})
	}

	for _, action := range []string{"verify", "reject", "cancel", "disqualify", "requalify"} {
		g.POST("/registrations/:id/"+action, func(c *gin.Context) {
			var body struct {
				Reason string `json:"reason"`
			}
			if !request.Bind(c, &body) {
				return
			}
			v, err := h.Comp.TicketAction(c.Request.Context(), c.Query("arena_id"), c.Param("id"), action, body.Reason)
			result(c, v, err)
		})
	}

	g.POST("/registrations/:id/proof", h.uploadProof)

	g.GET("/registrations/:id/proof", func(c *gin.Context) {
		b, m, err := h.Comp.ReadProof(c.Request.Context(), c.Query("arena_id"), c.Param("id"))
		if err != nil {
			result(c, nil, err)
			return
		}
		c.Header("Cache-Control", "private, no-store")
		c.Header("Content-Disposition", "inline")
		c.Data(200, m, b)
	})
}

func (h *Handler) uploadProof(c *gin.Context) {
	if err := c.Request.ParseMultipartForm(6 << 20); err != nil {
		result(c, nil, apperror.ErrInvalid)
		return
	}
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	f, _, err := c.Request.FormFile("file")
	if err != nil {
		result(c, nil, apperror.ErrInvalid)
		return
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, (5<<20)+1))
	if err != nil || len(b) > 5<<20 {
		result(c, nil, apperror.Public(apperror.ErrInvalid, "Gambar maksimal 5 MiB"))
		return
	}
	mime := http.DetectContentType(b)
	_, format, err := image.DecodeConfig(bytes.NewReader(b))
	if err != nil || (format != "png" && format != "jpeg") || (mime != "image/png" && mime != "image/jpeg") {
		result(c, nil, apperror.Public(apperror.ErrInvalid, "Signature gambar harus JPEG atau PNG"))
		return
	}
	err = h.Comp.UploadProof(c.Request.Context(), c.Query("arena_id"), c.Param("id"), b, mime)
	result(c, map[string]string{"status": "uploaded"}, err)
}
