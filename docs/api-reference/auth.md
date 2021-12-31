---
title: "Auth"
date: 2017-08-30T19:06:04-03:00
weight: 4
---

_**prestd**_ has support in **JWT Token** generation based on two fields (example user and password), being possible to use an existing table from your database to login configuring some parameters in the configuration file (or environment variable), _by default this feature is_ **disabled**.

- Bearer - [RFC 6750](https://tools.ietf.org/html/rfc6750), bearer tokens to access OAuth 2.0-protected resources
- Basic - [RFC 7617](https://tools.ietf.org/html/rfc7617), base64-encoded credentials. More information below

> understand more about _http authentication_ [see this documentation](https://developer.mozilla.org/en-US/docs/Web/HTTP/Authentication)

## Bearer

```sh
curl -i -X POST http://127.0.0.1:8000/auth -H "Content-Type: application/json" -d '{"username": "<username>", "password": "<password>"}'
```

## Basic

```sh
curl -i -X POST http://127.0.0.1:8000/auth --user "<username>:<password>"
```
