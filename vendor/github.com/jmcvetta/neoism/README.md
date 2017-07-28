neoism - Neo4j client for Go
===========================

![Neoism Logo](https://raw.github.com/jmcvetta/neoism/master/neoism.png)

Package `neoism` is a [Go](http://golang.org) client library providing access to
the [Neo4j](http://www.neo4j.org) graph database via its REST API.


# Status

[![Build Status](https://travis-ci.org/jmcvetta/neoism.png?branch=master)](https://travis-ci.org/jmcvetta/neoism)
[![Build Status](https://drone.io/github.com/jmcvetta/neoism/status.png)](https://drone.io/github.com/jmcvetta/neoism/latest)
[![Circle CI](https://circleci.com/gh/jmcvetta/neoism.svg?style=svg)](https://circleci.com/gh/jmcvetta/neoism)
[![Coverage Status](https://coveralls.io/repos/jmcvetta/neoism/badge.png?branch=master)](https://coveralls.io/r/jmcvetta/neoism)

This driver is fairly complete, and may now be suitable for general use.  The
code has an extensive set of integration tests, but little real-world testing.
YMMV; use in production at your own risk.


# Requirements

[Go 1.1](http://golang.org/doc/go1.1) or later is required.

Tested against Neo4j 2.2.4 and Go 1.4.1.


# Installation

```
go get -v github.com/jmcvetta/neoism
```


# Documentation

See [GoDoc](http://godoc.org/github.com/jmcvetta/neoism) or
[Go Walker](http://gowalker.org/github.com/jmcvetta/neoism) for 
automatically generated documentation.


# Usage

## Connect to Neo4j Database

```go
db, err := neoism.Connect("http://localhost:7474/db/data")
```

## Create a Node

```go
n, err := db.CreateNode(neoism.Props{"name": "Captain Kirk"})
```


## Issue a Cypher Query

```go
// res will be populated with the query results.  It must be a slice of structs.
res := []struct {
		// `json:` tags matches column names in query
		A   string `json:"a.name"` 
		Rel string `json:"type(r)"`
		B   string `json:"b.name"`
	}{}

// cq holds the Cypher query itself (required), any parameters it may have 
// (optional), and a pointer to a result object (optional).
cq := neoism.CypherQuery{
	// Use backticks for long statements - Cypher is whitespace indifferent
	Statement: `
		MATCH (a:Person)-[r]->(b)
		WHERE a.name = {name}
		RETURN a.name, type(r), b.name
	`,
	Parameters: neoism.Props{"name": "Dr McCoy"},
	Result:     &res,
}

// Issue the query.
err := db.Cypher(&cq)

// Get the first result.
r := res[0]
```

## Issue Cypher queries with a transaction

```go
tx, err := db.Begin(qs)
if err != nil {
  // Handle error
}

cq0 := neoism.CypherQuery{
  Statement: `MATCH (a:Account) WHERE a.uuid = {account_id} SET balance = balance + {amount}`,
  Parameters: neoism.Props{"uuid": "abc123", amount: 20},
}
err = db.Cypher(&cq0)
if err != nil {
  // Handle error
}

cq1 := neoism.CypherQuery{
  Statement: `MATCH (a:Account) WHERE a.uuid = {account_id} SET balance = balance + {amount}`,
  Parameters: neoism.Props{"uuid": "def456", amount: -20},
}
err = db.Cypher(&cq1)
if err != nil {
  // Handle error
}

err := tx.Commit()
if err != nil {
  // Handle error
}
```


# Roadmap


## Completed:

* Node (create/edit/relate/delete/properties)
* Relationship (create/edit/delete/properties)
* Legacy Indexing (create/edit/delete/add node/remove node/find/query)
* Cypher queries
* Batched Cypher queries
* Transactional endpoint (Neo4j 2.0)
* Node labels (Neo4j 2.0)
* Schema index (Neo4j 2.0)
* Authentication (Neo4j 2.2)


## To Do:

* Streaming API support - see Issue [#22](https://github.com/jmcvetta/neoism/issues/22)
* ~~Unique Indexes~~ - probably will not expand support for legacy indexing.
* ~~Automatic Indexes~~ - "
* High Availability
* Traversals - May never be supported due to security concerns.  From the
  manual:  "The Traversal REST Endpoint executes arbitrary Groovy code under
  the hood as part of the evaluators definitions. In hosted and open
  environments, this can constitute a security risk."
* Built-In Graph Algorithms
* Gremlin


# Testing

Neoism's test suite respects, but does not require, a `NEO4J_URL` environment
variable.  By default it assumes Neo4j is running on `localhost:7474`, with
username `neo4j` and password `foobar`.  

```bash
export NEO4J_URL=http://your_user:your_password@neo4j.yourdomain.com/db/data/
go test -v .
```

If you are using a fresh untouched Neo4j instance, you can use the included
`set_neo4j_password.sh` script to set the password to that expected by Neoism's
tests:

```bash
sh set_neo4j_password.sh
```


# Contributing

Contributions, in the form of Pull Requests or Issues, are gladly accepted.
Before submitting a Pull Request, please ensure your code passes all tests, and
that your changes do not decrease test coverage.  I.e. if you add new features,
also add corresponding new tests.

For fastest response when submitting an Issue, please create a failing test
case to demonstrate the problem.


# License

This is Free Software, released under the terms of the [GPL
v3](http://www.gnu.org/copyleft/gpl.html).
