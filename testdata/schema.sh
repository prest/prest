DB_HOST=${PREST_PG_HOST:-localhost}
DB_USER=${PREST_PG_USER:-postgres}
DB_PORT=${PREST_PG_PORT:-5438}
DB_NAME=${PREST_PG_DATABASE:-prest} 

psql -h $DB_HOST -p $DB_PORT -U $DB_USER -v DB_NAME=$DB_NAME -f testdata/schema.sql
