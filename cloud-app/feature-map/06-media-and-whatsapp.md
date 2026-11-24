# Feature Map: Media S3, Link Publik & Distribusi WhatsApp

Dokumen ini mendefinisikan peta fitur untuk penyimpanan video rekaman ke Cloud Object Storage (S3/MinIO), tautan tonton video publik, serta pemrosesan pengiriman pesan WhatsApp ke peserta lomba.

---

## Fitur 6.1: Pengiriman Tautan Video via WhatsApp

### 1. Nama & Fungsi Fitur
- **Nama Fitur:** WhatsApp Video Delivery Endpoint
- **Fungsi:** Mengirimkan pesan WhatsApp berisikan tautan video rekaman kepada pemilik burung untuk gantangan tertentu pada suatu ronde, dengan pengamanan Circuit Breaker agar tidak memblokir server jika gateway WhatsApp pihak ketiga mengalami gangguan.

### 2. Cara User Mengakses Fitur
- Melalui HTTP POST request ke `/api/v1/arenas/:arena/rounds/:id/deliveries/whatsapp`.

### 3. DOM Element / Selector / Endpoint yang Digunakan
| Parameter | Selector / Key | Tipe | Keterangan |
| :--- | :--- | :--- | :--- |
| Method & Path | `POST /api/v1/arenas/:arena/rounds/:id/deliveries/whatsapp` | HTTP Endpoint | `:id` adalah UUID Ronde |
| Path Param | `:arena` | UUID String | ID Arena terkait |
| Path Param | `:id` | UUID String | ID Ronde yang videonya dibagikan |
| Request Body | `registration_id` | UUID String (Opsional) | Kirim hanya ke peserta tertentu |
| Request Body | `custom_message` | String (Opsional) | Pesan tambahan dari panitia |

### 4. Interaksi yang Harus Dilakukan
```http
POST /api/v1/arenas/b188c037-8e68-45c1-9d58-cfab21d30f36/rounds/d300c037-8e68-45c1-9d58-cfab21d30f50/deliveries/whatsapp HTTP/1.1
Host: localhost:8080
Authorization: Bearer <token>
Content-Type: application/json

{
  "registration_id": "e400c037-8e68-45c1-9d58-cfab21d30f60",
  "custom_message": "Selamat atas penampilan gacoan Anda!"
}
```

### 5. Hasil yang Diharapkan
- **Status Code:** `200 OK` (atau `202 Accepted` jika background batch)
- **Response Payload:**
  ```json
  {
    "success": true,
    "message": "success",
    "data": {
      "delivered_count": 1,
      "failed_count": 0,
      "status": "COMPLETED"
    }
  }
  ```
- **Jika Gateway WhatsApp Down (Circuit Breaker Terbuka):**
  - Mengembalikan `503 Service Unavailable`:
    ```json
    {
      "success": false,
      "message": "circuit breaker is open: external service unavailable",
      "errors": { "request_id": "..." }
    }
    ```

---

## Fitur 6.2: Halaman Tonton Video Publik (`/v/:id`)

### 1. Nama & Fungsi Fitur
- **Nama Fitur:** Public Video Player Page
- **Fungsi:** Menyajikan pemutar video HTML5 publik tanpa mewajibkan login agar pemilik burung yang menerima link WhatsApp dapat langsung menonton aksi burungnya di browser HP.

### 2. Cara User Mengakses Fitur
- Pengguna mengklik tautan di WhatsApp: `https://dev.megaallstararena.com/v/:id`.

### 3. DOM Element / Selector yang Digunakan
| Elemen | Selector / ID | Tipe |
| :--- | :--- | :--- |
| Player Container | `video#public-player`, `.video-js` | HTML5 Video Player |
| Judul Event & Sesi | `h1.public-video-title` | Display Title |
| Nomor Gantangan | `span.gantangan-number-badge` | Display Badge |
| Nama Burung & Pemilik | `p.bird-meta` | Display Meta |

### 4. Interaksi yang Harus Dilakukan
1. Peserta membuka link dari WhatsApp di browser Chrome/Safari di smartphone.
2. Klik tombol Play pada pemutar video.

### 5. Hasil yang Diharapkan
- Video terputar secara responsif dengan streaming adaptif langsung dari bucket S3 / CDN.
