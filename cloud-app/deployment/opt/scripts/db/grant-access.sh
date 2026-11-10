#!/usr/bin/env bash
set -Eeuo pipefail
# shellcheck source=common.sh
source "$(dirname "${BASH_SOURCE[0]}")/common.sh"
user=${1:?Usage: grant-access.sh USER [reader|admin-reader|app|maintainer]}
[[ $user =~ ^[a-z][a-z0-9_]{0,30}$ && $user != postgres ]] || { echo 'Invalid login name'; exit 2; }
case ${2:-reader} in
  reader) role=kicau_cloud_reader;;
  admin-reader) role=kicau_cloud_admin_reader;;
  app) role=kicau_cloud_app;;
  maintainer) role=kicau_cloud_recap_maintainer;;
  *) echo 'Unknown access profile'; exit 2;;
esac
lock_database
role_exists=$(pg_psql -At -v target_role="$role" <<'SQL'
SELECT EXISTS(SELECT FROM pg_roles WHERE rolname=:'target_role' AND NOT rolcanlogin);
SQL
)
[[ $role_exists == t ]] || { echo "Required group role $role is missing; complete database migrations before creating a login"; exit 1; }
exists=$(pg_psql -At -v target_user="$user" <<'SQL'
SELECT EXISTS(SELECT FROM pg_roles WHERE rolname=:'target_user');
SQL
)
[[ $exists == f ]] || { echo 'Login already exists; change existing access explicitly with the DBA'; exit 1; }
if [[ -z ${KICAU_DB_PASSWORD:-} ]]; then
  read -r -s -p 'New login password: ' KICAU_DB_PASSWORD </dev/tty
  printf '\n'
fi
[[ ${#KICAU_DB_PASSWORD} -ge 16 ]] || { echo 'Use at least 16 characters'; exit 2; }
export KICAU_DB_PASSWORD
# Only the environment variable name appears in process arguments.
docker exec -i -e KICAU_DB_PASSWORD "$DB_CONTAINER" sh -c 'exec psql -X -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$POSTGRES_DB" "$@"' sh -1 -f - -v target_user="$user" -v target_role="$role" <<'SQL'
\getenv target_password KICAU_DB_PASSWORD
\getenv target_database POSTGRES_DB
SET password_encryption='scram-sha-256';
SELECT format('CREATE ROLE %I LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION PASSWORD %L', :'target_user', :'target_password') \gexec
SELECT format('GRANT CONNECT ON DATABASE %I TO %I', :'target_database', :'target_user') \gexec
SELECT format('GRANT %I TO %I', :'target_role', :'target_user') \gexec
SQL
unset KICAU_DB_PASSWORD
echo "Login created with profile: $role"
