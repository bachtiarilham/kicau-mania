# Koleksi Insomnia — schema 010

Import [cloud-app-mock.json](cloud-app-mock.json). Koleksi berisi **146 request**: 39 langkah mock berurutan serta katalog endpoint untuk pengujian manual. Akun dan master fixture dijelaskan pada [panduan seed](../../db/seeds/README.md).

## Import dan environment

1. Di Insomnia pilih **Import → File**, lalu pilih `cloud-app-mock.json` yang terbaru. Pastikan workspace hasil import adalah **Kicau Mania - VPS Dev - Schema 010** dan folder **00 Mock - jalankan berurutan setelah seed 010** muncul. Import ulang diperlukan jika sebelumnya memakai file lama; hindari memilih workspace lama atau duplikatnya.
2. Pilih **VPS Development** atau Base Environment. Keduanya memakai `base_url=https://dev.megaallstararena.com` dan `frontend_origin=https://dev.megaallstararena.com`. Koleksi ini hanya menyediakan target VPS; environment Localhost lama sudah diganti.
3. **Jangan tambahkan `/api/v1` pada base_url.** Path tersebut sudah berada di request. `/healthz` dan `/readyz` memakai root domain.
4. Seed schema 010 harus tersedia pada database tujuan. VPS dev sudah diisi pada 15 November 2026; koleksi ini mengakses VPS, bukan database localhost. Semua akun memakai password `KicauMock2026!`.
5. Biarkan cookie jar aktif. Jangan mengekspor koleksi yang sudah berisi token/cookie pengguna. Kredensial fixture hanya untuk pengembangan.

HTTPS memakai refresh cookie Secure/HttpOnly. Jika hasil import masih menampilkan Localhost atau URL lama, pilih workspace terbaru dan periksa override environment pribadi yang tersimpan di Insomnia.

## Jalankan folder 00 Mock

Jalankan request satu per satu menurut nomor **01–39**. Jangan menjalankan seluruh workspace dengan **Run All**, karena katalog manual juga memuat edit, pembatalan, pencabutan sesi, dan penghapusan.

- 01–14: health/readiness, login delapan role, profil, serta penolakan tanpa token dan akses salah arena.
- 15–22: baca burung/history, daftar dua peserta, verifikasi pembayaran CASH, dan tetapkan dua juri.
- 23–31: mulai ronde dan penjurian, uji penutupan sebelum lengkap, isi tiga kriteria untuk kedua peserta dari kedua juri, revisi, lalu baca rekap.
- 32–39: diskualifikasi/requalify, pembatasan pembacaan tiket, tutup ronde lengkap, dan tolak revisi setelah CLOSED.

After-response script menyimpan token terpisah: `token_admin`, `token_admin_a`, `token_operator_a`, `token_operator_b`, `token_juri_1`, `token_juri_2`, `token_peserta_1`, dan `token_peserta_2`. Login memeriksa ID, email, dan role sebelum menyimpan token. Tiket hasil registrasi disimpan ke `mock_registration_1_id`/`mock_registration_2_id`; tidak perlu menyalin UUID manual.

Request negatif mempunyai assertion statusnya sendiri, misalnya 401/403/409, sehingga respons penolakan yang benar tetap lulus. Jika script gagal, lihat **status HTTP dan body response** terlebih dahulu; variabel hasil tidak diperbarui saat respons tidak sesuai. Saat HTTP 429, tunggu sebelum mengulangi login; proxy membatasi burst request autentikasi. Untuk runner otomatis tambahkan jeda antar-request login, misalnya 12 detik.

Skenario utama mengubah A1 menjadi CLOSED. Seed ulang tidak membukanya kembali. Gunakan database pengujian baru untuk mengulang siklus penuh; A2/B1 tidak dipakai oleh skenario ini.

## Katalog endpoint manual

Folder selain `00 Mock` menyediakan endpoint sesuai Swagger aktif. Login ADMIN mock juga mengisi `access_token` untuk katalog umum. Login generik memakai satu `access_token`; pastikan identitasnya sesuai sebelum operasi petugas atau juri. Periksa payload, versi master, dan ID tujuan sebelum mengirim mutasi.

- GET detail user/burung mengisi versi terbaru; edit dengan versi lama menghasilkan 409. History read-only tersedia melalui ID master.
- Transfer burung memakai endpoint transfer dan `new_owner_id`, bukan perubahan owner langsung lewat edit umum.
- Penilaian mengirim seluruh kriteria arena tepat sekali; contoh fixture berisi **tiga kriteria**, `score_value` berupa string desimal. PUT yang sama dipakai untuk revisi.
- CASH diverifikasi eksplisit. Untuk TRANSFER, pilih file JPEG/PNG maksimal 5 MiB pada request proof. Jangan menentukan Content-Type multipart secara manual; Insomnia membuat boundary.
- Penghargaan, CRUD master, pembuatan ronde/event, reset password, dan pencabutan sesi tersedia sebagai request manual; isi field yang belum sesuai kebutuhanmu.
- Endpoint cloud media/engine yang belum dipasang tidak ditambahkan sebagai respons tiruan.

**Cookie jar tetap dipakai bersama**, walaupun Bearer mock dipisahkan per role. Jalankan refresh/logout segera setelah login akun yang hendak diuji. Access token bersifat opaque; refresh memakai cookie HttpOnly, bukan Bearer. Ganti password dapat mencabut seluruh sesi akun. Jangan menjalankannya di tengah skenario utama.

## Validasi dan regenerasi

Seluruh 146 script request diperiksa sintaksnya dan variabel environment-nya. Guard identitas login diuji dengan identitas benar/salah. Seluruh 39 langkah HTTP lolos terhadap image API aktif dengan database dan Redis sementara; seed ulang mempertahankan ronde CLOSED. Database sementara dibuang setelah pemeriksaan. Import UI Insomnia belum diverifikasi otomatis. Suite tes Go tidak dijalankan.

Generator mengambil endpoint dari Swagger aktif. Dari root repo:

```powershell
Push-Location cloud-app
go run ./cmd/openapi docs/generated/swagger.json
Pop-Location
python cloud-app/docs/insomnia/generate.py
```

Lihat [kontrak API](../API.md) dan [panduan backend](../../README.md). Swagger menjelaskan kontrak endpoint; folder mock menjelaskan urutan penggunaan fixture.
