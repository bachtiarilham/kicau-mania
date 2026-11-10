#!/usr/bin/env bash
set -Eeuo pipefail
# shellcheck source=common.sh
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
file=${1:?Usage: migrate.sh /absolute/path/008_description.up.sql}
[[ -f $file && $(basename "$file") =~ ^[0-9]{3}_[a-z0-9_]+\.up\.sql$ ]] || { echo 'Expected one explicit .up.sql migration'; exit 2; }
lock_database
require_app_stopped
pg_psql -1 < "$file"
printf '%s %s\n' "$(date -u +%FT%TZ)" "$(sha256sum "$file")" >> "$APP_DIR/state/migrations.log"
echo 'Migration committed; restart the compatible application release explicitly'
