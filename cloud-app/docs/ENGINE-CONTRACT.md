# Kontrak cloud-app dan engine media

**Status: kontrak transport; semua route di dokumen ini belum didaftarkan.** Repository persisten, autentikasi pengguna/mesin, dan aturan akses arena harus tersedia sebelum route diaktifkan. Jangan memakai storage memory atau respons sukses tiruan sebagai pengganti persistence.

Kontrak ini mengikuti [gateway local-app](../../local-app/ENGINE-CLOUD-CONTRACT.md). Skema tabel dan query ditunda sampai ERD disetujui. Login pengguna cloud terpisah dari kredensial engine; token mesin tidak boleh diberikan kepada browser.

## Endpoint yang akan disediakan

| Metode dan path | Pemanggil | Fungsi |
| --- | --- | --- |
| GET /api/v1/engines/{engine}/commands | Engine | Polling command media. |
| POST /api/v1/engines/{engine}/commands/results | Engine | Melaporkan hasil command. |
| POST /api/v1/engines/{engine}/heartbeat | Engine | Melaporkan status/kesehatan proses media. |
| POST /api/v1/engines/{engine}/media-updates | Engine | Mengirim metadata rekaman/upload. |
| POST /api/v1/engines/{engine}/playback/authorize | Engine | Memvalidasi tiket pengguna untuk gateway WHEP. |
| POST /api/v1/arenas/{arena}/playback-tickets | Browser dengan sesi pengguna | Meminta tiket channel dan alamat engine arena. |

Endpoint mesin memakai `Authorization: Bearer CLOUD_ENGINE_TOKEN`. Scope arena/engine ditentukan dari identitas mesin yang diverifikasi, lalu dibandingkan dengan path dan payload. Endpoint browser memakai autentikasi pengguna cloud; bentuk login/sesi belum ditentukan pada fase ini. Otorisasi bisnis dilakukan UseCase, bukan middleware, handler, atau repository.

## Command dan hasil

Seluruh response mesin memakai envelope success/message/data/errors, sama dengan REST. Contoh polling HTTP 200:

```json
{"success":true,"message":"Commands retrieved","data":{"commands":[]}}
```

Request tetap berupa payload langsung. Engine mengirim Authorization Bearer, Accept application/json, X-Request-ID, dan Content-Type application/json hanya jika ada body. Cloud mengembalikan Content-Type application/json dan X-Request-ID yang valid untuk korelasi. Client engine membuka data sebelum meneruskannya ke UseCase. Response tanpa envelope tidak didukung.

Client mempertahankan timeout 8 detik, larangan redirect, serta batas response 2 MiB. HTTP selain 200, media type bukan JSON, JSON rusak/berlebih, success hilang/false, atau data hilang/null ditolak. Retry tetap milik worker/UseCase. Pesan/body remote mentah dan URL privat tidak diteruskan sebagai error/log client. Detail aturan penerimaan juga tercantum pada kontrak local-app.

Bentuk setiap command di dalam `data.commands`:

```json
{
  "id": "command-001",
  "arena_id": "arena01",
  "engine_id": "engine01",
  "kind": "media.session.start",
  "payload": {"session_id":"session-001","channels":["gantangan01"]},
  "expires_at": "2026-11-09T12:00:00Z"
}
```

Jenis command: `media.session.start` dengan payload `{ "id": "session-001", "channels": ["gantangan01"], "restreams": [{ "channel_id": "gantangan01", "target": "youtube" }] }`, serta `media.session.stop` dengan payload `{ "id": "session-001" }`. Restreams boleh kosong. Target mengacu pada allowlist engine. Command lama tidak didukung; ID command dan payload tetap immutable.

Hasil dikirim sebagai `{ "results": [{"id":"command-001","state":"SUCCEEDED","at":"2026-11-09T12:00:01Z"}] }`. State hasil: `SUCCEEDED`, `FAILED`, `EXPIRED`. Response HTTP 200 `{"success":true,"message":"Results saved","data":{"acknowledged":["command-001"]}}` hanya setelah hasil tersimpan/commit. Command belum diakui harus tetap dapat dikirim ulang dengan isi yang sama. Hasil SUCCEEDED menunjukkan aksi engine telah diterima/dijalankan, bukan jaminan rekaman sudah selesai atau terupload.

## Heartbeat

```json
{
  "arena_id":"arena01",
  "engine_id":"engine01",
  "at":"2026-11-09T12:00:00Z",
  "media_state":"RUNNING",
  "playback_base_url":"https://lan-dev.megaallstararena.com",
  "channels":["gantangan01"],
  "recording_count":0,
  "pending_uploads":0,
  "restream_count":0
}
```

Response HTTP 200 `{"success":true,"message":"Heartbeat saved","data":{"acknowledged":true,"arena_id":"arena01","engine_id":"engine01"}}` setelah status tersimpan. Default pengiriman engine setiap 5 detik dan batas kesegaran 15 detik. Gunakan waktu penerimaan cloud untuk kesegaran, bukan hanya jam yang dilaporkan engine. Status proses RUNNING tidak membuktikan semua sumber kamera sehat.

URL dari heartbeat bukan izin mengganti alamat engine secara otomatis. URL playback untuk browser harus berasal dari registry engine yang disetujui administrator. Heartbeat tidak langsung mengubah nilai/status perlombaan; kebijakan bisnis tersebut merupakan pekerjaan domain lomba setelah ERD disetujui.

## Metadata rekaman/upload

Kirim `{ "updates": [...] }`, setiap update berisi:

```json
{
  "id":"hash-update",
  "arena_id":"arena01",
  "engine_id":"engine01",
  "recording_id":"recording-001",
  "session_id":"session-001",
  "channel_id":"gantangan01",
  "version":1,
  "state":"READY",
  "object_key":"rounds/session-001/gantangan01/recording-001.mp4",
  "started_at":"2026-11-09T12:00:00Z",
  "ended_at":"2026-11-09T12:05:00Z"
}
```

Response HTTP 200 `{"success":true,"message":"Metadata saved","data":{"acknowledged":["hash-update"]}}` hanya untuk update yang sudah tersimpan atau sudah dikenali secara idempotent. Cloud menerapkan versi terbaru per rekaman dalam scope engine. Versi lama boleh diakui tanpa menimpa versi baru; ID sama dengan payload berbeda ditolak. Versi 0 mendukung manifest engine lama. Satu batch dapat diakui sebagian; ID acknowledgment harus berasal dari batch tersebut.

Jangan menerima local file path, token kamera, maupun kredensial S3 sebagai metadata publik. `object_key` adalah referensi object storage, bukan izin download. Akses video kepada pengguna memerlukan otorisasi terpisah.

## Tiket playback pengguna

Browser mengirim ke endpoint arena:

```json
{"engine_id":"engine01","channel_id":"gantangan01"}
```

UseCase memeriksa pengguna aktif, kepemilikan engine/channel oleh arena, penugasan operator, atau penugasan arena/ronde untuk juri. Detail roster mengikuti ERD yang disetujui. Jangan mengganti pemeriksaan tersebut dengan token admin bersama.

Response HTTP 200:

```json
{
  "success":true,
  "message":"Playback ticket issued",
  "data":{
    "ticket":"opaque-short-lived-ticket",
    "expires_at":"2026-11-09T12:01:00Z",
    "whep_url":"https://lan-dev.megaallstararena.com/api/v1/playback/gantangan01/whep"
  }
}
```

Tiket berlaku 60 detik dan terikat pengguna, arena, engine, channel. `whep_url` dibentuk dari registry terpercaya, bukan input URL browser. Jangan mencatat tiket pada log atau menyisipkannya ke query URL. Detail penyimpanan tiket serta sesi pengguna ditentukan saat autentikasi/persistence diimplementasikan, bukan melalui migrasi pada fase fondasi.

Gateway memanggil endpoint otorisasi mesin dengan payload:

```json
{"ticket":"opaque-short-lived-ticket","arena_id":"arena01","engine_id":"engine01","channel_id":"gantangan01"}
```

Grant yang diizinkan:

```json
{"success":true,"message":"Playback allowed","data":{"allowed":true,"user_id":"user-id","arena_id":"arena01","engine_id":"engine01","channel_id":"gantangan01","expires_at":"2026-11-09T12:01:00Z"}}
```

Tiket tidak valid/kedaluwarsa/salah scope pada mesin yang terautentikasi mengembalikan HTTP 200 `{"success":true,"message":"Playback checked","data":{"allowed":false}}`; gateway memetakannya ke penolakan akses. Kegagalan pemrosesan memakai success false dan message/errors yang aman, misalnya `{"success":false,"message":"Request failed","errors":{"request_id":"cloud-request-id"}}`. Mesin yang tidak terautentikasi tetap ditolak dengan HTTP 401/403. Grant tidak boleh memperpanjang expiry tiket asli. Local-app menolak expiry yang lewat atau melebihi 65 detik dari waktu validasi; sinkronkan jam server.

Cloud tidak meneruskan byte media. Browser harus dapat menjangkau URL HTTPS engine dan UDP WebRTC melalui LAN. PATCH/DELETE memakai Location sesi WHEP rahasia. Expiry tiket mencegah sesi baru; tidak otomatis memutus WebRTC yang sudah terbentuk. Cloud tidak tersedia berarti pembukaan sesi baru ditolak; rekaman engine yang berjalan tetap berlanjut.

## Batas fase

Tidak ada endpoint login, CRUD, tiket, command, heartbeat, atau metadata aktif pada fondasi ini. Pembekuan SQL berlaku juga untuk tabel pendukung engine. Penerimaan integrasi berikutnya mencakup isolasi arena, hak operator/juri, expiry, command duplikat, restart sebelum acknowledgment, metadata terlambat, serta HTTPS/CORS dan browser LAN nyata.

## Lifecycle media sesi

Mulai mengaktifkan live video/audio, recording, PCM/metrik, serta restream terpilih. Satu sesi nonterminal per engine, termasuk selama FINALIZING. Stop menutup admission WHEP/audio, menghentikan seluruh media, lalu memfinalisasi rekaman; upload dan pelaporan tetap berjalan. Playback hanya untuk channel sesi aktif dan tetap membutuhkan tiket cloud.

Command sukses menyatakan permintaan background telah diterima, bukan media sudah RUNNING atau finalisasi sudah STOPPED. Status lifecycle tersedia pada API operator engine GET `/api/v1/media/sessions/{id}`; kontrak heartbeat/metadata tidak ditambah field pada fase ini. Jangan menyatakan Stop selesai berdasarkan acknowledgment command saja. Endpoint mesin pendamping cloud tetap belum aktif.

Setelah restart, sesi nonterminal menjadi INTERRUPTED tanpa melanjutkan media. Segmen diperiksa/salvage; hasil gagal diberi alasan dan sumber dipertahankan. Gunakan ID sesi baru setelah keputusan operator. Cloud terputus tidak menghentikan recording aktif; pembukaan playback baru tetap ditolak jika grant tidak tersedia.
