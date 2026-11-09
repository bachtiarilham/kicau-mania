# Local-app: engine media arena

Local-app menjalankan live WebRTC LAN, restream, pengolahan audio, rekaman MP4 video/audio, dan upload S3. Login, registrasi, penilaian, dan lifecycle lomba menjadi tanggung jawab cloud-app. Engine tidak memakai SQLite atau menyajikan UI penilaian.

Mulai membaca dari [alur end-to-end](ALUR-END-TO-END.md), [peta folder/file](PETA-KODE.md), dan [kontrak cloud](ENGINE-CLOUD-CONTRACT.md).

Untuk memahami engine media dari awal, buka **[Memahami mediaengine](MEMAHAMI-MEDIAENGINE.md)**. Panduan ini mengikuti alur dari kamera, live LAN, PCM/metrik, sesi rekaman, sampai upload dan recovery, dengan jalur baca kode per fitur.

## Menjalankan

Prasyarat: Go 1.25+, FFmpeg dengan libx264/aac, dan MediaMTX v1.15.3. Windows dan Linux didukung. Dari direktori local-app:

```powershell
Copy-Item .env.example .env
# Isi API_TOKEN acak minimal 32 karakter dan CHANNELS_JSON.
go run ./cmd/api
```

Gunakan `.env.webcam.example` dan script webcam di `scripts/` untuk webcam/mikrofon atau sumber sintetis. `go run ./cmd/api -render-config` menghasilkan konfigurasi MediaMTX pada RUNTIME_DIR, terpisah dari DATA_DIR. `scripts/build-local.ps1` membangun server dan alat konfigurasi LAN.

## Konfigurasi

| Env | Fungsi |
| --- | --- |
| API_TOKEN | Kredensial kontrol engine; jangan dibagikan kepada browser pengguna. |
| ARENA_ID / ENGINE_ID | Identitas engine yang didaftarkan pada cloud. |
| CHANNELS_JSON | Sumber kamera/NVR RTSP dan ID channel. |
| RESTREAM_TARGETS_JSON | Target restream dan kredensial layanan. |
| FRONTEND_ORIGIN | Origin frontend, misalnya https://dev.megaallstararena.com. |
| LOCAL_PUBLIC_URL | HTTPS engine LAN, misalnya https://lan-dev.megaallstararena.com. |
| WEBRTC_HOSTS | IP LAN mini PC untuk kandidat ICE. |
| CLOUD_ENGINE_TOKEN | Kredensial mesin untuk panggilan cloud. |
| CLOUD_COMMANDS_URL | Polling command; hasil ke akhiran /results. |
| CLOUD_HEARTBEAT_URL | Heartbeat, interval default 5 detik. |
| CLOUD_MEDIA_UPDATES_URL | Batch metadata rekaman/upload. |
| CLOUD_PLAYBACK_AUTHORIZE_URL | Validasi tiket playback pengguna. |
| HEARTBEAT_TIMEOUT | Batas heartbeat dianggap segar, default 15 detik. |
| DATA_DIR | Rekaman/manifest, journal command, dan antrean metadata. |
| FFPROBE_BIN | ffprobe untuk validasi segmen/MP4, default `ffprobe`. |
| RUNTIME_DIR | Konfigurasi MediaMTX dan file runtime. |
| S3_* | Bucket, endpoint, region, dan kredensial upload. |

Endpoint cloud baru harus mengikuti kontrak pendamping. Implementasi cloud untuk heartbeat, metadata, penerbitan tiket, dan otorisasi playback belum disertakan dalam perubahan engine ini. Playback menolak sesi baru jika otorisasi cloud belum tersedia.

## Frontend cloud ke LAN

Browser operator/juri login ke cloud lalu meminta tiket live. Frontend membuka `LOCAL_PUBLIC_URL/api/v1/playback/{channel}/whep` dengan Authorization. Local-app memvalidasi tiket ke cloud dan meneruskan signaling ke MediaMTX loopback. Media WebRTC mengalir melalui LAN.

Perangkat harus mempunyai rute ke LAN arena. Sertifikat HTTPS engine harus dipercaya, origin harus diizinkan, dan browser mungkin meminta izin jaringan lokal. API_TOKEN dan CLOUD_ENGINE_TOKEN tidak dibagikan ke browser.

Gunakan `.env.lan.example`, `scripts/configure-lan.ps1`, lalu `scripts/start-lan.ps1`. Domain, IP, DNS/hosting Hostinger, dan model TP-Link Archer C6 dapat dikustomisasi di env. Parameter router tidak otomatis mengubah pengaturan router.

DNS domain engine harus mengarah ke IP LAN mini PC dari perangkat arena. Jika memakai DNS bawaan, arahkan DNS jaringan ke mini PC melalui router/DHCP. Sertifikat publik membutuhkan konfigurasi DNS/ACME yang benar. Caddy meneruskan API/playback ke Go; HTTP MediaMTX dan RTSP internal tetap loopback. Izinkan HTTPS serta UDP WebRTC pada firewall LAN.

## API

Delivery dipisahkan menurut fungsi: `internal/delivery/router.go` merangkai semua route, `internal/delivery/middleware` menangani HTTP bersama, `internal/delivery/rest` berisi handler REST dan response JSON, `internal/delivery/whep` menangani signaling WebRTC, dan `internal/delivery/websocket` menangani koneksi audio PCM/metrik. Container merakit handler dan router memasang middleware sebelum mendaftarkan route protokol. UseCase media tetap berada di `internal/mediaengine`.

| Route | Akses / fungsi |
| --- | --- |
| GET /healthz | Publik; proses HTTP hidup. |
| GET /readyz | Publik; kesiapan MediaMTX dan heartbeat cloud. |
| GET /api/v1/engine/status | Token operator; status engine. |
| GET /api/v1/channels | Token operator; katalog tanpa kredensial sumber. |
| POST /api/v1/playback/:channel/whep | Tiket cloud; buka sesi WebRTC. |
| PATCH/DELETE /api/v1/playback/:channel/whep/:session | URL sesi WHEP rahasia sebagai capability. |
| POST /api/v1/media/sessions | Token operator; mulai seluruh media sesi, HTTP 202. |
| POST /api/v1/media/sessions/:id/stop | Token operator; hentikan seluruh media lalu finalisasi, HTTP 202. |
| GET /api/v1/media/sessions/:id | Token operator; status channel, gangguan, dan finalisasi. |
| GET /api/v1/recordings | Token operator; daftar/status rekaman. |

WebSocket audio/metrik hanya menerima subscription selama sesi aktif. GET `/api/v1/restreams` tetap tersedia; kontrol mutasi recording/restream yang terpisah dihapus. Alias `/api/v1/rounds`, endpoint arena, akun lokal, dan UI penilaian dihapus. Field `round_id` dalam manifest lama tetap untuk kompatibilitas penyimpanan; pada API baru ID tersebut adalah ID sesi media.

## Ketahanan dan logging

Header umum dipasang sekali melalui middleware bersama: `X-Content-Type-Options: nosniff`, `Cache-Control: no-store`, serta request ID. Middleware CORS global memeriksa origin dan menangani preflight sebelum autentikasi, termasuk pada error/route yang tidak ditemukan. Header `Vary: Origin` dipasang tanpa menimpa nilai Vary lain. Auth hanya memeriksa kredensial; handler tidak memasang ulang header umum. WHEP tetap memiliki Location/ETag dan WebSocket tetap memiliki header upgrade. Gateway membuang header umum duplikat dari upstream MediaMTX.

Response REST mengikuti format park-server-v3: `{"success":true,"message":"...","data":{...}}` atau `{"success":false,"message":"...","errors":...}`. Field data/errors dihilangkan jika nil. Error internal memakai pesan aman dan dapat menyertakan `errors.request_id`; request ID selalu tersedia di header jika berhasil dibuat. Pagination rekaman sekarang berada di `data.items`, `data.next_cursor` (jika ada), dan `data.has_more`; frontend perlu menyesuaikan pembacaan daftar. ID cursor tetap string opaque.

Health dan status engine juga memakai envelope ini. Response 204 tetap tanpa body. SDP/header WHEP dan frame WebSocket mengikuti protokol masing-masing. Payload request mesin ke cloud tetap langsung, tetapi response cloud kini wajib memakai envelope success/message/data/errors; client cloud membuka data sebelum menyerahkannya ke UseCase. Format response mesin lama tanpa envelope tidak didukung. Lihat ENGINE-CLOUD-CONTRACT.md untuk contoh dan batas penerimaan client.

Rekaman aktif tidak dihentikan hanya karena cloud terputus. Playback baru memerlukan cloud. Engine tidak memutuskan penghentian atau kelanjutan lomba. Journal menyimpan command sebelum eksekusi dan hasil sebelum acknowledgment. Restart merekonsiliasi pekerjaan melalui manifest; start dengan hasil tidak pasti tidak dijalankan ulang secara buta.

Metadata dikirim ulang sampai cloud mengakui ID update. Cloud wajib deduplikasi dan menerapkan versi terbaru. SQLite lama tidak dibaca atau dihapus otomatis; pengarsipannya tindakan operasional terpisah.

Logging mengikuti park-server-v3: Zap, request ID, console development, JSON production dan `${LOG_DIR}/app.log`. Kelola retensi log pada deployment.

Pemeriksaan dasar: `go build ./...`. Suite tes tidak dijalankan otomatis sesuai permintaan. Browser LAN/HTTPS, kamera, rekaman, dan S3 perlu diverifikasi pada lingkungan sebenarnya.

## Membaca komunikasi cloud

Semua komunikasi engine ke cloud berada di `internal/cloud`. Mulai dari `worker/scheduler.go` untuk jadwal pekerjaan, lalu pilih alur UseCase: `command_polling.go`, `command_execution.go`, `heartbeat.go`, `playback.go`, atau `recording_metadata.go`. `connection.go` merakit dependency dan state Engine; `status.go` menghitung readiness.

`client/http.go` memiliki satu implementasi HTTP/header/envelope, `client/command_api.go` menentukan endpoint command, dan `repository/journal.go` menyimpan command/hasil/ack serta antrean metadata. `model` berisi bentuk pesan. Delivery REST/WHEP/WebSocket tetap berada di `internal/delivery` dan tidak dipindahkan ke domain cloud.
## Struktur mediaengine

- `channel`: identitas, repository katalog, dan UseCase daftar playback; handler JSON di `delivery/rest/channel`. Domain video lama sudah digabung ke channel.
- `audio`: analisis dan worker audio; `service` berisi Broadcaster dan MetricsClient, disuntikkan melalui interface UseCase.
- `session`: lifecycle terpadu, journal, pipeline per channel, retry, recovery, finalisasi, dan guard.
- `saving/usecase`: query manifest dan upload; recording dikendalikan sesi.
- `streaming`: lifecycle restream yang sudah ada.
- `media`: kontrak proses dan adapter FFmpeg/MediaMTX.

Audio tidak lagi capture saat startup. Satu FFmpeg per channel menghasilkan recording video copy/audio AAC dan PCM mono 16 kHz. Mulai/Stop memakai satu sesi; pantau status hingga STOPPED. Detail dan contoh request tersedia di [panduan mediaengine](MEMAHAMI-MEDIAENGINE.md); [skenario penerimaan](SESSION-ACCEPTANCE.md) mencatat batas verifikasi.
