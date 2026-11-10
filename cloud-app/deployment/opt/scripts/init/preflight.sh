#!/usr/bin/env bash
set -Eeuo pipefail
# Inspection only: this script never installs packages or creates networks.
# shellcheck source=/dev/null
source /etc/os-release
[[ $ID == ubuntu || $ID == debian ]] || { echo 'Only Ubuntu/Debian are supported'; exit 1; }
case $(uname -m) in x86_64|aarch64) ;; *) echo 'Only AMD64/ARM64 are supported'; exit 1;; esac
printf 'Platform: %s %s\n' "$ID" "$(uname -m)"
for cmd in docker curl flock tar sha256sum openssl; do command -v "$cmd" >/dev/null || { echo "Missing command: $cmd"; exit 1; }; done
docker info --format 'Docker server: {{.ServerVersion}}'
docker compose version
if docker network inspect web_gateway >/dev/null 2>&1; then
  subnet=$(docker network inspect web_gateway --format '{{range .IPAM.Config}}{{.Subnet}}{{end}}')
  [[ $subnet == "${GATEWAY_SUBNET:-172.30.20.0/24}" ]] || { echo 'Existing gateway subnet differs; align infra IP and TRUSTED_PROXIES before continuing'; exit 1; }
else
  echo 'Create web_gateway once with the documented subnet before startup'
fi
ss -ltn | awk 'NR==1 || /:(80|443|5432)[[:space:]]/'
echo 'Inspect occupied ports before setup. Existing services and data are never removed automatically.'
