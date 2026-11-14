# API bisnis cloud-app

> **Perubahan database 15 November 2026:** skema lokal sudah mengikuti ERD baru melalui 008–009. Backend, query, bootstrap serta seed/koleksi request lama belum kompatibel. Jangan menerapkan seed lama atau menganggap endpoint bisnis siap sebelum adaptasi aplikasi. Lihat [status database](../db/README.md).

Seluruh route bisnis berada pada `/api/v1`. `GET/HEAD /healthz` dan `/readyz` berada di root. Envelope dan pagination mengikuti [README](../README.md).

## Swagger interaktif

Jalankan cloud-app, lalu buka `/api/docs` pada origin API yang sama, misalnya `http://127.0.0.1:8091/api/docs`. Halaman mengarah ke `/api/docs/index.html`. Sesudah commit ini dideploy, alamat VPS adalah `https://dev.megaallstararena.com/api/docs`. Kontrak Swagger 2.0 tersedia pada `/api/docs/doc.json` dan [hasil generator](generated/swagger.json). Dokumentasi mencakup route REST aktif; endpoint pendamping engine-cloud yang belum aktif tidak ditampilkan.

1. Buka **Auth → POST /api/v1/auth/login → Try it out**; isi email, password dan UUID device_id. Akun mock mengikuti [panduan seed](../db/seeds/README.md).
2. Klik **Execute** dan salin `data.access_token` dari response 200.
3. Klik **Authorize**, isi `Bearer <access_token>` lengkap dengan awalan `Bearer` dan satu spasi, lalu **Authorize**. Swagger 2.0 mendeskripsikan Authorization sebagai header apiKey. Request berikutnya memakai akun tersebut; ganti token saat berpindah role.
4. Buka endpoint yang diperlukan, isi parameter/payload, lalu Execute. Response 204 memang tidak berisi JSON. Try it out melakukan operasi nyata pada database yang sedang terhubung.

Bearer tidak disimpan ke localStorage dan hilang ketika halaman dimuat ulang. Cookie HttpOnly login/refresh ditangani browser, dipakai bersama antar-role pada origin yang sama. Jalankan login akun terkait sebelum mencoba refresh; jangan menyalin refresh token ke Authorize. Google membutuhkan Google ID token asli; dokumentasi ini tidak menyediakan Google login widget atau token tiruan.

Untuk Swagger pada localhost, gunakan origin yang konsisten. Misalnya `FRONTEND_PUBLIC_URL=http://127.0.0.1:8091` dan `ALLOWED_ORIGINS=http://127.0.0.1:8091,http://localhost:3000`, lalu restart proses development. Origin halaman Swagger harus diizinkan agar POST/refresh dari browser lolos CORS; konfigurasi produksi tetap memakai HTTPS. Header Origin ditentukan browser, bukan diisi manual pada Swagger.

Swagger UI disediakan oleh `gin-swagger` dan `swaggo/files`; tidak perlu CDN atau container tambahan. JSON kontrak dikirim mentah, bukan envelope bisnis. Handler dokumentasi pada `internal/delivery/rest/apidocs` tidak mengakses UseCase/database.

### Memperbarui dokumentasi dari kode

**Yang diedit:** komentar `@Summary`, `@Param`, `@Success`, `@Failure`, `@Security` dan `@Router` dekat route pada `internal/delivery/rest/`, serta tipe request yang dirujuk. Metadata API dan perintah generator berada pada [generate.go](../generate.go). Untuk request yang sudah memiliki model Go, anotasi memakai tipe tersebut. CRUD generik tetap menerima map; tipe pada `crud/requests.go` mendeskripsikan field create/patch per resource tanpa mengganti validasi UseCase. Response read model yang dinamis tetap memakai `model.Record`.

Dari direktori **cloud-app**, jalankan:

```powershell
go generate .
go build ./...
```

Tidak perlu menginstal executable `swag` global: perintah memakai `go run` dengan versi Swaggo **v1.16.6** yang dipin. Penggunaan pertama membutuhkan internet untuk mengunduh tool, lalu cache Go dapat digunakan lagi. `--parseFuncBody` membaca anotasi di dalam fungsi sehingga handler anonim dan registrasi CRUD generik tetap bisa dipakai.

**Jangan edit** `docs/generated/docs.go` atau `docs/generated/swagger.json`; keduanya hasil otomatis dan ditimpa generator. Commit hasilnya bersama perubahan sumber. CI menjalankan generator lalu menolak perubahan yang hasil dokumentasinya belum diperbarui. Build Docker memakai hasil yang sudah diperiksa CI.

Contoh anotasi dekat registrasi login:

```go
// @Summary Login email/password
// @Tags Auth
// @Accept json
// @Produce json
// @Param request body input.Login true "JSON body"
// @Success 200 {object} response.Response{data=input.Tokens}
// @Failure 401 {object} response.Response
// @Router /api/v1/auth/login [post]
g.POST("/login", func(c *gin.Context) {
    // Parsing request dan pemanggilan UseCase yang sudah ada.
})
```

Untuk endpoint privat, tambahkan `// @Security BearerAuth`. Ini mendokumentasikan kebutuhan token; autentikasi tetap dijalankan middleware, bukan Swaggo. Generator tidak otomatis memahami aturan bisnis atau semua ekspansi route dinamis: route baru tetap memerlukan anotasi dan verifikasi kecocokan. Alur aplikasi, JSON, endpoint bisnis dan aturan database tidak diubah oleh perpindahan dokumentasi ini.

Kecuali route auth publik, kirim `Authorization: Bearer <access_token>`. Request berisi JSON memakai `Content-Type: application/json`; body hanya boleh memuat satu nilai JSON. ID adalah UUID; nilai UUID pada parameter dan field ID dinormalisasi ke bentuk huruf kecil tanpa mengubah password/token. Nomor telepon memakai E.164. Password 12–256 byte. Kredensial dan token tidak dicatat pada log.

## Akun dan sesi

| Metode/path relatif /api/v1 | Payload/fungsi |
| --- | --- |
| POST /auth/register | name, email, phone, password; 202, lanjutkan verifikasi. |
| POST /auth/verify-email/request | email; kirim ulang verifikasi, 202. |
| POST /auth/verify-email | token; 200, token sekali pakai. |
| POST /auth/login | email, password, device_id; 200, access token dan profil. |
| POST /auth/google | credential (Google ID token), device_id, phone untuk akun baru. |
| POST /auth/google/link | Bearer wajib; credential untuk mengaitkan provider ke akun yang sedang login. |
| POST /auth/forgot-password | email; 202 generik, tidak mengungkap keberadaan akun. |
| POST /auth/reset-password | token, password; 200 dan seluruh sesi dicabut. |
| POST /auth/change-password | Bearer; old_password, password; 204 dan seluruh sesi dicabut. |
| POST /auth/refresh | Body `{}`, cookie refresh, Origin frontend yang diizinkan; 200 dan cookie dirotasi. |
| POST /auth/logout | Bearer; 204, sesi dan cookie terkait dicabut. |
| GET /auth/me | Profil, has_password, daftar auth_providers, role global dan penugasan arena sendiri. |
| PATCH /auth/me | name dan/atau phone; email/role tidak dapat diubah melalui profil. |
| GET /auth/sessions | Sesi perangkat sendiri, tanpa hash/token. |
| DELETE /auth/sessions/:id | Cabut sesi milik sendiri; 204. |

Access token disimpan frontend di memori dan dipakai sebagai Bearer. Refresh token hanya pada cookie HttpOnly; browser memakai credentials include untuk request refresh. Frontend harus menyerialkan refresh antar-request/tab: token refresh lama ditolak setelah rotasi. Login ulang device_id yang sama mengganti kredensial slot itu; perangkat keempat ditolak dengan 409.

Email verifikasi memakai halaman `FRONTEND_PUBLIC_URL/verify-email#token=...`; reset memakai `/reset-password#token=...`. Frontend membaca fragment lalu POST token ke API; token tidak dikirim sebagai query URL API. Frontend halaman ini belum termasuk implementasi backend.

Akun Google-only tidak otomatis mempunyai password. Forgot-password tidak mengubah akun Google-only menjadi akun password; pemulihan akun tersebut melalui Google. Akun email/password dapat login tanpa Google. Pengaitan provider tidak dilakukan otomatis berdasarkan email yang sama.

Pengiriman email dilakukan sesudah commit. Kegagalan SMTP dicatat tanpa token dan dapat dipulihkan dengan kirim ulang; tidak ada antrean email persisten pada fase ini. Google/SMTP yang belum dikonfigurasi menghasilkan 503 pada route terkait. Rate limit auth bersifat per proses, mengembalikan 429.

## Pengguna, role dan penugasan

| Metode/path | Hak/fungsi |
| --- | --- |
| GET /users?cursor=&limit= | Superadmin, profil pengguna aman. |
| GET /users/lookup?email=&arena_id= | Admin arena/superadmin; hasil id dan nama untuk penugasan. |
| PATCH /users/:id/active | Superadmin, `{ "is_active": false }`; tidak boleh menonaktifkan dirinya sendiri. |
| GET /roles | Role sistem tetap; bukan CRUD definisi permission dinamis. |
| POST /users/:id/roles | Superadmin, `{ "role": "peserta" }`; role global superadmin/peserta/penjoki. |
| DELETE /users/:id/roles/:role | Superadmin; tidak boleh melepas role superadmin dirinya sendiri. |
| GET/POST /arenas/:arena/assignments | Penugasan; POST user_id dan role. Admin arena dapat menugaskan operator/juri; penugasan admin_arena oleh superadmin. |
| DELETE /arenas/:arena/assignments/:assignment | Hapus penugasan sesuai wewenang. |

Akun yang mempunyai history dinonaktifkan, bukan dihapus. Pengguna baru mendaftar melalui register; belum ada alur undangan email staf. Setelah akun terverifikasi, petugas berwenang memberikan penugasan arena.

## CRUD persiapan

Resource berikut mendukung GET list, POST create, GET `/:id`, PATCH `/:id`, dan DELETE `/:id`. Mutasi membaca representasi view dalam transaksi yang sama. Delete hanya berhasil jika belum direferensikan.

| Resource | Field create; PATCH subset field yang dapat diubah |
| --- | --- |
| /arenas | name; address, timezone, is_active opsional. Create/delete superadmin; admin arena boleh memperbarui arenanya. |
| /owners | name; phone opsional. Peserta otomatis menghubungkan owner ke akunnya; petugas memakai `?arena_id=` untuk master manual. Pengaitan user_id hanya oleh superadmin. |
| /birds | owner_id, name, bird_type; ring_number opsional. Perubahan owner_id melalui aksi transfer. |
| /arenas/:arena/gantangan | number positif; description opsional. |
| /arenas/:arena/engines | name; is_active opsional. Satu engine aktif per arena. |
| /arenas/:arena/devices | engine_id, type, ip_address, rtsp_url; gantangan_id opsional. GET tidak membuka credential RTSP. |
| /arenas/:arena/events | title, date YYYY-MM-DD, location; status opsional. |
| /arenas/:arena/categories | code, name; unik per arena. |
| /arenas/:arena/rounds | event_id, category_id, round_number positif, bird_type, entry_fee positif; session_duration_minutes/judging_duration_minutes opsional. |
| /arenas/:arena/criteria | criteria_name, maksimal 100 kriteria. Daftar terkunci setelah pertama digunakan. |

List memakai `cursor` dan `limit` 1–100. List owner/bird dapat memakai arena_id untuk master manual; hasil tetap dibatasi pemilik/penugasan aktor. UUID diurutkan untuk cursor yang stabil. Daftar peserta penjurian diurutkan nomor gantangan.

`GET /birds/lookup?ring_number=` mengembalikan identitas minimum burung, tanpa kontak pemilik. `POST /birds/:id/transfer` memakai to_owner_id dan reason, hanya superadmin; pemilik terkini dan history diubah atomik. Pendaftaran lomba boleh memakai burung pemilik lain; ini tidak memberikan hak mengubah master burungnya.

## Registrasi dan pembayaran

POST `/registrations`:

```json
{
  "arena_id": "<uuid-arena>",
  "registration_source": "ONLINE",
  "registrant_name": "Nama pendaftar",
  "registrant_phone": "+6281234567890",
  "items": [{"round_id":"<uuid-ronde>","bird_id":"<uuid-burung>"}]
}
```

ONLINE adalah default, memakai akun login sebagai registered_by_user_id. Nama/kontak kosong memakai profil akun. MANUAL memerlukan petugas; kontak pendaftar wajib dan registered_by_user_id opsional. Harga dihitung server dari ronde, bukan dari payload. Ronde harus PREPARED dan tipe burung cocok.

| Endpoint | Perilaku |
| --- | --- |
| GET /registrations?arena_id=&cursor=&limit= | Registrasi milik sendiri atau scope petugas. |
| GET /registrations/:id | Header, items, dan payments sesuai hak akses. |
| PUT /registrations/:id | Ganti items/kontak sebelum pembayaran diajukan; arena/source tetap. |
| POST /registrations/:id/cancel | Batalkan sebelum konfirmasi dan lepas reservasi item. Tidak ada expiry otomatis. |
| POST /registrations/:id/payments | amount, method CASH/TRANSFER, proof_reference untuk transfer. Nominal wajib sama dengan total; CASH hanya petugas. |
| POST /payments/:id/verify | Petugas; payload allocations untuk seluruh item, contoh di bawah. |
| POST /payments/:id/reject | Petugas; menolak pembayaran PENDING. |
| GET /registrations/:id/recordings | Metadata rekaman saja; kosong sampai layanan sistem mengisinya. |

```json
{"allocations":[{"registration_item_id":"<uuid-item>","gantangan_id":"<uuid-gantangan>"}]}
```

Verifikasi pembayaran, alokasi semua gantangan dan konfirmasi header/item terjadi bersama. Ronde masih harus PREPARED. Pengulangan verifikasi yang sama idempoten; alokasi berbeda menghasilkan 409. Registrasi terkonfirmasi tidak dibatalkan melalui endpoint pending; refund belum diimplementasikan. Proof reference adalah referensi transaksi/bukti, tanpa upload atau pengambilan URL otomatis.

## Ronde, penilaian dan hasil

Petugas memanggil POST `/rounds/:id/{action}`:

| Action | Transisi |
| --- | --- |
| start | PREPARED → RUNNING; memerlukan peserta, juri dan kriteria. |
| begin-judging | RUNNING → JUDGING. |
| interrupt | RUNNING/JUDGING → INTERRUPTED. |
| resume | INTERRUPTED → fase asal. |
| cancel | Fase belum CLOSED → CANCELLED, tanpa pemenang. |
| close | JUDGING → CLOSED; body winner_registration_item_id. |

GET/POST `/rounds/:id/judges` membaca/menambah roster; POST memakai user_id. DELETE `/rounds/:id/judges/:assignment` menghapus roster sebelum start. Akun harus mempunyai penugasan juri pada arena. GET `/rounds/:id/recap` memberi petugas rekap langsung, bukan snapshot.

Juri memakai GET `/judging/rounds`, GET `/judging/rounds/:round/participants`, dan GET `/judging/rounds/:round/participants/:item/assessment`. GET assessment yang belum disubmit mengembalikan version 0 dan daftar kriteria.

POST pada path assessment untuk submit awal; PUT untuk revisi:

```json
{
  "operation_id":"<uuid-operasi-baru>",
  "expected_version":0,
  "values":[{"criteria_id":"<uuid-kriteria>","score_value":8}]
}
```

Values wajib memuat seluruh kriteria arena tepat satu kali dengan nilai integer 1–10. POST awal expected_version 0; PUT memakai versi terakhir. Simpan operation_id yang sama untuk retry request yang sama; buat ID baru untuk revisi baru. GET path assessment ditambah `/history` membaca seluruh revisi milik juri tersebut.

Versi stale, fase salah, dan operation ID yang dipakai untuk payload berbeda menghasilkan 409. Penutupan menunggu seluruh nilai peserta/juri/kriteria. Pemenang harus peserta terkonfirmasi dengan total/rata-rata tertinggi; nilai dibandingkan tanpa pembulatan tampilan. Jika seri, operator memilih satu kandidat tertinggi. CLOSED mengunci nilai dan pemenang. Bracket dan peserta lolos otomatis tidak tersedia.

## Laporan snapshot

GET `/arenas/:arena/reports/dashboard` dan `/arenas/:arena/reports/rounds/:round` hanya untuk petugas arena/superadmin. Data berisi snapshot_at, refreshed_at dan items. Snapshot boleh tertinggal; gunakan `/rounds/:id/recap` untuk keputusan operasional. Jangan menentukan pemenang melalui laporan snapshot.

Kode umum: 400 input tidak valid, 401 autentikasi gagal, 403 tidak berwenang, 404 data tidak ada/tidak terlihat, 409 konflik state/versi, 429 rate limit, 503 layanan/config/database belum tersedia, 504 deadline request. Response tidak membawa SQL error mentah.
