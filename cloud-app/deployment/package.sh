#!/usr/bin/env bash
set -Eeuo pipefail
# Explicit allowlist: VPS credentials, certificates and data never enter this bundle.
ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)
commit=${1:?Usage: package.sh COMMIT_SHA OUTPUT.tgz}
output=${2:?Output path required}
[[ $commit =~ ^[0-9a-f]{40}$ ]] || exit 2
work=$(mktemp -d)
trap 'rm -rf -- "$work"' EXIT
mkdir -p "$work/stack" "$work/database/migrations"
cp "$ROOT/cloud-app/deployment/opt/apps/kicau-mania/docker-compose.yml" "$work/stack/"
cp "$ROOT/cloud-app/deployment/opt/apps/kicau-mania/"*.example "$work/stack/" 2>/dev/null || true
# Hidden .env.example is explicit, not a wildcard over secrets.
cp "$ROOT/cloud-app/deployment/opt/apps/kicau-mania/.env.example" "$work/stack/"
while IFS= read -r -d '' script; do
  relative=${script#"$ROOT/cloud-app/deployment/opt/scripts/"}
  mkdir -p "$work/scripts/$(dirname "$relative")"
  cp "$script" "$work/scripts/$relative"
done < <(find "$ROOT/cloud-app/deployment/opt/scripts" -type f -name '*.sh' -print0)
mkdir -p "$work/infra/nginx/conf.d"
cp "$ROOT/cloud-app/deployment/opt/infra/docker-compose.yml" "$work/infra/"
cp "$ROOT/cloud-app/deployment/opt/infra/nginx/nginx.conf" "$work/infra/nginx/"
cp "$ROOT/cloud-app/deployment/opt/infra/nginx/conf.d/"*.conf "$work/infra/nginx/conf.d/"
cp "$ROOT/db-dump/full_db.dump" "$work/database/baseline.dump"
cp "$ROOT/cloud-app/db/migrations/"*.sql "$work/database/migrations/"
cp "$ROOT/cloud-app/deployment/devops_runbook.md" "$work/README.md"
printf '%s\n' "$commit" > "$work/COMMIT"
tar -czf "$output" -C "$work" .
sha256sum "$output"
