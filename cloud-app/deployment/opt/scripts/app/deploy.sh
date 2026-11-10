#!/usr/bin/env bash
set -Eeuo pipefail
umask 077
APP_DIR="${APP_DIR:-/opt/apps/kicau-mania}"
APP_CONTAINER="${APP_CONTAINER:-kicau_mania_app}"
PUBLIC_API_URL="${PUBLIC_API_URL:-https://dev-api.megaallstararena.com}"
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
switched=1
compose "$release" up -d --no-deps --pull never app
ready
ln -s "$release" "$APP_DIR/state/.current.$$"
mv -Tf "$APP_DIR/state/.current.$$" "$APP_DIR/state/current"
record ready
trap - ERR INT TERM
echo "Deployment ready: $COMMIT"
