# Dokumentasi Kicau Mania

[README proyek](../README.md) menjelaskan pembagian aplikasi. Direktori ini menyimpan penghubung lintas aplikasi dan catatan proyek. Petunjuk teknis masing-masing backend tetap dekat dengan source-nya.

## Jalur membaca panduan aktif

1. [Integrasi local-app dan cloud-app](integration.md): pembagian tanggung jawab, jalur bisnis/media, mode LAN/cloud, kredensial, gangguan dan batas integrasi.
2. [Panduan local-app](../local-app/docs/README.md) untuk media, atau [alur bisnis cloud-app](../cloud-app/docs/ALUR-BISNIS.md) dan [API cloud](../cloud-app/docs/API.md) untuk operasi bisnis.
3. [Database dan migrasi](../cloud-app/db/README.md), [ERD](../cloud-app/db/ERD.md), lalu [deployment VPS](../cloud-app/deployment/devops_runbook.md). Untuk belajar deployment dari awal, gunakan [panduan belajar](../cloud-app/deployment/BELAJAR-DEPLOYMENT.md).

Kontrak rinci tetap memiliki rujukan masing-masing: [kontrak engine lokal](../local-app/docs/cloud-contract.md) dan [kontrak pendamping cloud](../cloud-app/docs/ENGINE-CONTRACT.md). Dokumen integrasi menjelaskan hubungan dan statusnya tanpa menyalin payload/kontrak tersebut.

## Catatan rancangan dan pekerjaan

Dokumen berikut dipertahankan sebagai masukan/rancangan. Isinya tidak otomatis menjadi spesifikasi yang sudah diterapkan; cocokkan dengan panduan aktif dan source sebelum menggunakannya.

| Catatan | Konteks |
| --- | --- |
| [Brief local-app](notes/local-app-brief.md) | Instruksi awal engine dan target media; beberapa pendekatan sudah berkembang. |
| [Brief cloud-app](notes/cloud-app-brief.md) | Instruksi awal domain dan backend cloud, bukan konfigurasi operasional yang harus disalin. |
| [Edge cases lama](notes/edge-cases.md) | Rancangan offline-first penilaian. Pendekatan ini sudah digantikan bisnis online di cloud; bukan jaminan penilaian offline saat ini. |
| [Update script](notes/update-script.md) | Catatan perubahan yang dapat diperbarui untuk diskusi pekerjaan berikutnya; status penerapannya harus diperiksa, bukan diasumsikan. |

## Laporan historis

- [Progres ringkas](reports/progress-report.md).
- [Progres lengkap](reports/progress-report-full.md).

Kedua laporan merupakan snapshot **6 November 2026**. Pernyataan tentang SQLite, penilaian lokal, kontrol media terpisah atau sinkronisasi bisnis mencerminkan tahap sebelumnya dan bukan status implementasi terkini. Referensi dokumen lama yang telah dihapus ditandai sebagai historis, bukan diganti dengan dokumen yang artinya berbeda.

## Referensi dan hasil meeting

[Template server](reference/server-boilerplate.md) adalah referensi pola arsitektur. Seluruh stack, library, folder dan contoh dalam template bukan persyaratan wajib setiap aplikasi.

File teks berikut tetap berada di root repo, tanpa perubahan nama, lokasi atau isi:

- [Diskusi awal ERD](../discussion_11062026_erd.txt).
- [Diskusi target beres](../discussion_11092026_target%20beres.txt).
- [Catatan ERD teks](../ERD.txt).

Referensi Markdown yang tertulis di dalam hasil meeting dapat menyebut nama lama. Lokasi panduan sekarang ada pada indeks ini; file meeting tidak diedit untuk memperbarui referensi tersebut.

## Aturan penempatan

Panduan lintas aplikasi masuk `docs/`; panduan satu aplikasi tetap di dokumentasi aplikasi tersebut. Dokumen database dan deployment tetap dekat migrasi/script-nya. Catatan kerja masuk `notes/`, snapshot progres ke `reports/`, dan template ke `reference/`. Gunakan nama `kebab-case` untuk dokumen baru dan perbarui tautan ketika memindahkan file.
