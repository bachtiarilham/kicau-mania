# Cloud-app: backend bisnis online

Hubungan kedua aplikasi dijelaskan pada [panduan integrasi proyek](../docs/integration.md); indeks lintas aplikasi ada di [dokumentasi proyek](../docs/README.md).

Backend Go memakai **Gin**, pgx dan PostgreSQL. Tersedia autentikasi email/password serta Google, CRUD persiapan arena, registrasi/pembayaran, penilaian/revisi juri, dan pengesahan pemenang. Operasi penuh memerlukan migrasi database dan konfigurasi layanan terkait.

Mulai dari [alur bisnis dan peta kode](docs/ALUR-BISNIS.md), [kontrak API](docs/API.md), lalu [panduan SQL/migrasi](db/README.md). Hasil pemeriksaan dan batas integrasi dicatat pada [verifikasi backend](docs/VERIFIKASI-BACKEND.md).

## Menjalankan

Dari `cloud-app`, dengan Go 1.25+:

```powershell
Copy-Item .env.example .env
# Isi DATABASE_URL; terapkan migrasi secara manual sesuai db/README.md.
go run ./cmd/api -env .env
```

Default listener `127.0.0.1:8091`, untuk dipasang di belakang reverse proxy HTTPS VPS. Header forwarding hanya dipercaya dari `TRUSTED_PROXIES`. Tidak ada migrasi otomatis atau fallback database lokal.

`GET /healthz` menyatakan proses hidup. `GET /readyz` memeriksa koneksi, struktur read/workflow yang diperlukan, serta role sistem; keduanya juga menerima HEAD. Database belum dikonfigurasi atau migrasi belum lengkap menghasilkan readiness 503. Fitur yang belum tersedia karena konfigurasi Google/SMTP kosong menghasilkan 503, tanpa response sukses tiruan.

## Akun awal

Role sistem disiapkan migrasi, tetapi tidak ada akun/password bawaan. Buat superadmin secara eksplisit menggunakan environment proses:

```powershell
$env:BOOTSTRAP_ADMIN_NAME = 'Nama admin'
$env:BOOTSTRAP_ADMIN_EMAIL = 'email-admin-yang-Anda-miliki'
$env:BOOTSTRAP_ADMIN_PHONE = '+62nomor-telepon'
# Isi BOOTSTRAP_ADMIN_PASSWORD melalui mekanisme rahasia environment Anda.
go run ./cmd/bootstrap-admin -env .env
```

Password tidak diterima melalui argumen CLI dan tidak dicetak. Password wajib 12â€“256 byte; nomor akun memakai format E.164. Bootstrap membuat akun terverifikasi dan menolak email/nomor yang sudah digunakan. Jangan menyimpan nilai rahasia bootstrap dalam source control.

## Konfigurasi

Environment proses mengungguli dotenv, termasuk nilai kosong.

| Env | Fungsi/default |
| --- | --- |
| DATABASE_URL | Koneksi privat PostgreSQL; kosong membuat readiness 503. |
| DB_MAX_CONNS / DB_MIN_CONNS | 10 / 0. |
| DB_CONNECT_TIMEOUT / DB_HEALTH_TIMEOUT | 5s / 2s. |
| HTTP_ADDR | 127.0.0.1:8091. |
| HTTP_REQUEST_TIMEOUT / SHUTDOWN_TIMEOUT | 15s / 10s. |
| ALLOWED_ORIGINS | Origin browser eksplisit; default frontend dev. |
| FRONTEND_PUBLIC_URL | Origin halaman verifikasi/reset; default https://dev.megaallstararena.com. |
| TRUSTED_PROXIES | CIDR reverse proxy; default loopback IPv4/IPv6. |
| GOOGLE_CLIENT_ID | Kosong menonaktifkan jalur Google; login password tetap terpisah. |
| SMTP_HOST / SMTP_PORT / SMTP_FROM | Server, port, alamat pengirim; port default 587. |
| SMTP_USER / SMTP_PASSWORD | Kredensial SMTP privat. |
| SMTP_MODE | starttls atau tls. |
| SMTP_ALLOW_INSECURE | false; true hanya diperbolehkan pada SMTP loopback development. |
| MV_REFRESH_INTERVAL / MV_REFRESH_TIMEOUT | 60s / 2m. |
| APP_ENV / SERVICE_NAME / LOG_DIR | Mode logging Zap, nama layanan, lokasi app.log. |

Untuk frontend pengembangan lokal, ubah **ALLOWED_ORIGINS dan FRONTEND_PUBLIC_URL** menjadi origin loopback HTTP yang sama, misalnya `http://localhost:3000`. Cookie refresh memakai HttpOnly, SameSite=Lax, path `/`, serta Secure dan prefix `__Host-` ketika frontend memakai HTTPS. Domain cookie tidak diperlebar ke seluruh subdomain.

## Batas arsitektur

Handler hanya parsing/sintaks, auth/context, pemanggilan UseCase, DTO dan response. Aturan bisnis serta koordinasi lintas domain berada pada UseCase melalui interface. Repository tidak memanggil repository lain. Goroutine pemeliharaan berada pada worker, bukan UseCase.

SQL berada pada `db/queries/*.sql`, dinamai dengan komentar `-- name:`, dan di-embed ke binary. Tidak ada duplikasi SQL bisnis di Go. GET bisnis memakai view; INSERT/UPDATE/DELETE langsung ke tabel sumber. Pembacaan internal untuk otorisasi, autentikasi dan locking memakai jalur terbatas. Response mutasi membaca view biasa dalam transaksi yang sama.

Materialized view dipakai laporan administratif; rekap aktif dan keputusan pemenang membaca data terbaru. Worker melakukan refresh awal, setiap interval, serta setelah notifikasi commit pembayaran/pembatalan/penutupan. Snapshot dan metadata waktunya dibaca dalam satu statement.

## Kontrak HTTP

Envelope bersama: `{"success":true,"message":"...","data":...}` atau `{"success":false,"message":"...","errors":...}`. Field nil dihilangkan. Pagination: `data.items`, `data.next_cursor` bila ada, dan `data.has_more`; limit 1â€“100, cursor mengikuti ID. Response 204 tidak berisi body.

Header bersama meliputi request ID, nosniff, no-store, dan CORS origin eksplisit. Gin recovery tidak mengirim detail panic; log hanya memakai pola route, bukan body, token atau URL mentah. Internal error memakai pesan aman; error encoding JSON ditangani sebelum header dikirim.

## Batas integrasi dan verifikasi

Belum ada frontend, payment gateway/refund, pengiriman video, maupun endpoint komunikasi engine aktif. Tombol fase ronde saat ini mengubah status bisnis tanpa memulai/menghentikan local-app. Kontrak media tetap tersedia di [ENGINE-CONTRACT.md](docs/ENGINE-CONTRACT.md), untuk fase integrasi selanjutnya. Metadata rekaman hanya dapat dibaca sesuai hak registrasi.

Build dan pemeriksaan SQL/HTTP dilakukan pada PostgreSQL sementara dengan SMTP lokal serta verifier Google fixture. Suite tes Go tidak dijalankan otomatis. Google sign-in dan email produksi tetap memerlukan konfigurasi serta penerimaan dengan layanan sebenarnya.

## Deployment VPS

Pipeline GitHub Actions membangun image AMD64/ARM64, memublikasikan ke GHCR privat dan deploy melalui SSH saat push ke main. Migrasi tetap manual. Ikuti [runbook deployment](deployment/devops_runbook.md) untuk setup VPS, GitHub secrets, HTTPS, bootstrap dan rollback.

API VPS dev sekarang memakai `https://dev.megaallstararena.com/api/v1/`, satu domain dengan frontend. Gateway memakai Nginx Docker dan renewal Certbot Docker; credential registry disimpan di `/opt`. Frontend yang ada masih demo dan belum otomatis terhubung ke backend oleh perpindahan gateway.

Mulai belajar pengelolaan VPS dari [Belajar Deployment](deployment/BELAJAR-DEPLOYMENT.md): konsep, alur deploy kode, migrasi ketika data sudah ada, backup/restore, rollback dan diagnosis gangguan.

Swagger UI tersedia pada `/api/docs` setelah aplikasi dijalankan/deploy. [Panduan API](docs/API.md#swagger-interaktif) menjelaskan login, Authorize `Bearer <token>` dan refresh cookie. Edit anotasi/DTO dekat handler lalu jalankan `go generate .` dari cloud-app. [Swagger JSON](docs/generated/swagger.json) adalah hasil otomatis, bukan file yang diedit manual. UI dan kontrak ikut binary Docker tanpa layanan tambahan.
