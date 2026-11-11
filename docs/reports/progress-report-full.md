# Laporan Progres Lengkap Kicau Mania

6 November 2026 · Catatan internal

Versi ringkas untuk dibagikan tersedia terpisah. [Laporan progres ringkas](progress-report.md)

| Area | Posisi saat ini |
| --- | --- |
| `local-app` | REST API media, UI penilaian lokal, penyimpanan offline, dan sinkronisasi sudah diimplementasikan. |
| `cloud-app` | Layanan pertukaran paket, journal perubahan, dan perintah media sudah tersedia dengan SQLite. |
| Database PostgreSQL | Lapisan baca dan contoh query sudah dibuat serta diverifikasi dari dump. Integrasi ke repository Go masih tahap berikutnya. |
| Infrastruktur VPS | Lingkungan dasar VPS sudah disiapkan menurut catatan pemilik proyek. Status deployment layanan belum diverifikasi dalam laporan ini. |

## Local-app dan media arena

- REST API menyediakan daftar kanal, mulai dan hentikan sesi rekaman, daftar rekaman, serta kontrol restream. WebSocket membawa audio PCM dan metrik audio. Endpoint health tersedia untuk memeriksa layanan HTTP.
- MediaMTX menarik sumber RTSP dan menyediakan live WebRTC di LAN. FFmpeg menangani dekode audio, perekaman, dan restream ke tujuan yang dikonfigurasi. Live LAN dan restream dapat dikendalikan secara terpisah.
- Rekaman memakai file lokal dan manifest persisten. Finalisasi menangani file partial, dan upload S3 berjalan di background dengan retry serta object key stabil. Sesi rekaman memiliki lifecycle tersendiri dari ronde penilaian.
- Script persiapan webcam/mikrofon Windows dan sumber sintetis tersedia. Lokasi konfigurasi MediaMTX hasil render berada di direktori runtime.

[Panduan local-app](../../local-app/README.md)

## Penilaian offline dan pemulihan

- UI juri dan operator disajikan oleh server lokal. Operator dapat mengimpor paket JSON atau mengambil paket dari cloud sebelum lomba. Juri menggunakan akun khusus event yang dapat diautentikasi secara lokal.
- Ronde mendukung 24 lapak, penugasan juri, skor bulat 1–10 per kriteria, revisi, dan rekap rata-rata berbobot sama. Penutupan resmi mensyaratkan seluruh penilaian wajib lengkap. Operator dapat membatalkan ronde dengan alasan.
- Browser menyimpan draft dan kiriman tertunda di IndexedDB. Arena menyimpan revisi dan outbox dalam satu transaksi SQLite. ID operasi menangani pengiriman ulang, sementara versi revisi membantu mencegah penimpaan dari perangkat lain.
- Nilai yang sudah commit tersedia setelah restart. Ronde aktif menjadi `INTERRUPTED` untuk ditinjau operator. Backup snapshot SQLite tersedia secara manual, berkala, dan setelah penutupan ronde bila lokasi backup dikonfigurasi.
- Gangguan media atau cloud tidak memblokir penyimpanan nilai. Status media, antrean cloud, dan backup dapat diperiksa secara terpisah.

Panduan offline-first (`local-app/OFFLINE-FIRST.md`; dokumen historis sudah dihapus) · [Alur end-to-end](../../local-app/docs/end-to-end.md)

## Sinkronisasi dan layanan cloud

- Arena mengirim perubahan dari outbox secara bertahap. Cloud memvalidasi identitas engine dan mengakui perubahan setelah commit. Arena menandai perubahan yang telah diakui dan dapat mencoba ulang saat koneksi kembali.
- Cloud exchange menyediakan unggah paket, unduh paket per engine, penyimpanan journal perubahan, serta antrean dan laporan hasil perintah media.
- Perintah remote dibatasi ke start/stop rekaman dan restream. Arena menyimpan hasil perintah sebelum mengirimkannya kembali dan menangani perintah kedaluwarsa serta retry. Otoritas nilai resmi berada di arena.

[Kontrak API cloud exchange](../../cloud-app/README.md)

## Database PostgreSQL

- Dump menjadi acuan untuk 16 tabel bisnis di domain aset, kompetisi, IAM, penilaian, media, dan membership.
- Migrasi `cloud_read` menyediakan 16 view untuk daftar dan detail, satu materialized view untuk ringkasan skor administratif, indeks rekap, dan rollback. Grant dipisahkan untuk pembaca operasional, admin, dan pemelihara rekap.
- Query berparameter dibagi ke enam file sesuai domain. Contoh CRUD memakai tabel sumber untuk mutasi dan view untuk membaca hasil. Rekap di-refresh secara manual setelah batch skor commit.
- Materialized view menghitung jumlah skor masuk, valid, dan invalid, jumlah juri dengan skor valid, serta rata-rata skor valid. Keluaran ini menyediakan dasar untuk rekap administratif sebelum integrasi hasil resmi arena.

[Panduan migrasi dan query PostgreSQL](../../cloud-app/db/README.md)

## Infrastruktur dan pemeliharaan kode

- Lingkungan dasar VPS telah disiapkan untuk hosting komponen cloud.
- Persiapan deployment LAN mencakup resolver DNS lokal, konfigurasi HTTPS melalui Caddy, template Hostinger dan router, serta script build dan startup. Domain, IP, dan tujuan backup dapat dikonfigurasi per lingkungan.
- Struktur `local-app` dibagi menjadi `core`, `mediaengine`, `edgetocloud`, dan `infra`. UseCase arena dipisah per alur, termasuk event, auth, scoring, ronde, query, dan backup.
- Logging mengikuti pola `park-server-v3` dengan Zap, log file JSON, request ID, serta redaksi field sensitif. Peta kode menjelaskan jalur baca developer dan tanggung jawab file.

[Peta kode](../../local-app/docs/code-map.md) · Konfigurasi LAN dan backup (`local-app/OFFLINE-FIRST.md`; dokumen historis sudah dihapus)

## Verifikasi yang sudah dilakukan

| Area | Hasil |
| --- | --- |
| Arena dan cloud exchange | Pada implementasi awal, tes Go dan tes integrasi browser serta dua aplikasi berhasil. Skenarionya mencakup login saat cloud gagal, penyimpanan nilai saat media gagal, restart paksa, backup, dan pengiriman ulang setelah koneksi pulih. |
| Refactor folder dan logging | Build Go berhasil pada revisi tersebut. |
| PostgreSQL | Restore dump, penerapan migrasi, definisi query, contoh CRUD burung, batas masa langganan, grant, perhitungan skor, refresh concurrent, rollback, dan penerapan ulang berhasil pada instance PostgreSQL 18 sementara. |

## Tahap berikutnya

1. Hubungkan repository Go `cloud-app` ke PostgreSQL, dimulai dari daftar dan detail event, lalu lanjutkan ke CRUD event, ronde, dan peserta.
2. Hubungkan pendaftaran dan persiapan event ke pembentukan paket yang dikirim ke arena. Proyeksikan journal perubahan arena ke tabel bisnis PostgreSQL sebelum menyediakan hasil resmi dan publikasi.
3. Verifikasi deployment VPS dan operasional LAN di lingkungan tujuan, termasuk DNS router, sertifikat HTTPS, backup terpisah, perangkat juri, dan sumber kamera sebenarnya.
