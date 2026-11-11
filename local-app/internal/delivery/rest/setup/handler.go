// Package setup parses local setup requests and maps diagnostic results to HTTP.
package setup

import (
	"context"
	"github.com/gin-gonic/gin"
	"kicau.local/edge/internal/delivery/rest/response"
	"kicau.local/edge/internal/setup/model"
)

type UseCase interface {
	Guidance(string) model.Diagnostic
	Status(context.Context) model.RuntimeStatus
	Networks() []model.Network
	Check(context.Context) []model.Diagnostic
	Configuration(context.Context) (model.Configuration, error)
	Start(model.JobRequest) (model.Job, error)
	Job(string) (model.Job, error)
	Cancel(string) error
}
type Handler struct {
	uc    UseCase
	nonce string
}

func New(u UseCase, nonce string) *Handler { return &Handler{u, nonce} }
func (h *Handler) Routes(g *gin.RouterGroup) {
	g.GET("/guidance/:code", h.Guidance)
	g.GET("/bootstrap", h.bootstrap)
	g.GET("/checks", h.checks)
	g.GET("/configuration", h.configuration)
	g.POST("/jobs", h.startJob)
	g.GET("/jobs/:id", h.job)
	g.DELETE("/jobs/:id", h.cancelJob)
}

// BootstrapResponse contains a per-process local control nonce and available IPv4 networks.
type BootstrapResponse struct {
	Nonce    string          `json:"nonce"`
	Networks []model.Network `json:"networks"`
}

// bootstrap godoc
// @Summary Ambil nonce kontrol lokal dan interface LAN
// @Description Mode lan, localhost pada PC engine saja. Gunakan data.nonce pada Authorize LocalControl untuk request mutasi berikutnya; nonce bukan API_TOKEN dan tidak diberikan kepada penonton LAN.
// @Tags Setup
// @Produce json
// @Success 200 {object} response.Response{data=BootstrapResponse}
// @Failure 403 {object} response.Response
// @Router /api/v1/setup/bootstrap [get]
func (h *Handler) bootstrap(c *gin.Context) {
	response.Success(c, "Local control", gin.H{"nonce": h.nonce, "networks": h.uc.Networks()})
}

// checks godoc
// @Summary Periksa binary dan encoder aplikasi
// @Description Mode lan, localhost saja. Memeriksa FFmpeg, ffprobe, MediaMTX dan encoder; hasil pemeriksaan tidak otomatis memulai sesi.
// @Tags Setup
// @Produce json
// @Success 200 {object} response.Response{data=[]model.Diagnostic}
// @Failure 403 {object} response.Response
// @Router /api/v1/setup/checks [get]
func (h *Handler) checks(c *gin.Context) {
	response.Success(c, "Application checks", h.uc.Check(c.Request.Context()))
}

// configuration godoc
// @Summary Baca konfigurasi sumber yang disamarkan
// @Description Mode lan, localhost saja. Username, password, credential_ref, userinfo dan query RTSP tidak dikembalikan. Probe dan penyimpanan melalui jobs action apply.
// @Tags Setup
// @Produce json
// @Success 200 {object} response.Response{data=model.Configuration}
// @Failure 403,404,503 {object} response.Response
// @Router /api/v1/setup/configuration [get]
func (h *Handler) configuration(c *gin.Context) {
	v, e := h.uc.Configuration(c.Request.Context())
	if e != nil {
		response.WriteError(c, e)
		return
	}
	response.Success(c, "Configuration", v)
}

// startJob godoc
// @Summary Jalankan discovery, profil, probe, apply atau recheck
// @Description Mode lan dan localhost dengan X-Local-Control. action=discover memakai ip interface; profiles memakai endpoint/username/password; probe memakai channel; apply memakai configuration (LAN IP/CIDR dan 1-24 channel, live_mode copy atau transcode); recheck memeriksa sumber tersimpan. Lihat docs/setup-api.md untuk payload tiap action. Poll jobs/{id}: HTTP 202 hanya berarti diterima, hasil akhir ada di state/result/code/advice. Apply tidak boleh saat sesi aktif atau finalisasi, dan tidak memulai sesi. Result: discover=Device[], profiles=Profile[], probe=Probe, apply=Configuration tersamarkan, recheck=SourceCheck[].
// @Tags Setup
// @Accept json
// @Produce json
// @Security LocalControl
// @Param request body model.JobRequest true "Isi hanya field yang diperlukan oleh action"
// @Success 202 {object} response.Response{data=model.Job}
// @Failure 400,403,409,503 {object} response.Response
// @Router /api/v1/setup/jobs [post]
func (h *Handler) startJob(c *gin.Context) {
	var in model.JobRequest
	if c.ShouldBindJSON(&in) != nil {
		response.BadRequest(c, "invalid request syntax", nil)
		return
	}
	v, e := h.uc.Start(in)
	if e != nil {
		response.WriteError(c, e)
		return
	}
	response.Accepted(c, "Setup job accepted", v)
}

// job godoc
// @Summary Baca progres dan hasil pekerjaan setup
// @Description Mode lan, localhost saja. State RUNNING, SUCCEEDED atau FAILED; result sesuai action. Job dan credential_ref bersifat sementara, bukan konfigurasi persisten.
// @Tags Setup
// @Produce json
// @Param id path string true "ID job dari request start"
// @Success 200 {object} response.Response{data=model.Job}
// @Failure 403,404 {object} response.Response
// @Router /api/v1/setup/jobs/{id} [get]
func (h *Handler) job(c *gin.Context) {
	v, e := h.uc.Job(c.Param("id"))
	if e != nil {
		response.WriteError(c, e)
		return
	}
	response.Success(c, "Setup job", v)
}

// cancelJob godoc
// @Summary Minta pembatalan pekerjaan setup
// @Description Mode lan dan localhost dengan nonce. Pembatalan bersifat kooperatif; baca ulang status job untuk hasil akhir.
// @Tags Setup
// @Security LocalControl
// @Param id path string true "ID job"
// @Success 204 "Pembatalan diterima, tanpa body"
// @Failure 403,404 {object} response.Response
// @Router /api/v1/setup/jobs/{id} [delete]
func (h *Handler) cancelJob(c *gin.Context) {
	if e := h.uc.Cancel(c.Param("id")); e != nil {
		response.WriteError(c, e)
		return
	}
	c.Status(204)
}

// Status godoc
// @Summary Baca status lokal, codec, sesi dan diagnostik
// @Description Mode lan, dibatasi MEDIA_READ_CIDRS. Tersedia sebelum konfigurasi siap. Session dan source_checks dapat belum tersedia. Tidak memulai media.
// @Tags Local
// @Produce json
// @Success 200 {object} response.Response{data=model.RuntimeStatus}
// @Failure 403 {object} response.Response
// @Router /api/v1/local/status [get]
func (h *Handler) Status(c *gin.Context) {
	response.Success(c, "Local runtime status", h.uc.Status(c.Request.Context()))
}

// Guidance godoc
// @Summary Baca panduan penanganan kode gangguan
// @Description Jalur setup hanya localhost; jalur local tersedia untuk subnet pembaca dalam mode lan.
// @Tags Local
// @Produce json
// @Param code path string true "Kode seperti SOURCE_UNREACHABLE, DISK_LOW, UNSUPPORTED_CODEC"
// @Success 200 {object} response.Response{data=model.Diagnostic}
// @Failure 403 {object} response.Response
// @Router /api/v1/local/guidance/{code} [get]
// @Router /api/v1/setup/guidance/{code} [get]
func (h *Handler) Guidance(c *gin.Context) {
	response.Success(c, "Diagnostic guidance", h.uc.Guidance(c.Param("code")))
}
