# Penerimaan lifecycle media sesi

Suite tes otomatis tidak dijalankan. Build dan kompilasi fixture terpisah dari penerimaan media. Gunakan DATA_DIR/RUNTIME_DIR, port, token, serta bucket pengujian tersendiri; jangan memakai sesi operasional.

## Pemeriksaan yang sudah dilakukan

Pada Windows, sumber RTSP sintetis H.264/AAC diterbitkan melalui MediaMTX, lalu dibaca engine dengan FFmpeg dan ffprobe nyata. Endpoint cloud digantikan server fixture loopback; ini bukan implementasi endpoint cloud produksi.

| Skenario | Hasil percobaan |
| --- | --- |
| Mulai, menerima PCM WebSocket, lalu Stop | 202 → RUNNING → STOPPED; WebSocket tertutup, satu MP4 A/V menjadi READY. |
| Memulai sesi lain ketika sesi masih aktif | 409. |
| Source putus dan diterbitkan kembali | RETRYING → RUNNING; dua segmen digabung menjadi satu MP4 READY; satu gangguan tercatat. |
| Stop berulang | 202 idempoten. |
| Mematikan proses engine secara paksa, kemudian restart | INTERRUPTED tanpa auto-resume; segmen berhasil diselamatkan menjadi READY. ID lama ditolak; ID baru dapat mulai dan Stop. |
| Jalur POST restream lama | Tidak tersedia untuk mutasi (405). |

Percobaan restart menemukan buffering output yang dapat meninggalkan hanya header MP4 saat crash cepat. Pipeline kini memakai flush packet dan durasi fragment satu detik; percobaan restart setelah perubahan berhasil. Kehilangan fragment terakhir ketika crash tetap mungkin dan dicatat sebagai ketidakpastian pemulihan.

Tidak ada pengujian browser WHEP nyata, jaringan LAN/HTTPS arena, target restream eksternal, atau bucket S3 produksi dalam pemeriksaan ini.

## Langkah penerimaan operasional

1. Siapkan sumber RTSP yang memiliki video dan audio. Untuk perangkat yang belum terpasang, gunakan `scripts/start-webcam-test.ps1 -Synthetic`; script hanya menerbitkan sumber, bukan memulai sesi engine. Jalankan engine terpisah dan kirim POST `/api/v1/media/sessions`. Pastikan `FFMPEG_BIN`, `FFPROBE_BIN`, dan `MEDIAMTX_BIN` dapat dijalankan.
2. Buka GET status sampai RUNNING. Pastikan satu pipeline FFmpeg per channel, PCM mono 16 kHz tersedia, dan seluruh restream terpilih terlihat. Tidak ada pipeline/audio otomatis sebelum Mulai.
3. Gunakan frontend HTTPS dan grant cloud nyata untuk WHEP. Periksa audio dan video benar-benar terdengar/terlihat di browser, ICE LAN, CORS, izin jaringan lokal, sertifikat, dan codec sumber. Recording AAC tidak otomatis mengubah codec audio live kamera menjadi codec yang didukung browser.
4. Tekan Stop. Poll sampai STOPPED; video/audio player dan WebSocket harus terputus, proses media sesi harus keluar. Buka sesi baru dengan ID baru dan ulangi. Upload/heartbeat/metadata harus tetap bekerja setelah Stop.

## Matriks kasus lanjutan

| Kasus | Yang harus diamati |
| --- | --- |
| Payload invalid, channel/target asing, target duplikat | Ditolak sebelum pekerjaan media dimulai. |
| Channel sudah tidak tersedia ketika Mulai diterima | RETRYING/DEGRADED; server tetap hidup dan mencoba kembali 5 detik. |
| Satu dari beberapa channel putus/pulih | Channel lain tetap recording/live; hanya channel terdampak membuat segmen baru. |
| Pembaca PCM/metrik lambat | Memori antrean terbatas; frame boleh hilang, file recording terus bertambah. |
| Target restream mati/pulih | Retry restream; recording dan live lokal tetap berfungsi. |
| POST WHEP berbarengan Stop | POST yang telah di-admit didrain lalu ditutup; admission baru ditolak. |
| DELETE upstream gagal/timeout | MediaMTX ditutup sebagai fallback; player lama berhenti, supervisor memulihkan server. |
| Subscription audio/metrik berbarengan Stop | Koneksi lama tertutup dan tidak terbawa ke generasi sesi baru. |
| Cloud terputus | Mode cloud: recording terus berjalan, playback baru ditolak, pelaporan diulang. Mode LAN: playback tidak bergantung cloud dan tetap dibatasi CIDR/sesi. |
| Restart saat STARTING/RUNNING/STOPPING/FINALIZING | INTERRUPTED, tanpa auto-start; sumber diperiksa, hasil READY atau FAILED dengan alasan. |
| Codec berubah antarpercobaan | Finalisasi FAILED, sumber segmen tetap ada; tidak melakukan transcoding atau membuang bagian secara diam-diam. |
| Segmen rusak/hasil final tidak valid | Tidak READY dan tidak diupload; alasan dapat dibaca pada status/daftar rekaman. |
| Ruang disk melewati batas/durasi habis | Guard meminta Stop; finalisasi selesai atau gagal dengan alasan. |
| Upload gagal lalu aplikasi restart | NextAttempt dan Attempts tetap; object key stabil; retry memakai file final yang tervalidasi. |
| Manifest lama READY/UPLOADED | Tetap terbaca dan diproses tanpa field segmen baru. |
| Crash sebelum laporan/ack command | Journal merekonsiliasi; start lama tidak diputar ulang tanpa keputusan operator. |

Gunakan beberapa channel dan perangkat player nyata sebelum menyatakan integrasi siap operasional. Simpan hasil, log tanpa kredensial, status sesi, dan hasil ffprobe sebagai bukti penerimaan.

## Pemeriksaan backend setup — 11 November 2026

Build/vet dan pemeriksaan import arsitektur dilakukan tanpa menjalankan suite tes Go. Alur terisolasi memakai simulator layanan ONVIF Media1, MediaMTX v1.15.3 dan tiga sumber FFmpeg sintetis: H.264/AAC, H.264 video-only, dan H.265 video-only.

Yang sudah diperiksa: bootstrap tanpa konfigurasi, penolakan setup dari peer remote/nonce salah, profil tanpa kebocoran credential, konfigurasi DPAPI dan redaksi response, apply, penolakan apply saat sesi aktif, live H.264/Opus, video-only tanpa audio palsu, konversi H.265, finalisasi READY, restart tanpa auto-recording, serta probe startup. WHEP diuji sampai SDP answer, CORS origin frontend loopback, Location gateway dan DELETE; ini belum membuktikan ICE/DTLS serta playback browser nyata.

Route UI `/setup` dan `/dashboard` tidak disediakan. Frontend tetap terpisah. ONVIF Media2, model NVR asli, S3 nyata, kondisi disk penuh/crash saat apply, jaringan/firewall dan 24 live pada mini PC tujuan tetap memerlukan penerimaan tersendiri.
