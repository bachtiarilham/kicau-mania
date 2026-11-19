#!/usr/bin/env bash
set -Eeuo pipefail
umask 077
APP_DIR="${APP_DIR:-/opt/apps/kicau-mania}"
# Registry credentials remain within /opt, never /root/.docker.
DOCKER_CONFIG="${DOCKER_CONFIG:-$APP_DIR/config/docker}"
export DOCKER_CONFIG
APP_CONTAINER="${APP_CONTAINER:-kicau_mania_app}"
DB_CONTAINER="${DB_CONTAINER:-kicau_mania_db}"
PUBLIC_API_URL="${PUBLIC_API_URL:-https://dev.megaallstararena.com}"
READY_TIMEOUT=${DEPLOY_READY_TIMEOUT:-120}
[[ $READY_TIMEOUT =~ ^[0-9]+$ ]] && (( READY_TIMEOUT >= 1 && READY_TIMEOUT <= 120 )) || exit 2
BUNDLE=${1:?Usage: deploy.sh BUNDLE IMAGE_DIGEST COMMIT_SHA}
IMAGE=${2:?Image digest required}
COMMIT=${3:?Commit required}
[[ $APP_DIR == /* && $APP_DIR != *'/../'* ]] || exit 2
[[ $COMMIT =~ ^[0-9a-f]{40}$ ]] || { echo 'Invalid commit'; exit 2; }
[[ $IMAGE =~ ^ghcr\.io/bachtiarilham/kicau-mania-cloud@sha256:[0-9a-f]{64}$ ]] || { echo 'Immutable image digest required'; exit 2; }
[[ $(cat "$BUNDLE/COMMIT") == "$COMMIT" ]] || { echo 'Bundle commit mismatch'; exit 2; }
[[ -f $APP_DIR/.env && -f $APP_DIR/app.env && -f $APP_DIR/database.env ]] || { echo 'Complete VPS environment setup first'; exit 2; }
mkdir -p "$APP_DIR/state" "$APP_DIR/releases"
exec 9>"$APP_DIR/state/deploy.lock"
flock -w 180 9 || { echo 'Another deployment or database operation is running'; exit 1; }
previous=''
if [[ -L $APP_DIR/state/current ]]; then previous=$(readlink -f "$APP_DIR/state/current"); fi
release="$APP_DIR/releases/${COMMIT}-$(date -u +%Y%m%dT%H%M%S)-$$"
mkdir -m 700 "$release"
cp "$BUNDLE/stack/docker-compose.yml" "$release/docker-compose.yml"
cp "$APP_DIR/app.env" "$release/app.env"
# Releases predating schema 010 do not yet define Redis. Add the bundled-service
# default only to this release snapshot; preserve any explicitly configured URL.
if ! grep -Eq '^[[:space:]]*REDIS_URL[[:space:]]*=' "$release/app.env"; then
  printf '\nREDIS_URL=redis://redis:6379/0\n' >> "$release/app.env"
fi
printf 'APP_IMAGE=%s\nAPP_ENV_FILE=%s\n' "$IMAGE" "$release/app.env" > "$release/image.env"
printf '%s\n' "$COMMIT" > "$release/COMMIT"
compose() {
  local dir=$1; shift
  docker compose --project-name "${STACK_PROJECT:-kicau-mania}" --project-directory "$APP_DIR" \
    --env-file "$APP_DIR/.env" --env-file "$dir/image.env" -f "$dir/docker-compose.yml" "$@"
}
record() { printf '%s %s %s %s\n' "$(date -u +%FT%TZ)" "$COMMIT" "$IMAGE" "$1" >> "$APP_DIR/state/deployments.log"; }
ready() {
  local deadline=$((SECONDS+READY_TIMEOUT))
  while (( SECONDS < deadline )); do
    if docker exec "$APP_CONTAINER" wget -q -T 3 -O /dev/null http://127.0.0.1:8091/readyz 2>/dev/null \
      && curl --fail --silent --show-error --connect-timeout 3 --max-time 5 "$PUBLIC_API_URL/readyz" -o /dev/null 2>/dev/null; then return 0; fi
    sleep 3
  done
  return 1
}
switched=0
failure() {
  local result=$?
  trap - ERR INT TERM
  set +e
  record failed
  echo 'Deployment diagnostics: service status and application logs (no environment dump)' >&2
  compose "$release" ps --all >&2
  docker logs --tail 80 "$APP_CONTAINER" >&2
  if (( switched )); then
    if [[ -n $previous && -f $previous/image.env ]]; then
      echo 'Deployment failed; restoring previous image and application configuration'
      if compose "$previous" up -d --no-deps --pull never app && ready; then record rollback-ready; else record rollback-failed; echo 'Rollback failed; operator intervention required'; fi
    else
      compose "$release" stop app
      echo 'First deployment failed; no previous release is available'
    fi
  fi
  (( result != 0 )) || result=1
  exit "$result"
}
trap failure ERR INT TERM
compose "$release" config --quiet
compose "$release" pull app
record starting
# --no-deps on the app deliberately leaves PostgreSQL untouched. Start/wait for
# its newly required Redis service explicitly, including upgrades from old stacks.
compose "$release" up -d --no-deps --wait --wait-timeout "$READY_TIMEOUT" redis
shopt -s nullglob
migrations=("$BUNDLE"/database/migrations/*.up.sql)
if (( ${#migrations[@]} > 0 )); then
  mkdir -p "$APP_DIR/state"
  touch "$APP_DIR/state/migrations.log"
  for m in "${migrations[@]}"; do
    m_hash=$(sha256sum "$m" | awk '{print $1}')
    if ! grep -q "$m_hash" "$APP_DIR/state/migrations.log"; then
      echo "Applying pending database migration: $(basename "$m")"
      docker exec -i "$DB_CONTAINER" sh -c 'exec psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB"' < "$m"
      printf '%s %s\n' "$(date -u +%FT%TZ)" "$(sha256sum "$m")" >> "$APP_DIR/state/migrations.log"
    fi
  done
fi
switched=1
compose "$release" up -d --no-deps --pull never app
ready
ln -s "$release" "$APP_DIR/state/.current.$$"
mv -Tf "$APP_DIR/state/.current.$$" "$APP_DIR/state/current"
record ready
trap - ERR INT TERM
echo "Deployment ready: $COMMIT"
