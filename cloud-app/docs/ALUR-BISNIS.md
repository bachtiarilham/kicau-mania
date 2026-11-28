# Alur bisnis dan peta kode schema 010

1. Bootstrap ADMIN pertama melalui CLI, login, lalu buat arena. Pembuat arena mendapat ADMIN hanya di arena itu.
2. ADMIN membuat akun operator/juri, menugaskan role arena, lalu menetapkan password awal. Petugas mengganti password melalui profil.
3. Petugas membuat atau mencari peserta, spesies dan burung. Peserta biasa tidak otomatis mendapat login. Master bersama dapat diedit petugas; setiap edit/arsip memakai expected_version dan context actor. Trigger menulis snapshot berbeda saja; history tidak dapat diubah melalui API.
4. Buat EO (akun is_eo), event, kategori tiket, gantangan dan konfigurasi ronde.
5. Daftarkan akun → burung → ronde → gantangan. Bila akun baru sukses tetapi tiket gagal, frontend mempertahankan akun terpilih untuk percobaan berikutnya.
6. Tunai diverifikasi eksplisit; transfer unggah bukti terlebih dahulu. Tiket confirmed/verified menjadi peserta ronde. Edit tiket hanya sebelum terkonfirmasi dan saat kedua ronde PREPARED dalam arena sama. Batalkan tiket melalui aksi, bukan hard delete.
7. Tetapkan kriteria dan roster juri. Mulai ronde mengunci kriteria arena. Juri yang masih mempunyai ronde berjalan tidak dapat dicabut/dinonaktifkan.
8. Mulai penjurian. JURI hanya mengirim nilai untuk peserta eligible pada ronde tugasnya; seluruh kriteria disimpan atomik. Revisi mendukung desimal.
9. Diskualifikasi membutuhkan alasan dan hanya memengaruhi tiket pada ronde itu. Pemulihan tersedia sebelum CLOSED/CANCELLED. Pembayaran tetap utuh.
10. Tutup ronde setelah rekap langsung menyatakan semua nilai lengkap. Penutupan dan submit memakai round lock yang sama. Tidak ada pemenang atau bracket disimpan.
11. Transfer burung melalui tindakan khusus dan expected_version; catat history kepemilikan dan master. Penghargaan internal mengacu tiket yang sah, eksternal menyimpan nama event/penyelenggara.

## Lapisan aktif

| Lokasi | Tanggung jawab |
|---|---|
| internal/app | Wiring schema 010, PostgreSQL/Redis dan readiness; tidak memigrasi database. |
| internal/login/usecase | Login, sesi PostgreSQL, refresh dan logout. |
| internal/login/service | SHA-256 token Redis, TTL, rotasi Lua atomik dan fail closed. |
| internal/workflow/usecase | Scope arena/role, expected_version, transaksi dan aturan lomba. |
| internal/workflow/repository | Adapter port data ke named SQL; tidak menyimpan SQL inline. |
| db/queries | Satu sumber SQL; integration.sql menambahkan query aplikasi yang belum tersedia. |
| internal/delivery/rest/workflow | Parsing JSON/multipart, cookie, Origin, DTO transport dan envelope. |
| cmd/openapi | Menghasilkan Swagger dari route aktif. |

Urutan mutasi lomba: arena → ronde terurut bila lebih dari satu → registrasi → burung. Paket skor memakai ronde → registrasi; tidak mengambil lock arena setelah round lock. Lock arena menserialisasi perubahan kriteria/roster/status. Mutasi master memakai lock versi. Konflik deadlock/constraint/version dipetakan ke 409; input form tidak dibuang. Tidak ada worker materialized-view pada jalur keputusan resmi.

Lihat [kontrak API](API.md) dan [hasil verifikasi](VERIFIKASI-BACKEND.md). Kode domain lama yang tidak di-wire bukan referensi endpoint aktif.
