# Database dan SQL cloud-app

**Status lokal 15 November 2026:** `localhost:5433/kicau_mania` sudah menerapkan migrasi **008–010** dan mengikuti [ERD terbaru](ERD.md). Migrasi 008 sebelumnya mereset data lama; migrasi 010 menambahkan history/version/soft delete tanpa reset data. Ada **22 tabel bisnis, 24 view baca dan dua materialized view**. Role bisnis sekarang `ADMIN`, `OPERATOR`, `JURI`, `PESERTA`.

**Query untuk 13 flow bisnis sudah tersedia** pada [panduan query](queries/README.md). Repository/UseCase/handler, otorisasi role, bootstrap, seed/Insomnia dan dokumentasi endpoint lama belum diadaptasi ke kontrak SQL terbaru. Jangan menganggap login/API siap hanya karena SQL lolos pemeriksaan. Redis untuk token verifikasi/reset email juga belum dipasang atau diintegrasikan.

## Skema yang berlaku

| Schema | Tabel |
| --- | --- |
| `iam` | `users`, `user_master_history`, `roles`, `user_roles`, `arena_user_roles`, `user_sessions`, `user_auth_providers` |
| `asset` | `arenas`, `gantangan`, `bird_species`, `birds`, `bird_master_history`, `bird_ownership_history`, `bird_award_certificate_history` |
| `competition` | `events`, `categories`, `rounds`, `registrations` |
| `judging` | `score_criteria`, `round_judges`, `round_scores` |
| `media` | `round_recordings` |

EO adalah akun dengan `is_eo=true`, terpisah dari role/hak akses. Pemilik burung dan pendaftar mengacu ke akun; spesies memakai master bersama. Satu tiket mewakili satu burung pada satu ronde/gantangan. Pembayaran/BYTEA bukti transfer ada pada tiket; diskualifikasi terpisah dari pembayaran. Skor 1–10 mendukung desimal dan revisi langsung UPDATE. Ronde mandiri tidak menyimpan bracket otomatis atau pengesahan pemenang resmi.

`iam.user_master_history` dan `asset.bird_master_history` berbeda dari histori kepemilikan, achievement serta histori skor lama. Histori skor/ledger/assessment, tabel owner, item/pembayaran perantara, account_tokens PostgreSQL, engine/perangkat dan membership lama tetap tidak ada. Tidak ada shadow table kompatibilitas untuk query lama.

## Migrasi

| Versi | Isi dan batas |
| --- | --- |
| 001–007 | Riwayat skema lama; jangan diubah atau diulang. |
| 008 | Reset destruktif skema lama menjadi tiket tunggal; menghapus data bisnis lama. |
| 009 | Read layer baru: 22 view, dua snapshot dan privilege. |
| 010 | Version/soft delete master, snapshot BASELINE/trigger append-only, dua view history, serta penggabungan role SUPERADMIN/EO_ADMIN menjadi ADMIN. Data master dan penugasan lama dipertahankan. |

Target lokal **sudah sampai 010; jangan jalankan ulang**. Jika database lain sudah sampai 009, hentikan aplikasi dan terapkan hanya 010 dengan transaksi dan ON_ERROR_STOP. Contoh dari root repo:

```powershell
$pgBin = 'C:/Program Files/PostgreSQL/18/bin'
# Sesuaikan koneksi ke target yang memang hendak dimigrasikan.
& "$pgBin/psql.exe" -X -h localhost -p 5433 -U postgres -d NAMA_TARGET -v ON_ERROR_STOP=1 -1 `
  -f cloud-app/db/migrations/010_master_history_and_soft_delete.up.sql
if ($LASTEXITCODE -ne 0) { throw 'Migrasi gagal; transaksi dibatalkan' }
```

Tidak ada migrasi startup otomatis. Password tidak ditanam pada SQL. Database VPS tidak disentuh. Untuk database pengujian baru, restore [baseline historis](../../db-dump/full_db.dump) PostgreSQL 18 tanpa owner, lalu terapkan 001–010 berurutan. Dump itu tetap baseline lama, bukan snapshot skema sekarang. Jangan memakai migrasi 008 pada data yang ingin dipertahankan.

Rollback 009 hanya menghapus read layer. Down 008 sengaja menolak karena reset tidak dapat memulihkan data terhapus. Down 010 juga menolak pembuangan otomatis history/pemisahan role ADMIN yang sudah digabung. Pemulihan versi lama harus menggunakan database/backup yang dipilih secara eksplisit; bukan membuang history diam-diam.

## History, context dan optimistic concurrency

Setiap master mempunyai `version` positif dan `deleted_at`. Migrasi mencatat BASELINE versi 1 untuk master yang sudah ada; data baru menghasilkan CREATE versi 1. Snapshot hanya berisi field identitas allowlist, bukan password, provider ID, sesi, role atau token. Nilai null disimpan eksplisit.

Sebelum mutasi master, UseCase memulai transaksi, memverifikasi hak aktor, lalu repository menjalankan `SetMutationContext(actor_id, arena_id nullable)`. Context memakai `set_config(..., true)` dan habis ketika transaksi selesai. Aktor diambil dari auth/context yang telah diverifikasi; bukan dipercaya dari body browser. Context ini untuk audit, bukan mekanisme authorization database.

Trigger membandingkan snapshot bisnis. Isi identik tidak menaikkan versi/waktu/history; perubahan A → B → A tetap menghasilkan versi berurutan. Query update/delete menyertakan `expected_version`. Versi usang harus menjadi konflik, termasuk jika isinya sama. ID, kode burung dan versi dikelola sistem. Kolom autentikasi di luar snapshot tidak menambah versi identitas.

History otomatis ditulis oleh fungsi trigger security-definer dengan `search_path` tetap. Grup aplikasi hanya SELECT; INSERT/UPDATE/DELETE/TRUNCATE history ditolak. Kegagalan history membatalkan master juga. DBA/pemilik objek tetap dapat mengelola objek tersebut.

Soft delete akun menonaktifkan akun dan mencabut sesi PostgreSQL atomik. Soft delete tidak menghapus tiket/skor/rekaman/history atau membebaskan phone/email/kode burung. Query login/list master/registrasi baru menolak master terhapus; relasi historis tetap dapat dibaca. Tidak ada restore otomatis. Transfer owner menggunakan query khusus yang menggabungkan update owner, histori kepemilikan dan snapshot master dalam satu transaksi.

## View, query dan snapshot

GET bisnis memakai `cloud_read.v_*`; INSERT/UPDATE/DELETE memakai tabel sumber. Auth internal, locking dan resolusi bukti/URL adalah pengecualian internal terbatas. List/detail umum tidak mengeluarkan hash, provider ID, BLOB bukti atau URL privat. `v_users`/`v_birds` menyertakan version/deleted_at untuk administrasi; query list biasa memfilter master terhapus, sementara tiket historis tetap memakai JOIN master yang sama.

Lihat [queries/README.md](queries/README.md) untuk peta 13 flow, urutan transaksi, parameter dan query yang harus digabung. File `*.sql` memakai penanda nama, bukan PREPARE deployment; package Go meng-embed statement dan pgx mengikat `$1`, `$2`, dst. Query telah diganti untuk schema terkini; caller Go lama belum kompatibel dan tidak disediakan alias palsu.

`v_round_score_summary` memakai rata-rata desimal terkini dari tiket CONFIRMED, pembayaran VERIFIED dan peserta ACTIVE. Count awal nol, average NULL, dan `is_complete` membutuhkan roster/kriteria tidak kosong serta semua nilai wajib. Diskualifikasi tidak menghapus skor tetapi mengeluarkan tiket dari rekap eligible.

`mv_round_score_summary` dan `mv_arena_dashboard` adalah snapshot administratif, diisi pada 009 dan hanya berubah setelah refresh. Indeks unik penuh mendukung refresh concurrent. Jangan memakai snapshot untuk memutuskan kelengkapan penilaian/penutupan ronde. `verified_payment_amount` menjumlahkan fee VERIFIED termasuk tiket yang kemudian CANCELLED; refund otomatis tidak tersedia.

Refresh **setelah transaksi bisnis commit**, dalam transaksi pemeliharaan lain: lock advisory → catat waktu awal → refresh → update `refresh_status` → commit. Query `ReportRefreshLock`, `ReportRoundRefresh`/`ReportDashboardRefresh`, dan `ReportMarkRefreshed` tersedia. Lock skor 753120 dan dashboard 753121; timestamp snapshot adalah waktu awal, bukan akhir refresh. Worker aplikasi lama belum disambungkan ke kontrak query baru.

## Hak akses dan aturan UseCase

PUBLIC tidak mendapat akses schema bisnis/read layer. Reader operasional hanya view master yang diizinkan; IAM/history, kontak tiket, skor dan rekap memerlukan reader admin/aplikasi. History master hanya bisa dibaca role aplikasi, tidak dimutasi langsung. Master akun/burung tidak diberi DELETE/TRUNCATE. Grup pemelihara hanya MAINTAIN dua materialized view dan SELECT/UPDATE metadata refresh. Akun login dan membership PostgreSQL tidak dihapus oleh migrasi 010.

Database memeriksa FK/arena/roster, keunikan tiket/gantangan, bukti/MIME, status diskualifikasi, kecocokan burung penghargaan, skor 1–10 dan fee finite/nonnegatif. Kode burung memakai sequence yang aman untuk pembuatan bersamaan, dapat memiliki celah setelah rollback, dan tetap sama ketika nama/owner berubah.

**UseCase tetap wajib** mengatur role ADMIN/OPERATOR/JURI/PESERTA, scope arena, flag EO, kriteria/roster terkunci, kelayakan peserta, fase ronde, hak transfer/diskualifikasi, penghargaan untuk peserta aktif, validasi upload, batas tiga sesi dan verifikasi password/token. Trigger history tidak mengimplementasikan aturan bisnis tersebut. Query sesi mengikuti ERD: UUID sesi bukan credential; format access/refresh token harus dipilih serta diverifikasi service sebelum menggunakan query validasi sesi. Verifikasi/reset email memerlukan Redis TTL/consume-once pada adaptasi aplikasi berikutnya.

## Pemeriksaan yang sudah dilakukan

- PostgreSQL 18.6 sementara: restore baseline dan migrasi 001–010; master lama menjadi BASELINE dan dua role admin lama bergabung tanpa kehilangan penugasan.
- Semua query berparameter diprepare oleh PostgreSQL dengan role aplikasi; utility REFRESH dieksekusi dalam skenario.
- [Skenario bisnis SQL](verification/010_business_flows.sql) menggunakan statement aktual melalui [runner](verification/run_business_flows.py), lalu me-rollback fixture. Tidak menduplikasi body query pada fixture.
- Skenario mencakup create/no-op/update/A→B→A, konflik versi, transfer/no-op transfer, tiket/pembayaran, skor/revisi desimal, diskualifikasi, media, soft delete/pencabutan sesi, privilege history dan pembatalan master ketika history gagal.
- Dua koneksi editor bersamaan: satu update berhasil, satu tidak mendapatkan row karena versi usang; tepat satu history baru ditulis.
- Target lokal menerima 010 tanpa reset data. Tidak ada akun contoh/superadmin yang dibuat, tidak ada build/deploy VPS atau instalasi Redis. Suite tes Go tidak dijalankan.

Jalankan pemeriksaan pada database sementara yang telah menerima 010:

```powershell
python cloud-app/db/verification/run_business_flows.py --database NAMA_DATABASE_PENGUJIAN
```

Runner memakai koneksi host/port/user dari `.env`, tetapi mewajibkan nama database terpisah dan menolak DB aplikasi yang dikonfigurasi. Script verifikasi 009 hanya untuk schema sebelum 010; gunakan runner terbaru untuk schema aktif.
