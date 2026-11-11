# Dokumentasi local-app

Untuk instalasi dan cara menjalankan, mulai dari [README aplikasi](../README.md). Local-app berisi backend media; frontend, login dan penilaian merupakan bagian terpisah.

## Urutan belajar developer

1. [Alur end-to-end](end-to-end.md): hubungan startup, sesi, playback, rekaman, upload dan komunikasi cloud.
2. [Memahami media engine](media-engine.md): satu lifecycle Mulai/Stop, pipeline per channel, PCM/metrik, retry, finalisasi dan recovery.
3. [Peta kode](code-map.md): fungsi folder/file dan tautan ke source untuk menelusuri implementasi.

## Rujukan sesuai kebutuhan

| Panduan | Digunakan untuk |
| --- | --- |
| [Setup API](setup-api.md) | Menghubungkan NVR/kamera, discovery ONVIF, probe, konfigurasi terenkripsi, mode LAN, integrasi frontend, serta domain/HTTPS. |
| [Kontrak cloud](cloud-contract.md) | Command, heartbeat, metadata rekaman/upload, envelope HTTP dan otorisasi tiket playback. |
| [Penerimaan media](acceptance.md) | Skenario Start/Stop, kegagalan channel, restart, disk, upload, dan batas verifikasi sebelum operasional. |

Mode LAN dan mode cloud mempunyai aturan akses berbeda. Gunakan panduan setup sebagai rujukan kontrol lokal dan kontrak cloud sebagai rujukan komunikasi mesin. Tidak ada rekaman otomatis setelah setup atau restart.

## Swagger dan pemeliharaan dokumentasi

Swagger interaktif tersedia di [API lokal](http://localhost:8090/api/docs/index.html) setelah server berjalan; sesuaikan port dengan `HTTP_ADDR`. Request Try it out benar-benar dikirim. WHEP memakai SDP dan WebSocket memakai frame, sehingga Swagger tidak menggantikan client media.

Sumber dokumentasi API adalah anotasi pada handler dan [generate.go](../generate.go). Setelah kontrak berubah, jalankan dari root `local-app`:

```powershell
go generate .
go build ./...
```

Commit anotasi bersama [generated/docs.go](generated/docs.go) dan [generated/swagger.json](generated/swagger.json). Hasil generator tidak diedit manual; generator tidak diperlukan untuk menjalankan paket portable.

Setiap topik memiliki panduan utama di atas. Dokumen lain cukup menautkan. Jika memindahkan panduan, perbarui tautan source/antardokumen dan [script packaging portable](../scripts/package-portable.ps1). Paket portable berikutnya membawa panduan setup di `docs/setup-api.md`; paket yang sudah dibuat tidak diubah otomatis.
