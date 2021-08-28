#!/usr/bin/env bash
DB_HOST=${PREST_PG_HOST:-localhost}
DB_USER=${PREST_PG_USER:-postgres}
DB_PORT=${PREST_PG_PORT:-5432}
DB_NAME=${PREST_PG_DATABASE:-prest-test}

# create database var call PREST_PG_DATABASE
psql -h $DB_HOST -p $DB_PORT -U $DB_USER -c "DROP DATABASE IF EXISTS \"$DB_NAME\";"
psql -h $DB_HOST -p $DB_PORT -U $DB_USER -c "CREATE DATABASE \"$DB_NAME\";"

# load fixture data
psql -d $DB_NAME -h $DB_HOST -p $DB_PORT -U $DB_USER -f ./testdata/schema.sql
