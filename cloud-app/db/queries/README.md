# Query bisnis cloud-app — ERD 15 November 2026

SQL ini mengikuti migrasi **010**, bukan model lama sebelum tiket tunggal. Setiap statement diberi `-- name:` dan komentar parameter sesuai urutan `$1`, `$2`, dst. Package `queries` meng-embed file SQL; ambil dengan `queries.Get(name)` dan bind parameter melalui pgx. Jangan menjalankan seluruh file sebagai script migrasi atau menyisipkan input ke string SQL.

**Cakupan: skema database dan SQL.** Repository/UseCase/handler, Swagger/Insomnia, bootstrap akun serta layanan token aplikasi lama belum diadaptasi. Query login bukan endpoint login siap pakai. Tidak ada token/hash password tiruan untuk login, pemasangan Redis, seed pengguna atau perubahan VPS pada pekerjaan ini.

## Peta 13 alur

| Alur | File / statement utama |
| --- | --- |
| 1. Login | `auth.sql`: `LoginAccountByEmail`, `LoginUserLock`, `LoginExpireSessions`, `LoginOtherDeviceCount`, `LoginSessionSave`, `LoginSessionValidate`. |
| 2. Registrasi form peserta | `iam.sql`: `ParticipantCreate`, `ParticipantAssignRole`, `MasterUserGet`. Ini membuat master akun peserta; tiket lomba dibuat terpisah. |
| 3. Diskualifikasi peserta | `competition.sql`: `RegistrationLock`, `ParticipantDisqualify`, `RegistrationGet`. |
| 4. Bikin event lomba | `competition.sql`: `EventCreate/Get/List/Update/Delete`, `EOArenaHistory`. |
| 5. Tambah peserta ke lomba | `competition.sql`: `RegistrationCreate`, `RegistrationSetProof`, `RegistrationVerifyPayment`, `RegistrationGet`. Satu tiket = satu burung di satu ronde/gantangan. |
| 6. CRUD burung, owner dan achievement | `asset.sql`: `BirdCreate/Get/List/Update/SoftDelete`, `BirdTransferOwner`, `BirdOwnershipHistory`, `BirdMasterHistory`, `BirdAwardCreate/Get/List/Update/Delete`. |
| 7. Kategori lomba | `competition.sql`: `CategoryCreate/Get/List/Update/Delete`, lingkup arena. |
| 8. CRUD gantangan | `asset.sql`: `GantanganCreate/Get/List/Update/Delete`, termasuk `baris` dan `kolom`. |
| 9. CRUD arena | `asset.sql`: `ArenaCreate/Get/List/Update/Delete`. |
| 10. Competition round | `competition.sql`: `RoundCreate/Get/List/Update/Delete`, `RoundLock`, `RoundSetStatus`, `RoundParticipants`. |
| 11. Scoring juri | `judging.sql`: `RoundJudgeAssign/Remove/List`, `JudgeRoundList`, `ScoreList`, `ScoreUpsert`, `RoundLiveSummary`, `RoundCompleteness`. |
| 12. Kategori penilaian | `judging.sql`: `ScoreCriterionCreate/Get/List/Update/Delete`; ini **kriteria skor**, berbeda dari kategori tiket lomba. |
| 13. Media recording | `media.sql`: `MediaRecordingSave/Get/List/Delivery/Delete`, `MediaRecordingSourceInternal`. Metadata saja, bukan mengaktifkan ingest engine atau pengiriman WhatsApp. |

Master pendukung tersedia: `BirdSpeciesCreate/Get/List/Update/Delete`, master peserta berversi, history master, role global/arena, Google provider dan sesi perangkat. `reporting.sql` menyediakan pembacaan/refresh snapshot materialized view.

## Aturan penggunaan bersama

- Handler hanya menangani transport. UseCase memeriksa aktor, hak akses, sintaks bisnis, kelayakan, state dan koordinasi transaksi; repository hanya menjalankan statement. Query bukan pengganti otorisasi pengguna.
- GET bisnis membaca `cloud_read.v_*`; laporan membaca materialized view. Mutasi menulis tabel sumber, mengembalikan `id`/`version`, kemudian baca view pada transaksi yang sama. Jangan mengandalkan SELECT view dalam satu statement CTE mutasi untuk membaca state baru: gunakan statement berikutnya dalam transaksi tersebut.
- Lookup password/provider, sesi, `FOR UPDATE`, bukti transfer, URL sumber rekaman dan URL sertifikat adalah jalur internal terbatas. Hash/URL/kontak privat tidak boleh diteruskan tanpa DTO dan otorisasi yang sesuai. Query `*Internal` bukan endpoint GET publik.
- Daftar memakai cursor UUID nullable dan ID sebagai penentu urutan terakhir. UseCase membatasi limit 1–100 serta menerjemahkan cursor API. Parameter `NULL` berbeda dari string kosong.
- Patch JSON hanya menerima field yang tercantum pada statement; UseCase wajib menolak key asing dan memvalidasi tipe/null sebelum repository. `arena_id`, ID, kode burung dan versi tidak dapat dipindahkan melalui patch umum.
- Mutasi yang tidak mengembalikan baris harus diperiksa. Ini dapat berarti ID/scope tidak cocok, versi usang, state tidak layak, atau operasi idempoten. Jangan selalu menganggapnya sukses; gunakan hasil lock/read untuk memetakan 404/409 atau no-op.
- Untuk alur yang berbagi data, gunakan urutan lock konsisten: arena untuk konfigurasi, lalu ronde, lalu registrasi; master yang lebih dari satu dikunci dalam urutan UUID. Gunakan transaksi pendek dan klasifikasi konflik/deadlock pada UseCase.

## Master peserta/burung dan history otomatis

Setiap transaksi yang membuat/mengubah/soft-delete master dimulai dengan `SetMutationContext(actor_id, arena_id nullable)` di dalam `Store.Do`. Nilai context berasal dari aktor yang telah diverifikasi UseCase, bukan field actor/arena sembarang pada body browser. `set_config(..., true)` hanya berlaku sampai akhir transaksi; pemanggilan terpisah di luar transaksi tidak cukup.

```text
BEGIN
→ SetMutationContext
→ cek hak akses dan MasterUserLock / BirdLock
→ cek expected_version
→ jalankan mutasi
→ baca hasil dari view
COMMIT
```

Trigger mengelola `version`, `updated_at`, dan snapshot allowlist. Request tidak menulis history atau menetapkan versi baru secara langsung. Create menghasilkan versi 1. Update isi identik mempertahankan versi/waktu/history. Perubahan A → B → A menghasilkan versi berurutan. Expected version usang harus ditolak, termasuk ketika isi request kebetulan sama. Kolom autentikasi bukan bagian snapshot identitas; perubahan password/verifikasi email tidak menambah history/versi master.

`ParticipantCreate` menerima UUID yang dialokasikan aplikasi. Untuk pendaftaran mandiri yang diizinkan, aktor CREATE dapat memakai UUID akun baru itu sendiri; untuk pencatatan oleh petugas, aktornya adalah akun petugas. Kebijakan pendaftaran publik tetap di UseCase. Pembuatan ADMIN pertama memerlukan prosedur bootstrap baru/DBA; script Go bootstrap lama belum kompatibel.

History bersifat append-only melalui trigger security-definer dengan `search_path` tetap. Grup aplikasi hanya membaca history dan tidak dapat INSERT/UPDATE/DELETE/TRUNCATE. Jika pencatatan history gagal, perubahan master dibatalkan juga. Perlindungan ini tidak membatasi pemilik objek/DBA.

Soft delete akun mematikan akun, menaikkan versi dan mencabut sesi PostgreSQL dalam transaksi yang sama. Delete ulang dengan versi terkini tidak menghasilkan history baru. Token cache/Redis, bila kelak digunakan, juga harus dicabut oleh layanan autentikasi; validasi sesi tetap memeriksa PostgreSQL. Tidak ada pemulihan master otomatis.

## Login dan sesi

1. Lookup `LoginAccountByEmail`; verifikasi hash Argon2id pada UseCase. Jangan kirim plaintext password untuk dibandingkan SQL atau mengembalikan hash ke frontend. Periksa akun aktif, tidak terhapus dan kebijakan verifikasi email.
2. Dalam transaksi, `LoginUserLock` menserialisasi login akun tersebut. Jalankan `LoginExpireSessions`, lalu `LoginOtherDeviceCount`; tolak bila sudah tiga perangkat lain aktif.
3. `LoginSessionSave` membuat/memperbarui sesi perangkat dengan expiry yang ditentukan layanan. Issue access/refresh credential hanya setelah penyimpanan berhasil; format/penyimpanan token belum ditetapkan oleh query ini.
4. `LoginSessionValidate` menerima session ID dan user ID **hasil verifikasi credential**, bukan menerima UUID sesi mentah sebagai Bearer token. Query tersebut tidak memverifikasi signature/hash token.
5. Untuk Google, verifikasi Google ID token asli di service sebelum `LoginGoogleAccount`/`LoginGoogleLink`. Jangan otomatis menautkan provider hanya karena email dari body cocok.

`AccountVerifyEmail`/`AccountSetPassword` hanya operasi PostgreSQL setelah token Redis sudah divalidasi sesuai protokol layanan. Penyimpanan, TTL dan konsumsi token sekali pakai di Redis tidak dibuat oleh SQL ini. Saat reset password, cabut sesi sesuai kebijakan dalam transaksi yang sama. Integrasi auth lama belum diganti ke alur ini.

## Form peserta dan tiket lomba

Untuk akun/master peserta, jalankan `ParticipantCreate` dan `ParticipantAssignRole` dalam satu transaksi/context. Jika akun sudah ada, gunakan akun tersebut; jangan membuat duplikat berdasarkan nama. Phone/email akun terhapus tetap dicadangkan oleh constraint unik. `ParticipantList` dan `MasterUserGet` mengecualikan master terhapus.

Untuk memasukkan peserta ke lomba:

```text
Lock arena/ronde sesuai operasi
→ validasi akun pendaftar, burung/owner aktif, spesies, fase dan akses arena
→ RegistrationCreate
→ simpan bukti transfer jika perlu
→ verifikasi pembayaran dan gantangan
→ RegistrationVerifyPayment
→ RegistrationGet
```

Akun pendaftar boleh berbeda dari owner burung. Query insert mengecualikan pendaftar/burung terhapus dan memeriksa spesies ronde. Satu burung tidak boleh terdaftar dua kali pada ronde yang sama. Verifikasi pembayaran dan konfirmasi tiket dilakukan bersama; benturan gantangan menggagalkan transaksi. Gantangan dapat digunakan pada ronde lain. Pembayaran tunai tetap diverifikasi secara eksplisit; tidak otomatis VERIFIED karena metode CASH.

`RegistrationSetProof` menerima bytes, bukan nama file yang diinterpolasi. Validasi ukuran, signature/MIME gambar dan hak akses pada UseCase. `RegistrationProofInternal` menghasilkan satu objek JSON dengan data base64 dan MIME untuk service internal; jangan mencampurkannya ke list peserta.

Diskualifikasi memakai `RegistrationLock` dan `ParticipantDisqualify` setelah izin/kebijakan fase diperiksa. Ia tidak membatalkan tiket, mengubah pembayaran, menghapus skor, membebaskan gantangan, atau melakukan refund. Tidak ada fitur pembatalan diskualifikasi dalam query ini.

## Burung, transfer owner dan achievement

Create memakai `BirdCreate`; kode stabil dibuat database, snapshot awal dan histori owner awal disimpan atomik. Edit umum hanya mengubah nama/spesies. Perubahan spesies pada burung yang sudah digunakan memerlukan keputusan bisnis UseCase; FK saja tidak mengaturnya.

Transfer harus memakai `BirdTransferOwner`, bukan patch `owner_id` langsung. UseCase memeriksa hak petugas dan owner tujuan yang aktif/tidak terhapus. Query memakai expected version, memperbarui owner, menulis histori kepemilikan dan memicu snapshot master pada transaksi yang sama. Owner tujuan sama tidak menambah versi/history. Kode burung tidak berubah.

Penghargaan internal mengisi `registration_id` dan burung yang sama dengan tiket; eksternal memakai `registration_id=NULL`, nama event dan tanggal. Gunakan `BirdAward*` untuk metadata. UseCase menolak penghargaan internal bagi peserta diskualifikasi serta mengatur file sertifikat; query tidak mengunggah atau menerbitkan file.

## Event, ronde, fasilitas dan kriteria

Event memakai owner akun EO (`is_eo=true`) dan arena wajib. UseCase memeriksa aktor, arena aktif serta flag EO. ADMIN mengatur hak akses; flag EO tidak menggantikan role. Jumlah ronde fleksibel, nomor unik per event/arena, spesies wajib dan kategori opsional. Patch konfigurasi ronde dilakukan sebelum fase terkunci; perpindahan status memakai `RoundSetStatus` dengan expected status setelah `RoundLock`.

Kategori lomba dan kriteria skor merupakan master berbeda. UseCase mengunci arena sebelum perubahan kriteria; tolak mutasi jika `criteria_locked_at` sudah terisi. Penguncian saat Mulai memakai `ArenaLockCriteria`. Penugasan roster diperiksa dan dikunci sesuai lifecycle ronde; jangan mengubah roster ketika penjurian sudah berjalan.

Arena/gantangan/kategori/ronde/spesies menggunakan DELETE fisik hanya bila tidak dirujuk data lain; FK akan menolak penghapusan yang merusak relasi. Untuk arena yang masih dipakai, nonaktifkan melalui Update jika sesuai kebijakan. Soft delete/history berversi pada ERD ini khusus master akun dan burung.

## Scoring dan rekap

Dalam satu transaksi, lock ronde lalu registrasi. Verifikasi aktor sebagai juri pada roster arena/ronde, status JUDGING, tiket CONFIRMED/VERIFIED dan peserta ACTIVE; validasi seluruh kriteria/nilai 1–10. Jalankan `ScoreUpsert` untuk setiap kriteria dalam paket submit agar seluruh paket commit atau rollback bersama. Revisi menimpa skor terkini, tanpa ledger atau histori revisi skor. Expected version master bukan versi penilaian; tab juri bersamaan tidak mendapat mekanisme konflik versi skor dari schema ini.

`RoundLiveSummary` membaca rata-rata terkini. `RoundCompleteness` harus menyatakan ada peserta dan seluruh nilai wajib lengkap sebelum UseCase menutup ronde. Gunakan `RoundSetStatus` dalam transaksi yang sama dengan pemeriksaan; jangan memakai snapshot administratif untuk keputusan tersebut. ERD belum menyimpan pengesahan pemenang resmi.

## Media dan materialized view

`MediaRecordingSave` menyimpan metadata satu rekaman per tiket. `MediaRecordingDelivery` mencatat status/waktu kirim; nomor penerima ditelusuri dari akun pendaftar terkini. UseCase/worker memeriksa akses serta transisi status yang sah. Endpoint ingest engine, uploader dan pengiriman pesan tidak diaktifkan oleh query ini. `MediaRecordingDelete` hanya menghapus metadata, bukan file S3.

`ReportRoundSnapshot` dan `ReportArenaDashboard` membaca snapshot yang dapat tertinggal. Refresh **setelah transaksi bisnis commit**, pada transaksi pemeliharaan tersendiri: `ReportRefreshLock` → catat waktu awal di aplikasi → `ReportRoundRefresh` atau `ReportDashboardRefresh` → `ReportMarkRefreshed` → commit. Gunakan lock 753120 untuk skor dan 753121 untuk dashboard. Pembacaan status snapshot melalui `ReportRefreshStatus`.

## Pemeriksaan SQL

Seluruh query berparameter diperiksa dengan PREPARE oleh PostgreSQL, bukan hanya pencarian teks. Dua perintah REFRESH adalah utility tanpa parameter dan diuji melalui eksekusi. Skenario PostgreSQL menggunakan grup aplikasi terbatas dan rollback fixture; suite tes Go tidak dijalankan.

```powershell
# Gunakan database sementara yang sudah sampai migrasi 010, bukan DB aplikasi.
python cloud-app/db/verification/run_business_flows.py --database NAMA_DATABASE_PENGUJIAN
```

Runner mengambil statement aktual dari file SQL, sehingga tidak menduplikasi body query dalam fixture. Pemeriksaan meliputi perubahan/no-op/konflik versi, transfer, pembayaran, scoring/revisi, diskualifikasi, media, soft delete, pencabutan sesi, privilege history dan rollback ketika history gagal ditulis. Pemeriksaan query tidak berarti endpoint/API aplikasi lama sudah kompatibel.
