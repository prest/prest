// Copyright (c) 2012-2013 Jason McVetta.  This is Free Software, released under
// the terms of the GPL v3.  See http://www.gnu.org/copyleft/gpl.html for details.
// Resist intellectual serfdom - the ownership of ideas is akin to slavery.

package neoism

import (
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/assert"
	"net/http"
	"regexp"
	"strings"
	"testing"
)

func TestCreateIndex(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	defer cleanupIndexes(t, db)
	label := rndStr(t)
	prop0 := rndStr(t)
	idx, err := db.CreateIndex(label, prop0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, label, idx.Label)
	assert.Equal(t, prop0, idx.PropertyKeys[0])
	_, err = db.CreateIndex("", "")
	assert.Equal(t, NotAllowed, err)
}

func TestIndexes(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	defer cleanupIndexes(t, db)
	label0 := rndStr(t)
	label1 := rndStr(t)
	prop0 := rndStr(t)
	prop1 := rndStr(t)
	indexes0, err := db.Indexes(label0)
	assert.Equal(t, err, nil)
	assert.Equal(t, 0, len(indexes0))
	_, err = db.CreateIndex(label0, prop0)
	assert.Equal(t, err, nil)
	_, err = db.CreateIndex(label1, prop0)
	assert.Equal(t, err, nil)
	_, err = db.CreateIndex(label1, prop1)
	assert.Equal(t, err, nil)
	indexes1, err := db.Indexes(label1)
	assert.Equal(t, err, nil)
	assert.Equal(t, 2, len(indexes1))
	indexes2, err := db.Indexes("")
	assert.Equal(t, err, nil)
	assert.Equal(t, 3, len(indexes2))
}

func TestDropIndex(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	defer cleanupIndexes(t, db)
	label := rndStr(t)
	prop0 := rndStr(t)
	idx, _ := db.CreateIndex(label, prop0)
	indexes, _ := db.Indexes(label)
	assert.Equal(t, 1, len(indexes))
	err := idx.Drop()
	if err != nil {
		t.Fatal(err)
	}
	indexes, _ = db.Indexes(label)
	assert.Equal(t, 0, len(indexes))
	err = idx.Drop()
	assert.Equal(t, NotFound, err)
}

func cleanupIndexes(t *testing.T, db *Database) {
	indexes, err := allIndexes(db)
	if err != nil {
		t.Fatal(err)
	}
	qs := make([]*CypherQuery, len(indexes))
	for i, idx := range indexes {
		// Cypher doesn't support properties in DROP statements
		l := idx.Label
		p := idx.PropertyKeys[0]
		stmt := fmt.Sprintf("DROP INDEX ON :%s(%s)", l, p)
		cq := CypherQuery{
			Statement: stmt,
		}
		qs[i] = &cq
	}
	// db.Rc.Log = true
	err = db.CypherBatch(qs)
	if err != nil {
		t.Fatal(err)
	}
}

func TestCreateUniqueConstraints(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	defer cleanupUniqueConstraints(t, db)
	label0 := rndStr(t)
	label1 := rndStr(t)
	prop0 := rndStr(t)
	prop1 := rndStr(t)
	value0 := rndStr(t)
	value1 := rndStr(t)
	// Create constraint
	cstr, err := db.CreateUniqueConstraint(label0, prop0)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, label0, cstr.Label)
	assert.Equal(t, "UNIQUENESS", cstr.Type)
	assert.Equal(t, prop0, cstr.PropertyKeys[0])
	// Try to create the same constraint
	_, err = db.CreateUniqueConstraint(label0, prop0)
	assert.Equal(t, NotAllowed, err)
	// Try to violate the constraint
	// Create the first node
	n0, _ := db.CreateNode(Props{prop0: value0})
	n0.AddLabel(label0)
	// Add Label on existing node
	n1, err := db.CreateNode(Props{prop0: value0})
	assert.Equal(t, nil, err)
	err = n1.AddLabel(label0)
	labels, _ := n1.Labels()
	assert.NotNil(t, err)
	assert.Equal(t, 0, len(labels))
	// Create node with label
	stmt := fmt.Sprintf("CREATE (:%s {%s:%s})", label0, prop0, value0)
	cq := CypherQuery{
		Statement: stmt,
	}
	err = db.Cypher(&cq)
	assert.NotNil(t, err)
	// Try to create constraint that violate existing nodes
	n2, _ := db.CreateNode(Props{prop1: value1})
	_ = n2.AddLabel(label1)
	n3, err := db.CreateNode(Props{prop1: value1})
	assert.Equal(t, nil, err)
	err = n3.AddLabel(label1)
	assert.Equal(t, nil, err)
	_, err = db.CreateUniqueConstraint(label1, prop1)
	assert.NotNil(t, err)
	// Empty parameters
	_, err = db.CreateUniqueConstraint("", "")
	assert.Equal(t, NotAllowed, err)
}

func TestUniqueConstraints(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	defer cleanupUniqueConstraints(t, db)
	label0 := rndStr(t)
	label1 := rndStr(t)
	prop0 := rndStr(t)
	prop1 := rndStr(t)
	cstrs0, err := db.UniqueConstraints(label0, "")
	assert.Equal(t, nil, err)
	assert.Equal(t, 0, len(cstrs0))
	_, err = db.CreateUniqueConstraint(label0, prop0)
	assert.Equal(t, nil, err)
	_, err = db.CreateUniqueConstraint(label1, prop0)
	assert.Equal(t, nil, err)
	_, err = db.CreateUniqueConstraint(label1, prop1)
	assert.Equal(t, nil, err)
	cstrs1, err := db.UniqueConstraints(label0, "")
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(cstrs1))
	cstrs2, err := db.UniqueConstraints(label1, "")
	assert.Equal(t, nil, err)
	assert.Equal(t, 2, len(cstrs2))
	cstrs3, err := db.UniqueConstraints(label1, prop1)
	assert.Equal(t, nil, err)
	assert.Equal(t, 1, len(cstrs3))
	assert.Equal(t, prop1, cstrs3[0].PropertyKeys[0])
}

func TestDropUniqueConstraint(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	defer cleanupUniqueConstraints(t, db)
	label := rndStr(t)
	prop0 := rndStr(t)
	cstr0, _ := db.CreateUniqueConstraint(label, prop0)
	cstrs, _ := db.UniqueConstraints(label, "")
	assert.Equal(t, 1, len(cstrs))
	err := cstr0.Drop()
	if err != nil {
		t.Fatal(err)
	}
	cstrs, _ = db.UniqueConstraints(label, "")
	assert.Equal(t, 0, len(cstrs))
	err = cstr0.Drop()
	assert.Equal(t, NotFound, err)
}

func cleanupUniqueConstraints(t *testing.T, db *Database) {
	constraints, err := allConstraints(db)
	if err != nil {
		t.Fatal(err)
	}
	qs := make([]*CypherQuery, len(constraints))
	for i, cstr := range constraints {
		l := cstr.Label
		p := cstr.PropertyKeys[0]
		stmt := fmt.Sprintf("DROP CONSTRAINT ON (x:%s) ASSERT x.%s IS UNIQUE", l, p)
		cq := CypherQuery{
			Statement: stmt,
		}
		qs[i] = &cq
	}
	err = db.CypherBatch(qs)
	if err != nil {
		t.Fatal(err)
	}
}

var (
	indRegex        = regexp.MustCompile(`^ +ON +:(.*)\((.*)\) +ONLINE +$`)
	constraintRegex = regexp.MustCompile(`^ +ON +\(.*\:(.*)\) +ASSERT +.*\.(.*) +IS UNIQUE *`)
	re              = regexp.MustCompile("(.*/db/)data/")
)

func allIndexes(db *Database) ([]*Index, error) {
	schemaLines, err := fetchSchema(db)
	if err != nil {
		return nil, err
	}
	var indexes []*Index
	for _, line := range schemaLines {
		for _, match := range indRegex.FindAllStringSubmatch(line, -1) {
			indexes = append(indexes, &Index{db: db, Label: match[1], PropertyKeys: []string{match[2]}})
		}
	}
	return indexes, nil
}

func allConstraints(db *Database) ([]*UniqueConstraint, error) {
	schemaLines, err := fetchSchema(db)
	if err != nil {
		return nil, err
	}
	var constraints []*UniqueConstraint
	for _, line := range schemaLines {
		for _, match := range constraintRegex.FindAllStringSubmatch(line, -1) {
			constraints = append(constraints, &UniqueConstraint{db: db, Label: match[1], Type: "UNIQUENESS", PropertyKeys: []string{match[2]}})
		}
	}
	return constraints, nil
}

func fetchSchema(db *Database) ([]string, error) {
	url := re.ReplaceAllString(db.Url, "${1}manage/server/console/")

	resp, err := http.Post(url, "application/json", strings.NewReader(`{"command":"schema","engine":"shell"}`))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data []interface{}

	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&data); err != nil {
		return nil, err
	}

	if len(data) != 2 {
		return nil, fmt.Errorf("unexpected index response length %d", len(data))
	}

	return strings.Split(data[0].(string), "\n"), nil
}
