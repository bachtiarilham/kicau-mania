# Local-app: engine media arena

Hubungan kedua aplikasi dijelaskan pada [panduan integrasi proyek](../docs/integration.md); indeks lintas aplikasi ada di [dokumentasi proyek](../docs/README.md).

Local-app menyediakan live WebRTC melalui LAN, restream, pengolahan audio/metrik, rekaman MP4, dan upload S3. Frontend disediakan terpisah; login, registrasi dan penilaian menjadi tanggung jawab cloud-app.

Mulai membaca dari **[indeks dokumentasi](docs/README.md)**. Panduan developer mengikuti urutan [end-to-end](docs/end-to-end.md) → [media engine](docs/media-engine.md) → [peta kode](docs/code-map.md).

## Prasyarat

- Development: Go 1.25+, FFmpeg/ffprobe dengan libx264, libopus dan AAC, serta MediaMTX v1.15.3.
- Setup LAN portable: Windows; konfigurasi disimpan memakai DPAPI akun Windows yang menjalankannya. Paket sudah membawa executable dan binary media, sehingga mini PC tidak memerlukan Go atau Docker.
- Kamera/NVR RTSP yang dapat dijangkau mini PC; ONVIF membantu discovery bila perangkat mendukungnya. Kapasitas 24 channel tetap memerlukan uji perangkat.

## Menjalankan

Dari direktori `local-app`, untuk development mode LAN:

```powershell
Copy-Item .env.portable.example .env
# Sesuaikan FFMPEG_BIN, FFPROBE_BIN dan MEDIAMTX_BIN dengan lokasi binary.
go run ./cmd/api -env .env
```

Untuk paket portable, jalankan `Start-local-app.cmd`. HTTP tersedia sebelum NVR dikonfigurasi. Lanjutkan [setup melalui API](docs/setup-api.md); penyimpanan konfigurasi tidak otomatis memulai media. Mulai/Stop dilakukan melalui sesi media, lalu pantau sampai finalisasi selesai.

Mode cloud memakai [.env.example](.env.example) dan memerlukan konfigurasi sumber serta otorisasi cloud. Detail komunikasi tersedia pada [kontrak cloud](docs/cloud-contract.md). Pengujian webcam/sumber sintetis menggunakan [.env.webcam.example](.env.webcam.example) dan [script pengujian](scripts/start-webcam-test.ps1).

## Konfigurasi minimum

| Env | Fungsi |
| --- | --- |
| `ENGINE_ACCESS_MODE` | `lan` untuk setup lokal; `cloud` untuk otorisasi playback melalui cloud. |
| `HTTP_ADDR` | Listener API; contoh portable `0.0.0.0:8090`. |
| `FFMPEG_BIN`, `FFPROBE_BIN`, `MEDIAMTX_BIN` | Lokasi binary media. |
| `DATA_DIR`, `RUNTIME_DIR` | Rekaman/journal/config terenkripsi dan file runtime. |
| `ALLOWED_ORIGINS`, `MEDIA_READ_CIDRS` | Origin frontend dan subnet pembaca LAN. |
| `API_TOKEN` | Kredensial layanan, bukan token untuk penonton; boleh kosong khusus mode LAN agar dibuat per proses. |
| `CHANNELS_JSON` | Mode LAN boleh mulai dengan `[]`; sumber kemudian disimpan melalui API setup. |

Referensi environment, kontrol localhost, domain/HTTPS dan konfigurasi NVR dijelaskan pada [panduan setup](docs/setup-api.md). Jangan mengekspos mode LAN ke internet. Rahasia/env privat tidak masuk Git.

## Swagger

Setelah server berjalan, buka **[Swagger local-app](http://localhost:8090/api/docs/index.html)** pada mini PC; sesuaikan port dengan `HTTP_ADDR`. Kontrak JSON tersedia di `/api/docs/doc.json`, dan aset Swagger tertanam dalam executable.

Untuk setup LAN, jalankan `GET /api/v1/setup/bootstrap`, lalu masukkan `data.nonce` ke **Authorize > LocalControl**. Setup tetap hanya dapat digunakan melalui localhost. Try it out mengirim request nyata; WHEP/WebSocket membutuhkan client media tersendiri untuk playback atau grafik.

## Dokumentasi dan verifikasi

[Indeks dokumentasi](docs/README.md) menjelaskan urutan belajar, kontrak cloud, penerimaan media, dan cara memperbarui Swagger. Pemeriksaan dasar: `go build ./...`. Suite tes tidak dijalankan otomatis; kesiapan operasional memerlukan [penerimaan media pada perangkat arena](docs/acceptance.md).
