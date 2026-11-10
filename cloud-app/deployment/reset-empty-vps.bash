#!/usr/bin/env bash
# Development-only reset. Never called by the CI/CD deploy script.
# Usage: bash reset-empty-vps.bash --reset-empty-vps
set -Eeuo pipefail
umask 077
stage='preflight'
trap 'printf "GAGAL pada tahap %s, baris %s. Jangan rerun deployment sebelum diperbaiki.\n" "$stage" "$LINENO" >&2' ERR
[[ ${1:-} == --reset-empty-vps ]] || { echo 'Usage: bash reset-empty-vps.bash --reset-empty-vps'; exit 2; }
[[ $EUID == 0 ]] || { echo 'Jalankan sebagai root di VPS'; exit 2; }
APP_DIR="${APP_DIR:-/opt/apps/kicau-mania}"
APP_CONTAINER="${APP_CONTAINER:-kicau_mania_app}"
DB_CONTAINER="${DB_CONTAINER:-kicau_mania_db}"
project="${STACK_PROJECT:-kicau-mania}"
gateway="${GATEWAY_NETWORK:-web_gateway}"
for cmd in docker openssl readlink flock sed awk sort find; do command -v "$cmd" >/dev/null; done
docker info >/dev/null
docker compose version
[[ $APP_DIR == /*/kicau-mania && $(readlink -m "$APP_DIR") == "$APP_DIR" ]] || { echo 'APP_DIR harus absolut, tanpa symlink, dan berakhir /kicau-mania'; exit 2; }
[[ $APP_CONTAINER =~ ^[a-zA-Z0-9][a-zA-Z0-9_.-]+$ && $DB_CONTAINER =~ ^[a-zA-Z0-9][a-zA-Z0-9_.-]+$ && $project =~ ^[a-z0-9][a-z0-9_-]+$ ]] || exit 2
shopt -s nullglob
stacks=("$APP_DIR"/incoming/*/stack)
(( ${#stacks[@]} > 0 )) || { echo 'Bundle CI belum ditemukan di incoming/*/stack'; exit 1; }
STACK=$(find "$APP_DIR/incoming" -mindepth 2 -maxdepth 2 -type d -name stack -printf '%T@ %p\n' | sort -nr | sed -n '1s/^[^ ]* //p')
BUNDLE=${STACK%/stack}
for file in "$STACK/docker-compose.yml" "$STACK/.env.example" "$STACK/app.env.example" "$BUNDLE/database/baseline.dump" "$BUNDLE/database/migrations/007_cloud_read_layer.up.sql"; do
  [[ -f $file ]] || { echo "Bundle tidak lengkap: $file"; exit 1; }
done
migrations=("$BUNDLE"/database/migrations/*.up.sql)
(( ${#migrations[@]} >= 7 )) || { echo 'Migrasi 001..007 belum lengkap'; exit 1; }
DATA="$APP_DIR/data/postgres18"
[[ $(readlink -m "$DATA") == "$DATA" ]] || { echo 'Direktori data menunjuk symlink; reset dibatalkan'; exit 1; }
network="${project}_internal_net"
if docker network inspect "$network" >/dev/null 2>&1; then
  members=$(docker network inspect "$network" --format '{{range .Containers}}{{.Name}} {{end}}')
  for member in $members; do
    [[ $member == "$APP_CONTAINER" || $member == "$DB_CONTAINER" ]] || { echo 'Network database dipakai container lain; reset dibatalkan'; exit 1; }
  done
fi
mkdir -p "$APP_DIR/state"
exec 9>"$APP_DIR/state/deploy.lock"
flock -w 180 9
stage='menyiapkan konfigurasi'
echo '[1/5] Bundle lengkap. Menyiapkan konfigurasi tanpa mencetak password.'
admin_password=$(openssl rand -hex 24)
service_password=$(openssl rand -hex 24)
[[ -f $APP_DIR/.env ]] || cp "$STACK/.env.example" "$APP_DIR/.env"
[[ -f $APP_DIR/app.env ]] || cp "$STACK/app.env.example" "$APP_DIR/app.env"
# Only generated hex values and fixed non-secret names are passed to this helper.
set_value() {
  local file=$1 key=$2 value=$3 temp
  temp=$(mktemp "$APP_DIR/.reset-env.XXXXXX")
  KICAU_ENV_VALUE="$value" awk -v key="$key" '
    BEGIN { found=0; value=ENVIRON["KICAU_ENV_VALUE"] }
    $0 ~ "^" key "=" { if (!found) print key "=" value; found=1; next }
    { print }
    END { if (!found) print key "=" value }
  ' "$file" > "$temp"
  chmod 600 "$temp"
  mv -f -- "$temp" "$file"
}
printf 'POSTGRES_DB=kicau_mania\nPOSTGRES_USER=postgres\nPOSTGRES_PASSWORD=%s\n' "$admin_password" > "$APP_DIR/database.env"
set_value "$APP_DIR/app.env" DATABASE_URL "postgres://cloud_service:$service_password@postgres:5432/kicau_mania?sslmode=disable"
# Fixed container and network names must match the reset/deploy scope.
set_value "$APP_DIR/.env" APP_CONTAINER "$APP_CONTAINER"
set_value "$APP_DIR/.env" DB_CONTAINER "$DB_CONTAINER"
set_value "$APP_DIR/.env" GATEWAY_NETWORK "$gateway"
set_value "$APP_DIR/.env" APP_IMAGE ghcr.io/bachtiarilham/kicau-mania-cloud:bootstrap
chmod 600 "$APP_DIR/.env" "$APP_DIR/app.env" "$APP_DIR/database.env"
cp "$STACK/docker-compose.yml" "$APP_DIR/docker-compose.yml"
compose() { docker compose --project-name "$project" --project-directory "$APP_DIR" --env-file "$APP_DIR/.env" -f "$APP_DIR/docker-compose.yml" "$@"; }
compose config --quiet
docker pull postgres:18-alpine >/dev/null
stage='reset cluster aplikasi'
echo '[2/5] Menghapus container aplikasi/database dan data PostgreSQL 18 aplikasi.'
for container in "$APP_CONTAINER" "$DB_CONTAINER"; do
  if docker container inspect "$container" >/dev/null 2>&1; then docker rm -f "$container"; fi
done
# Validate the exact absolute target again immediately before recursive deletion.
[[ $DATA == "$APP_DIR/data/postgres18" && $(readlink -m "$DATA") == "$DATA" ]] || exit 2
rm -rf -- "$DATA"
mkdir -p "$DATA" "$APP_DIR/data/backups"
# The image owns the private PGDATA child; its mount root must be traversable.
chmod 755 "$DATA"
if [[ -L $APP_DIR/state/current ]]; then rm -- "$APP_DIR/state/current"; fi
if docker network inspect "$network" >/dev/null 2>&1; then docker network rm "$network"; fi
if ! docker network inspect "$gateway" >/dev/null 2>&1; then docker network create --subnet 172.30.20.0/24 "$gateway"; fi
compose up -d --wait postgres
stage='restore baseline dan migrasi'
echo '[3/5] Restore baseline dan terapkan seluruh migrasi.'
docker exec -i "$DB_CONTAINER" pg_restore -U postgres -d kicau_mania --exit-on-error --single-transaction --no-owner --no-privileges < "$BUNDLE/database/baseline.dump"
for migration in "${migrations[@]}"; do
  echo "Migrasi: $(basename "$migration")"
  docker exec -i "$DB_CONTAINER" psql -X -U postgres -d kicau_mania -v ON_ERROR_STOP=1 -1 -f - < "$migration"
  printf '%s reset-cluster %s\n' "$(date -u +%FT%TZ)" "$(sha256sum "$migration")" >> "$APP_DIR/state/migrations.log"
done
stage='akun service'
echo '[4/5] Membuat cloud_service non-superuser dengan role kicau_cloud_app.'
export KICAU_DB_PASSWORD=$service_password
docker exec -i -e KICAU_DB_PASSWORD "$DB_CONTAINER" psql -X -U postgres -d kicau_mania -v ON_ERROR_STOP=1 -1 -f - <<'SQL'
\getenv service_password KICAU_DB_PASSWORD
SET password_encryption='scram-sha-256';
SELECT format('CREATE ROLE cloud_service LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION PASSWORD %L', :'service_password') \gexec
GRANT CONNECT ON DATABASE kicau_mania TO cloud_service;
GRANT kicau_cloud_app TO cloud_service;
SQL
unset KICAU_DB_PASSWORD admin_password service_password
stage='verifikasi'
echo '[5/5] Memeriksa skema, role dan jumlah akun.'
docker exec "$DB_CONTAINER" psql -X -U postgres -d kicau_mania -v ON_ERROR_STOP=1 -c "SELECT current_database(),(SELECT count(*) FROM iam.users) AS users,(SELECT count(*) FROM iam.roles) AS roles,to_regclass('cloud_read.mv_arena_dashboard') AS dashboard;"
# Verify actual TCP authentication using the generated application credential.
# The name-only -e argument avoids putting a password into process arguments.
export PGPASSWORD
auth_url=$(sed -n 's/^DATABASE_URL=//p' "$APP_DIR/app.env")
PGPASSWORD=${auth_url#postgres://cloud_service:}; PGPASSWORD=${PGPASSWORD%%@*}
docker exec -e PGPASSWORD "$DB_CONTAINER" psql -X -h 127.0.0.1 -U cloud_service -d kicau_mania -v ON_ERROR_STOP=1 -c "SELECT current_user,rolsuper AS is_superuser FROM pg_roles WHERE rolname=current_user;"
unset PGPASSWORD auth_url
printf '%s database-reset-ready\n' "$(date -u +%FT%TZ)" >> "$APP_DIR/state/deployments.log"
echo 'SELESAI: PostgreSQL 18, migrasi dan akun cloud_service siap.'
echo 'Password sudah tersimpan di database.env dan app.env (mode 600), tidak dicetak.'
echo 'Aplikasi belum dijalankan. Lanjutkan setup GHCR serta Nginx/HTTPS sebelum rerun deploy.'
