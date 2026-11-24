# Feature Map - Local Edge Media Engine (`local-app`)

Direktori ini berisi katalog interaktif dan peta fitur (*Feature Map*) lengkap untuk **Local Edge Media Engine (`local-app`)**.
Engine ini berjalan on-premise di PC lapangan/arena untuk menangani koneksi kamera RTSP (NVR), perekaman multi-channel dengan `ffmpeg` & `mediamtx`, streaming lokal WebSocket, dan relay live streaming ke YouTube/Cloud.

---

## Model Akses Jaringan Fisik Arena

| Level Akses | Middleware / Guard | Syarat Autentikasi | Target Pengguna |
| :--- | :--- | :--- | :--- |
| **`LANViewer`** | `middleware.LANViewer(ReadCIDRs)` | IP Pengguna berada dalam subnet WiFi arena (misal `192.168.1.0/24`) tanpa token | Layar Juri di HP/Tablet & Penonton arena untuk memutar katalog kamera |
| **`LocalControl`** | `middleware.LocalControl(Token)` | Header `X-Local-Control-Token` | Aplikasi Desktop Operator di PC server arena |
| **`Operator`** | `middleware.Operator(Token)` | Header `Authorization: Bearer <Token>` | Cloud Backend atau Operator remote via internet |

---

## Indeks Dokumen Fitur

| File | Nama Domain / Fitur Utama | Endpoint Utama |
| :--- | :--- | :--- |
| [01-lan-setup-and-status.md](./01-lan-setup-and-status.md) | Konfigurasi LAN & Status Onboarding | `/api/v1/local/status`, `/guidance/:code`, `/setup/config` |
| [02-camera-and-channels.md](./02-camera-and-channels.md) | Katalog Kamera & Channel RTSP | `/api/v1/channels`, `/api/v1/channels/:id/audio` |
| [03-session-recording.md](./03-session-recording.md) | Perekaman Sesi & Journaling | `/api/v1/media/sessions/start`, `/stop`, `/current` |
| [04-live-streaming-restream.md](./04-live-streaming-restream.md) | Live Streaming & Restream RTMP | `/api/v1/streaming/start`, `/stop`, `/status` |
| [05-zoom-and-ptz.md](./05-zoom-and-ptz.md) | Digital Crop & Preset Zoom Kamera | `/api/v1/zoom`, `/api/v1/zoom/presets` |
| [06-audio-websocket.md](./06-audio-websocket.md) | Streaming Audio Real-time WebSocket | `/api/v1/ws/audio`, `/api/v1/channels/:id/audio` |
