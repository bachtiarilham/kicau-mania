package competition

import (
	"context"

	"github.com/gin-gonic/gin"

	input "kicau.cloud/control/internal/competition/model"
	"kicau.cloud/control/internal/core/model"
	"kicau.cloud/control/internal/delivery/rest/request"
	"kicau.cloud/control/internal/delivery/rest/response"
)

type UseCase interface {
	Registration(context.Context, string, string) (model.Record, error)
	Registrations(context.Context, string, string, string, int) ([]model.Record, error)
	CreateRegistration(context.Context, string, input.Registration) (model.Record, error)
	UpdateRegistration(context.Context, string, string, input.Registration) (model.Record, error)
	CancelRegistration(context.Context, string, string) (model.Record, error)
	CreatePayment(context.Context, string, string, input.Payment) (model.Record, error)
	VerifyPayment(context.Context, string, string, []input.Allocation) (model.Record, error)
	RejectPayment(context.Context, string, string) (model.Record, error)
	RoundAction(context.Context, string, string, string, string) (model.Record, error)
	Recap(context.Context, string, string) ([]model.Record, error)
	Judges(context.Context, string, string) ([]model.Record, error)
	AssignJudge(context.Context, string, string, string) ([]model.Record, error)
	RemoveJudge(context.Context, string, string, string) error
	Recordings(context.Context, string, string) ([]model.Record, error)
}
type Reports interface {
	Get(context.Context, string, string, string) (model.Record, error)
}
type Handler struct {
	uc      UseCase
	reports Reports
}

func New(u UseCase, r Reports) *Handler { return &Handler{u, r} }
func (h *Handler) Routes(g *gin.RouterGroup) {
	// @Summary Daftar registrasi
	// @Tags Competition
	// @Description Milik sendiri atau scope petugas.
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
	// @Router /api/v1/registrations [get]
	g.GET("/registrations", func(c *gin.Context) {
		cursor, limit, ok := request.Page(c)
		if !ok {
			return
		}
		rows, e := h.uc.Registrations(c.Request.Context(), request.Actor(c), c.Query("arena_id"), cursor, limit)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Registrations retrieved", request.PageData(rows, limit))
	})
	// @Summary Registrasi beberapa ronde
	// @Tags Competition
	// @Description Harga dihitung server. Ronde PREPARED, tipe burung cocok. ONLINE terikat akun login; MANUAL hanya petugas. Satu burung boleh ronde berbeda; satu keikutsertaan aktif per burung/ronde.
	// @Security BearerAuth
	// @Accept json
	// @Param request body input.Registration true "JSON body"
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
	// @Router /api/v1/registrations [post]
	g.POST("/registrations", func(c *gin.Context) {
		var in input.Registration
		if !request.Bind(c, &in) {
			return
		}
		row, e := h.uc.CreateRegistration(c.Request.Context(), request.Actor(c), in)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Created(c, "Registration created", row)
	})
	// @Summary Detail registrasi
	// @Tags Competition
	// @Description Hak akses dan validasi bisnis diperiksa oleh UseCase.
	// @Security BearerAuth
	// @Produce json
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
	// @Router /api/v1/registrations/{id} [get]
	g.GET("/registrations/:id", func(c *gin.Context) {
		row, e := h.uc.Registration(c.Request.Context(), request.Actor(c), c.Param("id"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Registration retrieved", row)
	})
	// @Summary Ganti item/kontak sebelum pembayaran
	// @Tags Competition
	// @Description Daftar item lengkap; arena/source tetap. Hanya sebelum pembayaran diajukan.
	// @Security BearerAuth
	// @Accept json
	// @Param request body input.Registration true "JSON body"
	// @Produce json
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
	// @Router /api/v1/registrations/{id} [put]
	g.PUT("/registrations/:id", func(c *gin.Context) {
		var in input.Registration
		if !request.Bind(c, &in) {
			return
		}
		row, e := h.uc.UpdateRegistration(c.Request.Context(), request.Actor(c), c.Param("id"), in)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Registration updated", row)
	})
	// @Summary Batalkan registrasi pending
	// @Tags Competition
	// @Description Sebelum konfirmasi; lepas reservasi item. Refund belum tersedia.
	// @Security BearerAuth
	// @Produce json
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
	// @Router /api/v1/registrations/{id}/cancel [post]
	g.POST("/registrations/:id/cancel", func(c *gin.Context) {
		row, e := h.uc.CancelRegistration(c.Request.Context(), request.Actor(c), c.Param("id"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Registration cancelled", row)
	})
	// @Summary Ajukan pembayaran
	// @Tags Competition
	// @Description Nominal tepat total registrasi. TRANSFER perlu proof_reference; CASH hanya petugas.
	// @Security BearerAuth
	// @Accept json
	// @Param request body input.Payment true "JSON body"
	// @Produce json
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 201 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/registrations/{id}/payments [post]
	g.POST("/registrations/:id/payments", func(c *gin.Context) {
		var in input.Payment
		if !request.Bind(c, &in) {
			return
		}
		row, e := h.uc.CreatePayment(c.Request.Context(), request.Actor(c), c.Param("id"), in)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Created(c, "Payment submitted", row)
	})
	// @Summary Verifikasi pembayaran dan alokasi gantangan
	// @Tags Competition
	// @Description Petugas; ronde PREPARED, seluruh item dialokasikan. Pengulangan identik idempoten; alokasi berbeda 409.
	// @Security BearerAuth
	// @Accept json
	// @Param request body VerifyPayment true "JSON body"
	// @Produce json
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
	// @Router /api/v1/payments/{id}/verify [post]
	g.POST("/payments/:id/verify", func(c *gin.Context) {
		var in VerifyPayment
		if !request.Bind(c, &in) {
			return
		}
		row, e := h.uc.VerifyPayment(c.Request.Context(), request.Actor(c), c.Param("id"), in.Allocations)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Payment verified and participants confirmed", row)
	})
	// @Summary Tolak pembayaran PENDING
	// @Tags Competition
	// @Description Petugas arena.
	// @Security BearerAuth
	// @Produce json
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
	// @Router /api/v1/payments/{id}/reject [post]
	g.POST("/payments/:id/reject", func(c *gin.Context) {
		row, e := h.uc.RejectPayment(c.Request.Context(), request.Actor(c), c.Param("id"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Payment rejected", row)
	})
	// @Summary Ronde: start
	// @Tags Competition
	// @Description Petugas arena. PREPARED → RUNNING; perlu peserta, juri dan kriteria.
	// @Security BearerAuth
	// @Produce json
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
	// @Router /api/v1/rounds/{id}/start [post]

	// @Summary Ronde: begin-judging
	// @Tags Competition
	// @Description Petugas arena. RUNNING → JUDGING.
	// @Security BearerAuth
	// @Produce json
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
	// @Router /api/v1/rounds/{id}/begin-judging [post]

	// @Summary Ronde: interrupt
	// @Tags Competition
	// @Description Petugas arena. RUNNING/JUDGING → INTERRUPTED.
	// @Security BearerAuth
	// @Produce json
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
	// @Router /api/v1/rounds/{id}/interrupt [post]

	// @Summary Ronde: resume
	// @Tags Competition
	// @Description Petugas arena. INTERRUPTED → fase asal.
	// @Security BearerAuth
	// @Produce json
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
	// @Router /api/v1/rounds/{id}/resume [post]

	// @Summary Ronde: cancel
	// @Tags Competition
	// @Description Petugas arena. Belum CLOSED → CANCELLED; tanpa pemenang.
	// @Security BearerAuth
	// @Produce json
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
	// @Router /api/v1/rounds/{id}/cancel [post]

	// @Summary Ronde: close
	// @Tags Competition
	// @Description Petugas arena. JUDGING → CLOSED. Semua nilai wajib lengkap; operator memilih kandidat tertinggi sebelum pembulatan. Seri memilih satu kandidat tertinggi. Nilai/pemenang terkunci. Bracket manual di luar aplikasi.
	// @Security BearerAuth
	// @Accept json
	// @Param request body CloseRound true "JSON body"
	// @Produce json
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
	// @Router /api/v1/rounds/{id}/close [post]
	for _, action := range []string{"start", "begin-judging", "interrupt", "resume", "cancel", "close"} {
		action := action
		g.POST("/rounds/:id/"+action, func(c *gin.Context) {
			winner := ""
			if action == "close" {
				var in CloseRound
				if !request.Bind(c, &in) {
					return
				}
				winner = in.WinnerRegistrationItemID
			}
			row, e := h.uc.RoundAction(c.Request.Context(), request.Actor(c), c.Param("id"), action, winner)
			if e != nil {
				response.WriteError(c, e)
				return
			}
			response.Success(c, "Round updated", row)
		})
	}
	// @Summary Rekap langsung untuk keputusan pemenang
	// @Tags Competition
	// @Description Petugas arena. Juri dan kriteria berbobot sama; sebelum pembulatan tampilan.
	// @Security BearerAuth
	// @Produce json
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=[]model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/rounds/{id}/recap [get]
	g.GET("/rounds/:id/recap", func(c *gin.Context) {
		rows, e := h.uc.Recap(c.Request.Context(), request.Actor(c), c.Param("id"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Live recap retrieved", rows)
	})
	// @Summary Roster juri ronde
	// @Tags Competition
	// @Description Hak akses dan validasi bisnis diperiksa oleh UseCase.
	// @Security BearerAuth
	// @Produce json
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=[]model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/rounds/{id}/judges [get]
	g.GET("/rounds/:id/judges", func(c *gin.Context) {
		rows, e := h.uc.Judges(c.Request.Context(), request.Actor(c), c.Param("id"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Judges retrieved", rows)
	})
	// @Summary Tambah juri roster
	// @Tags Competition
	// @Description Petugas; sebelum start. Juri ditugaskan pada arena sama.
	// @Security BearerAuth
	// @Accept json
	// @Param request body JudgeAssignment true "JSON body"
	// @Produce json
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 201 {object} response.Response{data=[]model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/rounds/{id}/judges [post]
	g.POST("/rounds/:id/judges", func(c *gin.Context) {
		var in JudgeAssignment
		if !request.Bind(c, &in) {
			return
		}
		rows, e := h.uc.AssignJudge(c.Request.Context(), request.Actor(c), c.Param("id"), in.UserID)
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Created(c, "Judge assigned", rows)
	})
	// @Summary Hapus juri roster
	// @Tags Competition
	// @Description Sebelum start; roster tetap sepanjang ronde.
	// @Security BearerAuth
	// @Produce json
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param assignment path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 204 "No response body"
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/rounds/{id}/judges/{assignment} [delete]
	g.DELETE("/rounds/:id/judges/:assignment", func(c *gin.Context) {
		if e := h.uc.RemoveJudge(c.Request.Context(), request.Actor(c), c.Param("id"), c.Param("assignment")); e != nil {
			response.WriteError(c, e)
			return
		}
		response.NoContent(c)
	})
	// @Summary Dashboard snapshot
	// @Tags Reports
	// @Description Petugas arena/superadmin. Materialized view dapat tertinggal; periksa snapshot_at/refreshed_at. Pengesahan memakai rekap langsung.
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
	// @Router /api/v1/arenas/{arena}/reports/dashboard [get]
	g.GET("/arenas/:arena/reports/dashboard", func(c *gin.Context) {
		row, e := h.reports.Get(c.Request.Context(), request.Actor(c), c.Param("arena"), "")
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Dashboard snapshot retrieved", row)
	})
	// @Summary Snapshot skor ronde
	// @Tags Reports
	// @Description Petugas arena/superadmin. Materialized view dapat tertinggal; periksa snapshot_at/refreshed_at. Pengesahan memakai rekap langsung.
	// @Security BearerAuth
	// @Produce json
	// @Param arena path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Param round path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/arenas/{arena}/reports/rounds/{round} [get]
	g.GET("/arenas/:arena/reports/rounds/:round", func(c *gin.Context) {
		row, e := h.reports.Get(c.Request.Context(), request.Actor(c), c.Param("arena"), c.Param("round"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Score snapshot retrieved", row)
	})
	// @Summary Metadata rekaman registrasi
	// @Tags Competition
	// @Description Kosong jika layanan media belum mengisi metadata. Tidak memulai streaming.
	// @Security BearerAuth
	// @Produce json
	// @Param id path string true "UUID sesuai arena dan hak aktor." Format(uuid)
	// @Success 200 {object} response.Response{data=[]model.Record}
	// @Failure 400 {object} response.Response
	// @Failure 401 {object} response.Response
	// @Failure 403 {object} response.Response
	// @Failure 404 {object} response.Response
	// @Failure 409 {object} response.Response
	// @Failure 500 {object} response.Response
	// @Failure 503 {object} response.Response
	// @Failure 504 {object} response.Response
	// @Router /api/v1/registrations/{id}/recordings [get]
	g.GET("/registrations/:id/recordings", func(c *gin.Context) {
		rows, e := h.uc.Recordings(c.Request.Context(), request.Actor(c), c.Param("id"))
		if e != nil {
			response.WriteError(c, e)
			return
		}
		response.Success(c, "Recording metadata retrieved", rows)
	})
}
