# Feature Map - Penyimpanan Rekaman Cloud, Unduhan, dan WhatsApp

Dokumen ini mencatat alur bisnis target untuk penerimaan video dari Local App dan distribusi rekaman kepada peserta.

## Penerimaan video

- Local App mengunggah otomatis video yang siap setelah ronde selesai. Jika ronde berikutnya dimulai, upload terjeda sampai ronde berhenti.
- Cloud App menyediakan akses unduhan dan menyiapkan tautan peserta setelah upload berhasil. Satu video yang belum selesai diunggah tetap menunggu, sementara akses untuk video lain yang sudah selesai bisa tersedia.

## Tautan peserta dan WhatsApp

- Cloud App memicu pengiriman tautan WhatsApp ke nomor yang tercatat pada pendaftaran peserta untuk gantangan terkait. Pemicu distribusi berada di Cloud App.
- Peserta membuka tautan tonton Cloud App di browser dan dapat mengunduh file dari halaman tersebut. Fitur unduh tersedia hanya di Cloud App.

## Masa simpan lima hari

- Masa simpan dimulai pada saat setiap video berhasil diunggah. Tautan tonton dan akses unduhan peserta berlaku selama lima hari sejak waktu tersebut.
- Saat masa simpan habis, Cloud App menolak akses tonton dan unduh, lalu menghapus file video dari penyimpanan Cloud.
- File asli tetap tersimpan di PC operator. Penghapusan dari Cloud tidak memicu penghapusan lokal atau upload ulang video yang sudah kedaluwarsa.
