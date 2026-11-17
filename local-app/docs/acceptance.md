# Acceptance NVR nyata

Unit test logika, metadata, keamanan, antrean dan lifecycle: `go test ./...`. Pengujian media tidak membuat sumber video buatan.

```powershell
cd D:/ilham/workdir/wails/kicau-mania-desktop
./scripts/test-native.ps1 -NVRRTSPURL 'rtsp://127.0.0.1:8554/gantangan01' -FFmpeg 'D:/ilham/app/ffmpeg/bin/ffmpeg.exe' -MediaMTX 'D:/ilham/workdir/go/kicau-mania/local-app/bin/mediamtx.exe'
```

Sumber dapat langsung dari NVR atau relay NVR aktif. Gunakan environment KICAU_TEST_NVR_RTSP_URL untuk menghindari credential dalam riwayat command. Tes hanya membaca NVR. Recording memakai port/direktori sementara; pemutusan/restart hanya pada relay pengujian. Tidak mengunggah ke cloud.

Verifikasi 16 November 2026: native libVLC, Start/Stop/finalisasi MKV, HEVC 2304×1296 20 FPS, audio pcm_alaw asli, reconnect relay dan pembacaan rekaman setelah restart engine pengujian lolos. Codec/resolusi/FPS/audio diperiksa dengan ffprobe. Tes penolakan video non-HEVC menggunakan metadata subprocess fixture tanpa encoder.

Latency kamera-ke-layar dan kapasitas 24 channel belum diukur. Storage S3/MinIO nyata belum diuji. Jika NVR tidak tersedia, laporkan acceptance belum dijalankan; jangan menggantinya dengan sumber buatan.

## Cleanup satu kali data testing

Stop/finalisasi sesi, lalu hentikan engine. Dari local-app, `./scripts/clear-testing-data.ps1` menampilkan preview. Tambahkan `-DeleteTestData` untuk menghapus hanya rekaman/manifest/journal/mapping/progres testing dalam data lokal. Script mempertahankan setup.json, pairing.bin dan credential, memeriksa batas path/reparse point serta lock engine. Tidak mengakses database/objek cloud. Tidak dijalankan otomatis saat startup.

Data testing lama dipertahankan atas instruksi pengguna; cleanup tidak dijalankan. Engine operasional sudah dihentikan setelah sesi terminal dan tidak dijalankan ulang. Manifest non-MKV tidak kompatibel dengan versi baru: untuk menjalankan versi MKV-only tanpa menghapus data lama, gunakan DATA_DIR baru dan salin hanya setup.json serta pairing.bin memakai akun Windows yang sama. Jangan menyalin queue/journal lama.
