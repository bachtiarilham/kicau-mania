#!/usr/bin/env bash
set -Eeuo pipefail
# Real local containers and SQL; only GHCR pull/digest lookup is replaced by local images.
# Never runs against /opt or existing VPS containers. No Go test suite is invoked.
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
CHECK_IMAGE=${CHECK_IMAGE:?Build a native image and set CHECK_IMAGE first}
REAL_DOCKER=$(command -v docker)
export REAL_DOCKER CHECK_IMAGE
work=$(mktemp -d)
name="kicau-check-$$"
export APP_DIR="$work/stack" APP_CONTAINER="${name}-app" DB_CONTAINER="${name}-db" STACK_PROJECT="$name"
export GATEWAY_NETWORK="${name}-gateway" DEPLOY_READY_TIMEOUT=12
mkdir -p "$APP_DIR/data/backups" "$APP_DIR/data/postgres18" "$work/bin" "$work/bundle"
cleanup() {
  "$REAL_DOCKER" rm -f "$APP_CONTAINER" "$DB_CONTAINER" "${name}-proxy" >/dev/null 2>&1 || true
  "$REAL_DOCKER" network rm "${name}_internal_net" "$GATEWAY_NETWORK" >/dev/null 2>&1 || true
  "$REAL_DOCKER" image rm "${name}-bad:fixture" >/dev/null 2>&1 || true
  # PostgreSQL bind mounts may contain files owned by the container UID.
  "$REAL_DOCKER" run --rm -v "$work:/cleanup" alpine:3.22 sh -c 'rm -rf /cleanup/stack/data' >/dev/null 2>&1 || true
  rm -rf -- "$work"
}
on_exit() {
  local status=$?
  if (( status )) && [[ -f $work/migrations.log ]]; then tail -n 40 "$work/migrations.log" >&2; fi
  cleanup
}
trap on_exit EXIT
bash "$ROOT/cloud-app/deployment/package.sh" "$(printf '%040d' 1)" "$work/bundle.tgz" >/dev/null
tar -xzf "$work/bundle.tgz" -C "$work/bundle"
cp "$work/bundle/stack/docker-compose.yml" "$APP_DIR/docker-compose.yml"
cp "$work/bundle/stack/app.env.example" "$APP_DIR/app.env"
printf 'APP_IMAGE=%s\nAPP_CONTAINER=%s\nDB_CONTAINER=%s\nGATEWAY_NETWORK=%s\nDB_BIND_PORT=0\n' "$CHECK_IMAGE" "$APP_CONTAINER" "$DB_CONTAINER" "$GATEWAY_NETWORK" > "$APP_DIR/.env"
printf 'POSTGRES_DB=kicau_mania\nPOSTGRES_USER=postgres\nPOSTGRES_PASSWORD=fixture-admin-only\n' > "$APP_DIR/database.env"
sed -i 's|^DATABASE_URL=.*|DATABASE_URL=postgres://cloud_fixture:fixture-service-only@postgres:5432/kicau_mania?sslmode=disable|' "$APP_DIR/app.env"
"$REAL_DOCKER" network create "$GATEWAY_NETWORK" >/dev/null
"$REAL_DOCKER" compose --project-name "$name" --project-directory "$APP_DIR" --env-file "$APP_DIR/.env" -f "$APP_DIR/docker-compose.yml" up -d --wait postgres
bash "$work/bundle/scripts/db/initialize-db.sh" "$work/bundle" > "$work/migrations.log" 2>&1
KICAU_DB_PASSWORD=fixture-service-only bash "$work/bundle/scripts/db/grant-access.sh" cloud_fixture app
# Exercise SQL quoting without placing a real credential in any process argument.
export KICAU_DB_PASSWORD="fixture'quoted;safe-long-password"
bash "$work/bundle/scripts/db/grant-access.sh" fixture_reader reader
export PGPASSWORD=$KICAU_DB_PASSWORD
"$REAL_DOCKER" exec -e PGPASSWORD "$DB_CONTAINER" psql -X -h 127.0.0.1 -U fixture_reader -d kicau_mania -v ON_ERROR_STOP=1 -c 'SELECT count(*) FROM cloud_read.v_arenas' >/dev/null
unset PGPASSWORD KICAU_DB_PASSWORD
"$REAL_DOCKER" exec -i "$DB_CONTAINER" psql -X -v ON_ERROR_STOP=1 -U postgres -d kicau_mania <<'SQL'
INSERT INTO asset.arenas(name) VALUES ('Persistence fixture');
SQL
# Proof that the application role is restricted, not superuser or view-writable.
"$REAL_DOCKER" exec -i "$DB_CONTAINER" psql -X -v ON_ERROR_STOP=1 -U postgres -d kicau_mania <<'SQL'
DO $$ BEGIN
IF (SELECT rolsuper FROM pg_roles WHERE rolname='cloud_fixture') THEN RAISE EXCEPTION 'Superuser fixture'; END IF;
IF has_table_privilege('cloud_fixture','cloud_read.v_arenas','INSERT') THEN RAISE EXCEPTION 'View is writable'; END IF;
END $$;
SQL
cat > "$work/proxy.conf" <<EOF
server { listen 80; resolver 127.0.0.11 valid=1s ipv6=off; set \$backend $APP_CONTAINER:8091; location / { proxy_pass http://\$backend; } }
EOF
"$REAL_DOCKER" run -d --name "${name}-proxy" --network "$GATEWAY_NETWORK" -p 127.0.0.1::80 -v "$work/proxy.conf:/etc/nginx/conf.d/default.conf:ro" nginx:1.28-alpine >/dev/null
port=$("$REAL_DOCKER" port "${name}-proxy" 80/tcp | awk -F: 'NR==1 {print $NF}')
export PUBLIC_API_URL="http://127.0.0.1:$port"
printf 'FROM %s\nENTRYPOINT ["sh", "-c", "sleep 600"]\n' "$CHECK_IMAGE" > "$work/Dockerfile.bad"
export CHECK_BAD_IMAGE="${name}-bad:fixture"
"$REAL_DOCKER" build -q -t "$CHECK_BAD_IMAGE" -f "$work/Dockerfile.bad" "$work" >/dev/null
cat > "$work/bin/docker" <<'WRAPPER'
#!/usr/bin/env bash
set -Eeuo pipefail
if [[ ${1:-} == compose ]]; then
  envfile=''
  args=("$@")
  for ((i=0;i<${#args[@]};i++)); do
    if [[ ${args[i]} == --env-file ]]; then envfile=${args[i+1]}; fi
  done
  if [[ $envfile == */image.env ]]; then
    if [[ " $* " == *' pull app '* ]]; then exit 0; fi
    image=$(grep '^APP_IMAGE=' "$envfile")
    if [[ $image == *@sha256:ffffffff* ]]; then export APP_IMAGE=$CHECK_BAD_IMAGE; else export APP_IMAGE=$CHECK_IMAGE; fi
  fi
fi
exec "$REAL_DOCKER" "$@"
WRAPPER
chmod +x "$work/bin/docker"
export PATH="$work/bin:$PATH"
good="ghcr.io/bachtiarilham/kicau-mania-cloud@sha256:$(printf 'a%.0s' {1..64})"
bad="ghcr.io/bachtiarilham/kicau-mania-cloud@sha256:$(printf 'f%.0s' {1..64})"
first=$(printf '%040d' 1)
second=$(printf '%040d' 2)
third=$(printf '%040d' 3)
deploy="$work/bundle/scripts/app/deploy.sh"
bash "$deploy" "$work/bundle" "$good" "$first"
previous=$(readlink -f "$APP_DIR/state/current")
export BOOTSTRAP_ADMIN_NAME='Fixture admin' BOOTSTRAP_ADMIN_EMAIL='admin@cicd.fixture.invalid' BOOTSTRAP_ADMIN_PHONE='+62811122334455' BOOTSTRAP_ADMIN_PASSWORD='FixtureBootstrapOnly123!'
"$REAL_DOCKER" exec -e BOOTSTRAP_ADMIN_NAME -e BOOTSTRAP_ADMIN_EMAIL -e BOOTSTRAP_ADMIN_PHONE -e BOOTSTRAP_ADMIN_PASSWORD "$APP_CONTAINER" /usr/local/bin/bootstrap-admin
unset BOOTSTRAP_ADMIN_PASSWORD
[[ $("$REAL_DOCKER" exec "$DB_CONTAINER" psql -X -U postgres -d kicau_mania -Atc 'SELECT count(*) FROM iam.users') == 1 ]]
# The failed release also carries broken configuration; rollback must use the old snapshot.
sed -i 's|^DATABASE_URL=.*|DATABASE_URL=postgres://invalid:fixture@postgres:5432/kicau_mania?sslmode=disable|' "$APP_DIR/app.env"
printf '%s\n' "$second" > "$work/bundle/COMMIT"
if bash "$deploy" "$work/bundle" "$bad" "$second"; then echo 'Expected failed deployment'; exit 1; fi
[[ $(readlink -f "$APP_DIR/state/current") == "$previous" ]]
grep -q 'rollback-ready' "$APP_DIR/state/deployments.log"
curl -fsS "$PUBLIC_API_URL/readyz" -o /dev/null
cp "$previous/app.env" "$APP_DIR/app.env"
printf '%s\n' "$third" > "$work/bundle/COMMIT"
bash "$deploy" "$work/bundle" "$good" "$third"
[[ $(readlink -f "$APP_DIR/state/current") != "$previous" ]]
count=$("$REAL_DOCKER" exec "$DB_CONTAINER" psql -X -U postgres -d kicau_mania -Atc "SELECT count(*) FROM asset.arenas WHERE name='Persistence fixture'")
[[ $count == 1 ]]
# Restart PostgreSQL without replacing its bind mount; sentinel must survive.
"$REAL_DOCKER" restart "$DB_CONTAINER" >/dev/null
for _ in {1..30}; do if "$REAL_DOCKER" exec "$DB_CONTAINER" pg_isready -U postgres -d kicau_mania >/dev/null 2>&1; then break; fi; sleep 1; done
[[ $("$REAL_DOCKER" exec "$DB_CONTAINER" psql -X -U postgres -d kicau_mania -Atc 'SELECT count(*) FROM asset.arenas') == 1 ]]
# Verify backup/restore on the same isolated PostgreSQL, then start the restored app.
bash "$work/bundle/scripts/db/backup-db.sh"
snapshot=$(find "$APP_DIR/data/backups" -name '*.dump' -type f | sort | head -1)
"$REAL_DOCKER" exec "$DB_CONTAINER" psql -X -v ON_ERROR_STOP=1 -U postgres -d kicau_mania -c "INSERT INTO asset.arenas(name) VALUES ('After snapshot')" >/dev/null
bash "$work/bundle/scripts/db/restore-db.sh" "$snapshot" --replace
[[ $("$REAL_DOCKER" exec "$DB_CONTAINER" psql -X -U postgres -d kicau_mania -Atc 'SELECT count(*) FROM asset.arenas') == 1 ]]
"$REAL_DOCKER" start "$APP_CONTAINER" >/dev/null
for _ in {1..30}; do if curl -fsS "$PUBLIC_API_URL/readyz" -o /dev/null 2>/dev/null; then break; fi; sleep 1; done
curl -fsS "$PUBLIC_API_URL/readyz" -o /dev/null
[[ $("$REAL_DOCKER" exec "$APP_CONTAINER" id -u) == 10001 ]]
echo 'Acceptance passed: migrations, restricted roles/password quoting, bootstrap, readiness, sequential deploys, failed image rollback, PostgreSQL persistence, and snapshot restore'
