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
CREATE TABLE IF NOT EXISTS items (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  name TEXT,
  qty INT,
  meta JSON
);
INSERT INTO items (name, qty, meta)
SELECT 'ada', 2, JSON_OBJECT('kind', 'hat')
WHERE NOT EXISTS (SELECT 1 FROM items WHERE name = 'ada');
SQL

mysql -h "$host" -P "$port" -u "${PREST_PG_USER}" other <<'SQL'
CREATE TABLE IF NOT EXISTS tags (
  id BIGINT AUTO_INCREMENT PRIMARY KEY,
  name TEXT
);
SQL
