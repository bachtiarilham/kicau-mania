# Feature Map: Live Streaming & Restreaming RTMP

Dokumen ini mendefinisikan peta fitur untuk push live stream RTMP ke platform eksternal (YouTube Live, Facebook Live, atau Cloud CDN) serta pemantauan bitrate dan status transmisi.

---

## Fitur 4.1: Memulai Restreaming Live ke YouTube / Cloud

### 1. Nama & Fungsi Fitur
- **Nama Fitur:** Start RTMP Restreaming Relay
- **Fungsi:** Mengambil stream gabungan arena atau stream gantangan terpilih dari MediaMTX lokal dan meneruskannya (*push relay*) ke server RTMP YouTube Live atau Cloud Media Ingestion.

### 2. Cara User Mengakses Fitur
- Melalui HTTP POST request ke `/api/v1/streaming/start`.
- Dapat dipicu dari tab Operator atau setting streaming desktop.

### 3. DOM Element / Selector / Endpoint yang Digunakan
| Parameter | Selector / Key | Tipe | Keterangan |
| :--- | :--- | :--- | :--- |
| Method & Path | `POST /api/v1/streaming/start` | HTTP Endpoint | Guard: `LocalControl` / `Operator` |
| Request Body | `target_url` | String | URL RTMP tujuan (misal `rtmp://a.rtmp.youtube.com/live2`) |
| Request Body | `stream_key` | String | Stream key rahasia YouTube |
| Request Body | `source_channel` | String | ID Channel sumber (misal `gantangan_all_grid` atau `gantangan01`) |

### 4. Interaksi yang Harus Dilakukan
```http
POST /api/v1/streaming/start HTTP/1.1
Host: 127.0.0.1:8090
X-Local-Control-Token: local-secret-token
Content-Type: application/json

{
  "target_url": "rtmp://a.rtmp.youtube.com/live2",
  "stream_key": "abcd-1234-efgh-5678",
  "source_channel": "gantangan_all_grid"
}
```

### 5. Hasil yang Diharapkan
- **Status Code:** `200 OK`
- **Response Payload:**
  ```json
  {
    "success": true,
    "message": "Streaming started",
    "data": {
      "stream_id": "stream-yt-001",
      "status": "LIVE",
      "destination": "rtmp://a.rtmp.youtube.com/live2/[Redacted]",
      "bitrate_kbps": 4500
    }
  }
  ```
- Aliran video muncul secara live di studio YouTube dalam waktu 3–5 detik.
- Kredensial `stream_key` disensor secara otomatis oleh `RedactCore` di seluruh log server.

---

## Fitur 4.2: Pemantauan Status Kesehatan Live Stream

### 1. Nama & Fungsi Fitur
- **Nama Fitur:** Streaming Telemetry & Health Probe
- **Fungsi:** Memeriksa apakah stream masih aktif, jumlah dropped frames, fps aktual, dan bitrate output saat ini.

### 2. Cara User Mengakses Fitur
- Melalui HTTP GET request ke `/api/v1/streaming/status`.

### 3. DOM Element / Selector / Endpoint yang Digunakan
| Parameter | Selector / Key | Tipe | Keterangan |
| :--- | :--- | :--- | :--- |
| Method & Path | `GET /api/v1/streaming/status` | HTTP Endpoint | Guard: `LocalControl` / `LANViewer` |

### 4. Interaksi yang Harus Dilakukan
```http
GET /api/v1/streaming/status HTTP/1.1
Host: 127.0.0.1:8090
Accept: application/json
```

### 5. Hasil yang Diharapkan
- **Status Code:** `200 OK`
- **Response Payload:**
  ```json
  {
    "success": true,
    "message": "Streaming status",
    "data": {
      "is_live": true,
      "fps": 30,
      "bitrate_kbps": 4480,
      "dropped_frames": 0,
      "uptime_seconds": 320
    }
  }
  ```
