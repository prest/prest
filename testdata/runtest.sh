echo "\n\n.:: POSTGRES: DROP/CREATE DATABASE"
psql -h $PREST_PG_HOST -p $PREST_PG_PORT -U $PREST_PG_USER -c "DROP DATABASE IF EXISTS \"$PREST_PG_DATABASE\";"
psql -h $PREST_PG_HOST -p $PREST_PG_PORT -U $PREST_PG_USER -c "CREATE DATABASE \"$PREST_PG_DATABASE\";"
echo "\n\n.:: POSTGRES: LOAD DATA SCHEMA"
psql -h $PREST_PG_HOST -p $PREST_PG_PORT -U $PREST_PG_USER -d $PREST_PG_DATABASE -f ./testdata/schema.sql

echo "\n\n.:: GOLANG: DOWNLOAD MODULES"
go mod download

echo "\n\n.:: PRESTD: PLUGIN BUILD"
go build -o ./lib/hello.so -buildmode=plugin ./lib/src/hello.go;

echo "\n\n.:: PRESTD: MIGRATE UP"
go run ./cmd/prestd/main.go migrate up

echo "\n\n.:: PRESTD: TESTING STARTING..."
if [ -z ${1+x} ]; then
    go test -v -race -covermode=atomic -coverprofile=coverage.out ./...;
else
    go test -v -race -covermode=atomic -coverprofile=coverage.out $@
fi

echo "\n\n.:: POSTGRES: DROP DATABASES"
psql -h $PREST_PG_HOST -p $PREST_PG_PORT -U $PREST_PG_USER -c "DROP DATABASE IF EXISTS \"$PREST_PG_DATABASE\";"
