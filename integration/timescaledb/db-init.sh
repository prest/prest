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

-- Create a continuous aggregate for testing
CREATE MATERIALIZED VIEW IF NOT EXISTS public.sensor_data_hourly WITH (timescaledb.continuous) AS
SELECT
  time_bucket('1 hour', time) AS hour,
  device_id,
  AVG(temperature) AS avg_temperature,
  COUNT(*) AS measurement_count
FROM public.sensor_data
GROUP BY hour, device_id
WITH NO DATA;

-- Refresh the continuous aggregate to populate with data
REFRESH MATERIALIZED VIEW public.sensor_data_hourly;
SQL
