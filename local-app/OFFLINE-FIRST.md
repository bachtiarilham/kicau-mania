# Operasional arena offline-first

Cloud menyiapkan event, peserta, kriteria, dan akun juri khusus event. Arena mengimpor satu snapshot sebelum lomba; selama event, arena adalah satu-satunya penerima nilai resmi. Internet, cloud, kamera, restream, dan S3 tidak berada pada jalur penyimpanan nilai. Pendaftaran dan CRUD peserta di cloud belum dibuat oleh implementasi ini; `cloud-app` menyediakan fondasi pertukaran paket, perubahan, dan perintah media.

## Pemisahan tanggung jawab

- `internal/core/arena`: model penilaian, UseCase, repository SQLite, handler dan frontend lokal, worker background.
- `internal/edgetocloud/synchronization`: UseCase pengiriman outbox/perintah dan HTTP client cloud.
- `internal/edgetocloud/operation/usecase`: koordinasi perintah media ke UseCase recording/restream.
- `internal/edgetocloud/exchange/model`: kontrak pesan pertukaran tanpa dependency infrastruktur.
- `internal/infra/network/dns`: resolver nama arena dari konfigurasi lokal tanpa WAN.
- `internal/app`: perakitan dependency; `internal/server`: lifecycle modul independen.

Handler hanya parsing/auth/context, memanggil UseCase, dan mengirim DTO. Transisi ronde, validasi paket/nilai, perhitungan, serta koordinasi berada di UseCase. Repository hanya mengurus persistence. Tidak ada repository yang memanggil repository lain.

Frontend bawaan `/arena/` di-embed di executable, tanpa CDN atau asset cloud. Perangkat baru tetap bisa mengunduh UI dan login dari server arena ketika WAN sudah mati. Frontend ini belum merupakan seluruh website pendaftaran cloud; asset yang sama dapat dipakai pada deployment web berikutnya.

## Menjalankan demo di komputer sendiri

Jalankan dari `local-app` dengan Go 1.25+ dan PowerShell 7. Untuk sekadar menguji penilaian, sumber kamera boleh tidak tersedia; konfigurasi tetap membutuhkan sedikitnya satu entri `CHANNELS_JSON`.

```powershell
pwsh -NoProfile -File ./scripts/start-webcam-test.ps1 -InitEnv
pwsh -NoProfile -File ./scripts/build-local.ps1
./bin/event-package.exe -demo -out ./runtime/demo -judges 2 -rounds 1
./bin/local-server.exe -env .env.webcam
```

Buka `http://127.0.0.1:8090/arena/`. Masuk operator menggunakan `API_TOKEN` dari env, impor `runtime/demo/event.json`, lalu mulai ronde. Juri menggunakan username/password dari `runtime/demo/judge-credentials.csv`. Generator `-demo` menghasilkan 24 peserta sintetis dan hash bcrypt; jangan gunakan data demo sebagai data lomba asli. Kedua file bersifat privat, khususnya CSV password. Kanal demo bernama `lapak01`–`lapak24`; sumber webcam contoh bernama `webcam01`, sehingga demo penilaian ini tidak otomatis memiliki 24 video. Sesuaikan snapshot dan katalog untuk uji kamera sebenarnya.

Frontend membedakan:

1. **Draft perangkat**: IndexedDB browser; belum merupakan nilai resmi.
2. **Tersimpan di server arena**: transaksi SQLite telah commit, termasuk revisi dan outbox.
3. **Tersinkron cloud**: cloud telah menyimpan dan mengirim acknowledgment; antrean lokal baru ditandai selesai setelah itu.

Gangguan request ke arena mempertahankan kiriman di browser dan melakukan retry dengan `operation_id` yang sama. Konflik revisi ditampilkan untuk diperiksa; browser tidak menimpa revisi lebih baru secara diam-diam. Draft bukan pengganti server arena saat PC atau WiFi mati. HTTPS LAN diperlukan untuk fitur browser pada perangkat lain; HTTP loopback hanya untuk pengembangan.

## Aturan penilaian

Setiap ronde memiliki tepat 24 lapak dan daftar juri tetap. Setiap juri mengisi seluruh kriteria untuk setiap peserta dengan angka bulat 1–10. Semua juri/kriteria berbobot sama; rekap adalah rata-rata nilai terakhir yang diterima. Seluruh revisi disimpan dan dapat diekspor operator.

Ronde berjalan `PREPARED → ACTIVE → CLOSED`, atau dibatalkan dengan alasan. `CLOSED` hanya diperbolehkan jika seluruh nilai wajib lengkap. Juri yang meninggalkan arena tidak dihapus dari roster; operator membatalkan ronde bila tidak dapat dilengkapi. Revisi hanya diterima saat `ACTIVE`. Setelah aplikasi restart, ronde yang tadinya aktif menjadi `INTERRUPTED`; nilai tetap ada dan operator memilih lanjutkan, tutup bila lengkap, atau batalkan. Gangguan media hanya ditampilkan sebagai status; keputusan ronde tetap pada operator.

Paket menyertakan `id`, `name`, `version`, `arena_id`, `engine_id`, `judges` (id/username/password_hash), `criteria`, dan `rounds` (id/name/judge_ids/participants dengan id/name/lapak/channel_id). Lihat file demo untuk format lengkap. Hash password harus bcrypt cost 10–16. Paket harus cocok dengan `ARENA_ID`/`ENGINE_ID`. Versi lebih baru hanya boleh mengganti event yang belum pernah dimulai. Sesi terikat versi paket. Login juri lokal tidak memerlukan password akun cloud.

## Deployment LAN: Hostinger dan Archer C6

Nilai contoh ada di `.env.lan.example`; seluruh domain/IP/token dapat diubah di `.env.lan`. Tidak ada script yang mengubah router, firewall, atau DNS publik otomatis.

```powershell
# Persiapan online satu kali: build executable dan proxy ber-plugin Hostinger.
pwsh -NoProfile -File ./scripts/build-local.ps1
pwsh -NoProfile -File ./scripts/build-proxy.ps1
# Membuat .env.lan dari sumber tanpa menimpa konfigurasi yang sudah ada.
pwsh -NoProfile -File ./scripts/configure-lan.ps1 -SourceEnv .env.webcam
# Edit .env.lan, jalankan configure lagi setelah perubahan.
pwsh -NoProfile -File ./scripts/start-lan.ps1
```

Konfigurasi contoh:

| Variabel | Nilai / tujuan |
| --- | --- |
| `FRONTEND_ORIGIN` | `https://dev.megaallstararena.com` |
| `LOCAL_PUBLIC_URL` | `https://lan-dev.megaallstararena.com` |
| `LAN_SERVER_IP` | `192.168.2.151`, IP tetap mini PC |
| `LAN_READ_CIDR` | `192.168.2.0/24`, sesuaikan subnet sebenarnya |
| `HOSTING_PROVIDER`, `DNS_PROVIDER` | `hostinger`; builder DNS-01 saat ini mendukung Hostinger |
| `ROUTER_VENDOR`, `ROUTER_MODEL` | metadata deployment `tp-link`, `archer-c6` |
| `ACME_EMAIL`, `HOSTINGER_API_TOKEN` | dipakai Caddy untuk DNS-01 dan renewal |
| `BACKUP_DIR` | path di disk fisik/backup share terpisah |

Reservasikan IP mini PC di DHCP router. Di Hostinger, siapkan record A subdomain lokal ke IP privat arena. Sertifikat HTTPS diperoleh melalui DNS-01 ketika internet masih tersedia, lalu tersimpan di `DATA_DIR/caddy`; siapkan/cek sertifikat sebelum event dan beri waktu renewal saat online. Token Hostinger tetap di `.env.lan` dan environment child Caddy, tidak disalin ke env aplikasi atau ditanam di Caddyfile.

DNS lokal pada mini PC melayani domain arena di UDP/TCP 53 tanpa internet. Atur DNS DHCP LAN router ke IP mini PC, lalu reconnect perangkat untuk menerima lease baru. Periksa menu sesuai firmware Archer C6; jangan mengandalkan DNS WAN router untuk distribusi DNS ke klien. Jangan menambahkan resolver publik sebagai DNS alternatif klien bila jawaban domain arena wajib tersedia offline. Private DNS/DoH manual pada perangkat dapat melewati resolver LAN; pastikan konfigurasi klien menggunakan DNS arena. Matikan client/AP isolation agar perangkat dapat menghubungi mini PC.

Izinkan inbound pada profil jaringan privat: TCP 443, UDP/TCP 53, dan UDP 8189 dari subnet arena. API 8090, RTSP 8554, dan HTTP MediaMTX 8889 tetap loopback. Tidak perlu port forwarding internet. Caddy meneruskan UI/API dan WHEP pada origin HTTPS yang sama; ICE memakai alamat lokal tanpa ketergantungan STUN eksternal. Streaming lokal dan restream adalah dua jalur terpisah; atur target restream di `RESTREAM_TARGETS_JSON`.

Uji dengan perangkat yang belum pernah membuka aplikasi: sambungkan WiFi, putus WAN router, buka domain lokal, login, kirim nilai, dan lihat live. Cache browser bukan syarat akses perangkat baru. IP/subnet contoh harus disesuaikan agar tidak bentrok dengan jaringan sebenarnya. Script startup menggunakan binary yang sudah dibangun sehingga tidak perlu download dependency ketika offline.

## Komunikasi cloud

Lihat `cloud-app/README.md` untuk menjalankan penerima. Isi konfigurasi berikut di `.env.lan` atau env aplikasi:

```dotenv
CLOUD_SYNC_URL=https://CLOUD_API/api/v1/engines/engine01/changes
CLOUD_COMMANDS_URL=https://CLOUD_API/api/v1/engines/engine01/commands
CLOUD_PACKAGES_URL=https://CLOUD_API/api/v1/engines/engine01/events
CLOUD_ENGINE_TOKEN=TOKEN_MESIN_UNIK_MINIMAL_32_KARAKTER
```

Semua koneksi diawali arena ke cloud; arena tidak membutuhkan inbound internet. Jika URL kosong atau cloud mati, nilai tetap diterima dan outbox tetap tersimpan. Sinkronisasi/perintah dipoll setiap 5 detik dengan timeout terbatas. Pengambilan paket cloud hanya atas aksi operator sebelum event, tidak pada jalur startup/login/submit. Acknowledgment divalidasi terhadap batch yang dikirim dan retry idempotent.

Cloud hanya boleh mengirim `restream.start`, `restream.stop`, `recording.start`, atau `recording.stop`, dengan identitas core/arena/engine dan masa berlaku. Hasil eksekusi disimpan lokal sebelum dikirim; retry laporan tidak menjalankan perintah yang sudah selesai. Rekonsiliasi desired state mencegah duplikasi setelah restart. Cloud tidak bisa mengirim perintah menutup ronde penilaian atau mengubah nilai. Endpoint `/api/v1/media/recording-sessions` membantu membedakan sesi rekaman dari ronde penilaian; endpoint media lama `/api/v1/rounds` tetap tersedia.

## Durabilitas dan backup

Database `DATA_DIR/arena.sqlite` memakai WAL dan `synchronous=FULL`. Nilai, revisi, dan outbox disimpan dalam satu transaksi; response sukses dikirim setelah commit. Jaminan ini tetap bergantung pada filesystem/perangkat penyimpanan yang menghormati flush; UPS membantu menjaga operasional ketika listrik padam. S3 SDK baru diinisialisasi ketika upload dibutuhkan, sehingga credential provider/cloud tidak menghalangi startup penilaian.

Isi `BACKUP_DIR` pada penyimpanan terpisah. Snapshot konsisten dibuat setiap 5 menit, saat ronde ditutup, atau tombol Backup operator. Snapshot tidak otomatis dipangkas; pantau kapasitas dan atur retensi/copy eksternal sesuai operasional. UI menampilkan backup belum dikonfigurasi/gagal. File backup mengandung akun hash, sesi, nilai, dan outbox; jaga aksesnya.

Untuk restore: hentikan aplikasi, simpan salinan direktori data lama beserta WAL/SHM, kemudian gunakan direktori data baru dengan snapshot bernama `arena.sqlite`. Jangan mencampur snapshot dengan WAL lama. Pertahankan `ARENA_ID`/`ENGINE_ID`, konfigurasi privat, rekaman/manifest, serta cache sertifikat Caddy. Jalankan kembali dan verifikasi status ronde serta nilai. Nilai sesudah snapshot terakhir tidak dapat dijamin kembali bila seluruh SSD rusak. Implementasi ini bukan server cadangan otomatis.

## API dan verifikasi

Prefix API penilaian `/api/v1/arena`. Public: `/events`, `/status`, `/login`; sesi juri HttpOnly/SameSiteStrict: `/me`, `/me/status`, `/logout`, `/rounds/:round/scores`. Operator Bearer `API_TOKEN`: `/operator/events`, `/operator/events/:event`, `/operator/events/:event/revisions`, `/operator/events/:event/rounds/:round/transition`, `/operator/packages/pull`, `/operator/sync`, `/operator/backup`. Cookie Secure otomatis saat `LOCAL_PUBLIC_URL` HTTPS. Token operator dan token mesin cloud berbeda dan tidak digunakan sebagai kredensial juri.

```powershell
go test ./...
go build ./...
go vet ./...
```

Pengujian mencakup 24 lapak, semua nilai wajib, rata-rata, revisi, konflik dan retry bersamaan, privacy juri, rollback transaksi, migrasi history, proses mati setelah commit, restart/interupsi, restore backup, outbox offline, dan perintah media. Kontrak mediaengine/media/HTTP/WebSocket lama tetap diuji.

Tes integrasi opsional (Node.js, Edge Windows; seluruh data uji di `runtime/browser-check`):

```powershell
go build -o runtime/browser-check/arena.exe ./cmd/api
go build -o runtime/browser-check/event-package.exe ./cmd/event-package
./runtime/browser-check/event-package.exe -demo -out runtime/browser-check/demo -judges 2 -criteria c1,c2
# Dari cloud-app: go build -o ../local-app/runtime/browser-check/cloud.exe ./cmd/api
npm install --prefix runtime/browser-check playwright@1.56.1 --no-package-lock --ignore-scripts
node scripts/tests/offline-browser.cjs
node scripts/tests/cloud-exchange.cjs
```

Tes browser memakai server nyata, browser baru, cloud gagal, media binary hilang, request LAN gagal sementara, restart server, dan sinkronisasi ulang. Tes kedua memakai dua binary/database lokal dan cloud. Tes otomatis belum menggantikan uji lapangan 24 feed, perangkat juri nyata, DNS/TLS/router, putus WAN, daya, serta ingest YouTube/S3 sebenarnya.
