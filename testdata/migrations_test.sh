go install
export PREST_MIGRATIONS=./testdata/migrations
DB_HOST=${PREST_PG_HOST:-localhost}
DB_USER=${PREST_PG_USER:-postgres}
DB_PORT=${PREST_PG_PORT:-5438}
DB_NAME=${PREST_PG_DATABASE:-prest} 
DB_URL="postgres://${DB_USER}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=disable"
env go run main.go migrate  --url="$DB_URL" up
