# Feature Map: Event Lomba & Penjadwalan Ronde

Dokumen ini mendefinisikan peta fitur untuk pembuatan event perlombaan, penjadwalan ronde/sesi, dan pengaturan durasi shift rotasi juri.

---

## Fitur 3.1: Membuat Event Perlombaan Baru

### 1. Nama & Fungsi Fitur
- **Nama Fitur:** Create Tournament Event
- **Fungsi:** Mendaftarkan event lomba kicau mania baru di bawah kepemilikan suatu arena dengan tanggal, lokasi, grand prize, dan PIC lomba.

### 2. Cara User Mengakses Fitur
- Melalui HTTP POST request ke `/api/v1/arenas/:arena/events`.

### 3. DOM Element / Selector / Endpoint yang Digunakan
| Parameter | Selector / Key | Tipe | Keterangan |
| :--- | :--- | :--- | :--- |
| Method & Path | `POST /api/v1/arenas/:arena/events` | HTTP Endpoint | `:arena` adalah UUID Arena |
| Path Param | `:arena` | UUID String | ID Arena penyelenggara |
| Request Body | `title` | String | Judul event (misal "Piala Raja 2026") |
| Request Body | `event_date` | Date String (`YYYY-MM-DD`) | Tanggal pelaksanaan |
| Request Body | `location` | String | Lokasi gantangan fisik |
| Request Body | `grand_prize` | String | Informasi hadiah utama |
| Request Body | `pic_name` | String | Nama penanggung jawab lapangan |

### 4. Interaksi yang Harus Dilakukan
```http
POST /api/v1/arenas/b188c037-8e68-45c1-9d58-cfab21d30f36/events HTTP/1.1
Host: localhost:8080
Authorization: Bearer <token>
Content-Type: application/json

{
  "title": "Piala Umi Kasum Champion 2026",
  "event_date": "2026-12-01",
  "location": "Arena Utama Jakarta",
  "grand_prize": "1 Unit Mobil",
  "pic_name": "Budi Santoso",
  "min_participants": 24,
  "max_participants": 72
}
```

### 5. Hasil yang Diharapkan
- **Status Code:** `200 OK` (atau `201 Created`)
- **Response Payload:** Mengembalikan object `EventView` lengkap dengan UUID event yang baru dibuat dan status `DRAFT` / `PUBLISHED`.

---

## Fitur 3.2: Penjadwalan Ronde & Multi-Shift Timer

### 1. Nama & Fungsi Fitur
- **Nama Fitur:** Create & Configure Round with Shift Timers
- **Fungsi:** Menjadwalkan ronde lomba di dalam event tertentu, menetapkan kategori burung (misal Murai Batu Ring), durasi total ronde (misal 600 detik), dan durasi per-shift juri (misal 150 detik).

### 2. Cara User Mengakses Fitur
- Melalui HTTP POST request ke `/api/v1/arenas/:arena/rounds`.

### 3. DOM Element / Selector / Endpoint yang Digunakan
| Parameter | Selector / Key | Tipe | Keterangan |
| :--- | :--- | :--- | :--- |
| Method & Path | `POST /api/v1/arenas/:arena/rounds` | HTTP Endpoint | Tier 2 Protected |
| Request Body | `event_id` | UUID String | ID Event induk |
| Request Body | `category_id` | UUID String | Kategori jenis burung |
| Request Body | `round_number` | Integer | Nomor urut sesi (misal `1`) |
| Request Body | `total_duration_sec` | Integer | Total durasi ronde dalam detik (`600`) |
| Request Body | `shift_duration_sec` | Integer | Durasi per shift dalam detik (`150`) |

### 4. Interaksi yang Harus Dilakukan
```http
POST /api/v1/arenas/b188c037-8e68-45c1-9d58-cfab21d30f36/rounds HTTP/1.1
Host: localhost:8080
Authorization: Bearer <token>
Content-Type: application/json

{
  "event_id": "c200c037-8e68-45c1-9d58-cfab21d30f40",
  "name": "Sesi 1 - Murai Batu VIP",
  "round_number": 1,
  "total_duration_sec": 600,
  "shift_duration_sec": 150
}
```

### 5. Hasil yang Diharapkan
- **Status Code:** `200 OK`
- **Response Payload:** Mengembalikan detail ronde dengan status awal `PENDING`, siap menerima registrasi peserta dan penugasan juri.
