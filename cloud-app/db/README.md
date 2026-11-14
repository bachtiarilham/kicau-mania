# Database dan SQL cloud-app

**Database lokal sudah mengikuti ERD revisi 14 November 2026 melalui migrasi 008–009, diterapkan pada 15 November 2026 ke `localhost:5433/kicau_mania`.** Data bisnis lama dihapus sesuai persetujuan. Hanya lima role konfigurasi sistem diisi; tidak ada akun/password, arena, spesies atau peserta contoh.

**Backend Go, query SQL bisnis, bootstrap superadmin dan seed/Insomnia lama belum disesuaikan.** Jangan menjalankan seed lama atau menyatakan API siap hanya karena skema berhasil dibuat. Kesiapan database aplikasi lama dapat gagal karena tabel/kolom yang dihapus. Integrasi Redis untuk token verifikasi/reset email juga belum diimplementasikan; migrasi PostgreSQL tidak memasang Redis.

## Skema yang berlaku

Rujukan: [ERD](ERD.md). Ada 20 tabel bisnis:

| Schema | Tabel |
| --- | --- |
| `iam` | `users`, `roles`, `user_roles`, `arena_user_roles`, `user_sessions`, `user_auth_providers` |
| `asset` | `arenas`, `gantangan`, `bird_species`, `birds`, `bird_ownership_history`, `bird_award_certificate_history` |
| `competition` | `events`, `categories`, `rounds`, `registrations` |
| `judging` | `score_criteria`, `round_judges`, `round_scores` |
| `media` | `round_recordings` |

EO adalah akun `iam.users.is_eo=true`. Pemilik burung dan pendaftar mengacu ke akun pengguna; spesies menjadi master bersama. Satu registrasi adalah satu burung pada satu ronde/gantangan, dengan pembayaran dan bukti transfer BYTEA pada tiket. Diskualifikasi terpisah dari status registrasi/pembayaran. Nilai desimal 1–10 direvisi dengan UPDATE pada baris yang sama. Penghargaan mendukung prestasi internal maupun eksternal.

Tabel pemilik/perantara pembayaran, assessment, ledger operasi, histori revisi skor, token akun PostgreSQL, engine/perangkat, dan membership lama tidak ada pada skema baru. Ronde tidak menyimpan pemenang resmi, akun pengesah, biaya atau durasi. Schema `member` lama dihapus karena tidak termasuk ERD ini.

Role aplikasi bisnis sekarang: `SUPERADMIN`, `OPERATOR`, `JURI`, `EO_ADMIN`, `PESERTA`. Penamaan ini mengikuti ERD dan berbeda dari seed/otorisasi backend lama. Grup login PostgreSQL `kicau_cloud_app` tetap merupakan grup privilege database yang terpisah dari role bisnis.

## Migrasi dan reset eksplisit

| Versi | Status |
| --- | --- |
| 001–007 | Riwayat skema lama; jangan diedit atau diulang pada database yang sudah menerapkannya. |
| 008 | **Reset destruktif** schema bisnis lama menjadi ERD baru. Menghapus seluruh data pada schema terkait dan mempertahankan akun/login PostgreSQL. |
| 009 | Membuat 22 view baca, dua materialized view, metadata snapshot dan grant sesuai skema 008. |

Target lokal sudah menerima 008 dan 009; **jangan jalankan ulang**. Database VPS tidak disentuh pada pekerjaan ini. Untuk database lain yang memang sudah di versi 007 dan sengaja boleh dikosongkan, hentikan aplikasi dahulu dan jalankan dari root repo:

```powershell
$pgBin = 'C:/Program Files/PostgreSQL/18/bin'
# Pastikan host/port/nama database sesuai target yang secara eksplisit akan dihapus.
& "$pgBin/psql.exe" -X -h localhost -p 5433 -U postgres -d kicau_mania -v ON_ERROR_STOP=1 -1 `
  -f cloud-app/db/migrations/008_simplified_erd_reset.up.sql `
  -f cloud-app/db/migrations/009_simplified_cloud_read.up.sql
if ($LASTEXITCODE -ne 0) { throw 'Reset dibatalkan; periksa error PostgreSQL' }
```

Dua file dijalankan dalam satu transaksi. Password diminta psql atau diberikan melalui konfigurasi koneksi lokal; tidak ditanam pada file SQL. Perintah ini bukan migrasi startup/deployment otomatis. Jangan menerapkannya pada database berisi data yang hendak dipertahankan.

Untuk database pengujian baru, [dump baseline lama](../../db-dump/full_db.dump) tetap dipertahankan sebagai baseline historis PostgreSQL 18. Restore tanpa owner/privilege ke database sementara, kemudian terapkan 001–009 berurutan dengan `ON_ERROR_STOP` dan transaksi. Dump tersebut bukan snapshot terbaru. Rantai lengkap ini telah diperiksa pada PostgreSQL 18.6; versi 008 mensyaratkan struktur 007 dan menolak penerapan ulang.

Rollback 009 hanya menghapus lapisan baca, tanpa menghapus tabel bisnis 008. **008 tidak menyediakan rollback otomatis** karena data lama telah dihapus; file down sengaja berhenti dengan pesan yang jelas. Untuk memakai aplikasi/skema lama, bangun database baru dari baseline dan 001–007 secara eksplisit. Data yang sudah dihapus tidak dapat dipulihkan lewat file down.

## Integritas dan batas aplikasi

Database memeriksa FK/lingkup arena, penugasan juri pada ronde skor, keunikan burung per ronde, gantangan terkonfirmasi per ronde, alasan/waktu diskualifikasi, kecocokan burung penghargaan internal, rentang skor, nominal pembayaran finite/nonnegatif, serta pasangan bukti bayar dan MIME. Gantangan harus memiliki baris/kolom positif dan posisi unik per arena.

`bird_code` dibuat dengan sequence (`BRD-000001`, dst.) sehingga aman dari benturan nomor default pada pembuatan bersamaan. Sequence dapat memiliki celah termasuk setelah rollback; bukan jumlah baris. Aplikasi harus mempertahankan kode saat mengubah nama/pemilik burung dan memperbarui `updated_at` ketika melakukan mutasi.

**Validasi semantik tetap wajib pada UseCase berikutnya:** akun EO saat membuat event, kecocokan spesies burung/ronde, batas tiga sesi perangkat, penugasan role arena, kriteria/roster terkunci, submit hanya pada fase JUDGING, kelayakan peserta, penolakan skor/penghargaan untuk peserta diskualifikasi, transisi/penutupan ronde dan otorisasi mutasi. FK tidak menggantikan aturan tersebut. Kolom sesi mengikuti ERD; penyimpanan/pemeriksaan token sesi lama belum kompatibel dan perlu diputuskan pada adaptasi aplikasi.

## View dan materialized view

GET nantinya membaca `cloud_read.v_*`; INSERT/UPDATE/DELETE menulis tabel sumber. Query lama pada [queries](queries/) masih memakai model sebelum 008 dan **ditahan untuk refactor berikutnya**, bukan query kompatibel skema terbaru. Jangan menjalankan file query berparameter sebagai migrasi.

View baru mencakup master spesies, histori kepemilikan/penghargaan, tiket tunggal, skor terkini, serta `v_eo_arena_history` untuk hubungan EO–arena melalui event. Pada histori EO, `held_event_count` hanya menghitung event ACTIVE/COMPLETED; rencana dan pembatalan dihitung terpisah.

`v_round_score_summary` menghitung rata-rata desimal terkini untuk peserta CONFIRMED, pembayaran VERIFIED dan peserta ACTIVE. Peserta tanpa skor mempunyai count 0 dan average NULL; `is_complete` membutuhkan roster/kriteria tidak kosong serta seluruh nilai wajib. Peserta diskualifikasi dikeluarkan dari rekap tetapi skornya tetap ada di tabel.

`mv_round_score_summary` dan `mv_arena_dashboard` adalah snapshot administratif. Keduanya diisi saat migrasi dan memiliki indeks unik penuh untuk refresh concurrent. Gunakan view biasa untuk penilaian/keputusan langsung. `verified_payment_amount` adalah jumlah fee tiket VERIFIED, termasuk tiket yang kemudian dibatalkan; skema tidak mencatat refund otomatis.

Refresh dilakukan manual setelah perubahan commit sampai worker aplikasi disesuaikan. Gunakan transaksi/lock terpisah untuk masing-masing snapshot:

```sql
BEGIN;
SELECT pg_advisory_xact_lock(753120);
-- Catat waktu awal, bukan akhir refresh, sebagai batas snapshot.
SELECT clock_timestamp() AS snapshot_started \gset
REFRESH MATERIALIZED VIEW CONCURRENTLY cloud_read.mv_round_score_summary;
UPDATE cloud_read.refresh_status
SET snapshot_at = :'snapshot_started'::timestamptz, refreshed_at = clock_timestamp()
WHERE view_name = 'mv_round_score_summary';
COMMIT;
```

Untuk dashboard, gunakan lock `753121` dan nama `mv_arena_dashboard`. Contoh memakai psql karena `\gset` dan variabel adalah fitur client psql, bukan sintaks SQL server. Aplikasi nantinya mengikat timestamp sebagai parameter. Jangan menganggap snapshot otomatis mutakhir tanpa refresh yang berhasil.

## Hak akses dan data privat

PUBLIC tidak mendapat akses schema bisnis/read layer. `kicau_cloud_reader` hanya membaca view master yang diizinkan. IAM, tiket/kontak, skor, histori dan rekap dibatasi ke pembaca admin/aplikasi. View tidak mengeluarkan password hash, ID provider eksternal, BYTEA bukti bayar atau URL rekaman/sertifikat mentah. Nomor penerima media dibaca dari akun pendaftar terkini; tidak memakai snapshot nomor lama.

Grup aplikasi mendapat privilege tabel sumber dan sequence untuk adaptasi repository nanti; role master tidak dapat dimutasi oleh grup tersebut. `kicau_cloud_recap_maintainer` mendapat MAINTAIN pada dua materialized view dan SELECT/UPDATE metadata refresh. Membership akun layanan PostgreSQL tetap dipertahankan oleh reset schema.

## Verifikasi yang sudah dilakukan

Di PostgreSQL sementara: restore baseline, terapkan 001–009, pastikan akun fixture lama terhapus, lalu periksa skema dan operasi melalui grup aplikasi terbatas. Di target lokal: 008–009 diterapkan bersama secara atomik, lalu pemeriksaan integrasi yang sama dijalankan dalam transaksi yang di-rollback.

Script [verification/009_simplified_erd.sql](verification/009_simplified_erd.sql) memeriksa relasi opsional, batas arena/roster, tiket lintas ronde, benturan burung/gantangan, skor desimal dan overwrite, snapshot sebelum/sesudah refresh, diskualifikasi, penghargaan internal/eksternal, nomor penerima media terkini, dan privilege read layer. Fixture tidak di-commit; sequence dapat tetap bertambah. Jalankan pada database pengujian, bukan saat aplikasi aktif:

```powershell
& 'C:/Program Files/PostgreSQL/18/bin/psql.exe' -X -h localhost -p 5433 -U postgres -d kicau_mania `
  -v ON_ERROR_STOP=1 -f cloud-app/db/verification/009_simplified_erd.sql
```

Suite tes Go tidak dijalankan. Tidak ada seed, akun superadmin, instalasi Redis atau perubahan source aplikasi pada fase database ini.
