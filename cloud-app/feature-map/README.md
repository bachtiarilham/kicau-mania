# Feature Map - Cloud App Backend (Clean Architecture REST API)

Direktori ini berisi katalog interaktif dan peta fitur (*Feature Map*) lengkap untuk **Cloud Backend App (`cloud-app`)**.
Peta ini merinci fungsionalitas REST API, cara akses, selector/parameter HTTP, interaksi request/response, dan hasil yang diharapkan.

---

## Arsitektur Rute & Keamanan (3-Tier Security)

| Tier | Prefix Rute | Syarat Autentikasi | Cakupan Fitur |
| :--- | :--- | :--- | :--- |
| **Tier 1 (Public)** | `/healthz`, `/readyz`, `/api/v1/auth/*`, `/v/:id` | Publik (Tanpa token) | Probe kesiapan server, login pengguna, pemutaran link video publik. |
| **Tier 2 (User Protected)**| `/api/v1/*` | Header `Authorization: Bearer <JWT>` atau Session Cookie | Manajemen arena, master data gantangan, pembuatan event & ronde, pendaftaran tiket, penilaian juri. |
| **Tier 3 (Engine Protected)**| `/api/v1/engine/media/*` | Header `Authorization: Bearer <EngineToken>` | Ingestion sesi rekaman video edge, upload chunk video multipart S3. |

---

## Indeks Dokumen Fitur

| File | Nama Domain / Fitur Utama | Endpoint Utama |
| :--- | :--- | :--- |
| [01-identity-and-auth.md](./01-identity-and-auth.md) | Identitas, Autentikasi & Profil | `/api/v1/auth/login`, `/logout`, `/users/me` |
| [02-arenas-and-assets.md](./02-arenas-and-assets.md) | Arena & Master Data Gantangan | `/api/v1/arenas`, `/gantangans`, `/species`, `/categories` |
| [03-tournaments-and-rounds.md](./03-tournaments-and-rounds.md) | Event Lomba & Penjadwalan Ronde | `/api/v1/arenas/:arena/events`, `/rounds` |
| [04-ticketing-and-registrations.md](./04-ticketing-and-registrations.md) | Pendaftaran Peserta & Tiket | `/api/v1/arenas/:arena/registrations`, `/proof` |
| [05-judging-and-scoring.md](./05-judging-and-scoring.md) | Penugasan Juri & Rekap Penilaian | `/api/v1/arenas/:arena/rounds/:id/scores`, `/recap` |
| [06-media-and-whatsapp.md](./06-media-and-whatsapp.md) | Rekaman Cloud & WhatsApp | Alur unduhan, pengiriman tautan peserta, dan masa simpan lima hari. |
