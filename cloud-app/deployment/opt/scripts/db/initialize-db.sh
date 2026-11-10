#!/usr/bin/env bash
set -Eeuo pipefail
# shellcheck source=common.sh
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
bundle=${1:?Usage: initialize-db.sh /path/to/release-bundle}
[[ -f $bundle/database/baseline.dump ]] || { echo 'Baseline missing'; exit 2; }
lock_database
require_app_stopped
count=$(pg_psql -Atc "SELECT count(*) FROM information_schema.tables WHERE table_schema NOT IN ('pg_catalog','information_schema') AND table_type='BASE TABLE'")
[[ $count == 0 ]] || { echo 'Database is not empty. Initialization never erases existing data.'; exit 1; }
docker exec -i "$DB_CONTAINER" sh -c 'exec pg_restore -U "$POSTGRES_USER" -d "$POSTGRES_DB" --exit-on-error --single-transaction --no-owner --no-privileges' < "$bundle/database/baseline.dump"
shopt -s nullglob
files=("$bundle"/database/migrations/*.up.sql)
(( ${#files[@]} > 0 )) || { echo 'Migrations missing'; exit 1; }
for file in "${files[@]}"; do
  pg_psql -1 -f - < "$file"
  printf '%s %s\n' "$(date -u +%FT%TZ)" "$(sha256sum "$file")" >> "$APP_DIR/state/migrations.log"
done
echo 'Database initialized. Create the application login and superadmin separately.'
