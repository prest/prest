---
title: "Custom Queries"
date: 2017-08-30T19:06:24-03:00
weight: 6
---

If need perform an advanced SQL, you can write some scripts SQL and access them by REST. These scripts are templates where you can pass by URL, values to them.

**_awesome_folder/example_of_powerful.read.sql_:**

```sql
SELECT * FROM table WHERE name = "{{.field1}}" OR name = "{{.field2}}";
```

**Get result:**

```
GET /_QUERIES/awesome_folder/example_of_powerful?field1=foo&field2=bar
```

**To activate it, you need configure a location to scripts in your prest.toml like:**

```
[queries]
location = /path/to/queries/
```

## Scripts templates rules

In your scripts, the fields to replace have to look like: _field1 or field2 are examples_

```sql
SELECT * FROM table WHERE name = "{{.field1}}" OR name = "{{.field2}}";
```

Script file must have a suffix based on http verb:

|HTTP Verb|Suffix|
|---|---|
|GET|.read.sql|
|POST|.write.sql|
|PUT, PATCH|.update.sql|
|DELETE|.delete.sql|

In `queries.location`, you need given a folder to your scripts:

```sh
queries/
└── foo
    └── some_get.read.sql
    └── some_create.write.sql
    └── some_update.update.sql
    └── some_delete.delete.sql
└── bar
    └── some_get.read.sql
    └── some_create.write.sql
    └── some_update.update.sql
    └── some_delete.delete.sql

URLs to foo folder:

GET    /_QUERIES/foo/some_get?field1=bar
POST   /_QUERIES/foo/some_create?field1=bar
PUT    /_QUERIES/foo/some_update?field1=bar
PATCH  /_QUERIES/foo/some_update?field1=bar
DELETE /_QUERIES/foo/some_delete?field1=bar


URLs to bar folder:

GET    /_QUERIES/bar/some_get?field1=foo
POST   /_QUERIES/bar/some_create?field1=foo
PUT    /_QUERIES/bar/some_update?field1=foo
PATCH  /_QUERIES/bar/some_update?field1=foo
DELETE /_QUERIES/bar/some_delete?field1=foo
```

## Template data

You can access the query parameters of the incoming HTTP request using the `.` notation.

For instance, the following request:

```
GET    /_QUERIES/bar/some_get?field1=foo&field2=bar
```

makes available the fields `field1` and `field2` in the script:

```
{{.field1}}
{{.field2}}
```

You can also access the query headers of the incoming HTTP requests using the `.header` notation.

For instance, the following request:

```
GET    /_QUERIES/bar/some_get
X-UserId: am9obi5kb2VAYW5vbnltb3VzLmNvbQ
X-Application: prest
```

makes available the headers `X-UserId` and `X-Application` in the script:

```
{{index .header "X-UserId"}}
{{index .header "X-Application"}}
```

## Template functions

### isSet

Return true if param is set.

```sql
SELECT * FROM table
{{if isSet "field1"}}
WHERE name = "{{.field1}}"
{{end}}
;
```

### defaultOrValue

Return param value or default value.

```sql
SELECT * FROM table WHERE name = '{{defaultOrValue "field1" "gopher"}}';
```

### inFormat

If you need to format data for a usage on a `IN ('option1', 'option2')` statement.
You can use this with the field inside the format. Whenever passing multiple arguments to
`inFormat` function, you must use multiple `field1` instances on the URL.

In example: I want to query field `name` with options `Mary` and `John`.

```sql
-- URL will be equal to /_QUERIES/custom_query/query?name=Mary&name=John

SELECT * FROM names WHERE name IN {{inFormat "name"}};
```

**Future updates** will include the support of multiple strings splited by `,` on the same
instance of the field.

### split

Splits a string into substrings separated by a delimiter

```sql
SELECT * FROM table WHERE
name IN ({{ range $index,$part := split 'test1,test2,test3' `,` }}{{if gt $index 0 }},{{end}}'{{$part}}'{{ end }});
```

### limitOffset

Assemble `limit offset()` string with validation for non-allowed characters
_parameters must be integer values_

```sql
SELECT * FROM table {{limitOffset "1" "10"}}
```

**generating the query:**

```sql
SELECT * FROM table LIMIT 10 OFFSET(1 - 1) * 10
```

_We recommend using the default pREST variables `_page` and `_page_size`:_

```sql
{{limitOffset ._page ._page_size}}
```

## Ready-made queries

_consultations ready to use prest_

- [Opps CMS](https://github.com/opps/prest-queries)
