# Kontrak engine media dan frontend cloud

Local-app menerima command dan otorisasi playback dari layanan cloud melalui HTTPS. `ARENA_ID`, `ENGINE_ID`, dan token mesin harus cocok dengan registry cloud. Dokumen ini merupakan kontrak untuk endpoint pendamping yang perlu tersedia di cloud-app.

## Endpoint engine keluar

| Env | Metode | Payload dan response |
| --- | --- | --- |
| `CLOUD_COMMANDS_URL` | GET | Response `data` berisi `{ "commands": [...] }`. |
| `CLOUD_COMMANDS_URL` + `/results` | POST | Kirim `{ "results": [...] }`, response `data` berisi `{ "acknowledged": ["command-id"] }`. |
| `CLOUD_HEARTBEAT_URL` | POST | Kirim identitas, waktu, `media_state`, `playback_base_url`, `channels`, `recording_count`, `pending_uploads`, dan `restream_count`. Response `data` berisi `{ "acknowledged": true, "arena_id": "arena01", "engine_id": "engine01" }`. |
| `CLOUD_MEDIA_UPDATES_URL` | POST | Kirim `{ "updates": [...] }`, response `data` berisi `{ "acknowledged": ["update-id"] }`. |
| `CLOUD_PLAYBACK_AUTHORIZE_URL` | POST | Kirim tiket/scope; response `data` berisi grant pengguna/arena/engine/channel. |

Request engine memakai `Authorization: Bearer CLOUD_ENGINE_TOKEN`, `Accept: application/json`, dan `X-Request-ID`. Content-Type application/json hanya dikirim jika ada body. Request ID diteruskan dari context atau dibuat baru untuk request background. Request payload tetap langsung, tanpa envelope.

Semua response sukses memakai HTTP 200, Content-Type application/json (charset diperbolehkan), dan envelope berikut. Client hanya menyerahkan `data` ke UseCase:

```json
{"success":true,"message":"Commands retrieved","data":{"commands":[]}}
```

Kegagalan pemrosesan memakai `{"success":false,"message":"Request failed","errors":{"request_id":"cloud-request-id"}}` beserta status HTTP yang sesuai. Penolakan tiket yang berhasil diperiksa memakai HTTP 200 `{"success":true,"message":"Playback checked","data":{"allowed":false}}`; UseCase memetakannya menjadi akses ditolak.

Client menolak HTTP selain 200, media type bukan JSON, JSON rusak/berlebih, envelope tanpa success, success false, serta data hilang/null. Response mesin lama tanpa envelope tidak didukung. Timeout 8 detik, batas body response 2 MiB, redirect ditolak, dan tidak ada retry otomatis client. Retry/acknowledgment tetap dikelola worker/UseCase. Error hanya membawa kategori, status, dan request ID cloud yang valid; body, URL privat, token, dan pesan remote mentah tidak dimasukkan ke log/error. Cancellation tetap dapat diperiksa dengan errors.Is.

URL kosong membuat integrasi terkait belum aktif. `/readyz` tetap mengembalikan 503 sampai seluruh endpoint dikonfigurasi, heartbeat diakui, dan proses MediaMTX berjalan. Endpoint pendamping cloud tetap perlu diimplementasikan; perubahan kontrak ini tidak mengaktifkannya.

## Command

```json
{
  "id": "command-001",
  "arena_id": "arena01",
  "engine_id": "engine01",
  "kind": "media.session.start",
  "payload": { "session_id": "session-001", "channels": ["gantangan01"] },
  "expires_at": "2026-11-09T12:00:00Z"
}
```

Jenis command: `media.session.start` dengan payload `{ "id": "session-001", "channels": ["gantangan01"], "restreams": [{ "channel_id": "gantangan01", "target": "youtube" }] }`, serta `media.session.stop` dengan payload `{ "id": "session-001" }`. Restreams boleh kosong. Target mengacu pada allowlist engine. Command lama tidak didukung; ID command dan payload tetap immutable.

Hasil berisi `id`, `state`, dan `at`. State hasil adalah `SUCCEEDED`, `FAILED`, atau `EXPIRED`. Hasil sukses menunjukkan aksi UseCase telah diterima/dijalankan. Cloud membaca kesehatan proses melalui heartbeat. Command disimpan sebelum dieksekusi dan hasil disimpan sebelum dilaporkan. Setelah restart, command yang hasilnya belum pasti direkonsiliasi tanpa memulai ulang pekerjaan start secara otomatis. Operator dapat mengirim command baru setelah meninjau kegagalan.

## Metadata rekaman

Setiap update berisi `id`, `arena_id`, `engine_id`, `recording_id`, `session_id`, `channel_id`, `version`, `state`, `object_key`, `started_at`, dan `ended_at`. File manifest memiliki versi monoton per rekaman. ID update merupakan hash stabil dari payload.

Cloud harus menyimpan update secara idempotent dan menerapkan state rekaman berdasarkan versi terbaru. Versi yang lebih rendah dapat tetap diakui sebagai sudah diproses tanpa menimpa state terbaru. Versi 0 mendukung manifest yang sudah ada sebelum penghitung versi ditambahkan. Path file lokal tidak dikirim.

## Tiket playback

Frontend meminta tiket ke cloud setelah pengguna menekan **Lihat live**. Endpoint penerbitan di cloud memeriksa sesi pengguna, akses arena, serta penugasan operator/juri. Response yang diperlukan frontend berisi `ticket`, `expires_at`, dan `whep_url`.

Contoh URL lokal adalah `https://lan-dev.megaallstararena.com/api/v1/playback/gantangan01/whep`. Tiap arena mempunyai URL engine sendiri. Tiket disimpan sementara dalam memori frontend dan dikirim sebagai Bearer header saat POST SDP.

Gateway mengirim request berikut ke endpoint otorisasi cloud.

```json
{
  "ticket": "opaque-playback-ticket",
  "arena_id": "arena01",
  "engine_id": "engine01",
  "channel_id": "gantangan01"
}
```

Response grant cloud harus memiliki seluruh scope yang cocok. Client membuka envelope sebelum UseCase memeriksanya.

```json
{
  "success": true,
  "message": "Playback allowed",
  "data": {
    "allowed": true,
    "user_id": "user-id",
    "arena_id": "arena01",
    "engine_id": "engine01",
    "channel_id": "gantangan01",
    "expires_at": "2026-11-09T12:00:00Z"
  }
}
```

Cloud menerbitkan masa berlaku 60 detik. Gateway menolak grant kedaluwarsa atau expiry melebihi 65 detik dari waktu validasi. Sinkronkan jam server. Tiket mengizinkan pembukaan sesi. Sesi yang sudah terbentuk memakai secret session MediaMTX melalui URL `Location` untuk PATCH/DELETE. Perlakukan URL tersebut sebagai kapabilitas privat dan hapus sesi ketika player ditutup. Token kedaluwarsa tidak memutus sesi WebRTC yang sudah terbuka.

Contoh signaling frontend berikut mengasumsikan `pc` adalah RTCPeerConnection dengan transceiver audio/video receive-only dan penawaran yang ICE gathering-nya sudah selesai.

```javascript
// Helper meminta tiket ke cloud, memeriksa success, lalu mengembalikan data.
const grant = await requestPlaybackTicketFromCloud(arenaID, channelID);
const response = await fetch(grant.whep_url, {
  method: 'POST',
  headers: {
    Authorization: `Bearer ${grant.ticket}`,
    'Content-Type': 'application/sdp'
  },
  body: pc.localDescription.sdp,
  credentials: 'omit'
});
if (!response.ok) throw new Error(`Playback HTTP ${response.status}`);
const sessionURL = new URL(response.headers.get('Location'), grant.whep_url);
await pc.setRemoteDescription({type: 'answer', sdp: await response.text()});
// Saat player ditutup, tutup pc dan DELETE sessionURL tanpa menambahkan tiket ke URL.
```

Frontend menangani izin Local Network Access browser melalui aksi pengguna. Gunakan HTTPS frontend dan engine, CORS origin cloud yang eksplisit, DNS lokal yang benar, serta akses UDP media dari jaringan arena. Pemutaran LAN membutuhkan perangkat berada pada jaringan yang dapat menjangkau engine. Cloud tidak meneruskan byte video ke browser.

## Status dan gangguan

Heartbeat berjalan default setiap 5 detik dan menjadi usang setelah 15 detik. Cloud menggunakan heartbeat untuk interupsi/resume bisnis. Local-app melanjutkan rekaman berjalan saat cloud terputus dan menyimpan hasil command serta metadata yang belum diakui. Pembukaan playback baru memerlukan otorisasi cloud yang berhasil.

Cloud-app saat ini menyediakan fondasi HTTP/PostgreSQL dan dokumentasi kontrak; endpoint command, heartbeat, media updates, penerbitan tiket, dan otorisasi playback belum aktif. Implementasinya menunggu autentikasi dan repository persisten pada pekerjaan cloud berikutnya. Local-app menolak playback baru ketika endpoint otorisasi belum tersedia.

## Lifecycle media sesi

Mulai mengaktifkan live video/audio, recording, PCM/metrik, serta restream terpilih. Satu sesi nonterminal per engine, termasuk selama FINALIZING. Stop menutup admission WHEP/audio, menghentikan seluruh media, lalu memfinalisasi rekaman; upload dan pelaporan tetap berjalan. Playback hanya untuk channel sesi aktif dan tetap membutuhkan tiket cloud.

Command sukses menyatakan permintaan background telah diterima, bukan media sudah RUNNING atau finalisasi sudah STOPPED. Status lifecycle tersedia pada API operator engine GET `/api/v1/media/sessions/{id}`; kontrak heartbeat/metadata tidak ditambah field pada fase ini. Jangan menyatakan Stop selesai berdasarkan acknowledgment command saja. Endpoint mesin pendamping cloud tetap belum aktif.

Setelah restart, sesi nonterminal menjadi INTERRUPTED tanpa melanjutkan media. Segmen diperiksa/salvage; hasil gagal diberi alasan dan sumber dipertahankan. Gunakan ID sesi baru setelah keputusan operator. Cloud terputus tidak menghentikan recording aktif; pembukaan playback baru tetap ditolak jika grant tidak tersedia.
