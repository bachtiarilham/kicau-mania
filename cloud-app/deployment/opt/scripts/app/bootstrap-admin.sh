#!/usr/bin/env bash
set -Eeuo pipefail
APP_CONTAINER="${APP_CONTAINER:-kicau_mania_app}"
read -r -p 'Admin name: ' BOOTSTRAP_ADMIN_NAME
read -r -p 'Admin email: ' BOOTSTRAP_ADMIN_EMAIL
read -r -p 'Admin phone (E.164): ' BOOTSTRAP_ADMIN_PHONE
read -r -s -p 'Admin password: ' BOOTSTRAP_ADMIN_PASSWORD </dev/tty
printf '\n'
export BOOTSTRAP_ADMIN_NAME BOOTSTRAP_ADMIN_EMAIL BOOTSTRAP_ADMIN_PHONE BOOTSTRAP_ADMIN_PASSWORD
docker exec -i -e BOOTSTRAP_ADMIN_NAME -e BOOTSTRAP_ADMIN_EMAIL -e BOOTSTRAP_ADMIN_PHONE -e BOOTSTRAP_ADMIN_PASSWORD \
  "$APP_CONTAINER" /usr/local/bin/bootstrap-admin
unset BOOTSTRAP_ADMIN_PASSWORD
