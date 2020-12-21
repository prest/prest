---
title: "Configurations"
date: 2017-08-30T19:05:46-03:00
weight: 3
menu: main
---

Via environment variables or via toml file.

### Environment vars

- PREST\_CONF
- PREST\_HTTP_HOST (*default 0.0.0.0*)
- PREST\_HTTP_PORT or **PORT** (PORT is cloud factor, _when declaring this variable overwritten PREST\_HTTP_PORT, default 3000_)
- PREST\_PG_HOST (*default 127.0.0.1*)
- PREST\_PG_USER
- PREST\_PG_PASS
- PREST\_PG_DATABASE
- PREST\_PG_PORT (*default 5432*)
- PREST\_PG_URL or **DATABASE\_URL** (cloud factor, _when declaring this variable all the previous connection fields are overwritten_)
- PREST\_JWT_KEY
- PREST\_JWT_ALGO
- PREST\_JWT_WHITELIST (*default /auth*)
- PREST\_AUTH_ENABLED (*default false*)
- PREST\_AUTH_ENCRYPT (*default MD5*)
- PREST\_AUTH_TYPE (*default body*)
- PREST\_AUTH_TABLE (*default prest_users*)
- PREST\_AUTH_USERNAME (*default username*)
- PREST\_AUTH_PASSWORD (*default password*)


## TOML
Optionally the pREST can be configured by TOML file.

- You can follow this sample and create your own `prest.toml` file and put this on the same folder that you run `prest` command.

```toml
migrations = "./migrations"

# debug = true
# enabling debug mode will disable JWT authorization

[http]
port = 6000 
# Port 6000 is blocked on windows. You must change to 8080 or any unblocked port

[jwt]
key = "secret"
algo = "HS256"

[auth]
enabled = true
type = "body"
encrypt = "MD5"
table = "prest_users"
username = "username"
password = "password"

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

### JWT

JWT middleware is enabled by default. To disable JWT need to set default to false. Enabling debug mode will also disable it.

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

#### White list

By default the endpoints `/auth` do not require JWT, the **whitelist** option serves to configure which endpoints will not ask for jwt token

```toml
[jwt]
default = true
whitelist = ["\/auth", "\/ping", "\/ping\/.*"]
```

### Auth

pREST has support in jwt token generation based on two fields (example user and password), being possible to use an existing table from your database to login configuring some parameters in the configuration file (or environment variable), _by default this feature is_ **disabled**.

```toml
[auth]
enabled = true
type = "body"
encrypt = "MD5"
table = "prest_users"
username = "username"
password = "password"
```

| Name     | Description                                                                                                                                                           |
| -------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| enabled  | **Boolean** field that activates or deactivates token generation endpoint support                                                                                     |
| type     | Type that will receive the login, support for **body and http basic authentication**                                                                                  |
| encrypt  | Type of encryption used in password field, support for **MD5 and SHA1**                                                                                               |
| table    | Table name we will consult (query)                                                                                                                                    |
| username | User **field** that will be consulted - if your software uses email just abstract name username (at pREST code level it was necessary to define an internal standard) |
| password | Password **field** that will be consulted                                                                                                                             |


> to validate all endpoints with generated jwt token must be activated jwt option


## SSL

- There is 4 options to set on ssl mode:

```toml
"disable" -  # no SSL (default)
"require" - # Always SSL (skip verification)
"verify-ca" - # Always SSL (verify that the certificate presented by the server was signed by a trusted CA)
"verify-full" - # Always SSL (verify that the certification presented by the server was signed by a trusted CA and the server host name matches the one in the certificate)
```

## Debug Mode

- Set environment variable `PREST_DEBUG` or `debug=true` on top of prest.toml file.

```
PREST_DEBUG=true
```

## Migrations

`--url` and `--path` flags are optional if pREST configurations already set.

```bash
# env var for migrations directory
PREST_MIGRATIONS

# apply all available migrations
prestd migrate --url driver://url --path ./migrations up

# roll back all migrations
prestd migrate --url driver://url --path ./migrations down

# roll back the most recently applied migration, then run it again.
prestd migrate --url driver://url --path ./migrations redo

# run down and then up command
prestd migrate --url driver://url --path ./migrations reset

# show the current migration version
prestd migrate --url driver://url --path ./migrations version

# apply the next n migrations
prestd migrate --url driver://url --path ./migrations next +1
prestd migrate --url driver://url --path ./migrations next +2
prestd migrate --url driver://url --path ./migrations next +n

# roll back the previous n migrations
prestd migrate --url driver://url --path ./migrations next -1
prestd migrate --url driver://url --path ./migrations next -2
prestd migrate --url driver://url --path ./migrations next -n

# create or remove default pREST authentication table
prestd migrate up auth
prestd migrate down auth
```
