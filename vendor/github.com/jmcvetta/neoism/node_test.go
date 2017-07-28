// Copyright (c) 2012-2013 Jason McVetta.  This is Free Software, released under
// the terms of the GPL v3.  See http://www.gnu.org/copyleft/gpl.html for details.
// Resist intellectual serfdom - the ownership of ideas is akin to slavery.

//
// The Neo4j Manual section numbers quoted herein refer to the manual for
// milestone release 1.8.  http://docs.neo4j.org/chunked/1.8/

package neoism

import (
	"github.com/jmcvetta/randutil"
	"github.com/stretchr/testify/assert"
	"testing"
)

// 18.4.1. Create Node
func TestCreateNode(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	n0, err := db.CreateNode(nil)
	if err != nil {
		t.Error(err)
	}
	// Confirm creation
	_, err = db.Node(n0.Id())
	if err != nil {
		t.Error(err)
	}
	//
	// Bad Href
	//
	db1 := connectTest(t)
	db1.HrefNode = db1.HrefNode + "foobar"
	_, err = db1.CreateNode(nil)
	assert.Equal(t, NotFound, err)
}

// 18.4.2. Create Node with properties
func TestCreateNodeWithProperties(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	props0 := Props{}
	props0["foo"] = "bar"
	props0["spam"] = "eggs"
	n0, err := db.CreateNode(props0)
	if err != nil {
		t.Error(err)
	}
	// Confirm creation
	_, err = db.Node(n0.Id())
	if err != nil {
		t.Error(err)
	}
	// Confirm properties
	props1, _ := n0.Properties()
	assert.Equal(t, props0, props1, "Node properties not as expected")
}

// 18.4.2. Create Node with properties
func TestGetOrCreateNode(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	label, err := randutil.String(12, randutil.Alphabet)
	if err != nil {
		t.Fatal(err)
	}
	key, err := randutil.String(12, randutil.Alphabet)
	if err != nil {
		t.Fatal(err)
	}
	value, err := randutil.String(12, randutil.Alphabet)
	if err != nil {
		t.Fatal(err)
	}
	p0 := Props{key: value, "foo": "bar"}
	p1 := Props{key: value}
	p2 := Props{"foo": "bar"}
	//
	// Create unique node
	//
	n0, created, err := db.GetOrCreateNode(label, key, p0)
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("Failed to create unique node")
	}
	check0, err := n0.Properties()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, p0, check0)
	//
	// Get unique node
	//
	n1, created, err := db.GetOrCreateNode(label, key, p1)
	if err != nil {
		t.Fatal(err)
	}
	if created {
		t.Fatal("Failed to retrieve unique node")
	}
	check1, err := n1.Properties()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, p0, check1)
	//
	// No key in props
	//
	_, _, err = db.GetOrCreateNode(label, key, p2)
	assert.NotEqual(t, nil, err)
	//
	// Empty label
	//
	_, _, err = db.GetOrCreateNode("", key, p0)
	assert.NotEqual(t, nil, err)
}

// 18.4.3. Get node
func TestGetNode(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	n0, _ := db.CreateNode(Props{})
	// Get Node
	n1, err := db.Node(n0.Id())
	if err != nil {
		t.Error(err)
	}
	// Confirm nodes are the same
	assert.Equal(t, n0.Id(), n1.Id(), "Nodes do not have same ID")
}

// 18.4.4. Get non-existent node
func TestGetNonexistentNode(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create a node
	n0, _ := db.CreateNode(Props{})
	// Try to get non-existent node with next Id
	implausible := n0.Id() + 1000
	_, err := db.Node(implausible)
	assert.Equal(t, err, NotFound)
}

// 18.4.5. Delete node
func TestDeleteNode(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create then delete a node
	n0, _ := db.CreateNode(Props{})
	id := n0.Id()
	err := n0.Delete()
	if err != nil {
		t.Error(err)
	}
	// Check that node is no longer in db
	_, err = db.Node(id)
	assert.Equal(t, err, NotFound)
	//
	// Delete non-existent node
	//
	err = n0.Delete()
	assert.Equal(t, NotFound, err)

}

// 18.4.6. Nodes with relationships can not be deleted;
func TestDeleteNodeWithRelationships(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	n0, _ := db.CreateNode(Props{})
	n1, _ := db.CreateNode(Props{})
	n0.Relate("knows", n1.Id(), Props{})
	// Attempt to delete node without deleting relationship
	err := n0.Delete()
	assert.Equal(t, err, CannotDelete, "Should not be possible to delete node with relationship.")
}

// 18.7.1. Set property on node
func TestSetPropertyOnNode(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	n0, _ := db.CreateNode(Props{})
	key := rndStr(t)
	value := rndStr(t)
	err := n0.SetProperty(key, value)
	if err != nil {
		t.Error(err)
	}
	// Confirm
	props, _ := n0.Properties()
	checkVal, present := props[key]
	assert.True(t, present, "Expected property key not found")
	assert.True(t, checkVal == value, "Expected property value not found")
}

// 18.7.1. Set property on node
func TestSetBadPropertyOnNode(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	n0, _ := db.CreateNode(Props{})
	key := ""
	value := rndStr(t)
	err := n0.SetProperty(key, value)
	if _, ok := err.(NeoError); !ok {
		t.Fatal(err)
	}
}

// 18.7.2. Update node properties
func TestUpdatePropertyOnNode(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	props0 := Props{rndStr(t): rndStr(t)}
	props1 := Props{rndStr(t): rndStr(t)}
	n0, _ := db.CreateNode(props0)
	// Update
	err := n0.SetProperties(props1)
	if err != nil {
		t.Error(err)
	}
	// Confirm
	checkProps, _ := n0.Properties()
	assert.Equal(t, props1, checkProps, "Did not recover expected properties after updating with SetProperties().")
}

// 18.7.3. Get properties for node
func TestGetPropertiesForNode(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	props := Props{rndStr(t): rndStr(t)}
	n0, _ := db.CreateNode(props)
	// Get properties & confirm
	checkProps, err := n0.Properties()
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, props, checkProps, "Did not return expected properties.")
}

//
// 18.7.4. Property values can not be null
//
// This section cannot be tested.  Properties - which is a map[string]string -
// cannot be instantiated with a nil value.  If you try, the code will not compile.
//

//
// 18.7.5. Property values can not be nested
//
// This section cannot be tested.  Properties is defined as map[string]string -
// only strings may be used as values.  If you try to create a nested
// Properties, the code will not compile.
//

// 18.7.6. Delete all properties from node
func TestDeleteAllPropertiesFromNode(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	props := Props{
		rndStr(t): rndStr(t),
		rndStr(t): rndStr(t),
	}
	n0, _ := db.CreateNode(props)
	// Delete properties
	err := n0.DeleteProperties()
	if err != nil {
		t.Error(err)
	}
	// Confirm deletion
	checkProps, _ := n0.Properties()
	assert.Equal(t, Props{}, checkProps, "Properties should be empty after call to DeleteProperties()")
	n0.Delete()
	err = n0.DeleteProperties()
	assert.Equal(t, NotFound, err)
}

// 18.7.7. Delete a named property from a node
func TestDeleteNamedPropertyFromNode(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	props0 := Props{"foo": "bar"}
	props1 := Props{"foo": "bar", "spam": "eggs"}
	n0, _ := db.CreateNode(props1)
	// Delete
	err := n0.DeleteProperty("spam")
	if err != nil {
		t.Error(err)
	}
	// Confirm
	checkProps, _ := n0.Properties()
	assert.Equal(t, props0, checkProps, "Failed to remove named property with DeleteProperty().")
	//
	// Delete non-existent property
	//
	err = n0.DeleteProperty("eggs")
	assert.NotEqual(t, nil, err)
	//
	// Delete and check 404
	//
	n0.Delete()
	err = n0.DeleteProperty("spam")
	assert.Equal(t, NotFound, err)
}

func TestNodeProperty(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	props := Props{"foo": "bar"}
	n0, _ := db.CreateNode(props)
	value, err := n0.Property("foo")
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, value, "bar", "Incorrect value when getting single property.")
	//
	// Check Not Found
	//
	n0.Delete()
	_, err = n0.Property("foo")
	assert.Equal(t, NotFound, err)
}

func TestAddLabels(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	n0, _ := db.CreateNode(nil)
	labels, err := n0.Labels()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, []string{}, labels)
	newLabels := []string{"Person", "Bicyclist"}
	err = n0.AddLabel(newLabels...)
	if err != nil {
		t.Fatal(err)
	}
	labels, err = n0.Labels()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, newLabels, labels)
}

func TestLabelsInvalidNode(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	n0, _ := db.CreateNode(nil)
	n0.Delete()
	err := n0.AddLabel("foobar")
	assert.Equal(t, NotFound, err)
	_, err = n0.Labels()
	assert.Equal(t, NotFound, err)
}

func TestRemoveLabel(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	n0, _ := db.CreateNode(nil)
	n0.AddLabel("foobar")
	labels, _ := n0.Labels()
	assert.Equal(t, []string{"foobar"}, labels)
	err := n0.RemoveLabel("foobar")
	if err != nil {
		t.Fatal(err)
	}
	labels, _ = n0.Labels()
	assert.Equal(t, []string{}, labels)
	n0.Delete()
	err = n0.RemoveLabel("foobar")
	assert.Equal(t, NotFound, err)

}

func TestAddLabelInvalidName(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	n0, _ := db.CreateNode(nil)
	err := n0.AddLabel("") // Blank string is invalid label name
	if _, ok := err.(NeoError); !ok {
		t.Fatal(err)
	}
}

func TestSetLabels(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	n0, _ := db.CreateNode(nil)
	n0.AddLabel("spam", "eggs")
	err := n0.SetLabels([]string{"foobar"})
	if err != nil {
		t.Fatal(err)
	}
	labels, _ := n0.Labels()
	assert.Equal(t, []string{"foobar"}, labels)
	n0.Delete()
	err = n0.SetLabels([]string{"foobar"})
	assert.Equal(t, NotFound, err)
}

func TestNodesByLabel(t *testing.T) {
	db := connectTest(t)
	cleanup(t, db) // Make sure no nodes exist before we start
	defer cleanup(t, db)
	nodes, err := db.NodesByLabel("foobar")
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 0, len(nodes))
	n0, _ := db.CreateNode(nil)
	n0.AddLabel("foobar")
	nodes, err = db.NodesByLabel("foobar")
	if err != nil {
		t.Fatal(err)
	}
	exp := []*Node{n0}
	assert.Equal(t, exp, nodes)
}

func TestGetAllLabels(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	rndLabel := rndStr(t)
	n0, _ := db.CreateNode(nil)
	n0.AddLabel(rndLabel)
	labels, err := db.Labels()
	if err != nil {
		t.Fatal(err)
	}
	m := make(map[string]bool, len(labels))
	for _, l := range labels {
		m[l] = true
	}
	if _, ok := m[rndLabel]; !ok {
		t.Fatal("Label not returned: " + rndLabel)
	}

}
