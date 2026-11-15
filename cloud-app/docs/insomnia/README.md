# Insomnia - schema 010

Import [cloud-app-mock.json](cloud-app-mock.json) sebagai collection baru. Nama file lama dipertahankan agar tautannya tetap berlaku; isinya sudah diganti menjadi **Kicau Mania - Schema 010**, tanpa akun/password demo, ID seed tetap, endpoint legacy atau media cloud.

Request mengubah API/database nyata. Jalankan satu per satu; jangan **Run All**, karena koleksi juga berisi transisi ronde, pencabutan sesi, logout dan penghapusan. Hapus workspace mock lama dari Insomnia bila tidak digunakan lagi; import baru tidak otomatis menghapus workspace lama.

## Persiapan

1. Backend memakai PostgreSQL schema 010 dan Redis. Jangan menjalankan ulang reset 008 atau seed lama untuk koleksi ini.
2. Base Environment sudah memakai `base_url=https://dev.megaallstararena.com` dan `frontend_origin=https://dev.megaallstararena.com`. Request bisnis menambahkan `/api/v1`, sehingga alamat akhirnya `https://dev.megaallstararena.com/api/v1`. Jangan menambahkan `/api/v1` lagi pada `base_url`. Isi `email` dan `password` ADMIN yang sudah dibuat melalui CLI bootstrap.
3. `frontend_origin` harus sama persis dengan origin yang diizinkan backend. Request auth mengirim header Origin tersebut. Cookie refresh HTTPS bernama `__Host-kicau_refresh` ditangani cookie jar secara otomatis.
4. Biarkan cookie jar aktif. Login/refresh otomatis menerima refresh cookie HttpOnly. Jangan menyalin refresh token ke Bearer atau mencoba membaca JWT; access token bersifat opaque.
5. Isi tanggal `event_date` serta kolom telepon kosong pada request pembuatan akun sesuai data pengujian. Simpan kredensial pada environment privat lokal jika tersedia; jangan mengekspor token/password/cookie yang sudah terisi.

## Urutan alur utama

1. **Health/readiness**, kemudian **POST /auth/login**. Respons sukses menyimpan `access_token` dan `my_user_id`; request privat memakai token itu. Refresh memperbarui token yang sama, logout menghapusnya setelah sukses.
2. **POST /arenas**, lalu **POST /users** untuk peserta. Gunakan request **Buat akun EO** untuk mengisi `owner_id`. ID hasil create disimpan otomatis; tidak perlu mengarang UUID atau memakai seed lama.
3. Buat spesies, burung, event, kategori, gantangan dan ronde. Burung baru memakai pemilik peserta `user_id`; EO event memakai `owner_id`. Kode burung dibuat server. Ronde dibuat PREPARED; status tidak dikirim melalui edit konfigurasi.
4. Buat kriteria pertama dan request **Buat kriteria kedua**, sebelum ronde dimulai. Keduanya mengisi `criterion_id` dan `criterion_2_id` yang dipakai contoh paket nilai.
5. Buat akun petugas dengan `staff_email` dan `staff_phone`. Sebagai ADMIN, tugaskan `staff_role=JURI` pada arena, kemudian tetapkan password awal melalui `/staff/{user}/credentials` memakai `staff_password` minimal 12 karakter. Tambahkan petugas tersebut ke roster `/rounds/{id}/judges/{judge}`. Password awal tidak dapat digunakan untuk mereset password akun yang sudah memiliki password.
6. Buat registrasi. Akun tersimpan dahulu; jika tiket gagal, gunakan kembali `user_id` tersebut. Satu tiket memuat user, arena, ronde, burung, gantangan, biaya dan metode bayar.
7. Untuk CASH, panggil **verify** secara eksplisit. Untuk TRANSFER, ubah payment_method saat membuat tiket, pilih file pada request multipart **POST /registrations/{id}/proof**, lalu verify. Jangan menambahkan header Content-Type multipart secara manual; Insomnia membuat boundary. JPEG/PNG maksimal 5 MiB, dengan pemeriksaan signature. GET proof mengembalikan gambar, bukan envelope JSON.
8. **start** (expected PREPARED), kemudian **begin-judging** (expected RUNNING). Kriteria arena terkunci setelah mulai. Roster/konfigurasi tidak dapat diubah lagi seperti saat PREPARED.
9. Gunakan **Login JURI**. Login ini mengganti token dan refresh cookie aktif; panggil `/judge/rounds`, baca peserta eligible, lalu PUT paket skor. Contoh berisi dua kriteria; sesuaikan array agar mencakup **seluruh** kriteria arena tepat sekali. `score_value` dikirim sebagai string desimal. PUT yang sama digunakan untuk revisi.
10. Login ADMIN kembali sebelum tindakan petugas, lihat recap, lalu **close** (expected JUDGING) setelah semua nilai lengkap. Tidak ada penyimpanan bracket atau pemenang.

Satu cookie jar dan satu `access_token` berarti satu identitas aktif. Jangan mencampur token ADMIN dengan refresh cookie JURI secara manual. Untuk uji beberapa identitas bersamaan gunakan workspace/cookie jar terpisah.

## Operasi tambahan

- **Edit/arsip master:** user dan burung memakai `user_version`/`bird_version`; nilainya diperbarui dari respons sukses. GET detail dahulu sebelum edit. Respons 409 tidak menimpa variabel atau input. DELETE master adalah soft delete. History read-only tetap tersedia melalui ID master setelah diarsipkan; pagination memakai `history_after_version`.
- **Pencarian:** aktifkan parameter `q` pada daftar user/burung, isi `search`. Parameter opsional seperti cursor, event, round, owner dan role awalnya dinonaktifkan; aktifkan sesuai kebutuhan. Pagination memakai `data.next_cursor` yang disalin ke `cursor`.
- **Edit tiket:** PUT hanya sebelum confirmed/VERIFIED dan ketika ronde asal/tujuan masih PREPARED pada arena sama. Gunakan POST cancel untuk pembatalan; tidak ada DELETE registrasi.
- **Diskualifikasi:** reason wajib. Requalify membatalkannya sebelum CLOSED/CANCELLED. Pembayaran tidak berubah. Cancel tiket/ronde adalah tindakan nyata; gunakan data yang memang hendak dibatalkan.
- **Transfer burung:** isi `new_owner_id` akun aktif. Gunakan endpoint transfer khusus dengan versi terkini, bukan PATCH owner_id. Baca ownership/history untuk memeriksa hasilnya.
- **Penghargaan:** request dasar membuat penghargaan eksternal; nama event wajib. Request internal memakai `registration_id` dan `arena_id`, dengan tiket burung confirmed/verified/ACTIVE. ID tiket tidak dapat dipindah saat edit. GET certificate mengembalikan URL setelah otorisasi; GET proof mengembalikan bytes.
- **Profil/sesi:** GET sessions, salin ID sesi ke `session_id`, lalu revoke bila diperlukan. Ganti password mencabut semua sesi; login kembali dengan password baru dan perbarui environment lokal.
- **Penghapusan/pencabutan:** dipisahkan di folder 99. Relasi dapat menyebabkan 409. ADMIN terakhir arena tidak dapat dicabut. Juri dengan ronde berjalan juga tidak dapat dicabut/dinonaktifkan.

## Hasil yang diharapkan

Request normal menguji HTTP 200 dan envelope success sebelum menyimpan ID/versi/token. Error 400/401/403/409/503 tetap terlihat sebagai kegagalan dan tidak mengisi hasil palsu. Saat melakukan pengujian negatif, sesuaikan assertion status pada salinan request. Contoh: akses arena tanpa penugasan -> 403; versi master stale -> 409; skor parsial -> 400 saat JUDGING; kriteria terkunci -> 409; Redis tidak tersedia -> 503. Readiness memerlukan PostgreSQL/schema dan Redis.

Koleksi divalidasi secara struktural terhadap Swagger aktif, termasuk cakupan method/path dan placeholder. Import UI Insomnia serta menjalankan seluruh request dengan akun/perangkat pengguna belum diverifikasi otomatis.

## Regenerasi

Dari root repository:

```powershell
Push-Location cloud-app
go run ./cmd/openapi docs/generated/swagger.json
Pop-Location
python cloud-app/docs/insomnia/generate.py
```

Kontrak lengkap: [API schema 010](../API.md). Konfigurasi layanan/ADMIN pertama: [README backend](../../README.md).
