# Verifikasi schema 010 — 15 November 2026

Pengujian memakai PostgreSQL 17.6 port 55439 dan Redis 5.0.14 port 56379 khusus pengujian, dengan schema akhir migrasi 010. Database operasional tidak dibaca, direset, atau dimigrasikan. Setiap suite membuat/menghapus database `kicau_test_*` sendiri. Deployment contoh tetap PostgreSQL 18 dan Redis 7.4; pengujian Docker versi deployment tidak dijalankan pada mesin ini karena daemon/WSL tidak tersedia.

`internal/app/integration_test.go` memuat dan PREPARE seluruh named SQL sebagai role `kicau_cloud_app`, menjalankan skenario SQL rollback, lalu menjalankan middleware/handler HTTP Gin dalam proses tes memakai grant role kicau_cloud_app. PostgreSQL dan Redis tetap layanan nyata. Suite ini lolos dua kali berturut-turut. Smoke test opsional tests/live-contract.mjs juga pernah lolos terhadap listener Go nyata; pengulangannya mengalami timeout/socket error localhost Windows yang intermiten, sehingga koneksi lintas proses tersebut belum dinyatakan stabil pada host ini. Skenario mencakup:

- Bootstrap, login gagal/berhasil, profil, hash token dan TTL Redis, refresh/reuse/concurrent refresh, logout, batas tiga perangkat, penggantian token pada perangkat sama, dan Redis gagal → 503.
- Penolakan akses lintas arena dan OPERATOR memberi role/password atau mengubah akun berprivilege; perlindungan ADMIN terakhir.
- Master no-op, expected_version stale, transfer owner no-op, soft delete dan larangan menulis history melalui role aplikasi.
- Akun, event, ronde, burung, gantangan, tiket duplikat, spesies salah, edit sebelum/sesudah verifikasi, pembayaran tunai/transfer, bukti palsu/lebih besar/PNG valid, verifikasi/penolakan, pembatalan.
- Diskualifikasi/pemulihan tanpa mengubah pembayaran, penolakan setelah CLOSED, roster wajib, juri tanpa tugas, kriteria terkunci.
- Paket skor atomik (kegagalan kriteria terakhir membatalkan nilai sebelumnya), revisi desimal, penutupan tidak lengkap, dan submit bersamaan dengan penutupan.

Tes frontend fixture berada hanya di folder tests frontend; tidak ada mode mock aplikasi. Pemeriksaan browser desktop/mobile/keyboard belum dilakukan karena akses browser preview sebelumnya ditolak. Dashboard kamera dipertahankan, tetapi live ingest, perangkat kamera/mikrofon, deploy produksi, PostgreSQL 18/Redis 7.4 dalam Docker, serta unduh sertifikat penyedia eksternal belum diverifikasi end-to-end pada lingkungan pengguna.

Perintah dan konfigurasi pengujian ada pada [README](../README.md). Hasil: go test ./... dengan integrasi PostgreSQL/Redis dan handler HTTP dalam proses, pnpm check, 31 tes pnpm test, dan pnpm build berhasil. Smoke test socket frontend-ke-Go bersifat opsional dan mempunyai batas verifikasi yang dijelaskan di atas.
