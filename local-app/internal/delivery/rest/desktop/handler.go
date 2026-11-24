package desktop

import (
	"context"
	"github.com/gin-gonic/gin"
	"kicau.local/edge/internal/apperror"
	bridge "kicau.local/edge/internal/cloud/model"
	"kicau.local/edge/internal/delivery/rest/response"
	"net/http"
	"os"
	"path/filepath"
)

type Cloud interface {
	Configure(bridge.Pairing) error
	Status() map[string]any
	Bind(bridge.Ingestion) error
	Progress(string) bridge.Progress
	Retry(string) error
}
type Saving interface {
	RecordingFile(context.Context, string) (string, error)
	RetryRecording(context.Context, string) error
}
type Handler struct {
	Cloud  Cloud
	Saving Saving
}

func result(c *gin.Context, e error) {
	if e != nil {
		response.WriteError(c, e)
		return
	}
	response.Success(c, "Berhasil", nil)
}
func (h *Handler) Routes(g *gin.RouterGroup) {
	g.GET("/desktop/cloud", h.cloudStatus)
	g.POST("/desktop/cloud", h.configureCloud)
	g.POST("/desktop/mappings", h.bindMapping)
	g.GET("/recordings/:id/upload", h.uploadStatus)
	g.POST("/recordings/:id/retry", h.retryUpload)
	g.GET("/recordings/:id/file", h.recordingFile)
}

// cloudStatus godoc
// @Summary Status pairing engine
// @Tags Desktop media
// @Security LocalControl
// @Produce json
// @Success 200 {object} response.Response
// @Failure 400,403,404,409,503 {object} response.Response
// @Router /api/v1/desktop/cloud [get]
func (h *Handler) cloudStatus(c *gin.Context) { response.Success(c, "Status cloud", h.Cloud.Status()) }

// configureCloud godoc
// @Summary Simpan pairing engine
// @Tags Desktop media
// @Security LocalControl
// @Produce json
// @Accept json
// @Param body body bridge.Pairing true "Payload engine"
// @Success 200 {object} response.Response
// @Failure 400,403,404,409,503 {object} response.Response
// @Router /api/v1/desktop/cloud [post]
func (h *Handler) configureCloud(c *gin.Context) {
	var p bridge.Pairing
	if c.ShouldBindJSON(&p) != nil {
		result(c, apperror.ErrInvalid)
		return
	}
	result(c, h.Cloud.Configure(p))
}

// bindMapping godoc
// @Summary Simpan mapping yang sudah dikonfirmasi cloud
// @Tags Desktop media
// @Security LocalControl
// @Produce json
// @Accept json
// @Param body body bridge.Ingestion true "Payload engine"
// @Success 200 {object} response.Response
// @Failure 400,403,404,409,503 {object} response.Response
// @Router /api/v1/desktop/mappings [post]
func (h *Handler) bindMapping(c *gin.Context) {
	var m bridge.Ingestion
	if c.ShouldBindJSON(&m) != nil {
		result(c, apperror.ErrInvalid)
		return
	}
	result(c, h.Cloud.Bind(m))
}

// uploadStatus godoc
// @Summary Status upload MKV
// @Tags Desktop media
// @Security LocalControl
// @Produce json
// @Param id path string true "ID recording"
// @Success 200 {object} response.Response
// @Failure 400,403,404,409,503 {object} response.Response
// @Router /api/v1/recordings/{id}/upload [get]
func (h *Handler) uploadStatus(c *gin.Context) {
	response.Success(c, "Status upload", h.Cloud.Progress(c.Param("id")))
}

// retryUpload godoc
// @Summary Coba ulang upload MKV
// @Tags Desktop media
// @Security LocalControl
// @Produce json
// @Param id path string true "ID recording"
// @Success 200 {object} response.Response
// @Failure 400,403,404,409,503 {object} response.Response
// @Router /api/v1/recordings/{id}/retry [post]
func (h *Handler) retryUpload(c *gin.Context) {
	if h.Saving == nil {
		result(c, apperror.ErrUnavailable)
		return
	}
	if e := h.Cloud.Retry(c.Param("id")); e != nil {
		result(c, e)
		return
	}
	result(c, h.Saving.RetryRecording(c.Request.Context(), c.Param("id")))
}

// recordingFile godoc
// @Summary Baca rekaman MKV terotorisasi
// @Tags Desktop media
// @Security LocalControl
// @Produce video/x-matroska
// @Param id path string true "ID recording"
// @Success 200 {file} file
// @Success 206 {file} file
// @Failure 400,403,404,409,503 {object} response.Response
// @Router /api/v1/recordings/{id}/file [get]
func (h *Handler) recordingFile(c *gin.Context) {
	if h.Saving == nil {
		result(c, apperror.ErrUnavailable)
		return
	}
	file, e := h.Saving.RecordingFile(c.Request.Context(), c.Param("id"))
	if e != nil {
		result(c, e)
		return
	}
	f, e := os.Open(file)
	if e != nil {
		result(c, apperror.ErrNotFound)
		return
	}
	defer f.Close()
	s, e := f.Stat()
	if e != nil {
		result(c, apperror.ErrUnavailable)
		return
	}
	name := "recording" + filepath.Ext(file)
	contentType := "video/x-matroska"
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", "inline; filename="+name)
	http.ServeContent(c.Writer, c.Request, name, s.ModTime(), f)
}
