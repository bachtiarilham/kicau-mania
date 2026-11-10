#!/usr/bin/env bash
set -Eeuo pipefail
[[ $EUID == 0 ]] || { echo 'Run as root'; exit 1; }
# shellcheck source=/dev/null
source /etc/os-release
[[ $ID == ubuntu || $ID == debian ]] || { echo 'Only Ubuntu/Debian are supported'; exit 1; }
case $(uname -m) in x86_64|aarch64) ;; *) echo 'Unsupported architecture'; exit 1;; esac
apt-get update
apt-get install -y ca-certificates curl gnupg openssl util-linux
install -m 0755 -d /etc/apt/keyrings
curl -fsSL "https://download.docker.com/linux/$ID/gpg" | gpg --dearmor --yes -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg
printf 'deb [arch=%s signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/%s %s stable\n' "$(dpkg --print-architecture)" "$ID" "$VERSION_CODENAME" > /etc/apt/sources.list.d/docker.list
apt-get update
apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
docker --version
docker compose version
