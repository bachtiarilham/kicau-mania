# Perangkat dan data

Mini PC Windows menjalankan local-app, FFmpeg/ffprobe, MediaMTX 1.15.3 dan desktop Wails/libVLC. NVR terjangkau jaringan; ONVIF untuk discovery, RTSP/TCP HEVC/H.265 untuk media. Credential DPAPI tidak masuk WebView.

Relay meneruskan media asli. Capture menyalin media ke MKV; decode PCM hanya untuk metrik. Upload multipart langsung ke storage privat melalui URL cloud bertanda tangan.

Periksa bandwidth/disk/CPU/GPU/izin folder/subnet. Resolusi/FPS/audio mengikuti NVR. Decoder, reordering, jaringan dan sumber memengaruhi latency. Kapasitas banyak channel perlu diuji pada perangkat arena. [Acceptance](acceptance.md).
