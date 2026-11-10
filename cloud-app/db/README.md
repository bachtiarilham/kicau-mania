# Database dan SQL cloud-app

Skema bisnis mengikuti [ERD](ERD.md). SQL adalah satu sumber yang di-embed ke binary Go melalui package `db/queries`; aplikasi tidak menjalankan migrasi otomatis. View hanya untuk pembacaan, sedangkan create/update/delete menulis tabel sumber.

## Urutan migrasi

Baseline adalah `db-dump/full_db.dump` PostgreSQL 18, dengan 16 tabel kosong saat diperiksa.

| Versi | Isi |
| --- | --- |
| 001–003 | Riwayat read layer lama, tetap dipertahankan. |
| 004 | Mengganti struktur kosong menjadi multi-arena dan menonaktifkan read layer lama. Menolak jika baseline berisi data. |
| 005 | Pemenang tunggal, pengesah, dan waktu pengesahan ronde. |
| 006 | Verifikasi/reset email, sesi access/refresh, Google-only password nullable, provenance pemilik manual, penguncian kriteria, induk penilaian dan ledger operasi. Menolak skor lama yang belum dikonversi ke model assessment. |
| 007 | View baru, dua materialized view, metadata refresh dan privilege aplikasi/pemelihara. |

Jika database sudah sampai versi tertentu, jalankan hanya versi berikutnya. Jangan mengulang file yang sudah diterapkan. Belum ada migration runner atau ledger otomatis; catat versi pada operasional deployment.

Contoh PowerShell dari root repository, untuk database pengembangan baru:

```powershell
$pgBin = 'C:/Program Files/PostgreSQL/18/bin'
$dbName = 'kicau_schema_' + (Get-Date -Format 'yyyyMMddHHmmss')
$pgConn = @('-h', '127.0.0.1', '-p', '5432', '-U', 'postgres')
& "$pgBin/createdb.exe" @pgConn $dbName
if ($LASTEXITCODE -ne 0) { throw 'Database tidak berhasil dibuat' }
& "$pgBin/pg_restore.exe" @pgConn --exit-on-error --no-owner --no-privileges -d $dbName db-dump/full_db.dump
if ($LASTEXITCODE -ne 0) { throw 'Restore gagal' }
foreach ($migration in (Get-ChildItem cloud-app/db/migrations/*.up.sql | Sort-Object Name)) {
    & "$pgBin/psql.exe" -X @pgConn -d $dbName -v ON_ERROR_STOP=1 -1 -f $migration.FullName
    if ($LASTEXITCODE -ne 0) { throw "Migrasi gagal: $($migration.Name)" }
}
```

Wajib gunakan transaksi per file (`psql -1`) dan hentikan penulisan aplikasi selama migrasi. Gunakan pemilik objek dengan wewenang membuat role grup NOLOGIN bila belum tersedia. Tidak ada arena, kategori, pemilik, pemenang, akun atau password fiktif yang diisikan. Role sistem merupakan data konfigurasi; akun superadmin dibuat melalui bootstrap eksplisit.

## View, tabel, dan pengecualian internal

| Jalur | Sumber |
| --- | --- |
| GET list/detail bisnis | `cloud_read.v_*`, memakai scope aktor/arena. |
| Response setelah mutasi | View biasa dalam transaksi yang sama. |
| INSERT/UPDATE/DELETE | Tabel sumber iam/asset/competition/judging. |
| Auth, otorisasi internal dan SELECT FOR UPDATE | Tabel sumber/jalur internal terbatas. Pemeriksaan otorisasi selesai sebelum data dikembalikan ke browser. |
| Rekap penilaian aktif dan kandidat pemenang | `cloud_read.v_round_score_summary`, bukan materialized view. |
| Laporan administratif | `cloud_read.mv_round_score_summary` dan `cloud_read.mv_arena_dashboard`. |

View tidak memuat hash/token autentikasi, credential RTSP atau URL rekaman. View profil/pemilik/registrasi mengandung kontak privat yang hanya boleh dibaca oleh aktor yang berhak. Role database bukan pengganti otorisasi per pengguna/arena pada UseCase.

FK gabungan menjaga konsistensi arena pada event/kategori/ronde, registrasi/item/gantangan, engine/perangkat dan skor/juri/kriteria. Satu burung dapat ikut beberapa ronde, tetapi hanya satu item aktif per burung per ronde. Kriteria bersama per arena terkunci setelah ronde pertama dimulai.

Satu penilaian peserta–juri mempunyai satu versi. Seluruh kriteria masuk bersama; setiap revisi menulis history, memperbarui skor berlaku, serta mencatat operation ID/hash payload/hasil dalam satu transaksi. Submit/revisi dan penutupan mengunci baris ronde yang sama. Pemeriksaan role, fase, kelengkapan, nilai tertinggi dan penguncian hasil berada di UseCase.

## Materialized view

| Snapshot | Isi |
| --- | --- |
| `mv_round_score_summary` | Satu peserta ronde: arena/event/ronde, burung/gantangan, jumlah nilai/juri, jumlah nilai wajib dan rata-rata. |
| `mv_arena_dashboard` | Satu arena: jumlah event, ronde, ronde CLOSED, registrasi, peserta terkonfirmasi dan total pembayaran VERIFIED. Agregasi terpisah mencegah perkalian baris. |

Migrasi membuat snapshot pertama dan indeks unik penuh. Worker me-refresh menggunakan `CONCURRENTLY` pada awal proses, setiap `MV_REFRESH_INTERVAL` (60s), dan setelah notifikasi commit verifikasi pembayaran, pembatalan registrasi, atau perubahan fase ronde. Notifikasi dikelompokkan; submit/revisi skor tidak memicu refresh per request.

Setiap refresh memakai transaksi sendiri, advisory lock per snapshot dan timeout. Isi snapshot serta `refresh_status` di-commit bersama. Endpoint laporan mengambil data dan metadata dalam satu statement; `snapshot_at` menunjukkan batas konservatif waktu snapshot dan `refreshed_at` waktu refresh selesai. Jika gagal, snapshot sebelumnya tetap tersedia dan dicoba kembali pada siklus berikutnya. Data dapat lebih lama dari interval ketika refresh gagal.

Pemeliharaan manual, setelah transaksi bisnis commit:

```sql
BEGIN;
SELECT pg_advisory_xact_lock(753120);
-- Catat clock_timestamp() sebelum refresh sebagai snapshot_at untuk metadata.
REFRESH MATERIALIZED VIEW CONCURRENTLY cloud_read.mv_round_score_summary;
-- Update refresh_status dengan waktu awal yang dicatat dan waktu selesai.
COMMIT;
```

Gunakan worker aplikasi untuk pembaruan metadata otomatis. Angka lock 753120 untuk rekap ronde, 753121 untuk dashboard arena. Jangan memakai snapshot untuk menerima nilai, memeriksa pembayaran, atau menentukan pemenang resmi.

## Query berparameter

File query memakai penanda nama, bukan PREPARE:

```sql
-- name: ExampleOperation
SELECT id FROM example.table WHERE id = $1;
```

Nama harus unik. Repository meminta statement berdasarkan nama dan mengikat nilai melalui pgx; tidak menyisipkan input pengguna ke string SQL. Prefix resource CRUD berasal dari allowlist server. Contoh di atas menjelaskan format, bukan statement yang dijalankan aplikasi.

| File | Alur |
| --- | --- |
| `auth.sql` | Lookup kredensial internal, verifikasi/reset, provider Google dan sesi. |
| `iam.sql` | Profil aman, role dan penugasan arena, pemeriksaan otorisasi. |
| `asset.sql` | Arena/gantangan/engine/perangkat, pemilik/burung dan transfer. |
| `competition.sql` | Event/kategori/ronde, registrasi/pembayaran, locking dan rekap langsung. |
| `judging.sql` | Kriteria, roster juri, assessment peserta, revisi dan idempotensi. |
| `reporting.sql` | Refresh, lock dan pembacaan snapshot administratif. |
| `media.sql` | Metadata rekaman sesuai hak registrasi; belum ada ingest engine. |

Statement pembacaan biasa dapat dicoba dengan PREPARE/EXECUTE dalam sesi psql untuk inspeksi, tetapi repository memakai body SQL langsung. Jangan menjalankan semua file query sebagai script migrasi karena placeholder membutuhkan binding.

## Privilege runtime

Migrasi membuat grup `kicau_cloud_app` NOLOGIN. Pemilik deployment memberikan membership ke akun layanan yang dibuatnya sendiri:

```sql
GRANT kicau_cloud_app TO cloud_service;
```

Grup aplikasi menerima SELECT pada read layer, privilege yang diperlukan pada tabel sumber, dan membership pemelihara snapshot. Tidak diberi INSERT/UPDATE/DELETE pada view. History transfer/revisi serta ledger operasi menerima INSERT/SELECT sesuai kebutuhan. Reader operasional hanya melihat view publik minimum; reader admin dapat membaca read layer lengkap. Login role, password, TLS, dan membership akun layanan dikonfigurasi di deployment, bukan ditanam dalam migrasi.

## Rollback dan verifikasi

Jalankan `.down.sql` dari nomor terbesar, masing-masing dalam transaksi. 007 melepaskan read layer baru dan grant aplikasi; 006 menolak bila data workflow sudah digunakan; 005 menolak penghapusan hasil resmi; 004 hanya mengembalikan struktur kosong dan read layer 001–003. Role seed yang dibuat 006 dicatat agar rollback hanya menghapus role milik migrasi, bukan role yang sudah ada sebelumnya. Role grup cluster tetap dipertahankan.

Database operasional yang sudah terisi membutuhkan rencana pemulihan dari backup. Rollback ini bukan konversi balik data bisnis.

Migrasi 001–007 telah diperiksa up/down/penerapan ulang di PostgreSQL sementara. Statement query diperiksa dengan PREPARE di bawah role aplikasi terbatas; view read-only dan hak MAINTAIN juga diperiksa. Pemeriksaan HTTP memakai data sintetis dan SMTP loopback, bukan database atau email produksi. Suite tes Go tidak dijalankan otomatis.
