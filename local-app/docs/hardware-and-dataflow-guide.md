# Panduan Alur Komunikasi Data & Testing Device Nyata (Awam / Non-REST)

Dokumen ini menjelaskan bagaimana data audio dan video mengalir secara fisik di arena lomba burung, protokol yang dipakai di luar REST API, serta panduan praktis melakukan pengujian langsung dengan kamera/NVR fisik di lapangan.

---

## 1. Gambaran Perangkat Keras di Arena

Di arena fisik terdapat 4 komponen utama yang saling terhubung dalam satu jaringan LAN lokal:

```
[ Kamera CCTV / NVR ] (Di tiang / atas gantangan)
       │ (Kabel LAN UTP)
       ▼
 [ Switch / Router Wi-Fi Arena ]
       ▲                              ▲
       │ (Kabel LAN UTP)              │ (Sinyal Wi-Fi Arena)
       ▼                              │
[ Komputer Mini PC Engine ]           ▼
   (Menjalankan local-app) ─────► [ HP / Tablet / Laptop Juri ]
```

1. **Kamera CCTV / NVR**: Mengambil gambar burung dan suara di gantangan.
2. **Switch / Router Wi-Fi**: Menghubungkan CCTV, Mini PC, dan gadget juri ke subnet IP yang sama (misal `192.168.1.x`).
3. **Komputer Mini PC Engine**: Tempat program backend `local-app` berjalan di Windows/Linux.
4. **Perangkat Juri / Operator**: Membuka browser (Chrome/Edge) untuk melihat video live gantangan dan grafik suara.

---

## 2. Tiga Program di Dalam Mini PC

Di dalam Mini PC, ada 3 aktor internal yang bekerja secara simultan:

```
┌─────────────────────────────────────────────────────────────┐
│                       MINI PC ENGINE                        │
│                                                             │
│  [ local-app (Go Backend) ] ── (Mandor / Orchestrator)      │
│         │                                                   │
│         ├── Memerintah ──► [ FFmpeg ] (Kuli Olah Media)     │
│         │                     │                             │
│         │                     ├── Simpan .mp4 ke Disk       │
│         │                     ├── Hitung desibel (dBFS)     │
│         │                     └── Oper video ke MediaMTX    │
│         │                                                   │
│         └── Mengawasi  ──► [ MediaMTX ] (Server WebRTC)     │
│                                 │                           │
│                                 └── Pancarkan ke HP Juri    │
└─────────────────────────────────────────────────────────────┘
```

1. **`local-app` (Program Go Utama)**:
   Bertindak sebagai mandor. Mengatur konfigurasi, menerima perintah start/stop sesi, dan memastikan FFmpeg serta MediaMTX berjalan normal.
2. **`FFmpeg` (Pekerja Kasar Media)**:
   Program background yang menarik stream video dari CCTV, mengonversinya jika perlu, menulis potongan file rekaman MP4 ke harddisk, serta menghitung amplitudo suara burung per 200 ms.
3. **`MediaMTX` (Server Streaming / Pemancar)**:
   Program server media internal v1.15.3 yang menerima umpan video dari FFmpeg lalu mendistribusikannya ke ratusan browser penonton/juri melalui WebRTC tanpa delay.

---

## 3. Alur Komunikasi Data Non-REST (Jalur Pipa Media)

Selain REST API (yang dipakai untuk klik tombol dan bertukar JSON), ada **3 pipa data kontinu**:

```
[Kamera CCTV] 
      │
      │ Pipa 1: RTSP (TCP/UDP Port 554)
      ▼
 [FFmpeg di Mini PC]
      ├──► Harddisk: Simpan potongan segmen .mp4 (data/recordings/)
      │
      ├──► Pipa 2: WebSocket (ws:// - Port 8090)
      │            └─► Kirim data desibel suara burung ke HP Juri
      │
      └──► MediaMTX (Internal Loopback)
                 │
                 │ Pipa 3: WebRTC / WHEP (UDP Port 8189)
                 ▼
          [Layar Browser HP Juri] (Ultra-low latency < 300 ms)
```

### Pipa 1: Dari CCTV ke Mini PC (RTSP)
* **Protokol**: RTSP (*Real-Time Streaming Protocol*), default port `554`.
* **Karakter**: CCTV bertindak sebagai pemancar pasif. Dia terus-menerus menyemburkan paket data video H.264/H.265. `local-app` melalui FFmpeg bertindak sebagai penerima yang "menyedot" semburan tersebut saat sesi lomba dimulai.

### Pipa 2: Dari Mini PC ke Layar HP Juri (WebRTC via WHEP)
* **Kenapa bukan HLS / RTMP / YouTube?**
  Karena streaming konvensional memiliki delay 3 sampai 10 detik. Dalam lomba kicau, juri wajib melihat burung loncat atau berkicau secara instan (delay di bawah 300 milidetik).
* **Bagaimana WHEP bekerja?**
  1. Browser HP Juri mengirimkan surat kesiapan teknis ("SDP Offer") ke `local-app`.
  2. `local-app` membalas dengan parameter server MediaMTX ("SDP Answer").
  3. Terjadi negosiasi koneksi langsung (*ICE Candidate*).
  4. Video langsung mengalir lewat protokol **UDP** (port `8189`). Jika ada 1 frame hilang di jaringan Wi-Fi, sistem tidak menunggu paket itu ulang, melainkan langsung menampilkan frame berikutnya agar video tidak pernah macet (*bufferless*).

### Pipa 3: Grafik Suara Burung ke HP Juri (WebSocket)
* **Protokol**: WebSocket (`ws://` / `wss://`) pada port `8090`.
* **Karakter**: Berbeda dengan REST yang harus ditanya berulang-ulang (*polling*), WebSocket adalah sambungan pipa dua arah yang terus terbuka.
* **Isi Data**: Setiap 200 milidetik, backend mendorong frame JSON kecil berisi kekuatan suara:
  ```json
  {"channel_id":"gantangan01","peak_dbfs":-14.2,"rms_dbfs":-22.1,"active_ms":180}
  ```
  Browser juri menangkap data ini untuk menggerakkan grafik getaran suara secara real-time.

---

## 4. Panduan Eksekusi Testing dengan Perangkat Fisik (CCTV Asli)

Gunakan checklist ini saat membawa Mini PC dan mengujinya dengan NVR/kamera IP di lokasi.

### Langkah 1 — Fisik & Pengkabelan
1. Hubungkan kabel LAN dari **Kamera/NVR** ke **Switch/Router Arena**.
2. Hubungkan kabel LAN dari **Mini PC** ke **Switch/Router Arena** yang sama.
3. Sambungkan laptop/HP penguji ke Wi-Fi yang dipancarkan oleh Router Arena tersebut.
4. Pastikan IP subnet seragam. Contoh:
   - IP Router: `192.168.1.1`
   - IP Kamera: `192.168.1.64`
   - IP Mini PC: `192.168.1.100`
   - IP HP Juri: `192.168.1.150`

### Langkah 2 — Uji Selang CCTV Mandiri (Sebelum Menjalankan Aplikasi)
Sebelum menjalankan backend, buktikan terlebih dahulu bahwa selang RTSP dari kamera bisa disedot oleh Mini PC. Buka terminal Mini PC:

```powershell
ffprobe -i "rtsp://admin:password_cctv@192.168.1.64:554/Streaming/Channels/101"
```

> *Tips*: Path RTSP tergantung merk kamera:
> - **Hikvision**: `/Streaming/Channels/101` (Main Stream) atau `/Streaming/Channels/102` (Sub Stream)
> - **Dahua**: `/cam/realmonitor?channel=1&subtype=0`
> - **Umum/ONVIF**: `/live/ch0` atau sesuai manual NVR.

**Periksa output teks `ffprobe`**:
- Jika tertulis `Video: h264 ...`: Sangat ideal. Dapat menggunakan mode `copy`.
- Jika tertulis `Video: hevc` (H.265): Kamera memancarkan codec H.265. Karena browser HP belum tentu mendukung WebRTC H.265, nanti di konfigurasi channel harus disetel `live_mode: "transcode"`.

### Langkah 3 — Konfigurasi & Jalankan `local-app`
Di file `.env` Mini PC:
```dotenv
ENGINE_ACCESS_MODE=lan
HTTP_ADDR=0.0.0.0:8090
DATA_DIR=./data
RUNTIME_DIR=./runtime
ALLOWED_ORIGINS=http://localhost:3000,http://127.0.0.1:8090,http://192.168.1.100:8090
MEDIA_READ_CIDRS=127.0.0.1/32,192.168.1.0/24
```
Jalankan aplikasi:
```powershell
go run ./cmd/api -env .env
```

### Langkah 4 — Daftarkan Kamera (Setup di Loopback)
Buka browser/Insomnia di dalam **Mini PC itu sendiri** (`http://127.0.0.1:8090`):
1. **Ambil Nonce**: Panggil `GET /api/v1/setup/bootstrap`. Simpan string `nonce` yang didapat.
2. **Kirim Probe**: Panggil `POST /api/v1/setup/jobs` dengan header `X-Local-Control: <nonce>`:
   ```json
   {
     "action": "probe",
     "channel": {
       "id": "gantangan01",
       "source": "rtsp://admin:password_cctv@192.168.1.64:554/Streaming/Channels/101",
       "live_mode": "copy"
     }
   }
   ```
   Poll status job hingga `SUCCEEDED`.
3. **Simpan (Apply)**: Kirim job `action: "apply"` berisi IP LAN Mini PC (`192.168.1.100`), CIDR (`192.168.1.0/24`), dan channel di atas. Backend akan mengunci setelan ini ke file terenkripsi DPAPI.

### Langkah 5 — Mulai Sesi Lomba (Start Session)
Kamera tidak akan disedot dan tidak direkam sebelum tombol start ditekan:
```http
POST /api/v1/media/sessions
X-Local-Control: <nonce>

{
  "id": "ronde-01",
  "channels": ["gantangan01"]
}
```
Detik ini juga:
1. FFmpeg langsung menyalakan pipeline untuk menyedot RTSP kamera `192.168.1.64`.
2. Segmen file `.mp4` mulai ditulis ke folder `data/recordings/`.
3. Stream video diumpankan ke MediaMTX internal.

### Langkah 6 — Buka Video di Browser HP Juri
Dari HP/Laptop juri yang terhubung ke Wi-Fi arena:
1. Buka player WebRTC (atau file contoh `test-whep.html`) yang mengarah ke:
   `http://192.168.1.100:8090/api/v1/playback/gantangan01/whep`
2. Browser dan Mini PC melakukan jabat tangan WHEP.
3. Video siaran gantangan langsung menyala di layar HP juri tanpa delay.

### Langkah 7 — Selesai Ronde (Stop Session)
Kirim perintah Stop:
```http
POST /api/v1/media/sessions/ronde-01/stop
X-Local-Control: <nonce>
```
1. Sinyal WebRTC ke HP juri diputus.
2. FFmpeg berhenti menyedot RTSP.
3. Segmen-segmen video disatukan, divalidasi dengan `ffprobe`, dan diberi status `READY` di `data/recordings/`. File MP4 utuh siap diputar ulang untuk barang bukti bila ada protes nilai juri.

---

## 5. Ringkasan Diagnostik Masalah di Lapangan

| Gejala di Lapangan | Kemungkinan Penyebab Utama | Solusi Praktis |
| :--- | :--- | :--- |
| **Probe gagal / Timeout** | Kabel LAN kamera lepas, IP kamera salah, atau akun CCTV salah. | Coba tes dengan `ffprobe` manual di terminal. Pastikan IP kamera bisa di-ping dari Mini PC. |
| **Video tidak muncul di HP Juri (WHEP Error)** | Port UDP `8189` diblokir oleh Windows Firewall Mini PC. | Buka Windows Defender Firewall → Inbound Rules → Izinkan port **TCP 8090** dan **UDP 8189**. |
| **Video hitam tapi suara ada / sebaliknya** | Codec kamera adalah H.265 / HEVC tanpa dukungan hardware di HP juri. | Saat setup, ubah `live_mode` dari `"copy"` menjadi `"transcode"`. |
| **Tombol Start ditolak (409 Conflict)** | Sesi ronde sebelumnya belum di-stop atau masih tahap finalisasi file. | Tunggu beberapa detik sampai status sesi lama menjadi `STOPPED`, atau gunakan `session_id` baru. |
| **Request setup ditolak (403 Forbidden)** | Mengakses URL setup via IP LAN (bukan localhost) atau nonce salah. | Setup hanya boleh dibuka dari `http://127.0.0.1:8090` pada keyboard/layar Mini PC itu sendiri. |
