---
date: 2020-12-28T11:30:00+03:00
title: Amazon Redshift
type: homepage
menu:
  redshift:
    parent: "redshift"
weight: 3
---

Analyze all of your data with the fastest and most widely used cloud data warehouse.

> Amazon Redshift is based on PostgreSQL. Amazon Redshift and PostgreSQL have a number of very important differences that you must be aware of as you design and develop your data warehouse applications.
> [read more](https://docs.aws.amazon.com/redshift/latest/dg/c_redshift-and-postgres-sql.html)

Amazon Redshift is compatible with postgresql just use the ODBC connection in the environment variable `DATABASE_URL` with the parameter `OpenSourceSubProtocolOverride` to make pREST connect to the database.

```sh
DATABASE_URL=postgresql://localhost:5432/postgres?OpenSourceSubProtocolOverride=true
```
