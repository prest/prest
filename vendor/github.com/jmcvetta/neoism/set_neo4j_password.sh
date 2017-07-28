#!/bin/bash
#
# Set Neo4j password, on a fresh install, to 'foobar'.
#

curl -u neo4j:neo4j -H "Content-Type: application/json" -X POST -d '{"password":"foobar"}'  http://localhost:7474/user/neo4j/password
