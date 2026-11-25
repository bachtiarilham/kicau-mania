# Data mock schema 012

Fixture khusus database pengembangan, diterapkan manual. Seed tidak dijalankan saat startup, migrasi, atau deployment. Database harus sudah melalui migrasi 012; lihat [panduan database](../README.md).

## Akun

Semua akun aktif dan terverifikasi, dengan password **`KicauMock2026!`**. Jangan gunakan kredensial fixture untuk produksi.

| Key Insomnia | Email | Nama | Penugasan |
| --- | --- | --- | --- |
| admin | admin@mock.kicau.test | Hendra Wijaya | ADMIN global dan arena Kemang + Bandung |
| admin_a | admin.a@mock.kicau.test | Budi Santoso | ADMIN arena Kemang & Bandung |
| operator_a | operator.a@mock.kicau.test | Rudi Hartono | OPERATOR arena Kemang |
| operator_b | operator.b@mock.kicau.test | Agus Salim | OPERATOR arena Bandung |
| juri_1 | juri.1@mock.kicau.test | Dedi Supriadi | JURI arena Kemang |
| juri_2 | juri.2@mock.kicau.test | Eko Prasetyo | JURI arena Kemang |
| peserta_1 | peserta.1@mock.kicau.test | Slamet Riyadi | PESERTA, pemilik burung Raja Rimba & Naga Bonar |
| peserta_2 | peserta.2@mock.kicau.test | Andi Pratama | PESERTA, pemilik burung Black Mamba & Si Pitung |
| *(tambahan)* | juri.3@mock.kicau.test | Wahyu Hidayat | JURI arena Kemang & Bandung |
| *(tambahan)* | juri.4@mock.kicau.test | Fajar Nugroho | JURI arena Bandung |
| *(tambahan)* | peserta.3@mock.kicau.test | Joko Susilo | PESERTA, pemilik burung Bintang Timur & Krakatau |
| *(tambahan)* | peserta.4@mock.kicau.test | Reza Pahlevi | PESERTA, pemilik burung Garuda Sakti & Srikandi |

## Isi seed

Seluruh 18 tabel pada schema `asset`, `competition`, dan `iam` (ditambah `judging.score_criteria`) terisi dengan data realistis:
- **IAM**: 12 akun lengkap (`nickname`, `address`, `position`), 4 role (`ADMIN`, `OPERATOR`, `JURI`, `PESERTA`), 5 role global, 11 penugasan arena, 4 tautan Google OAuth (`user_auth_providers`), 4 sesi perangkat aktif (`user_sessions`), dan 12 catatan `user_master_history` (via trigger).
- **Asset**: 2 arena (`Gantangan Mega Allstar Kemang` & `Gantangan Taman Impian Bandung`), 48 gantangan (4×6 per arena), 5 spesies (`Murai Batu`, `Cucak Ijo`, `Kacer`, `Lovebird`, `Kenari`), 8 burung juara (`BRD-000001`..`BRD-000008`), 9 catatan kepemilikan (termasuk histori take-over), 8 catatan `bird_master_history` (via trigger), dan 6 sertifikat penghargaan internal & eksternal (`bird_award_certificate_history`).
- **Competition**: 2 event (`Piala Walikota Jakarta Selatan 2026` & `Latpres Akhir Bulan Bandung`), 4 kategori (`EKS`, `BTG`, `VIP`, `REG`), 5 ronde (`PREPARED`, `RUNNING`, `JUDGING`, `CLOSED` lengkap dengan jadwal & susunan juri), serta 10 registrasi tiket (`CASH`/`TRANSFER`, `VERIFIED`/`PENDING`, `ACTIVE`/`DISQUALIFIED`). Ronde A1 tetap `PREPARED` tanpa registrasi agar skenario Insomnia `00 Mock` tetap dapat dijalankan.

## Menjalankan

Dari folder `cloud-app`, contoh PowerShell untuk database lokal:

```powershell
$pgBin = 'C:/Program Files/PostgreSQL/18/bin'
& "$pgBin/psql.exe" -X -h localhost -p 5433 -U postgres -d kicau_mania -W -1 -v ON_ERROR_STOP=1 -f db/seeds/mock_data.sql
if ($LASTEXITCODE -ne 0) { throw 'Seed gagal; transaksi dibatalkan' }
```

Password database diminta oleh psql. Pilih database pengujian yang benar sebelum menjalankan. `-1` dan `ON_ERROR_STOP` wajib: seluruh seed berada dalam satu transaksi, termasuk tabel sementara dan pembaruan snapshot. Akun pelaksana memerlukan izin insert fixture serta refresh materialized view; pengujian juga dilakukan dengan role `kicau_cloud_app`.

Pada 15 November 2026, fixture ini sudah diterapkan pada **VPS dev**. Salinan operasional ada di `/opt/apps/kicau-mania/database/seeds/mock_data.sql`. Penerapan di database utama localhost tetap manual. Contoh penerapan ulang di VPS:

```bash
docker exec -i kicau_mania_db sh -c 'exec psql -X -1 -v ON_ERROR_STOP=1 -f - -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < /opt/apps/kicau-mania/database/seeds/mock_data.sql
```

Jangan menjalankannya bersamaan dengan pemeliharaan/deployment database.

Seed ulang melewati ID yang sudah ada: password, versi master, kepemilikan terkini, dan fase ronde tidak dikembalikan ke kondisi awal. Ketidakcocokan identitas fixture membatalkan transaksi. Seed ulang **bukan reset**; gunakan database pengujian baru untuk mengulang seluruh skenario setelah ronde CLOSED.

## Peta UUID

Format `6d6f636b-GGGG-4000-8000-NNNNNNNNNNNN`, dengan nomor akhir 12 digit.

| GGGG | Entitas |
| --- | --- |
| 0001 | Akun 1–8 sesuai tabel di atas |
| 0002 | Arena A=1, B=2 |
| 0003 | Gantangan A=1–24, B=101–124 |
| 0004 | Event A=1, B=2 |
| 0005 | Kategori A=1, B=2 |
| 0006 | Ronde A1=1, A2=2, B1=3 |
| 0007 | Kriteria A=1–3, B=101–103 |
| 0009 | Burung peserta 1=1–2, peserta 2=3–4 |
| 000a | Role global ADMIN=1, peserta 1=2, peserta 2=3 |
| 000b | Penugasan arena 1–7 |
| 000e | Spesies Mock Murai Batu=1 |
| 000f | History kepemilikan awal 1–4 |

Contoh arena A: `6d6f636b-0002-4000-8000-000000000001`. Semua ID untuk request tersedia di environment koleksi.

## Pengujian API

Ikuti [panduan Insomnia](../../docs/insomnia/README.md). Folder `00 Mock` menyediakan 39 langkah: login delapan role, pemeriksaan hak akses, registrasi dua peserta ke ronde A1 dengan biaya Rp50.000 per tiket, verifikasi CASH, roster dua juri, penilaian tiga kriteria, revisi, diskualifikasi/requalify, dan penutupan.

Nilai contoh memakai string desimal sesuai API terkini. Ronde A1 menjadi CLOSED setelah skenario; A2 dan B1 tetap PREPARED. Tidak ada bracket/pemenang otomatis pada kontrak API ini. Request registrasi akun, verifikasi email/reset lengkap memerlukan SMTP pengujian; login Google memerlukan konfigurasi Google asli.

Verifikasi dilakukan pada database sementara: seed berulang, preservasi perubahan master/password/fase, login delapan akun, dan seluruh 39 langkah HTTP. Suite tes Go tidak dijalankan. Import langsung melalui UI Insomnia belum diverifikasi.
