DATABASES="$PREST_PG_DATABASE secondary-db"

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

echo "\n\n.:: GOLANG: DOWNLOAD MODULES"
go mod download

echo "\n\n.:: PRESTD: PLUGIN BUILD"
go build -o ./lib/hello.so -buildmode=plugin ./lib/src/hello.go;

echo "\n\n.:: PRESTD: MIGRATE UP"
go run ./cmd/prestd/main.go migrate up --path ./testdata/migrations

echo "\n\n.:: PRESTD: TESTING STARTING..."
if [ -z ${1+x} ]; then
    go test -v -race -failfast -covermode=atomic -coverprofile=coverage.out ./...;
else
    go test -v -race -failfast -covermode=atomic -coverprofile=coverage.out $@
fi

for db in $DATABASES; do
    echo "\n\n.:: POSTGRES: DROP DATABASE $db"
    psql -h $PREST_PG_HOST -p $PREST_PG_PORT -U $PREST_PG_USER -c "DROP DATABASE IF EXISTS \"$db\";"
done
