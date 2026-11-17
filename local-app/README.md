# Local-app: NVR HEVC/H.265 dan MKV

Engine desktop CCTV: setup ONVIF/RTSP, MediaMTX RTSP/TCP, metrik, sesi, MKV dan upload cloud privat. Video wajib HEVC/H.265, audio memakai codec sumber. Live/rekaman tidak memakai encode ulang, resize, perubahan FPS atau fallback konversi.

## Menjalankan

Windows, FFmpeg/ffprobe, MediaMTX 1.15.3 dan NVR nyata dibutuhkan. Dari folder local-app:

```powershell
./scripts/build-local.ps1
./bin/local-server.exe -env .env.portable.example
```

API http://127.0.0.1:8090; relay rtsp://127.0.0.1:8554/{channel}. Konfigurasi awal boleh kosong. Desktop atau [setup API](docs/setup-api.md) memilih interface dan menguji/menyimpan channel. Probe, startup/restore dan reconnect menolak video non-HEVC. DPAPI memerlukan akun Windows penyimpan credential.

Mode LAN memakai nonce localhost. Mode cloud tetap menyediakan command, heartbeat dan metadata. Pairing/mapping/upload desktop memakai credential media terpisah. DNS/HTTPS dan restream terkonfigurasi tetap tersedia; kontainer transport restream mengikuti target, sedangkan file rekaman selalu MKV.

Stop sesi dan tunggu finalisasi sampai READY. Recording menggunakan stream copy; decode PCM hanya untuk metrik. PTS kosong dapat diisi dari DTS tanpa mengubah encoded payload. Audio yang tidak dapat disimpan menghasilkan error. File/manifest non-MKV ditolak. Upload selalu container mkv dan object key .mkv. File lokal tidak dihapus setelah upload; tanpa mapping, upload menunggu.

[Dokumentasi](docs/README.md), [acceptance NVR](docs/acceptance.md), [Insomnia](docs/insomnia/README.md), [kontrak media cloud](../cloud-app/docs/desktop-media-contract.md). Swagger: http://127.0.0.1:8090/api/docs/index.html. Perbarui dengan `go generate .`, jalankan `go test ./...`, lalu build.

Desktop: D:/ilham/workdir/wails/kicau-mania-desktop. Pembersihan satu kali data testing mempertahankan setup/credential/pairing. Tidak ada reset startup atau penghapusan database/objek cloud.

Data testing lama dipertahankan atas instruksi pengguna. Engine operasional dihentikan setelah sesi terminal dan belum dijalankan ulang. Versi baru menolak manifest non-MKV: gunakan DATA_DIR baru bila ingin menyimpan seluruh data lama; salin hanya setup.json dan pairing.bin menggunakan akun Windows yang sama, tanpa queue/journal lama. Tidak ada pemindahan data otomatis.

Template portable sekarang memakai DATA_DIR=./data-nvr dan RUNTIME_DIR=./runtime-nvr. Konfigurasi NVR telah disalin dari data/setup.json; rekaman/metadata lama dalam data/ tetap dipertahankan dan tidak dimuat ke queue baru. Direktori privat baru diabaikan Git.
