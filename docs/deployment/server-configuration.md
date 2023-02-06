---
title: "Server Configuration"
date: 2017-08-30T19:05:46-03:00
weight: 2
---

The _**prestd**_ configuration is via an _environment variable_ or _toml_ file. Starting from version `v1.2.0` it will be possible to use `prestd` without any _environment variable_ or the _toml_ file, but the configurations used will be the described in the default column bellow.

## Environment variables

| var | default | description |
| --- | --- | --- |
| `PREST_VERSION` | 1 | version used for environment variables, v2 introduces better naming for SSL pg connection |
| `PREST_CONF` | ./prest.conf | |
| `PREST_SILENT_ERRORS` | false | enables silent errors, not displaying database infrastructure |
| `PREST_MIGRATIONS` | ./migrations | |
| `PREST_QUERIES_LOCATION` | ./queries | |
| `PREST_HTTP_HOST` | 0.0.0.0 | |
| `PREST_HTTP_PORT` or **PORT** | `3000` | `PORT` is for cloud factor, _when declared this variable overwrittes_ `PREST_HTTP_PORT` |
| `PREST_PG_HOST` | `127.0.0.1` | host used to connect |
| `PREST_PG_USER` | `postgres` | user used to connect |
| `PREST_PG_PASS` | `postgres` | password used to connect |
| `PREST_PG_DATABASE` | `prest` | database name used to connect |
| `PREST_PG_PORT` | `5432` | |
| `PREST_PG_URL` or **DATABASE\_URL** | | cloud factor, _when declaring this variable all the previous connection fields are overwritten_ |
| `PREST_CACHE_ENABLED` | false | embedded cache system |
| `PREST_CACHE_TIME` | 10 | TTL in minute (time to live) |
| `PREST_CACHE_STORAGEPATH` | ./ | path where the cache file will be created |
| `PREST_CACHE_SUFIXFILE` | .cache.prestd.db | suffix of the name of the file that is created |
| `PREST_JWT_KEY` | | |
| `PREST_JWT_ALGO` | HS256 | |
| `PREST_JWT_WHITELIST` | [/auth] | |
| `PREST_AUTH_ENABLED` | false | |
| `PREST_AUTH_ENCRYPT` | MD5 | |
| `PREST_AUTH_TYPE` | body | |
| `PREST_AUTH_SCHEMA` | public | |
| `PREST_AUTH_TABLE` | prest_users | |
| `PREST_AUTH_USERNAME` | username | |
| `PREST_AUTH_PASSWORD` | password | |
| `PREST_SSL_MODE` | require | SSL mode used to connect to postgres, not related to server SSL |
| `PREST_SSL_CERT` | | SSL certificate used to connect to postgres, not related to server SSL |
| `PREST_SSL_KEY` | | SSL key used to connect to postgres, not related to server SSL |
| `PREST_SSL_ROOTCERT` | | SSL root certificate used to connect to postgres, not related to server SSL |
| `PREST_PG_SSL_MODE` | require | v2 of configuration envs, is the postgres connection SSL mode |
| `PREST_PG_SSL_CERT` | | v2 of configuration envs, is the postgres connection SSL certificate |
| `PREST_PG_SSL_KEY` | | v2 of configuration envs, is the postgres connection SSL key |
| `PREST_PG_SSL_ROOTCERT` | | v2 of configuration envs, is the postgres connection SSL root certificate |
| `PREST_PLUGINPATH` | ./lib | path to plugin storage `.so`  |

## TOML

Optionally the prestd can be configured by TOML file.

You can follow this sample and create your own `prest.toml` file and put this on the same folder that you run `prestd` command.

```toml
migrations = "./migrations"

# debug = true
# enabling debug mode will disable JWT authorization

[http]
port = 3000

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
single = true
## or used cloud factor
# URL = "postgresql://user:pass@localhost/mydatabase/?sslmode=disable"

[ssl]
mode = "disable"
sslcert = "./PATH"
sslkey = "./PATH"
sslrootcert = "./PATH"

[expose]
enabled = true
databases = true
schemas = true
tables = true

[silent]
errors = true
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

The `HS256` algorithm is used by default.

The JWT algorithm can be specified by using either the environment variable `PREST_JWT_ALGO` or the `algo` parameter in the section `[jwt]` of the `prest.toml` configuration file.

The supported signing algorithms are:

* The [HMAC signing method](https://en.wikipedia.org/wiki/HMAC): `HS256`,`HS384`,`HS512`
* The [RSA signing method](https://en.wikipedia.org/wiki/RSA_(cryptosystem)): `RS256`,`RS384`,`RS512`
* The [ECDSA signing method](https://en.wikipedia.org/wiki/Elliptic_Curve_Digital_Signature_Algorithm): `ES256`,`ES384`,`ES512`

## White list

By default the endpoints `/auth` do not require JWT, the **whitelist** option serves to configure which endpoints will not ask for jwt token

```toml
[jwt]
default = true
whitelist = ["\/auth", "\/ping", "\/ping\/.*"]
```

## Auth

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

| Name | Description |
| --- | --- |
| enabled | **Boolean** field that activates or deactivates token generation endpoint support |
| type | Type that will receive the login, support for **body and http basic authentication** |
| encrypt | Type of encryption used in password field, support for **MD5 and SHA1** |
| table | Table name we will consult (query) |
| username | User **field** that will be consulted - if your software uses email just abstract name username (at pREST code level it was necessary to define an internal standard) |
| password | Password **field** that will be consulted |

> to validate all endpoints with generated jwt token must be activated jwt option

## Expose Data

The expose data setting enables you to configure if you want users to be able to reach listing endpoints, such as:

 - /databases
 - /schemas
 - /tables


An example of a configuration file disabling all listings:

```toml
# previous toml content
[expose]
    enabled = true
```

If you want to disable just the database listing:

```toml
# previous toml content
[expose]
    databases = true
```

| Name | Description |
| --- | --- |
| enabled | Set this as `true` if you want to **disable** all listing endpoints available. |
| databases | Set this as `false` if you want to **disable** *databases* listing endpoints only. |
| schemas | Set this as `false` if you want to **disable** *schemas* listing endpoints only. |
| tables | Set this as `false` if you want to **disable** *tables* listing endpoints only. |

### Default values for Exposure Settings

| Name | Default Value |
| --- | --- |
| enabled | `false` |
| databases | `true` |
| schemas | `true` |
| tables | `true` |


## SSL

There are 4 options to set on ssl mode:

| Name | Description | Comment |
| --- | --- | --- |
| `require` | Always SSL, is the default value | skips SSL verification step |
| `disable` | SSL off | also used when prestd is started without a `toml` file |
| `verify-ca` | Always SSL | verifies that the certificate presented is signed by a trusted CA |
| `verify-full` | Always SSL | verifies that the certificate presented is signed by a trusted CA and the server host name matches the one in the certificate |


## Debug Mode

Set environment variable `PREST_DEBUG` or `debug=true` on top of prest.toml file.

```toml
PREST_DEBUG=true
```

## Single mode

Serving multiple databases over the same API with `prestd` is doable, but it is not currently supported. Thus it was introduced by default the `single` configuration, it can be disabled by the following config in the `toml` file:

```toml
[pg]
    single = false
```

Since `v1.1.2` it is a lot safer to use multiple databases, but not yet in the ideal state of security that we want, so use it in your own risk.

## CORS support

**Cross-Origin Resource Sharing**

Read the specific topic where we talk about CROS [here](/prestd/deployment/cors-support/).


## Health check endpoint

If you need to setup a health check on your deployment (ECS/EKS or others), you can use `/_health` as a provider of this information.

The server will return 503 whenever a pREST is not working properly.  