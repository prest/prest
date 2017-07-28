#!/bin/bash
#
# Copied from https://github.com/versae/neo4j-rest-client under GPL v3 license.
#

DEFAULT_VERSION="1.6.3"
VERSION=${1-$DEFAULT_VERSION}
DIR="neo4j-community-$VERSION"
FILE="$DIR-unix.tar.gz"
SERVER_PROPERTIES_FILE="lib/neo4j/conf/neo4j-server.properties"
#set a default neo4j port if none has been set
NEO4J_PORT=${NEO4J_PORT:="7474"}

if [[ ! -d lib/$DIR ]]; then
    wget http://dist.neo4j.org/$FILE
    tar xvfz $FILE &> /dev/null
    rm $FILE
    [[ ! -d lib ]] && mkdir lib
    mv $DIR lib/
    [[ -h lib/neo4j ]] && unlink lib/neo4j
    ln -fs $DIR lib/neo4j
    mkdir lib/neo4j/testing/
    PLUGIN_VERSION="1.6"
    if [[ $NEO4J_VERSION == 1.5* ]]; then
        PLUGIN_VERSION="1.5"
    fi
    DELETE_DB_PLUGIN_JAR="test-delete-db-extension-$PLUGIN_VERSION.jar"
    wget --no-check-certificate -O lib/neo4j/testing/$DELETE_DB_PLUGIN_JAR https://github.com/downloads/jexp/neo4j-clean-remote-db-addon/$DELETE_DB_PLUGIN_JAR
    ln -s ../testing/$DELETE_DB_PLUGIN_JAR lib/neo4j/plugins/$DELETE_DB_PLUGIN_JAR
    cat >> $SERVER_PROPERTIES_FILE <<EOF
org.neo4j.server.thirdparty_jaxrs_classes=org.neo4j.server.extension.test.delete=/cleandb
org.neo4j.server.thirdparty.delete.key=supersecretdebugkey!
EOF
fi

if grep 7474 $SERVER_PROPERTIES_FILE > /dev/null; then
    sed -i s/7474/$NEO4J_PORT/g  $SERVER_PROPERTIES_FILE #change port
fi
