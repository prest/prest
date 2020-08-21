---
title: "Permissions"
date: 2017-08-30T19:06:32-03:00
weight: 13
menu: main
---

### Restrict mode
In the prest.toml you can configure read/write/delete permissions of each table.

```
[access]
restrict = true  # can access only the tables listed below
```

`restrict = false`: (default) the prest will serve in publish mode. You can write/read/delete everydata without configure permissions.

`restrict = true`: you need configure the permissions of all tables.

### Table permissions

Example:

```
[[access.tables]]
name = "test"
permissions = ["read", "write", "delete"]
fields = ["id", "name"]
```

Multiple configurations for the same table:

```
[access]
restrict = true  # can access only the tables listed below

    [[access.tables]]
    name = "test"
    permissions = ["read"]
    fields = ["id", "name"]

    [[access.tables]]
    name = "test"
    permissions = ["write"]
    fields = ["name"]
```

|attribute|description|
|---|---|
|name|Table name|
|permissions|Table permissions. Options: `read`, `write` and `delete`|
|fields|Fields permitted for operations|


Configuration example: [prest.toml](https://github.com/prest/prest/blob/master/testdata/prest.toml)
