# Feature Map: Katalog Kamera & Saluran RTSP

Dokumen ini mendefinisikan peta fitur untuk penemuan dan pemutaran saluran kamera gantangan berbasis protokol RTSP dan audio sinkron.

---

## Fitur 2.1: Mengambil Katalog Saluran Kamera (`/api/v1/channels`)

### 1. Nama & Fungsi Fitur
- **Nama Fitur:** Playable Channels Catalog
- **Fungsi:** Mengembalikan daftar saluran kamera gantangan yang aktif, URL stream RTSP lokal (`rtsp://127.0.0.1:8554/...`), dan rute audio sinkron untuk masing-masing nomor gantangan.

### 2. Cara User Mengakses Fitur
- Melalui HTTP GET request ke `/api/v1/channels`.
- Diakses langsung oleh komponen HUD Juri dan Grid Kamera di desktop atau tablet arena.

### 3. Keyboard Shortcut
- `None (API Endpoint)`

### 4. DOM Element / Selector / Endpoint yang Digunakan
| Parameter | Selector / Key | Tipe | Keterangan |
| :--- | :--- | :--- | :--- |
| Method & Path | `GET /api/v1/channels` | HTTP Endpoint | Mode LAN bebas token dalam subnet |
| Response Item `id` | `id` | String | ID Gantangan (misal `gantangan01`) |
| Response Item `rtsp_url` | `rtsp_url` | String | URL RTSP dari MediaMTX lokal |
| Response Item `audio_path`| `audio_path` | String | Path endpoint streaming audio |

### 5. Interaksi yang Harus Dilakukan
```http
GET /api/v1/channels HTTP/1.1
Host: 127.0.0.1:8090
Accept: application/json
```

### 6. Hasil yang Diharapkan
- **Status Code:** `200 OK`
- **Response Payload:**
  ```json
  {
    "success": true,
    "message": "Channels retrieved",
    "data": [
      {
        "id": "gantangan01",
        "rtsp_url": "rtsp://127.0.0.1:8554/gantangan01",
        "audio_path": "/api/v1/channels/gantangan01/audio"
      },
      {
        "id": "gantangan02",
        "rtsp_url": "rtsp://127.0.0.1:8554/gantangan02",
        "audio_path": "/api/v1/channels/gantangan02/audio"
      }
    ]
  }
  ```
