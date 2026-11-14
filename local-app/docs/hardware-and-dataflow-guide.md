# Panduan Alur Komunikasi Data & Testing Device Nyata (Hardware, NVR ONVIF, & Media)

Dokumen ini menjelaskan bagaimana data audio dan video mengalir secara fisik di arena lomba burung, protokol yang dipakai di luar REST API, serta panduan praktis melakukan pengujian langsung dengan kamera/NVR fisik di lapangan (berdasarkan hasil verifikasi nyata di hardware).

---

## 1. Gambaran Perangkat Keras di Arena

Di arena fisik terdapat 4 komponen utama yang saling terhubung dalam satu jaringan LAN lokal:

```
[ Kamera CCTV / NVR ] (Di tiang / atas gantangan)
       │ (Kabel LAN UTP)
       ▼
 [ Switch / Router Wi-Fi Arena ] (Subnet misal: 192.168.1.0/24)
       ▲                              ▲
       │ (Kabel LAN UTP)              │ (Sinyal Wi-Fi Arena)
       ▼                              │
[ Komputer Mini PC Engine ]           ▼
   (Menjalankan local-app) ─────► [ HP / Tablet / Laptop Juri ]
   IP misal: 192.168.1.106        IP misal: 192.168.1.150
```

1. **Kamera CCTV / NVR**: Mengambil gambar burung dan suara di gantangan (misal IP `192.168.1.2`).
2. **Switch / Router Wi-Fi**: Menghubungkan NVR, Mini PC, dan gadget juri ke subnet IP yang sama (misal `192.168.1.x`).
3. **Komputer Mini PC Engine**: Tempat program backend `local-app` berjalan di Windows.
4. **Perangkat Juri / Operator**: Membuka browser (Chrome/Edge) untuk melihat video live gantangan tanpa delay dan grafik desibel suara burung.

---

## 2. Tiga Program di Dalam Mini PC & Struktur Folder `bin`

Di dalam Mini PC, ada 3 aktor internal yang bekerja secara simultan:

```
┌─────────────────────────────────────────────────────────────┐
│                       MINI PC ENGINE                        │
│                                                             │
│  [ local-app (Go Backend) ] ── (Mandor / Orchestrator)      │
│         │                                                   │
│         ├── Memerintah ──► [ FFmpeg & ffprobe ]             │
│         │                     │                             │
│         │                     ├── Cek stream nyata (Probe)  │
│         │                     ├── Transcode / Copy video    │
│         │                     ├── Simpan .mp4 ke Disk       │
│         │                     └── Hitung desibel (dBFS)     │
│         │                                                   │
│         └── Mengawasi  ──► [ MediaMTX ] (Server WebRTC)     │
│                                 │                           │
│                                 └── Pancarkan ke HP Juri    │
└─────────────────────────────────────────────────────────────┘
```

### Struktur Binary yang Wajib Ada
Sistem portable `local-app` mencari binary pada path relatif `./bin/` (sesuai `.env.portable.example`). File executable berikut **wajib berada di dalam folder `local-app\bin\`**:
```
local-app/
├── bin/
│   ├── ffmpeg.exe      (Pengolah media, segmentasi MP4, dan transcode)
│   ├── ffprobe.exe     (Pemeriksa paket stream nyata kamera/NVR)
│   └── mediamtx.exe    (Server streaming WebRTC internal v1.15.3)
├── cmd/
├── internal/
└── .env.portable.example
```
> **Catatan Penting:** Karena `.env.portable.example` menggunakan path eksplisit `./bin/ffprobe.exe`, mengubah `$env:PATH` di terminal tidak akan dibaca oleh Go (`exec.LookPath` mencari langsung ke path relatif `./bin/`). Pastikan file binary fisik benar-benar diletakkan di dalam folder `local-app\bin\`.

---

## 3. Alur Komunikasi Data Non-REST (Jalur Pipa Media)

Selain REST API (yang dipakai untuk klik tombol dan bertukar JSON), ada **3 pipa data kontinu**:

```
[NVR / Kamera CCTV] 
      │
      │ Pipa 1: RTSP (TCP/UDP Port 554)
      ▼
 [FFmpeg di Mini PC]
      ├──► Harddisk: Simpan potongan segmen .mp4 (data/recordings/)
      │
      ├──► Pipa 2: WebSocket (ws:// - Port 8090, subprotocol: edge-v1)
      │            └─► Kirim data desibel suara burung ke HP Juri per 200 ms
      │
      └──► MediaMTX (Internal Loopback)
                 │
                 │ Pipa 3: WebRTC / WHEP (UDP Port 8189)
                 ▼
          [Layar Browser HP Juri] (Ultra-low latency < 300 ms)
```

### Pipa 1: Dari NVR ke Mini PC (RTSP Port 554)
NVR bertindak sebagai pemancar pasif. Dia terus-menerus menyemburkan paket data video dan audio lewat RTSP port `554`. `local-app` melalui FFmpeg bertindak sebagai penerima yang menyedot semburan tersebut saat sesi ronde aktif.

### Pipa 2: Dari Mini PC ke Layar HP Juri (WebRTC via WHEP)
Streaming konvensional (HLS/RTMP/YouTube) memiliki delay 3–10 detik, tidak cocok untuk penilaian real-time lomba burung. 
* WHEP (*WebRTC HTTP Egress Protocol*) melakukan jabat tangan SDP offer/answer melalui HTTP REST port `8090`.
* Media video kemudian dialirkan langsung melalui **UDP port 8189** dengan latensi di bawah 300 milidetik (*bufferless*).

### Pipa 3: Grafik Suara Burung ke HP Juri (WebSocket)
Koneksi pipa dua arah terbuka di `ws://<ip-engine>:8090/api/v1/metrics` dengan subprotocol `edge-v1`. Setiap 200 milidetik, backend mendorong frame JSON:
```json
{"channel_id":"gantangan01","at":"2026-11-14T00:20:10Z","peak_dbfs":-14.2,"rms_dbfs":-22.1,"active_ms":180,"window_ms":200}
```

---

## 4. Panduan Eksekusi Testing NVR Fisik (Langkah demi Langkah)

Berikut panduan riil yang terbukti berhasil saat menguji NVR/Kamera di lapangan (contoh kasus: NVR IP `192.168.1.2`, PC Engine `192.168.1.106`).

### Langkah 1 — Jalankan Backend `local-app`
Buka terminal PowerShell di folder `local-app`:
```powershell
go run ./cmd/api -env .env.portable.example
```
Backend akan aktif di `http://127.0.0.1:8090`.

### Langkah 2 — Buka Swagger UI & Ambil Nonce Operator
Setup hanya bisa dikontrol melalui **loopback langsung** dari komputer Mini PC itu sendiri:
1. Buka browser: **`http://localhost:8090/api/docs/index.html`**
2. Buka endpoint **`GET /api/v1/setup/bootstrap`** -> klik **Try it out** -> **Execute**.
3. Salin nilai `data.nonce` dan catat IP kartu jaringan PC yang satu subnet dengan NVR (misal `"192.168.1.106"`).
4. Klik tombol hijau **Authorize** di pojok kanan atas Swagger:
   - Masukkan string `nonce` ke kolom **LocalControl** (`X-Local-Control`) -> klik **Authorize** -> **Close**.

### Langkah 3 — Cari Endpoint ONVIF Asli (`action: "discover"`)
Port ONVIF NVR sering kali bukan port 80 (pada NVR Xiongmai/XM/Umum adalah port **`8899`**, pada Hikvision/Dahua port **`8000`** atau **`80`**).
Jalankan pencarian otomatis di **`POST /api/v1/setup/jobs`**:
```json
{
  "action": "discover",
  "ip": "192.168.1.106"
}
```
Cek hasil job di **`GET /api/v1/setup/jobs/{id}`**:
```json
"result": [
  {
    "endpoint": "http://192.168.1.2:8899/onvif/device_service",
    "name": "192.168.1.2"
  }
]
```

### Langkah 4 — Baca Profil Stream NVR (`action: "profiles"`)
Kirim request ke **`POST /api/v1/setup/jobs`**:
```json
{
  "action": "profiles",
  "ip": "192.168.1.106",
  "endpoint": "http://192.168.1.2:8899/onvif/device_service",
  "username": "admin",
  "password": "admin"
}
```
> **Peringatan Syntax JSON:** Jangan biarkan Swagger mengisi template default seperti `"channel"` atau `"tested_at": "string"`. Cukup kirim field yang diperlukan di atas agar tidak ditolak dengan pesan `invalid request syntax`.

Cek hasil di **`GET /api/v1/setup/jobs/{id}`**:
Backend akan mengembalikan daftar profil (misal `mainStream`, `subStream`), URL RTSP lengkap dengan token autentikasi, serta `credential_ref` sementara.

### Langkah 5 — Uji Paket Stream Nyata (`action: "probe"`)
Verifikasi apakah stream RTSP benar-benar bisa ditarik oleh FFprobe:
```json
{
  "action": "probe",
  "channel": {
    "id": "gantangan01",
    "source": "rtsp://192.168.1.2:554/user=admin_password=tlJwpbo6_channel=0_stream=0&onvif=0.sdp",
    "live_mode": "transcode"
  }
}
```
Hasil probe di **`GET /api/v1/setup/jobs/{id}`** akan membeberkan karakteristik teknis kamera:
* **`video_codec`**: `hevc` (H.265) atau `h264`
* **`audio_codec`**: `pcm_alaw`, `aac`, dsb.
* **`width` & `height`**: Resolusi asli (misal `2304x1296`)
* **`fps`**: Frame rate (misal `20`)
* **`b_frames`**: Jumlah B-frame (`0` ideal untuk WebRTC)

---

## 5. Menentukan `live_mode`: "copy" vs "transcode"

WebRTC pada browser HP juri secara universal membutuhkan codec **H.264**. Di lapangan, banyak NVR generasi baru hanya mendukung **H.265 / H.265X**:

| Kondisi Codec Kamera | Pilihan `live_mode` | Perilaku Backend & Kualitas |
| :--- | :--- | :--- |
| **Kamera H.264** (dan `b_frames: 0`) | `"copy"` | **Zero CPU overhead**. Stream video langsung dialirkan ke WebRTC tanpa encode ulang. |
| **Kamera H.265 / HEVC** (hanya ada H.265 di NVR) | `"transcode"` | **Wajib**. Backend otomatis mengonversi live stream menjadi H.264 (640x360 15 FPS) agar browser HP bisa memutar video, **sedangkan file rekaman MP4 tetap disimpan dalam resolusi asli H.265 2304x1296**. |

> **Catatan Validasi:** Jika kamera menghasilkan H.265 namun Anda memaksa `live_mode: "copy"`, backend pada tahap `apply` akan menolak dengan error `UNSUPPORTED_CODEC: Pilih profil H.264 tanpa B-frame atau aktifkan konversi H.264`.

---

## 6. Menyimpan Konfigurasi (`action: "apply"`)

Sebelum menjalankan `apply`, pastikan file **`mediamtx.exe`** (v1.15.3) sudah berada di `local-app\bin\`. Download jika belum ada:
```powershell
Invoke-WebRequest -Uri "https://github.com/bluenviron/mediamtx/releases/download/v1.15.3/mediamtx_v1.15.3_windows_amd64.zip" -OutFile "mediamtx.zip"
Expand-Archive -Path "mediamtx.zip" -DestinationPath "local-app\bin" -Force
Remove-Item "mediamtx.zip"
```

Lalu kirimkan perintah `apply` ke **`POST /api/v1/setup/jobs`**:
```json
{
  "action": "apply",
  "configuration": {
    "version": 1,
    "lan_ip": "192.168.1.106",
    "lan_cidr": "192.168.1.0/24",
    "channels": [
      {
        "id": "gantangan01",
        "source": "rtsp://192.168.1.2:554/user=admin_password=tlJwpbo6_channel=0_stream=0&onvif=0.sdp",
        "live_mode": "transcode"
      }
    ]
  }
}
```
Setelah job berstatus `SUCCEEDED`, konfigurasi terkunci secara aman dengan enkripsi DPAPI Windows.

---

## 7. Menjalankan Sesi Lomba (Operasional Nyata)

### A. Mulai Sesi (Start Session)
```http
POST /api/v1/media/sessions
X-Local-Control: <nonce>
Content-Type: application/json

{
  "id": "ronde-01",
  "channels": ["gantangan01"]
}
```
Backend mengembalikan status `202 Accepted` (`STARTING` → `RUNNING`). Detik ini juga:
1. FFmpeg mulai menyedot stream RTSP NVR.
2. File rekaman `.mp4` mulai ditulis ke folder `data/recordings/`.
3. Pipa WebRTC MediaMTX aktif.

### B. Membuka Live Video di Browser Juri
Browser juri di jaringan Wi-Fi arena membuka player WHEP:
* URL: `http://192.168.1.106:8090/api/v1/playback/gantangan01/whep`
* Video live langsung menyala tanpa delay (< 300 ms).

### C. Menghentikan Sesi (Stop Session)
```http
POST /api/v1/media/sessions/ronde-01/stop
X-Local-Control: <nonce>
```
1. Sesi WHEP dan WebSocket ditutup.
2. Pipeline rekaman FFmpeg dihentikan secara graceful.
3. Segmen-segmen video digabung dan divalidasi ke file MP4 final berstatus `READY`.

---

## 8. Ringkasan Diagnostik Error Realita Lapangan

| Kode Error / Gejala | Penyebab Nyata di Lapangan | Solusi Terverifikasi |
| :--- | :--- | :--- |
| `invalid request syntax` | Swagger UI otomatis menyisipkan dummy struct (`"tested_at": "string"`). | Hapus field yang tidak perlu. Cukup kirim field yang dibutuhkan action terkait. |
| `NO_PROFILES (0 layanan)` | Port ONVIF salah (misal mencoba port 80 padahal NVR memakai port lain). | Jalankan `action: "discover"` untuk mendeteksi port ONVIF asli (misal port `8899`). |
| `BINARY_MISSING` (`ffprobe tidak ditemukan`) | File `ffprobe.exe` tidak berada di folder `local-app\bin\`. | Salin `ffprobe.exe` dan `ffmpeg.exe` langsung ke dalam folder `local-app\bin\`. |
| `UNSUPPORTED_CODEC` saat apply | Kamera bertipe H.265 / HEVC tetapi di-apply dengan `live_mode: "copy"`. | Ubah `live_mode` menjadi `"transcode"` pada konfigurasi channel. |
| `MediaMTX tidak ditemukan` saat apply | File `mediamtx.exe` belum diunduh ke folder `local-app\bin\`. | Download MediaMTX v1.15.3 Windows dan letakkan file `mediamtx.exe` di folder `local-app\bin\`. |
| `403 Forbidden` pada request setup | Tidak menyertakan header `X-Local-Control` atau tidak diakses dari `localhost`. | Ambil nonce dari `GET /api/v1/setup/bootstrap` dan panggil via `http://127.0.0.1:8090`. |
