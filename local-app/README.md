# Local arena server

Server arena untuk penilaian offline-first dan engine media: hingga 24 feed RTSP, video WebRTC lokal, audio WebSocket, rekaman ke SSD, upload S3 asinkron, dan restream sesuai permintaan. Penilaian disimpan di arena tanpa menunggu cloud atau media.

Panduan operasional baru ada di [OFFLINE-FIRST.md](OFFLINE-FIRST.md): persiapan event, akun juri lokal, deployment LAN/HTTPS, sinkronisasi cloud, backup, dan pengujian gangguan. Backend penerima sinkronisasi berada di [cloud-app](../cloud-app/README.md).

Alur logic dari persiapan event sampai penilaian, media, sinkronisasi, dan recovery dijelaskan di [ALUR-END-TO-END.md](ALUR-END-TO-END.md), termasuk diagram sequence dan state ronde.

## Membaca kode penilaian

Panduan lengkap fungsi setiap folder dan file, termasuk tes, script, konfigurasi, dan lampiran cloud, tersedia di [PETA-KODE.md](PETA-KODE.md).

Mulai dengan satu request submit nilai: [model](internal/core/arena/model/event.go) → [handler `submit()`](internal/core/arena/delivery/handler.go) → [UseCase `Submit()`](internal/core/arena/usecase/scoring.go) → [repository `UpdateScores()`](internal/core/arena/repository/sqlite.go). Model menjelaskan data, handler membaca request, UseCase menerapkan aturan bisnis, dan repository melakukan transaksi penyimpanan.

Semua file berikut berada di `internal/core/arena/usecase`, dalam satu package dengan satu instance `UseCase` dan constructor `New()`:

| File | Baca ketika ingin memahami |
| --- | --- |
| [arena.go](internal/core/arena/usecase/arena.go) | Kontrak dependency, state bersama, constructor, dan fungsi internal bersama |
| [event.go](internal/core/arena/usecase/event.go) | Pengambilan, impor, dan validasi paket event |
| [auth.go](internal/core/arena/usecase/auth.go) | Login, logout, sesi juri, dan pembatasan percobaan login |
| [scoring.go](internal/core/arena/usecase/scoring.go) | Submit nilai, revisi, konflik, serta deduplikasi retry |
| [round.go](internal/core/arena/usecase/round.go) | Mulai, lanjutkan, tutup, batalkan, dan recovery ronde |
| [query.go](internal/core/arena/usecase/query.go) | Katalog, tampilan juri/operator, rekap, history, dan status operasional |
| [backup.go](internal/core/arena/usecase/backup.go) | Pembuatan snapshot database dan pencatatan hasil backup |

Setelah alur submit jelas, lanjutkan ke lifecycle ronde dan sinkronisasi. Contoh perilaku penilaian ada di [arena_test.go](internal/core/arena/usecase/arena_test.go), terutama `TestOfflineScoringRevisionsCompletenessAndRoundLock`.

## Menjalankan

### Logging

Pola logging mengikuti `park-server-v3`: Zap dengan field `time`, `level`, `msg`, `caller`, `service`, dan `hostname`, serta stacktrace untuk level error. `APP_ENV=development`, `local`, atau kosong memakai console berwarna; nilai lain seperti `production` memakai JSON stdout. File `${LOG_DIR}/app.log` selalu JSON untuk level info ke atas; default `LOG_DIR=./logs` dan `SERVICE_NAME=local-app`. Env dimuat sebelum logger diinisialisasi. Jika file log tidak dapat dibuka, aplikasi tetap memakai stdout.

Middleware membuat atau meneruskan `X-Request-ID` yang valid, memasang `requestId` pada context/response, lalu mencatat method, route template, status, latency, dan IP. Gunakan `logger.FromContext(ctx)` untuk log yang mengikuti request; helper `logger.Info/Warn/Error/Debug` dipakai untuk lifecycle/background. Field sensitif dan URL pada pesan/field disensor. Body, query, cookie, token, dan argumen proses media tidak dicatat oleh access logger. Request ID ini merupakan korelasi log, belum distributed tracing OpenTelemetry.

File log ditulis append dan belum memiliki rotasi otomatis; kelola retensi melalui deployment. Lokasi source logger adalah `internal/infra/logger`, sedangkan file log operasional ada di `logs/` atau lokasi `LOG_DIR` yang dipilih.

### Prasyarat dan startup

Prasyarat: Go 1.25+, FFmpeg dengan encoder `libx264`/`aac`, dan **MediaMTX v1.15.3** di PATH. Schema konfigurasi diikat ke versi MediaMTX tersebut; versi lain harus diverifikasi sebelum dipakai. Windows dan Linux didukung.

```powershell
cd D:\Workdir\Go\kicau-mania\local-app
Copy-Item .env.example .env
# Edit .env: token acak minimal 32 karakter, URL RTSP asli, IP LAN, dan origin UI.
go mod download
go run ./cmd/api
```

Contoh membuat token di PowerShell:

```powershell
$bytes = New-Object byte[] 32
$rng = [System.Security.Cryptography.RandomNumberGenerator]::Create()
$rng.GetBytes($bytes)
[Convert]::ToBase64String($bytes)
$rng.Dispose()
```

`API_TOKEN` contoh sengaja ditolak. Konfigurasi dimuat dari satu `.env`; environment proses memiliki prioritas. Jangan commit `.env`. `DATA_DIR` menyimpan rekaman dan manifest antrean pada SSD persisten. `RUNTIME_DIR` (default `./runtime`) menyimpan `mediamtx.yml` hasil generate dan lock proses; direktori ini dapat dibuat ulang, tetapi harus privat karena konfigurasi media memuat kredensial sumber. Gunakan direktori data dan runtime tersendiri untuk setiap instance; file lock mencegah penggunaan bersamaan. Jalankan dari working directory yang sama atau gunakan path absolut. Lokasi konfigurasi lama `data/mediamtx.yml` tidak digunakan lagi; render/start berikutnya menulis ke `runtime/mediamtx.yml` secara default. Jangan pindahkan atau hapus rekaman/manifest ketika mengubah lokasi runtime.

```powershell
go run ./cmd/api -env .env -render-config
go build -o bin/local-server.exe ./cmd/api
go test ./...
go vet ./...
```

API menjalankan MediaMTX sebagai child process dan mencoba menjalankannya kembali ketika berhenti. Kegagalan media atau upload tidak menghentikan HTTP penilaian. Gunakan Windows Service/systemd untuk startup aplikasi otomatis. `/healthz` hanya menandakan proses HTTP hidup; `/api/v1/arena/status` memisahkan status proses media, backlog cloud, dan backup. Status proses media belum menjamin seluruh kamera sehat.

## Testing dengan webcam dan mic Windows

Gunakan PowerShell 7 (`pwsh`), FFmpeg dengan `dshow`, `libx264`, dan `libopus`, serta MediaMTX v1.15.3. Script memakai binary MediaMTX di `bin/mediamtx-v1.15.3/mediamtx.exe` jika ada, atau `mediamtx` dari PATH. Opsi `-FFmpegBin` dan `-MediaMTXBin` menerima lokasi binary lain.

Jalankan dari `local-app`:

```powershell
# Hanya menampilkan nama perangkat, tanpa memulai capture.
pwsh -NoProfile -File ./scripts/start-webcam-test.ps1 -ListDevices

# Membuat .env.webcam dengan token acak; tidak menimpa file yang sudah ada.
pwsh -NoProfile -File ./scripts/start-webcam-test.ps1 -InitEnv

# Terminal pertama: gunakan nama perangkat persis dari hasil -ListDevices.
pwsh -NoProfile -File ./scripts/start-webcam-test.ps1 -VideoDevice 'Integrated Camera' -AudioDevice 'Microphone (Realtek Audio)'

# Terminal kedua: jalankan aplikasi menggunakan konfigurasi pengujian.
go run ./cmd/api -env .env.webcam
```

Periksa/edit `.env.webcam` jika diperlukan. Data uji berada di `data/webcam`, konfigurasi aplikasi yang digenerasi di `runtime/webcam-app/mediamtx.yml`, dan token API hanya tersimpan di file env. Pengiriman metrik ke backend, S3, dan restream dinonaktifkan pada contoh pengujian. Environment proses tetap mengungguli file env; pastikan variabel konfigurasi produksi tidak terpasang pada terminal pengujian. Jika `.env.webcam` sudah ada, opsi binary pada script tidak mengubah isinya.

```text
webcam + mic → FFmpeg → MediaMTX sumber uji (127.0.0.1:8555/webcam)
                     → MediaMTX aplikasi (127.0.0.1:8554/webcam01)
                     → WebRTC / audio WebSocket / metrik / rekaman
```

`config/mediamtx-webcam.yml` adalah konfigurasi sumber uji yang dapat diedit; berbeda dengan file konfigurasi aplikasi yang digenerasi. Sumber uji hanya menerima publish/read loopback untuk path `webcam`, tanpa WebRTC atau protokol tambahan. Aplikasi tetap memakai port 8090, 8554, 8889, dan 8189; hentikan instance aplikasi lain pada port tersebut sebelum menguji. Jangan mengarahkan channel ke port 8554 milik aplikasi itu sendiri.

FFmpeg menerbitkan H264 dan Opus; aplikasi mendekode audio ke PCM 16 kHz seperti sumber NVR biasa. Gunakan endpoint yang sama dengan channel `webcam01`: `GET /api/v1/channels`, WebSocket `/api/v1/channels/webcam01/audio`, `/api/v1/metrics`, dan endpoint ronde/rekaman. Untuk video saja, URL WHEP adalah `http://127.0.0.1:8889/webcam01/whep`; ini endpoint signaling untuk player WebRTC, bukan file video biasa. Endpoint API/WebSocket memerlukan token dari `.env.webcam`.

Tanpa perangkat fisik, gunakan video pola dan nada sintetis:

```powershell
pwsh -NoProfile -File ./scripts/start-webcam-test.ps1 -Synthetic
# Pengujian berbatas waktu, tanpa webcam/mic:
pwsh -NoProfile -File ./scripts/start-webcam-test.ps1 -Synthetic -DurationSeconds 15
```

`Ctrl+C` pada terminal sumber menghentikan FFmpeg dan proses MediaMTX yang dibuat script. Aplikasi Go pada terminal kedua dihentikan tersendiri dengan `Ctrl+C`. Script tidak menghentikan proses lain. Jika perangkat tidak dapat dibuka, periksa nama perangkat, izin kamera/mikrofon Windows, dan aplikasi lain yang sedang memakainya. Jika port 8555 sudah dipakai, script gagal tanpa menghentikan pemilik port tersebut.

Referensi perintah capture: [FFmpeg DirectShow](https://ffmpeg.org/ffmpeg-devices.html#dshow); publikasi stream: [MediaMTX FFmpeg](https://github.com/bluenviron/mediamtx/blob/v1.15.3/docs/2-usage/02-publish.md).

## Konfigurasi media dan jaringan

`.env.example` menyediakan 24 entri `CHANNELS_JSON`. Ganti **setiap** URL RTSP dengan URL yang terbukti bekerja pada model NVR ZiTech yang digunakan. Path RTSP tidak diasumsikan berdasarkan merek. Percent-encode karakter khusus username/password di URL. Kanal yang belum terpasang harus dihapus dari konfigurasi.

Untuk membentuk URL dari username/password mentah di Go, gunakan `url.UserPassword`, bukan `url.QueryEscape`. Contoh: `(&url.URL{Scheme: "rtsp", Host: "192.0.2.1:554", Path: "/feed", User: url.UserPassword("operator", "P@ss#123")}).String()` menghasilkan `rtsp://operator:P%40ss%23123@192.0.2.1:554/feed`. Jangan encode ulang URL yang sudah di-encode.

`Catalog.HubRTSP(id)` mengembalikan URL hub lokal; URL upstream hanya tersedia melalui `Inputs()` untuk konfigurasi infrastruktur. Error parsing katalog mempertahankan penyebab lewat `errors.As`, tetapi pesan error tidak mencetak URL atau fragmen kredensial. Hindari mencatat penyebab mentah dari error URL ke log.

MediaMTX menarik satu upstream per kanal. Worker audio dan perekam membaca dari hub lokal; tidak menarik lagi langsung ke NVR. WebRTC tidak melewati cloud. Set `WEBRTC_HOSTS` ke IP LAN server yang dapat diakses browser; buka TCP 8889 untuk signaling dan UDP 8189 untuk media. API default hanya bind `127.0.0.1:8090`; ganti `HTTP_ADDR` jika UI di komputer lain. `RTSP_ADDR` sebaiknya tetap loopback. Untuk UI HTTPS gunakan reverse proxy TLS pada API dan signaling WebRTC agar tidak terkena mixed content. Reverse proxy harus meneruskan upgrade WebSocket.

`MEDIA_READ_CIDRS` membatasi pembaca media berdasarkan jaringan. `API_TOKEN` hanya melindungi API/WebSocket Go, **bukan** akses WebRTC MediaMTX. Default tidak mempublikasikan RTSP ke LAN dan mematikan RTMP ingest, HLS, SRT, playback, serta Control API MediaMTX. Akses publish RTSP hanya diizinkan dari loopback. Origin UI harus terdaftar eksplisit di `ALLOWED_ORIGINS`.

Seluruh pengaturan codec dilakukan pada NVR/kamera. Gunakan **H264 Main/Baseline tanpa B-frames** agar video kompatibel dengan browser. Server meneruskan video tanpa encoding ulang dan tidak mengubah pengaturan fisik NVR.

```text
NVR H264 → RTSP TCP → MediaMTX → WebRTC → Browser
```

Setiap kanal langsung dipetakan ke path lokal `<id>`, misalnya `rtsp://127.0.0.1:8554/lapak01`. Audio, recording, dan restream membaca alamat yang sama. Tidak ada jalur video alternatif atau proses FFmpeg video di arena. FFmpeg tetap digunakan oleh domain audio, recording, dan restream YouTube sesuai fungsi masing-masing. Passthrough menghindari biaya encoding ulang video, tetapi transport/protokol tetap memakai CPU dan bandwidth.

`ALLOWED_ORIGINS` berlaku penuh pada API Go; MediaMTX **v1.15.3** memakai opsi tunggal `webrtcAllowOrigin`, sehingga origin pertama pada daftar tersebut menjadi origin UI WebRTC. Letakkan origin UI utama di urutan pertama. Gunakan `-render-config` untuk menghasilkan konfigurasi hub dari `.env`; jangan mengedit file MediaMTX hasil generasi secara manual.

## API

Semua `/api/v1` memerlukan `Authorization: Bearer <API_TOKEN>`. Identitas lokal diperoleh middleware dari kredensial operator; tidak menerima tenant/branch dari body. Server ini adalah perangkat arena satu operator, bukan layanan SaaS multi-tenant.

| Method | Path | Fungsi |
| --- | --- | --- |
| GET | `/healthz` | Liveness tanpa kredensial |
| GET | `/api/v1/channels` | ID kanal, URL WHEP, path audio |
| POST | `/api/v1/rounds` | `{"id":"ronde-001","channels":["lapak01","lapak02"]}` |
| POST | `/api/v1/rounds/ronde-001/stop` | Hentikan dan finalisasi semua rekaman ronde |
| GET | `/api/v1/recordings?limit=25&cursor=<next_cursor>` | Status rekaman dengan cursor; limit 1..100 |
| GET | `/api/v1/channels/lapak01/audio` | Upgrade WebSocket; audio biner PCM |
| GET | `/api/v1/metrics` | Upgrade WebSocket; JSON metrik semua lapak |
| POST | `/api/v1/restreams` | `{"channel_id":"lapak01","target":"youtube"}` |
| GET | `/api/v1/restreams` | Status proses restream |
| DELETE | `/api/v1/restreams/youtube` | Matikan restream target tersebut |

Start ronde/restream mengembalikan `202`: permintaan diterima, bukan bukti frame pertama sudah tiba. Periksa status rekaman dan player/YouTube untuk kesiapan media. Hanya satu ronde aktif; ID ronde tidak dapat dipakai ulang. Duplikasi kanal atau ID/path berbahaya ditolak di UseCase. Konflik state menghasilkan `409`, resource tidak dikenal `404`, kapasitas/proses tidak tersedia `503`.

WebSocket browser tidak mendukung header Authorization kustom. Gunakan subprotocol sebagai pembawa token (base64url tanpa padding):

```javascript
const encoded = btoa(token).replaceAll('+', '-').replaceAll('/', '_').replace(/=+$/, '');
const ws = new WebSocket('ws://192.168.10.2:8090/api/v1/channels/lapak01/audio',
  ['edge-v1', 'edge-token.' + encoded]);
ws.binaryType = 'arraybuffer';
```

Gunakan token ASCII acak dan `wss` pada koneksi TLS. Server hanya menegosiasikan `edge-v1`, tidak memantulkan token. Redact header `Authorization` dan `Sec-WebSocket-Protocol` pada log reverse proxy. Origin diperiksa; klien lambat kehilangan frame agar tidak menahan worker lain.

Audio: **PCM signed 16-bit little-endian, mono, 16000 Hz**, 3200 sampel/6400 byte setiap 200 ms. UI perlu AudioWorklet/ring buffer sendiri untuk playback; WebSocket tidak dapat langsung menjadi `src` elemen audio. Ketika reconnect ada gap; format raw PCM tidak menjamin sinkronisasi lip-sync dengan video WebRTC.

Metrik tiap 200 ms berisi `channel_id`, `at` (RFC3339 WIB), `peak_dbfs`, `rms_dbfs`, `active_ms`, `window_ms`. `active_ms` bernilai 200 atau 0 menurut ambang RMS `AUDIO_ACTIVE_DBFS`; diam diberi floor -120 dBFS. Jumlahkan window valid untuk durasi suara aktif; jangan menghitung gap feed sebagai diam. Mikrofon terpisah per lapak diasumsikan sesuai konfirmasi. Ini bukan kalibrasi dB SPL atau klasifikasi kicau: suara tetangga, juri, dan noise tetap perlu mitigasi akustik/kalibrasi.

`BACKEND_METRICS_URL` opsional menerima POST satu objek metrik; 2xx adalah acknowledgment. Pengiriman metrik memakai antrean memori terbatas 128 item dan bersifat best effort. Statistik gagal/berlebih dapat hilang saat offline/restart; rekaman video adalah arsip persisten. Endpoint backend proyek belum memiliki kontrak di repository ini, sehingga URL dan bearer diisi sesuai API penerima.

## Rekaman, SSD, dan antrean upload

Setiap lapak direkam per ronde sebagai fragmented MP4: video stream-copy, audio AAC bila tersedia. Start disimpan ke manifest atomik lebih dulu. Stop mengirim `q` ke FFmpeg, menunggu flush, lalu mengganti `.mp4.partial` menjadi `.mp4`. State: `RECORDING` → `READY` → `UPLOADED`; kegagalan proses menjadi `FAILED`. API stop tidak menyatakan semua kanal sukses; periksa status tiap rekaman.

Worker baru meng-upload file `READY` setelah tidak ada kanal `RECORDING` pada ronde tersebut. Upload S3 kompatibel multipart (2 part paralel, 8 MiB/part) memakai object key unik dan stabil, sehingga retry menimpa object yang sama. Kegagalan dijadwalkan ulang dengan exponential backoff hingga 256 detik; upaya dilanjutkan setelah restart, tanpa batas retry. Satu file di-upload pada satu waktu dengan timeout 30 menit agar bandwidth/CPU terbatas. Bucket kosong menonaktifkan worker; file tetap `READY`. Kredensial dapat berasal dari `.env` atau AWS credential chain.

Setelah restart, manifest `RECORDING` dengan file final yang sudah ada dipulihkan menjadi `READY`; lainnya menjadi `INTERRUPTED`. File partial tidak otomatis di-upload atau dihapus. Gangguan feed lokal menandai rekaman gagal; ronde tidak otomatis disambung karena gap tidak boleh disembunyikan. Untuk pemulihan manual, jalankan ffprobe/FFmpeg pada salinan partial, verifikasi isi, lalu arsipkan melalui alur operator. Retensi/penghapusan file dilakukan operator setelah verifikasi cloud; aplikasi tidak otomatis menghapus bukti rekaman.

Server menolak ronde baru jika ruang tersedia di bawah `MIN_FREE_BYTES`, memeriksa ulang setiap 5 detik, dan menghentikan perekam jika batas tercapai. `MAX_ROUND_DURATION` juga memicu stop. Sisakan margin untuk bitrate puncak, flush, dan delay pemeriksaan: 24 kanal Ã— 4 Mbps memerlukan sekitar **43,2 GB/jam** video, belum mediaengine/audio/overhead. Ketika disk benar-benar penuh, penulisan manifest dapat gagal; periksa log dan lakukan restart setelah ruang dipulihkan. Rename + fsync manifest melindungi restart proses; ketahanan terhadap putus daya tetap memerlukan SSD/filesystem dan UPS yang memadai.

## YouTube dan cloud restream

Isi target di `.env`, misalnya:

```dotenv
RESTREAM_TARGETS_JSON='{"youtube":"rtmps://a.rtmps.youtube.com/live2/STREAM_KEY","cloud":"rtsp://user:pass@cloud.example:8554/arena"}'
```

Gunakan URL ingest/key yang diberikan YouTube Studio. Restream baru berjalan melalui POST; tidak otomatis aktif saat boot. Satu target hanya boleh diisi satu lapak pada satu waktu; pergantian lapak memakai DELETE lalu POST. Target URL tidak diterima dari request. Ini stream lapak terpilih, bukan mosaic 24 kamera. FFmpeg mengubah video ke H264 25 fps/3 Mbps dan audio AAC 128 kbps; sumber tanpa audio ditolak proses. Worker reconnect setiap 5 detik dan berhenti ketika di-DELETE atau server shutdown. `RUNNING` berarti proses hidup, belum acknowledgment YouTube menerima siaran. Setelah restart operator harus mengaktifkan kembali restream agar kuota tidak terpakai tanpa perintah.

## Arsitektur dan verifikasi

Alur: Handler → UseCase → Repository → filesystem. UseCase juga memanggil service Media/Storage/Backend serta kalkulasi Domain. Handler hanya parsing/validasi sintaks, mengambil context autentikasi, memanggil UseCase, memetakan DTO, dan mengirim response. UseCase tidak mengimpor HTTP/Gin; repository tidak memanggil repository lain. DI eksplisit berada di `internal/app/container.go`; shutdown ditangani `internal/server` dengan timezone WIB dan embedded tzdata.

Empat domain memiliki model, UseCase, dan handler sendiri. Audio, streaming, dan saving juga memiliki package worker Go; video diteruskan langsung oleh MediaMTX tanpa encoder lokal. State bisnis berada di UseCase; context pembatalan, goroutine, timer, antrean teknis, dan penantian proses untuk ketiga domain lain berada di worker.

Pada setiap domain, delivery dipisahkan menjadi:

```text
delivery/
├── handler.go   # Parsing request, pemanggilan UseCase, dan response HTTP/WebSocket
└── router.go    # Pemetaan HTTP method dan path ke method handler
```

Pada audio, `delivery/handler.go` memanggil `SubscribeAudio(channelID)` atau `SubscribeMetrics()` pada UseCase. `delivery/websocket.go` mengurus koneksi, ping/pong, deadline, dan pengiriman frame WebSocket. Perhitungan PCM berada di `model/analyzer.go`, sedangkan struktur metrik berada di `model/metric.go`. Service pengiriman metrik memakai interface `MetricSender`.

Masing-masing `router.go` menyediakan `RegisterRoutes(group, handler)` dalam package `delivery` yang sama. Router utama memanggil fungsi tersebut menggunakan struct `Handlers` yang diberikan oleh `app`. Handler `/healthz` dan preflight berada di `internal/delivery/http/handler.go`; middleware global dan pemasangan route tetap berada di router utama.

| Modul | Pemilik proses bisnis |
| --- | --- |
| `internal/mediaengine/video` | Daftar kanal dan URL WebRTC untuk video langsung dari NVR. |
| `internal/mediaengine/audio` | PCM, metrik energi, WebSocket mediaengine/audio/metrik, dan pengiriman statistik backend. |
| `internal/mediaengine/streaming` | Restream YouTube/cloud, allowlist target, status, dan retry. |
| `internal/mediaengine/saving` | Ronde, rekaman, finalisasi, recovery, disk guard, repository manifest, dan upload S3. |

Koordinasi lintas domain dilakukan di UseCase melalui kontrak/model; implementasi repository, worker, dan handler domain lain tidak diimpor. `mediaengine/channel/repository` menyediakan katalog konfigurasi, sedangkan `mediaengine/saving/repository` menyimpan manifest rekaman. Setiap UseCase menerima interface service yang sempit; adapter FFmpeg audio, streaming, dan recording berada di `internal/mediaengine/media/ffmpeg` dengan runner child process bersama di `internal/infra/runtime/subprocess`.


Struktur komponen pendukung berdasarkan fungsi (folder `pkg` telah dihapus):

```text
internal/
  core/
    arena/                     # Event, akun juri, nilai, rekap, dan UI lokal
    identifier/                # Pemeriksaan format identifier
  mediaengine/
    channel/                   # Identitas dan katalog sumber kamera
    video/                     # Daftar channel dan URL live
    audio/                     # PCM, metrik, pubsub, dan WebSocket
    streaming/                 # Restream ke target yang diizinkan
    saving/                    # Rekaman, manifest, dan koordinasi upload
    media/                     # Kontrak Process dan adapter MediaMTX/FFmpeg
  edgetocloud/                 # Sinkronisasi pusat dan store-and-forward
    synchronization/           # Outbox, paket cloud, dan polling command
    exchange/model/            # Kontrak pertukaran arena/cloud
    operation/usecase/         # Koordinasi command recording/restream
  infra/                       # OS, filesystem, storage, dan network
    network/dns/               # Resolver lokal tanpa internet
    storage/                   # Filesystem, kapasitas, lock, dan uploader S3
    logger/                    # Zap console/file, redaksi, dan context request
    runtime/                   # Subprocess Windows/Linux dan jobgroup
  app/                         # Perakitan dependency
  server/                      # Lifecycle HTTP, modul, dan background
  delivery/http/               # Router, middleware, dan response bersama
  apperror/                    # Error aplikasi bersama
```

`app` merakit repository, adapter, worker, UseCase, dan handler. Router hanya menerima `http.Handlers`, tanpa container atau adapter konkret. Container aplikasi memakai nama `ChannelCatalog` dan `MediaServer`. Server serta CLI `-render-config` menggunakan builder di `app`.

UseCase mendeklarasikan interface repository/service yang diperlukannya. Channel dibaca melalui `ChannelRepository`; URL upstream tetap hanya untuk konfigurasi infrastruktur. `media.Process` tidak bergantung pada OS, sedangkan runner konkretnya berada di `infra/runtime/subprocess`. Adapter tidak mengimpor UseCase, handler, atau repository konkret; repository tidak memanggil repository lain. Tes arsitektur memeriksa batas ini dan menolak import lama `pkg`.

Pembagian pengelolaan proses background:

| Domain | File worker |
| --- | --- |
| `video` | Tidak ada worker encoding. MediaMTX meneruskan video NVR ke pembaca WebRTC. |
| `audio` | `worker.go` mengelola lifecycle; `capture.go` menyediakan `StartCaptureLoop` untuk pembacaan/reconnect; `backend.go` menyediakan `StartMetricDeliveryLoop` dan `EnqueueMetric` untuk antrean 128 item serta pengiriman background. |
| `streaming` | `worker.go` mengelola lifecycle dan daftar tugas; `restream.go` menangani retry, cancel, dan wait per target. |
| `saving` | `worker.go` mengelola lifecycle; `recording.go` mengawasi proses dan menunggu finalisasi; `upload.go` menjadwalkan upload dan timeout; `guard.go` menjadwalkan pemeriksaan disk/durasi. |

Alur video:

```text
server.Run() → MediaMTX.Start() → tarik sumber RTSP NVR
Browser meminta /<id>/whep → MediaMTX mengirim video melalui WebRTC
GET /api/v1/channels → handler video → UseCase.Channels() → metadata URL
```

UseCase video hanya menerima katalog kanal dan resolver URL playback. Tidak ada penjadwalan transcode maupun pemilihan mode video; semua kanal memakai video yang telah dikonfigurasi pada NVR.

Audio, streaming, dan saving tetap menggunakan pola executor yang sama seperti sebelumnya:

```text
server.Run()
  → UseCase.Start()
    → executor worker domain menerima callback operasi
      → worker menjalankan callback UseCase sesuai jadwal
        → UseCase memanggil service / repository / kalkulasi domain
```

Pada audio, streaming, dan saving, DI membuat worker terlebih dahulu, lalu menyuntikkannya melalui interface `Executor` yang dideklarasikan UseCase. Worker tidak mengimpor implementasi UseCase atau repository. Ketiga domain tersebut tetap memakai callback tanpa siklus dependency; video tidak memerlukan executor Go. `internal/infra/runtime/jobgroup` menyediakan mekanisme umum untuk penerimaan tugas, cancellation, dan WaitGroup, sedangkan jadwal spesifik tetap berada di worker domain.

Worker tidak menentukan status bisnis: kelayakan upload, backoff `NextAttempt`, batas disk/durasi, recovery, finalisasi file, dan perubahan status tetap dikerjakan UseCase. Callback penyelesaian rekaman harus selesai sebelum worker menandai tugas selesai. Saat shutdown saving, UseCase menolak ronde baru, meminta worker menghentikan perekam secara paralel, menunggu callback finalisasi, lalu menutup worker dan membatalkan upload yang masih aktif. Tidak ada mutex state bisnis yang ditahan selama menunggu callback tersebut.

Worker tetap berjalan dalam executable server yang sama. Ingest NVR dan worker audio dimulai saat startup; video WebRTC diterima ketika browser membuka player; restream dan recording dimulai sesuai perintah operator. Reconnect tetap lima detik, upload tetap berurutan dengan timeout 30 menit, dan backlog metrik tetap best effort tanpa memblokir capture. Tidak ada perubahan endpoint, konfigurasi, atau format penyimpanan akibat pemisahan worker.

Inisialisasi sumber NVR berada di `internal/mediaengine/channel/repository`: katalog kanal dari `.env`, validasi kanal, dan resolusi URL hub lokal. `app.NewMedia` mengambil snapshot sumber dari katalog dan menyuntikkannya ke `mediamtx.Options.Sources`. `internal/mediaengine/media/mediamtx` membangun konfigurasi upstream dari snapshot tersebut, membuka sesi RTSP melalui proses MediaMTX, mengelola lifecycle hub, dan menyediakan URL WHEP. Kredensial RTSP upstream hanya digunakan oleh infrastruktur; UseCase memakai URL lokal dari katalog NVR. `config` hanya memuat/memvalidasi konfigurasi dan tidak lagi menulis konfigurasi MediaMTX. Opsi `-render-config` memanggil renderer hub tanpa menjalankan proses.

Alamat UDP media WebRTC dapat diatur dengan `WEBRTC_LOCAL_UDP_ADDR` (default `:8189`); sesuaikan firewall jika port diganti. `Close()` menunggu proses MediaMTX berhenti sebelum mengizinkan `Start()` berikutnya. Jika daftar `ReadCIDRs` pada opsi hub kosong, renderer hanya memberikan akses loopback; rule dengan `ips: []` tidak dibuat karena MediaMTX mengartikannya sebagai semua IP. Sumber tetap selalu aktif (`sourceOnDemand: false`) agar audio dan perekaman siap digunakan, dan API MediaMTX tetap nonaktif.

Saat startup: buka database arena dan pulihkan ronde aktif menjadi `INTERRUPTED`, lalu jalankan modul media, worker, dan HTTP. Modul media yang gagal tidak menutup modul penilaian; database arena wajib tersedia agar nilai tidak diterima tanpa penyimpanan. Saat shutdown: hentikan penerimaan request dan perintah cloud, tutup WebSocket, finalisasi rekaman, hentikan worker, lalu tutup media. Menghentikan restream tidak menghentikan penilaian, audio, video lokal, atau recording. Endpoint media lama, format WebSocket, manifest rekaman, dan object key S3 tetap kompatibel.

Struktur domain memisahkan penilaian dan media. SQLite menyimpan event, akun/sesi juri, history nilai, dan outbox secara transaksional; manifest file tetap digunakan untuk antrean rekaman. MySQL, Redis/Asynq, dan JWT SaaS tidak diperlukan untuk operasional arena. Pembacaan metadata rekaman masih memindai manifest; kelola arsip sesuai volume satu arena.

Pengujian mencakup perhitungan audio, validasi bisnis ronde, konflik, disk guard, finalisasi/recovery dan retry upload persisten, allowlist restream, autentikasi/origin, resolusi sumber NVR, pemetaan kanal langsung MediaMTX, isolasi lifecycle antar domain, rollback startup termasuk upload aktif, kompatibilitas HTTP/WebSocket, dan flush MP4 dengan FFmpeg sintetis bila executable tersedia. Pengujian worker memakai proses palsu dan timer terkontrol untuk reconnect, antrean penuh, penjadwalan berurutan, cancel saat retry, penantian finalisasi, dan penolakan tugas setelah close. Uji lapangan tetap diperlukan: 24 feed asli, putus internet ketika upload, restart ketika merekam, disk mendekati batas, codec browser juri, serta ingest YouTube/S3 yang sebenarnya.

Referensi: [MediaMTX v1.15.3 config](https://github.com/bluenviron/mediamtx/blob/v1.15.3/mediamtx.yml), [re-encoding](https://mediamtx.org/docs/usage/re-encoding), [forwarding dan YouTube](https://mediamtx.org/docs/features/forward).
