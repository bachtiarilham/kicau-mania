# Feature Map: Identitas, Autentikasi & Profil Pengguna

Dokumen ini mendefinisikan peta fitur untuk autentikasi kredensial, penerbitan token sesi, pembatasan rate limiter, dan manajemen profil pengguna.

---

## Fitur 1.1: Login Kredensial & Pengambilan Sesi

### 1. Nama & Fungsi Fitur
- **Nama Fitur:** User Credential Login & Session Issuance
- **Fungsi:** Mengotentikasi pengguna berdasarkan email dan password (hashed bcrypt), memeriksa role pengguna (`ADMIN`, `OPERATOR`, `JUDGE`), dan menerbitkan token sesi JWT atau HttpOnly Cookie.

### 2. Cara User Mengakses Fitur
- Melalui HTTP POST request ke endpoint publik `/api/v1/auth/login`.

### 3. Keyboard Shortcut
- `None (API Endpoint)`

### 4. DOM Element / Selector / Endpoint yang Digunakan
| Parameter | Selector / Key | Tipe | Keterangan |
| :--- | :--- | :--- | :--- |
| Method & Path | `POST /api/v1/auth/login` | HTTP Endpoint | Endpoint publik tanpa auth |
| Header Content-Type | `Content-Type: application/json` | Header | Wajib JSON |
| Payload Email | `email` | String (JSON) | Email pengguna terdaftar |
| Payload Password | `password` | String (JSON) | Password akun plaintext |

### 5. Interaksi yang Harus Dilakukan
Kirim HTTP Request:
```http
POST /api/v1/auth/login HTTP/1.1
Host: localhost:8080
Content-Type: application/json

{
  "email": "operator@kicaumania.com",
  "password": "operator123"
}
```

### 6. Hasil yang Diharapkan
- **Status Code:** `200 OK`
- **Response Payload:**
  ```json
  {
    "success": true,
    "message": "success",
    "data": {
      "user_id": "909fb1d3-413e-497b-a0ea-b8501bdf39aa",
      "email": "operator@kicaumania.com",
      "roles": ["OPERATOR"],
      "arena_roles": [
        { "arena_id": "b188c037-8e68-45c1-9d58-cfab21d30f36", "role": "OPERATOR" }
      ],
      "token": "eyJhbGciOi..."
    }
  }
  ```
- **Error Cases:**
  - Kredensial salah: `401 Unauthorized` (`{"success":false,"message":"authentication required"}`).
  - Brute-force terdeteksi: `429 Too Many Requests` (`{"success":false,"message":"too many requests"}`).

---

## Fitur 1.2: Membaca Profil Pengguna Aktif (Self Profile)

### 1. Nama & Fungsi Fitur
- **Nama Fitur:** Current User Profile Discovery (`/users/me`)
- **Fungsi:** Mengambil data identitas, nama tampilan, dan hak akses arena untuk token pengguna yang sedang aktif.

### 2. Cara User Mengakses Fitur
- Melalui HTTP GET request ke `/api/v1/users/me`.

### 3. DOM Element / Selector / Endpoint yang Digunakan
| Parameter | Selector / Key | Tipe | Keterangan |
| :--- | :--- | :--- | :--- |
| Method & Path | `GET /api/v1/users/me` | HTTP Endpoint | Tier 2 Protected |
| Header Auth | `Authorization: Bearer <token>` | Header | Token JWT hasil login |

### 4. Interaksi yang Harus Dilakukan
```http
GET /api/v1/users/me HTTP/1.1
Host: localhost:8080
Authorization: Bearer <token>
```

### 5. Hasil yang Diharapkan
- **Status Code:** `200 OK`
- **Response Payload:**
  ```json
  {
    "success": true,
    "message": "success",
    "data": {
      "id": "909fb1d3-413e-497b-a0ea-b8501bdf39aa",
      "email": "operator@kicaumania.com",
      "name": "Operator Arena 1",
      "status": "ACTIVE"
    }
  }
  ```
