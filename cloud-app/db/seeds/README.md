# Data mock dan pengujian Insomnia

Fixture ini hanya untuk database pengembangan/pengujian. Tidak dipanggil startup, migrasi atau pipeline deployment. SQL tidak menghapus data lama dan tidak menimpa baris yang sudah ada. Gunakan database terpisah untuk mengulang satu siklus penuh.

## 1. Terapkan seed secara eksplisit

Pastikan baseline dan migrasi 001–007 telah diterapkan sesuai [panduan database](../README.md). Dari folder cloud-app, contoh PowerShell untuk database lokal:

```powershell
$pgBin = 'C:/Program Files/PostgreSQL/18/bin'
& "$pgBin/psql.exe" -X -h localhost -p 5433 -U postgres -d kicau_mania -W -1 -v ON_ERROR_STOP=1 -f db/seeds/mock_data.sql
if ($LASTEXITCODE -ne 0) { throw 'Seed gagal; transaksi dibatalkan' }
```

Perintah tersebut akan menambahkan fixture ke database yang disebutkan. Untuk isolasi, ganti nama database dengan database pengujianmu. Jangan memakai perintah ini pada database operasional/VPS. Password PostgreSQL diminta oleh psql; tidak perlu menuliskannya dalam command.

Gunakan **-1 dan ON_ERROR_STOP**: seed menggunakan tabel sementara ON COMMIT DROP dan wajib berada pada satu transaksi. Jalankan sebagai pemilik database/objek atau akun yang memiliki izin insert seluruh tabel fixture dan pemeliharaan materialized view.

Seed memuat 85 baris: 8 akun, 2 arena, 48 gantangan, 2 event, 2 kategori, 3 ronde, 6 kriteria, 2 pemilik, 4 burung, 3 role global dan 5 penugasan arena. Tidak ada roster ronde, registrasi, pembayaran, skor, akun Google, sesi, token email, engine/device, atau rekaman tiruan. Materialized view disegarkan dalam transaksi seed.

Email seluruh akun memakai domain fiktif. Semua akun aktif, sudah terverifikasi khusus fixture, dan memakai password **KicauMock2026!** dengan hash Argon2id sesuai aplikasi. Ini tidak melewati verifikasi pada endpoint register yang sebenarnya.

| Role pengujian | Email | Hak awal |
| --- | --- | --- |
| superadmin | superadmin@mock.kicau.test | Superadmin global |
| admin_a | admin.a@mock.kicau.test | Admin arena A |
| operator_a | operator.a@mock.kicau.test | Operator arena A |
| operator_b | operator.b@mock.kicau.test | Operator arena B |
| juri_1 | juri.1@mock.kicau.test | Juri arena A |
| juri_2 | juri.2@mock.kicau.test | Juri arena A |
| peserta_1 | peserta.1@mock.kicau.test | Peserta/pemilik 1 |
| peserta_2 | peserta.2@mock.kicau.test | Peserta/pemilik 2 |

## 2. Peta UUID dan skenario

Semua UUID fixture memakai prefix `6d6f636b` (mock). Format: `6d6f636b-GGGG-4000-8000-NNNNNNNNNNNN`.

| GGGG | Entitas dan nomor akhir (12 digit, nol di depan) |
| --- | --- |
| 0001 | User 1–8 menurut urutan tabel akun di atas. |
| 0002 | Arena A=1, B=2. |
| 0003 | Gantangan A=1–24, B=101–124. |
| 0004 | Event A=1, B=2. |
| 0005 | Kategori A=1, B=2. |
| 0006 | Ronde A1=1, A2=2, B1=3. |
| 0007 | Kriteria A=1–3, B=101–103. |
| 0008 | Pemilik peserta 1=1, peserta 2=2. |
| 0009 | Burung peserta 1=1–2, peserta 2=3–4. |
| 000a | Role global: superadmin=1, peserta 1=2, peserta 2=3. |
| 000b | Penugasan arena: admin A=1, operator A=2, operator B=3, juri 1=4, juri 2=5. |
| 000c | Operation ID request koleksi, bukan baris seed. |
| 000d | Device ID 1–8 mengikuti urutan akun, bukan baris seed. |

Contoh arena A: `6d6f636b-0002-4000-8000-000000000001`. Seluruh nilai siap pakai juga berada pada Base Environment koleksi.

Event bertanggal 14 November 2026; seluruh ronde awal PREPARED dan bertipe Murai Batu. Kriteria contoh: Mock Irama, Mock Volume, Mock Gaya; nama ini bukan ketetapan penjurian produksi. Ronde A1 Rp50.000, A2 Rp75.000, B1 Rp50.000.

Koleksi mendaftarkan burung 1 dan 3, masing-masing ke A1+A2, sehingga setiap header Rp125.000. Verifikasi mengalokasikan gantangan A1 untuk peserta 1 dan A2 untuk peserta 2 pada kedua ronde. Nomor gantangan yang sama boleh digunakan kembali pada ronde berbeda.

Hanya ronde A1 dijalankan. Setiap juri memberi tiga nilai; peserta 1 awalnya 8/8/8 dari kedua juri, peserta 2 7/7/7. Revisi juri 1 menjadi 9/8/8 memberi peserta 1 total 49, rata-rata 49/6; peserta 2 total 42, rata-rata 7. Peserta 1 menjadi pemenang. Ronde A2 tetap PREPARED dengan peserta terkonfirmasi; B1 tidak disentuh.

## 3. Menjalankan API dan import Insomnia

Sesuaikan .env lokal, tanpa mengganti DATABASE_URL ke VPS. Untuk menguji refresh cookie lewat HTTP localhost, gunakan:

```dotenv
HTTP_ADDR=127.0.0.1:8091
FRONTEND_PUBLIC_URL=http://localhost:3000
ALLOWED_ORIGINS=http://localhost:3000
```

Restart API setelah mengubah konfigurasi. Frontend di port 3000 tidak perlu dijalankan untuk request Insomnia; nilai tersebut dipakai sebagai Origin yang diizinkan. Konfigurasi produksi tetap HTTPS dan tidak perlu diubah karena fixture.

```powershell
go run ./cmd/api -env .env
```

Di Insomnia pilih Import → From File → [cloud-app-mock.json](../../docs/insomnia/cloud-app-mock.json), lalu pilih environment **Localhost**. Koleksi memakai format JSON v4 yang masih dapat diimport Insomnia; dibutuhkan versi yang mendukung after-response scripts.

Jalankan folder **00 sampai 10** dari atas ke bawah. Setiap request mempunyai status yang diharapkan dan assertion. Script mengambil token, ID registration/item/payment serta versi dari envelope `data`; tidak perlu menyalin ID response secara manual. Berhenti jika request gagal karena request sesudahnya dapat membutuhkan variabel hasilnya.

Script membaca kode HTTP melalui `insomnia.response.code`; `insomnia.response.status` berisi teks status seperti `OK`, bukan angka `200`. Jika memakai koleksi lama dengan error `Unexpected status; variables were not updated` meskipun response 200, ubah baris `const status = insomnia.response.status;` menjadi `const status = insomnia.response.code;` pada Scripts → After-response, atau import ulang koleksi terbaru sebagai koleksi terpisah. Impor ulang file tidak otomatis memperbarui script pada koleksi lama. Bila kode HTTP memang berbeda, lihat status dan body response sebelum melanjutkan. `base_url` hanya origin, tanpa `/api/v1`.

Login delapan role menyimpan token berbeda, misalnya `token_operator_a` dan `token_juri_1`. Cookie pada login bisnis tidak disimpan/dikirim agar tidak bertukar antaraktor. Access token berlaku 15 menit; jika 401 karena kedaluwarsa, login ulang role itu lalu lanjutkan. Device ID tetap sehingga login ulang memakai slot yang sama.

Pembacaan variabel pada script memakai `insomnia.variables.get()` agar menemukan nilai dari environment terpilih maupun Base Environment. `insomnia.environment.get()` hanya membaca scope environment terpilih; ID fixture di Base Environment dapat tidak ditemukan. Penulisan token/ID tetap melalui `insomnia.environment.set()` agar hasil localhost dan VPS tidak tercampur. Untuk koleksi lama, import file terbaru sebagai koleksi terpisah dan pilih environment yang sesuai; login ulang setiap role untuk mengisi token baru.

Request create registration dijalankan sekali pada fixture baru. Retry submit memakai operation ID dan payload awal yang identik, termasuk expected_version=0. Revisi membaca versi lewat request 'Baca versi sebelum revisi' lalu menyimpannya sebagai revision_base_version; response revisi tidak menimpa nilai dasar itu. Retry revisi tetap memakai payload yang sama. Untuk revisi baru berikutnya, buat operation ID baru dan ambil versi terbaru.

## 4. Request negatif dan cookie sesi

Folder 90 **bukan** bagian yang dapat dijalankan seluruhnya sesudah ronde ditutup. Ikuti prasyarat masing-masing:

| Kasus | Kapan | Hasil |
| --- | --- | --- |
| Tanpa token | Kapan saja. | 401 |
| Operator B membuka laporan arena A | Sesudah login operator B. | 403 |
| Nominal pembayaran salah | Sesudah registrasi P1, sebelum pembayaran/verifikasi. | 400 |
| Skor 11 | Setelah begin-judging, sebelum submit awal. | 400 |
| Menutup sebelum seluruh nilai lengkap | Setelah begin-judging, sebelum seluruh submit. | 409 |
| Versi revisi stale | Setelah revisi menjadi versi 2, sebelum close. | 409 |
| Pemenang bukan nilai tertinggi | Setelah semua nilai masuk, sebelum close sah. | 400 |
| Revisi baru setelah CLOSED | Setelah close; gunakan operation ID baru. | 409 |

Folder 91 dijalankan berurutan setelah alur utama: login peserta 1 dengan penyimpanan cookie aktif → refresh → daftar sesi → logout → akses token lama ditolak. Pada folder ini cookie jar sengaja digunakan. Jangan menyisipkan login akun lain selama pengujian cookie. Logout mencabut sesi peserta 1; login kembali bila hendak memakai akunnya lagi.

Laporan materialized view dapat tertinggal. Periksa snapshot_at dan refreshed_at, lalu ulangi GET bila diperlukan; penentuan pemenang memakai rekap langsung. Seed dan startup worker memuat snapshot awal; pembayaran/close memberi notifikasi refresh, sementara submit nilai tidak memicu refresh per request.

## 5. Mengulang dan batas verifikasi

Seed ulang tidak mengubah password, nama yang sudah ada, status ronde, pembayaran, atau nilai. Fase CLOSED tidak kembali PREPARED. Jika identitas fixture pada ID yang sama tidak sesuai atau natural key sudah dipakai ID lain, transaksi gagal seluruhnya. Perbaiki benturan secara eksplisit; jangan menghapus data pengguna lain untuk memaksa seed berhasil.

Untuk satu pengujian end-to-end baru, gunakan database sementara baru, restore baseline, jalankan migrasi dan seed, lalu import koleksi sebagai workspace baru agar variabel ID/token lama kosong. Tidak disediakan reset destruktif otomatis.

Seed tidak mendaftarkan provider Google atau token email palsu. Register/verifikasi/forgot/reset lengkap membutuhkan SMTP pengujian yang bisa dibaca; Google membutuhkan ID token asli. Endpoint terkait dapat 503 saat layanan belum dikonfigurasi. Hasil bisnis melalui fixture tidak membuktikan integrasi email, Google, frontend browser, maupun streaming.

Suite tes Go tidak dijalankan. Pemeriksaan fixture harus memakai PostgreSQL sementara; seed database utama hanya melalui perintah operator yang eksplisit. Catatan hasil verifikasi implementasi ditambahkan setelah pemeriksaan selesai.


## Environment VPS dev

Koleksi juga menyediakan environment VPS dev: base_url `https://dev.megaallstararena.com`, sehingga path request tetap `/api/v1/...`. Ini tidak mengisi database VPS otomatis. Akun mock hanya tersedia bila operator secara eksplisit menerapkan seed pada database pengujian yang dituju; sebelum itu, login mock di VPS akan ditolak.
