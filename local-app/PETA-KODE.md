# Peta folder dan file engine media

Indeks ini menjelaskan source, tes, script, konfigurasi contoh, dan dokumentasi local-app. Data operasional dan env privat tidak diindeks.

## Mulai membaca

- Startup: cmd/api/main.go → config/config.go → internal/server/server.go → internal/app/container.go.
- Rekaman: internal/mediaengine/saving/model/model.go → internal/delivery/rest/saving/handler.go → internal/mediaengine/saving/usecase/saving.go → internal/mediaengine/saving/repository/recording.go.
- Playback: internal/delivery/whep/handler.go → internal/cloud/usecase/playback.go → client cloud → MediaMTX.
- Upload: UseCase saving → worker saving → adapter S3 → manifest → worker metadata.

Lihat [alur end-to-end](ALUR-END-TO-END.md) dan [kontrak cloud](ENGINE-CLOUD-CONTRACT.md).

## Fungsi folder

| Folder | Fungsi |
| --- | --- |
| cmd | Entry point server dan alat konfigurasi deployment. |
| config | Pembacaan env dan validasi konfigurasi. |
| internal/app | Perakitan dependency. |
| internal/server | Lifecycle proses, startup/shutdown modul. |
| internal/delivery | Router utama yang merangkai semua protokol. |
| internal/delivery/middleware | Middleware HTTP bersama untuk REST, WHEP, dan handshake WebSocket. |
| internal/delivery/rest | Handler JSON REST per fungsi serta helper response. |
| internal/delivery/whep | Gateway signaling WebRTC yang memakai otorisasi cloud. |
| internal/delivery/websocket | Handler audio/metrik dan lifecycle koneksi WebSocket. |
| internal/apperror | Error aplikasi bersama. |
| internal/core/identifier | Validasi format identifier. |
| internal/mediaengine/channel | Identitas channel dan katalog sumber tanpa membocorkan kredensial. |
| internal/mediaengine/video | Katalog channel dan informasi playback untuk REST. |
| internal/mediaengine/audio | Decode PCM, analisis/distribusi audio dan metrik. |
| internal/mediaengine/streaming | Lifecycle restream. |
| internal/mediaengine/saving | Rekaman, manifest, antrean upload, recovery media. |
| internal/mediaengine/media | Kontrak proses serta adapter FFmpeg/MediaMTX. |
| internal/cloud | Model pesan, UseCase komunikasi, client HTTP, journal, dan scheduler cloud. |
| internal/infra | Logger, DNS, filesystem/S3, subprocess, lifecycle goroutine. |
| scripts | Build dan deployment LAN/webcam/sumber sintetis. |

Handler membaca request dan memanggil UseCase. UseCase mengoordinasi aturan/layanan melalui interface. Repository mengelola penyimpanan dan tidak memanggil repository lain. Adapter infrastruktur tidak bergantung pada handler atau UseCase.

Handler REST menerima interface kecil yang didefinisikan di file handler pemakainya: `RecordingUseCase` untuk saving, `RestreamUseCase` untuk streaming, `ChannelUseCase` untuk video, dan `EngineUseCase` untuk status engine. Container tetap membangun implementasi konkret dan menyuntikkannya melalui constructor. Interface engine memakai tipe hasil `usecase.EngineStatus` yang sudah ada; model tidak dipindahkan. WHEP, WebSocket, dan health cloud-app juga sudah menerima interface. DTO, helper response, dan container tetap konkret.

## Indeks file

### Root

| File | Fungsi |
| --- | --- |
| [.env.example](.env.example) | Template konfigurasi engine media dan komunikasi cloud. |
| [.env.lan.example](.env.lan.example) | Template domain, LAN, DNS/hosting, router, dan komunikasi cloud. |
| [.env.webcam.example](.env.webcam.example) | Template pengembangan dengan sumber webcam/mic atau media sintetis lokal. |
| [.gitignore](.gitignore) | Mengabaikan env privat, data, runtime, executable, binary unduhan, dan hasil coverage; mengecualikan panduan ini dari aturan ignore Markdown induk. |
| [ALUR-END-TO-END.md](ALUR-END-TO-END.md) | Alur startup, playback LAN, command journal, rekaman/upload, metadata, dan restart. |
| [ENGINE-CLOUD-CONTRACT.md](ENGINE-CLOUD-CONTRACT.md) | Kontrak pendamping cloud untuk command, heartbeat, metadata, dan tiket playback. |
| [go.mod](go.mod) | Identitas module `kicau.local/edge`, versi Go, serta daftar dependency. |
| [go.sum](go.sum) | Checksum dependency untuk verifikasi modul yang diunduh. Dikelola tooling Go. |
| [PETA-KODE.md](PETA-KODE.md) | Indeks fungsi folder dan file engine media. |
| [README.md](README.md) | Panduan menjalankan engine, env, API, LAN/HTTPS, dan batas operasi. |


### cmd/api

| File | Fungsi |
| --- | --- |
| [main.go](cmd/api/main.go) | Entry point server. Memasang logging/timezone, membaca `-env`, memuat konfigurasi, lalu memanggil `server.Run()`. Opsi `-render-config` hanya merender konfigurasi MediaMTX melalui builder aplikasi. |


### cmd/lan-config

| File | Fungsi |
| --- | --- |
| [main.go](cmd/lan-config/main.go) | Membuat/membaca `.env.lan`, memvalidasi domain/IP/subnet, merender `app.env` dan Caddyfile. Mode `-serve-proxy` menjalankan Caddy ber-plugin Hostinger; tidak mengubah DNS publik atau router otomatis. |


### config

| File | Fungsi |
| --- | --- |
| [config.go](config/config.go) | Mendefinisikan `Config`, memuat dotenv/env, mengubah nilai menjadi tipe Go, menetapkan default/path turunan, serta memvalidasi konfigurasi. Tidak menjalankan kamera atau menulis konfigurasi MediaMTX. |
| [mediamtx-webcam.yml](config/mediamtx-webcam.yml) | Template MediaMTX **sumber pengujian** pada RTSP loopback port 8555 dengan path `webcam`. Berbeda dari MediaMTX utama aplikasi pada port 8554. |


### internal/app

| File | Fungsi |
| --- | --- |
| [container_test.go](internal/app/container_test.go) | Memastikan snapshot channel dipetakan ke konfigurasi media tanpa harus menjalankan proses. |
| [container.go](internal/app/container.go) | Perakitan katalog channel, server media, domain, journal, client cloud, dan handler. |


### internal/apperror

| File | Fungsi |
| --- | --- |
| [errors.go](internal/apperror/errors.go) | Jenis error bersama seperti invalid/conflict/unavailable/unauthorized/forbidden, serta pembungkus pesan publik dengan penyebab tetap dapat diperiksa. |


### internal/core/identifier

| File | Fungsi |
| --- | --- |
| [id.go](internal/core/identifier/id.go) | Memeriksa format ID menggunakan pola yang disepakati. Validasi keberadaan/otorisasi ID tetap dilakukan UseCase. |


### internal/delivery

| File | Fungsi |
| --- | --- |
| [architecture_test.go](internal/delivery/architecture_test.go) | Memeriksa arah dependency, larangan `pkg`, import adapter konkret/HTTP dari UseCase, hubungan repository, dan koordinasi lintas domain. |

| [router_test.go](internal/delivery/router_test.go) | Menguji kontrak HTTP/WebSocket dan memastikan menghentikan restream tidak menghentikan domain lain. |
| [router.go](internal/delivery/router.go) | Perakitan route API media, WHEP, health/readiness, dan middleware. |


### internal/delivery/middleware

| File | Fungsi |
| --- | --- |
| [auth_test.go](internal/delivery/middleware/auth_test.go) | Menguji perilaku auth dan pembatasan origin. |
| [auth.go](internal/delivery/middleware/auth.go) | Memeriksa token operator HTTP/WebSocket dan memasang identitas; CORS ditangani middleware tersendiri. |
| [logger.go](internal/delivery/middleware/logger.go) | Memasang requestId pada context/header dan mencatat status, route template, latency, serta IP menggunakan logger context. Tidak mencatat body, query atau kredensial. |
| [cors.go](internal/delivery/middleware/cors.go) | Satu kebijakan origin/CORS bersama untuk REST, WHEP, dan handshake WebSocket; menangani preflight sebelum autentikasi. |
| [headers.go](internal/delivery/middleware/headers.go) | Memasang Cache-Control no-store dan X-Content-Type-Options nosniff pada seluruh response. |
| [limits.go](internal/delivery/middleware/limits.go) | Membatasi body request tanpa mengatur header response. |


### internal/delivery/rest/response

| File | Fungsi |
| --- | --- |
| [response.go](internal/delivery/rest/response/response.go) | `WriteError()` menerjemahkan jenis error aplikasi menjadi status HTTP dan pesan aman untuk klien. |


### internal/delivery/rest/engine

| File | Fungsi |
| --- | --- |
| [handler.go](internal/delivery/rest/engine/handler.go) | Response HTTP readiness dan status melalui UseCase. |


### internal/infra/logger

| File | Fungsi |
| --- | --- |
| [logger.go](internal/infra/logger/logger.go) | Konfigurasi Zap, console development/JSON production, file JSON app.log, caller/stacktrace, identitas service/hostname, helper global, propagation requestId melalui context, dan flush/close sink saat shutdown. |
| [redact.go](internal/infra/logger/redact.go) | Core filter untuk menyensor field sensitif, data terstruktur bersarang, dan URL pada pesan/field sebelum masuk ke sink. |


### internal/infra/network/dns

| File | Fungsi |
| --- | --- |
| [server_test.go](internal/infra/network/dns/server_test.go) | Menguji resolusi nama arena melalui UDP/TCP ketika upstream internet tidak tersedia. |
| [server.go](internal/infra/network/dns/server.go) | Melayani DNS UDP/TCP untuk nama arena dari konfigurasi lokal, membatasi sumber klien menurut CIDR, dan meneruskan nama lain ke upstream dengan batas concurrency/timeout. Nama lokal tetap dapat dijawab tanpa internet. |


### internal/infra/storage/filesystem

| File | Fungsi |
| --- | --- |
| [disk.go](internal/infra/storage/filesystem/disk.go) | Adapter `Disk`: menyiapkan direktori, membaca ukuran file, rename partial menjadi final, dan membaca kapasitas bebas. |
| [free_linux.go](internal/infra/storage/filesystem/free_linux.go) | Implementasi Linux untuk kapasitas disk dan exclusive file lock. |
| [free_windows.go](internal/infra/storage/filesystem/free_windows.go) | Implementasi Windows untuk kapasitas disk dan exclusive file lock. |
| [json.go](internal/infra/storage/filesystem/json.go) | Penulisan JSON melalui temporary file, flush, dan rename. |
| [lock_test.go](internal/infra/storage/filesystem/lock_test.go) | Memastikan lock mencegah dua instance memakai lokasi yang sama. |


### internal/infra/storage/s3

| File | Fungsi |
| --- | --- |
| [lazy_test.go](internal/infra/storage/s3/lazy_test.go) | Menguji tidak ada inisialisasi sebelum upload dan kegagalan inisialisasi dapat diulang. |
| [lazy.go](internal/infra/storage/s3/lazy.go) | `LazyUploader` menunda inisialisasi SDK/credential provider sampai upload pertama, sehingga cloud tidak menghambat startup arena. Inisialisasi yang gagal dapat dicoba kembali. |
| [s3_test.go](internal/infra/storage/s3/s3_test.go) | Menguji upload memakai object key stabil. |
| [s3.go](internal/infra/storage/s3/s3.go) | Konfigurasi `Options` dan adapter `Uploader` untuk upload file ke bucket S3-compatible menggunakan object key dari UseCase. |


### internal/delivery/whep

| File | Fungsi |
| --- | --- |
| [handler.go](internal/delivery/whep/handler.go) | Gateway WHEP: parsing, panggilan otorisasi UseCase, proxy loopback, rewrite Location. |
| [router.go](internal/delivery/whep/router.go) | Mendaftarkan POST/PATCH/DELETE signaling pada group playback yang sudah diberi middleware origin. |

### internal/delivery/websocket

| File | Fungsi |
| --- | --- |
| [handler.go](internal/delivery/websocket/handler.go) | Mengambil ID mediaengine/channel/context request, meminta subscription UseCase, lalu menyerahkan pengiriman ke pengelola WebSocket. |
| [router.go](internal/delivery/websocket/router.go) | Mendaftarkan endpoint audio `/channels/:id/audio` dan metrik `/metrics` di bawah `/api/v1`. |
| [websocket_test.go](internal/delivery/websocket/websocket_test.go) | Menguji cleanup koneksi WebSocket saat stream ditutup. |
| [websocket.go](internal/delivery/websocket/websocket.go) | Upgrade WebSocket, pemeriksaan origin, pengiriman frame, ping/pong, deadline, dan penutupan koneksi/subscription. |


### internal/mediaengine/audio/metricsclient

| File | Fungsi |
| --- | --- |
| [client.go](internal/mediaengine/audio/metricsclient/client.go) | Mengirim satu objek metrik JSON ke `BACKEND_METRICS_URL` memakai Bearer token dan timeout. Respons 2xx dianggap diterima; redirect tidak diikuti. |


### internal/mediaengine/audio/model

| File | Fungsi |
| --- | --- |
| [analyzer_test.go](internal/mediaengine/audio/model/analyzer_test.go) | Menguji hasil perhitungan energi audio. |
| [analyzer.go](internal/mediaengine/audio/model/analyzer.go) | `Analyze()` menghitung peak, RMS, nilai dBFS, dan durasi aktif dari PCM 16-bit little-endian. |
| [metric.go](internal/mediaengine/audio/model/metric.go) | Struktur metrik audio per channel, termasuk waktu, peak/RMS, durasi aktif, dan jendela pengukuran. |


### internal/mediaengine/audio/pubsub

| File | Fungsi |
| --- | --- |
| [broadcaster.go](internal/mediaengine/audio/pubsub/broadcaster.go) | `Broadcaster` mendistribusikan payload berdasarkan topic ke subscriber dalam memori. Subscriber lambat dapat kehilangan frame ketika buffer penuh sehingga capture tidak tertahan. |


### internal/mediaengine/audio/usecase

| File | Fungsi |
| --- | --- |
| [audio_test.go](internal/mediaengine/audio/usecase/audio_test.go) | Memastikan subscription audio per channel terpisah dari subscription metrik global dan memvalidasi channel. |
| [audio.go](internal/mediaengine/audio/usecase/audio.go) | Mengatur capture tiap channel, analisis PCM, publikasi mediaengine/audio/metrik, subscription, dan pengiriman metrik melalui interface. Worker menerima callback dari UseCase. |


### internal/mediaengine/audio/worker

| File | Fungsi |
| --- | --- |
| [backend.go](internal/mediaengine/audio/worker/backend.go) | Antrean metrik best effort dan satu loop pengiriman ke backend. Metrik yang gagal dikirim tidak menjadi outbox persisten. |
| [capture.go](internal/mediaengine/audio/worker/capture.go) | Loop capture per channel, jeda reconnect setelah gangguan, dan penghentian melalui context. |
| [worker_test.go](internal/mediaengine/audio/worker/worker_test.go) | Menguji reconnect/cancel, batas antrean, backend gagal, dan shutdown ketika pengiriman berlangsung. |
| [worker.go](internal/mediaengine/audio/worker/worker.go) | State worker, antrean metrik terbatas, jobgroup, dan lifecycle Start/Close. |


### internal/mediaengine/channel/model

| File | Fungsi |
| --- | --- |
| [channel.go](internal/mediaengine/channel/model/channel.go) | Identitas channel yang aman untuk dipakai domain, tanpa URL upstream atau kredensial kamera. |


### internal/mediaengine/channel/repository

| File | Fungsi |
| --- | --- |
| [catalog_test.go](internal/mediaengine/channel/repository/catalog_test.go) | Menguji resolusi lokal, input tidak valid/duplikat, percent-encoding kredensial, dan error parser yang tidak membocorkan URL rahasia. |
| [catalog.go](internal/mediaengine/channel/repository/catalog.go) | Membentuk `Catalog` dari sumber konfigurasi. `Channels()` memberikan identitas; `HubRTSP()` memberikan URL hub lokal; `Inputs()` memberi snapshot upstream khusus perakitan infrastruktur. |


### internal/mediaengine/media

| File | Fungsi |
| --- | --- |
| [process.go](internal/mediaengine/media/process.go) | Interface `Process`: operasi untuk mengamati selesainya proses, membaca error, dan menghentikannya tanpa dependency implementasi OS. |
| [status.go](internal/mediaengine/media/status.go) | Kontrak data status proses media yang dapat ditampilkan arena. Status proses berjalan belum menjamin setiap kamera sehat. |


### internal/mediaengine/media/ffmpeg

| File | Fungsi |
| --- | --- |
| [audio.go](internal/mediaengine/media/ffmpeg/audio.go) | `AudioDecoder` menjalankan FFmpeg untuk menghasilkan PCM dan meneruskannya ke callback capture. |
| [input.go](internal/mediaengine/media/ffmpeg/input.go) | Menyusun argumen input RTSP bersama: TCP, timeout, serta pembatasan output log. |
| [recording.go](internal/mediaengine/media/ffmpeg/recording.go) | `Recorder` merekam video stream-copy dan audio AAC opsional ke fragmented MP4. Mengembalikan kontrak `media.Process`. |
| [streaming.go](internal/mediaengine/media/ffmpeg/streaming.go) | `Restreamer` melakukan encode H.264/AAC dan mengirim ke RTSP atau keluaran FLV untuk target RTMP/RTMPS. |


### internal/mediaengine/media/mediamtx

| File | Fungsi |
| --- | --- |
| [server_test.go](internal/mediaengine/media/mediamtx/server_test.go) | Menguji mapping sumber/konfigurasi, CORS, pembatasan akses, snapshot options, gagal start/retry, dan shutdown proses. |
| [server.go](internal/mediaengine/media/mediamtx/server.go) | `Server` menerima snapshot sumber melalui `Options`, merender konfigurasi upstream/RTSP/WebRTC/auth, membentuk URL WHEP, menjalankan/menutup MediaMTX, serta melaporkan status proses. |


### internal/delivery/rest/saving

| File | Fungsi |
| --- | --- |
| [handler.go](internal/delivery/rest/saving/handler.go) | Parsing request mulai/stop/daftar rekaman, memanggil UseCase, dan membentuk response yang aman untuk API. |
| [router.go](internal/delivery/rest/saving/router.go) | Mendaftarkan `/media/recording-sessions`, alias lama `/rounds`, endpoint stop, dan `/recordings`. Semua berada di bawah `/api/v1`. |


### internal/mediaengine/saving/model

| File | Fungsi |
| --- | --- |
| [model.go](internal/mediaengine/saving/model/model.go) | Permintaan mulai sesi dan metadata rekaman: channel, file, status, waktu, object key, dan informasi retry/upload. |


### internal/mediaengine/saving/repository

| File | Fungsi |
| --- | --- |
| [recording_test.go](internal/mediaengine/saving/repository/recording_test.go) | Memastikan schema manifest lama tetap dapat dibaca dan ditulis kembali. |
| [recording.go](internal/mediaengine/saving/repository/recording.go) | Repository `Files` membaca dan menyimpan manifest rekaman JSON dengan penulisan sementara, sinkronisasi, dan rename atomik. |


### internal/mediaengine/saving/usecase

| File | Fungsi |
| --- | --- |
| [saving_test.go](internal/mediaengine/saving/usecase/saving_test.go) | Menguji lifecycle rekaman, validasi, recovery partial, disk guard, kegagalan spawn, retry upload setelah restart, dan cleanup kegagalan startup. |
| [saving.go](internal/mediaengine/saving/usecase/saving.go) | Validasi/reservasi sesi, start/stop recorder, finalisasi, recovery manifest, daftar rekaman, batas disk/durasi, pemilihan file upload, serta state/retry upload. Fungsi `uploadPending()` adalah keputusan bisnis satu putaran upload. |


### internal/mediaengine/saving/worker

| File | Fungsi |
| --- | --- |
| [guard.go](internal/mediaengine/saving/worker/guard.go) | Timer berkala untuk memanggil pemeriksaan batas disk/durasi dari UseCase. |
| [recording.go](internal/mediaengine/saving/worker/recording.go) | Menunggu proses recorder selesai, menjalankan callback finalisasi, menghentikan proses, dan menunggu seluruh completion. |
| [upload.go](internal/mediaengine/saving/worker/upload.go) | `StartUploadLoop()` menjadwalkan callback upload; `UploadWithTimeout()` memberi deadline satu upload. Pemilihan file dan retry state tetap di UseCase. |
| [worker_test.go](internal/mediaengine/saving/worker/worker_test.go) | Menguji urutan siklus upload, penundaan guard, penantian finalisasi, timeout/cancel upload, dan shutdown proses recorder. |
| [worker.go](internal/mediaengine/saving/worker/worker.go) | Lifecycle worker recording/upload/guard dengan jobgroup dan context bersama. |


### internal/delivery/rest/streaming

| File | Fungsi |
| --- | --- |
| [handler.go](internal/delivery/rest/streaming/handler.go) | Handler list/start/stop restream; memetakan request ke UseCase. |
| [router.go](internal/delivery/rest/streaming/router.go) | Mendaftarkan GET/POST `/restreams` dan DELETE `/restreams/:target` di bawah `/api/v1`. |


### internal/mediaengine/streaming/model

| File | Fungsi |
| --- | --- |
| [model.go](internal/mediaengine/streaming/model/model.go) | Payload permintaan start serta tampilan status restream per target/channel. |


### internal/mediaengine/streaming/usecase

| File | Fungsi |
| --- | --- |
| [streaming_test.go](internal/mediaengine/streaming/usecase/streaming_test.go) | Menguji allowlist tujuan dan pembatalan restream. |
| [streaming.go](internal/mediaengine/streaming/usecase/streaming.go) | Validasi mediaengine/channel/allowlist target, konflik target, state restream, start/stop, dan koordinasi adapter melalui executor. |


### internal/mediaengine/streaming/worker

| File | Fungsi |
| --- | --- |
| [restream.go](internal/mediaengine/streaming/worker/restream.go) | Menjalankan callback pembuat proses, mengawasi proses, retry, mengirim notifikasi running, serta membatalkan target tertentu. |
| [worker_test.go](internal/mediaengine/streaming/worker/worker_test.go) | Menguji retry restream, stop per target, dan cancel selama menunggu retry. |
| [worker.go](internal/mediaengine/streaming/worker/worker.go) | Menyimpan pekerjaan per target, jobgroup, serta lifecycle worker. |


### internal/delivery/rest/video

| File | Fungsi |
| --- | --- |
| [handler.go](internal/delivery/rest/video/handler.go) | Memanggil UseCase daftar channel dan mengirim response HTTP. |

| [router.go](internal/delivery/rest/video/router.go) | Mendaftarkan `GET /api/v1/channels` pada route group yang diberikan. |


### internal/mediaengine/video/model

| File | Fungsi |
| --- | --- |
| [model.go](internal/mediaengine/video/model/model.go) | Bentuk data channel video yang disediakan ke konsumen: ID dan URL WHEP. |


### internal/mediaengine/video/usecase

| File | Fungsi |
| --- | --- |
| [video.go](internal/mediaengine/video/usecase/video.go) | Menggabungkan identitas dari interface katalog dengan URL dari interface playback menjadi daftar channel video. |


### internal/server

| File | Fungsi |
| --- | --- |
| [media_test.go](internal/server/media_test.go) | Menguji kegagalan/restart proses media tidak membatalkan context engine HTTP. |
| [media.go](internal/server/media.go) | Mengawasi MediaMTX dan mencoba restart secara independen. Menjalankan modul media yang sehat serta menutup modul yang gagal tanpa menjatuhkan seluruh aplikasi. |
| [server_test.go](internal/server/server_test.go) | Menguji isolasi kegagalan startup modul dan shutdown berurutan yang idempotent. |
| [server.go](internal/server/server.go) | Lock, listener, supervisor media, startup worker, dan shutdown engine. |


### scripts

| File | Fungsi |
| --- | --- |
| [build-local.ps1](scripts/build-local.ps1) | Membangun executable API dan renderer LAN ke bin/ sebelum deployment. |
| [build-proxy.ps1](scripts/build-proxy.ps1) | Membangun Caddy dengan versi/plugin Hostinger yang ditentukan script; butuh persiapan online. |
| [configure-lan.ps1](scripts/configure-lan.ps1) | Wrapper executable `lan-config` untuk menyiapkan `.env.lan` dan hasil render. |
| [start-lan.ps1](scripts/start-lan.ps1) | Merender konfigurasi, menjalankan binary aplikasi dan proxy tanpa jendela tambahan, mengawasi proses, lalu menghentikan proses miliknya saat selesai. |
| [start-webcam-test.ps1](scripts/start-webcam-test.ps1) | Listing perangkat DirectShow, inisialisasi env uji, dan publikasi webcam/mic atau media sintetis ke server RTSP sumber uji. Tidak menjalankan API Go secara otomatis. |

## Direktori operasional

DATA_DIR berisi media/manifest, commands/, dan media-updates/. RUNTIME_DIR berisi konfigurasi proses yang dihasilkan; bin/ berisi executable dan logs/ berisi log. SQLite lama tidak dipakai atau dihapus otomatis.

### internal/delivery/rest

| File | Fungsi |
| --- | --- |
| [health.go](internal/delivery/rest/health.go) | Handler global `/healthz` dan preflight HTTP. Health menandakan HTTP hidup, bukan seluruh kamera/cloud siap. |

### internal/cloud/model

| File | Fungsi |
| --- | --- |
| [command.go](internal/cloud/model/command.go) | Command media, identitas tujuan, expiry, dan hasil eksekusi. |
| [heartbeat.go](internal/cloud/model/heartbeat.go) | Payload status operasional engine untuk heartbeat. |
| [playback.go](internal/cloud/model/playback.go) | Permintaan otorisasi tiket dan grant playback terikat scope. |
| [recording_metadata.go](internal/cloud/model/recording_metadata.go) | Metadata versi/lifecycle rekaman dan referensi object storage. |

### internal/cloud/usecase

| File | Fungsi |
| --- | --- |
| [connection.go](internal/cloud/usecase/connection.go) | Interface dependency, options, constructor, satu Engine, mutex, dan state heartbeat bersama. |
| [status.go](internal/cloud/usecase/status.go) | Status koneksi cloud, kesegaran heartbeat, dan kesiapan engine. |
| [heartbeat.go](internal/cloud/usecase/heartbeat.go) | Menyusun heartbeat dan memeriksa acknowledgment cloud. |
| [playback.go](internal/cloud/usecase/playback.go) | Memvalidasi channel, tiket, scope grant, dan expiry sebelum playback. |
| [recording_metadata.go](internal/cloud/usecase/recording_metadata.go) | Membaca manifest, membuat hash update, menulis antrean, mengirim metadata, dan memeriksa acknowledgment. |
| [command_polling.go](internal/cloud/usecase/command_polling.go) | Polling command, validasi identitas, deduplikasi/recovery, persistensi hasil, dan acknowledgment. |
| [command_polling_test.go](internal/cloud/usecase/command_polling_test.go) | Tes command kedaluwarsa, kegagalan laporan, dan deduplikasi setelah restart; tidak dijalankan otomatis. |
| [command_execution.go](internal/cloud/usecase/command_execution.go) | CommandExecutor mengoordinasi tindakan recording/restream dan rekonsiliasi setelah restart. |

### internal/cloud/client

| File | Fungsi |
| --- | --- |
| [http.go](internal/cloud/client/http.go) | Satu helper request HTTP, header/request ID, encoding/decoding envelope, timeout, dan error transport aman. |
| [command_api.go](internal/cloud/client/command_api.go) | Menentukan metode/path dan payload polling serta pelaporan hasil command. |

### internal/cloud/repository

| File | Fungsi |
| --- | --- |
| [journal.go](internal/cloud/repository/journal.go) | Journal JSON command/hasil/ack dan antrean metadata persisten, tanpa repository lain. |

### internal/cloud/worker

| File | Fungsi |
| --- | --- |
| [scheduler.go](internal/cloud/worker/scheduler.go) | Lifecycle dan penjadwalan pekerjaan polling, heartbeat, serta metadata melalui jobgroup. |
