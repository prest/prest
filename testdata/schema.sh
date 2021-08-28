#!/usr/bin/env bash
PGPASSWORD=${PREST_PG_PASS:-prest} # psql uses another variable name for password

# create database var call PREST_PG_DATABASE
psql -h $PREST_PG_HOST -p $PREST_PG_PORT -U $PREST_PG_USER -c "DROP DATABASE IF EXISTS \"$PREST_PG_DATABASE\";"
psql -h $PREST_PG_HOST -p $PREST_PG_PORT -U $PREST_PG_USER -c "CREATE DATABASE \"$PREST_PG_DATABASE\";"

# load fixture data
psql -h $PREST_PG_HOST -p $PREST_PG_PORT -U $PREST_PG_USER -d $PREST_PG_DATABASE -f ./testdata/schema.sql
