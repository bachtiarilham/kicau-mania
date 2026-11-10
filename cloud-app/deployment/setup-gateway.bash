#!/usr/bin/env bash
# Post-migration API HTTPS setup. All configuration/credentials stay under /opt.
# Requires the Docker Nginx gateway and its Certbot tool service to be installed.
set -Eeuo pipefail
umask 077
[[ ${1:-} == --setup ]] || { echo 'Usage: bash setup-gateway.bash --setup'; exit 2; }
[[ $EUID == 0 ]] || { echo 'Run as root on the VPS'; exit 2; }
APP_DIR="${APP_DIR:-/opt/apps/kicau-mania}"
INFRA_DIR="${INFRA_DIR:-/opt/infra}"
DOMAIN="${DOMAIN:-dev.megaallstararena.com}"
EXPECTED_PUBLIC_IP="${EXPECTED_PUBLIC_IP:-187.124.138.204}"
DOCKER_CONFIG="${DOCKER_CONFIG:-$APP_DIR/config/docker}"
export DOCKER_CONFIG
stage=preflight
trap 'printf "GAGAL tahap %s, baris %s. Database tidak direset.\n" "$stage" "$LINENO" >&2' ERR
[[ $APP_DIR == /*/kicau-mania && $(readlink -m "$APP_DIR") == "$APP_DIR" ]] || exit 2
[[ $INFRA_DIR == /*/infra && $(readlink -m "$INFRA_DIR") == "$INFRA_DIR" ]] || exit 2
[[ $DOMAIN =~ ^[a-z0-9]([a-z0-9.-]*[a-z0-9])?$ && $DOMAIN == *.* ]] || exit 2
[[ -f $INFRA_DIR/docker-compose.yml && -f $APP_DIR/app.env ]]
grep -q 'certbot-tool:' "$INFRA_DIR/docker-compose.yml" || { echo 'Install the current Docker gateway stack first'; exit 1; }
STACK=$(find "$APP_DIR/incoming" -mindepth 2 -maxdepth 2 -type d -name stack -printf '%T@ %p\n' | sort -nr | sed -n '1s/^[^ ]* //p')
BUNDLE=${STACK%/stack}
[[ -f $BUNDLE/COMMIT ]]
commit=$(cat "$BUNDLE/COMMIT")
[[ $commit =~ ^[0-9a-f]{40}$ ]] || exit 2
mkdir -p "$DOCKER_CONFIG" "$APP_DIR/state"
chmod 700 "$DOCKER_CONFIG"
exec 9>"$APP_DIR/state/deploy.lock"
flock -w 180 9
stage=GHCR
image="ghcr.io/bachtiarilham/kicau-mania-cloud:$commit"
if ! docker pull "$image" >/dev/null 2>&1; then
  read -r -p 'GitHub username [bachtiarilham]: ' username </dev/tty
  username=${username:-bachtiarilham}
  read -r -s -p 'GitHub PAT classic read:packages: ' token </dev/tty; printf '\n'
  [[ -n $token ]]
  printf '%s' "$token" | docker login ghcr.io -u "$username" --password-stdin
  unset token
  docker pull "$image" >/dev/null
fi
stage=DNS
addresses=$(getent ahostsv4 "$DOMAIN" | awk '{print $1}' | sort -u) || addresses=''
if ! grep -Fxq "$EXPECTED_PUBLIC_IP" <<< "$addresses"; then
  echo "DNS belum siap: tambahkan A record $DOMAIN menuju $EXPECTED_PUBLIC_IP di pengelola DNS domain."
  echo 'Frontend utama/dev tetap berjalan; database dan layanan host tidak diubah.'
  exit 1
fi
cd "$INFRA_DIR"
compose() { docker compose --project-name gateway --project-directory "$INFRA_DIR" --env-file "$INFRA_DIR/.env" -f "$INFRA_DIR/docker-compose.yml" "$@"; }
stage=HTTPS
# Certbot writes only to the /opt-backed mounts. No apt, systemd or host crontab changes.
compose run --rm --no-deps certbot-tool certonly --non-interactive --agree-tos --register-unsafely-without-email \
  --webroot -w /var/www/acme --config-dir /etc/letsencrypt --work-dir /var/lib/letsencrypt --logs-dir /var/log/letsencrypt \
  --cert-name "$DOMAIN" -d "$DOMAIN" --keep-until-expiring
openssl x509 -in "$INFRA_DIR/certs/live/$DOMAIN/fullchain.pem" -noout -checkhost "$DOMAIN" >/dev/null
docker exec global_reverse_proxy nginx -t
docker exec global_reverse_proxy nginx -s reload
compose up -d certbot-renew
stage=TLS-check
status=$(curl --silent --show-error --connect-timeout 5 --max-time 15 -o /dev/null -w '%{http_code}' "https://$DOMAIN/healthz")
[[ $status =~ ^[2345][0-9]{2}$ ]]
printf '%s api-tls-ready %s HTTP-%s\n' "$(date -u +%FT%TZ)" "$DOMAIN" "$status" >> "$APP_DIR/state/deployments.log"
echo "SELESAI: GHCR dan HTTPS $DOMAIN siap (HTTP $status)."
echo 'HTTP 502 berarti aplikasi belum aktif; lanjutkan deployment lalu periksa /readyz.'
