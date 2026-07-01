DATABASES="$PREST_PG_DATABASE secondary-db"
SECONDARY_CLUSTER_DB="secondary-cluster"

# Create databases for tests
set -euo pipefail
export PGPASSWORD="${PREST_PG_PASS:-}"
export PATH="/usr/local/go/bin:/go/bin:${PATH}"

for db in $DATABASES; do
    echo "\n\n.:: POSTGRES: DROP/CREATE DATABASE $db"
    psql -h $PREST_PG_HOST -p $PREST_PG_PORT -U $PREST_PG_USER -c "DROP DATABASE IF EXISTS \"$db\";"
    psql -h $PREST_PG_HOST -p $PREST_PG_PORT -U $PREST_PG_USER -c "CREATE DATABASE \"$db\";"
    echo "\n\n.:: POSTGRES: LOAD DATA SCHEMA"
    psql -h $PREST_PG_HOST -p $PREST_PG_PORT -U $PREST_PG_USER -d $db -f ./testdata/schema.sql
done

if [ -n "${PREST_PG_HOST_B:-}" ]; then
    echo "\n\n.:: POSTGRES-B: PROVISION SECONDARY CLUSTER DATABASE $SECONDARY_CLUSTER_DB"
    psql -h "$PREST_PG_HOST_B" -p "${PREST_PG_PORT:-5432}" -U $PREST_PG_USER -d "$SECONDARY_CLUSTER_DB" -f ./testdata/schema.sql
fi

echo "\n\n.:: GOLANG: DOWNLOAD MODULES"
go mod download

echo "\n\n.:: PRESTD: PLUGIN BUILD"
go build -o ./lib/hello.so -buildmode=plugin ./lib/src/hello.go;

echo "\n\n.:: PRESTD: MIGRATE UP"
go run ./cmd/prestd/main.go migrate up --path ./testdata/migrations

echo "\n\n.:: PRESTD: TESTING STARTING..."
if [ -z ${1+x} ]; then
    go test -tags prest_test_hooks -v -race -failfast ./integration/...;
else
    go test -tags prest_test_hooks -v -race -failfast "$@"
fi

for db in $DATABASES; do
    echo "\n\n.:: POSTGRES: DROP DATABASE $db"
    psql -h $PREST_PG_HOST -p $PREST_PG_PORT -U $PREST_PG_USER -c "DROP DATABASE IF EXISTS \"$db\";"
done
