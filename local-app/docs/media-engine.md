# Media engine

NVR HEVC/H.265 → MediaMTX RTSP/TCP → libVLC desktop. Relay read-only loopback tidak memerlukan publisher pipeline. HTTP untuk kontrol, WebSocket untuk metrik.

Capture memvalidasi HEVC setiap mulai/reconnect, menyalin video/audio ke MKV dan mendecode PCM terpisah untuk metrik. Audio yang tidak dapat dimuat ke MKV menghasilkan error. Tidak ada resize, perubahan FPS atau encoder.

STARTING → RUNNING/DEGRADED → STOPPING → terminal. Reconnect membuat segmen MKV dan gap. Finalizer memeriksa packet/durasi/kecocokan codec sebelum concat copy dan rename atomik. PTS kosong dapat diisi dari DTS tanpa mengubah payload. Partial tidak READY hanya karena byte positif.

DATA_DIR menyimpan rekaman/manifest/journal. Recovery menyelesaikan segmen tanpa melanjutkan sesi lama otomatis. Queue tetap berjalan saat desktop ditutup. File non-MKV ditolak. [Cloud](cloud-contract.md), [acceptance](acceptance.md).
