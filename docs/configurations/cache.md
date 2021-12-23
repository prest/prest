---
title: "Cache"
date: 2021-12-23T19:05:46-03:00
weight: 1
type: homepage
menu: configurations
---

Simple caching system to cache the API return in the http GET method.

We use key and value database embedded in _prestd_ ([BuntDB](https://github.com/tidwall/buntdb)).

We have a timeout system (TTL) for the cached data, by default it is kept for `10 minutes` - with the possibility to change it in the settings.

## Data Architecture

For each URI (with its parameters) a _BuntDB_ database cache file is created.

> It was implemented this way with performance in mind - there is no point in putting in a caching system that is slower than the SQL query in PostgreSQL.

- **key:** URI with all string query parameters
- **value:** json return (http body)

### Because BuntDB

Is a low-level, in-memory, key/value store in pure Go. It persists to disk, is ACID compliant, and uses locking for multiple readers and a single writer. It supports custom indexes and geospatial data. It's ideal for projects that need a dependable database and favor speed over data size.

We didn't want to depend on an external database (and we can't create tables in the existing database), with this premise we decided to use an embedded database (write in Go language) and BuntDB proved to be the best option at the moment, [here you can see the discussion existing since **2017**](https://github.com/prest/prest/issues/112).

## Environment vars

| var | default | description |
| --- | --- | --- |
| PREST\_CACHE | false | embedded cache system |
| PREST\_CACHE_TIME | 10 | TTL in minute (time to live) |
| PREST\_CACHE_STORAGEPATH | ./ | path where the cache file will be created |
| PREST\_CACHE_SUFIXFILE | .cache.prestd.db | suffix of the name of the file that is created |

## TOML

Optionally the pREST can be configured by TOML file.

You can follow this sample and create your own `prest.toml` file and put this on the same folder that you run `prestd` command.

```toml
cache = true
[cache]
time = 10
storagepath = "./"
sufixfile = ".cache.prestd.db"
```
