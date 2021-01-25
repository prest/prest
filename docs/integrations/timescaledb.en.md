---
date: 2020-12-28T11:30:00+03:00
title: Timescaledb
type: homepage
menu:
  timescaledb:
    parent: "timescaledb"
weight: 2
---

# pREST + TimescaleDB

Sample Dataset used: https://docs.timescale.com/latest/tutorials/other-sample-datasets#in-depth-devices

### Setup

#### Creating `docker-compose.yml` file

```bash
mkdir /tmp/prest+timescaledb
cd /tmp/prest+timescaledb
cat <<YML > docker-compose.yml
---
version: "3"
services:
  timescaledb:
    image: timescale/timescaledb:latest-pg12
    volumes:
      - "./data:/var/lib/postgresql/data"
      - "/tmp:/var/tmp"
    environment:
      - POSTGRES_USER=prest
      - POSTGRES_DB=prest
      - POSTGRES_PASSWORD=prest
    ports:
      - "5432:5432"
  prest:
    image: prest/prest:latest
    links:
      - "timescaledb:timescaledb"
    environment:
      - PREST_DEBUG=true  # remove comment for enable DEBUG mode (disable JWT)
      - PREST_PG_HOST=timescaledb
      - PREST_PG_USER=prest
      - PREST_PG_PASS=prest
      - PREST_PG_DATABASE=prest
      - PREST_PG_PORT=5432
      - PREST_JWT_DEFAULT=false  # remove if need jwt
    depends_on:
      - timescaledb
    ports:
      - "3000:3000"
YML
```

#### Starting up the containers

```bash
docker-compose pull
docker-compose up -d
```

#### Creating database structure

```bash
curl https://timescaledata.blob.core.windows.net/datasets/devices_small.tar.gz -o /tmp/devices_small.tar.gz
tar -C /tmp -xzvf /tmp/devices_small.tar.gz
docker-compose exec -T timescaledb psql -U prest -f /var/tmp/devices.sql
```

#### Loading data

```bash
docker-compose exec -T timescaledb psql -U prest <<SQL
COPY device_info FROM '/var/tmp/devices_small_device_info.csv' WITH (FORMAT CSV);
COPY readings FROM '/var/tmp/devices_small_readings.csv' WITH (FORMAT CSV);
SQL
```


### Simple Query

#### SQL execution

```bash
docker-compose exec -T timescaledb psql -U prest <<SQL
SELECT
    time, device_id, battery_temperature
FROM
    readings
WHERE
    battery_status = 'charging'
ORDER BY
    time DESC
LIMIT
    10;
SQL
```

#### pREST execution

```bash
curl -G http://localhost:3000/prest/public/readings \
  -d battery_status='$eq.charging' \
  -d _select=time,device_id,battery_temperature \
  -d _order=-time \
  -d _page=1 \
  -d _page_size=10
```


### Joining tables

#### SQL execution

```bash
docker-compose exec -T timescaledb psql -U prest <<SQL
SELECT
    time, readings.device_id, cpu_avg_1min,
    battery_level, battery_status, device_info.model
FROM
    readings
    JOIN device_info ON readings.device_id = device_info.device_id
WHERE
    battery_level < 33
    AND battery_status = 'discharging'
ORDER BY
    cpu_avg_1min DESC, time DESC
LIMIT
    5;
SQL
```

#### pREST execution

```bash
curl -G http://localhost:3000/prest/public/readings \
  -d battery_level='$lt.33' \
  -d battery_status='$eq.discharging' \
  -d _select='time,readings.device_id,cpu_avg_1min,battery_level,battery_status,device_info.model' \
  -d _join='inner:device_info:readings.device_id:$eq:device_info.device_id' \
  -d _order='-cpu_avg_1min,-time' \
  -d _page=1 \
  -d _page_size=5
```


### Using VIEWs

#### Creating VIEW named `battery_level_by_hour`

```bash
docker-compose exec -T timescaledb psql -U prest <<SQL
CREATE VIEW battery_level_by_hour AS
SELECT
    time_bucket('1 hour', time) AS "hour",
    model,
    min(battery_level) AS min_battery_level,
    max(battery_level) AS max_battery_level
FROM
    readings
    JOIN device_info ON readings.device_id = device_info.device_id
GROUP BY
    "hour", model;
SQL
```

#### Aggregating data over `battery_level_by_hour` view

```bash
docker-compose exec -T timescaledb psql -U prest <<SQL
SELECT
    hour, min(min_battery_level), max(max_battery_level)
FROM
    battery_level_by_hour
WHERE
    model IN ('pinto', 'focus')
GROUP BY
    hour
ORDER BY
    hour ASC
LIMIT
    12;
SQL
```

#### pREST execution

```bash
curl -G http://localhost:3000/prest/public/battery_level_by_hour \
  -d model='$in.pinto,focus' \
  -d _select='hour,min:min_battery_level,max:max_battery_level' \
  -d _groupby='hour' \
  -d _order='hour'
```

#### Simple SELECT over `battery_level_by_hour` view

```bash
docker-compose exec -T timescaledb psql -U prest <<SQL
SELECT
    hour, min_battery_level, max_battery_level
FROM
    battery_level_by_hour
WHERE
    model = 'mustang'
LIMIT
    5;
SQL
```

#### pREST execution over `battery_level_by_hour` view

```bash
curl -G http://localhost:3000/prest/public/battery_level_by_hour \
  -d model='$eq.mustang' \
  -d _select='hour,min_battery_level,max_battery_level' \
  -d _order='hour' \
  -d _page='1' \
  -d _page_size='5'
```


### Batch Insert data

#### Using default INSERT statement WITH returning inserted data

```bash
curl http://localhost:3000/batch/prest/public/readings \
  -H "Content-Type: application/json" \
  -d @- << JSON
  [
    {"time": "2020-10-17T20:19:30+00:00", "device_id": "demo000000", "battery_level": 43, "battery_status": "discharging", "battery_temperature": 89.8, "bssid": "01:02:03:04:05:06", "cpu_avg_1min": 28.84, "cpu_avg_5min": 16.9047812612903, "cpu_avg_15min": 10.8993036332756, "mem_free": 420023054, "mem_used": 579976946, "rssi": -40, "ssid": "demo-net"},
    {"time": "2020-10-17T20:19:30+00:00", "device_id": "demo000001", "battery_level": 27, "battery_status": "discharging", "battery_temperature": 89.3, "bssid": "A0:B1:C5:D2:E0:F3", "cpu_avg_1min": 4.89, "cpu_avg_5min": 6.63334573320236, "cpu_avg_15min": 9.25968056754939, "mem_free": 717784757, "mem_used": 282215243, "rssi": -41, "ssid": "stealth-net"},
    {"time": "2020-10-17T20:19:30+00:00", "device_id": "demo000002", "battery_level": 29, "battery_status": "discharging", "battery_temperature": 93.7, "bssid": "A0:B1:C5:D2:E0:F3", "cpu_avg_1min": 8.29, "cpu_avg_5min": 6.78591150918263, "cpu_avg_15min": 7.37546420066158, "mem_free": 634081377, "mem_used": 365918623, "rssi": -54, "ssid": "stealth-net"},
    {"time": "2020-10-17T20:19:30+00:00", "device_id": "demo000003", "battery_level": 14, "battery_status": "discharging", "battery_temperature": 93.1, "bssid": "01:02:03:04:05:06", "cpu_avg_1min": 8.83, "cpu_avg_5min": 8.18492270691781, "cpu_avg_15min": 11.3986054360923, "mem_free": 563352328, "mem_used": 436647672, "rssi": -30, "ssid": "demo-net"},
    {"time": "2020-10-17T20:19:30+00:00", "device_id": "demo000004", "battery_level": 58, "battery_status": "discharging", "battery_temperature": 93.2, "bssid": "22:32:A2:B3:05:98", "cpu_avg_1min": 8.79, "cpu_avg_5min": 10.3900175308572, "cpu_avg_15min": 13.5103326842724, "mem_free": 642162250, "mem_used": 357837750, "rssi": -62, "ssid": "demo-5ghz"}
  ]
JSON
```

#### Using COPY statement WITHOUT returning inserted data

```bash
curl http://localhost:3000/batch/prest/public/readings \
  -H "Content-Type: application/json" \
  -H "Prest-Batch-Method: copy" \
  -d @- << JSON
  [
    {"time": "2020-10-17T20:19:30+00:00", "device_id": "demo000000", "battery_level": 43, "battery_status": "discharging", "battery_temperature": 89.8, "bssid": "01:02:03:04:05:06", "cpu_avg_1min": 28.84, "cpu_avg_5min": 16.9047812612903, "cpu_avg_15min": 10.8993036332756, "mem_free": 420023054, "mem_used": 579976946, "rssi": -40, "ssid": "demo-net"},
    {"time": "2020-10-17T20:19:30+00:00", "device_id": "demo000001", "battery_level": 27, "battery_status": "discharging", "battery_temperature": 89.3, "bssid": "A0:B1:C5:D2:E0:F3", "cpu_avg_1min": 4.89, "cpu_avg_5min": 6.63334573320236, "cpu_avg_15min": 9.25968056754939, "mem_free": 717784757, "mem_used": 282215243, "rssi": -41, "ssid": "stealth-net"},
    {"time": "2020-10-17T20:19:30+00:00", "device_id": "demo000002", "battery_level": 29, "battery_status": "discharging", "battery_temperature": 93.7, "bssid": "A0:B1:C5:D2:E0:F3", "cpu_avg_1min": 8.29, "cpu_avg_5min": 6.78591150918263, "cpu_avg_15min": 7.37546420066158, "mem_free": 634081377, "mem_used": 365918623, "rssi": -54, "ssid": "stealth-net"},
    {"time": "2020-10-17T20:19:30+00:00", "device_id": "demo000003", "battery_level": 14, "battery_status": "discharging", "battery_temperature": 93.1, "bssid": "01:02:03:04:05:06", "cpu_avg_1min": 8.83, "cpu_avg_5min": 8.18492270691781, "cpu_avg_15min": 11.3986054360923, "mem_free": 563352328, "mem_used": 436647672, "rssi": -30, "ssid": "demo-net"},
    {"time": "2020-10-17T20:19:30+00:00", "device_id": "demo000004", "battery_level": 58, "battery_status": "discharging", "battery_temperature": 93.2, "bssid": "22:32:A2:B3:05:98", "cpu_avg_1min": 8.79, "cpu_avg_5min": 10.3900175308572, "cpu_avg_15min": 13.5103326842724, "mem_free": 642162250, "mem_used": 357837750, "rssi": -62, "ssid": "demo-5ghz"}
  ]
JSON
```
