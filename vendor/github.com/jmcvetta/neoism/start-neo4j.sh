#!/bin/sh

DIR="neo4j-community-2.2.0"
FILE="$DIR-unix.tar.gz"

wget "http://dist.neo4j.org/$FILE"
tar zxf $FILE
# Disable authentication, if we enable it, we have to change the default neo4j user
# password and then run all the tests with a different one
sed -i "s/auth_enabled\=true/auth_enabled\=false/g" $DIR/conf/neo4j-server.properties
$DIR/bin/neo4j start
sleep 3
