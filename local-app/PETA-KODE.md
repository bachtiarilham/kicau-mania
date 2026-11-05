# Peta folder dan file untuk developer

Panduan ini menjelaskan fungsi setiap file source, konfigurasi contoh, script, tes, dan dokumentasi di `local-app`, serta file aplikasi pasangannya di `cloud-app`. Deskripsi mengikuti implementasi yang ada, termasuk pemecahan UseCase arena. Direktori hasil build, data operasional, dan dependency unduhan dijelaskan sebagai kelompok karena isinya berubah setiap menjalankan aplikasi.

Gunakan dokumen ini sebagai indeks: cari fitur yang sedang dikerjakan, lalu buka file terkait. Alur transaksi lengkap ada di [ALUR-END-TO-END.md](ALUR-END-TO-END.md), sedangkan cara menjalankan dan menyiapkan LAN ada di [OFFLINE-FIRST.md](OFFLINE-FIRST.md).

## 1. Mulai membaca dari mana?

| Tujuan | Urutan file yang dibaca |
| --- | --- |
| Memahami submit nilai | [model arena](internal/core/arena/model/event.go) → [handler `submit()`](internal/core/arena/delivery/handler.go) → [UseCase `Submit()`](internal/core/arena/usecase/scoring.go) → [repository `UpdateScores()`](internal/core/arena/repository/sqlite.go) |
| Memahami login offline | [handler arena](internal/core/arena/delivery/handler.go) → [auth.go](internal/core/arena/usecase/auth.go) → [repository sesi](internal/core/arena/repository/sqlite.go) |
| Memahami lifecycle ronde | [round.go](internal/core/arena/usecase/round.go) → [model arena](internal/core/arena/model/event.go) → [tes skenario penilaian](internal/core/arena/usecase/arena_test.go) |
| Memahami startup aplikasi | [cmd/api/main.go](cmd/api/main.go) → [config.go](config/config.go) → [server.go](internal/server/server.go) → [container.go](internal/app/container.go) |
| Memahami live video lokal | [mediaengine/video/usecase](internal/mediaengine/video/usecase/video.go) → [MediaMTX Server](internal/mediaengine/media/mediamtx/server.go) → [player browser](internal/core/arena/delivery/web/app.js) |
| Memahami recording dan upload | [mediaengine/saving/usecase](internal/mediaengine/saving/usecase/saving.go) → [manifest repository](internal/mediaengine/saving/repository/recording.go) → [worker](internal/mediaengine/saving/worker/worker.go) → [S3 uploader](internal/infra/storage/s3/s3.go) |
| Memahami hubungan arena–cloud | [sync.go](internal/edgetocloud/synchronization/usecase/sync.go) → [HTTP client](internal/edgetocloud/synchronization/client/http.go) → [cloud UseCase](../cloud-app/internal/exchange/usecase/exchange.go) |

## 2. Arti nama folder dan batas tanggung jawab

```text
local-app/
├── cmd/                 Pintu masuk executable dan alat command-line
├── config/              Pembacaan env dan konfigurasi sumber uji
├── internal/
│   ├── core/            Bisnis lomba: arena dan pemeriksaan identifier
│   │   ├── arena/       Event, akun juri, penilaian, rekap, dan UI lokal
│   │   └── identifier/  Pemeriksaan format identifier
│   ├── mediaengine/     Engine media dan domain pengguna media
│   │   ├── channel/    Katalog identitas channel dan sumbernya
│   │   ├── video/      Daftar channel dan alamat live
│   │   ├── audio/      PCM, metrik audio, dan WebSocket
│   │   ├── streaming/  Restream ke target lain
│   │   ├── saving/     Rekaman dan antrean upload
│   │   └── media/      Kontrak proses dan adapter MediaMTX/FFmpeg
│   ├── edgetocloud/     Sinkronisasi pusat dan store-and-forward
│   │   ├── synchronization/ Pertukaran perubahan, paket, dan perintah
│   │   ├── exchange/   Bentuk pesan arena–cloud
│   │   └── operation/  Koordinasi perintah media lintas domain
│   ├── infra/           OS, filesystem, storage, dan network
│   │   ├── network/    DNS lokal arena
│   │   ├── storage/    Filesystem dan object storage
│   │   └── runtime/    Child process dan lifecycle goroutine
│   ├── app/             Merakit dependency konkret
│   ├── server/          Startup, shutdown, dan supervisi modul
│   ├── delivery/http/   Router, middleware, dan respons HTTP bersama
│   └── apperror/        Jenis error aplikasi bersama
└── scripts/             Persiapan, menjalankan lingkungan, dan tes integrasi
```

`core`, `mediaengine`, `edgetocloud`, dan `infra` adalah kelompok folder dalam satu module dan executable, bukan layanan terpisah. `edgetocloud` menangani kontrak dan koordinasi komunikasi pusat, sedangkan `infra` menyediakan adapter OS, filesystem, storage, dan jaringan. Autentikasi juri tetap di `core/arena/usecase/auth.go`; middleware operator tetap di shared HTTP. DNS lokal menjawab hostname dari `LOCAL_PUBLIC_URL`; refactor ini tidak mengubah domain yang telah dikonfigurasi.

Subfolder domain memakai pola berikut. Sebuah domain hanya memiliki subfolder yang dibutuhkan; misalnya `video` tidak memiliki repository atau worker sendiri.

| Nama | Fungsi dan batasnya |
| --- | --- |
| `model/` | Bentuk data domain dan perhitungan murni yang terkait. |
| `delivery/` | Parsing request, validasi sintaks, ekstraksi auth/context, memanggil UseCase, dan mengirim DTO/HTTP/WebSocket. |
| `usecase/` | Validasi semantik, aturan bisnis, serta koordinasi repository, service, dan domain. Dependency dinyatakan melalui interface. |
| `repository/` | Membaca/menulis data ke penyimpanan atau katalog. Tidak memanggil repository lain. |
| `worker/` | Menjalankan callback background, retry, timer, cancellation, dan menunggu pekerjaan selesai. Keputusan bisnis tetap di UseCase. |
| `client/` | Adapter request ke layanan luar. Di sini dipakai untuk HTTP cloud. |
| `doc.go` | Komentar penjelas tanggung jawab package; tidak menjalankan fitur tersendiri. |
| `*_test.go` | Pengujian perilaku. Tidak ikut menjadi kode runtime executable produksi. |
| `*_windows.go`, `*_linux.go` | Implementasi khusus OS; Go memilih file sesuai target build. |

Arah utama: **Handler → UseCase → Repository → penyimpanan/infrastruktur**. UseCase juga memanggil service melalui interface. Implementasi dependency dipasang oleh `app`; koordinasi antar domain berada pada UseCase.

## 3. File di akar `local-app`

| File | Fungsi |
| --- | --- |
| [go.mod](go.mod) | Identitas module `kicau.local/edge`, versi Go, serta daftar dependency. |
| [go.sum](go.sum) | Checksum dependency untuk verifikasi modul yang diunduh. Dikelola tooling Go. |
| [.gitignore](.gitignore) | Mengabaikan env privat, data, runtime, executable, binary unduhan, dan hasil coverage; mengecualikan panduan ini dari aturan ignore Markdown induk. |
| [.env.example](.env.example) | Template konfigurasi umum: channel RTSP, API, media, recording, S3, dan opsi offline/cloud. |
| [.env.webcam.example](.env.webcam.example) | Template pengembangan dengan sumber webcam/mic atau media sintetis lokal. |
| [.env.lan.example](.env.lan.example) | Template deployment LAN: domain cloud/lokal, IP/subnet, Hostinger, router, backup, dan cloud exchange. |
| [README.md](README.md) | Pintu masuk panduan proyek, cara menjalankan, kontrak media, struktur, dan jalur baca developer. |
| [OFFLINE-FIRST.md](OFFLINE-FIRST.md) | Panduan operasional: paket event, akun juri, konfigurasi LAN/HTTPS/DNS, backup, serta tes gangguan. |
| [ALUR-END-TO-END.md](ALUR-END-TO-END.md) | Menjelaskan perjalanan data dari persiapan sampai penilaian, rekap, media, sinkronisasi, dan recovery menggunakan diagram. |
| [PETA-KODE.md](PETA-KODE.md) | Indeks tanggung jawab folder/file yang sedang dibaca. |

File privat `.env`, `.env.webcam`, dan `.env.lan` adalah konfigurasi aktual milik lingkungan masing-masing. `-env` menentukan file aplikasi yang dibaca; `.env.lan` dipakai alat deployment untuk merender konfigurasi aplikasi/proxy. Environment proses mengungguli nilai file. Isi token, password, dan URL berkredensial tidak disalin ke panduan ini.

## 4. `cmd/` — executable dan alat bantu

Setiap subfolder memiliki `main.go`, sehingga dapat dibangun atau dijalankan sebagai command terpisah.

| File | Fungsi |
| --- | --- |
| [cmd/api/main.go](cmd/api/main.go) | Entry point server. Memasang logging/timezone, membaca `-env`, memuat konfigurasi, lalu memanggil `server.Run()`. Opsi `-render-config` hanya merender konfigurasi MediaMTX melalui builder aplikasi. |
| [cmd/event-package/main.go](cmd/event-package/main.go) | Generator paket **demo**: peserta sintetis, ronde, kriteria, akun juri dengan hash bcrypt, serta CSV kredensial privat. Memerlukan `-demo`; bukan implementasi pendaftaran peserta resmi. |
| [cmd/lan-config/main.go](cmd/lan-config/main.go) | Membuat/membaca `.env.lan`, memvalidasi domain/IP/subnet, merender `app.env` dan Caddyfile. Mode `-serve-proxy` menjalankan Caddy ber-plugin Hostinger; tidak mengubah DNS publik atau router otomatis. |

## 5. `config/` — konfigurasi aplikasi

| File | Fungsi |
| --- | --- |
| [config/config.go](config/config.go) | Mendefinisikan `Config`, memuat dotenv/env, mengubah nilai menjadi tipe Go, menetapkan default/path turunan, serta memvalidasi konfigurasi. Tidak menjalankan kamera atau menulis konfigurasi MediaMTX. |
| [config/config_test.go](config/config_test.go) | Menguji default, prioritas environment, nilai kosong eksplisit, konfigurasi tidak valid, lokasi dotenv, dan opsi deployment offline/cloud. |
| [config/mediamtx-webcam.yml](config/mediamtx-webcam.yml) | Template MediaMTX **sumber pengujian** pada RTSP loopback port 8555 dengan path `webcam`. Berbeda dari MediaMTX utama aplikasi pada port 8554. |

## 6. `internal/app/` — perakitan dependency

Folder ini menjawab: **implementasi apa dipasang ke interface mana?**

| File | Fungsi |
| --- | --- |
| [internal/app/container.go](internal/app/container.go) | `NewMedia()` merakit katalog dan snapshot sumber untuk MediaMTX. `NewContainer()` merakit repository, adapter, worker, UseCase, dan handler; membuka SQLite serta menjalankan recovery arena. Menyediakan fallback recording repository yang tidak tersedia agar penilaian tetap dapat dijalankan. |
| [internal/app/container_test.go](internal/app/container_test.go) | Memastikan snapshot channel dipetakan ke konfigurasi media tanpa harus menjalankan proses. |
| [internal/app/doc.go](internal/app/doc.go) | Ringkasan tanggung jawab package perakitan aplikasi. |

## 7. `internal/server/` — lifecycle aplikasi

| File | Fungsi |
| --- | --- |
| [internal/server/server.go](internal/server/server.go) | `Run()` mengambil lock data/runtime, membuka listener, merakit aplikasi, menjalankan DNS/background/media, melayani HTTP, menangani sinyal, dan menutup resource. Database arena wajib tersedia; cloud dan media tidak menjadi syarat menerima nilai. |
| [internal/server/media.go](internal/server/media.go) | Mengawasi MediaMTX dan mencoba restart secara independen. Menjalankan modul media yang sehat serta menutup modul yang gagal tanpa menjatuhkan seluruh aplikasi. |
| [internal/server/server_test.go](internal/server/server_test.go) | Menguji isolasi kegagalan startup modul dan shutdown berurutan yang idempotent. |
| [internal/server/media_test.go](internal/server/media_test.go) | Menguji kegagalan/restart proses media tidak membatalkan context penilaian arena. |

`app` menentukan hubungan objek, sedangkan `server` menentukan kapan objek mulai dan berhenti. Keduanya dipisah agar lifecycle tidak tercampur dengan pembuatan dependency.

## 8. `internal/core/arena/` — bisnis lomba offline

Folder induk ini mengelola event, akun khusus event, penilaian, history revisi, rekap, dan UI lokal. Ini pusat alur bisnis lomba.

### `core/arena/model/` — bentuk data dan perhitungan

| File | Fungsi |
| --- | --- |
| [internal/core/arena/model/event.go](internal/core/arena/model/event.go) | Model paket, juri, kriteria, peserta, ronde, submission, revisi, audit, dan sesi. Mendefinisikan state ronde serta `Latest()` dan `Results()` untuk memilih revisi terakhir dan menghitung rata-rata. |

### `core/arena/usecase/` — aturan bisnis per alur

Seluruh file memakai package yang sama dan satu `UseCase`; mutex/state bersama tetap didefinisikan di `arena.go`.

| File | Fungsi |
| --- | --- |
| [internal/core/arena/usecase/arena.go](internal/core/arena/usecase/arena.go) | Interface repository/playback/package source, `Options`, state `UseCase`, constructor, serta fungsi bersama untuk ID operasi, hash, error, dan pembentukan perubahan outbox. |
| [internal/core/arena/usecase/event.go](internal/core/arena/usecase/event.go) | `Pull()` mengambil paket atas permintaan operator; `Prepare()` menyimpan paket secara idempotent; `validateBundle()` memeriksa identitas, versi, roster, bcrypt, kriteria, dan 24 lapak. |
| [internal/core/arena/usecase/auth.go](internal/core/arena/usecase/auth.go) | Login/logout lokal, verifikasi bcrypt, rate limit percobaan login, pembuatan token sesi, serta pemeriksaan expiry. Tidak membutuhkan login cloud. |
| [internal/core/arena/usecase/scoring.go](internal/core/arena/usecase/scoring.go) | `Submit()` memeriksa sesi, scope akun, ronde aktif, penugasan, peserta, kriteria 1–10, dan versi revisi. Menangani retry dengan `operation_id`, lalu menyimpan revisi dan outbox melalui transaksi repository. |
| [internal/core/arena/usecase/round.go](internal/core/arena/usecase/round.go) | `Transition()` menangani start/resume/close/cancel dan aturan kelengkapan. `Recover()` menandai ronde aktif sebagai `INTERRUPTED` setelah restart. |
| [internal/core/arena/usecase/query.go](internal/core/arena/usecase/query.go) | Membentuk katalog, tampilan juri sendiri/operator, rekap, history revisi, daftar state ronde, jumlah perubahan pending, dan status mediaengine/media/backup. Menyimpan tipe hasil query tanpa dependency HTTP. |
| [internal/core/arena/usecase/backup.go](internal/core/arena/usecase/backup.go) | Meminta snapshot database ke `BACKUP_DIR`, membuat nama file unik, dan memperbarui status waktu/kegagalan backup. |
| [internal/core/arena/usecase/arena_test.go](internal/core/arena/usecase/arena_test.go) | Skenario nilai/revisi, kelengkapan, privasi juri, scope akun, konflik, retry bersamaan, rollback outbox, sesi, crash setelah commit, recovery, dan restore snapshot. Helper tes juga tetap di file ini. |

### `core/arena/repository/` — database lokal

| File | Fungsi |
| --- | --- |
| [internal/core/arena/repository/sqlite.go](internal/core/arena/repository/sqlite.go) | SQLite WAL/FULL, schema dan migrasi history, metadata event, revisi append-only, sesi, outbox, penyimpanan command/hasil, serta snapshot `VACUUM INTO`. Nilai dan perubahan cloud di-commit bersama. |
| [internal/core/arena/repository/sqlite_test.go](internal/core/arena/repository/sqlite_test.go) | Menguji migrasi history revisi yang semula tertanam pada event menjadi tabel revisi, dengan data dan fingerprint tetap utuh. |

### `core/arena/delivery/` dan `core/arena/delivery/web/` — HTTP dan UI lokal

| File | Fungsi |
| --- | --- |
| [internal/core/arena/delivery/handler.go](internal/core/arena/delivery/handler.go) | Handler API arena, parsing payload, cookie juri, context, pemanggilan UseCase, dan response. Memasang route publik/juri/operator dan menyajikan asset UI melalui `go:embed`. |
| [internal/core/arena/delivery/web/index.html](internal/core/arena/delivery/web/index.html) | Struktur halaman: login juri/operator, pemilihan event, impor/pull paket, status, ronde, dan tempat player video. |
| [internal/core/arena/delivery/web/style.css](internal/core/arena/delivery/web/style.css) | Tampilan halaman, toolbar, kartu peserta, input, status, dan layout responsif. |
| [internal/core/arena/delivery/web/app.js](internal/core/arena/delivery/web/app.js) | Interaksi UI: request API, render peserta, draft/antrean IndexedDB, retry submission, deteksi perubahan state, aksi operator, ekspor history, dan player WebRTC/WHEP. Validasi bisnis final tetap dilakukan UseCase server. |
| [internal/core/arena/delivery/web/sw.js](internal/core/arena/delivery/web/sw.js) | Service worker untuk cache shell HTML/JS/CSS dan fallback ketika fetch asset gagal. Tidak meng-cache API, kredensial, skor, atau media. |

### `core/arena/worker/` — jadwal pekerjaan nonkritis

| File | Fungsi |
| --- | --- |
| [internal/core/arena/worker/worker.go](internal/core/arena/worker/worker.go) | Menjalankan callback sinkronisasi, polling command, dan backup dengan lifecycle jobgroup. Menjadwalkan siklus; aturan sinkronisasi/backup berada pada UseCase masing-masing. |

## 9. `internal/mediaengine/channel/` — katalog sumber kamera

`channel` menjawab **channel apa yang dikonfigurasi dan alamat RTSP lokalnya apa**. Koneksi upstream sesungguhnya dikelola MediaMTX.

| File | Fungsi |
| --- | --- |
| [internal/mediaengine/channel/model/channel.go](internal/mediaengine/channel/model/channel.go) | Identitas channel yang aman untuk dipakai domain, tanpa URL upstream atau kredensial kamera. |
| [internal/mediaengine/channel/repository/catalog.go](internal/mediaengine/channel/repository/catalog.go) | Membentuk `Catalog` dari sumber konfigurasi. `Channels()` memberikan identitas; `HubRTSP()` memberikan URL hub lokal; `Inputs()` memberi snapshot upstream khusus perakitan infrastruktur. |
| [internal/mediaengine/channel/repository/catalog_test.go](internal/mediaengine/channel/repository/catalog_test.go) | Menguji resolusi lokal, input tidak valid/duplikat, percent-encoding kredensial, dan error parser yang tidak membocorkan URL rahasia. |
| [internal/mediaengine/channel/repository/doc.go](internal/mediaengine/channel/repository/doc.go) | Ringkasan tanggung jawab repository katalog. |

## 10. `internal/mediaengine/video/` — daftar live video

Domain ini menyediakan metadata untuk membuka video. Transport media dijalankan MediaMTX; tidak ada worker encoding video tersendiri pada domain ini.

| File | Fungsi |
| --- | --- |
| [internal/mediaengine/video/model/model.go](internal/mediaengine/video/model/model.go) | Bentuk data channel video yang disediakan ke konsumen: ID dan URL WHEP. |
| [internal/mediaengine/video/usecase/video.go](internal/mediaengine/video/usecase/video.go) | Menggabungkan identitas dari interface katalog dengan URL dari interface playback menjadi daftar channel video. |
| [internal/mediaengine/video/delivery/handler.go](internal/mediaengine/video/delivery/handler.go) | Memanggil UseCase daftar channel dan mengirim response HTTP. |
| [internal/mediaengine/video/delivery/router.go](internal/mediaengine/video/delivery/router.go) | Mendaftarkan `GET /api/v1/channels` pada route group yang diberikan. |

## 11. `internal/mediaengine/audio/` — PCM, analisis, dan WebSocket

Alurnya: **RTSP lokal → decoder PCM → perhitungan metrik → broadcaster → subscriber WebSocket**, ditambah pengiriman statistik backend bila dikonfigurasi.

### `mediaengine/audio/model/` dan `mediaengine/audio/usecase/`

| File | Fungsi |
| --- | --- |
| [internal/mediaengine/audio/model/metric.go](internal/mediaengine/audio/model/metric.go) | Struktur metrik audio per channel, termasuk waktu, peak/RMS, durasi aktif, dan jendela pengukuran. |
| [internal/mediaengine/audio/model/analyzer.go](internal/mediaengine/audio/model/analyzer.go) | `Analyze()` menghitung peak, RMS, nilai dBFS, dan durasi aktif dari PCM 16-bit little-endian. |
| [internal/mediaengine/audio/model/analyzer_test.go](internal/mediaengine/audio/model/analyzer_test.go) | Menguji hasil perhitungan energi audio. |
| [internal/mediaengine/audio/usecase/audio.go](internal/mediaengine/audio/usecase/audio.go) | Mengatur capture tiap channel, analisis PCM, publikasi mediaengine/audio/metrik, subscription, dan pengiriman metrik melalui interface. Worker menerima callback dari UseCase. |
| [internal/mediaengine/audio/usecase/audio_test.go](internal/mediaengine/audio/usecase/audio_test.go) | Memastikan subscription audio per channel terpisah dari subscription metrik global dan memvalidasi channel. |

### `mediaengine/audio/delivery/` — koneksi browser

| File | Fungsi |
| --- | --- |
| [internal/mediaengine/audio/delivery/handler.go](internal/mediaengine/audio/delivery/handler.go) | Mengambil ID mediaengine/channel/context request, meminta subscription UseCase, lalu menyerahkan pengiriman ke pengelola WebSocket. |
| [internal/mediaengine/audio/delivery/router.go](internal/mediaengine/audio/delivery/router.go) | Mendaftarkan endpoint audio `/channels/:id/audio` dan metrik `/metrics` di bawah `/api/v1`. |
| [internal/mediaengine/audio/delivery/websocket.go](internal/mediaengine/audio/delivery/websocket.go) | Upgrade WebSocket, pemeriksaan origin, pengiriman frame, ping/pong, deadline, dan penutupan koneksi/subscription. |
| [internal/mediaengine/audio/delivery/websocket_test.go](internal/mediaengine/audio/delivery/websocket_test.go) | Menguji cleanup koneksi WebSocket saat stream ditutup. |

### `mediaengine/audio/pubsub/` — distribusi dalam proses

| File | Fungsi |
| --- | --- |
| [internal/mediaengine/audio/pubsub/broadcaster.go](internal/mediaengine/audio/pubsub/broadcaster.go) | `Broadcaster` mendistribusikan payload berdasarkan topic ke subscriber dalam memori. Subscriber lambat dapat kehilangan frame ketika buffer penuh sehingga capture tidak tertahan. |
| [internal/mediaengine/audio/pubsub/doc.go](internal/mediaengine/audio/pubsub/doc.go) | Ringkasan tanggung jawab distribusi mediaengine/audio/metrik. |

Pubsub ini memakai channel Go dalam executable yang sama. Tidak memerlukan server broker terpisah, dan tidak menyimpan nilai juri atau antrean sinkronisasi persisten.

### `mediaengine/audio/metricsclient/` — HTTP ke backend statistik

| File | Fungsi |
| --- | --- |
| [internal/mediaengine/audio/metricsclient/client.go](internal/mediaengine/audio/metricsclient/client.go) | Mengirim satu objek metrik JSON ke `BACKEND_METRICS_URL` memakai Bearer token dan timeout. Respons 2xx dianggap diterima; redirect tidak diikuti. |
| [internal/mediaengine/audio/metricsclient/doc.go](internal/mediaengine/audio/metricsclient/doc.go) | Ringkasan tanggung jawab client metrik backend. |

### `mediaengine/audio/worker/` — pekerjaan background audio

| File | Fungsi |
| --- | --- |
| [internal/mediaengine/audio/worker/worker.go](internal/mediaengine/audio/worker/worker.go) | State worker, antrean metrik terbatas, jobgroup, dan lifecycle Start/Close. |
| [internal/mediaengine/audio/worker/capture.go](internal/mediaengine/audio/worker/capture.go) | Loop capture per channel, jeda reconnect setelah gangguan, dan penghentian melalui context. |
| [internal/mediaengine/audio/worker/backend.go](internal/mediaengine/audio/worker/backend.go) | Antrean metrik best effort dan satu loop pengiriman ke backend. Metrik yang gagal dikirim tidak menjadi outbox persisten. |
| [internal/mediaengine/audio/worker/worker_test.go](internal/mediaengine/audio/worker/worker_test.go) | Menguji reconnect/cancel, batas antrean, backend gagal, dan shutdown ketika pengiriman berlangsung. |

Metrik audio membantu mengamati sinyal. Nilai resmi lomba berasal dari input juri di `arena`.

## 12. `internal/mediaengine/streaming/` — restream

Domain ini mengendalikan pengiriman channel ke target yang diizinkan. Live LAN tetap disediakan jalur MediaMTX/WebRTC.

| File | Fungsi |
| --- | --- |
| [internal/mediaengine/streaming/model/model.go](internal/mediaengine/streaming/model/model.go) | Payload permintaan start serta tampilan status restream per target/channel. |
| [internal/mediaengine/streaming/usecase/streaming.go](internal/mediaengine/streaming/usecase/streaming.go) | Validasi mediaengine/channel/allowlist target, konflik target, state restream, start/stop, dan koordinasi adapter melalui executor. |
| [internal/mediaengine/streaming/usecase/streaming_test.go](internal/mediaengine/streaming/usecase/streaming_test.go) | Menguji allowlist tujuan dan pembatalan restream. |
| [internal/mediaengine/streaming/delivery/handler.go](internal/mediaengine/streaming/delivery/handler.go) | Handler list/start/stop restream; memetakan request ke UseCase. |
| [internal/mediaengine/streaming/delivery/router.go](internal/mediaengine/streaming/delivery/router.go) | Mendaftarkan GET/POST `/restreams` dan DELETE `/restreams/:target` di bawah `/api/v1`. |
| [internal/mediaengine/streaming/worker/worker.go](internal/mediaengine/streaming/worker/worker.go) | Menyimpan pekerjaan per target, jobgroup, serta lifecycle worker. |
| [internal/mediaengine/streaming/worker/restream.go](internal/mediaengine/streaming/worker/restream.go) | Menjalankan callback pembuat proses, mengawasi proses, retry, mengirim notifikasi running, serta membatalkan target tertentu. |
| [internal/mediaengine/streaming/worker/worker_test.go](internal/mediaengine/streaming/worker/worker_test.go) | Menguji retry restream, stop per target, dan cancel selama menunggu retry. |

## 13. `internal/mediaengine/saving/` — rekaman dan upload

Domain ini mengelola **sesi rekaman media**. Nama method lama `StartRound`/`StopRound` di sini mengacu ke sesi rekaman, sedangkan lifecycle ronde penilaian berada di `core/arena/usecase/round.go`.

### `mediaengine/saving/model/`, `mediaengine/saving/usecase/`, dan `mediaengine/saving/repository/`

| File | Fungsi |
| --- | --- |
| [internal/mediaengine/saving/model/model.go](internal/mediaengine/saving/model/model.go) | Permintaan mulai sesi dan metadata rekaman: channel, file, status, waktu, object key, dan informasi retry/upload. |
| [internal/mediaengine/saving/usecase/saving.go](internal/mediaengine/saving/usecase/saving.go) | Validasi/reservasi sesi, start/stop recorder, finalisasi, recovery manifest, daftar rekaman, batas disk/durasi, pemilihan file upload, serta state/retry upload. Fungsi `uploadPending()` adalah keputusan bisnis satu putaran upload. |
| [internal/mediaengine/saving/usecase/saving_test.go](internal/mediaengine/saving/usecase/saving_test.go) | Menguji lifecycle rekaman, validasi, recovery partial, disk guard, kegagalan spawn, retry upload setelah restart, dan cleanup kegagalan startup. |
| [internal/mediaengine/saving/repository/recording.go](internal/mediaengine/saving/repository/recording.go) | Repository `Files` membaca dan menyimpan manifest rekaman JSON dengan penulisan sementara, sinkronisasi, dan rename atomik. |
| [internal/mediaengine/saving/repository/recording_test.go](internal/mediaengine/saving/repository/recording_test.go) | Memastikan schema manifest lama tetap dapat dibaca dan ditulis kembali. |

### `mediaengine/saving/delivery/` — API sesi rekaman

| File | Fungsi |
| --- | --- |
| [internal/mediaengine/saving/delivery/handler.go](internal/mediaengine/saving/delivery/handler.go) | Parsing request mulai/stop/daftar rekaman, memanggil UseCase, dan membentuk response yang aman untuk API. |
| [internal/mediaengine/saving/delivery/router.go](internal/mediaengine/saving/delivery/router.go) | Mendaftarkan `/media/recording-sessions`, alias lama `/rounds`, endpoint stop, dan `/recordings`. Semua berada di bawah `/api/v1`. |

### `mediaengine/saving/worker/` — jadwal dan pengawasan proses

| File | Fungsi |
| --- | --- |
| [internal/mediaengine/saving/worker/worker.go](internal/mediaengine/saving/worker/worker.go) | Lifecycle worker recording/upload/guard dengan jobgroup dan context bersama. |
| [internal/mediaengine/saving/worker/recording.go](internal/mediaengine/saving/worker/recording.go) | Menunggu proses recorder selesai, menjalankan callback finalisasi, menghentikan proses, dan menunggu seluruh completion. |
| [internal/mediaengine/saving/worker/upload.go](internal/mediaengine/saving/worker/upload.go) | `StartUploadLoop()` menjadwalkan callback upload; `UploadWithTimeout()` memberi deadline satu upload. Pemilihan file dan retry state tetap di UseCase. |
| [internal/mediaengine/saving/worker/guard.go](internal/mediaengine/saving/worker/guard.go) | Timer berkala untuk memanggil pemeriksaan batas disk/durasi dari UseCase. |
| [internal/mediaengine/saving/worker/worker_test.go](internal/mediaengine/saving/worker/worker_test.go) | Menguji urutan siklus upload, penundaan guard, penantian finalisasi, timeout/cancel upload, dan shutdown proses recorder. |

Pemisahannya: **UseCase memilih apa yang harus diunggah; worker menjadwalkan dan membatasi waktunya; adapter S3 mengirim byte file.**

## 14. `internal/mediaengine/media/` — kontrak dan implementasi media

Folder ini berisi teknologi media yang dipakai oleh beberapa domain.

| File | Fungsi |
| --- | --- |
| [internal/mediaengine/media/process.go](internal/mediaengine/media/process.go) | Interface `Process`: operasi untuk mengamati selesainya proses, membaca error, dan menghentikannya tanpa dependency implementasi OS. |
| [internal/mediaengine/media/status.go](internal/mediaengine/media/status.go) | Kontrak data status proses media yang dapat ditampilkan arena. Status proses berjalan belum menjamin setiap kamera sehat. |

### `mediaengine/media/mediamtx/` — server media lokal

| File | Fungsi |
| --- | --- |
| [internal/mediaengine/media/mediamtx/server.go](internal/mediaengine/media/mediamtx/server.go) | `Server` menerima snapshot sumber melalui `Options`, merender konfigurasi upstream/RTSP/WebRTC/auth, membentuk URL WHEP, menjalankan/menutup MediaMTX, serta melaporkan status proses. |
| [internal/mediaengine/media/mediamtx/server_test.go](internal/mediaengine/media/mediamtx/server_test.go) | Menguji mapping sumber/konfigurasi, CORS, pembatasan akses, snapshot options, gagal start/retry, dan shutdown proses. |
| [internal/mediaengine/media/mediamtx/doc.go](internal/mediaengine/media/mediamtx/doc.go) | Ringkasan tanggung jawab adapter server media. |

### `mediaengine/media/ffmpeg/` — tiga adapter FFmpeg

| File | Fungsi |
| --- | --- |
| [internal/mediaengine/media/ffmpeg/input.go](internal/mediaengine/media/ffmpeg/input.go) | Menyusun argumen input RTSP bersama: TCP, timeout, serta pembatasan output log. |
| [internal/mediaengine/media/ffmpeg/audio.go](internal/mediaengine/media/ffmpeg/audio.go) | `AudioDecoder` menjalankan FFmpeg untuk menghasilkan PCM dan meneruskannya ke callback capture. |
| [internal/mediaengine/media/ffmpeg/recording.go](internal/mediaengine/media/ffmpeg/recording.go) | `Recorder` merekam video stream-copy dan audio AAC opsional ke fragmented MP4. Mengembalikan kontrak `media.Process`. |
| [internal/mediaengine/media/ffmpeg/streaming.go](internal/mediaengine/media/ffmpeg/streaming.go) | `Restreamer` melakukan encode H.264/AAC dan mengirim ke RTSP atau keluaran FLV untuk target RTMP/RTMPS. |
| [internal/mediaengine/media/ffmpeg/doc.go](internal/mediaengine/media/ffmpeg/doc.go) | Ringkasan tanggung jawab adapter FFmpeg. |

Adapter membentuk perintah teknis. Keputusan mediaengine/channel/target/sesi yang boleh digunakan berasal dari UseCase pemanggil.

## 15. `internal/infra/storage/` — filesystem dan object storage

### `infra/storage/filesystem/`

| File | Fungsi |
| --- | --- |
| [internal/infra/storage/filesystem/disk.go](internal/infra/storage/filesystem/disk.go) | Adapter `Disk`: menyiapkan direktori, membaca ukuran file, rename partial menjadi final, dan membaca kapasitas bebas. |
| [internal/infra/storage/filesystem/free_windows.go](internal/infra/storage/filesystem/free_windows.go) | Implementasi Windows untuk kapasitas disk dan exclusive file lock. |
| [internal/infra/storage/filesystem/free_linux.go](internal/infra/storage/filesystem/free_linux.go) | Implementasi Linux untuk kapasitas disk dan exclusive file lock. |
| [internal/infra/storage/filesystem/lock_test.go](internal/infra/storage/filesystem/lock_test.go) | Memastikan lock mencegah dua instance memakai lokasi yang sama. |
| [internal/infra/storage/filesystem/doc.go](internal/infra/storage/filesystem/doc.go) | Ringkasan operasi filesystem, kapasitas, dan lock. |

### `infra/storage/s3/`

| File | Fungsi |
| --- | --- |
| [internal/infra/storage/s3/s3.go](internal/infra/storage/s3/s3.go) | Konfigurasi `Options` dan adapter `Uploader` untuk upload file ke bucket S3-compatible menggunakan object key dari UseCase. |
| [internal/infra/storage/s3/lazy.go](internal/infra/storage/s3/lazy.go) | `LazyUploader` menunda inisialisasi SDK/credential provider sampai upload pertama, sehingga cloud tidak menghambat startup arena. Inisialisasi yang gagal dapat dicoba kembali. |
| [internal/infra/storage/s3/s3_test.go](internal/infra/storage/s3/s3_test.go) | Menguji upload memakai object key stabil. |
| [internal/infra/storage/s3/lazy_test.go](internal/infra/storage/s3/lazy_test.go) | Menguji tidak ada inisialisasi sebelum upload dan kegagalan inisialisasi dapat diulang. |
| [internal/infra/storage/s3/doc.go](internal/infra/storage/s3/doc.go) | Ringkasan tanggung jawab adapter object storage. |

## 16. `internal/infra/runtime/` — proses OS dan pekerjaan Go

`runtime` di dalam `internal` adalah **source code**, sedangkan folder `runtime/` di akar aplikasi berisi file hasil generate dan pengujian.

### `infra/runtime/subprocess/` — child process OS

| File | Fungsi |
| --- | --- |
| [internal/infra/runtime/subprocess/process.go](internal/infra/runtime/subprocess/process.go) | Membungkus `os/exec` menjadi `media.Process`, mengamati exit, menangani context, mengirim `q` untuk flush FFmpeg, dan kill setelah timeout. Memisahkan cara start proses media server. |
| [internal/infra/runtime/subprocess/mediaserver_windows.go](internal/infra/runtime/subprocess/mediaserver_windows.go) | Cara menghentikan proses MediaMTX pada Windows. |
| [internal/infra/runtime/subprocess/mediaserver_linux.go](internal/infra/runtime/subprocess/mediaserver_linux.go) | Cara menghentikan proses MediaMTX pada Linux melalui sinyal. |
| [internal/infra/runtime/subprocess/process_test.go](internal/infra/runtime/subprocess/process_test.go) | Menguji penghentian FFmpeg menghasilkan MP4 yang dapat diperiksa, menggunakan executable media bila tersedia. |
| [internal/infra/runtime/subprocess/doc.go](internal/infra/runtime/subprocess/doc.go) | Ringkasan tanggung jawab eksekusi child process. |

### `infra/runtime/jobgroup/` — goroutine background

| File | Fungsi |
| --- | --- |
| [internal/infra/runtime/jobgroup/group.go](internal/infra/runtime/jobgroup/group.go) | Context/cancellation, penerimaan tugas, WaitGroup, Start/Close, timer yang dapat dibatalkan, dan penantian proses. Menjadi fondasi worker domain. |
| [internal/infra/runtime/jobgroup/group_test.go](internal/infra/runtime/jobgroup/group_test.go) | Menguji aturan penerimaan pekerjaan dan penutupan group. |
| [internal/infra/runtime/jobgroup/doc.go](internal/infra/runtime/jobgroup/doc.go) | Ringkasan lifecycle pekerjaan background. |

## 17. `internal/edgetocloud/synchronization/` — komunikasi cloud

### `edgetocloud/synchronization/usecase/` — aturan pertukaran

| File | Fungsi |
| --- | --- |
| [internal/edgetocloud/synchronization/usecase/sync.go](internal/edgetocloud/synchronization/usecase/sync.go) | Membaca maksimal 100 perubahan pending, mengirim batch, memvalidasi acknowledgment terhadap batch, lalu menandai yang diterima. Yang gagal/tidak diakui tetap pending. |
| [internal/edgetocloud/synchronization/usecase/sync_test.go](internal/edgetocloud/synchronization/usecase/sync_test.go) | Menguji outbox bertahan saat offline/restart, acknowledgment parsial, dan penolakan acknowledgment tidak valid. |
| [internal/edgetocloud/synchronization/usecase/commands.go](internal/edgetocloud/synchronization/usecase/commands.go) | Memeriksa identitas/expiry command, deduplikasi, menyimpan command/hasil, memanggil runner operasi media, dan mengirim hasil hingga diakui cloud. |
| [internal/edgetocloud/synchronization/usecase/commands_test.go](internal/edgetocloud/synchronization/usecase/commands_test.go) | Menguji command kedaluwarsa, laporan hasil gagal, dan deduplikasi sesudah restart. |

### `edgetocloud/synchronization/client/` — transport HTTP

| File | Fungsi |
| --- | --- |
| [internal/edgetocloud/synchronization/client/http.go](internal/edgetocloud/synchronization/client/http.go) | HTTP client pengiriman batch perubahan dengan Bearer token, timeout, batas response, dan aturan tidak mengikuti redirect. |
| [internal/edgetocloud/synchronization/client/commands.go](internal/edgetocloud/synchronization/client/commands.go) | Request polling command dan pengiriman hasil melalui endpoint command yang dikonfigurasi. |
| [internal/edgetocloud/synchronization/client/packages.go](internal/edgetocloud/synchronization/client/packages.go) | Mengunduh paket event privat untuk aksi persiapan eksplisit; membatasi ukuran dan memeriksa response HTTP. |

Repository penyimpanan outbox/command adalah implementasi SQLite pada `core/arena/repository`. Dependency tersebut dipasang di `app`; UseCase sinkronisasi hanya mengetahui interface.

## 18. `internal/edgetocloud/exchange/` dan `internal/edgetocloud/operation/`

Keduanya punya peran berbeda: `edgetocloud/exchange/model` mendefinisikan pesan, sedangkan `edgetocloud/operation/usecase` menjalankan koordinasi yang diminta pesan tersebut.

| File | Fungsi |
| --- | --- |
| [internal/edgetocloud/exchange/model/change.go](internal/edgetocloud/exchange/model/change.go) | Kontrak `Change`: ID perubahan, identitas core/arena/engine/event, jenis, waktu, dan payload. Tidak bergantung pada HTTP atau database. |
| [internal/edgetocloud/exchange/model/command.go](internal/edgetocloud/exchange/model/command.go) | Kontrak command media dari cloud dan hasil eksekusinya, termasuk identitas tujuan dan masa berlaku. |
| [internal/edgetocloud/operation/usecase/media.go](internal/edgetocloud/operation/usecase/media.go) | Menerjemahkan command menjadi panggilan interface UseCase streaming dan saving, memvalidasi payload, dan merekonsiliasi operasi yang mungkin sudah dilakukan. Tidak mengubah state penilaian. |

## 19. `internal/infra/network/dns/` — resolver lokal

Folder induk `network` menampung adapter jaringan; implementasi yang ada saat ini adalah DNS arena.

| File | Fungsi |
| --- | --- |
| [internal/infra/network/dns/server.go](internal/infra/network/dns/server.go) | Melayani DNS UDP/TCP untuk nama arena dari konfigurasi lokal, membatasi sumber klien menurut CIDR, dan meneruskan nama lain ke upstream dengan batas concurrency/timeout. Nama lokal tetap dapat dijawab tanpa internet. |
| [internal/infra/network/dns/server_test.go](internal/infra/network/dns/server_test.go) | Menguji resolusi nama arena melalui UDP/TCP ketika upstream internet tidak tersedia. |

## 20. `internal/delivery/http/` — pintu masuk bersama

Folder ini memasang transport global. Handler fitur tetap berada pada `delivery` domainnya.

| File | Fungsi |
| --- | --- |
| [internal/delivery/http/router.go](internal/delivery/http/router.go) | Mendefinisikan struct `Handlers`, memasang recovery, trusted proxy, batas body/no-store API, health, redirect UI, auth/origin, dan route setiap domain. Menerima handler, bukan container aplikasi. |
| [internal/delivery/http/handler.go](internal/delivery/http/handler.go) | Handler global `/healthz` dan preflight HTTP. Health menandakan HTTP hidup, bukan seluruh kamera/cloud siap. |
| [internal/delivery/http/router_test.go](internal/delivery/http/router_test.go) | Menguji kontrak HTTP/WebSocket dan memastikan menghentikan restream tidak menghentikan domain lain. |
| [internal/delivery/http/architecture_test.go](internal/delivery/http/architecture_test.go) | Memeriksa arah dependency, larangan `pkg`, import adapter konkret/HTTP dari UseCase, hubungan repository, dan koordinasi lintas domain. |

### `delivery/http/middleware/`

| File | Fungsi |
| --- | --- |
| [internal/delivery/http/middleware/auth.go](internal/delivery/http/middleware/auth.go) | Memeriksa token operator untuk API yang dilindungi, origin, serta kebutuhan autentikasi endpoint/WebSocket terkait. |
| [internal/delivery/http/middleware/origin.go](internal/delivery/http/middleware/origin.go) | Pemeriksaan origin untuk route cookie/API, izin origin konfigurasi atau same-host, serta respons preflight. |
| [internal/delivery/http/middleware/auth_test.go](internal/delivery/http/middleware/auth_test.go) | Menguji perilaku auth dan pembatasan origin. |
| [internal/delivery/http/middleware/doc.go](internal/delivery/http/middleware/doc.go) | Ringkasan tanggung jawab middleware auth/origin. |

### `delivery/http/response/`

| File | Fungsi |
| --- | --- |
| [internal/delivery/http/response/response.go](internal/delivery/http/response/response.go) | `WriteError()` menerjemahkan jenis error aplikasi menjadi status HTTP dan pesan aman untuk klien. |
| [internal/delivery/http/response/doc.go](internal/delivery/http/response/doc.go) | Ringkasan pemetaan error ke response HTTP. |

## 21. `internal/apperror/` dan `internal/core/identifier/`

| File | Fungsi |
| --- | --- |
| [internal/apperror/errors.go](internal/apperror/errors.go) | Jenis error bersama seperti invalid/conflict/unavailable/unauthorized/forbidden, serta pembungkus pesan publik dengan penyebab tetap dapat diperiksa. |
| [internal/apperror/doc.go](internal/apperror/doc.go) | Ringkasan error bersama yang tidak bergantung pada transport. |
| [internal/core/identifier/id.go](internal/core/identifier/id.go) | Memeriksa format ID menggunakan pola yang disepakati. Validasi keberadaan/otorisasi ID tetap dilakukan UseCase. |
| [internal/core/identifier/doc.go](internal/core/identifier/doc.go) | Ringkasan pemeriksaan sintaks identifier. |

## 22. `scripts/` — persiapan dan pengujian operasional

| File | Fungsi |
| --- | --- |
| [scripts/build-local.ps1](scripts/build-local.ps1) | Membangun executable API, renderer LAN, dan generator paket demo ke `bin/`. Dilakukan sebelum event sehingga startup offline tidak perlu kompilasi/download. |
| [scripts/build-proxy.ps1](scripts/build-proxy.ps1) | Membangun Caddy dengan versi/plugin Hostinger yang ditentukan script; butuh persiapan online. |
| [scripts/configure-lan.ps1](scripts/configure-lan.ps1) | Wrapper executable `lan-config` untuk menyiapkan `.env.lan` dan hasil render. |
| [scripts/start-lan.ps1](scripts/start-lan.ps1) | Merender konfigurasi, menjalankan binary aplikasi dan proxy tanpa jendela tambahan, mengawasi proses, lalu menghentikan proses miliknya saat selesai. |
| [scripts/start-webcam-test.ps1](scripts/start-webcam-test.ps1) | Listing perangkat DirectShow, inisialisasi env uji, dan publikasi webcam/mic atau media sintetis ke server RTSP sumber uji. Tidak menjalankan API Go secara otomatis. |
| [scripts/tests/offline-browser.cjs](scripts/tests/offline-browser.cjs) | Tes integrasi browser Edge melalui Playwright: akun/perangkat baru, cloud/media tidak tersedia, nilai/revisi, antrean perangkat saat request gagal, restart, backup, dan sinkronisasi ulang. |
| [scripts/tests/cloud-exchange.cjs](scripts/tests/cloud-exchange.cjs) | Tes dua binary/database nyata: unggah/pull paket, submit/sync, command media, cloud mati/hidup kembali, dan penolakan penggantian paket event aktif. |

Subfolder `scripts/tests` berisi runner tes integrasi lintas komponen, sedangkan `*_test.go` menguji package/perilaku Go.

## 23. File privat, data, binary, dan hasil generate

Lokasi dapat berubah lewat env; contoh berikut adalah default atau lokasi yang digunakan script pengembangan. Nama file di direktori tes dapat berubah setiap run, sehingga dependency unduhan dan setiap artefak sementara tidak didaftarkan satu per satu.

| Folder / pola | Isi dan fungsi |
| --- | --- |
| `bin/` | Executable hasil build seperti `local-server.exe`, `lan-config.exe`, `event-package.exe`, `caddy.exe`, dan MediaMTX unduhan. File program siap dijalankan, bukan source utama. |
| `data/` atau `DATA_DIR` | Penyimpanan persisten aplikasi: database penilaian, manifest, rekaman, lock instance, dan cache sertifikat pada deployment LAN. |
| `DATA_DIR/arena.sqlite` | Event, revisi, sesi juri, audit dalam metadata, outbox, serta command/hasil lokal. |
| `arena.sqlite-wal`, `arena.sqlite-shm` | File kerja SQLite WAL. Kelola melalui SQLite dan prosedur backup/restore; bukan file temporary yang bebas dihapus saat database aktif. |
| `DATA_DIR/queue/` | Manifest JSON rekaman dan informasi retry/upload. Ini antrean media, berbeda dari outbox nilai di SQLite. |
| `DATA_DIR/recordings/` | File rekaman per sesi/channel, termasuk partial sebelum finalisasi. |
| `DATA_DIR/caddy/` | Cache/persistence sertifikat proxy agar HTTPS lokal tidak memerlukan penerbitan sertifikat baru pada setiap startup. |
| `DATA_DIR/edge.lock` | Lock penggunaan direktori data oleh instance aplikasi. |
| `runtime/` atau `RUNTIME_DIR` | Konfigurasi media hasil generate, lock runtime, serta artefak pengujian/persiapan. Dapat mengandung URL berkredensial sehingga aksesnya perlu privat. |
| `RUNTIME_DIR/mediamtx.yml` | Konfigurasi MediaMTX utama hasil renderer. Perubahan permanen dilakukan lewat env/builder, lalu render kembali. |
| `RUNTIME_DIR/mediamtx.lock` | Lock penggunaan runtime media antar instance. |
| `runtime/lan-app/app.env` | Env aplikasi yang dirender dari pengaturan LAN; token Hostinger tidak disalin ke file ini. |
| `runtime/lan-app/Caddyfile` | Konfigurasi proxy HTTPS dan DNS-01 hasil renderer. |
| `runtime/webcam-app/` | Runtime MediaMTX utama untuk env uji webcam. |
| `runtime/demo/` atau output `event-package` | Paket demo dan CSV password juri. Lokasi mengikuti argumen `-out`. |
| `runtime/browser-check/` | Binary uji, fixture, direktori run, database/env uji, dan dependency Playwright sementara. |
| `runtime/proxy-build/` | Workspace/build cache alat pembuat Caddy. |
| `runtime/refactor-arena/` | Salinan source dan fingerprint sementara yang digunakan untuk memverifikasi pemecahan UseCase tanpa perubahan deklarasi. Tidak dibutuhkan aplikasi saat berjalan. |
| `BACKUP_DIR` | Snapshot SQLite konsisten untuk pemulihan; konfigurasi harus di luar `DATA_DIR`, idealnya pada penyimpanan fisik terpisah. |
| `coverage.out` | Hasil coverage jika pengujian dijalankan dengan output coverage. |

Rekaman, database nilai, dan manifest adalah data operasional. Perlakukan pemeliharaan/restore ketiganya sesuai panduan operasional, terpisah dari pembersihan hasil build.

## 24. Lampiran `cloud-app/` — pasangan komunikasi arena

`cloud-app` merupakan module Go terpisah untuk pertukaran paket, journal perubahan, dan command media. Website pendaftaran, CRUD peserta, serta proyeksi hasil publik belum dibuat pada module ini.

| Folder | Fungsi |
| --- | --- |
| `cloud-app/cmd/api/` | Entry point server cloud. |
| `cloud-app/internal/exchange/` | Fitur pertukaran data; dipisah menjadi model, delivery, UseCase, dan repository. |
| `cloud-app/data/`, `cloud-app/bin/` | Database persisten dan executable hasil build sesuai konfigurasi; bukan kode bisnis. |

| File | Fungsi |
| --- | --- |
| [cloud-app/go.mod](../cloud-app/go.mod) | Identitas module `kicau.cloud/control`, versi Go, dan dependency cloud. |
| [cloud-app/go.sum](../cloud-app/go.sum) | Checksum dependency module cloud. |
| [cloud-app/.gitignore](../cloud-app/.gitignore) | Mengabaikan env privat, data, binary, dan executable cloud. |
| [cloud-app/.env.example](../cloud-app/.env.example) | Contoh bind HTTP, lokasi database, token admin, serta registry core/arena/engine dan token mesin. File `.env` adalah salinan privat untuk deployment. |
| [cloud-app/README.md](../cloud-app/README.md) | Cara menjalankan cloud exchange dan kontrak endpoint. |
| [cloud-app/cmd/api/main.go](../cloud-app/cmd/api/main.go) | Memuat env, membuka repository, merakit UseCase/router, dan menjalankan lifecycle HTTP cloud. |
| [cloud-app/internal/exchange/model/model.go](../cloud-app/internal/exchange/model/model.go) | Kontrak paket/engine/perubahan/command/hasil/status serta jenis error cloud. |
| [cloud-app/internal/exchange/delivery/http.go](../cloud-app/internal/exchange/delivery/http.go) | Route dan handler HTTP: ekstrak token, baca payload/path, panggil UseCase, dan kirim response. Memasang batas body dan no-store. |
| [cloud-app/internal/exchange/usecase/exchange.go](../cloud-app/internal/exchange/usecase/exchange.go) | Autentikasi engine/admin, aturan identitas, penerimaan perubahan, penyediaan paket, allowlist/expiry command, serta hasil dan acknowledgment. |
| [cloud-app/internal/exchange/usecase/exchange_test.go](../cloud-app/internal/exchange/usecase/exchange_test.go) | Menguji acknowledgment persisten, retry/deduplikasi, isolasi antar engine, rollback konflik batch, dan command/hasil. |
| [cloud-app/internal/exchange/repository/sqlite.go](../cloud-app/internal/exchange/repository/sqlite.go) | Persistence paket versi, journal perubahan, antrean command, dan hasil memakai SQLite WAL/FULL dengan transaksi serta pemeriksaan duplikasi. |

## 25. Memilih file saat ingin mengubah sesuatu

| Perubahan yang diinginkan | Mulai dari |
| --- | --- |
| Mengubah aturan angka/kriteria/revisi | `core/arena/usecase/scoring.go`, lalu tes arena |
| Mengubah syarat penutupan ronde | `core/arena/usecase/round.go` |
| Mengubah rumus rekap | `core/arena/model/event.go`, lalu tampilan query dan tes terkait |
| Mengubah aturan impor paket | `core/arena/usecase/event.go` |
| Mengubah autentikasi juri lokal | `core/arena/usecase/auth.go`; bentuk cookie berada di handler arena |
| Mengubah UI input/tampilan | `core/arena/delivery/web/`; aturan server tetap pada UseCase |
| Mengubah endpoint atau DTO | `delivery` domain terkait; middleware global di `delivery/http` |
| Mengubah codec/argumen FFmpeg | Adapter terkait di `mediaengine/media/ffmpeg/` |
| Mengubah format atau lifecycle rekaman | `mediaengine/saving/model`, `mediaengine/saving/usecase`, dan `mediaengine/saving/repository` sesuai kebutuhan kompatibilitas |
| Mengubah jadwal retry/cancellation media | Worker domain terkait |
| Mengubah pengiriman skor ke cloud | `edgetocloud/synchronization/usecase/sync.go` dan adapter client |
| Menambah command media | Validasi cloud, UseCase command lokal, dan `edgetocloud/operation/usecase/media.go` |
| Mengubah domain/IP/provider deployment | Env LAN dan `cmd/lan-config/main.go` bila kemampuan builder perlu berubah |
| Memasang implementasi service baru | Kontrak UseCase terkait dan wiring `internal/app/container.go` |

Saat menambah atau memindahkan file source, perbarui bagian indeks terkait agar panduan ini tetap sesuai struktur proyek.
