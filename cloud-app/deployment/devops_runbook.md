# Deployment cloud-app â€” GitHub Actions, Docker dan VPS

Dokumen ini menjadi sumber operasional utama. Template `/opt` berada di [opt](opt/). Frontend dan local-app tidak dideploy oleh pipeline ini.

## Alur dan kontrak deployment

```text
PR â†’ build Go + vet + validasi konfigurasi + acceptance container + build AMD64/ARM64
main â†’ pemeriksaan yang sama â†’ GHCR privat â†’ SSH â†’ pull digest â†’ ganti app â†’ readiness
                                                                    â†“ gagal
                                                       pulihkan rilis sebelumnya
```

API: `https://dev-api.megaallstararena.com`. Frontend: `https://dev.megaallstararena.com`. Push yang hanya mengubah local-app tidak memicu pipeline. Tidak ada suite tes Go otomatis; acceptance shell menjalankan container dan SQL sintetis.

Image: `ghcr.io/bachtiarilham/kicau-mania-cloud:<commit-sha>`. VPS memakai digest dari hasil build, bukan tag `latest`. Image mencakup API dan bootstrap-admin. Runtime non-root, filesystem read-only, CA certificates, log stdout berotasi dan direktori log sementara terbatas 16 MiB. Tidak ada secrets/cert/data yang dibundel.

Deployment satu container mempunyai jeda singkat. Pemeriksaan readiness mencakup database/skema dan akses melalui Nginx; ini bukan pengujian Google/SMTP atau seluruh alur bisnis. Rollback image tidak membatalkan migrasi database atau perubahan data.

## Struktur VPS

```text
/opt/infra/                         # Compose Nginx, nginx/, certs/, acme/
/opt/apps/kicau-mania/
  docker-compose.yml               # Template untuk pengelolaan awal database
  .env                             # Interpolasi Compose, APP_IMAGE
  app.env                          # Kredensial service dan konfigurasi aplikasi
  database.env                     # Kredensial administrator PostgreSQL saja
  data/postgres18/                  # Mount /var/lib/postgresql; terpisah dari data PG17
  data/backups/
  incoming/<commit>/               # Bundle tanpa rahasia dari CI
  releases/<commit>-<time>-<pid>/   # Compose, digest, snapshot app.env (mode privat)
  state/current                    # Symlink rilis terakhir yang siap
  state/deployments.log
  state/migrations.log
  state/deploy.lock
/opt/scripts/{init,db,app}/
```

`releases` mengandung salinan rahasia app.env: batasi direktori dan jangan salin ke Git. Rahasia bootstrap/role database dibaca interaktif; tidak diberikan sebagai argumen CLI. Script tidak melakukan pruning image otomatis.

## 1. Persiapan awal (sekali, manual)

Periksa OS/CPU dengan `cat /etc/os-release; uname -m`. Dukungan: Debian/Ubuntu pada x86_64 atau aarch64. Installer menolak platform lain. Sediakan Docker Engine + Compose v2+, curl, OpenSSL, util-linux dan tar. Jalankan `scripts/init/install-docker.sh` sebagai root hanya jika Docker belum terpasang, lalu `scripts/init/preflight.sh` sebagai operator Docker.

Siapkan akun Linux khusus deployment dengan login SSH key dan akses Docker. Akses Docker memberi wewenang administratif pada host; gunakan akun ini khusus deployment. Direktori aplikasi dan `data/backups` harus dapat ditulis akun tersebut; buat direktori backup sebelum menyalakan Compose agar tidak dibuat root oleh Docker. Nginx/konfigurasi infrastruktur dipasang oleh administrator secara manual.

Dari checkout commit yang akan digunakan, buat bundle lalu transfer ke VPS dengan scp:

```bash
bash cloud-app/deployment/package.sh "$(git rev-parse HEAD)" /tmp/cloud-release.tgz
# Transfer melalui workstation; sesuaikan user, host dan port SSH.
scp /tmp/cloud-release.tgz deploy@VPS:/tmp/cloud-release.tgz
```

Di VPS, contoh dengan user Linux `deploy`:

```bash
sudo install -d -m 750 -o deploy -g deploy /opt/apps/kicau-mania /opt/infra /opt/scripts
umask 077
mkdir -p /opt/apps/kicau-mania/incoming/bootstrap
cd /opt/apps/kicau-mania/incoming/bootstrap
tar -xzf /tmp/cloud-release.tgz
RELEASE=$PWD
cp "$RELEASE/stack/docker-compose.yml" /opt/apps/kicau-mania/
cp "$RELEASE/stack/.env.example" /opt/apps/kicau-mania/.env
cp "$RELEASE/stack/app.env.example" /opt/apps/kicau-mania/app.env
cp "$RELEASE/stack/database.env.example" /opt/apps/kicau-mania/database.env
cp -R "$RELEASE/infra/." /opt/infra/
cp -R "$RELEASE/scripts/." /opt/scripts/
find /opt/scripts -type f -name '*.sh' -exec chmod 750 {} +
chmod 600 /opt/apps/kicau-mania/{.env,app.env,database.env}
mkdir -p /opt/apps/kicau-mania/data/{postgres18,backups} /opt/infra/acme
```

Edit ketiga environment file di VPS; jangan menyalin `.env` localhost. `database.env` hanya diberikan ke PostgreSQL. `app.env` hanya diberikan ke aplikasi. Password DATABASE_URL harus di-URL-encode. Contoh template tidak berisi kredensial nyata. Gunakan nilai APP_IMAGE digest valid (64 karakter hex) pada `.env`; image aplikasi belum perlu tersedia untuk menyalakan hanya PostgreSQL.

Jaringan gateway default menggunakan subnet `172.30.20.0/24`, IP Nginx `172.30.20.2`, dan `TRUSTED_PROXIES=172.30.20.2/32`. Periksa konflik dengan jaringan VPS sebelum membuat:

```bash
docker network create --subnet 172.30.20.0/24 web_gateway
/opt/scripts/init/preflight.sh
```

Jika subnet tersebut sudah dipakai, pilih subnet bebas; ubah IP statis pada environment Compose infrastruktur (`GATEWAY_NGINX_IP`), TRUSTED_PROXIES aplikasi, dan `GATEWAY_SUBNET` saat menjalankan preflight agar semuanya konsisten. Jangan hapus jaringan layanan lain.

## 2. PostgreSQL 18 dan skema awal

Compose mempublikasikan PostgreSQL hanya ke `127.0.0.1:5432`. Dari aplikasi gunakan hostname `postgres` dan port `5432`; port localhost pengembangan `5433` tidak dipakai di jaringan container.

**Jangan** mengganti image 17 menjadi 18 dengan direktori data 17 yang sama. PostgreSQL 18 menggunakan `data/postgres18` baru. Jika stack lama ada, backup dengan image versi lamanya dan hentikan container lama secara eksplisit sebelum setup stack ini. Jangan menjalankan `down -v`. Hasil unduhan folder VPS bukan backup database.

```bash
cd /opt/apps/kicau-mania
docker compose up -d --wait postgres
/opt/scripts/db/initialize-db.sh /opt/apps/kicau-mania/incoming/bootstrap
/opt/scripts/db/grant-access.sh cloud_service app
```

Initializer hanya menerima database kosong: restore baseline dan migrasi 001â€“007, transaksi per file serta ON_ERROR_STOP. Database tidak kosong ditolak tanpa dihapus. Tidak ada hard reset otomatis pada deploy. Untuk VPS lama yang memang boleh dibangun ulang, administrator menamai ulang direktori PG17 sebagai arsip dan menggunakan direktori PG18 baru; penggantian database selalu terpisah dari pipeline.

Masukkan password login `cloud_service` yang baru dibuat ke DATABASE_URL dalam app.env. Akun ini bukan superuser dan hanya mendapat role `kicau_cloud_app`. Superuser PostgreSQL hanya dipakai untuk operasi database.

Jika inisialisasi gagal di tengah, jangan mengulang dari awal: periksa `state/migrations.log` dan objek database, lalu terapkan file gagal/berikutnya secara eksplisit. Jangan melompati migrasi yang belum berhasil.

## 3. DNS, TLS dan Nginx

Di Hostinger, buat A record `dev-api` menuju IPv4 publik VPS. Tambahkan AAAA hanya jika IPv6 VPS benar-benar melayani domain tersebut. Buka inbound 80/443 dan port SSH sesuai konfigurasi host; PostgreSQL tidak dibuka ke internet.

Gunakan sertifikat yang cocok untuk `dev-api.megaallstararena.com`. Sertifikat hasil unduhan VPS mungkin untuk domain lain. Jangan commit `certs`, `privkey.pem`, `.env`, atau database dump cadangan.

Contoh penerbitan pertama memakai Certbot host (sebelum Nginx dinyalakan; port 80 harus bebas):

```bash
sudo apt-get install -y certbot
sudo certbot certonly --standalone \
  --config-dir /opt/infra/certs --work-dir /var/lib/letsencrypt --logs-dir /var/log/letsencrypt \
  -d dev-api.megaallstararena.com
cd /opt/infra
docker compose up -d
docker exec global_reverse_proxy nginx -t
```

Nginx membaca `/etc/letsencrypt/live/dev-api.megaallstararena.com/{fullchain,privkey}.pem`; mount seluruh direktori certs agar symlink ke archive valid. Konfigurasi default menolak host tak dikenal. Docker DNS dipakai untuk menemukan container aplikasi yang berganti IP.

Setelah penerbitan pertama, atur renewal webroot memakai `/opt/infra/acme`:

```bash
sudo certbot renew --cert-name dev-api.megaallstararena.com --webroot -w /opt/infra/acme \
  --config-dir /opt/infra/certs --work-dir /var/lib/letsencrypt --logs-dir /var/log/letsencrypt --dry-run
```

Cron root dua kali sehari, tanpa `--dry-run`:

```cron
15 0,12 * * * certbot renew --webroot -w /opt/infra/acme --config-dir /opt/infra/certs --work-dir /var/lib/letsencrypt --logs-dir /var/log/letsencrypt --quiet && docker exec global_reverse_proxy nginx -s reload
```

Pastikan host memakai zona waktu yang diinginkan. Nginx tidak mencatat query string, token/header auth atau body request. Forwarded IP ditulis ulang oleh gateway, bukan dipercaya dari browser. Jika nanti ada CDN/proxy tambahan, konfigurasi real IP perlu ditinjau ulang.

## 4. GitHub dan GHCR

Buat GitHub Environment **development**. Daftarkan secrets:

| Secret | Isi |
| --- | --- |
| VPS_HOST | Hostname/IPv4 VPS, tanpa skema atau username. |
| VPS_PORT | Port SSH; kosong berarti 22. |
| VPS_USER | Akun Linux khusus deployment. |
| VPS_SSH_KEY | Private key untuk login akun tersebut. |
| VPS_KNOWN_HOSTS | Baris known_hosts yang sidik jarinya diverifikasi melalui konsol VPS/provider; port nonstandar memakai `[host]:port`. |

Pasang public key pada authorized_keys akun VPS. Jangan hanya mengambil host key dari jaringan tanpa mencocokkan fingerprint melalui jalur tepercaya. Workflow selalu StrictHostKeyChecking dan BatchMode.

GitHub Actions memublikasikan image menggunakan GITHUB_TOKEN dengan packages:write. Pastikan package GHCR **private** dan terhubung dengan repository; workflow berhenti jika visibility bukan private. Fork PR tidak login/push GHCR dan tidak memperoleh secrets deployment.

Login GHCR sekali di VPS **sebagai akun deployment**, memakai PAT classic `read:packages` yang diizinkan membaca package privat:

```bash
read -r -s -p 'GHCR read token: ' GHCR_TOKEN; printf '\n'
printf '%s' "$GHCR_TOKEN" | docker login ghcr.io -u NAMA_AKUN_GITHUB --password-stdin
unset GHCR_TOKEN
```

Konfigurasi SMTP/Google tetap berada di app.env VPS, bukan GitHub workflow. GHCR token read tidak disertakan di image atau bundle.

## 5. Deploy pertama dan deploy berikutnya

Setelah PostgreSQL/skema, service login, Nginx/TLS dan SSH siap, push commit cloud-app ke `main`. Pipeline memublikasikan kedua arsitektur, mengirim bundle commit yang sama, dan mengaktifkan app. Jika setup belum lengkap, job deployment gagal tanpa membangun ulang atau menghapus database; perbaiki setup lalu rerun workflow.

Deployment menyimpan snapshot app.env, Compose, commit dan digest dalam releases. Lock host juga menghalangi migrasi/restore bersamaan. Symlink current hanya berubah setelah readiness internal dan melalui HTTPS berhasil. Timeout readiness default 120 detik, polling 3 detik. Public URL tidak membawa token.

Jika gagal, script mencoba mengaktifkan kembali rilis sebelumnya memakai image lokal dan snapshot konfigurasi sebelumnya, lalu memeriksa readiness. Workflow tetap gagal walaupun rollback berhasil. Rilis pertama yang gagal dihentikan karena belum ada rilis sebelumnya. Periksa `state/deployments.log` untuk `ready`, `failed`, `rollback-ready`, atau `rollback-failed`.

Buat superadmin sekali setelah deployment pertama berhasil:

```bash
/opt/scripts/app/bootstrap-admin.sh
```

Script meminta nama/email/nomor E.164/password, memakai environment proses dan binary yang sama, tanpa password dalam argumen CLI. Bootstrap bukan bagian pipeline dan tidak membuat akun bawaan.

Perubahan app.env berlaku pada deployment selanjutnya. Jika deployment gagal, konfigurasi rilis sebelumnya ikut dipulihkan. Penghapusan rilis lama beserta rahasianya dan image rollback merupakan maintenance manual setelah memastikan rilis aktif/rollback masih tersedia.

## 6. Migrasi, backup, restore dan akses rekan kerja

Migrasi **tidak pernah** dijalankan oleh workflow. Deploy aplikasi yang membutuhkan skema baru harus didahului migrasi manual yang kompatibel, atau dilakukan dalam maintenance terencana.

```bash
/opt/scripts/db/backup-db.sh
docker stop -t 20 kicau_mania_app
/opt/scripts/db/migrate.sh /path/008_description.up.sql
# Aktifkan rilis yang kompatibel melalui pipeline; jangan otomatis menjalankan down.sql.
```

Script migrate menerima tepat satu nama file up.sql dan mencatat checksum/waktu sesudah commit. Ini catatan operasional, bukan migration runner: operator memastikan file belum pernah diterapkan. Jika menggunakan psql langsung, gunakan -X -1 -v ON_ERROR_STOP=1 dan catat versi/checksum secara konsisten.

Cron backup pukul 02:00 WIB jika zona waktu host Asia/Jakarta:

```cron
0 2 * * * /opt/scripts/db/backup-db.sh >> /var/log/kicau-db-backup.log 2>&1
```

Backup custom format ditulis ke `.partial`, di-rename setelah sukses, baru retensi snapshot >7 hari dijalankan. Salin backup penting ke lokasi lain: salinan di disk VPS yang sama tidak melindungi dari kehilangan VPS.

Restore memakai **path host**, tanpa terminal interaktif:

```bash
/opt/scripts/db/restore-db.sh /opt/apps/kicau-mania/data/backups/backup_NAMA.dump --replace
```

Restore membuat backup dahulu, memulihkan snapshot ke database staging dalam transaksi, baru menghentikan app dan mengganti nama database secara atomik. App tetap dihentikan sesudah restore; operator memeriksa kompatibilitas schema/image sebelum deployment. Restore database tidak memulihkan role cluster/password: buat role grup yang diperlukan sebelum restore pada cluster baru, lalu siapkan login service secara terpisah.

Profil rekan kerja: `reader` (default), `admin-reader`, `maintainer`; `app` khusus service. Contoh: `/opt/scripts/db/grant-access.sh nama_rekan reader`. Script menolak mengganti login yang sudah ada. Password dapat dipasok lewat environment KICAU_DB_PASSWORD untuk otomasi tepercaya; bila kosong, script meminta input tersembunyi. Jangan menaruh password pada argumen CLI. Reader tidak otomatis mendapat kontak privat; admin-reader dapat membaca seluruh read layer dan hanya diberikan pada pihak berwenang.

Akses database memakai SSH tunnel ke 127.0.0.1:5432 dari sisi VPS. Jangan membagikan akun postgres. Perubahan Nginx, sertifikat, database dan script sistem tidak diaktifkan otomatis oleh deploy aplikasi; review bundle, salin file terkait ke /opt, nginx -t lalu reload secara manual.

## Verifikasi dan batas penerimaan

Jalankan dari root repository pada Linux dengan Docker:

```bash
bash cloud-app/deployment/check.sh
docker buildx build --platform linux/amd64 --load -t kicau-cloud-check:local \
  -f cloud-app/deployment/opt/apps/kicau-mania/app/Dockerfile cloud-app
CHECK_IMAGE=kicau-cloud-check:local bash cloud-app/deployment/acceptance.sh
```

Acceptance memakai database/container/jaringan sementara dengan nama unik; tidak menyentuh /opt atau database utama. Menguji migrasi, role terbatas, readiness melalui proxy, deployment berurutan, kegagalan image/rollback data setelah restart PostgreSQL, bootstrap admin, password berkarakter khusus, serta backup/restore. Hanya pengambilan image GHCR/digest diganti image lokal; jalur registry privat/SSH tetap memerlukan penerimaan nyata. Mode acceptance menggunakan HTTP proxy internal, sementara check.sh menguji sintaks HTTPS Nginx dengan sertifikat sintetis.

Sebelum operasional, buktikan deployment dari main pada VPS, HTTPS/DNS/CORS/cookie browser, login, SMTP dan Google sesungguhnya, serta pemulihan backup. Build/simulasi lokal tidak menjadi bukti penerimaan VPS. Tidak ada akses VPS, push GitHub, penerbitan sertifikat atau perubahan DNS otomatis oleh implementasi repository ini.

### Hasil pemeriksaan lokal — 10 November 2026

- go build dan go vet lolos; suite tes Go tidak dijalankan.
- Actionlint 1.7.12, ShellCheck 0.11, validasi Compose dan nginx -t lolos.
- Image linux/amd64 dan linux/arm64 berhasil dibangun; runtime memakai UID 10001.
- Acceptance container lolos: migrasi 001–007, role terbatas, password dengan kutip, bootstrap superadmin, readiness melalui proxy, deployment berurutan, image/config gagal dan rollback, persistensi PostgreSQL serta backup/restore staging.
- Audit build context tidak menemukan .env aktual, app.env, database.env, private key atau sertifikat VPS.
- Belum dilakukan push/publikasi GHCR, SSH ke VPS, perubahan DNS/TLS nyata, atau deployment GitHub Actions sesungguhnya.

### SSH gagal: error in libcrypto

Pesan `Load key ... error in libcrypto` terjadi ketika runner tidak bisa membaca private key; ini belum membuktikan bahwa VPS menolak public key. Periksa secret VPS_SSH_KEY pada Environment development: isinya harus seluruh file private key (tanpa .pub), termasuk BEGIN/END dan newline asli, tanpa tanda kutip/backtick pembungkus. Jangan memakai path file, fingerprint, atau teks literal \n.

Workflow menormalkan CRLF Windows dan memeriksa key dengan ssh-keygen sebelum koneksi. Key berpassphrase ditolak karena belum ada dukungan SSH agent/passphrase. Buat key deployment tersendiri tanpa passphrase, pertahankan key pribadi lama, dan tambahkan public key pasangan deployment ke authorized_keys user VPS yang sesuai.

Contoh di PowerShell Windows, dengan passphrase kosong saat diminta:

```powershell
ssh-keygen -t ed25519 -C "github-actions-kicau" -f "$env:USERPROFILE\.ssh\kicau_deploy"
Get-Content -Raw "$env:USERPROFILE\.ssh\kicau_deploy" | Set-Clipboard
```

Tempel clipboard ke VPS_SSH_KEY. File kicau_deploy.pub dipasang di VPS, bukan pada secret tersebut. Jangan menimpa file key jika nama itu sudah dipakai. Setelah perubahan workflow dipush, jalankan workflow commit terbaru; rerun run lama tetap menggunakan workflow lama. Mengganti secret saja dapat diuji dengan rerun failed job.
