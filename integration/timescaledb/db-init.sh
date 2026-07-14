#!/usr/bin/env bash
# TimescaleDB integration seed: stock pREST schema plus extension + sample hypertable.
set -euo pipefail

bash -x ./testdata/db-init.sh

export PGPASSWORD="${PREST_PG_PASS:-}"

echo -e "\n\n.:: TIMESCALEDB: ENABLE EXTENSION + HYPERTABLE ON ${PREST_PG_DATABASE}"
psql -h "$PREST_PG_HOST" -p "$PREST_PG_PORT" -U "$PREST_PG_USER" -d "$PREST_PG_DATABASE" <<'SQL'
CREATE EXTENSION IF NOT EXISTS timescaledb CASCADE;

CREATE TABLE IF NOT EXISTS public.sensor_data (
    time TIMESTAMPTZ NOT NULL,
    device_id TEXT NOT NULL,
    temperature DOUBLE PRECISION
);

SELECT create_hypertable('public.sensor_data', 'time', if_not_exists => TRUE);

INSERT INTO public.sensor_data (time, device_id, temperature)
SELECT NOW() - (g || ' minutes')::interval, 'device-1', 20 + g
FROM generate_series(1, 5) AS g
WHERE NOT EXISTS (SELECT 1 FROM public.sensor_data LIMIT 1);
SQL
