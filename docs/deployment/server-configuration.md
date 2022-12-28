---
title: "Server Configuration"
date: 2017-08-30T19:05:46-03:00
weight: 2
---

The _**prestd**_ configuration is via an _environment variable_ or _toml_ file.

## Environment variables

| var | default | description |
| --- | --- | --- |
| PREST\_CONF | ./prest.conf | |
| PREST\_MIGRATIONS | ./migrations | |
| PREST\_QUERIES_LOCATION | ./queries | |
| PREST\_HTTP_HOST | 0.0.0.0 | |
| PREST\_HTTP_PORT or **PORT** | 3000 | PORT is cloud factor, _when declaring this variable overwritten PREST\_HTTP_PORT |
| PREST\_PG_HOST | 127.0.0.1 | |
| PREST\_PG_USER | | |
| PREST\_PG_PASS | | |
| PREST\_PG_DATABASE | | |
| PREST\_PG_PORT | 5432 | |
| PREST\_PG_URL or **DATABASE\_URL** | | cloud factor, _when declaring this variable all the previous connection fields are overwritten_ |
| PREST\_CACHE_ENABLED | false | embedded cache system |
| PREST\_CACHE_TIME | 10 | TTL in minute (time to live) |
| PREST\_CACHE_STORAGEPATH | ./ | path where the cache file will be created |
| PREST\_CACHE_SUFIXFILE | .cache.prestd.db | suffix of the name of the file that is created |
| PREST\_JWT_KEY | | |
| PREST\_JWT_ALGO | HS256 | |
| PREST\_JWT_WHITELIST | [/auth] | |
| PREST\_AUTH_ENABLED | false | |
| PREST\_AUTH_ENCRYPT | MD5 | |
| PREST\_AUTH_TYPE | body | |
| PREST\_AUTH_SCHEMA | public | |
| PREST\_AUTH_TABLE | prest_users | |
| PREST\_AUTH_USERNAME | username | |
| PREST\_AUTH_PASSWORD | password | |
| PREST\_SSL_MODE | require | |
| PREST\_SSL_CERT | | |
| PREST\_SSL_KEY | | |
| PREST\_SSL_ROOTCERT | | |
| PREST\_PLUGINPATH | ./lib | path to plugin storage `.so`  |

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

There is 4 options to set on ssl mode:

* `require` - Always SSL (skip verification) **by default**
* `disable` - SSL off
* `verify-ca` - Always SSL (verify that the certificate presented by the server was signed by a trusted CA)
* `verify-full` - Always SSL (verify that the certification presented by the server was signed by a trusted CA and the server host name matches the one in the certificate)

## Debug Mode

Set environment variable `PREST_DEBUG` or `debug=true` on top of prest.toml file.

```toml
PREST_DEBUG=true
```

## Single mode

While serving multiple databases over the same API with pREST is doable, it's by default a single database setup. This is this way to prevent unwanted behavior that may make prest instable for users, in order to change that It's possible to pass a variable on your `toml` file to disable it under the `[pg]` tag as shown bellow.

```toml
[pg]
    single = false
```

## CORS support

**Cross-Origin Resource Sharing**

Read the specific topic where we talk about CROS [here](/prestd/deployment/cors-support/).


## Health check endpoint

If you need to setup a health check on your deployment (ECS/EKS or others), you can use `/_health` as a provider of this information.

The server will return 503 whenever a Postgres connection is not reachable.  