#!/usr/bin/env bash
set -Eeuo pipefail
# shellcheck source=common.sh
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
file=${1:?Usage: restore-db.sh /host/path/snapshot.dump --replace}
[[ ${2:-} == --replace && -f $file ]] || { echo 'A host snapshot path and --replace are required'; exit 2; }
lock_database
backup_database
db=$(docker exec "$DB_CONTAINER" printenv POSTGRES_DB)
[[ $db =~ ^[a-z][a-z0-9_]{0,62}$ && $db != postgres ]] || { echo 'Unsupported database name'; exit 2; }
stage="restore_$(date -u +%Y%m%d%H%M%S)_$$"
old="previous_$(date -u +%Y%m%d%H%M%S)_$$"
cleanup() { docker exec "$DB_CONTAINER" sh -c 'exec dropdb -U "$POSTGRES_USER" --if-exists "$1"' sh "$stage" >/dev/null 2>&1 || true; }
trap cleanup EXIT
docker exec "$DB_CONTAINER" sh -c 'exec createdb -U "$POSTGRES_USER" -T template0 "$1"' sh "$stage"
docker exec -i "$DB_CONTAINER" sh -c 'exec pg_restore -U "$POSTGRES_USER" -d "$1" --exit-on-error --single-transaction --no-owner' sh "$stage" < "$file"
stop_app
pg_psql -d postgres -v target="$db" -v stage="$stage" -v old="$old" <<'SQL'
SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname=:'target' AND pid<>pg_backend_pid();
BEGIN;
SELECT format('ALTER DATABASE %I RENAME TO %I', :'target', :'old') \gexec
SELECT format('ALTER DATABASE %I RENAME TO %I', :'stage', :'target') \gexec
COMMIT;
SQL
docker exec "$DB_CONTAINER" sh -c 'exec dropdb -U "$POSTGRES_USER" --force "$1"' sh "$old"
echo 'Restore committed. App remains stopped; verify schema and deploy a compatible release.'
