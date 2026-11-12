# Panduan Pengujian local-app dengan Insomnia

Dokumen ini menjelaskan cara menguji REST API, siklus setup LAN, siklus sesi media, dan endpoint signaling pada `local-app` menggunakan Insomnia. Koleksi siap impor tersedia di [local-app-testing.json](local-app-testing.json).

Untuk referensi kontrak API dan arsitektur yang lebih rinci, lihat:

- [Panduan setup API](../setup-api.md)
- [Kontrak cloud](../cloud-contract.md)
- [Penerimaan media](../acceptance.md)

## 1. Menyiapkan server `local-app`

Pilih salah satu skenario berikut dari direktori `local-app`:

### Skenario A — Mode LAN (Setup lokal tanpa cloud)

Cocok untuk menguji bootstrap, ONVIF discovery/profiles, probe, apply DPAPI, status lokal, sesi media, dan playback LAN:

```powershell
Copy-Item .env.portable.example .env
# Sesuaikan FFMPEG_BIN, FFPROBE_BIN, dan MEDIAMTX_BIN bila tidak memakai paket portable
go run ./cmd/api -env .env
```

Catatan Mode LAN:

- `ENGINE_ACCESS_MODE=lan` dan `CHANNELS_JSON=[]` valid saat startup; `/healthz` langsung mengembalikan `200`, sementara `/readyz` mengembalikan `503` sampai setup diterapkan dan MediaMTX berjalan.
- `API_TOKEN` boleh kosong; backend membuat token acak per proses. Kontrol operator di Mode LAN dilakukan melalui loopback dengan header `X-Local-Control`.
- Semua endpoint `/api/v1/setup/*` dan kontrol `X-Local-Control` **wajib** diakses melalui loopback (`http://127.0.0.1:8090` atau `http://localhost:8090`), tidak melalui IP LAN atau reverse proxy.

### Skenario B — Mode Cloud / Webcam Test (Bearer `API_TOKEN`)

Cocok untuk menguji siklus sesi media dan sumber sintetis/webcam tanpa setup NVR:

```powershell
# Terminal 1: siapkan .env.webcam dan jalankan sumber RTSP sintetis
./scripts/start-webcam-test.ps1 -InitEnv
./scripts/start-webcam-test.ps1 -Synthetic -DurationSeconds 300

# Terminal 2: jalankan local-app
go run ./cmd/api -env .env.webcam
```

Salin nilai `API_TOKEN` dari file `.env.webcam` ke variabel `api_token` pada environment Insomnia.

## 2. Import dan pilih Environment di Insomnia

1. Buka Insomnia → **Import** → **From File** → pilih [local-app-testing.json](local-app-testing.json).
2. Pilih sub-environment sesuai skenario yang dijalankan:
   - **Mode LAN (Loopback)** untuk Skenario A.
   - **Mode Cloud / Webcam Test** untuk Skenario B (isi `api_token` sesuai `.env.webcam` / `.env`).
3. Koleksi memakai export format v4 dan membaca status HTTP melalui `insomnia.response.code` serta membaca variabel melalui `insomnia.variables.get(...)` / menulis ke active environment melalui `insomnia.environment.set(...)`.

### Ringkasan variabel Environment

| Variabel | Fungsi | Terisi otomatis oleh |
| --- | --- | --- |
| `base_url` | Origin HTTP API (tanpa `/api/v1`), default `http://127.0.0.1:8090` | Manual / sub-env |
| `ws_base_url` | Origin WebSocket, default `ws://127.0.0.1:8090` | Manual / sub-env |
| `api_token` | `API_TOKEN` untuk Bearer pada Mode Cloud / `.env.webcam` | Manual |
| `local_control_nonce` | Nonce kontrol lokal Mode LAN (berubah setiap restart server) | `01 / 01 — Bootstrap` |
| `lan_ip`, `lan_cidr` | IPv4 dan CIDR interface LAN PC engine | `01 / 01 — Bootstrap` |
| `setup_job_id` | ID pekerjaan setup asinkron (`discover`/`profiles`/`probe`/`apply`/`recheck`) | `01 / 04, 06, 07, 08, 09` |
| `onvif_endpoint` | URL layanan ONVIF perangkat hasil discovery | `01 / 05 — Poll status job` |
| `credential_ref` | Referensi kredensial sementara (berlaku ~15 menit) | `01 / 05 — Poll status job` |
| `channel_id`, `channel_source`, `live_mode` | ID channel, URL RTSP uji, dan mode live (`copy` / `transcode`) | Manual / sub-env |
| `session_id` | ID sesi media (`id` baru untuk setiap siklus Start baru) | Manual / sub-env |
| `recordings_cursor` | Cursor paginasi daftar rekaman | `05 / 01..03` |
| `playback_ticket` | Tiket playback dari `cloud-app` (khusus Mode Cloud WHEP) | Manual |
| `whep_session_path`, `whep_etag` | Path `Location` dan `ETag` dari `POST /whep` | `06 / 01..02` |

## 3. Aturan Autentikasi Penting

Perilaku middleware di `local-app` berbeda antara endpoint viewer, setup, dan operator:

1. **Publik**: `GET /`, `GET /healthz`, `GET /readyz`, `GET /api/docs/doc.json`.
2. **Setup Mode LAN (`/api/v1/setup/*`)**:
   - Hanya aktif bila `ENGINE_ACCESS_MODE=lan` dan peer adalah loopback (`127.0.0.1`/`::1`) dengan `Host` loopback.
   - `GET` tidak membutuhkan header `X-Local-Control`.
   - `POST` dan `DELETE` **wajib** menyertakan header `X-Local-Control: {{ _.local_control_nonce }}`.
3. **Viewer LAN (`/api/v1/local/status`, `/api/v1/local/guidance/:code`, `/api/v1/channels`)**:
   - Di Mode LAN, dapat diakses dari subnet `MEDIA_READ_CIDRS` (dan loopback) tanpa token maupun nonce.
   - Di Mode Cloud, `/api/v1/channels` wajib `Authorization: Bearer {{ _.api_token }}`.
4. **Operator (`/api/v1/engine/status`, `/api/v1/media/sessions*`, `/api/v1/recordings`, `/api/v1/restreams`)**:
   - Di **Mode LAN**, jika ingin memakai kontrol lokal tanpa `API_TOKEN`, **kirim header `X-Local-Control` pada request `GET` maupun `POST`** dari loopback. Jika `X-Local-Control` kosong, middleware otomatis beralih memeriksa `Authorization: Bearer API_TOKEN`.
   - Di **Mode Cloud** (atau saat `API_TOKEN` disetel), kirim `Authorization: Bearer {{ _.api_token }}`.

## 4. Urutan Pengujian

### Alur A — Mode LAN (dari nol sampai stop sesi)

1. **Folder `00 — Health & Docs`**:
   - Jalankan `02 — Liveness` (`200`) dan `03 — Readiness` (`503` sebelum konfigurasi diterapkan, `200` setelah apply berhasil dan MediaMTX berjalan).
2. **Folder `01 — Setup Mode LAN (Loopback)`**:
   - Jalankan `01 — Bootstrap nonce & interface LAN` untuk mengisi `local_control_nonce`, `lan_ip`, dan `lan_cidr`. Jika PC memiliki lebih dari satu interface, periksa `data.networks` pada response lalu sesuaikan `lan_ip` dan `lan_cidr` bila perlu.
   - Jalankan `02 — Cek binary & encoder` untuk memastikan FFmpeg, ffprobe, MediaMTX, `libx264`, `libopus`, dan `aac` terbaca `OK`.
   - *(Opsional jika memakai NVR ONVIF)*: Jalankan `04 — Job: Discover`, poll dengan `05 — Poll status job setup` hingga `state: "SUCCEEDED"`, isi `onvif_username`/`onvif_password`, jalankan `06 — Job: Baca profil ONVIF`, lalu poll lagi dengan `05`.
   - *(Uji sumber)*: Jalankan `07 — Job: Probe stream (RTSP manual)` (pastikan sumber RTSP aktif, misalnya dari `start-webcam-test.ps1 -Synthetic`), lalu poll dengan `05` untuk melihat `video_codec`, `fps`, `b_frames`, dan `has_audio`.
   - Jalankan `08 — Job: Apply konfigurasi channel & LAN`, lalu poll dengan `05` sampai `SUCCEEDED`.
3. **Folder `02 — Status, Guidance & Channels`**:
   - Jalankan `01 — Status lokal`, `03 — Daftar channel (Mode LAN)`, dan `05 — Engine status (Mode LAN Nonce)`.
4. **Folder `03 — Media Session (Mode LAN Nonce)`**:
   - Pastikan `session_id` belum pernah dipakai sebelumnya.
   - Jalankan `01 — Mulai sesi media (LAN)` (`202`).
   - Ulangi `02 — Cek status sesi media (LAN)` hingga `state` menjadi `RUNNING` (atau `DEGRADED` jika sumber belum aktif).
   - Jalankan `03 — Stop sesi media (LAN)` (`202`), lalu poll `04 — Poll status setelah Stop` hingga `state: "STOPPED"`.
5. **Folder `05 — Recordings & Restreams`**:
   - Jalankan `01 — Daftar rekaman (Mode LAN Nonce)` dan `04 — Daftar status restream (Mode LAN Nonce)`. Pastikan rekaman mencapai `state: "READY"` (atau `"UPLOADED"` jika S3 dikonfigurasi).

### Alur B — Mode Cloud / `.env.webcam`

1. Pilih environment **Mode Cloud / Webcam Test** dan pastikan `api_token` terisi.
2. Jalankan folder **`00 — Health & Docs`** (`02 — Liveness`, `03 — Readiness`).
3. Jalankan folder **`02 — Status, Guidance & Channels`** (`04 — Daftar channel (Cloud Bearer)` dan `06 — Engine status (Cloud / Bearer)`).
4. Jalankan folder **`04 — Media Session (Mode Cloud / Bearer)`** (`01` → `02` → `03` → `04`).
5. Jalankan folder **`05 — Recordings & Restreams`** (`03` dan `05`).

> **Catatan:** Jangan menjalankan folder `01 — Setup Mode LAN` pada Mode Cloud karena route `/api/v1/setup/*` dan `/api/v1/local/*` tidak didaftarkan di Mode Cloud (`404`).

### Kasus Negatif (Folder `90 — Negatif & Guardrails`)

Jalankan sesuai mode yang sedang aktif:

- Request `01` dan `02` berlaku di kedua mode.
- Request `03`–`06`, `07`, dan `09` ditujukan untuk **Mode LAN** (menguji penolakan nonce/payload/404).
- Request `08` dan `10` ditujukan untuk **Mode Cloud / Bearer**.
- Request `11` menguji penolakan `Content-Type` non-`application/sdp` (`415`) pada endpoint WHEP (membutuhkan `channel_id` yang sudah terdaftar).

## 5. Pengujian WebSocket & WHEP

### WebSocket di Insomnia

Insomnia mendukung tab **WebSocket Request** terpisah (tidak digabung dalam request HTTP di koleksi ini agar kompatibel lintas versi importer v4). Buat WebSocket Request baru di Insomnia dengan detail berikut saat **sesi media sedang `RUNNING`**:

1. **Metrik audio (JSON text frame)**:
   - URL: `{{ _.ws_base_url }}/api/v1/metrics`
   - Header: `Sec-WebSocket-Protocol: edge-v1`
   - Mode Cloud: tambahkan `Authorization: Bearer {{ _.api_token }}` (atau gunakan subprotocol `edge-v1, edge-token.<BASE64URL_RAW_TOKEN>`).
   - Output tiap frame: JSON berisi `channel_id`, `at`, `peak_dbfs`, `rms_dbfs`, `active_ms`, `window_ms` (hanya jika sumber memiliki audio).
2. **Audio PCM mentah (binary frame)**:
   - URL: `{{ _.ws_base_url }}/api/v1/channels/{{ _.channel_id }}/audio`
   - Header: `Sec-WebSocket-Protocol: edge-v1` (plus `Authorization` jika Mode Cloud).
   - Output: binary PCM mono signed 16-bit little-endian 16 kHz.

### WHEP Signaling (Folder `06`)

- Endpoint `POST /api/v1/playback/{channel}/whep` mem-proxy signaling WebRTC ke MediaMTX internal.
- SDP dummy (`sample_sdp_offer`) tidak dapat menyelesaikan negosiasi DTLS/ICE MediaMTX sehingga kemungkinan ditolak upstream. Untuk menguji aliran video/audio nyata, kirim SDP offer dari `RTCPeerConnection` frontend/browser atau player uji WHEP.
- Jika `POST /whep` berhasil (`201`), script otomatis menyimpan `Location` ke `whep_session_path` dan `ETag` ke `whep_etag` untuk diuji pada `PATCH` (trickle ICE) dan `DELETE` (tutup viewer).

## 6. Troubleshooting Singkat

| Gejala | Penyebab umum & solusi |
| --- | --- |
| `403` pada `/api/v1/setup/*` | Server tidak dijalankan dengan `ENGINE_ACCESS_MODE=lan`, URL bukan `127.0.0.1`/`localhost`, atau `X-Local-Control` tidak cocok karena server sempat restart (jalankan ulang `01 / 01 — Bootstrap`). |
| `401` pada `/api/v1/engine/status` atau `/api/v1/media/sessions` | Di Mode LAN lupa menyertakan header `X-Local-Control`; di Mode Cloud `api_token` kosong atau tidak sesuai `.env`. |
| `409` saat `POST /api/v1/setup/jobs` | Masih ada 2 job setup yang `RUNNING`, atau mencoba `apply` saat sesi media masih aktif/finalisasi. |
| `409` saat `POST /api/v1/media/sessions` | Masih ada sesi aktif lain, atau `session_id` sudah pernah dipakai untuk sesi yang sudah selesai. Gunakan `session_id` baru. |
| `503` pada `/readyz` | Di Mode LAN: belum `apply` konfigurasi atau binary MediaMTX belum siap. Di Mode Cloud: endpoint `CLOUD_*_URL` belum dikonfigurasi/terhubung. |
