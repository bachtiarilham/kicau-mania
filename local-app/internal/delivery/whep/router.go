package whep

import "github.com/gin-gonic/gin"

// RegisterRoutes receives the playback group after its origin middleware is installed.
func RegisterRoutes(group *gin.RouterGroup, handler *Handler) {
	group.POST("/:channel/whep", handler.create)
	group.PATCH("/:channel/whep/:session", handler.patch)
	group.DELETE("/:channel/whep/:session", handler.remove)
}

// create godoc
// @Summary Buka sesi playback WebRTC melalui WHEP
// @Description Kirim SDP offer dari RTCPeerConnection; response SDP answer, bukan JSON. Mode LAN: tanpa tiket, subnet pembaca saja. Mode cloud: Authorization Bearer tiket playback cloud (bukan API_TOKEN). Harus ada sesi media aktif untuk channel. Location menunjuk resource WHEP lanjutan pada gateway lokal. Swagger tidak membuat SDP/ICE atau memutar video.
// @Tags WHEP
// @Accept application/sdp
// @Produce application/sdp
// @Param channel path string true "ID channel"
// @Security PlaybackTicket
// @Param offer body string true "SDP offer dari browser, maksimal 128 KiB"
// @Success 201 {string} string "SDP answer"
// @Header 201 {string} Location "Resource gateway untuk PATCH/DELETE"
// @Header 201 {string} ETag "ETag sesi upstream"
// @Failure 400,401,403,404,409,413,415,502,503 "Request ditolak; error dapat berupa JSON, text atau tanpa body sesuai gateway/upstream"
// @Router /api/v1/playback/{channel}/whep [post]
func (h *Handler) create(c *gin.Context) { h.Serve(c) }

// patch godoc
// @Summary Teruskan kandidat ICE pada sesi WHEP
// @Description Gunakan URL Location dan ETag dari POST. Diteruskan ke MediaMTX; dukungan PATCH/status mengikuti upstream. Tidak menerbitkan tiket atau sesi baru. Mode LAN tetap dibatasi subnet pembaca. Path sesi adalah capability; jangan dibagikan.
// @Tags WHEP
// @Accept application/trickle-ice-sdpfrag
// @Param channel path string true "ID channel"
// @Param session path string true "UUID resource dari Location"
// @Param If-Match header string false "ETag sesi, jika diminta upstream"
// @Param fragment body string true "SDP fragment, maksimal 128 KiB"
// @Success 204 "Update diterima upstream, tanpa envelope JSON"
// @Failure 400,403,404,405,412,413,415,422,501,502,503 "Status gateway/upstream, dapat tanpa body JSON"
// @Router /api/v1/playback/{channel}/whep/{session} [patch]
func (h *Handler) patch(c *gin.Context) { h.Serve(c) }

// remove godoc
// @Summary Tutup satu koneksi playback WHEP
// @Description Gunakan URL Location dari POST. Menutup viewer ini saja; bukan Stop sesi media engine. Mode LAN tetap dibatasi subnet pembaca. Gunakan status upstream, bukan envelope REST.
// @Tags WHEP
// @Param channel path string true "ID channel"
// @Param session path string true "UUID resource dari Location"
// @Success 200 "Sesi ditutup oleh upstream"
// @Success 204 "Sesi ditutup, tanpa body"
// @Failure 400,403,404,502,503 "Status gateway/upstream, dapat tanpa body JSON"
// @Router /api/v1/playback/{channel}/whep/{session} [delete]
func (h *Handler) remove(c *gin.Context) { h.Serve(c) }
