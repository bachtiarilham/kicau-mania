# Deployment cloud-app: Nginx Docker dan GitHub Actions

Dokumen operasional untuk stack saat ini. Untuk belajar dari awal, baca [Belajar Deployment](BELAJAR-DEPLOYMENT.md). Template berada di [opt](opt/). Local-app tidak dideploy oleh pipeline cloud-app.

## Alamat layanan dan alur

| Layanan | Alamat |
| --- | --- |
| Situs utama | https://megaallstararena.com dan https://www.megaallstararena.com |
| Frontend dev | https://dev.megaallstararena.com/ |
| REST API | https://dev.megaallstararena.com/api/v1/ |
| Liveness | https://dev.megaallstararena.com/healthz |
| Readiness database/skema | https://dev.megaallstararena.com/readyz |

Frontend dan API memakai domain dev yang sama; tidak membutuhkan DNS subdomain API baru. Nginx menyajikan file statis dan meneruskan path /api/, /healthz, /readyz ke cloud-app di jaringan Docker. API tetap membutuhkan Bearer/auth sesuai kontrak. Frontend yang disalin dari Apache masih berupa frontend demo; migrasi gateway tidak menghubungkan login demo ke backend secara otomatis.

```text
Browser → Nginx Docker :443 → /api/ → cloud-app:8091 → PostgreSQL 18
                         └→ halaman statis /opt/infra/www/{main,dev}
GitHub main → verify → image GHCR → SSH → pull digest → readiness HTTPS → current
```

## Struktur yang dipakai

```text
/opt/infra/
  docker-compose.yml
  .env                            # IP Nginx dan digest image Certbot
  nginx/nginx.conf
  nginx/conf.d/{default,frontends}.conf
  nginx/40-reload-certificates.sh
  www/{main,dev}/                  # Frontend aktif; perubahan konten dilakukan di sini
  certs/{accounts,archive,live,renewal}/
  acme/                           # Public HTTP-01 challenge
  acme-work/                      # Marker reload dan workdir Certbot
  acme-logs/
  migration/                      # Bukti uji perpindahan dan diagnostik
/opt/apps/kicau-mania/
  docker-compose.yml
  .env                            # Interpolasi Compose; bukan env aplikasi
  app.env                         # DATABASE_URL dan konfigurasi aplikasi
  database.env                    # Kredensial admin PostgreSQL saja
  config/docker/config.json       # Login GHCR; hanya disimpan di VPS
  data/{postgres18,backups}/
  incoming/<commit>/
  releases/<commit>-<time>-<pid>/   # Snapshot Compose, digest dan app.env
  state/{current,deploy.lock,deployments.log,migrations.log}
/opt/scripts/{app,db,infra,init}/
```

File konfigurasi dan credential VPS disimpan di /opt. Perubahan di luar /opt memerlukan izin operator. Pada migrasi 10 November 2026, Apache dan Nginx bawaan OS serta certbot.timer lama dinonaktifkan setelah Nginx Docker dan renewal teruji; source /var/www serta sertifikat /etc/letsencrypt dipertahankan sebagai sumber lama yang tidak aktif. Tidak ada penghapusan paket OS.

Apache tidak diperlukan untuk melayani domain sekarang. Jangan menyalakannya bersamaan dengan Nginx Docker karena port 80/443 sudah dipakai. PostgreSQL 17 lama beserta direktori /opt/apps/kicau-mania/data/postgres dan Dockerfile Go 1.22 yang tidak digunakan telah dibersihkan; data PostgreSQL 18 dipertahankan.

## Setup dan pemeliharaan gateway

Stack gateway memiliki service nginx, certbot-renew dan certbot-tool (profile tools). Compose memount sertifikat dan frontend dari /opt. Tidak ada Docker socket pada Certbot. Certbot memeriksa renewal setiap 12 jam; sesudah renewal menulis marker, Nginx memeriksanya tiap 60 detik dan reload setelah nginx -t berhasil.

Sertifikat utama mencakup megaallstararena.com dan www; sertifikat dev mencakup dev.megaallstararena.com. Konfigurasi renewal menggunakan webroot, bukan plugin Apache. Saat mengimpor sertifikat lama, salin accounts/archive/live/renewal agar symlink dan akun ACME tetap konsisten; ubah renewal copy di /opt ke webroot. Jangan mengubah sertifikat sumber /etc tanpa izin.

Pada VPS saat ini web_gateway memakai 172.18.0.0/16, IP Nginx 172.18.0.2, dan app.env TRUSTED_PROXIES=172.18.0.2/32. Template baru mempunyai fallback subnet/IP berbeda; selalu cocokkan IP yang disimpan pada /opt/infra/.env dengan jaringan yang benar. Aplikasi dan PostgreSQL terhubung melalui internal_net; aplikasi mempunyai akses keluar melalui web_gateway.

```bash
export DOCKER_CONFIG=/opt/apps/kicau-mania/config/docker
cd /opt/infra
docker compose --project-name gateway ps
docker exec global_reverse_proxy nginx -t
# Setelah konfigurasi yang benar disimpan:
docker exec global_reverse_proxy nginx -s reload
```

Jika mengubah key pada shared rate-limit zone, gunakan nama zone baru atau lakukan restart terencana; nginx -t saja tidak membuktikan reload master berhasil. Periksa log dan response endpoint sesudah reload.

Uji renewal tanpa mengganti sertifikat aktif:

```bash
cd /opt/infra
docker compose --project-name gateway run --rm --no-deps certbot-tool renew \
  --dry-run --non-interactive --webroot -w /var/www/acme \
  --config-dir /etc/letsencrypt --work-dir /var/lib/letsencrypt \
  --logs-dir /var/log/letsencrypt --no-random-sleep-on-renew
```

Untuk host baru, infrastruktur DNS, sertifikat dan konten frontend harus diprovisikan sebelum menjalankan stack penuh. Script [setup-gateway.bash](setup-gateway.bash) adalah pemeriksaan/setup HTTPS API sesudah stack gateway terpasang, bukan installer Apache atau reset database. Script tidak memasang paket OS, mengedit crontab atau mengubah systemd.

## Login GHCR dan GitHub Environment

Image privat: ghcr.io/bachtiarilham/kicau-mania-cloud, tag commit SHA; deployment memakai digest. VPS menggunakan PAT classic dengan read:packages dari akun yang mempunyai akses package. Token dimasukkan langsung di terminal VPS, tidak melalui chat atau argumen command:

```bash
docker --config /opt/apps/kicau-mania/config/docker login ghcr.io -u bachtiarilham
```

Saat diminta Password, masukkan PAT. User SSH deployment saat ini root; bila diganti, berikan izin baca credential dan izin Docker yang sesuai. Deploy script mengekspor DOCKER_CONFIG ke folder /opt tersebut; /root/.docker tidak diperlukan. Jangan commit app.env, database.env, config Docker, private key, sertifikat atau backup.

GitHub Environment development memuat secrets VPS_HOST, VPS_USER, VPS_PORT (default 22), VPS_SSH_KEY dan VPS_KNOWN_HOSTS. Private key harus lengkap, tanpa passphrase untuk workflow sekarang; public key pasang pada authorized_keys user VPS. Known hosts harus cocok dengan fingerprint yang diverifikasi melalui konsol provider. Workflow menormalkan CRLF dan menolak key rusak sebelum SSH; tidak mencetak key.

Perubahan sumber CI/deploy di workspace harus di-commit dan di-push agar workflow GitHub memakai versi baru. Rerun commit lama menggunakan workflow/script dalam commit lama.

## Deploy dan rollback aplikasi

PR menjalankan build/vet, validasi workflow/shell/Compose/Nginx, acceptance container, serta build image kedua arsitektur. Main memublikasikan image AMD64/ARM64 dan melakukan deploy melalui SSH. Tidak ada suite tes Go otomatis. Perubahan local-app saja tidak memicu deploy cloud.

Deployment tidak menjalankan git pull/build di VPS, migrasi, reset, restart PostgreSQL, atau perubahan gateway. File Compose dan app.env disnapshot dalam rilis; hanya service app diganti. Lock host mencegah deployment bersamaan operasi database. Readiness internal dan https://dev.megaallstararena.com/readyz diperiksa hingga 120 detik. Gagal berarti rollback image/config sebelumnya; job tetap gagal. current hanya berubah sesudah siap. Tidak ada pruning image otomatis.

Validasi operasional:

```bash
curl --fail https://dev.megaallstararena.com/healthz
curl --fail https://dev.megaallstararena.com/readyz
docker logs --tail 50 kicau_mania_app
docker logs --tail 50 global_reverse_proxy
```

API mengembalikan envelope JSON; tanpa token, resource terlindungi menghasilkan 401. HTTPS valid dan /readyz sukses membuktikan transport serta skema tersedia, bukan login frontend atau integrasi Google/SMTP. Tidak ada akun produksi bawaan. Pada 10 November 2026, delapan akun mock telah dimasukkan manual ke VPS dev untuk pengujian. Buat superadmin nyata secara eksplisit dengan `bash /opt/scripts/app/bootstrap-admin.sh`; password dibaca tersembunyi. Koleksi Insomnia mempunyai environment VPS dev dengan base_url https://dev.megaallstararena.com. Akun fixture baru tersedia bila seed diterapkan secara eksplisit, bukan melalui deploy.

## Database dan reset pengembangan

PostgreSQL 18 memakai data/postgres18 yang dimount ke /var/lib/postgresql; port host hanya 127.0.0.1:5432. Akun aplikasi cloud_service bukan superuser. Semua password hanya berada di database.env/app.env mode 600. DATABASE_URL menggunakan host postgres:5432 dari jaringan aplikasi.

Migrasi tetap manual. Hentikan app saat perubahan skema yang membutuhkannya, lalu jalankan `bash /opt/scripts/db/migrate.sh /path/008_description.up.sql`. Script menjalankan psql -1 -f - dan ON_ERROR_STOP, serta mencatat checksum sesudah commit. Operator memastikan versi belum diterapkan. Restore/backward migration tidak dijalankan otomatis ketika rollback image.

Backup: `bash /opt/scripts/db/backup-db.sh`; snapshot .partial di-rename sesudah sukses dan retensi >7 hari berjalan setelah keberhasilan. Restore: `bash /opt/scripts/db/restore-db.sh /host/path/snapshot.dump --replace`; memakai staging dan membiarkan app berhenti untuk pemeriksaan kompatibilitas. Profil akses: reader, admin-reader, maintainer, app; grant menolak role group yang belum dibuat migrasi sebelum membuat login.

[reset-empty-vps.bash](reset-empty-vps.bash) hanya untuk database pengembangan yang boleh dibuang. Tidak dipanggil pipeline dan jangan digunakan lagi pada VPS yang sudah siap hanya untuk deployment aplikasi. Script membuat env yang hilang, password acak, cluster PG18 kosong dan migrasi 001–007. Saat gagal, berhenti pada tahap yang disebutkan; jangan menambahkan role kosong untuk melewati migrasi.

## Bukti penerimaan 10 November 2026

- Nginx staging diperiksa pada port loopback, lalu port 80/443 dipindahkan dengan fallback Apache jika gagal.
- HTTPS domain utama, www dan dev valid; hash index HTML cocok dengan sumber sebelum perpindahan.
- Public HTTP-01 challenge dan dry-run renewal kedua sertifikat berhasil melalui Docker; marker reload memicu pergantian worker Nginx.
- GHCR privat berhasil ditarik dengan credential di /opt; cloud-app sehat dan deployment rilis tercatat siap.
- /healthz dan /readyz menghasilkan JSON sukses melalui HTTPS dev; API tanpa token ditolak 401.
- Apache, Nginx OS dan timer Certbot lama dinonaktifkan; data PG18 tidak direset saat migrasi gateway.
- Perubahan gateway bukan integrasi frontend demo dengan backend, bukan pengujian SMTP/Google, dan bukan penerimaan streaming CCTV/LAN.
