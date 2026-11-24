# Indeks Dokumentasi Kicau Mania

Dokumen ini merupakan panduan navigasi dan peta dokumentasi teknis untuk ekosistem Kicau Mania.

---

## 1. Panduan Arsitektur & Spesifikasi Fitur

### A. Panduan Utama & Aturan Repositori
- **[README Proyek Utama](../README.md)**: Gambaran menyeluruh ekosistem (`cloud-app`, `local-app`, `desktop`, `agent-ctl`).
- **[Aturan Pengembang & AI Agent (GEMINI.md)](../GEMINI.md)**: Aturan wajib Clean Architecture, Zero-Comment, eliminasi workaround, dan protokol `agent-ctl`.
- **[Alur Integrasi Local-App dan Cloud-App](integration.md)**: Batas tanggung jawab, jalur media LAN vs bisnis cloud, format credential, dan alur upload presigned S3.

### B. Spesifikasi Peta Fitur (Feature Maps)
- **Cloud Backend API**: [cloud-app/feature-map/](../cloud-app/feature-map/)
- **Edge Media Engine**: [local-app/feature-map/](../local-app/feature-map/)
- **Desktop HUD & Operator**: [kicau-mania-desktop/feature-map/](../../wails/kicau-mania-desktop/feature-map/)
  - `01-auth-and-session.md`: Autentikasi operator & pairing sesi ronde.
  - `02-operator-console.md`: Konsol taktis operator, 24 kamera matrix, timer kontrol, dan quick save.
  - `03-juri-hud.md`: Layar dewan juri, grid 3x2, dan rotasi shift juri 4 kuadran.
  - `04-recordings-and-whatsapp.md`: Manajemen file rekaman MKV/MP4 dan integrasi export WhatsApp.
  - `05-desktop-settings.md`: Konfigurasi storage, DPAPI credentials, dan koneksi local-app/cloud.

---

## 2. Dokumentasi Komponen Individual

| Komponen | Dokumentasi Teknis | Topik Pembahasan |
| :--- | :--- | :--- |
| **`local-app`** | [local-app/docs/README.md](../local-app/docs/README.md) | Domain `camera`, `recording`, `streaming`, `cloud`, driver FFmpeg/MediaMTX. |
| **`cloud-app`** | [cloud-app/docs/API.md](../cloud-app/docs/API.md) | Kontrak REST API, OpenAPI/Swagger docs, autentikasi JWT, dan webhook. |
| **Database** | [cloud-app/db/README.md](../cloud-app/db/README.md) | Skema PostgreSQL, migrasi goose, dan [ERD](../cloud-app/db/ERD.md). |
| **Deployment** | [cloud-app/deployment/devops_runbook.md](../cloud-app/deployment/devops_runbook.md) | Runbook Docker Compose, VPS Ubuntu, Nginx reverse proxy, dan SSL. |
| **Desktop** | [kicau-mania-desktop/README.md](../../wails/kicau-mania-desktop/README.md) | Setup Wails v3, libVLC native Win32 embedding, Svelte 5 Runes. |
| **CLI Tool** | [tools/agent-ctl/README.md](../tools/agent-ctl/README.md) | Binary `agent-ctl.exe`, lifecycle status, automated verify, evidence reports. |

---

## 3. Catatan Historis & Referensi

Catatan awal dan laporan arsip disimpan sebagai dokumentasi historis:
- **[Progres Ringkas](reports/progress-report.md)** & **[Progres Lengkap](reports/progress-report-full.md)** (Snapshot arsip).
- **[Brief Awal Local-App](notes/local-app-brief.md)** & **[Brief Awal Cloud-App](notes/cloud-app-brief.md)**.
- **[Diskusi ERD Awal](../discussion_11062026_erd.txt)**.
