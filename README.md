# Kicau Mania

Sistem pengelolaan lomba dan media arena. **Cloud-app menangani data bisnis; local-app menangani media di arena.** Frontend menggunakan API cloud untuk login, registrasi dan penilaian, lalu mengakses engine melalui LAN untuk live dan metrik.

Mulai dari **[indeks dokumentasi](docs/README.md)** dan **[alur integrasi local-app–cloud-app](docs/integration.md)**. Panduan integrasi membedakan fitur yang tersedia dari endpoint cloud pendamping engine yang belum aktif.

## Bagian sistem

| Bagian | Tanggung jawab | Mulai menjalankan |
| --- | --- | --- |
| Local-app | Setup kamera/NVR, live LAN, analisis audio, sesi media, recording, restream dan upload. | [README local-app](local-app/README.md) |
| Cloud-app | Login email/password atau Google, arena, peserta, registrasi/pembayaran, penjurian dan hasil di PostgreSQL. | [README cloud-app](cloud-app/README.md) |
| Frontend | Tampilan operator/juri; menghubungkan API bisnis cloud dengan API/media LAN. | [Alur akses dan batas integrasi](docs/integration.md) |

Aplikasi backend dijalankan dari direktori masing-masing; tidak ada satu perintah startup bersama pada root repo. Local-app menyediakan mode LAN mandiri tanpa login cloud. Mode cloud dengan tiket playback masih memerlukan implementasi endpoint pendamping pada cloud-app; perpindahan fase ronde saat ini belum otomatis mengendalikan media.

## Panduan proyek

- [Dokumentasi local-app](local-app/docs/README.md): alur media, peta kode, setup API dan penerimaan perangkat.
- [Panduan bisnis cloud](cloud-app/docs/ALUR-BISNIS.md) dan [API cloud](cloud-app/docs/API.md).
- [Database, migrasi dan query](cloud-app/db/README.md) serta [ERD](cloud-app/db/ERD.md).
- [Runbook deployment](cloud-app/deployment/devops_runbook.md) dan [belajar deployment](cloud-app/deployment/BELAJAR-DEPLOYMENT.md).

[Indeks dokumentasi](docs/README.md) juga memuat catatan kerja, laporan historis, dan template referensi. Semua file `.txt` hasil diskusi tetap berada di root dengan nama dan isi aslinya; catatan tersebut bukan pengganti status implementasi pada panduan aktif.
