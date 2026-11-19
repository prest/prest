#!/bin/sh
# Apply the deterministic seed to both e2e databases; prest_users only exists in prest_auth.
set -eu
for db in prest prest_auth; do
	psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$db" -f /seed/seed.sql
done
psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname prest_auth -f /seed/seed-auth.sql
