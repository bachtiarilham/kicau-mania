# Cloud exchange

Penerima sinkronisasi untuk arena offline-first. Layanan ini menyimpan paket event yang disiapkan, perubahan arena, dan antrean perintah media. Website pendaftaran, CRUD peserta, penyusunan paket dari data bisnis, serta publikasi hasil belum diimplementasikan di sini.

## Jalankan

Go 1.25+, tanpa database eksternal:

```powershell
Copy-Item .env.example .env
# Isi CLOUD_ADMIN_TOKEN dan CLOUD_ENGINES_JSON dengan token acak yang berbeda.
go mod download
go test ./...
go build -o bin/cloud-api.exe ./cmd/api
./bin/cloud-api.exe -env .env
```

Default HTTP loopback `127.0.0.1:8091`; tempatkan reverse proxy HTTPS di depannya untuk akses arena. `CLOUD_DATABASE` menunjuk SSD persisten. Proses memuat `.env`; environment proses mengungguli file. Token minimum 32 karakter, placeholder `REPLACE_` ditolak. Berikan token mesin unik per pasangan `arena_id`/`engine_id`, terpisah dari token operator dan password juri. Jangan taruh database atau `.env` pada direktori publik.

```dotenv
CLOUD_ENGINES_JSON='[{"arena_id":"arena01","engine_id":"engine01","token":"TOKEN_MESIN_UNIK_MINIMAL_32_KARAKTER"}]'
```

## Kontrak

Semua endpoint di bawah memakai `Authorization: Bearer ...`; `/healthz` public. Payload paket dan perintah mengikuti model di `internal/exchange/model` dan file demo dari `local-app/cmd/event-package`.

| Endpoint | Auth | Fungsi |
| --- | --- | --- |
| `POST /api/v1/engines/:engine/changes` | mesin terkait | Terima `{changes:[...]}`, kembalikan `{acknowledged:[id...]}` setelah commit |
| `GET /api/v1/engines/:engine/commands` | mesin terkait | Ambil perintah media |
| `POST /api/v1/engines/:engine/commands/results` | mesin terkait | Simpan hasil eksekusi idempotent |
| `GET /api/v1/engines/:engine/events/:event/package` | mesin terkait | Unduh snapshot privat event yang telah disiapkan |
| `POST /api/v1/operator/packages` | admin | Unggah snapshot JSON dengan versi meningkat |
| `POST /api/v1/operator/commands` | admin | Antrekan perintah media dengan expiry maksimal 15 menit |
| `GET /api/v1/operator/commands/:engine/:command` | admin | Baca status/hasil perintah |

Perintah yang diterima hanya `restream.start`, `restream.stop`, `recording.start`, dan `recording.stop`. Tidak tersedia perintah cloud untuk mengubah skor atau lifecycle ronde penilaian. Contoh payload perintah:

```json
{"id":"stop-stream-001","arena_id":"arena01","engine_id":"engine01","kind":"restream.stop","payload":{"target":"youtube"},"expires_at":"GANTI_DENGAN_RFC3339_MAKSIMAL_15_MENIT_KE_DEPAN"}
```

SQLite menggunakan WAL/FULL. Identitas mesin dicek per request. Pengiriman ulang perubahan yang sama diakui tanpa insert ganda; perubahan isi dengan ID sama ditolak. Paket hanya dapat diunduh engine tujuan. Arena tetap melakukan validasi semantik lengkap terhadap snapshot dan menolak penggantian roster setelah event berjalan.

Data perubahan disimpan sebagai journal; ini belum berupa proyeksi rekap publik atau database CRUD lomba. Integrasi web berikutnya perlu membaca journal melalui repository/usecase dan menerapkan kebijakan publikasi. Cloud tidak mengambil alih otoritas skor arena saat WAN terputus. Backup database cloud dan proteksi reverse proxy dikelola deployment.

Validasi: `go test ./...`, `go build ./...`, `go vet ./...`. Tes end-to-end dua aplikasi ada di `local-app/scripts/tests/cloud-exchange.cjs`; panduan LAN dan skenario offline ada di `local-app/OFFLINE-FIRST.md`.
