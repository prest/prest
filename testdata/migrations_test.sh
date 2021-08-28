#!/usr/bin/env bash
go install
DB_HOST=${PREST_PG_HOST:-localhost}
DB_USER=${PREST_PG_USER:-postgres}
DB_PORT=${PREST_PG_PORT:-5438}
DB_NAME=${PREST_PG_DATABASE:-prest-test}
DB_URL="postgres://${PREST_PG_USER}:${PREST_PG_PASS}@${PREST_PG_HOST}:${PREST_PG_PORT}/${PREST_PG_DATABASE}?sslmode=disable"
env go run ./cmd/prestd/main.go migrate --url="$DB_URL" up
env go run ./cmd/prestd/main.go migrate --url="$DB_URL" up auth
