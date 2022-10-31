---
title: "Parameters"
date: 2017-08-30T19:07:12-03:00
weight: 2
description: >
  API parameters can be used to filter, sort, and paginate results and to select fields and relations to populate)
---

_**prestd**_ uses query string to apply filtering, sorting, paginating, and etc to api queries.

## Filters

HTTP method `GET`

| query string | Description |
| --- | --- |
| `_page={set page number}` | the api return is paged, this parameter sets which page you want |
| `_page_size={number to return by pages}` | delimits the number of records per page, default `10`. Every time you specify a page size, you must include the page you are accessing. |
| `?_select={field name 1},{fiel name 2}` | Limit fields list on result - sql ansii standard |
| `?_count={field name}` | Count per field - `*` representation all fields |
| `?_count_first=true` | Query string `_count` returns a list, passing this parameter will return the first record as a non-list object, **by default** this parameter is set to `false` (_return list non-object_) |
| `?_renderer=xml` | Set API render syntax, supported: `json` (by default), `xml` |
| `?_distinct=true` | `DISTINCT` clause with SELECT |
| `?_order={FIELD}` | `ORDER BY` in sql query. For `DESC` order, use the prefix `-`. For *multiple* orders, the fields are separated by comma `fieldname01,-fieldname02,fieldname03` |
| `?_groupby={FIELD}` | `GROUP BY` in sql query, The grouper is more complicated, a topic has been created to describe how to use |
| `?{FIELD NAME}={VALUE}` | Filter by field, you can set as many query parameters as needed |

### Functions support

Used to perform data **aggregation**(**grouping** and **selection**)

| name | Use in request |
| --- | --- |
| SUM | `sum:field` |
| AVG | `avg:field` |
| MAX | `max:field` |
| MIN | `min:field` |
| MEDIAN | `median:field` |
| STDDEV | `stddev:field` |
| VARIANCE | `variance:field` |

**`SELECT` with function:**

```
/{DATABASE}/{SCHEMA}/{TABLE}?_select=fieldname00,sum:fieldname01&_groupby=fieldname01
```

**`GROUP BY` with function:**

```
/{DATABASE}/{SCHEMA}/{TABLE}?_groupby=fieldname->>having:GROUPFUNC:FIELDNAME:CONDITION:VALUE-CONDITION
/{DATABASE}/{SCHEMA}/{TABLE}?_select=fieldname00,sum:fieldname01&_groupby=fieldname01->>having:sum:fieldname01:$gt:500
```

## Operators

Uses these operators in various filter applications

| Name | Description |
| --- | --- |
| `$eq` | Matches values that are equal to a specified value |
| `$gt` | Matches values that are greater than a specified value |
| `$gte` | Matches values that are greater than or equal to a specified value |
| `$lt` | Matches values that are less than a specified value |
| `$lte` | Matches values that are less than or equal to a specified value |
| `$ne` | Matches all values that are not equal to a specified value |
| `$in` | Matches any of the values specified in an array |
| `$nin` | Matches none of the values specified in an array |
| `$null` | Matches if field is null |
| `$notnull` | Matches if field is not null |
| `$true` | Matches if field is true |
| `$nottrue` | Matches if field is not true |
| `$false` | Matches if field is false |
| `$notfalse` | Matches if field is not false |
| `$like` | Matches always cover the entire string |
| `$ilike` | Matches _case-insensitive_ always cover the entire string |
| `$nlike` | Matches results without the entire string |
| `$nilike` | Matches _case-insensitive_ results without the entire string |
