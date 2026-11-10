#!/usr/bin/env bash
# Shared database operations. Never source environment files as shell code.
set -Eeuo pipefail
umask 077
APP_DIR="${APP_DIR:-/opt/apps/kicau-mania}"
APP_CONTAINER="${APP_CONTAINER:-kicau_mania_app}"
DB_CONTAINER="${DB_CONTAINER:-kicau_mania_db}"
[[ $APP_DIR == /* && $APP_DIR != *'/../'* ]] || { echo 'APP_DIR must be absolute'; exit 2; }
pg_psql() {
  docker exec -i "$DB_CONTAINER" sh -c 'exec psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" "$@"' sh "$@"
}
lock_database() {
  mkdir -p "$APP_DIR/state"
  exec 9>"$APP_DIR/state/deploy.lock"
  flock -w 180 9 || { echo 'Another deployment/database operation is running'; exit 1; }
}
stop_app() { if docker inspect "$APP_CONTAINER" >/dev/null 2>&1; then docker stop -t 20 "$APP_CONTAINER" >/dev/null; fi; }
require_app_stopped() {
  if [[ $(docker inspect -f '{{.State.Running}}' "$APP_CONTAINER" 2>/dev/null || true) == true ]]; then
    echo 'Stop the app before applying migrations'; exit 1
  fi
}
backup_database() {
  local dir="$APP_DIR/data/backups" name
  mkdir -p "$dir"
  [[ -w $dir ]] || { echo "Backup directory is not writable by the operator: $dir" >&2; return 1; }
  name="$dir/backup_kicau_mania_$(date -u +%Y%m%d_%H%M%S)_$$.dump"
  local partial="$name.partial"
  if docker exec "$DB_CONTAINER" sh -c 'exec pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > "$partial"; then
    mv "$partial" "$name"
    # Retention runs only after a complete successful snapshot.
    find "$dir" -maxdepth 1 -type f -name 'backup_kicau_mania_*.dump' -mtime +7 -delete
    printf 'Backup ready: %s\n' "$name"
  else
    rm -f -- "$partial"
    echo 'Backup failed; previous snapshots retained' >&2
    return 1
  fi
}
