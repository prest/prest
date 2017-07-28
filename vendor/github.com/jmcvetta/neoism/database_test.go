// Copyright (c) 2012-2013 Jason McVetta.  This is Free Software, released under
// the terms of the GPL v3.  See http://www.gnu.org/copyleft/gpl.html for details.
// Resist intellectual serfdom - the ownership of ideas is akin to slavery.

/*******************************************************************************

The Neo4j Manual section numbers quoted in the test suite refer to the manual
for milestone release 1.8.  http://docs.neo4j.org/chunked/1.8/

*******************************************************************************/

/*

To run these tests, a Neo4j 1.8 instance must be running on localhost on the
default port.

If the test suite complete successfully, all new nodes & relationships created
for testing will have been deleted in cleanup.  However if the test suite fails
to complete due to a panic, the cleanup code may not get called.  Therefore it
is not recommended to run this test suite on a db containing valuable data - run
it on a throwaway testing db instead! It's possible we could reduce this risk by
using defer() for cleanup.

To run these test for Google App Engine, run:
	goapp test

*/

package neoism

import (
	"log"
	"os"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/jmcvetta/randutil"
)

// neo4jUrl is global in order for TestConnect() to work when NEO4J_URL is set.
var neo4jUrl string

func connectTest(t *testing.T) *Database {
	log.SetFlags(log.Ltime | log.Lshortfile)
	neo4jUrl = os.Getenv("NEO4J_URL")
	if neo4jUrl == "" {
		// As of Neo4j v2.2.x, authentication is enabled by default.
		neo4jUrl = "http://neo4j:foobar@localhost:7474/db/data/"
	}
	db, err := Connect(neo4jUrl)
	// db.Session.Log = true
	if err != nil {
		t.Fatal(err)
	}
	return db
}


func cleanup(t *testing.T, db *Database) {
	qs := []*CypherQuery{
		&CypherQuery{
			Statement: `START r=rel(*) DELETE r`,
		},
		&CypherQuery{
			Statement: `START n=node(*) DELETE n`,
		},
	}
	err := db.CypherBatch(qs)
	if err != nil {
		t.Fatal(err)
	}
}

func rndStr(t *testing.T) string {
	// Neo4j doesn't like object names beginning with numerals.
	name, err := randutil.String(12, randutil.Alphabet)
	if err != nil {
		t.Fatal(err)
	}
	return name
}

func TestConnect(t *testing.T) {
	db := connectTest(t)
	assert.Equal(t, neo4jUrl, db.Url)
}

func TestConnectInvalidUrl(t *testing.T) {
	//
	//  Missing protocol scheme - url.Parse should fail
	//
	_, err := dbConnect("://foobar.com")
	if err == nil {
		t.Fatal("Expected error due to missing protocol scheme")
	}
	//
	// Unsupported protocol scheme - Session.Get should fail
	//
	_, err = dbConnect("foo://bar.com")
	if err == nil {
		t.Fatal("Expected error due to unsupported protocol scheme")
	}
	//
	// Not Found
	//
	_, err = dbConnect(neo4jUrl + "foo/")
	assert.Equal(t, InvalidDatabase, err)
}

func TestConnectIncompleteUrl(t *testing.T) {
	// url now has the format hostname:port/db/data, delete everything after the port
	regex := regexp.MustCompile(`^(https?:\/\/[^:]+:\d+)\/.*$`)
	replaced := regex.ReplaceAllString(neo4jUrl, "$1")
	//
	// 200 Success and HTML returned
	//
	_, err := dbConnect(replaced)
	if err != nil {
		t.Fatal("Hardsetting path on incomplete url failed")
	}
}

func TestPropertyKeys(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)

	// Prepare query for testing data creation
	var queryString string
	var createdPropertyKeys []string
	for i := 0; i < 10; i++ {
		propertyKeyNodeA := rndStr(t)
		propertyKeyRel := rndStr(t)
		propertyKeyNodeB := rndStr(t)
		createdPropertyKeys = append(createdPropertyKeys, propertyKeyNodeA)
		createdPropertyKeys = append(createdPropertyKeys, propertyKeyRel)
		createdPropertyKeys = append(createdPropertyKeys, propertyKeyNodeB)

		queryString += `
			CREATE ({` + propertyKeyNodeA + `:""})-[:LINK {` + propertyKeyRel + `:""}]->({` + propertyKeyNodeB + `:"Bob"})
		`
	}
	cq := CypherQuery{
		Statement: queryString,
	}

	// Execute the test data creation query
	err := db.Cypher(&cq)
	if err != nil {
		t.Error(err)
	}

	// Get all live property keys on nodes and relationships
	livePropertyKeys, err := PropertyKeys(db)
	if err != nil {
		t.Error(err)
	}

	// Check if the created property keys are among the extracted
	for _, createdPropertyKey := range createdPropertyKeys {
		keyExists := false
		for _, livePropertyKey := range livePropertyKeys {
			if createdPropertyKey == livePropertyKey {
				keyExists = true
				break
			}
		}
		if !keyExists {
			t.Error("Could not find the expected property key: " + createdPropertyKey)
		}
	}
}

