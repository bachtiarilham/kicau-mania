# Catatan verifikasi backend bisnis

Pemeriksaan dilakukan pada PostgreSQL 18 sementara dengan data sintetis, role database aplikasi terbatas, SMTP loopback, dan verifier Google fixture. Database aplikasi dan dump sumber tidak dimigrasikan otomatis. Suite tes Go tidak dijalankan.

## Pemeriksaan yang sudah dilakukan

- Build cloud-app, go vet, pemeriksaan arah dependency, dan kompilasi fixture/tes tanpa menjalankan suite.
- Migrasi 001–007 up/down/penerapan ulang pada baseline kosong; statement SQL berparameter diparsing/prepared di bawah role aplikasi.
- Role aplikasi tidak dapat menulis melalui view; role pemelihara dapat menjalankan REFRESH CONCURRENTLY serta memperbarui metadata yang diperlukan.
- 122 pemeriksaan HTTP end-to-end: daftar/verifikasi akun, login password, role/penugasan, master milik sendiri/manual, registrasi multi-ronde, verifikasi pembayaran/allocation, penilaian/revisi, kelengkapan, pengesahan pemenang, perangkat, logout, rotasi refresh dan reset password.
- Race operasi penilaian yang sama menghasilkan satu versi; dua revisi dengan versi awal sama menghasilkan satu keberhasilan dan satu konflik. Penutupan bersamaan revisi tidak membiarkan penilaian baru masuk setelah CLOSED.
- Snapshot laporan tetap lama selama submit/revisi; rekap langsung segera melihat skor terbaru. Refresh setelah commit memperbarui laporan. Total pembayaran tidak berlipat karena item multi-ronde.
- 20 pemeriksaan transport tambahan: HEAD health, 404/405/Allow, preflight, origin ditolak, JSON null/array ditolak, field scalar salah, batas varchar, normalisasi UUID dan perlindungan superadmin dari menonaktifkan/melepas role dirinya sendiri.
- Google port fixture: akun baru, sub stabil, nomor wajib, konflik email, pengaitan eksplisit, verifikasi email tambahan, batas perangkat dan larangan reset lokal untuk akun Google-only.
- Kegagalan transaksi refresh mempertahankan data snapshot dan metadata sebelumnya; advisory lock mencegah dua refresh aktif dan dilepas setelah transaksi selesai. Refresh berikutnya berhasil memulihkan snapshot.

Fixture lokal berada pada direktori runtime yang diabaikan Git. Pemeriksaan arsitektur yang dapat dijalankan kemudian berada pada internal/delivery/architecture_test.go.

## Penerimaan yang masih perlu layanan sebenarnya

- Google Client ID/origin frontend yang benar dan login Google melalui browser, termasuk pengaitan akun.
- SMTP produksi, pengiriman email, serta halaman frontend yang membaca token fragment dan memanggil API.
- Cookie HTTPS/CORS reverse proxy, penyimpanan access token frontend di memori dan koordinasi refresh antar-tab.
- Beban pengguna/arena nyata dan pengukuran interval/waktu refresh laporan.
- Integrasi engine, WHEP/playback, ingest metadata, upload/pengiriman video dan payment gateway/refund merupakan fase berikutnya; tidak disimulasikan sebagai fitur aktif.
