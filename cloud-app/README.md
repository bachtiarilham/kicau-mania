# Kicau Mania Cloud — schema 011

API Go/Gin untuk alur lomba 1–12. Routing aktif memakai `internal/login`, `internal/workflow`, dan `internal/media`; SQL tetap berada di `db/queries`. PostgreSQL schema 011 diperlukan. Aplikasi tidak menjalankan migrasi, seed, atau reset saat startup. Modul lama masih tersimpan sebagai kode historis dan tidak dipasang pada router.

## Konfigurasi dan menjalankan

Salin `.env.example`, lalu isi koneksi dan origin:

```dotenv
DATABASE_URL=postgres://cloud_service:URL_ENCODED_PASSWORD@127.0.0.1:5432/kicau_mania?sslmode=disable
REDIS_URL=redis://127.0.0.1:6379/0
FRONTEND_PUBLIC_URL=http://localhost:5173
ALLOWED_ORIGINS=http://localhost:5173
```

Gunakan PostgreSQL role layanan yang mewarisi `kicau_cloud_app`, bukan superuser. Untuk koneksi remote gunakan TLS PostgreSQL yang terverifikasi dan `rediss://` sesuai deployment. Docker Compose contoh menyediakan Redis pada jaringan internal tanpa port publik. Tidak ada fallback token ketika Redis gagal.

```powershell
go run ./cmd/api -env .env
```

`/healthz` memeriksa proses; `/readyz` memeriksa PostgreSQL/schema/role dan Redis. Konfigurasi/koneksi belum tersedia menghasilkan 503. Refresh/logout memerlukan cookie backend dan Origin yang cocok tepat dengan `ALLOWED_ORIGINS`.

## ADMIN pertama

Jika belum ada ADMIN global, isi `BOOTSTRAP_ADMIN_NAME`, `BOOTSTRAP_ADMIN_EMAIL`, `BOOTSTRAP_ADMIN_PHONE` dan `BOOTSTRAP_ADMIN_PASSWORD` pada environment proses, lalu jalankan:

```powershell
$env:BOOTSTRAP_ADMIN_PASSWORD = Read-Host 'Password ADMIN (minimal 12 karakter)' -MaskInput
go run ./cmd/bootstrap-admin -env .env
Remove-Item Env:BOOTSTRAP_ADMIN_PASSWORD
```

CLI membuat akun email terverifikasi dengan password Argon2id, role ADMIN, serta context history dalam transaksi. Bootstrap menolak bila ADMIN global sudah ada. ADMIN kemudian membuat arena; hak ADMIN arena diberikan otomatis kepada pembuatnya. Hak itu tidak membuka arena lain. Tidak ada akun demo atau seed otomatis.

## Alur dan kontrak

Lihat [API aktif](docs/API.md), [koleksi dan panduan Insomnia](docs/insomnia/README.md), [alur bisnis](docs/ALUR-BISNIS.md), [panduan database](db/README.md), dan [verifikasi](docs/VERIFIKASI-BACKEND.md). Swagger `/api/docs/` dibuat dari route yang benar-benar dipasang:

```powershell
go run ./cmd/openapi docs/generated/swagger.json
```

Access token opaque berlaku 15 menit; refresh berotasi atomik di Redis dan dibatasi umur sesi PostgreSQL maksimal 30 hari. SHA-256 token, TTL dan referensi sesi disimpan di Redis. Setiap autentikasi memeriksa pencabutan sesi dan status akun pada PostgreSQL. Maksimal tiga perangkat aktif, dan login ulang pada perangkat yang sama mengganti pasangan token. Password awal petugas hanya dapat ditetapkan ADMIN setelah role arena ditugaskan. Akun peserta baru tidak memiliki password/login otomatis. Ganti password mencabut semua sesi.

## Pemeriksaan

```powershell
go test ./...
$env:KICAU_TEST_POSTGRES_URL='postgres://postgres@127.0.0.1:55439/postgres?sslmode=disable'
$env:KICAU_TEST_REDIS_URL='redis://127.0.0.1:56379/15'
# Opsional: uji service/Zod frontend langsung selama server tes hidup.
$env:KICAU_TEST_FRONTEND_DIR='D:/ilham/workdir/sveltekit/kicau-mania-web'
go test ./internal/app -run TestSchema010BusinessFlows -v
```

Kedua URL pengujian harus menunjuk layanan khusus tes. Tes membuat database bernama unik `kicau_test_*`, memuat schema 011, menjalankan named SQL dan HTTP, lalu menghapus database tes tersebut. Tes tidak membaca `DATABASE_URL` atau `.env` operasional. Tanpa dua environment tes, integrasi dilewati dengan alasan eksplisit.

Tidak mencakup media recording cloud/ingest, email otomatis, Google login, pendaftaran mandiri, bracket/pemenang tersimpan, atau rollback history otomatis. Dashboard kamera tetap menggunakan local-app tersendiri.


## Media desktop

Desktop baru berada pada [wails/kicau-mania-desktop](../D:/ilham/workdir/wails/kicau-mania-desktop/README.md). Migrasi 011 menambah pairing engine, mapping sesi/tiket dan aset rekaman multipart tanpa menimpa tabel lama atau mereset data. Terapkan satu file migrasi ini secara eksplisit mengikuti prosedur deployment sebelum menjalankan release baru. Konfigurasi MEDIA_S3_* hanya berada di cloud. Lihat [kontrak media](../wails/kicau-mania-desktop/docs/cloud-media-contract.md) untuk endpoint pengguna dan credential engine yang berbeda.
