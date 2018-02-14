---
title: "Query Statements"
date: 2017-08-30T19:06:04-03:00
weight: 12
menu: main
---

### Filter (WHERE)

```
GET /DATABASE/SCHEMA/TABLE?FIELD=$eq.VALUE
```

Query Operators:

| Name | Description |
|-------|-------------|
| $eq | Matches values that are equal to a specified value.|
| $gt | Matches values that are greater than a specified value.|
| $gte | Matches values that are greater than or equal to a specified value.|
| $lt | Matches values that are less than a specified value.|
| $lte | Matches values that are less than or equal to a specified value.|
| $ne | Matches all values that are not equal to a specified value.|
| $in | Matches any of the values specified in an array.|
| $nin | Matches none of the values specified in an array.|
| $null | Matches if field is null.|
| $notnull | Matches if field is not null.|
| $true | Matches if field is true.|
| $nottrue | Matches if field is not true.|
| $false | Matches if field is false.|
| $notfalse | Matches if field is not false.|


### Filter (WHERE) with JSONb field

```
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?FIELD->>JSONFIELD:jsonb=VALUE (filter)
```

### Select - GET

```
http://127.0.0.1:8000/databases (show all databases)
http://127.0.0.1:8000/databases?_count=* (count all databases)
http://127.0.0.1:8000/databases?_renderer=xml (JSON by default)
http://127.0.0.1:8000/schemas (show all schemas)
http://127.0.0.1:8000/schemas?_count=* (count all schemas)
http://127.0.0.1:8000/schemas?_renderer=xml (JSON by default)
http://127.0.0.1:8000/tables (show all tables)
http://127.0.0.1:8000/tables?_renderer=xml (JSON by default)
http://127.0.0.1:8000/DATABASE/SCHEMA (show all tables, find by schema)
http://127.0.0.1:8000/DATABASE/SCHEMA?_renderer=xml (JSON by default)
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE (show all rows, find by database and table)
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?_select=column (select statement by columns)
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?_select=column[array id] (select statement by array colum)

http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?_select=* (select all from TABLE)
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?_count=* (use count function)
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?_count=column (use count function)
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?_page=2&_page_size=10 (pagination, page_size 10 by default)
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?FIELD=VALUE (filter)
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?_renderer=xml (JSON by default)


Select operations over a VIEW
http://127.0.0.1:8000/DATABASE/SCHEMA/VIEW?_select=column (select statement by columns in VIEW)
http://127.0.0.1:8000/DATABASE/SCHEMA/VIEW?_select=* (select all from VIEW)
http://127.0.0.1:8000/DATABASE/SCHEMA/VIEW?_count=* (use count function)
http://127.0.0.1:8000/DATABASE/SCHEMA/VIEW?_count=column (use count function)
http://127.0.0.1:8000/DATABASE/SCHEMA/VIEW?_page=2&_page_size=10 (pagination, page_size 10 by default)
http://127.0.0.1:8000/DATABASE/SCHEMA/VIEW?FIELD=VALUE (filter)
http://127.0.0.1:8000/DATABASE/SCHEMA/VIEW?_renderer=xml (JSON by default)

```

### Insert - POST

```
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE
```

JSON DATA:
```
{
    "FIELD1": "string value",
    "FIELD2": 1234567890
}
```

### Update - PATCH/PUT

Using query string to make filter (WHERE), example:

```
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?FIELD1=xyz
```

JSON DATA:
```
{
    "FIELD1": "string value",
    "FIELD2": 1234567890,
    "ARRAYFIELD": ["value 1","value 2"]
}
```
### Delete - DELETE

Using query string to make filter (WHERE), example:

```
http://127.0.0.1:8000/DATABASE/SCHEMA/TABLE?FIELD1=xyz
```

## JOIN

```
/DATABASE/SCHEMA/Table?_join=Type:Table2:Table.field:Operator:Table2.field
```
Parameters:

1. Type (INNER, LEFT, RIGHT, OUTER)
1. Table2
1. Table.field
1. Operator ($eq, $lt, $gt, $lte, $gte)
1. Table2.field

Using query string to JOIN tables, example:

```
/DATABASE/SCHEMA/friends?_join=inner:users:friends.userid:$eq:users.id
```

## Query Operators

| Name | Description |
|-------|-------------|
| $eq | Matches values that are equal to a specified value.|
| $gt | Matches values that are greater than a specified value.|
| $gte | Matches values that are greater than or equal to a specified value.|
| $lt | Matches values that are less than a specified value.|
| $lte | Matches values that are less than or equal to a specified value.|
| $ne | Matches all values that are not equal to a specified value.|
| $in | Matches any of the values specified in an array.|
| $nin | Matches none of the values specified in an array.|

## DISTINCT

To use *DISTINCT* clause with SELECT, follow this syntax `_distinct=true`.

Examples:
```
    GET /DATABASE/SCHEMA/TABLE/?_distinct=true
```

## ORDER BY

Using *ORDER BY* in queries you must pass in *GET* request the attribute `_order` with fieldname(s) as value. For *DESC* order, use the prefix `-`. For *multiple* orders, the fields are separated by comma.

Examples:

### ASC
    GET /DATABASE/SCHEMA/TABLE/?_order=fieldname

### DESC
    GET /DATABASE/SCHEMA/TABLE/?_order=-fieldname

### Multiple Orders
    GET /DATABASE/SCHEMA/TABLE/?_order=fieldname01,-fieldname02,fieldname03

    ## GROUP BY

We support this Group Functions:

| name | Use in request |
| ------- | ------------- |
| SUM | sum:field |
| AVG | avg:field |
| MAX | max:field |
| MIN | min:field |
| MEDIAN | median:field |
| STDDEV | stddev:field |
| VARIANCE | variance:field |

### Examples:
	GET /DATABASE/SCHEMA/TABLE/?_select=fieldname00,fieldname01&_groupby=fieldname01

#### Using Group Functions
	GET /DATABASE/SCHEMA/TABLE/?_select=fieldname00,sum:fieldname01&_groupby=fieldname01

#### Having support
To use Having clause with **Group By**, follow this syntax:

	_groupby=fieldname->>having:GROUPFUNC:FIELDNAME:CONDITION:VALUE-CONDITION

Example:

	GET /DATABASE/SCHEMA/TABLE/?_select=fieldname00,sum:fieldname01&_groupby=fieldname01->>having:sum:fieldname01:$gt:500
