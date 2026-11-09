# Memahami mediaengine: dari kamera sampai rekaman dan cloud

Panduan ini untuk developer yang ingin memahami alur sebelum membaca semua file. Penjelasan mengikuti implementasi saat ini. Mulailah dari gambaran media, kemudian satu alur fitur; tidak perlu membuka seluruh folder sekaligus.

## 1. Pegangan utama: Go mengendalikan, MediaMTX dan FFmpeg mengolah media

Local-app menjalankan live LAN, analisis audio, rekaman, restream, dan upload. Login pengguna, peserta, nilai juri, dan aturan perlombaan menjadi urusan cloud-app.

Ada dua alur yang perlu dibedakan:

| Alur | Contoh | Yang bergerak |
| --- | --- | --- |
| Kontrol | Mulai rekaman channel gantangan01 | Request JSON, pemanggilan UseCase, status pekerjaan |
| Media | Kamera mengirim video/audio | Stream RTSP/WebRTC, PCM, dan file MP4 |

Request mulai rekaman tidak membawa byte video. Request tersebut meminta Go menjalankan proses FFmpeg yang mengambil stream dari MediaMTX.

```mermaid
flowchart LR
    C["Kamera / NVR / sumber uji"] -->|RTSP| M["MediaMTX lokal"]
    M -->|WebRTC lewat LAN| B["Browser operator / juri"]
    M -->|RTSP lokal| A["FFmpeg decode audio"]
    A -->|PCM| G["Go: analisis dan distribusi audio"]
    M -->|RTSP lokal| R["FFmpeg recording"]
    R --> F["MP4 video + audio"]
    F --> U["Go uploader S3"]
    M -->|RTSP lokal| S["FFmpeg restream"]
    S --> T["Target streaming terkonfigurasi"]
```

Signaling browser melewati gateway WHEP dan otorisasi cloud. Setelah koneksi terbentuk, media WebRTC mengalir dari MediaMTX ke browser; tidak dibungkus menjadi response JSON.

## 2. Fungsi lima kelompok di mediaengine

| Folder | Pertanyaan yang dijawab | Mulai membaca |
| --- | --- | --- |
| `channel` | Channel apa yang tersedia dan URL playback-nya apa? | [channel/usecase/channel.go](internal/mediaengine/channel/usecase/channel.go) |
| `audio` | Bagaimana mengambil PCM, menghitung metrik, dan membagikannya? | [audio/usecase/audio.go](internal/mediaengine/audio/usecase/audio.go) |
| `streaming` | Bagaimana memulai/menghentikan restream ke target? | [streaming/usecase/streaming.go](internal/mediaengine/streaming/usecase/streaming.go) |
| `saving` | Bagaimana sesi direkam, dipulihkan, lalu diupload? | [saving/usecase/session.go](internal/mediaengine/saving/usecase/session.go) |
| `media` | Bagaimana konfigurasi MediaMTX dan argumen FFmpeg dibentuk? | [media/mediamtx/server.go](internal/mediaengine/media/mediamtx/server.go) |

Nama `streaming` pada kode saat ini merujuk **restream ke target**. Live browser melalui LAN ditangani MediaMTX dan gateway WHEP. Nama `saving` mencakup **recording beserta antrean upload**.

## 3. Arti lapisan kode dan interface

```text
Handler → UseCase → Repository / Service
                       ↓
                 Filesystem / FFmpeg / S3
```

| Komponen | Pekerjaan |
| --- | --- |
| Handler | Membaca request/auth/context, validasi sintaks, memanggil UseCase, memetakan DTO, mengirim response |
| UseCase | Memeriksa aturan operasi, mengoordinasi dependency, dan menentukan perubahan status |
| Repository | Membaca/menulis katalog atau manifest; tidak memanggil repository lain |
| Service/adapter | Menjalankan kemampuan teknis seperti FFmpeg, broadcaster, atau upload S3 |
| Worker | Menjalankan pekerjaan background, penjadwalan/retry, dan menunggu proses |
| Model | Bentuk data yang digunakan kontrak dan logika fitur |

Contoh: UseCase saving menerima interface `Recorder`. Container memasang `ffmpeg.Recorder` sebagai implementasinya. UseCase memutuskan kapan merekam; adapter FFmpeg membentuk argumen; subprocess menjalankan proses OS.

Worker menerima fungsi/callback dari UseCase. Karena itu, kode bisa tampak bolak-balik: UseCase meminta worker memantau proses, lalu worker memanggil callback UseCase ketika proses selesai. Ini pemisahan penjadwalan dari aturan perubahan status.

## 4. Bagaimana aplikasi mulai berjalan?

Urutan bacanya:

1. [cmd/api/main.go](cmd/api/main.go): memuat konfigurasi dan menjalankan server.
2. [app/container.go](internal/app/container.go): membuat katalog, MediaMTX, adapter FFmpeg, repository, worker, UseCase, dan handler. Ini tempat melihat implementasi apa yang dipasang pada setiap interface.
3. [server/server.go](internal/server/server.go): mengunci direktori, menjalankan supervisor MediaMTX, memulai saving/audio/streaming, kemudian worker komunikasi cloud.
4. [delivery/router.go](internal/delivery/router.go): memasang middleware serta route REST, WHEP, dan WebSocket.

Supervisor MediaMTX berjalan secara asynchronous. Proses startup HTTP tidak berarti seluruh kamera sudah tersambung. Audio/restream memiliki retry ketika sumber belum tersedia.

Saat startup modul:

- Saving membaca manifest lama, menyiapkan direktori, memulai upload jika S3 dikonfigurasi, dan menjalankan pemeriksaan batas disk/durasi.
- Audio memulai capture untuk seluruh channel yang dikonfigurasi. Pengiriman metrik backend hanya dimulai jika backend diisi.
- Streaming menyiapkan pengelola pekerjaan; restream dimulai setelah ada request/command.

`/healthz` menunjukkan HTTP hidup. `/readyz` memeriksa status proses MediaMTX, heartbeat cloud, dan konfigurasi komunikasi yang diperlukan; bukan bukti bahwa setiap kamera dan upload sudah sehat.

## 5. Alur katalog channel dan live LAN

### Katalog

```text
GET /api/v1/channels
→ delivery/rest/channel/handler.go
→ channel/usecase.Channels()
→ repository katalog + interface playback
→ DTO ChannelResponse
```

`channel/model.Channel` hanya menyimpan identitas. `PlaybackChannel` menambahkan URL WHEP publik. Kredensial kamera tetap berada pada konfigurasi sumber, tidak dikirim melalui response katalog.

### Menonton live

```text
Browser meminta tiket ke cloud
→ POST /api/v1/playback/:channel/whep membawa SDP dan tiket
→ gateway WHEP memanggil UseCase cloud untuk validasi
→ signaling diteruskan ke MediaMTX loopback
→ browser menerima SDP dan Location sesi
→ MediaMTX mengirim media WebRTC melalui LAN
```

File utamanya: [gateway WHEP](internal/delivery/whep/handler.go), [otorisasi playback](internal/cloud/usecase/playback.go), dan [server MediaMTX](internal/mediaengine/media/mediamtx/server.go).

Tiket salah/expired atau cloud tidak tersedia menolak pembukaan sesi baru. URL sesi dari Location adalah capability privat untuk PATCH/DELETE. Perangkat juga harus mempunyai akses LAN, HTTPS yang dipercaya, CORS yang benar, serta izin browser yang diperlukan.

**Batas implementasi:** endpoint pendamping cloud untuk tiket/otorisasi belum aktif. Adanya kode gateway lokal belum berarti login dan live dari frontend cloud sudah tersambung end-to-end.

## 6. Alur audio: PCM dan metrik adalah dua keluaran

```text
audio.UseCase.Start()
→ worker capture per channel
→ decoder FFmpeg membaca RTSP lokal
→ PCM mono 16 kHz, signed 16-bit little-endian
→ Analyze()
→ broadcaster
   ├── audio:{channel} → WebSocket binary PCM
   └── metrics:        → WebSocket text JSON metrik
→ antrean backend metrik jika dikonfigurasi
```

Urutan file: [audio UseCase](internal/mediaengine/audio/usecase/audio.go) → [decoder](internal/mediaengine/media/ffmpeg/audio.go) → [analyzer](internal/mediaengine/audio/model/analyzer.go) → [broadcaster](internal/mediaengine/audio/service/broadcaster.go).

WebSocket audio tersedia pada `/api/v1/channels/:id/audio`, sedangkan metrik pada `/api/v1/metrics`. Handler berada di [delivery/websocket](internal/delivery/websocket/handler.go), bukan di domain audio.

Metrik saat ini berisi peak dBFS, RMS dBFS, active_ms berdasarkan ambang, dan window_ms. Ini belum berupa penilaian kualitas kicauan atau deteksi otomatis peserta.

Subscriber lambat dapat kehilangan frame. Antrean backend juga bersifat best effort: jika penuh, metrik dapat dibuang; pengiriman gagal dicatat. Jangan menyamakannya dengan antrean upload rekaman yang persisten. Mekanisme ini menjaga capture tidak tertahan oleh konsumen lambat.

Audio dari live WebRTC tidak melewati broadcaster PCM ini. Kedua jalur membaca media yang sama untuk tujuan berbeda: playback browser dan pemrosesan audio khusus.

## 7. Alur restream

Contoh request kontrol:

```json
{"channel_id":"gantangan01","target":"youtube"}
```

```text
POST /api/v1/restreams
→ handler REST streaming
→ StartRestream(channel, target)
→ validasi channel dan target allowlist
→ worker.Launch()
→ ffmpeg.Restreamer
→ proses FFmpeg mengirim ke tujuan
```

UseCase menyimpan status target; worker menangani proses dan reconnect. Target yang sudah aktif ditolak sebagai konflik. Saat koneksi/proses putus, worker mencoba lagi setelah jeda 5 detik sampai dibatalkan.

`DELETE /api/v1/restreams/:target` membatalkan pekerjaan target dan menunggu proses selesai. `GET /api/v1/restreams` membaca status yang disimpan UseCase.

Status RUNNING menandakan proses berhasil dimulai, bukan konfirmasi bahwa platform tujuan sudah menayangkan siaran. Implementasi sekarang memakai encoding video H.264 ulang; belum ada pemilihan stream-copy otomatis.

Mulai membaca [UseCase streaming](internal/mediaengine/streaming/usecase/streaming.go), lalu [worker restream](internal/mediaengine/streaming/worker/restream.go) dan [adapter FFmpeg](internal/mediaengine/media/ffmpeg/streaming.go).

## 8. Alur rekaman: ikuti satu sesi dahulu

Contoh request:

```http
POST /api/v1/media/recording-sessions
Content-Type: application/json
Authorization: Bearer TOKEN_OPERATOR_LOKAL
```

```json
{"id":"session-001","channels":["gantangan01","gantangan02"]}
```

ID channel harus sudah ada pada konfigurasi. Satu request dapat membuat satu rekaman per channel; file MP4 mengandung video dan audio jika sumber memiliki audio. Sesi media ini tidak mengelola lifecycle ronde penjurian.

```mermaid
sequenceDiagram
    participant H as Handler REST
    participant U as UseCase saving
    participant J as Repository manifest
    participant F as FFmpeg
    participant W as Worker pemantau
    H->>U: StartSession(id, channels)
    U->>U: Validasi dan periksa kapasitas
    U->>J: Simpan reservasi RECORDING
    U->>F: Mulai tulis file .partial
    U->>W: Watch proses + callback finalisasi
    U-->>H: Daftar rekaman, HTTP 202
    F-->>W: Proses selesai
    W->>U: finishRecording(error)
    U->>J: Simpan READY atau FAILED
```

Urutan konkretnya:

1. Handler membaca JSON dan memanggil `StartSession()`.
2. `reserve()` memeriksa lifecycle engine, format ID, daftar channel, duplikasi, konflik sesi, serta ruang disk. Sesi lain tidak boleh dimulai ketika masih ada recording aktif pada engine ini.
3. Dibuat ID rekaman dan manifest RECORDING per channel sebelum proses dimulai.
4. Adapter recorder menjalankan FFmpeg untuk menulis file `.mp4.partial`; worker memantau proses tersebut.
5. Jika startup salah satu rekaman gagal, UseCase melakukan rollback terhadap pekerjaan/reservasi terkait.
6. Ketika proses selesai, `finishRecording()` memeriksa hasil. Proses sukses dengan file partial berukuran positif dapat di-commit menjadi nama final `.mp4`, lalu READY. Selain itu hasil menjadi FAILED.

**HTTP 202 bukan berarti file selesai atau sudah diupload.** Baca status rekaman melalui `GET /api/v1/recordings`.

Untuk menghentikan sesi, panggil `POST /api/v1/media/recording-sessions/:id/stop`. UseCase meminta penghentian proses, lalu menunggu callback finalisasi. Subprocess mencoba mengakhiri FFmpeg dengan `q` agar container MP4 ditutup; jika tidak selesai dalam batasnya, proses dapat dipaksa berhenti.

### File mana yang dibuka saat mengerjakan saving?

| Perlu memahami | Buka |
| --- | --- |
| Dependency, state, startup/shutdown | [saving.go](internal/mediaengine/saving/usecase/saving.go) |
| Mulai/stop, rollback, finalisasi | [session.go](internal/mediaengine/saving/usecase/session.go) |
| Daftar rekaman dan pencarian sesi | [query.go](internal/mediaengine/saving/usecase/query.go) |
| Pemulihan manifest setelah restart | [recovery.go](internal/mediaengine/saving/usecase/recovery.go) |
| Antrean upload dan retry | [upload.go](internal/mediaengine/saving/usecase/upload.go) |
| Kapasitas disk dan batas durasi | [limits.go](internal/mediaengine/saving/usecase/limits.go) |
| Penulisan/pembacaan manifest | [repository/recording.go](internal/mediaengine/saving/repository/recording.go) |
| Pemantauan dan penghentian proses | [worker/recording.go](internal/mediaengine/saving/worker/recording.go) |

Keenam file UseCase berada dalam package yang sama. Semuanya menggunakan satu `UseCase`, map proses aktif, mutex, dan state lifecycle bersama; bukan enam service yang berjalan sendiri-sendiri.

## 9. Alur upload dan pelaporan cloud

```text
Manifest READY
→ worker menjalankan uploadPending()
→ UseCase memilih rekaman yang boleh diupload
→ adapter S3 mengirim file dengan object key tetap
→ manifest disimpan sebagai UPLOADED, atau dijadwalkan retry
→ worker cloud membaca versi manifest
→ metadata dijournal dan dikirim sampai acknowledged
```

Upload dimulai hanya jika S3 dikonfigurasi. Worker melakukan satu pass, lalu menunggu 5 detik sebelum pass berikutnya. Rekaman ditunda jika masih ada recording dalam sesi yang sama atau NextAttempt belum tiba. Batas satu upaya upload adalah 30 menit; retry memakai backoff yang tersimpan pada manifest.

Upload file dan pengiriman metadata adalah dua pekerjaan terpisah. Jika metadata belum diakui cloud, file yang sudah berhasil diupload tidak otomatis dikirim ulang hanya karena acknowledgment metadata belum masuk. Metadata belum diakui tetap berada dalam journal cloud.

Adapter S3 ada di [infra/storage/s3/s3.go](internal/infra/storage/s3/s3.go). Pelaporan metadata berada di [cloud/usecase/recording_metadata.go](internal/cloud/usecase/recording_metadata.go). Endpoint penerima metadata cloud masih memerlukan implementasi pendamping.

## 10. Data yang disimpan dan arti status

| Lokasi/field | Arti |
| --- | --- |
| `DATA_DIR/recordings/` | File MP4 dan file partial yang sedang/belum selesai |
| `DATA_DIR/queue/` | Manifest JSON rekaman: status, file, object key, versi, dan retry |
| `DATA_DIR/commands/` | Journal command cloud, hasil, dan acknowledgment |
| `DATA_DIR/media-updates/` | Journal metadata yang menunggu/sudah mendapat acknowledgment |
| `RUNTIME_DIR/mediamtx.yml` | Konfigurasi MediaMTX hasil generate, bukan database/rekaman |
| `round_id` dan awalan object key `rounds/` | Nama lama yang dipertahankan untuk kompatibilitas penyimpanan; dipakai sebagai referensi sesi media |

| Status rekaman | Makna |
| --- | --- |
| RECORDING | Reservasi/pekerjaan recording belum difinalisasi; bukan bukti setiap frame berhasil direkam |
| READY | File final tersedia dan menunggu upload bila S3 aktif |
| UPLOADED | Upload telah dilaporkan sukses dan status disimpan |
| FAILED | Startup/finalisasi rekaman gagal |
| INTERRUPTED | Recovery menemukan recording lama tanpa penanda file final yang tersedia |

Saat restart, recovery tidak melanjutkan proses FFmpeg lama. Manifest RECORDING diperiksa: jika nama file final ada dan ukurannya positif, status dapat menjadi READY; jika tidak, menjadi INTERRUPTED. Recovery ini tidak otomatis memperbaiki semua file partial atau membuktikan validitas seluruh isi MP4.

## 11. Apa yang terjadi jika ada gangguan?

| Gangguan | Perilaku saat ini |
| --- | --- |
| Sumber audio terputus | Worker capture mencoba tersambung lagi |
| Restream berhenti | Worker mencoba menjalankan ulang sampai target dihentikan |
| Satu channel gagal mulai recording | UseCase melakukan rollback pekerjaan/reservasi terkait |
| Disk terlalu sedikit / durasi habis | Guard meminta penghentian recording aktif yang terkena batas |
| S3 gagal | Rekaman tetap READY dan retry dijadwalkan |
| Cloud terputus | Recording aktif berlanjut; playback baru yang butuh otorisasi ditolak; metadata menunggu |
| Aplikasi restart | Manifest dan journal dipakai untuk recovery; proses lama tidak otomatis dianggap masih berjalan |

Saat shutdown normal, worker cloud dihentikan sebelum modul media difinalisasi. Koneksi WebSocket ditutup. Saving menunggu callback finalisasi sebelum menutup worker-nya; supervisor MediaMTX dihentikan setelah modul media. Jangan membalik urutan ini tanpa memahami ketergantungan proses.

## 12. Urutan belajar yang disarankan

1. Baca `channel/usecase/channel.go`: alur pendek, tidak ada proses background.
2. Ikuti satu start/stop recording dari handler ke `session.go`, recorder FFmpeg, worker pemantau, lalu callback finalisasi.
3. Baca `upload.go` dan manifest repository untuk memahami antrean persisten.
4. Baca audio untuk memahami subscription dan frame yang dapat di-drop.
5. Baca restream untuk memahami reconnect dan cancellation.
6. Terakhir buka container/server untuk melihat semua komponen dirangkai dan dihentikan.

Ketika bingung, tentukan dahulu apakah yang dicari adalah request kontrol, byte media, state bisnis operasi, atau penjadwalan background. Empat hal ini berada pada komponen berbeda.

Panduan ini menjelaskan perilaku kode, bukan hasil uji kamera/CPU atau bukti integrasi cloud sudah selesai. Lihat [README](README.md) untuk konfigurasi, [peta kode](PETA-KODE.md) untuk indeks seluruh file, dan [kontrak cloud](ENGINE-CLOUD-CONTRACT.md) untuk payload komunikasi.
