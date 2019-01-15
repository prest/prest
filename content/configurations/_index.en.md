---
title: "Configurations"
date: 2017-08-30T19:05:46-03:00
weight: 3
menu: main
---

Via environment variables or via toml file.

### Environment vars

- PREST\_HTTP_HOST (*default 0.0.0.0*)
- PREST\_HTTP_PORT or PORT (PORT is cloud factor, **when declaring this variable overwritten PREST\_HTTP_PORT**, *default 3000*)
- PREST\_PG_HOST (*default 127.0.0.1*)
- PREST\_PG_USER
- PREST\_PG_PASS
- PREST\_PG_DATABASE
- PREST\_PG_PORT (*default 5432*)
- PREST\_PG_URL or DATABASE\_URL (cloud factor, **when declaring this variable all the previous connection fields are overwritten**)
- PREST\_JWT_KEY
- PREST\_JWT_ALGO


## TOML
Optionally the pREST can be configured by TOML file.

- You can follow this sample and create your own `prest.toml` file and put this on the same folder that you run `prest` command.

```toml
migrations = "./migrations"

[http]
port = 6000 
# Port 6000 is blocked on windows. You must change to 8080 or any unblocked port

[jwt]
key = "secret"
algo = "HS256"

[pg]
host = "127.0.0.1"
user = "postgres"
pass = "mypass"
port = 5432
database = "prest"
## or used cloud factor
# URL = "postgresql://user:pass@localhost/mydatabase/?sslmode=disable"

[ssl]
mode = "disable"
sslcert = "./PATH"
sslkey = "./PATH"
sslrootcert = "./PATH"
```

## Authorization

- JWT middleware is enabled by default. To disable JWT need to set default to false.

```toml
[jwt]
default = false
```

```sh
Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWV9.TJVA95OrM7E2cBab30RMHrHDcEfxjoYZgeFONFh7HgQ
```

- The `HS256` algorithm is used by default.

The JWT algorithm can be specified by using either the environment variable `PREST_JWT_ALGO` or the `algo` parameter in the section `[jwt]` of the `prest.toml` configuration file.

The supported signing algorithms are:

* The [HMAC signing method](https://en.wikipedia.org/wiki/HMAC): `HS256`,`HS384`,`HS512`
* The [RSA signing method](https://en.wikipedia.org/wiki/RSA_(cryptosystem)): `RS256`,`RS384`,`RS512`
* The [ECDSA signing method](https://en.wikipedia.org/wiki/Elliptic_Curve_Digital_Signature_Algorithm): `ES256`,`ES384`,`ES512`


### SSL

- There is 4 options to set on ssl mode:

```toml
"disable" -  # no SSL (default)
"require" - # Always SSL (skip verification)
"verify-ca" - # Always SSL (verify that the certificate presented by the server was signed by a trusted CA)
"verify-full" - # Always SSL (verify that the certification presented by the server was signed by a trusted CA and the server host name matches the one in the certificate)
```

### Debug Mode

- Set environment variable `PREST_DEBUG` or `debug=true` on top of prest.toml file.

```
PREST_DEBUG=true
```

## Migrations

`--url` and `--path` flags are optional if pREST configurations already set.

```bash
# env var for migrations directory
PREST_MIGRATIONS

# create new migration file in path
prest migrate --url driver://url --path ./migrations create migration_file_xyz

# apply all available migrations
prest migrate --url driver://url --path ./migrations up

# roll back all migrations
prest migrate --url driver://url --path ./migrations down

# roll back the most recently applied migration, then run it again.
prest migrate --url driver://url --path ./migrations redo

# run down and then up command
prest migrate --url driver://url --path ./migrations reset

# show the current migration version
prest migrate --url driver://url --path ./migrations version

# apply the next n migrations
prest migrate --url driver://url --path ./migrations next +1
prest migrate --url driver://url --path ./migrations next +2
prest migrate --url driver://url --path ./migrations next +n

# roll back the previous n migrations
prest migrate --url driver://url --path ./migrations next -1
prest migrate --url driver://url --path ./migrations next -2
prest migrate --url driver://url --path ./migrations next -n

# go to specific migration
prest migrate --url driver://url --path ./migrations goto 1
prest migrate --url driver://url --path ./migrations goto 10
prest migrate --url driver://url --path ./migrations goto v
```
