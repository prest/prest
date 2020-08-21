---
date: 2016-04-23T15:21:22+02:00
title: Installation
type: homepage
menu:
  getting-started:
    parent: "getting-started"
weight: 2
---

Here are all the ways you can install pREST, choose one that best fits your needs.

## Index

1. [Downloading the binary](/getting-started/installation/#downloading-the-binary)
1. [With Docker](/getting-started/installation/#with-docker)
1. [Using go install](/getting-started/installation/#using-go-install)
1. [With Homebrew](/getting-started/installation/#with-homebrew)

### Downloading the binary

For any OS you can download the latest version [here](https://github.com/prest/prest/releases/latest).

### With Docker

We only will need to download the pREST image from Docker Hub with:

```sh
docker pull prest/prest:v1
```

### Using go install

```sh
go install github.com/prest/prest/cmd/prestd
```

### With Homebrew

If none of the above suits you, there's still an option of installing using [Homebrew](https://brew.sh/)

```sh
brew install prest
```
