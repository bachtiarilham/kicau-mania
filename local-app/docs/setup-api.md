# Setup NVR melalui API

Mode LAN: GET /api/v1/setup/bootstrap untuk nonce, lalu X-Local-Control pada kontrol localhost. Nonce berubah saat restart; proteksi origin tetap berlaku.

GET /api/v1/setup/checks memeriksa FFmpeg/ffprobe/MediaMTX tanpa encoder. POST /api/v1/setup/jobs menerima discover, profiles, probe, apply atau recheck; poll jobs/{id} sampai selesai. Video selain HEVC/H.265 menghasilkan UNSUPPORTED_VIDEO_CODEC. Startup/restore dan reconnect juga memvalidasi sumber.

```json
{"action":"apply","configuration":{"version":1,"lan_ip":"192.168.1.20","lan_cidr":"192.168.1.0/24","channels":[{"id":"gantangan01","source":"rtsp://192.168.1.123:554/Streaming/Channels/101","username":"AKUN_RTSP","password":"PASSWORD_RTSP"}]}}
```

Sesuaikan path perangkat. Akun ONVIF/RTSP dapat berbeda. Source/credential disamarkan; DPAPI memakai akun Windows penyimpan. Tidak ada field mode konversi. Apply ditolak saat sesi/finalisasi aktif dan tidak otomatis merekam. POST /api/v1/media/sessions memulai sesi; POST /api/v1/media/sessions/{id}/stop lalu tunggu terminal. Checks/Swagger tersedia sebelum NVR siap.
