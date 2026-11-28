# Feature Map: Perekaman Sesi & Journaling Lokal

Dokumen ini mendefinisikan peta fitur untuk memulai perekaman multi-kamera NVR, manajemen file journal lokal, dan penghentian perekaman sesi.

## Alur bisnis target upload rekaman

1. Saat ronde selesai, Local App memasukkan video yang siap ke antrean upload dan mengunggahnya otomatis.
2. Saat operator menekan **START** untuk ronde berikutnya, Local App menjeda transfer yang sedang berlangsung dan menahan transfer baru. Antrean lanjut otomatis saat ronde berhenti. Upload hanya aktif saat tidak ada ronde yang berjalan.
3. Setelah upload berhasil, Local App memperbarui status dan mempertahankan file asli di penyimpanan lokal.
4. Cloud App memicu pengiriman tautan WhatsApp dan melayani unduhan peserta. Video disimpan di Cloud selama lima hari sejak upload berhasil. Penghapusan di Cloud tidak memengaruhi file lokal.

---

## Fitur 3.1: Memulai Perekaman Multi-Channel Sesi Lomba

### 1. Nama & Fungsi Fitur
- **Nama Fitur:** Start Session Multi-Channel Recording
- **Fungsi:** Menginstruksikan `ffmpeg` lokal untuk merekam aliran video dari 24 stream RTSP sekaligus ke direktori lokal tanpa transcode berat (*copy stream mode*) serta mencatat metadata sesi ke file journal JSON.

### 2. Cara User Mengakses Fitur
- Dipicu saat tombol **Mulai Ronde** pada Konsol Operator Desktop diklik.
- Mengirim HTTP POST ke `/api/v1/media/sessions/start`.

### 3. DOM Element / Selector / Endpoint yang Digunakan
| Parameter | Selector / Key | Tipe | Keterangan |
| :--- | :--- | :--- | :--- |
| Method & Path | `POST /api/v1/media/sessions/start` | HTTP Endpoint | Guard: `LocalControl` / `Operator` |
| Request Header | `X-Local-Control-Token: <token>` | Header | Token kontrol lokal |
| Request Body | `session_name` | String | Nama sesi (misal `Sesi_1_Murai_VIP`) |
| Request Body | `round_id` | String | ID ronde dari cloud |
| Request Body | `duration_seconds`| Integer | Maksimal batas aman durasi (misal `600`) |

### 4. Interaksi yang Harus Dilakukan
```http
POST /api/v1/media/sessions/start HTTP/1.1
Host: 127.0.0.1:8090
X-Local-Control-Token: local-secret-token
Content-Type: application/json

{
  "session_name": "Piala_Raja_Sesi_1",
  "round_id": "d300c037-8e68-45c1-9d58-cfab21d30f50",
  "duration_seconds": 600
}
```

### 5. Hasil yang Diharapkan
- **Status Code:** `200 OK`
- **Response Payload:**
  ```json
  {
    "success": true,
    "message": "Session recording started",
    "data": {
      "session_id": "sess-20261124-001",
      "status": "RECORDING",
      "started_at": "2026-11-24T13:00:00Z",
      "active_channels": 24
    }
  }
  ```
- Sub-proses `ffmpeg.exe` aktif di latar belakang dan menulis segmen video ke `D:\KicauRecordings\Piala_Raja_Sesi_1\`.

---

## Fitur 3.2: Menghentikan & Finalisasi Rekaman Sesi

### 1. Nama & Fungsi Fitur
- **Nama Fitur:** Stop Session Recording & Finalize Journal
- **Fungsi:** Mengirim sinyal `SIGINT` / EOF ke sub-proses `ffmpeg`, memfinalisasi file rekaman MP4 agar tidak *corrupt*, dan memperbarui status journal lokal menjadi `FINISHED`.

### 2. Cara User Mengakses Fitur
- Dipicu saat tombol **Selesai / Simpan Rekaman** pada Konsol Operator Desktop diklik.
- Mengirim HTTP POST ke `/api/v1/media/sessions/stop`.

### 3. DOM Element / Selector / Endpoint yang Digunakan
| Parameter | Selector / Key | Tipe | Keterangan |
| :--- | :--- | :--- | :--- |
| Method & Path | `POST /api/v1/media/sessions/stop` | HTTP Endpoint | Guard: `LocalControl` / `Operator` |
| Request Header | `X-Local-Control-Token: <token>` | Header | Token kontrol lokal |

### 4. Interaksi yang Harus Dilakukan
```http
POST /api/v1/media/sessions/stop HTTP/1.1
Host: 127.0.0.1:8090
X-Local-Control-Token: local-secret-token
```

### 5. Hasil yang Diharapkan
- **Status Code:** `200 OK`
- **Response Payload:**
  ```json
  {
    "success": true,
    "message": "Session recording stopped successfully",
    "data": {
      "session_id": "sess-20261124-001",
      "status": "FINISHED",
      "recorded_files": [
        "D:\\KicauRecordings\\Piala_Raja_Sesi_1\\gantangan01.mp4",
        "D:\\KicauRecordings\\Piala_Raja_Sesi_1\\gantangan02.mp4"
      ]
    }
  }
  ```
