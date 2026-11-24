# Kicau Mania

Sistem ekosistem terpadu pengelolaan kompetisi burung berkicau, penayangan live stream multi-kamera arena, perekaman presisi, dan penjurian digital.

Ekosistem Kicau Mania terdiri dari tiga komponen utama:
1. **`cloud-app`**: Backend API terpusat untuk autentikasi, manajemen turnamen, jadwal ronde, penjurian, registrasi peserta, dan sinkronisasi media (Port `8080`).
2. **`local-app`**: Edge Media Engine yang berjalan di komputer arena lokal untuk relay RTSP (MediaMTX), analisis audio, restreaming, perekaman MKV H.265/HEVC, dan upload multipart terisolasi (Port `8090`).
3. **`kicau-mania-desktop`**: Aplikasi desktop Windows (Wails v3 + Svelte 5) yang menyediakan Tactical Operator Console (24 kamera arena) dan Layar Penilaian Dewan Juri.

---

## 1. Komponen Ekosistem

| Komponen | Peran & Tanggung Jawab | Port Default | Panduan |
| :--- | :--- | :--- | :--- |
| **`cloud-app`** | Master data bisnis PostgreSQL: Auth (JWT/Cookie), Arenas, Competitions, Rounds, Judging Sessions, Media Upload Gateways. | `8080` | [README cloud-app](cloud-app/README.md) |
| **`local-app`** | Edge Media Engine: RTSP ingest (MediaMTX), audio metering (FFmpeg PCM), stream copy H.265, storage segmen & upload worker. | `8090` | [README local-app](local-app/README.md) |
| **`kicau-mania-desktop`** | Aplikasi Desktop Windows: HUD Juri (Grid 3x2), Operator Console (Grid 24 Gantangan), LibVLC HWND hardware decoding, Quick Save. | `5173` (Vite dev) | [README desktop](../wails/kicau-mania-desktop/README.md) |
| **`agent-ctl`** | Standalone Controller CLI: Orchestrator status proses, healthcheck, automated test verification, dan evidence reporting. | N/A | [Runbook CLI](tools/agent-ctl/README.md) |

---

## 2. Arsitektur Bersih (Clean Architecture)

Seluruh komponen backend Go (`cloud-app`, `local-app`, dan `kicau-mania-desktop`) mengadopsi standar **Vertical Slice (Domain-First Modular Monolith)**:
```
internal/
├── <domain-a>/
│   ├── model/         # Entitas data, DTO, dan konstanta domain
│   ├── repository/    # Database store / driver persistensi lokal
│   ├── gateway/       # Adapter HTTP/WebSocket ke layanan eksternal
│   ├── usecase/       # Logika bisnis inti (independen dari transport/UI)
│   └── delivery/      # Transport handler (REST Gin / Wails RPC)
├── infra/             # Shared low-level adapters (FFmpeg, MediaMTX, Logger, S3)
└── app/               # Composition root & lifecycle wiring
```

### Domain Utama:
- **`local-app`**: `camera`, `recording`, `streaming`, `cloud` + `infra/` (`ffmpeg`, `mediamtx`, `media`, `logger`, `storage`, `runtime`, `network`).
- **`cloud-app`**: `identity`, `competition`, `asset`, `media` + `infra/` (`database`, `logger`, `storage`).
- **`kicau-mania-desktop`**: `auth`, `config`, `engine`, `player`, `recording` + `bindings/`, `container/`, `app/`.

---

## 3. Kontrol Operasional: `agent-ctl` CLI

Pengontrolan proses, pengujian otomatis, dan audit verifikasi sistem dilakukan secara terpusat melalui binary mandiri: **`bin/agent-ctl.exe`**.

```powershell
# 1. Cek status seluruh service (cloud, edge, desktop)
bin\agent-ctl.exe status

# 2. Verifikasi automated unit tests & AST rules
bin\agent-ctl.exe verify cloud --no-smoke
bin\agent-ctl.exe verify edge --no-smoke
bin\agent-ctl.exe verify desktop --no-smoke

# 3. Kontrol proses service
bin\agent-ctl.exe start edge
bin\agent-ctl.exe stop edge

# 4. Generate laporan bukti kerja terverifikasi (Evidence Report)
bin\agent-ctl.exe evidence edge --no-smoke
```

---

## 4. Rujukan Peta Fitur (Feature Maps)

Sebelum menambahkan atau memodifikasi interaksi bisnis atau UI/API, konsultasikan dokumen spesifikasi pada folder `feature-map/`:
- **Desktop HUD & Operator**: `../wails/kicau-mania-desktop/feature-map/`
  - `01-auth-and-session.md`
  - `02-operator-console.md`
  - `03-juri-hud.md`
  - `04-recordings-and-whatsapp.md`
  - `05-desktop-settings.md`
- **Cloud Backend API**: `cloud-app/feature-map/`
- **Edge Media Engine**: `local-app/feature-map/`

---

## 5. Dokumentasi Terkait

- **[Indeks Dokumentasi Lengkap](docs/README.md)**: Rujukan integrasi, diagram interaksi, dan catatan arsitektur.
- **[Panduan Integrasi Local-Cloud](docs/integration.md)**: Alur pairing, binding sesi ronde, dan upload rekaman.
- **[Aturan Pengembang (GEMINI.md)](GEMINI.md)**: Aturan Clean Architecture, Zero-Comment Rule, dan protokol verifikasi agen.
