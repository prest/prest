---
title: "Batch Operations"
date: 2018-05-14T12:00:00-03:00
weight: 13
menu: main
---

## Batch Insert - POST

You can insert many rows at once using batch endpoint.

```
http://127.0.0.1:8000/batch/DATABASE/SCHEMA/TABLE

```

JSON DATA:

```
[
    {"FIELD1": "string value", "FIELD2": 1234567890},
    {"FIELD1": "other string value", "FIELD2":1234567891},
]
```

The default insert method is using multiple tuple values like `insert into table values ("value", 123), ("other", 456)`. Returns inserted rows.

You can change this behaviour using the header `Prest-Batch-Method` with value `copy`. It's useful for large insertions, but the return is empty.
