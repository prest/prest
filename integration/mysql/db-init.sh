#!/usr/bin/env bash
# Seed MySQL 8.0+ for pREST: app user, two databases, auto-inc / text / int / JSON.
set -euo pipefail

host="${MYSQL_HOST:-mysql}"
port="${MYSQL_PORT:-3306}"

export MYSQL_PWD="${MYSQL_ROOT_PASSWORD:?root password is required}"
mysql -h "$host" -P "$port" -u root <<'SQL'
CREATE DATABASE IF NOT EXISTS shop;
CREATE DATABASE IF NOT EXISTS other;
CREATE USER IF NOT EXISTS 'prest'@'%' IDENTIFIED BY 'prest';
GRANT ALL PRIVILEGES ON shop.* TO 'prest'@'%';
GRANT ALL PRIVILEGES ON other.* TO 'prest'@'%';
FLUSH PRIVILEGES;
SQL

export MYSQL_PWD="${PREST_PG_PASS:?app password is required}"
mysql -h "$host" -P "$port" -u "${PREST_PG_USER:?app user is required}" shop <<'SQL'
-- flag/raw/created_at/day/big cover boolean filters, binary-as-hex, date and
-- datetime formatting, and integers above 2^53. qty DEFAULT 0 backs the
-- heterogeneous-batch test (a missing key takes the column default).
CREATE TABLE IF NOT EXISTS items (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  name TEXT,
  qty INT DEFAULT 0,
  meta JSON,
  flag BOOLEAN,
  raw VARBINARY(8),
  created_at DATETIME(3),
  day DATE,
  big BIGINT
);
INSERT INTO items (name, qty, meta, flag, raw, created_at, day)
SELECT 'ada', 2, JSON_OBJECT('kind', 'hat'), TRUE, x'DEADBEEF', '2026-10-07 13:45:01.123', '2026-10-07'
WHERE NOT EXISTS (SELECT 1 FROM items WHERE name = 'ada');
-- A NULL-name, false-flag row for $null / $false without a trailing dot.
INSERT INTO items (name, qty, flag)
SELECT NULL, 3, FALSE
WHERE NOT EXISTS (SELECT 1 FROM items WHERE name IS NULL AND qty = 3);
-- Same DDL as the adapter's EnsureAuthTable. Password is bcrypt("s3cret").
CREATE TABLE IF NOT EXISTS prest_users (
  `id` BIGINT AUTO_INCREMENT PRIMARY KEY,
  `name` TEXT,
  `username` TEXT,
  `password` TEXT,
  `metadata` JSON,
  UNIQUE KEY `username` (`username`(255))
);
INSERT INTO prest_users (name, username, password)
SELECT 'Ada', 'ada', '$2a$10$/Vn89FdYAhur6fRHZt/cL.kIaGgtT8U36BDFjYy8CWArdkHFyJife'
WHERE NOT EXISTS (SELECT 1 FROM prest_users WHERE username = 'ada');
-- Queries-server admin (prest_queries.toml uses encrypt = "md5"): md5("123456").
INSERT INTO prest_users (name, username, password)
SELECT 'Queries Admin', 'test@postgres.rest', 'e10adc3949ba59abbe56e057f20f883e'
WHERE NOT EXISTS (SELECT 1 FROM prest_users WHERE username = 'test@postgres.rest');
-- Same shape as testdata/schema.sql test7, for the shared fulltable templates and
-- the registry lifecycle helper.
CREATE TABLE IF NOT EXISTS test7 (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  name TEXT,
  surname TEXT
);
INSERT INTO test7 (name, surname)
SELECT 'gopher', 'da silva'
WHERE NOT EXISTS (SELECT 1 FROM test7 WHERE name = 'gopher');
-- Second row so {{limitOffset .page .size}} has a page 2.
INSERT INTO test7 (name, surname)
SELECT 'gopher2', 'x'
WHERE NOT EXISTS (SELECT 1 FROM test7 WHERE name = 'gopher2');
SQL

mysql -h "$host" -P "$port" -u "${PREST_PG_USER}" other <<'SQL'
CREATE TABLE IF NOT EXISTS tags (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  name TEXT
);
SQL
