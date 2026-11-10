#!/usr/bin/env bash
set -Eeuo pipefail
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
DEPLOY="$ROOT/cloud-app/deployment"
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
while IFS= read -r file; do bash -n "$file"; done < <(find "$DEPLOY" -type f -name '*.sh')
mkdir -p "$work/stack" "$work/infra"
cp "$DEPLOY/opt/apps/kicau-mania/docker-compose.yml" "$work/stack/"
cp "$DEPLOY/opt/apps/kicau-mania/app.env.example" "$work/stack/app.env"
cp "$DEPLOY/opt/apps/kicau-mania/database.env.example" "$work/stack/database.env"
printf 'APP_IMAGE=ghcr.io/bachtiarilham/kicau-mania-cloud@sha256:%064d\n' 0 > "$work/stack/.env"
docker compose --project-directory "$work/stack" -f "$work/stack/docker-compose.yml" --env-file "$work/stack/.env" config --quiet
docker compose -f "$DEPLOY/opt/infra/docker-compose.yml" config --quiet
mkdir -p "$work/certs/live/dev-api.megaallstararena.com"
openssl req -x509 -newkey rsa:2048 -nodes -days 1 -subj '/CN=dev-api.megaallstararena.com' \
  -keyout "$work/certs/live/dev-api.megaallstararena.com/privkey.pem" \
  -out "$work/certs/live/dev-api.megaallstararena.com/fullchain.pem" >/dev/null 2>&1
# Generated fixture key is temporary and never included in a release.
docker run --rm --network none \
  -v "$DEPLOY/opt/infra/nginx/nginx.conf:/etc/nginx/nginx.conf:ro" \
  -v "$DEPLOY/opt/infra/nginx/conf.d:/etc/nginx/conf.d:ro" \
  -v "$work/certs:/etc/letsencrypt:ro" nginx:1.28-alpine nginx -t
printf 'Deployment syntax and configuration checks passed\n'
