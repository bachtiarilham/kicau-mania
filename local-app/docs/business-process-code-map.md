# Proses bisnis

- Setup: delivery/rest/setup → setup/usecase → ONVIF/ffprobe → DPAPI → Coordinator.
- Sesi/MKV: delivery/rest/session → session/usecase → FFmpeg copy → finalizer → manifest.
- Metrik: PCM → audio/usecase → WebSocket.
- Upload: delivery/rest/desktop → cloudmedia/service → mapping/queue/multipart/retry.
- Command/heartbeat: cloud/usecase dan worker → cloud terkonfigurasi.

[Peta file](code-map.md), [cloud](cloud-contract.md), [alur](end-to-end.md).
