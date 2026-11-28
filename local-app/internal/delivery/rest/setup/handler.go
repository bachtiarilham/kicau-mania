package setup

import (
	"context"
	"net/url"

	"github.com/gin-gonic/gin"
	"kicau.local/edge/config"
	"kicau.local/edge/internal/delivery/rest/response"
	"kicau.local/edge/internal/infra/network"
)

type Diagnostic struct {
	Component string `json:"component,omitempty"`
	Code      string `json:"code"`
	Message   string `json:"message,omitempty"`
	Advice    string `json:"advice,omitempty"`
}

type ViewChannel struct {
	ID             string `json:"id"`
	IP             string `json:"ip,omitempty"`
	Source         string `json:"source"`
	Codec          string `json:"codec,omitempty"`
	HasAudio       bool   `json:"has_audio"`
	Width          int    `json:"width,omitempty"`
	Height         int    `json:"height,omitempty"`
	HasCredentials bool   `json:"has_credentials,omitempty"`
}

type Configuration struct {
	Version  int           `json:"version"`
	LANIP    string        `json:"lan_ip"`
	LANCIDR  string        `json:"lan_cidr"`
	NVRIP    string        `json:"nvr_ip,omitempty"`
	NVRMAC   string        `json:"nvr_mac,omitempty"`
	Channels []ViewChannel `json:"channels"`
}

type RuntimeStatus struct {
	ChecksJobID    string        `json:"checks_job_id,omitempty"`
	SourceChecks   any           `json:"source_checks,omitempty"`
	CapacityNotice string        `json:"capacity_notice,omitempty"`
	Advice         string        `json:"advice,omitempty"`
	Channels       []ViewChannel `json:"channels"`
	Configured     bool          `json:"configured"`
	Ready          bool          `json:"ready"`
	ErrorCode      string        `json:"error_code,omitempty"`
	Session        any           `json:"session,omitempty"`
	Targets        []string      `json:"targets"`
	APIURLs        []string      `json:"api_urls"`
}

type JobRequest struct {
	Action string `json:"action"`
}

type Job struct {
	ID       string `json:"id"`
	State    string `json:"state"`
	Progress string `json:"progress"`
	Result   any    `json:"result,omitempty"`
	Code     string `json:"code,omitempty"`
	Message  string `json:"message,omitempty"`
}

type SystemInfo interface {
	Guidance(string) Diagnostic
	Status(context.Context) RuntimeStatus
	Networks() []network.Network
	Check(context.Context) []Diagnostic
	Configuration(context.Context) (Configuration, error)
	Start(JobRequest) (Job, error)
	Job(string) (Job, error)
	Cancel(string) error
	Arena(context.Context) (config.ArenaSettings, error)
	SaveArena(config.ArenaSettings) error
	TestArenaCamera(context.Context, config.ArenaChannel) (any, error)
	TestArenaNVR(context.Context, string) error
}

type Handler struct {
	sys   SystemInfo
	nonce string
}

func New(sys SystemInfo, nonce string) *Handler {
	return &Handler{sys: sys, nonce: nonce}
}

func (h *Handler) Routes(g *gin.RouterGroup) {
	g.GET("/guidance/:code", h.Guidance)
	g.GET("/bootstrap", h.bootstrap)
	g.GET("/checks", h.checks)
	g.GET("/configuration", h.configuration)
	g.GET("/arena", h.arena)
	g.POST("/arena", h.saveArena)
	g.POST("/arena/test-camera", h.testArenaCamera)
	g.POST("/arena/test-nvr", h.testArenaNVR)
	g.POST("/jobs", h.startJob)
	g.GET("/jobs/:id", h.job)
	g.DELETE("/jobs/:id", h.cancelJob)
}

func (h *Handler) arena(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	settings, err := h.sys.Arena(c.Request.Context())
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, "Pengaturan arena", settings)
}

func (h *Handler) saveArena(c *gin.Context) {
	var settings config.ArenaSettings
	if c.ShouldBindJSON(&settings) != nil {
		response.BadRequest(c, "Pengaturan arena tidak valid", nil)
		return
	}
	if err := h.sys.SaveArena(settings); err != nil {
		response.WriteError(c, err)
		return
	}
	response.NoContent(c)
}

func (h *Handler) testArenaCamera(c *gin.Context) {
	var camera config.ArenaChannel
	if c.ShouldBindJSON(&camera) != nil {
		response.BadRequest(c, "Sumber RTSP tidak valid", nil)
		return
	}
	stream, err := h.sys.TestArenaCamera(c.Request.Context(), camera)
	if err != nil {
		response.WriteError(c, err)
		return
	}
	response.Success(c, "Video tersedia", stream)
}

func (h *Handler) testArenaNVR(c *gin.Context) {
	var target struct {
		IP string `json:"ip" binding:"required,ipv4"`
	}
	if c.ShouldBindJSON(&target) != nil {
		response.BadRequest(c, "Masukkan IPv4 NVR yang valid", nil)
		return
	}
	if err := h.sys.TestArenaNVR(c.Request.Context(), target.IP); err != nil {
		response.WriteError(c, err)
		return
	}
	response.NoContent(c)
}

func (h *Handler) bootstrap(c *gin.Context) {
	response.Success(c, "Local control", gin.H{"nonce": h.nonce, "networks": h.sys.Networks()})
}

func (h *Handler) checks(c *gin.Context) {
	response.Success(c, "Application checks", h.sys.Check(c.Request.Context()))
}

func (h *Handler) configuration(c *gin.Context) {
	v, e := h.sys.Configuration(c.Request.Context())
	if e != nil {
		response.WriteError(c, e)
		return
	}
	response.Success(c, "Configuration", v)
}

func (h *Handler) startJob(c *gin.Context) {
	var in JobRequest
	if c.ShouldBindJSON(&in) != nil {
		response.BadRequest(c, "invalid request syntax", nil)
		return
	}
	v, e := h.sys.Start(in)
	if e != nil {
		response.WriteError(c, e)
		return
	}
	response.Accepted(c, "Setup job accepted", v)
}

func (h *Handler) job(c *gin.Context) {
	v, e := h.sys.Job(c.Param("id"))
	if e != nil {
		response.WriteError(c, e)
		return
	}
	response.Success(c, "Setup job", v)
}

func (h *Handler) cancelJob(c *gin.Context) {
	if e := h.sys.Cancel(c.Param("id")); e != nil {
		response.WriteError(c, e)
		return
	}
	c.Status(204)
}

func (h *Handler) Status(c *gin.Context) {
	response.Success(c, "Local runtime status", h.sys.Status(c.Request.Context()))
}

func (h *Handler) Guidance(c *gin.Context) {
	response.Success(c, "Diagnostic guidance", h.sys.Guidance(c.Param("code")))
}

func Advice(code string) string {
	switch code {
	case "MEDIA_UNAVAILABLE":
		return "MediaMTX belum berjalan. Periksa binary, port RTSP internal dan proses lain; API tetap tersedia."
	case "MAX_DURATION":
		return "Batas durasi sesi tercapai. Tunggu finalisasi sebelum memulai sesi berikutnya."
	case "DISK_LOW", "DISK_UNAVAILABLE":
		return "Periksa ruang disk dan izin folder rekaman; hentikan sesi dan tunggu finalisasi sebelum mengelola file."
	case "DISK_FULL":
		return "Stop sesi, periksa kapasitas disk dan upload; jangan menghapus manifest/segmen yang masih aktif."
	case "SOURCE_INTERRUPTED":
		return "Engine mencoba ulang channel setiap 5 detik. Periksa sumber/jaringan; channel lain tetap berjalan."
	case "BINARY_START_FAILED":
		return "Periksa lokasi binary, izin eksekusi dan batas proses pada komputer engine."
	case "RECORDING_FINALIZATION_FAILED":
		return "Simpan segmen untuk pemeriksaan dan lihat alasan validasi; jangan mengunggah file yang belum READY."
	case "SESSION_ACTIVE":
		return "Stop seluruh media dan tunggu status terminal sebelum mengubah konfigurasi."
	case "BINARY_MISSING", "ENCODER_MISSING":
		return "Periksa paket FFmpeg, ffprobe dan MediaMTX; codec sumber diteruskan tanpa encode ulang."
	case "SOURCE_UNREACHABLE", "DEVICE_UNREACHABLE", "TIMEOUT":
		return "Periksa IP, port, path stream, kabel dan firewall; lalu jalankan tes koneksi kembali."
	case "NETWORK_SCOPE", "NETWORK_UNAVAILABLE":
		return "Pilih interface LAN aktif pada subnet yang sama dengan perangkat."
	case "UNSUPPORTED_VIDEO_CODEC":
		return "Pilih profil NVR HEVC/H.265. Codec lain ditolak; konversi tidak tersedia."
	case "CONFIG_APPLY_FAILED", "ENGINE_START_FAILED":
		return "Periksa binary, port internal dan izin folder data; API tetap dapat dipakai untuk konfigurasi ulang."
	default:
		return "Periksa status engine dan hasil pemeriksaan aplikasi. Jangan membagikan kredensial atau log proses mentah."
	}
}

func MaskSource(raw string) (string, bool) {
	u, err := url.Parse(raw)
	if err != nil {
		return raw, false
	}
	hasCreds := u.User != nil || u.RawQuery != ""
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), hasCreds
}
