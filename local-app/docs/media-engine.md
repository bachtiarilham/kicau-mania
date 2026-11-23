# Media engine

NVR HEVC/H.265 → MediaMTX RTSP/TCP → libVLC desktop. HTTP untuk kontrol dan WebSocket untuk metrik.

Slider zoom desktop mengatur crop tengah 100–300% pada player yang sama. Untuk konsumen RTSP, path `zoom/<id-kamera>_zoom` menyediakan crop tengah 2×. MediaMTX menjalankan FFmpeg saat path itu dibuka. Path zoom memakai H.264 dan membawa audio AAC jika tersedia. Kalibrasi gantangan dengan kamera yang terdaftar tersedia di path `<id-gantangan>_zoom`. Restart backend setelah mengubah kalibrasi untuk memuat crop baru.

Capture memvalidasi HEVC setiap mulai atau reconnect, menyalin video ke MKV, dan mendekode PCM untuk metrik. `AUDIO_FILTER_ENABLED` mengontrol filter PCM dan audio stream zoom. `AUDIO_GATE_DBFS` mengatur ambang gate (default −55 dBFS), sedangkan `AUDIO_ACTIVE_DBFS` mengatur deteksi aktivitas. Audio rekaman disalin dari sumber secara default. Aktifkan `AUDIO_FILTER_APPLY_RECORDING` bersama filter untuk memfilter dan menyimpan audio rekaman sebagai AAC.

STARTING → RUNNING/DEGRADED → STOPPING → terminal. Reconnect membuat segmen MKV dan gap. Finalizer memeriksa packet/durasi/kecocokan codec sebelum concat copy dan rename atomik. PTS kosong dapat diisi dari DTS tanpa mengubah payload. Partial tidak READY hanya karena byte positif.

DATA_DIR menyimpan rekaman/manifest/journal. Recovery menyelesaikan segmen tanpa melanjutkan sesi lama otomatis. Queue tetap berjalan saat desktop ditutup. File non-MKV ditolak. [Cloud](cloud-contract.md), [acceptance](acceptance.md).
