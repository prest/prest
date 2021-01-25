---
title: "Query Statements"
date: 2017-08-30T19:06:04-03:00
weight: 12
menu: main
---

## Auth /auth - POST

pREST has support in **JWT Token** generation based on two fields (example user and password), being possible to use an existing table from your database to login configuring some parameters in the configuration file (or environment variable), _by default this feature is_ **disabled**.

- Bearer - [RFC 6750](https://tools.ietf.org/html/rfc6750), bearer tokens to access OAuth 2.0-protected resources
- Basic - [RFC 7617](https://tools.ietf.org/html/rfc7617), base64-encoded credentials. More information below

> understand more about _http authentication_ [see this documentation](https://developer.mozilla.org/en-US/docs/Web/HTTP/Authentication)

### Bearer
```sh
curl -i -X POST http://127.0.0.1:8000/auth -H "Content-Type: application/json" -d '{"username": "<username>", "password": "<password>"}'
```

### Basic
```sh
curl -i -X POST http://127.0.0.1:8000/auth --user "<username>:<password>"
```

## Filter (WHERE)
Applying filter to the remaining queries, we use the parameters of the http **GET** method (_query string_), being converted to **WHERE** from _syntax SQL_.

```
GET /DATABASE/SCHEMA/TABLE?FIELD=$eq.VALUE
```

**Query Operators:**

| Name      | Description                                                         |
| --------- | ------------------------------------------------------------------- |
| $eq       | Matches values that are equal to a specified value.                 |
| $gt       | Matches values that are greater than a specified value.             |
| $gte      | Matches values that are greater than or equal to a specified value. |
| $lt       | Matches values that are less than a specified value.                |
| $lte      | Matches values that are less than or equal to a specified value.    |
| $ne       | Matches all values that are not equal to a specified value.         |
| $in       | Matches any of the values specified in an array.                    |
| $nin      | Matches none of the values specified in an array.                   |
| $null     | Matches if field is null.                                           |
| $notnull  | Matches if field is not null.                                       |
| $true     | Matches if field is true.                                           |
| $nottrue  | Matches if field is not true.                                       |
| $false    | Matches if field is false.                                          |
| $notfalse | Matches if field is not false.                                      |
| $like     | Matches always cover the entire string.                             |
| $ilike    | Matches *case-insensitive* always cover the entire string.          |


### With JSONb field

```
?FIELD->>JSONFIELD:jsonb=VALUE
```

### With Full Text Search (tsquery)

```
?FIELD:tsquery=VALUE
```

> **Set language:** `FIELD$LANGUAGE:tsquery=VALUE`

### Filters parameters (query string) - GET

| Query String                             | Description                                                                                                                                                    |
| ---------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `?_select={field name 1},{fiel name 2}`  | Limit fields list on result - sql ansii standard                                                                                                               |
| `?_count={field name}`                   | Count per field - `*` representation all fields                                                                                                                |
| `?_render=xml`                           | Set API render syntax - default is `json`                                                                                                                      |
| `?page={set page number}`                | Navigation on return pages with large volume of data                                                                                                           |
| `?page_size={number to return by pages}` | 10 is default number                                                                                                                                           |
| `?distinct=true`                         | `DISTINCT` clause with SELECT                                                                                                                                  |
| `?_order={FIELD}`                        | `ORDER BY` in sql query. For `DESC` order, use the prefix `-`. For *multiple* orders, the fields are separated by comma `fieldname01,-fieldname02,fieldname03` |
| `?_groupby={FIELD}`                      | `GROUP BY` in sql query, The grouper is more complicated, a topic has been created to describe how to use                                                      |
| `?{FIELD NAME}={VALUE}`                  | Filter by field, you can set as many query parameters as needed                                                                                                |

### Group/Select functions support:

| name     | Use in request |
| -------- | -------------- |
| SUM      | sum:field      |
| AVG      | avg:field      |
| MAX      | max:field      |
| MIN      | min:field      |
| MEDIAN   | median:field   |
| STDDEV   | stddev:field   |
| VARIANCE | variance:field |

#### Filter with function

```
/{DATABASE}/{SCHEMA}/{TABLE}?_select=fieldname00,sum:fieldname01&_groupby=fieldname01
```

#### GROUP BY with function

```
/{DATABASE}/{SCHEMA}/{TABLE}?_groupby=fieldname->>having:GROUPFUNC:FIELDNAME:CONDITION:VALUE-CONDITION
/{DATABASE}/{SCHEMA}/{TABLE}?_select=fieldname00,sum:fieldname01&_groupby=fieldname01->>having:sum:fieldname01:$gt:500
```


## Query Operators

Uses these operators in various filter applications

| Name | Description                                                         |
| ---- | ------------------------------------------------------------------- |
| $eq  | Matches values that are equal to a specified value.                 |
| $gt  | Matches values that are greater than a specified value.             |
| $gte | Matches values that are greater than or equal to a specified value. |
| $lt  | Matches values that are less than a specified value.                |
| $lte | Matches values that are less than or equal to a specified value.    |
| $ne  | Matches all values that are not equal to a specified value.         |
| $in  | Matches any of the values specified in an array.                    |
| $nin | Matches none of the values specified in an array.                   |


## GET - Endpoints

| Endpointis                          | Description                                               |
| ----------------------------------- | --------------------------------------------------------- |
| `/databases`                        | List all databases                                        |
| `/shemas`                           | List all schemas                                          |
| `/tables`                           | List all tables                                           |
| `/show/{DATABASE}/{SCHEMA}/{TABLE}` | Lists table structure - all fields contained in the table |
| `/{DATABASE}/{SCHEMA}`              | Lists table tables - find by schema                       |
| `/{DATABASE}/{SCHEMA}/{TABLE}`      | List all rows, find by database, schema and table         |
| `/{DATABASE}/{SCHEMA}/{VIEW}`       | List all rows, find by database, schema and view          |


## POST - Insert

```
/{DATABASE}/{SCHEMA}/{TABLE}
```

JSON DATA:
```
{
    "FIELD1": "string value",
    "FIELD2": 1234567890
}
```

## PATCH/PUT - Update

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
## DELETE - Delete

Using query string to make filter (WHERE), example:

```
/{DATABASE}/{SCHEMA}/{TABLE}?{FIELD NAME}={VALUE}
```

## JOIN

```
/{DATABASE}/{SCHEMA}/{TABLE}?_join={TYPE}:{TABLE JOIN}:{TABLE.FIELD}:{OPERATOR}:{TABLE JOIN.FIELD}
```
Parameters:

1. Type (INNER, LEFT, RIGHT, OUTER)
1. Table used in the join
1. Table.field - table name **dot** field
1. Operator ($eq, $lt, $gt, $lte, $gte)
1. Table2.field - table name **dot** field

Using query string to JOIN tables, example:

```
/{DATABASE}/{SCHEMA}/friends?_join=inner:users:friends.userid:$eq:users.id
```
