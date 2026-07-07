#!/usr/bin/env bash
DATABASES="$PREST_PG_DATABASE secondary-db"

set -euo pipefail
export PGPASSWORD="${PREST_PG_PASS:-}"
export PATH="/usr/local/go/bin:/go/bin:${PATH}"

echo -e "\n\n.:: PRESTD: TESTING STARTING..."
if [ -z ${1+x} ]; then
    go test -tags prest_test_hooks -v -race -failfast ./integration/...
else
    go test -tags prest_test_hooks -v -race -failfast "$@"
fi

for db in $DATABASES; do
    echo -e "\n\n.:: POSTGRES: DROP DATABASE $db"
    psql -h "$PREST_PG_HOST" -p "$PREST_PG_PORT" -U "$PREST_PG_USER" -c "DROP DATABASE IF EXISTS \"$db\";"
done
