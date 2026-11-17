# Integrasi cloud

Desktop ADMIN/OPERATOR melakukan pairing dan mengonfirmasi mapping arena/ronde/tiket. Credential DPAPI, mapping dan progres disimpan atomik. [Kontrak media desktop](../../cloud-app/docs/desktop-media-contract.md).

Upload selalu container mkv dan object key .mkv. Multipart/checksum/resume/retry/konfirmasi metadata tetap tersedia. File lokal dipertahankan; tanpa mapping berarti WAITING_MAPPING. Recording tidak mengubah pembayaran/peserta/ronde.

Mode cloud memakai CLOUD_COMMANDS_URL (GET, POST /results), CLOUD_HEARTBEAT_URL (POST), CLOUD_MEDIA_UPDATES_URL (POST), CLOUD_ENGINE_TOKEN. Response memakai success/message/data/errors. Heartbeat berisi identitas, media state, channels dan hitungan recording/upload/restream. Readiness cloud memerlukan MediaMTX, heartbeat, command dan metadata; LAN bisa merekam tanpa cloud.

Endpoint desktop/cloud, desktop/mappings, recordings/{id}/upload, recordings/{id}/retry dan recordings/{id}/file memakai localhost/nonce. MKV mendukung Range. Token tidak diteruskan ke WebView/storage; credential bucket hanya di cloud.
