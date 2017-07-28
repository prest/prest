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

// 18.9.1. Create node index
func TestCreateLegacyNodeIndex(t *testing.T) {
	db := connectTest(t)
	name := rndStr(t)
	//
	// Create new index
	//
	idx0, err := db.CreateLegacyNodeIndex(name, "", "")
	if err != nil {
		t.Error(err)
	}
	defer idx0.Delete()
	assert.Equal(t, idx0.Name, name)
	//
	// Get the index we just created
	//
	idx1, err := db.LegacyNodeIndex(name)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, idx0.Name, idx1.Name)
	assert.Equal(t, idx0.HrefIndex, idx1.HrefIndex)
}

// 18.9.2. Create node index with configuration
func TestLegacyNodeIndexCreateWithConf(t *testing.T) {
	db := connectTest(t)
	name := rndStr(t)
	indexType := "fulltext"
	provider := "lucene"
	//
	// Create new index
	//
	idx0, err := db.CreateLegacyNodeIndex(name, indexType, provider)
	if err != nil {
		t.Error(err)
	}
	defer idx0.Delete()
	assert.Equal(t, idx0.IndexType, indexType)
	assert.Equal(t, idx0.Provider, provider)
	assert.Equal(t, idx0.Name, name)
	//
	// Get the index we just created
	//
	idx1, err := db.LegacyNodeIndex(name)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, idx0.Name, idx1.Name)
}

// 18.9.3. Delete node index
func TestDeleteLegacyNodeIndex(t *testing.T) {
	db := connectTest(t)
	// Include a space in the name to ensure correct URL escaping.
	name := rndStr(t) + " " + rndStr(t)
	idx0, _ := db.CreateLegacyNodeIndex(name, "", "")
	err := idx0.Delete()
	if err != nil {
		t.Error(err)
	}
	_, err = db.LegacyNodeIndex(name)
	assert.Equal(t, err, NotFound)
}

// 18.9.4. List node indexes
func TestListLegacyNodeIndexes(t *testing.T) {
	db := connectTest(t)
	name := rndStr(t)
	idx0, _ := db.CreateLegacyNodeIndex(name, "", "")
	defer idx0.Delete()
	indexes, err := db.LegacyNodeIndexes()
	if err != nil {
		t.Error(err)
	}
	valid := false
	for _, i := range indexes {
		if i.Name == name {
			valid = true
		}
	}
	assert.True(t, valid, "Newly created Index not found in listing of all Indexes.")
}

// 18.9.5. Add node to index
func TestAddNodeToIndex(t *testing.T) {
	db := connectTest(t)
	name := rndStr(t)
	key := rndStr(t)
	value := rndStr(t)
	idx0, _ := db.CreateLegacyNodeIndex(name, "", "")
	defer idx0.Delete()
	n0, _ := db.CreateNode(Props{})
	defer n0.Delete()
	err := idx0.Add(n0, key, value)
	if err != nil {
		t.Error(err)
	}
}

func TestAddNodeToExistingIndex(t *testing.T) {
	db := connectTest(t)
	name := rndStr(t)
	key := rndStr(t)
	value := rndStr(t)
	idx0, _ := db.CreateLegacyNodeIndex(name, "", "")
	defer idx0.Delete()
	idx1, _ := db.LegacyNodeIndex(name)
	n0, _ := db.CreateNode(Props{})
	defer n0.Delete()
	err := idx1.Add(n0, key, value)
	if err != nil {
		t.Fatal(err)
	}
}

// 18.9.6. Remove all entries with a given node from an index
func TestRemoveNodeFromIndex(t *testing.T) {
	db := connectTest(t)
	name := rndStr(t)
	key := rndStr(t)
	value := rndStr(t)
	idx0, _ := db.CreateLegacyNodeIndex(name, "", "")
	defer idx0.Delete()
	n0, _ := db.CreateNode(Props{})
	defer n0.Delete()
	idx0.Add(n0, key, value)
	err := idx0.Remove(n0, "", "")
	if err != nil {
		t.Error(err)
	}
}

// 18.9.7. Remove all entries with a given node and key from an indexj
func TestRemoveNodeAndKeyFromIndex(t *testing.T) {
	db := connectTest(t)
	name := rndStr(t)
	key := rndStr(t)
	value := rndStr(t)
	idx0, _ := db.CreateLegacyNodeIndex(name, "", "")
	defer idx0.Delete()
	n0, _ := db.CreateNode(Props{})
	defer n0.Delete()
	idx0.Add(n0, key, value)
	err := idx0.Remove(n0, key, "")
	if err != nil {
		t.Error(err)
	}
}

// 18.9.8. Remove all entries with a given node, key and value from an index
func TestRemoveNodeKeyAndValueFromIndex(t *testing.T) {
	db := connectTest(t)
	name := rndStr(t)
	key := rndStr(t)
	value := rndStr(t)
	idx0, _ := db.CreateLegacyNodeIndex(name, "", "")
	defer idx0.Delete()
	n0, _ := db.CreateNode(Props{})
	defer n0.Delete()
	idx0.Add(n0, key, value)
	err := idx0.Remove(n0, key, "")
	if err != nil {
		t.Error(err)
	}
}

// 18.9.9. Find node by exact match
func TestFindNodeByExactMatch(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	idxName := rndStr(t)
	key0 := rndStr(t)
	key1 := rndStr(t)
	value0 := rndStr(t)
	value1 := rndStr(t)
	idx0, _ := db.CreateLegacyNodeIndex(idxName, "", "")
	defer idx0.Delete()
	n0, _ := db.CreateNode(Props{})
	n1, _ := db.CreateNode(Props{})
	n2, _ := db.CreateNode(Props{})
	// These two will be located by Find() below
	idx0.Add(n0, key0, value0)
	idx0.Add(n1, key0, value0)
	// These two will NOT be located by Find() below
	idx0.Add(n2, key1, value0)
	idx0.Add(n2, key0, value1)
	//
	nodes, err := idx0.Find(key0, value0)
	if err != nil {
		t.Error(err)
	}
	// This query should have returned a map containing just two nodes, n1 and n0.
	assert.Equal(t, len(nodes), 2)
	_, present := nodes[n0.Id()]
	assert.True(t, present, "Find() failed to return node with id "+strconv.Itoa(n0.Id()))
	_, present = nodes[n1.Id()]
	assert.True(t, present, "Find() failed to return node with id "+strconv.Itoa(n1.Id()))
}

// 18.9.10. Find node by query
func TestFindNodeByQuery(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	idx0, _ := db.CreateLegacyNodeIndex("test index", "", "")
	defer idx0.Delete()
	key0 := rndStr(t)
	key1 := rndStr(t)
	value0 := rndStr(t)
	value1 := rndStr(t)
	n0, _ := db.CreateNode(Props{})
	idx0.Add(n0, key0, value0)
	idx0.Add(n0, key1, value1)
	n1, _ := db.CreateNode(Props{})
	idx0.Add(n1, key0, value0)
	n2, _ := db.CreateNode(Props{})
	idx0.Add(n2, rndStr(t), rndStr(t))
	// Retrieve
	luceneQuery0 := fmt.Sprintf("%v:%v AND %v:%v", key0, value0, key1, value1) // Retrieve n0 only
	luceneQuery1 := fmt.Sprintf("%v:%v", key0, value0)                         // Retrieve n0 and n1
	nodes0, err := idx0.Query(luceneQuery0)
	if err != nil {
		t.Error(err)
	}
	nodes1, err := idx0.Query(luceneQuery1)
	if err != nil {
		t.Error(err)
	}
	// Confirm
	assert.Equal(t, len(nodes0), 1, "Query should have returned only one Node.")
	_, present := nodes0[n0.Id()]
	assert.True(t, present, "Query() failed to return node with id "+strconv.Itoa(n0.Id()))
	assert.Equal(t, len(nodes1), 2, "Query should have returned exactly 2 Nodes.")
	_, present = nodes1[n0.Id()]
	assert.True(t, present, "Query() failed to return node with id "+strconv.Itoa(n0.Id()))
	_, present = nodes1[n1.Id()]
	assert.True(t, present, "Query() failed to return node with id "+strconv.Itoa(n1.Id()))
}
