# Cloud-app: fondasi backend online

Cloud-app akan menangani login, registrasi, penilaian, CRUD, dan kendali engine arena secara online. Fase sekarang hanya menyediakan fondasi HTTP/PostgreSQL serta [kontrak engine](docs/ENGINE-CONTRACT.md). Endpoint bisnis dan engine belum aktif.

ERD, migrasi, view, materialized view, dan query bisnis **ditahan sampai rancangan database disetujui**. File SQL lama tetap ada sebagai draft, bukan skema siap deploy. Aplikasi tidak menjalankan migrasi otomatis.

## Menjalankan

Prasyarat Go 1.25+. Jalankan dari folder cloud-app:

```powershell
Copy-Item .env.example .env
go run ./cmd/api -env .env
```

Default HTTP `127.0.0.1:8091`; gunakan reverse proxy HTTPS pada VPS. Isi DATABASE_URL untuk PostgreSQL yang ingin diperiksa. URL kosong diperbolehkan pada fase fondasi: proses tetap berjalan dan readiness mengembalikan 503. Database tidak terjangkau juga menghasilkan 503; proses dapat kembali ready saat koneksi pulih tanpa fallback SQLite.

| Endpoint aktif | Fungsi |
| --- | --- |
| GET /healthz | 200 ketika HTTP hidup. |
| GET /readyz | 200 jika PostgreSQL dapat di-ping; 503 jika belum dikonfigurasi/tidak tersedia. |

Readiness ini hanya memeriksa koneksi, belum memeriksa tabel, view, migrasi, atau kesiapan fitur bisnis. Route login/CRUD/engine yang belum dibuat mengembalikan 404. Tidak ada response sukses tiruan.

## Konfigurasi

| Env | Default / fungsi |
| --- | --- |
| HTTP_ADDR | 127.0.0.1:8091 |
| DATABASE_URL | Kosong; URL/connection string PostgreSQL privat. |
| DB_MAX_CONNS / DB_MIN_CONNS | 10 / 0; mengungguli opsi pool dalam DATABASE_URL. |
| DB_CONNECT_TIMEOUT | 5s; batas koneksi database. |
| DB_HEALTH_TIMEOUT | 2s; batas pemeriksaan readiness. |
| HTTP_REQUEST_TIMEOUT | 15s; deadline context request dan batas baca HTTP. |
| SHUTDOWN_TIMEOUT | 10s; waktu menyelesaikan HTTP sebelum koneksi ditutup. |
| ALLOWED_ORIGINS | https://dev.megaallstararena.com; daftar origin eksplisit dipisahkan koma, tanpa slash akhir. |
| APP_ENV | development memakai console; production memakai JSON stdout. |
| SERVICE_NAME / LOG_DIR | cloud-app / ./logs. |

Environment proses mengungguli dotenv, termasuk nilai kosong. Nilai konfigurasi salah menghentikan startup dengan pesan yang tidak menampilkan kredensial. Deadline request harus diteruskan oleh UseCase/repository; ini bukan mekanisme untuk menghentikan goroutine secara paksa.

## Membaca kode

```text
cmd/api/main.go → config → server → app → router → handler → UseCase
```

| Folder | Tanggung jawab |
| --- | --- |
| cmd/api | Entry point, pemuatan konfigurasi, logger, dan sinyal proses. |
| config | Validasi env tanpa membuka koneksi. |
| internal/app | Merakit pool, UseCase readiness, dan handler; menutup dependency. |
| internal/server | Listener HTTP, timeout, dan graceful shutdown. |
| internal/delivery | Router utama yang merangkai handler dan middleware. |
| internal/delivery/middleware | Auth boundary, CORS, request ID, logging, recovery, batas body 2 MiB. |
| internal/delivery/rest | Handler health/readiness dan format response JSON di rest/response. |
| internal/health/usecase | Menentukan readiness melalui interface database, tanpa import HTTP atau adapter. |
| internal/auth/model | Identitas pengguna/engine yang tidak bergantung pada transport; belum ada login. |
| internal/apperror | Error aplikasi dan pemetaan aman melalui delivery/rest/response. |
| internal/infra/postgres | Connection pool pgx; tanpa SQL bisnis atau migrasi. |
| internal/infra/logger | Pola Zap seperti local-app/park-server-v3, file JSON dan redaksi rahasia. |
| docs | Kontrak transport yang belum diaktifkan. |
| db | ERD dan SQL draft yang dibekukan. |

Handler hanya parsing, validasi sintaks, ekstraksi auth/context, pemanggilan UseCase, pemetaan DTO, dan HTTP response. Validasi semantik/otorisasi bisnis serta koordinasi lintas domain berada di UseCase. Repository tidak memanggil repository lain. Model identitas memungkinkan UseCase autentikasi kelak memenuhi interface middleware tanpa mengimpor HTTP.

## Aturan repository setelah ERD disetujui

Response REST mengikuti format aktual park-server-v3: `{"success":true,"message":"...","data":{...}}` dan `{"success":false,"message":"...","errors":...}`. Data/errors dihilangkan jika nil; tidak ada pembungkus meta. Error internal tidak dikirim mentah dan dapat menyertakan `errors.request_id`. Response health/readiness juga memakai format tersebut. Pagination cursor memakai `data.items`, `data.next_cursor` (jika ada), dan `data.has_more`. Response 204 tidak memiliki body. Kontrak response engine juga memakai envelope ini dengan data non-null; request engine tetap berupa payload langsung. Endpoint pendamping tetap belum aktif sampai autentikasi dan repository tersedia.

- List/detail bisnis membaca view; create/update/delete menulis tabel sumber dalam transaksi.
- Mutasi menggunakan RETURNING id lalu membaca representasi view dalam transaksi yang sama.
- Pembacaan internal untuk transaksi/locking dan autentikasi sensitif memakai jalur terbatas yang sesuai; hash password tidak masuk view operasional.
- Materialized view untuk rekap administratif yang menerima keterlambatan refresh, bukan nilai operasional yang wajib langsung terbaru.

SQL draft tidak otomatis memenuhi aturan multi-arena terbaru. Implementasi repository serta query menunggu review ERD.

## Penghapusan legacy dan batas fitur

Exchange SQLite, paket event offline, penerima sinkronisasi skor, endpoint exchange lama, serta token admin/registry mesin berbasis env lama telah dihapus. File database lama tidak dibaca dan tidak dihapus otomatis. Arsipkan data lama melalui prosedur operasional sebelum memutuskan penghapusannya.

Boundary autentikasi pengguna dan mesin tersedia sebagai middleware terpisah, tetapi belum dipasang pada route bisnis karena verifier dan repository belum dibuat. Tidak ada akun, token, maupun akses arena default.

Kontrak command, heartbeat, metadata, tiket playback, dan otorisasi engine ada di [ENGINE-CONTRACT.md](docs/ENGINE-CONTRACT.md). Frontend LAN belum dapat diintegrasikan end-to-end sebelum endpoint tersebut diimplementasikan.

## Logging dan verifikasi

Header umum dikelola middleware bersama: `X-Content-Type-Options: nosniff`, `Cache-Control: no-store`, dan request ID. CORS memeriksa origin serta menangani preflight sebelum handler, dengan X-Request-ID diekspos ke frontend yang diizinkan. Limits hanya membatasi body/deadline; auth hanya menangani autentikasi. Content-Type JSON tetap dimiliki helper response. Kebijakan ini berlaku juga pada response error dan route yang tidak ditemukan.

Log mencakup request ID, route template, method, status, latency, service, hostname, serta caller. Body, query, Authorization, dan raw path tidak dicatat oleh access logger. File `${LOG_DIR}/app.log` memakai JSON; rotasi/retensi dikelola deployment. Kegagalan membuka file log tetap memungkinkan stdout.

Verifikasi fase ini: `go build ./...`, pemeriksaan dependency/referensi legacy, dan diff. Suite tes tidak dijalankan otomatis sesuai permintaan. Uji integrasi database, auth, CRUD, dan engine dilakukan pada fase berikutnya. Petunjuk SQL lama tersedia sebagai draft di [db/README.md](db/README.md).
