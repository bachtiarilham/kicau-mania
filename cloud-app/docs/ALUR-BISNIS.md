# Membaca backend cloud-app dari alur bisnis

Mulai dari usecase yang sedang dikerjakan, lalu ikuti `model → handler → UseCase → repository → SQL`. Jangan membaca semua folder sekaligus. [Kontrak API](API.md) menjelaskan request/response; [SQL](../db/README.md) menjelaskan transaksi dan view.

## Peta folder

| Lokasi | Tugas |
| --- | --- |
| `cmd/api` | Memuat env/logger, sinyal proses, dan menjalankan server. |
| `cmd/bootstrap-admin` | Membuat akun awal yang dipilih operator melalui environment, tanpa password bawaan. |
| `config` | Validasi koneksi, origin, proxy, SMTP, Google dan jadwal refresh. |
| `internal/app` | Merakit dependency konkret dan interface; menutup worker sebelum pool. |
| `internal/server` | HTTP listener, timeout dan graceful shutdown. |
| `internal/delivery` | Router Gin dan kelompok route. |
| `internal/delivery/middleware` | Header, request ID, log route, CORS, batas body/context, verifikasi identitas. |
| `internal/delivery/rest` | Handler per domain, helper parsing/response dan adapter CRUD bersama. |
| `internal/auth` | Identitas, login, verifikasi/reset, Google, sesi serta adapter password/SMTP. |
| `internal/iam` | Profil aman, pemeriksaan role dan penugasan arena. |
| `internal/asset` | Arena/gantangan/engine/perangkat, master pemilik/burung dan transfer. |
| `internal/competition` | Persiapan event/ronde, registrasi, pembayaran, lifecycle dan laporan. |
| `internal/judging` | Kriteria bersama arena, assessment peserta, revisi/history dan idempotensi. |
| `internal/core/model` | Record transport-independent, scope aktor, pemeriksaan nilai scalar/UUID/uang. |
| `internal/infra/postgres` | Pool, eksekusi SQL bernama, transaksi dan klasifikasi error DB. |
| `internal/infra/logger` | Logging Zap dan redaksi rahasia. |
| `db/queries` | Satu sumber SQL berparameter yang di-embed. Tidak ada SQL bisnis duplikat di Go. |
| `db/migrations` | Perubahan skema dan rollback manual. |

Model Record dipakai pada resource CRUD agar setiap tabel tidak memerlukan lapisan DTO identik. Field mutasi tetap mempunyai allowlist dan validasi pada UseCase. Alur transaksi kompleks memakai input bertipe seperti Registration, Payment, dan Submit assessment. View membatasi field output; query autentikasi sensitif tidak dikembalikan sebagai DTO.

## 1. Daftar akun dan login password

`auth/model/auth.go` → `delivery/rest/auth/handler.go` → `auth/usecase/account.go` → `auth/repository/auth.go` → `db/queries/auth.sql`.

UseCase menormalisasi email, memeriksa format telepon/password, meng-hash password, lalu menyimpan akun dan role peserta dalam transaksi. Token verifikasi disimpan hash-nya; email dikirim setelah commit. SMTP gagal tidak membatalkan akun pending; tersedia kirim ulang. Token tidak masuk log.

`auth/usecase/session.go` memeriksa password terverifikasi dan status akun, mengunci akun, membersihkan slot kedaluwarsa, menghitung perangkat lain, lalu menyimpan hash access/refresh token. Argon2 dibatasi dua pekerjaan bersamaan agar penggunaan memori terbatas; penantian menghormati context cancellation.

Handler memasang cookie refresh HttpOnly; UseCase tidak mengetahui Gin/cookie. Access token menjadi Bearer request berikutnya. Profil mengembalikan has_password dan nama provider, tanpa hash/sub provider. Middleware hanya meminta verifikasi identitas; otorisasi arena tetap pada UseCase.

## 2. Google, lupa password dan sesi

`auth/usecase/google.go` memakai interface verifier; implementasi berada pada `auth/service/google.go`. ID token diperiksa library Google, audience dari konfigurasi dan issuer yang diterima. Provider disimpan menggunakan sub. Email sama tidak otomatis mengaitkan akun. Akun baru perlu nomor telepon; email pihak ketiga yang tidak authoritative perlu verifikasi aplikasi.

Forgot-password → token RESET_PASSWORD → email → reset sekali pakai → password baru dan semua sesi dicabut. Akun Google-only tidak diam-diam memperoleh password dari jalur reset lokal. Link Google memerlukan sesi akun lama yang sudah login.

Refresh menggunakan compare-and-swap hash lama sehingga hanya satu rotasi berhasil. Frontend perlu menyerialkan refresh antar-request/tab. Logout dan pencabutan perangkat mematikan akses tanpa menunggu expiry token.

## 3. Mengelola master

`delivery/rest/crud/handler.go` menangani bentuk HTTP umum. Instance handler dipasang dengan resource tetap dari router, bukan nama tabel dari browser. Domain asset/competition/judging memeriksa field yang boleh berubah, scope aktor, ownership, fase dan penguncian.

Peserta hanya mengelola owner/burung sendiri. Petugas dapat membuat owner manual dengan provenance arena dan pembuat; master manual tidak otomatis dikaitkan ke akun berdasarkan nama/nomor. Transfer owner hanya superadmin, dengan perubahan pemilik dan history dalam satu transaksi.

## 4. Registrasi sampai peserta terkonfirmasi

`competition/model/workflow.go` → `delivery/rest/competition/handler.go` → `competition/usecase/registration.go` dan `payment.go` → `competition/repository/workflow.go` → `db/queries/competition.sql`.

Pendaftar memilih burung dan ronde. UseCase mengunci arena/ronde, memeriksa fase PREPARED serta tipe burung, mengambil tarif dari database dan menyimpan snapshot. Satu header menampung beberapa item; burung sama boleh ada pada ronde berbeda.

Pembayaran merupakan percobaan terpisah; nominal wajib sama dengan total header. Operator memverifikasi pembayaran dan menetapkan gantangan seluruh item. Semua perubahan commit bersama, sehingga satu alokasi gagal tidak meninggalkan pembayaran VERIFIED atau peserta terkonfirmasi sebagian. Pembayaran yang sudah diajukan membekukan perubahan item; registrasi pending dapat dibatalkan manual.

## 5. Juri menilai dan merevisi

`judging/model/assessment.go` → `delivery/rest/judging/handler.go` → `judging/usecase/assessment.go` → `judging/repository/scoring.go` → `db/queries/judging.sql`.

Urutan submit: lock ronde → periksa role/roster dan operation ID → fase JUDGING → peserta terkonfirmasi → seluruh kriteria → expected version → simpan assessment, skor dan history → simpan hasil operasi → commit.

Satu versi meliputi seluruh kriteria seorang peserta yang dinilai seorang juri. Operation ID sama/payload sama mengembalikan snapshot hasil sebelumnya, termasuk setelah ronde ditutup, tanpa menulis nilai baru. Payload berbeda konflik. Revisi dengan expected version lama juga konflik.

History menyimpan setiap nilai kriteria pada tiap versi. Juri hanya dapat membaca assessment/history miliknya. Daftar peserta memuat nama burung dan nomor gantangan untuk form penilaian, tanpa membuka kontak pendaftar/pemilik.

## 6. Menutup ronde dan memilih pemenang

`competition/usecase/round.go` memakai lock ronde yang sama dengan submit/revisi. Saat lock didapat, semua penilaian yang lebih dahulu commit terlihat dan penilaian baru harus menunggu.

UseCase memeriksa semua nilai wajib, lalu membandingkan total skor secara exact. Karena seluruh peserta harus mempunyai jumlah nilai yang sama dengan bobot sama, perbandingan total ekuivalen dengan perbandingan rata-rata tanpa pembulatan. Operator hanya dapat memilih kandidat tertinggi; seri dipilih dari kandidat tersebut. Winner dan CLOSED di-commit bersama.

Penilaian baru tidak diterima setelah CLOSED. Operator tetap mengatur fase secara manual; durasi 10/15 menit adalah default konfigurasi ronde, bukan timer yang otomatis memulai pertandingan. Ronde tidak memiliki hubungan bracket atau promosi peserta otomatis.

## 7. Laporan dan materialized view

`competition/usecase/reporting.go` memisahkan baca laporan dan pemeliharaan snapshot. `competition/worker/reports.go` memiliki goroutine/ticker/notifikasi; UseCase tidak membuat goroutine.

Snapshot rekap peserta dan dashboard arena di-refresh bersamaan dengan metadata suksesnya dalam transaksi pemeliharaan masing-masing. Advisory lock mencegah refresh bertumpuk lintas instance. Pembacaan laporan menggabungkan metadata dan rows dalam satu SQL statement agar tidak mencampur generasi snapshot.

Laporan boleh tertinggal; rekap aktif dan pemenang membaca view biasa. Kegagalan refresh mempertahankan snapshot sebelumnya. Tidak ada refresh per pengetikan/submission nilai. Worker berhenti sebelum pool ditutup.

## Membaca perubahan berikutnya

Pilih satu workflow dan lihat SQL bernama yang dipanggil repository. Perubahan hak akses, validasi lintas entitas dan transaksi masuk UseCase; perubahan JSON/header masuk delivery; perubahan query masuk SQL. Jangan menambahkan koordinasi repository-ke-repository.

Integrasi engine, playback, penerimaan metadata media dan pengiriman video merupakan fase berikutnya. Tidak ada response sukses palsu untuk endpoint mesin yang belum dibuat.
