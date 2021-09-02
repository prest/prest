---
date: 2016-04-23T15:21:22+02:00
title: pREST Documentation
type: homepage
menu: main
weight: 1
---

## pREST (PostgreSQL REST)

Simplify and accelerate development, instant, realtime, high-performance on any **Postgres** application, **existing or new**.

It started with **PostgreSQL** and we'll stop there, we want to provide quality support on Postgres features. At first we wanted to embrace the world by supporting other databases, but as time went by (gaining experience) we realized that we would not do a good job in even one database.
> For this reason we decided to focus on Postgres, if you want to use it with another database (besides postgres) we recommend you to look at [postgres_fdw](https://www.postgresql.org/docs/9.5/postgres-fdw.html).

<a href="https://www.producthunt.com/posts/prest?utm_source=badge-featured&utm_medium=badge&utm_souce=badge-prest" target="_blank"><img src="https://api.producthunt.com/widgets/embed-image/v1/featured.svg?post_id=303506&theme=light" alt="pREST - instant, realtime, high-performance on PostgreSQL | Product Hunt" style="width: 250px; height: 54px;" width="250" height="54" /></a>

[**All releases**](/releases/)

## Mission

At _p_**REST**, we are on a mission to make the process of new business development faster by making data access _fast, secure and scalable_. We want to reach a world where access to large volumes of data is simple without complex software development!

## Test using Docker

> _To simplify the process of bringing up the test environment we will use **docker-compose**

```sh
# Download docker compose file
wget https://raw.githubusercontent.com/prest/prest/main/docker-compose-prod.yml -O docker-compose.yml

# Up (run) PostgreSQL and prestd
docker-compose up
# Run data migration to create user structure for access (JWT)
docker-compose exec prest prestd migrate up auth

# Create user and password for API access (via JWT)
## user: prest
## pass: prest
docker-compose exec postgres psql -d prest -U prest -c "INSERT INTO prest_users (name, username, password) VALUES ('pREST Full Name', 'prest', MD5('prest'))"
# Check if the user was created successfully (by doing a select on the table)
docker-compose exec postgres psql -d prest -U prest -c "select * from prest_users"

# Generate JWT Token with user and password created
curl -i -X POST http://127.0.0.1:3000/auth -H "Content-Type: application/json" -d '{"username": "prest", "password": "prest"}'
# Access endpoint using JWT Token
curl -i -X GET http://127.0.0.1:3000/prest/public/prest_users -H "Accept: application/json" -H "Authorization: Bearer {TOKEN}"
```

## Supported Operating System

- Linux
- macOS
- Windows
- BSD

Download the latest version binary [here](https://github.com/prest/prest/releases/latest)!
