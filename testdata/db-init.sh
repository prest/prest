#!/usr/bin/env bash
DATABASES="$PREST_PG_DATABASE secondary-db"
SECONDARY_CLUSTER_DB="secondary-cluster"

set -euo pipefail
export PGPASSWORD="${PREST_PG_PASS:-}"
export PATH="/usr/local/go/bin:/go/bin:${PATH}"

for db in $DATABASES; do
    echo -e "\n\n.:: POSTGRES: DROP/CREATE DATABASE $db"
    psql -h "$PREST_PG_HOST" -p "$PREST_PG_PORT" -U "$PREST_PG_USER" -c "DROP DATABASE IF EXISTS \"$db\";"
    psql -h "$PREST_PG_HOST" -p "$PREST_PG_PORT" -U "$PREST_PG_USER" -c "CREATE DATABASE \"$db\";"
    echo -e "\n\n.:: POSTGRES: LOAD DATA SCHEMA"
    psql -h "$PREST_PG_HOST" -p "$PREST_PG_PORT" -U "$PREST_PG_USER" -d "$db" -f ./testdata/schema.sql
done

if [ -n "${PREST_PG_HOST_B:-}" ]; then
    echo -e "\n\n.:: POSTGRES-B: PROVISION SECONDARY CLUSTER DATABASE $SECONDARY_CLUSTER_DB"
    psql -h "$PREST_PG_HOST_B" -p "${PREST_PG_PORT:-5432}" -U "$PREST_PG_USER" -d "$SECONDARY_CLUSTER_DB" -f ./testdata/schema.sql
fi

echo -e "\n\n.:: GOLANG: DOWNLOAD MODULES"
go mod download

echo -e "\n\n.:: PRESTD: PLUGIN BUILD"
go build -o ./lib/hello.so -buildmode=plugin ./lib/src/hello.go

mkdir -p ./testdata/queries
chmod -R u+w ./testdata/queries

echo -e "\n\n.:: PRESTD: MIGRATE UP"
DB_URL="postgres://${PREST_PG_USER}:${PREST_PG_PASS}@${PREST_PG_HOST}:${PREST_PG_PORT}/${PREST_PG_DATABASE}?sslmode=disable"
export DATABASE_URL="$DB_URL"
go run ./cmd/prestd/main.go migrate --url="$DB_URL" up --path ./testdata/migrations
