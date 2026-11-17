# API aktif schema 011

Prefix `/api/v1`. Semua respons JSON memakai `{success,message,data,errors}`; field null opsional dapat tidak disertakan. Daftar CRUD memakai `{items,next_cursor,has_more}`, `limit` 1–100 (default 25). Cursor adalah UUID terakhir; history memakai `after_version`. Error 400 validasi, 401 sesi invalid, 403 hak akses, 404 tidak ditemukan, 409 konflik versi/relasi/status, 429 batas percobaan login, 503 layanan tidak tersedia. Tidak ada konversi error menjadi daftar kosong.

Swagger `/api/docs/` dan [JSON](generated/swagger.json) dihasilkan melalui `go run ./cmd/openapi docs/generated/swagger.json`. Query SQL bukan endpoint tersendiri. Media desktop ditambahkan melalui migrasi 011. Tidak ada resource inventory owners/engines/devices lama, signup, Google atau reset email.

## Auth dan petugas

| Method/path | Perilaku |
|---|---|
| POST `/auth/login` | `{email,password,device_id}`; JSON access_token/expires_in/user, refresh pada cookie HttpOnly. |
| POST `/auth/refresh` | Cookie refresh + Origin frontend; pasangan token berotasi atomik. |
| POST `/auth/logout` | Cookie refresh + Origin; cabut sesi PostgreSQL dan hapus token Redis. Tetap dapat dipanggil saat access token kedaluwarsa. |
| GET `/auth/me` | Profil aman, role global dan `arena_roles`; tidak berisi password/hash/token. |
| POST `/auth/change-password` | `{old_password,new_password}`; semua sesi dicabut. |
| GET `/auth/sessions` | Sesi perangkat akun sendiri. |
| POST `/auth/sessions/{session}/revoke` | Cabut sesi milik sendiri; token Redis tidak dapat melampaui pencabutan PostgreSQL. |
| GET `/arenas/{arena}/staff` | ADMIN arena; daftar penugasan. |
| POST/DELETE `/arenas/{arena}/staff/{user}/{role}` | ADMIN arena; role tepat ADMIN/OPERATOR/JURI. ADMIN terakhir tidak dapat dicabut. |
| POST `/arenas/{arena}/staff/{user}/credentials` | ADMIN arena menetapkan `{password}` awal, minimal 12 karakter; akun harus punya email dan role petugas. Password yang sudah ada diganti oleh pemilik melalui change-password. |

ADMIN global bootstrap dapat membuat arena dan mengelola master bersama, tetapi data lomba tetap membutuhkan penugasan arena. OPERATOR tidak dapat memberi role, mengaktifkan password atau mengubah akun berprivilege. Pengubahan akun petugas lintas arena dibatasi agar ADMIN satu arena tidak mengambil alih akun arena lain. JURI harus mempunyai role arena dan roster ronde. PESERTA membaca data miliknya sendiri.

## Master

GET/POST collection, GET/PATCH/PUT/DELETE `/{id}` untuk `/users`, `/birds`, `/bird-species`, `/arenas`; serta `/arenas/{arena}/events`, `/categories`, `/gantangan`, `/criteria`, `/rounds`. PATCH dianjurkan untuk perubahan parsial. DELETE users/birds adalah soft delete dan memerlukan query `expected_version`; mutasi master users/birds memerlukan `expected_version` pada body. Burung menggunakan `owner_id`, `bird_species_id`, dan kode burung buatan sistem. Owner tidak diubah melalui PATCH biasa.

| Resource | Field mutasi |
|---|---|
| users | name, phone, email opsional, is_eo, is_active, expected_version pada edit |
| birds | name, owner_id saat baru, bird_species_id, expected_version pada edit |
| bird-species | name |
| arenas | name, address, timezone, is_active |
| events | owner_id akun EO aktif, title, description, event_date, status |
| categories | code, name |
| gantangan | number, description, baris, kolom |
| criteria | criteria_name; terkunci setelah ronde pertama dimulai |
| rounds | event_id, category_id opsional, bird_species_id, round_number, title; status melalui tindakan |

Nama/telepon peserta wajib saat akun dibuat. Akun disimpan dahulu sebelum tiket. `GET /users?q=...` mencari nama/telepon/email, filter `is_eo` dan `role`; `GET /birds?q=...` mencari nama/kode/pemilik/spesies, filter `owner_id`. Lookup arena mengikuti penugasan/peserta, kategori/gantangan/event mengikuti arena. Jenis burung tidak dapat diubah setelah burung digunakan pada tiket.

`GET /users/{id}/history` dan `/birds/{id}/history`: history read-only, snapshot allowlist, actor_id/actor_name, waktu, change_type, version; tetap tersedia bagi petugas setelah master diarsipkan. Trigger menulis history dalam transaksi dan tidak menambah versi untuk snapshot identik. Tidak tersedia endpoint update/delete/rollback history.

`POST /birds/{id}/transfer` memakai `{owner_id,notes,expected_version}`; no-op tidak menambah history kepemilikan. `GET /birds/{id}/ownership` untuk history kepemilikan. `/birds/{id}/awards`: GET daftar, POST baru, GET/PATCH/DELETE `/{award}`. Field: registration_id opsional, award_name, event_name/organizer_name eksternal, rank, awarded_at, certificate_url. Penghargaan internal memerlukan tiket burung yang confirmed/verified/ACTIVE dan akses arena, lewat query `arena_id`. Hubungan tiket tidak dapat dipindah saat edit. URL sertifikat tidak disertakan pada daftar; GET `/{award}/certificate` mengembalikan URL setelah otorisasi. Penyimpanan URL eksternal bukan penyimpanan objek privat; izin unduh objek juga perlu diterapkan oleh penyedia berkas.

## Tiket dan pembayaran

GET/POST `/registrations`; GET/PUT `/registrations/{id}`; query `arena_id` untuk baca/aksi. Body tiket: user_id, arena_id, round_id, bird_id, gantangan_id, entry_fee sebagai desimal, payment_method CASH/TRANSFER. Satu tiket untuk satu burung/ronde/gantangan; duplikat menghasilkan 409. Burung harus sesuai spesies ronde; akun/pemilik aktif dan belum dihapus; gantangan/event/kategori harus sesuai arena.

Edit hanya ketika tiket PENDING_PAYMENT, belum VERIFIED, dan ronde asal/tujuan PREPARED dalam arena sama. Pembatalan memakai POST `/{id}/cancel`, bukan DELETE. Registrasi, pembayaran, dan peserta mempunyai status terpisah.

POST `/{id}/verify`, `/reject`, `/cancel`, `/disqualify`, `/requalify` menerima JSON `{reason}` (wajib tidak kosong untuk diskualifikasi). Verifikasi tunai eksplisit; transfer wajib ada bukti. Pembayaran/batal tiket dilakukan sebelum ronde mulai. Diskualifikasi/pemulihan diperbolehkan sebelum CLOSED/CANCELLED dan tidak mengubah pembayaran.

POST `/{id}/proof`: multipart `file`, JPEG/PNG maksimal 5 MiB; pemeriksaan signature dan decode config gambar, bukan ekstensi/MIME browser saja. GET pada path yang sama mengembalikan bytes setelah otorisasi. Bytes tidak masuk envelope daftar. Reverse proxy dan SvelteKit perlu mengizinkan body multipart 6 MiB; JSON tetap dibatasi 2 MiB di Go.

## Ronde dan nilai

GET `/arenas/{arena}/judges` adalah lookup juri aktif pada arena. GET `/rounds/{id}/judges`, POST/DELETE `/judges/{judge}` mengelola roster hanya PREPARED. GET `/participants` menampilkan tiket confirmed/verified termasuk status peserta, dan GET `/recap` membaca rekap langsung. Semua path ronde memakai prefix `/arenas/{arena}`.

POST `/rounds/{id}/start`: PREPARED → RUNNING, memerlukan arena aktif, peserta eligible, roster juri aktif dan kriteria. `/begin-judging`: RUNNING → JUDGING. `/close`: JUDGING → CLOSED hanya setelah nilai lengkap. `/cancel`: sebelum CLOSED. Body `{expected_status}` mencegah perubahan dari tampilan stale. Tidak ada field bracket atau pemenang tersimpan.

GET `/judge/rounds` hanya tugas akun sendiri. GET/PUT `/arenas/{arena}/rounds/{id}/scores/{registration}` hanya juri roster pada arena tersebut. PUT `{scores:[{criteria_id,score_value:"9.25"},...]}` wajib mencakup setiap kriteria tepat sekali dalam satu transaksi. Nilai 1–10, desimal maksimal 32 karakter; revisi langsung. Round lock yang sama dipakai submit dan penutupan, sehingga penutupan tidak melihat paket nilai separuh. Rekap tidak memakai materialized view.

## Media desktop (schema 011)

Pairing/mapping memakai autentikasi pengguna arena, sedangkan `/engine/media/uploads` memakai credential engine khusus. Detail endpoint, payload, multipart, checksum dan akses file ada pada [kontrak media desktop](desktop-media-contract.md). File disimpan pada bucket privat; cloud tidak menerima body MKV/MP4 besar melalui handler JSON. Signed URL tidak disimpan permanen. Daftar/detail/download peserta dibatasi kepemilikan tiket; petugas dibatasi penugasan arena.

Migrasi 011 bersifat additif dan wajib diterapkan secara eksplisit sebelum menjalankan release baru; `/readyz` memeriksa tabel ingestion. Konfigurasi `MEDIA_S3_ENDPOINT`, `MEDIA_S3_REGION`, `MEDIA_S3_BUCKET`, `MEDIA_S3_ACCESS_KEY`, `MEDIA_S3_SECRET_KEY` hanya di server cloud. Tanpa konfigurasi storage, CRUD lama tetap berjalan tetapi operasi upload/download media menghasilkan 503. Mode desktop tidak memerlukan endpoint command/heartbeat/WHEP cloud lama.
