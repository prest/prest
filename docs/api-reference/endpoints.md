---
title: "Endpoints"
date: 2017-08-30T19:07:12-03:00
weight: 2
description: >
  Interact with prestd using the auto-generated RESTful protocol
---

_**prestd**_ implements all http verbs, transcribing to SQL ANSI (American National Standards Institute) statements.

## GET

> Postgres `SELECT` instruction

| Endpoints | Description |
| --- | --- |
| `/_health` | Health check endpoint |
| `/databases` | List all databases |
| `/schemas` | List all schemas |
| `/tables` | List all tables |
| `/show/{DATABASE}/{SCHEMA}/{TABLE}` | Lists table structure - all fields contained in the table |
| `/{DATABASE}/{SCHEMA}` | Lists table tables - find by schema |
| `/{DATABASE}/{SCHEMA}/{TABLE}` | List all rows, find by database, schema and table |
| `/{DATABASE}/{SCHEMA}/{VIEW}` | List all rows, find by database, schema and view |

## POST

> Postgres `INSERT` instruction

```
/{DATABASE}/{SCHEMA}/{TABLE}
```

**JSON DATA:**

```
{
    "FIELD1": "string value",
    "FIELD2": 1234567890
}
```

## PATCH and PUT

> Postgres `UPDATE` instruction

Using query string to make filter (WHERE), example:

```
/{DATABASE}/{SCHEMA}/{TABLE}?{FIELD NAME}={VALUE}
```

JSON DATA:

```
{
    "FIELD1": "string value",
    "FIELD2": 1234567890,
    "ARRAYFIELD": ["value 1","value 2"]
}
```

{{< tip "warning" >}}
> unconditional `update` can update unwanted record
{{</ tip >}}

## DELETE

> Postgres `DELETE` instruction

Using query string to make filter (WHERE), example:

```
/{DATABASE}/{SCHEMA}/{TABLE}?{FIELD NAME}={VALUE}
```

{{< tip "warning" >}}
> unconditional `delete` can delete unwanted record
{{</ tip >}}
