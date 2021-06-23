#!/usr/bin/env bash
DB_HOST=${PREST_PG_HOST:-localhost}
DB_USER=${PREST_PG_USER:-postgres}
DB_PORT=${PREST_PG_PORT:-5432}
DB_NAME=${PREST_PG_DATABASE:-prest}

# create database var call PREST_PG_DATABASE
psql -h $DB_HOST -p $DB_PORT -U $DB_USER -c "DROP DATABASE IF EXISTS \"$DB_NAME\";"
psql -h $DB_HOST -p $DB_PORT -U $DB_USER -c "create database \"$DB_NAME\";"

# create database loadtest
psql -h $DB_HOST -p $DB_PORT -U $DB_USER -c "DROP DATABASE IF EXISTS \"loadtest\";"
psql -h $DB_HOST -p $DB_PORT -U $DB_USER -c "create database \"loadtest\";"

# load fixture data
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -f $GITHUB_WORKSPACE/testdata/schema.sql
