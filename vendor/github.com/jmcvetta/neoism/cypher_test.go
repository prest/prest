// Copyright (c) 2012-2013 Jason McVetta.  This is Free Software, released under
// the terms of the GPL v3.  See http://www.gnu.org/copyleft/gpl.html for details.
// Resist intellectual serfdom - the ownership of ideas is akin to slavery.

package neoism

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"strconv"
	"testing"
)

// 18.3.1. Send queries with parameters
func TestCypherParameters(t *testing.T) {
	var db *Database
	db = connectTest(t)
	defer cleanup(t, db)
	// defer cleanup(t, db)
	// Create
	nameIdx, _ := db.CreateLegacyNodeIndex("name_index", "", "")
	defer nameIdx.Delete()
	floatIdx, _ := db.CreateLegacyNodeIndex("float_index", "", "")
	defer floatIdx.Delete()
	numIdx, _ := db.CreateLegacyNodeIndex("num_index", "", "")
	defer numIdx.Delete()
	n0, _ := db.CreateNode(Props{"name": "I"})
	nameIdx.Add(n0, "name", "I")
	n1, _ := db.CreateNode(Props{"name": "you"})
	n2, _ := db.CreateNode(Props{"name": "num", "num": 42})
	numIdx.Add(n2, "num", 42)
	n3, _ := db.CreateNode(Props{"name": "float", "float": 3.14})
	floatIdx.Add(n3, "float", 3.14)
	r0, _ := n0.Relate("knows", n1.Id(), nil)
	r1, _ := n0.Relate("loves", n1.Id(), nil)
	n0.Relate("understands", n2.Id(), nil)
	//
	// Query with string parameters and integer results
	//
	type resultStruct0 struct {
		N int `json:"id(n)"`
		R int `json:"id(r)"`
		M int `json:"id(m)"`
	}
	result0 := []resultStruct0{}
	cq := CypherQuery{
		Statement: `
			START n = node:name_index(name={startName})
			MATCH path = (n)-[r]->(m)
			WHERE m.name = {name}
			RETURN id(n), id(r), id(m)
			ORDER by id(n), id(r), id(m)
		`,
		Parameters: map[string]interface{}{
			"startName": "I",
			"name":      "you",
		},
		Result: &result0,
	}
	err := db.Cypher(&cq)
	if err != nil {
		t.Error(err)
	}
	// Check result
	expCol := []string{"id(n)", "id(r)", "id(m)"}
	expDat0 := []resultStruct0{
		resultStruct0{n0.Id(), r0.Id(), n1.Id()},
		resultStruct0{n0.Id(), r1.Id(), n1.Id()},
	}
	assert.Equal(t, expCol, cq.Columns())
	assert.Equal(t, expDat0, result0)
	//
	// Query with integer parameter and string results
	//
	type resultStruct1 struct {
		Name string `json:"n.name"`
	}
	result1 := []resultStruct1{}
	cq = CypherQuery{

		Statement: `
		START n = node:num_index(num={num})
		RETURN n.name
		`,
		Parameters: map[string]interface{}{
			"num": 42,
		},
		Result: &result1,
	}
	err = db.Cypher(&cq)
	if err != nil {
		t.Error(err)
	}
	expCol = []string{"n.name"}
	expDat1 := []resultStruct1{resultStruct1{Name: "num"}}
	assert.Equal(t, expCol, cq.Columns())
	assert.Equal(t, expDat1, result1)
	//
	// Query with float parameter
	//
	result2 := []resultStruct1{}
	cq = CypherQuery{
		Statement: `
		START n = node:float_index(float={float})
		RETURN n.name
		`,
		Parameters: map[string]interface{}{
			"float": 3.14,
		},
		Result: &result2,
	}
	err = db.Cypher(&cq)
	if err != nil {
		t.Error(err)
	}
	expCol = []string{"n.name"}
	expDat2 := []resultStruct1{resultStruct1{Name: "float"}}
	assert.Equal(t, expCol, cq.Columns())
	assert.Equal(t, expDat2, result2)
	//
	// Query with array parameter
	//
	result3 := []resultStruct1{}
	cq = CypherQuery{
		Statement: `
			START n=node(*)
			WHERE id(n) IN {arr}
			RETURN n.name
			ORDER BY id(n)
			`,
		Parameters: map[string]interface{}{
			"arr": []int{n0.Id(), n1.Id()},
		},
		Result: &result3,
	}
	err = db.Cypher(&cq)
	if err != nil {
		t.Error(err)
	}
	expCol = []string{"n.name"}
	expDat3 := []resultStruct1{
		resultStruct1{Name: "I"},
		resultStruct1{Name: "you"},
	}
	assert.Equal(t, expCol, cq.Columns())
	assert.Equal(t, expDat3, result3)
}

// 18.3.2. Send a Query
func TestCypher(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	idx0, _ := db.CreateLegacyNodeIndex("name_index", "", "")
	defer idx0.Delete()
	n0, _ := db.CreateNode(Props{"name": "I"})
	idx0.Add(n0, "name", "I")
	n1, _ := db.CreateNode(Props{"name": "you", "age": 69})
	n0.Relate("know", n1.Id(), nil)
	// Query
	// query := "START x = node:name_index(name=I) MATCH path = (x-[r]-friend) WHERE friend.name = you RETURN TYPE(r)"
	type resultStruct struct {
		Type string `json:"type(r)"`
		Name string `json:"n.name"`
		Age  int    `json:"n.age"`
	}
	result := []resultStruct{}
	cq := CypherQuery{
		Statement: "start x = node(" + strconv.Itoa(n0.Id()) + ") match x -[r]-> n return type(r), n.name, n.age",
		Result:    &result,
	}
	err := db.Cypher(&cq)
	if err != nil {
		t.Error(err)
	}
	// Check result
	//
	// Our test only passes if Neo4j returns columns in the expected order - is
	// there any guarantee about order?
	expCol := []string{"type(r)", "n.name", "n.age"}
	expDat := []resultStruct{
		resultStruct{
			Type: "know",
			Name: "you",
			Age:  69,
		},
	}
	assert.Equal(t, expCol, cq.Columns())
	assert.Equal(t, expDat, result)
}

// Test multi-line Cypher query with embedded comments.
func TestCypherComment(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	idx0, _ := db.CreateLegacyNodeIndex("name_index", "", "")
	defer idx0.Delete()
	n0, _ := db.CreateNode(Props{"name": "I"})
	idx0.Add(n0, "name", "I")
	n1, _ := db.CreateNode(Props{"name": "you", "age": 69})
	n0.Relate("know", n1.Id(), nil)
	// Query
	// query := "START x = node:name_index(name=I) MATCH path = (x-[r]-friend) WHERE friend.name = you RETURN TYPE(r)"
	type resultStruct struct {
		Type string `json:"type(r)"`
		Name string `json:"n.name"`
		Age  int    `json:"n.age"`
	}
	result := []resultStruct{}
	stmt := `
		START x = NODE(%d)
		// This is a comment
		MATCH x -[r]-> n
		// This is another comment
		RETURN TYPE(r), n.name, n.age
		`
	stmt = fmt.Sprintf(stmt, n0.Id())
	cq := CypherQuery{
		Statement: stmt,
		Result:    &result,
	}
	err := db.Cypher(&cq)
	if err != nil {
		t.Error(err)
	}
	// Check result
	//
	// Our test only passes if Neo4j returns columns in the expected order - is
	// there any guarantee about order?
	expCol := []string{"TYPE(r)", "n.name", "n.age"}
	expDat := []resultStruct{
		resultStruct{
			Type: "know",
			Name: "you",
			Age:  69,
		},
	}
	assert.Equal(t, expCol, cq.Columns())
	assert.Equal(t, expDat, result)
}
func TestCypherBadQuery(t *testing.T) {
	db := connectTest(t)
	cq := CypherQuery{
		Statement: "foobar",
	}
	err := db.Cypher(&cq)
	ne, ok := err.(NeoError)
	if !ok {
		t.Error(err)
	}
	s := ne.Error()
	assert.NotEqual(t, "", s)
}

func TestCypherStats(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	cq := CypherQuery{
		Statement: `
			CREATE (n:Person)
		`,
		IncludeStats: true,
	}
	err := db.Cypher(&cq)
	if err != nil {
		t.Error(err)
	}

	stats, err := cq.Stats()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, Stats{ContainsUpdates: true, LabelsAdded: 1, NodesCreated: 1}, *stats)
}

func TestCypherNoStats(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	cq := CypherQuery{
		Statement: `
			CREATE (n:Person)
		`,
		IncludeStats: false,
	}
	err := db.Cypher(&cq)
	if err != nil {
		t.Error(err)
	}

	_, err = cq.Stats()
	if err == nil {
		t.Fatal("Stats not requested - expected an error")
	}
}

func TestCypherBatch(t *testing.T) {
	db := connectTest(t)
	type resultStruct0 struct {
		N Node
	}
	type resultStruct2 struct {
		R Relationship
	}
	r0 := []resultStruct0{}
	r1 := []resultStruct0{}
	r2 := []resultStruct2{}
	// n0 := []interface{}{}
	qs := []*CypherQuery{
		&CypherQuery{
			Statement: `CREATE (n:Person {name: "Mr Spock"}) RETURN n`,
			Result:    &r0,
		},
		&CypherQuery{
			Statement: `CREATE (n:Person {name: "Mr Sulu"}) RETURN n`,
			Result:    &r1,
		},
		&CypherQuery{
			Statement: `
				MATCH (a:Person), (b:Person)
				WHERE a.name = 'Mr Spock' AND b.name = 'Mr Sulu'
				CREATE a-[r:Knows]->b
				RETURN r
			`,
			Result: &r2,
		},
	}
	err := db.CypherBatch(qs)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "Mr Spock", r0[0].N.Data["name"])
	assert.Equal(t, "Mr Sulu", r1[0].N.Data["name"])
	assert.Equal(t, "Knows", r2[0].R.Type)
	//
	// Cleanup
	//
	for _, r := range r2 {
		r.R.Db = db
		err = r.R.Delete()
		if err != nil {
			t.Error(err)
		}
	}
	for _, n := range r0 {
		n.N.Db = db
		err = n.N.Delete()
		if err != nil {
			t.Error(err)
		}
	}
	for _, n := range r1 {
		n.N.Db = db
		err = n.N.Delete()
		if err != nil {
			t.Error(err)
		}
	}
}

func TestCypherBadBatch(t *testing.T) {
	db := connectTest(t)
	cq := CypherQuery{
		Statement: "foobar",
	}
	qs := []*CypherQuery{&cq}
	err := db.CypherBatch(qs)
	if _, ok := err.(NeoError); !ok {
		t.Error(err)
	}
}

func TestCypherBatchStats(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	qs := []*CypherQuery{
		&CypherQuery{
			Statement:    `CREATE (n:Person)`,
			IncludeStats: true,
		},
	}
	err := db.CypherBatch(qs)
	if err != nil {
		t.Error(err)
	}

	stats, err := qs[0].Stats()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, Stats{ContainsUpdates: true, LabelsAdded: 1, NodesCreated: 1}, *stats)
}
