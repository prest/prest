---
title: "Advanced Queries"
date: 2017-08-30T19:07:12-03:00
weight: 5
description: >
  Allows you to do some advanced queries, but with some limitations in order not to "dirty" the URL and parameters.
---

_**prestd**_ allows you to do some advanced queries, but with some limitations in order not to "dirty" the URL and parameters.

## JOIN

HTTP verb `GET`, allows you to join tables, with 1 level of depth - unfortunately the syntax is not so friendly so we limited it to 1 level only.

```
/{DATABASE}/{SCHEMA}/{TABLE}?_join={TYPE}:{TABLE JOIN}:{TABLE.FIELD}:{OPERATOR}:{TABLE JOIN.FIELD}
```

Parameters:

1. Type:
   - `inner`
   - `left`
   - `right`
   - `outer`
2. **Table** used in the join
3. **Table.field** - table name **dot** field
4. Operator:
   - `$eq`
   - `$lt`
   - `$gt`
   - `$lte`
   - `$gte`
6. **Table2.field** - table name **dot** field

Using query string to JOIN tables, example:

```
/{DATABASE}/{SCHEMA}/friends?_join=inner:users:friends.userid:$eq:users.id
```

{{< tip >}}
If you need multiple joins, we recommend using the queues feature (sql script execution).
{{</ tip >}}

## JSONb support

PostgreSQL offers type for storing jsonb data. To implement efficient query mechanisms for these data types.

```
?FIELD->>JSONFIELD:jsonb=VALUE
```

## Full Text Search (with tsquery)

Full Text Searching (or just text search) provides the capability to identify natural-language documents that satisfy a query, and optionally to sort them by relevance to the query. The most common type of search is to find all documents containing given query terms and return them in order of their similarity to the query. Notions of query and similarity are very flexible and depend on the specific application. The simplest search considers query as a set of words and similarity as the frequency of query words in the document.

Is native feature of PostgreSQL since version 8.3, read more [here](https://www.postgresql.org/docs/9.5/textsearch-intro.html).

> A tsquery value stores lexemes that are to be searched for, and combines them honoring the Boolean operators & (AND), | (OR), and ! (NOT). Parentheses can be used to enforce grouping of the operators.
> `SELECT 'fat & rat'::tsquery;`

```
?FIELD:tsquery=VALUE
```

#### Set language

You can specify the language you want to tokenize in, for example: **portuguese**

```
FIELD$LANGUAGE:tsquery=VALUE
```

**Language list:**

- simple
- arabic
- danish
- dutch
- english
- finnish
- french
- german
- hungarian
- indonesian
- irish
- italian
- lithuanian
- nepali
- norwegian
- portuguese
- romanian
- russian
- spanish
- swedish
- tamil
- turkish

To see all the languages available in your PostgreSQL run this query:

```sql
SELECT cfgname FROM pg_ts_config;
```

## Batch Insert

HTTP verb `POST`, you can insert many rows at once using batch endpoint `/batch/...`.

```
/batch/DATABASE/SCHEMA/TABLE

```

**JSON DATA:**

```
[
    {"FIELD1": "string value", "FIELD2": 1234567890},
    {"FIELD1": "other string value", "FIELD2":1234567891},
]
```

The default insert method is using multiple tuple values like `insert into table values ("value", 123), ("other", 456)`. Returns inserted rows.

You can change this behaviour using the header `Prest-Batch-Method` with value `copy`. It's useful for large insertions, but the return is empty.
