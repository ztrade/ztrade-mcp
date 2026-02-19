#!/bin/bash
set -euo pipefail

RO_USER="${MYSQL_READONLY_USER:-exchange_readonly}"
RO_PASS="${MYSQL_READONLY_PASSWORD:-change_me_readonly_password}"
DB_NAME="${MYSQL_DATABASE:-exchange}"

if [[ ! "$RO_USER" =~ ^[A-Za-z0-9_]+$ ]]; then
  echo "[init-readonly] invalid MYSQL_READONLY_USER: $RO_USER" >&2
  exit 1
fi

if [[ "$RO_PASS" == *"'"* ]]; then
  echo "[init-readonly] MYSQL_READONLY_PASSWORD must not contain single quote (')." >&2
  exit 1
fi

if [[ ! "$DB_NAME" =~ ^[A-Za-z0-9_]+$ ]]; then
  echo "[init-readonly] invalid MYSQL_DATABASE: $DB_NAME" >&2
  exit 1
fi

mysql -uroot -p"${MYSQL_ROOT_PASSWORD}" <<-EOSQL
CREATE USER IF NOT EXISTS '$RO_USER'@'%' IDENTIFIED BY '$RO_PASS';
GRANT SELECT ON ${DB_NAME}.* TO '$RO_USER'@'%';
FLUSH PRIVILEGES;
EOSQL

echo "[init-readonly] readonly user '$RO_USER' granted SELECT on $DB_NAME"
