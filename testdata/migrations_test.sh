#!/usr/bin/env bash
go install
DB_URL="postgres://${PREST_PG_USER}:${PREST_PG_PASS}@${PREST_PG_HOST}:${PREST_PG_PORT}/${PREST_PG_DATABASE}?sslmode=${PREST_SSL_MODE}"
env go run ./cmd/prestd/main.go migrate --url="$DB_URL" up
env go run ./cmd/prestd/main.go migrate --url="$DB_URL" up auth
