package media

import (
	"context"
	"fmt"
	"html"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"kicau.cloud/control/internal/apperror"
	core "kicau.cloud/control/internal/core/model"
	"kicau.cloud/control/internal/delivery/middleware"
	"kicau.cloud/control/internal/delivery/rest/request"
	"kicau.cloud/control/internal/delivery/rest/response"
	"kicau.cloud/control/internal/media/model"
)

type Business interface {
	Pair(context.Context, string, string, string) (core.Record, error)
	Revoke(context.Context, string, string) error
	Authenticate(context.Context, string) (core.Record, error)
	Ingest(context.Context, string, model.Ingestion) (core.Record, error)
	Create(context.Context, core.Record, model.Upload) (core.Record, error)
	Status(context.Context, core.Record, string) (core.Record, error)
	Sign(context.Context, core.Record, string, model.SignPart) (model.Signed, error)
	Complete(context.Context, core.Record, string) (core.Record, error)
	Read(context.Context, string, string) (core.Record, error)
	List(context.Context, string, string, string, string, string, string, int) ([]core.Record, error)
	Download(context.Context, string, string) (string, error)
	DeliverWhatsApp(context.Context, string, string, string) (*model.WhatsAppDeliveryResult, error)
	BroadcastRoundWhatsApp(context.Context, string, string, string, string) ([]model.WhatsAppDeliveryResult, error)
	PublicWatch(context.Context, string) (*model.PublicVideoWatch, error)
}
type Handler struct{ UC Business }

func send(c *gin.Context, v any, e error) {
	if e != nil {
		response.WriteError(c, e)
		return
	}
	response.Success(c, "Berhasil", v)
}
func (h *Handler) Routes(users, engine *gin.RouterGroup) {
	users.POST("/media/engines/:engine_id", func(c *gin.Context) {
		var b struct {
			Arena string `json:"arena_id"`
		}
		if !request.Bind(c, &b) {
			return
		}
		r, e := h.UC.Pair(c.Request.Context(), request.Actor(c), c.Param("engine_id"), b.Arena)
		send(c, r, e)
	})
	users.DELETE("/media/engines/:engine_id", func(c *gin.Context) {
		send(c, nil, h.UC.Revoke(c.Request.Context(), request.Actor(c), c.Param("engine_id")))
	})
	users.POST("/media/ingestions", func(c *gin.Context) {
		var b model.Ingestion
		if !request.Bind(c, &b) {
			return
		}
		r, e := h.UC.Ingest(c.Request.Context(), request.Actor(c), b)
		send(c, r, e)
	})
	users.GET("/media/recordings", func(c *gin.Context) {
		n, e := strconv.Atoi(c.DefaultQuery("limit", "25"))
		if e != nil || n < 1 || n > 100 {
			response.WriteError(c, apperror.ErrInvalid)
			return
		}
		rows, e := h.UC.List(c.Request.Context(), request.Actor(c), c.Query("arena_id"), c.Query("round_id"), c.Query("session_id"), c.Query("channel_id"), c.Query("cursor"), n+1)
		if e != nil {
			send(c, nil, e)
			return
		}
		more := len(rows) > n
		next := ""
		if more {
			rows = rows[:n]
			next = core.Text(rows[len(rows)-1], "id")
		}
		send(c, gin.H{"items": rows, "has_more": more, "next_cursor": next}, nil)
	})
	users.GET("/media/recordings/:id", func(c *gin.Context) {
		r, e := h.UC.Read(c.Request.Context(), request.Actor(c), c.Param("id"))
		send(c, r, e)
	})
	users.GET("/media/recordings/:id/download", func(c *gin.Context) {
		url, e := h.UC.Download(c.Request.Context(), request.Actor(c), c.Param("id"))
		send(c, gin.H{"url": url, "expires_in": 60}, e)
	})
	users.POST("/media/recordings/:id/deliver/whatsapp", func(c *gin.Context) {
		baseURL := getBaseURL(c)
		r, e := h.UC.DeliverWhatsApp(c.Request.Context(), request.Actor(c), c.Param("id"), baseURL)
		send(c, r, e)
	})
	users.POST("/arenas/:arena/rounds/:id/deliveries/whatsapp", func(c *gin.Context) {
		baseURL := getBaseURL(c)
		r, e := h.UC.BroadcastRoundWhatsApp(c.Request.Context(), request.Actor(c), c.Param("arena"), c.Param("id"), baseURL)
		send(c, r, e)
	})
	engine.Use(func(c *gin.Context) {
		token, ok := middleware.Bearer(c)
		if !ok {
			c.Abort()
			response.WriteError(c, apperror.ErrUnauthorized)
			return
		}
		r, e := h.UC.Authenticate(c.Request.Context(), token)
		if e != nil {
			c.Abort()
			response.WriteError(c, e)
			return
		}
		c.Set("media_engine", r)
		c.Next()
	})
	engine.POST("/uploads", func(c *gin.Context) {
		var b model.Upload
		if !request.Bind(c, &b) {
			return
		}
		r, e := h.UC.Create(c.Request.Context(), identity(c), b)
		send(c, r, e)
	})
	engine.GET("/uploads/:id", func(c *gin.Context) {
		r, e := h.UC.Status(c.Request.Context(), identity(c), c.Param("id"))
		send(c, r, e)
	})
	engine.POST("/uploads/:id/parts", func(c *gin.Context) {
		var b model.SignPart
		if !request.Bind(c, &b) {
			return
		}
		r, e := h.UC.Sign(c.Request.Context(), identity(c), c.Param("id"), b)
		send(c, r, e)
	})
	engine.POST("/uploads/:id/complete", func(c *gin.Context) {
		r, e := h.UC.Complete(c.Request.Context(), identity(c), c.Param("id"))
		send(c, r, e)
	})
}

func (h *Handler) PublicRoutes(r *gin.Engine) {
	r.GET("/v/:id", func(c *gin.Context) {
		id := c.Param("id")
		w, err := h.UC.PublicWatch(c.Request.Context(), id)
		if err != nil {
			c.Header("Content-Type", "text/html; charset=utf-8")
			c.String(http.StatusNotFound, `<!DOCTYPE html><html lang="id"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1.0"><title>Video Tidak Ditemukan</title><style>body{background:#101718;color:#e9efec;font-family:sans-serif;display:flex;align-items:center;justify-content:center;height:100vh;margin:0;text-align:center;padding:20px;box-sizing:border-box}h2{color:#f87171;margin-bottom:8px}p{color:#9ca3af;font-size:14px}</style></head><body><div><h2>Video Tidak Ditemukan</h2><p>Rekaman video gantangan ini belum tersedia atau tautan sudah kadaluarsa.</p></div></body></html>`)
			return
		}
		c.Header("Content-Type", "text/html; charset=utf-8")
		c.String(http.StatusOK, renderVideoPlayerHTML(w))
	})

	r.GET("/api/v1/public/watch/:id", func(c *gin.Context) {
		w, err := h.UC.PublicWatch(c.Request.Context(), c.Param("id"))
		send(c, w, err)
	})
}

func getBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	host := c.Request.Host
	if host == "" {
		host = "localhost:8091"
	}
	return scheme + "://" + host
}

func renderVideoPlayerHTML(w *model.PublicVideoWatch) string {
	birdName := html.EscapeString(w.BirdName)
	ownerName := html.EscapeString(w.OwnerName)
	code := html.EscapeString(w.GantanganCode)
	streamURL := html.EscapeString(w.StreamingURL)
	downloadURL := html.EscapeString(w.DownloadURL)

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="id">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Rekaman Gantangan %s — %s</title>
  <style>
    * { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      background: #101718;
      color: #e9efec;
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
      min-height: 100vh;
      display: flex;
      flex-direction: column;
      align-items: center;
      padding: 16px;
    }
    .container {
      width: 100%%;
      max-width: 720px;
      margin: 0 auto;
    }
    header {
      display: flex;
      align-items: center;
      justify-content: space-between;
      margin-bottom: 16px;
      padding-bottom: 12px;
      border-bottom: 1px solid #2b3838;
    }
    .badge {
      display: inline-flex;
      align-items: center;
      gap: 6px;
      background: rgba(212, 237, 149, 0.15);
      color: #d4ed95;
      border: 1px solid rgba(212, 237, 149, 0.3);
      padding: 4px 10px;
      border-radius: 9999px;
      font-size: 11px;
      font-weight: 700;
      text-transform: uppercase;
      letter-spacing: 0.05em;
    }
    .player-card {
      background: #172122;
      border: 1px solid #2b3838;
      border-radius: 16px;
      overflow: hidden;
      box-shadow: 0 10px 25px -5px rgba(0, 0, 0, 0.5);
    }
    video {
      width: 100%%;
      max-height: 480px;
      background: #000;
      display: block;
    }
    .details {
      padding: 16px;
    }
    .title {
      font-size: 18px;
      font-weight: 800;
      color: #fff;
      margin-bottom: 4px;
    }
    .subtitle {
      font-size: 13px;
      color: #9ca3af;
      margin-bottom: 16px;
    }
    .info-grid {
      display: grid;
      grid-template-columns: 1fr 1fr;
      gap: 10px;
      margin-bottom: 16px;
    }
    .info-box {
      background: #1b2425;
      border: 1px solid #2b3838;
      border-radius: 10px;
      padding: 10px;
    }
    .info-label {
      font-size: 10px;
      font-weight: 700;
      color: #9ca3af;
      text-transform: uppercase;
      letter-spacing: 0.05em;
      margin-bottom: 2px;
    }
    .info-val {
      font-size: 13px;
      font-weight: 700;
      color: #38bdf8;
    }
    .speed-controls {
      display: flex;
      align-items: center;
      gap: 8px;
      margin-bottom: 16px;
      background: #1b2425;
      padding: 8px 12px;
      border-radius: 10px;
      border: 1px solid #2b3838;
    }
    .speed-label {
      font-size: 11px;
      font-weight: 700;
      color: #9ca3af;
      margin-right: 4px;
    }
    .speed-btn {
      background: #23302b;
      color: #e9efec;
      border: 1px solid #2b3838;
      padding: 4px 10px;
      border-radius: 6px;
      font-size: 11px;
      font-weight: 700;
      cursor: pointer;
      transition: all 0.15s;
    }
    .speed-btn:hover, .speed-btn.active {
      background: #d4ed95;
      color: #101718;
      border-color: #d4ed95;
    }
    .download-btn {
      display: block;
      width: 100%%;
      text-align: center;
      background: #d4ed95;
      color: #101718;
      font-weight: 800;
      font-size: 13px;
      padding: 12px;
      border-radius: 10px;
      text-decoration: none;
      transition: background 0.15s;
    }
    .download-btn:hover {
      background: #c2e276;
    }
    footer {
      text-align: center;
      margin-top: 24px;
      font-size: 11px;
      color: #6b7280;
    }
  </style>
</head>
<body>
  <div class="container">
    <header>
      <div class="badge">🎥 CCTV Resmi Kicau Mania</div>
      <span style="font-size: 11px; color: #fbbf24; font-weight: 700;">Full HD 10 Menit</span>
    </header>

    <div class="player-card">
      <video id="player" controls playsinline autoplay preload="metadata">
        <source src="%s" type="video/mp4">
        <source src="%s" type="video/x-matroska">
        Browser Anda tidak mendukung pemutar video HTML5.
      </video>

      <div class="details">
        <div class="title">%s</div>
        <div class="subtitle">Pemilik: %s · Saluran: %s</div>

        <div class="info-grid">
          <div class="info-box">
            <div class="info-label">Gantangan</div>
            <div class="info-val" style="color: #fbbf24;">%s</div>
          </div>
          <div class="info-box">
            <div class="info-label">Durasi Ronde</div>
            <div class="info-val">10:00 (600 Detik)</div>
          </div>
        </div>

        <div class="speed-controls">
          <span class="speed-label">Kecepatan:</span>
          <button class="speed-btn" onclick="setSpeed(0.5, this)">0.5x (Slow)</button>
          <button class="speed-btn active" onclick="setSpeed(1.0, this)">1.0x (Normal)</button>
          <button class="speed-btn" onclick="setSpeed(1.5, this)">1.5x</button>
          <button class="speed-btn" onclick="setSpeed(2.0, this)">2.0x</button>
        </div>

        <a href="%s" download class="download-btn">⬇️ Download Video HD ke Galeri</a>
      </div>
    </div>

    <footer>
      Siaran Resmi Penjurian CCTV Kicau Mania · Dilindungi Hak Cipta
    </footer>
  </div>

  <script>
    const vid = document.getElementById('player');
    function setSpeed(rate, btn) {
      if (vid) vid.playbackRate = rate;
      document.querySelectorAll('.speed-btn').forEach(b => b.classList.remove('active'));
      if (btn) btn.classList.add('active');
    }
  </script>
</body>
</html>`, code, birdName, streamURL, streamURL, birdName, ownerName, code, code, downloadURL)
}

func identity(c *gin.Context) core.Record { v, _ := c.Get("media_engine"); return v.(core.Record) }
