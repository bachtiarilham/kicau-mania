# Kontrak baca PostgreSQL cloud-app

SQL ini memakai skema dari `db-dump/full_db.dump` pada PostgreSQL 18. Ada 16 view untuk list/detail dan satu materialized view untuk rekap administratif. Integrasi repository Go merupakan fase terpisah. Cloud exchange masih berjalan dengan SQLite.

## Restore dan migrasi

Jalankan dari root `kicau-mania` dengan CLI PostgreSQL 18 di PATH. Contoh memakai instance pengembangan lokal pada port 5432. Ganti port dan akun sesuai instance pengembangan. `createdb` membuat database baru, sehingga hentikan rangkaian jika nama database sudah dipakai.

```powershell
$dbName = 'kicau_read_verify_' + (Get-Date -Format 'yyyyMMddHHmmss')
$pgConn = @('-h', '127.0.0.1', '-p', '5432', '-U', 'postgres')
createdb @pgConn $dbName
if ($LASTEXITCODE -ne 0) { throw 'Gagal membuat database sementara' }
pg_restore @pgConn --exit-on-error --no-owner --no-privileges -d $dbName db-dump/full_db.dump
if ($LASTEXITCODE -ne 0) { throw 'Restore gagal' }
foreach ($migration in (Get-ChildItem cloud-app/db/migrations/*.up.sql | Sort-Object Name)) {
    psql -X @pgConn -d $dbName -v ON_ERROR_STOP=1 -1 -f $migration.FullName
    if ($LASTEXITCODE -ne 0) { throw "Migrasi gagal: $($migration.Name)" }
}
```

Jalankan migrasi sebagai pemilik objek SQL dengan akses baca ke tabel sumber. Migrasi 003 membutuhkan wewenang membuat role jika role grup belum tersedia. Role `NOLOGIN` ini berada pada tingkat cluster dan dapat dipakai kembali lintas database.

| Role grup | Hak di `cloud_read` |
| --- | --- |
| `kicau_cloud_reader` | List/detail operasional lomba, aset kecuali perangkat, metadata rekaman, kriteria, dan membership. |
| `kicau_cloud_admin_reader` | Seluruh view operasional ditambah perangkat, IAM, skor, dan ringkasan skor. |
| `kicau_cloud_recap_maintainer` | `USAGE` schema dan `MAINTAIN` pada materialized view rekap. |

Berikan role grup ke akun layanan yang sesuai saat repository diintegrasikan. Otorisasi akses baris per pengguna atau event tetap menjadi tugas aplikasi. Privilege efektif juga bergantung pada membership dan grant lain di database tujuan. Role baca dari migrasi ini mendapat grant pada view. Contoh mutasi membutuhkan role penulis tersendiri dengan privilege tabel dan kolom yang dipakai `WHERE` serta `RETURNING`.

## Query aplikasi

Contoh `PREPARE` ada di [asset.sql](queries/asset.sql), [competition.sql](queries/competition.sql), [iam.sql](queries/iam.sql), [judging.sql](queries/judging.sql), [media.sql](queries/media.sql), dan [member.sql](queries/member.sql). File hanya menyiapkan statement. Contoh ini menyiapkan query dan menampilkan satu halaman event di sesi `psql` yang sama.

```powershell
psql -X @pgConn -d $dbName -v ON_ERROR_STOP=1 `
    -f cloud-app/db/queries/competition.sql `
    -c 'EXECUTE list_events(25, 0);'
```

Pada repository Go, gunakan isi statement setelah `PREPARE … AS` dengan parameter `$1`, `$2`, dan seterusnya. List dengan filter menerima ID scope, limit, lalu offset. List tanpa filter menerima limit dan offset. Detail menerima ID entitas. Jalankan contoh mutasi di dalam transaksi, ambil `id` dari `RETURNING`, dan baca detail dari view dalam transaksi yang sama sebelum commit. Contoh berikut dapat dicoba pada database sementara. Variabel `bird_id` berasal dari hasil `EXECUTE` pertama.

```sql
\i cloud-app/db/queries/asset.sql
BEGIN;
EXECUTE create_bird(NULL, 'demo', 'Burung demo') \gset bird_
EXECUTE get_birds(:'bird_id');
EXECUTE update_bird(:'bird_id', NULL, 'demo', 'Nama baru');
EXECUTE get_birds(:'bird_id');
EXECUTE delete_bird(:'bird_id');
ROLLBACK;
```

## Refresh rekap

`cloud_read.mv_round_score_summary` berisi snapshot per peserta ronde. Rata-rata hanya memakai skor bulat 1–10 dengan bobot sama. Skor lain masuk hitungan invalid. Peserta tanpa skor mendapat count nol dan rata-rata `NULL`. Gunakan rekap untuk administrasi. Ranking dan kelengkapan resmi memerlukan roster, revisi, dan status final dari arena.

Migrasi mengisi snapshot pertama dan membuat indeks unik yang diperlukan untuk refresh concurrent. Setelah impor atau koreksi skor commit, jalankan refresh manual dari sesi pemeliharaan. Refresh menghitung ulang seluruh snapshot. Pembaca dapat tetap mengakses snapshot selama refresh, dan permintaan baca berikutnya mendapatkan hasil baru setelah refresh selesai.

```sql
REFRESH MATERIALIZED VIEW CONCURRENTLY cloud_read.mv_round_score_summary;
```

## Rollback

Jalankan migrasi `.down.sql` dari nomor terbesar ke terkecil. Rollback mencabut grant, menghapus rekap dan indeksnya, lalu menghapus view dan schema `cloud_read`. Role grup tetap tersedia karena bersifat lintas database. Objek sumber dari dump tetap tersedia.

```powershell
foreach ($migration in (Get-ChildItem cloud-app/db/migrations/*.down.sql | Sort-Object Name -Descending)) {
    psql -X @pgConn -d $dbName -v ON_ERROR_STOP=1 -1 -f $migration.FullName
    if ($LASTEXITCODE -ne 0) { throw "Rollback gagal: $($migration.Name)" }
}
```
