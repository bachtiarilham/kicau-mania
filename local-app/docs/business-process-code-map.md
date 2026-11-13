# Peta Proses Bisnis & Struktur Direktori `local-app`

Dokumen ini memetakan setiap proses bisnis dalam `local-app` ke folder dan berkas kode Go tempat proses tersebut diolah. Gunakan panduan ini saat hendak mencari file implementasi, men-debug fitur tertentu, atau melakukan pemeliharaan kode.

Arsitektur aplikasi menganut pola Clean Architecture berlapis:
- **`cmd/`**: Titik masuk eksekusi program (*Entry Point*).
- **`internal/delivery/`**: Pengendali protokol luar (*HTTP REST, WHEP, WebSocket, Middleware*).
- **`internal/.../usecase/`**: Inti logika bisnis (*Business Logic & Workflow*).
- **`internal/.../repository/`**: Penyimpanan data fisik (*File System, JSON Journal, DPAPI*).
- **`internal/.../worker/`**: Pemrosesan asinkron di latar belakang (*Background Process, Goroutine*).
- **`internal/infra/`**: Komunikasi dengan sistem operasi dan hardware (*FFmpeg Subprocess, DNS, Storage*).

---

## 1. Peta Navigasi Proses Bisnis ke File Kode

| Proses Bisnis / Fitur | Apa yang Dikerjakan? | Folder & File Pengolah Utama |
| :--- | :--- | :--- |
| **1. Startup & Mode Server** | Membaca `.env`, mengunci file lock (`edge.lock`), memilih mode LAN atau Cloud, menyalakan listener HTTP. | • `cmd/api/main.go`<br>• `internal/server/server.go`<br>• `internal/server/lan.go`<br>• `config/config.go` |
| **2. Pengamanan & Hak Akses** | Validasi Loopback (`127.0.0.1`), token Bearer, nonce `X-Local-Control`, CORS Origin, dan subnet `MEDIA_READ_CIDRS`. | • `internal/delivery/middleware/local.go`<br>• `internal/delivery/middleware/auth.go`<br>• `internal/delivery/middleware/cors.go` |
| **3. Setup LAN & Discovery ONVIF** | Mengirim multicast WS-Discovery UDP 3702, membaca profil kamera, dan probe stream RTSP via `ffprobe`. | • Handler: `internal/delivery/rest/setup/handler.go`<br>• Logika: `internal/setup/usecase/setup.go`<br>• Worker Job: `internal/setup/worker/jobs.go`<br>• Adapter ONVIF: `internal/setup/adapter/` |
| **4. Penyimpanan Setelan Terenkripsi** | Menyimpan konfigurasi kamera dan jaringan LAN ke disk dengan enkripsi Windows DPAPI agar password CCTV aman. | • `internal/setup/repository/configuration.go`<br>• File target: `data/setup.json` |
| **5. Siklus Sesi Media (Start / Stop)** | Menerima order mulai lomba, memastikan tidak ada sesi bentrok, menyalakan pipeline FFmpeg, menghentikan sesi saat selesai. | • Handler: `internal/delivery/rest/session/handler.go`<br>• Logika: `internal/mediaengine/session/usecase/session.go`<br>• Journal Status: `internal/mediaengine/session/repository/journal.go` |
| **6. Eksekusi FFmpeg & Pipeline Video** | Menjalankan subprocess `ffmpeg.exe` per channel untuk menyedot RTSP, membuat segmen MP4, dan mengumpankan video ke MediaMTX. | • `internal/mediaengine/media/ffmpeg/pipeline.go`<br>• `internal/mediaengine/media/ffmpeg/streaming.go`<br>• `internal/infra/runtime/subprocess/` |
| **7. Server Streaming WebRTC (MediaMTX)** | Mengelola hidup/matinya proses `mediamtx.exe`, membuat file `mediamtx.yml` otomatis, mengawasi jika proses crash. | • `internal/mediaengine/media/mediamtx/server.go`<br>• `internal/server/media.go` |
| **8. Playback Live WHEP ke Browser** | Menerima SDP offer browser, memvalidasi tiket/LAN, mem-proxy signaling ke MediaMTX, mengelola ICE candidate. | • Handler: `internal/delivery/whep/handler.go`<br>• Router: `internal/delivery/whep/router.go`<br>• Logika Playback: `internal/mediaengine/channel/usecase/playback.go` |
| **9. Olah Suara Burung & WebSocket** | Menerima potongan audio PCM dari FFmpeg, menghitung dBFS/rms, menyiarkan grafik desibel via WebSocket `/api/v1/metrics`. | • Handler WS: `internal/delivery/websocket/handler.go`<br>• Transport WS: `internal/delivery/websocket/websocket.go`<br>• Logika Audio: `internal/mediaengine/audio/usecase/audio.go`<br>• Broadcaster: `internal/mediaengine/audio/service/broadcaster.go` |
| **10. Rekaman MP4 & Penggabungan Segmen** | Menyimpan potongan segmen video saat siaran, menggabungkan segmen menjadi MP4 utuh saat sesi ditutup (*Finalizer*). | • Logika Rekam: `internal/mediaengine/saving/usecase/saving.go`<br>• Penggabung: `internal/mediaengine/media/ffmpeg/finalizer.go`<br>• Antrean Disk: `internal/mediaengine/saving/repository/recording.go` |
| **11. Restreaming ke YouTube / Cloud** | Meneruskan stream video lokal ke rute RTMP YouTube atau RTSP cloud luar saat opsi restream diaktifkan. | • Handler: `internal/delivery/rest/streaming/handler.go`<br>• Logika: `internal/mediaengine/streaming/usecase/streaming.go`<br>• Worker Restream: `internal/mediaengine/streaming/worker/restream.go` |
| **12. Upload Rekaman ke Cloud S3 (Opsional)** | Membaca antrean MP4 berstatus `READY`, mengunggah ke S3/MinIO, melakukan retry otomatis jika koneksi internet lambat. | • Logika Upload: `internal/mediaengine/saving/usecase/upload.go`<br>• Worker Upload: `internal/mediaengine/saving/worker/upload.go`<br>• Client S3: `internal/infra/storage/s3/` |
| **13. Komunikasi Cloud Engine (Mode Cloud)** | Heartbeat berkala, polling perintah jarak jauh dari VPS cloud, dan lapor metadata rekaman. | • `internal/cloud/usecase/engine.go`<br>• `internal/cloud/usecase/heartbeat.go`<br>• `internal/cloud/usecase/commands.go` |

---

## 2. Rincian Alur per Skenario Nyata

### A. Alur Menyalakan Server (Bootstrapping)
1. **Titik Awal**: `cmd/api/main.go` membaca flag `-env`.
2. **Parsing Config**: `config/config.go` memvalidasi variabel environment.
3. **Pemberian Kunci File**: `internal/infra/storage/filesystem/lock.go` mengunci `data/edge.lock` agar program tidak bisa dijalankan dobel di satu komputer.
4. **Pemilihan Mode**:
   - Jika `ENGINE_ACCESS_MODE=lan` → dialihkan ke `internal/server/lan.go`.
   - Jika `ENGINE_ACCESS_MODE=cloud` → dijalankan oleh `internal/server/server.go`.
5. **Perakitan Handler**: `internal/app/container.go` menyatukan database file, worker latar belakang, dan router HTTP.
6. **Listener HTTP**: `internal/delivery/router.go` mendaftarkan seluruh endpoint REST, WHEP, dan WebSocket.

---

### B. Alur Setup Kamera (Discovery → Probe → Apply)
1. **Request Masuk**: Juri/Operator menembak endpoint `POST /api/v1/setup/jobs`.
2. **Filter Keamanan**: `internal/delivery/middleware/local.go` memeriksa apakah request berasal dari `127.0.0.1` dan memiliki header `X-Local-Control`.
3. **Penyusunan Job Asinkron**: `internal/setup/worker/jobs.go` membuat ID pekerjaan (job ID) dan menjalankannya di thread latar belakang (*goroutine*).
4. **Eksekusi Teknis**:
   - Jika `action: "discover"`: `internal/setup/usecase/setup.go` memanggil scanner WS-Discovery.
   - Jika `action: "probe"`: `internal/mediaengine/media/ffmpeg/probe.go` menjalankan `ffprobe` untuk mengukur resolusi dan fps stream kamera.
   - Jika `action: "apply"`: `internal/setup/repository/configuration.go` menyimpan konfigurasi final ke `data/setup.json` menggunakan enkripsi DPAPI Windows.
5. **Pemutakhiran MediaMTX**: `internal/mediaengine/media/mediamtx/server.go` menulis ulang berkas `runtime/mediamtx.yml` agar kamera yang baru didaftarkan siap dipancarkan.

---

### C. Alur Sesi Dimulai (Start Session: `POST /api/v1/media/sessions`)
1. **Validasi Request**: `internal/delivery/rest/session/handler.go` menerima daftar ID gantangan (`channels: ["gantangan01"]`).
2. **Pengecekan Bentrok**: `internal/mediaengine/session/usecase/session.go` memastikan belum ada sesi lain yang sedang aktif dan ruang harddisk masih cukup (`MinFreeBytes`).
3. **Pencatatan Status**: `internal/mediaengine/session/repository/journal.go` menulis file status sesi ke `data/sessions/<id>.json` dengan status `STARTING`.
4. **Pemicuan Mesin Media**:
   - **Video**: `internal/mediaengine/media/ffmpeg/pipeline.go` meluncurkan subprocess `ffmpeg.exe` untuk menyedot RTSP kamera dan mengumpankannya ke MediaMTX loopback.
   - **Audio**: `internal/mediaengine/audio/usecase/audio.go` membuka antrean penyerapan suara.
   - **Rekaman**: `internal/mediaengine/saving/repository/recording.go` menyiapkan file segmen baru di `data/recordings/`.
5. **Sesi Siap**: Status berubah menjadi `RUNNING`.

---

### D. Alur Penonton Membuka Video Live di Browser (WHEP)
1. **Salaman SDP Offer**: Browser juri menembak `POST /api/v1/playback/gantangan01/whep`.
2. **Otorisasi**:
   - Di Mode LAN: `internal/delivery/middleware/local.go` (`LANViewer`) memeriksa apakah IP browser juri terdaftar di `MEDIA_READ_CIDRS`.
   - Di Mode Cloud: `internal/mediaengine/channel/usecase/playback.go` memvalidasi tiket JWT dari cloud-app.
3. **Proxy Signaling**: `internal/delivery/whep/handler.go` meneruskan SDP Offer ke port internal MediaMTX (`127.0.0.1:8889`).
4. **Salaman SDP Answer**: MediaMTX membalas dengan SDP Answer. Header `Location` sesi disimpan di memory handler WHEP.
5. **Jalur Terbuka**: Media video dan audio langsung mengalir melalui UDP port `8189` ke browser penonton.

---

### E. Alur Sesi Selesai (Stop Session: `POST /api/v1/media/sessions/:id/stop`)
1. **Penutupan Akses**: `internal/delivery/whep/handler.go` langsung memutuskan seluruh sesi WHEP juri yang masih terbuka.
2. **Pemutusan WebSocket**: `internal/delivery/websocket/handler.go` menutup sambungan metrik audio.
3. **Penghentian FFmpeg**: Subprocess FFmpeg diperintahkan berhenti merekam secara teratur (`q` via stdin atau SIGINT).
4. **Finalisasi Rekaman**: `internal/mediaengine/media/ffmpeg/finalizer.go` memanggil FFmpeg untuk menggabungkan segmen-segmen parsial menjadi satu file `.mp4` utuh.
5. **Validasi File**: `internal/mediaengine/saving/usecase/saving.go` mengecek file MP4 hasil gabungan dengan `ffprobe`. Jika valid, status rekaman diubah menjadi `READY`.
6. **Sesi Selesai**: Jurnal sesi di-update menjadi `STOPPED`. Jika integrasi S3 aktif, worker upload di `internal/mediaengine/saving/worker/upload.go` akan mulai mengunggah file tersebut di latar belakang.
