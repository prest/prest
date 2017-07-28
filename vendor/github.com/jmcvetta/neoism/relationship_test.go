// Copyright (c) 2012-2013 Jason McVetta.  This is Free Software, released under
// the terms of the GPL v3.  See http://www.gnu.org/copyleft/gpl.html for details.
// Resist intellectual serfdom - the ownership of ideas is akin to slavery.

package neoism

import (
	"github.com/stretchr/testify/assert"
	"sort"
	"testing"
)

// 18.5.1. Get Relationship by ID
func TestGetRelationshipById(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	n0, _ := db.CreateNode(Props{})
	n1, _ := db.CreateNode(Props{})
	r0, _ := n0.Relate("knows", n1.Id(), Props{})
	// Get relationship
	r1, err := db.Relationship(r0.Id())
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, r0.Id(), r1.Id())
}

// 18.5.2. Create relationship
func TestCreateRelationship(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	n0, _ := db.CreateNode(Props{})
	n1, _ := db.CreateNode(Props{})
	r0, err := n0.Relate("knows", n1.Id(), Props{})
	if err != nil {
		t.Error(err)
	}
	// Confirm relationship exists on both nodes
	rels, _ := n0.Outgoing("knows")
	_, present := rels.Map()[r0.Id()]
	assert.True(t, present, "Outgoing relationship not present on origin node.")
	rels, _ = n1.Incoming("knows")
	_, present = rels.Map()[r0.Id()]
	assert.True(t, present, "Incoming relationship not present on destination node.")
}

// 18.5.3. Create a relationship with properties
func TestCreateRelationshipWithProperties(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	props0 := Props{"foo": "bar", "spam": "eggs"}
	n0, _ := db.CreateNode(Props{})
	n1, _ := db.CreateNode(Props{})
	r0, err := n0.Relate("knows", n1.Id(), props0)
	if err != nil {
		t.Error(err)
	}
	// Confirm relationship was created with specified properties.
	props1, _ := r0.Properties()
	assert.Equal(t, props0, props1, "Properties queried from relationship do not match properties it was created with.")
}

// 18.5.4. Delete relationship
func TestDeleteRelationship(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	n0, _ := db.CreateNode(Props{})
	n1, _ := db.CreateNode(Props{})
	r0, err := n0.Relate("knows", n1.Id(), Props{})
	if err != nil {
		t.Error(err)
	}
	// Delete and confirm
	r0.Delete()
	_, err = db.Relationship(r0.Id())
	assert.Equal(t, err, NotFound, "Should not be able to Get() a deleted relationship.")
}

// 18.5.5. Get all properties on a relationship
func TestGetAllPropertiesOnRelationship(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	props0 := Props{"foo": "bar", "spam": "eggs"}
	n0, _ := db.CreateNode(Props{})
	n1, _ := db.CreateNode(Props{})
	r0, _ := n0.Relate("knows", n1.Id(), props0)
	// Confirm relationship was created with specified properties.  No need to
	// check success of creation itself, as that is handled by TestCreateRelationship().
	props1, err := r0.Properties()
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, props0, props1, "Properties queried from relationship do not match properties it was created with.")
}

// 18.5.6. Set all properties on a relationship
func TestSetAllPropertiesOnRelationship(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	props0 := Props{"foo": "bar"}
	props1 := Props{"spam": "eggs"}
	// Create
	n0, _ := db.CreateNode(Props{})
	n1, _ := db.CreateNode(Props{})
	r0, _ := n0.Relate("knows", n1.Id(), props0)
	// Set all properties
	r0.SetProperties(props1)
	// Confirm
	checkProps, _ := r0.Properties()
	assert.Equal(t, checkProps, props1, "Failed to set all properties on relationship")
}

// 18.5.7. Get single property on a relationship
func TestGetSinglePropertyOnRelationship(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	props := Props{"foo": "bar"}
	n0, _ := db.CreateNode(Props{})
	n1, _ := db.CreateNode(Props{})
	r0, _ := n0.Relate("knows", n1.Id(), props)
	// Get property
	value, err := r0.Property("foo")
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, value, "bar", "Incorrect value when getting single property.")
}

// 18.5.8. Set single property on a relationship
func TestSetSinglePropertyOnRelationship(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	n0, _ := db.CreateNode(Props{})
	n1, _ := db.CreateNode(Props{})
	r0, _ := n0.Relate("knows", n1.Id(), Props{})
	// Set property
	r0.SetProperty("foo", "bar")
	// Confirm
	expected := Props{"foo": "bar"}
	props, _ := r0.Properties()
	assert.Equal(t, props, expected, "Failed to set single property on relationship.")
}

// 18.5.9. Get all relationships
func TestGetAllRelationships(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	n0, _ := db.CreateNode(Props{})
	n1, _ := db.CreateNode(Props{})
	r0, _ := n0.Relate("knows", n1.Id(), Props{})
	r1, _ := n1.Relate("knows", n0.Id(), Props{})
	r2, _ := n0.Relate("knows", n1.Id(), Props{})
	// Check relationships
	rels, err := n0.Relationships()
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, len(rels), 3, "Wrong number of relationships")
	for _, r := range []*Relationship{r0, r1, r2} {
		_, present := rels.Map()[r.Id()]
		assert.True(t, present, "Missing expected relationship")
	}
}

// 18.5.10. Get incoming relationships
func TestGetIncomingRelationships(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	n0, _ := db.CreateNode(Props{})
	n1, _ := db.CreateNode(Props{})
	n0.Relate("knows", n1.Id(), Props{})
	r1, _ := n1.Relate("knows", n0.Id(), Props{})
	n0.Relate("knows", n1.Id(), Props{})
	// Check relationships
	rels, err := n0.Incoming()
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, len(rels), 1, "Wrong number of relationships")
	_, present := rels.Map()[r1.Id()]
	assert.True(t, present, "Missing expected relationship")
}

// 18.5.11. Get outgoing relationships
func TestGetOutgoingRelationships(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	n0, _ := db.CreateNode(Props{})
	n1, _ := db.CreateNode(Props{})
	r0, _ := n0.Relate("knows", n1.Id(), Props{})
	n1.Relate("knows", n0.Id(), Props{})
	r2, _ := n0.Relate("knows", n1.Id(), Props{})
	// Check relationships
	rels, err := n0.Outgoing()
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, len(rels), 2, "Wrong number of relationships")
	for _, r := range []*Relationship{r0, r2} {
		_, present := rels.Map()[r.Id()]
		assert.True(t, present, "Missing expected relationship")
	}
}

// 18.5.12. Get typed relationships
func TestGetTypedRelationships(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	relType0 := rndStr(t)
	relType1 := rndStr(t)
	n0, _ := db.CreateNode(Props{})
	n1, _ := db.CreateNode(Props{})
	r0, _ := n0.Relate(relType0, n1.Id(), Props{})
	r1, _ := n0.Relate(relType1, n1.Id(), Props{})
	// Check one type of relationship
	rels, err := n0.Relationships(relType0)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, len(rels), 1, "Wrong number of relationships")
	_, present := rels.Map()[r0.Id()]
	assert.True(t, present, "Missing expected relationship")
	// Check two types of relationship together
	rels, err = n0.Relationships(relType0, relType1)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, len(rels), 2, "Wrong number of relationships")
	for _, r := range []*Relationship{r0, r1} {
		_, present := rels.Map()[r.Id()]
		assert.True(t, present, "Missing expected relationship")
	}
}

// 18.5.13. Get relationships on a node without relationships
func TestGetRelationshipsOnNodeWithoutRelationships(t *testing.T) {
	db := connectTest(t)
	n0, _ := db.CreateNode(Props{})
	defer n0.Delete()
	rels, err := n0.Relationships()
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, len(rels), 0, "Node with no relationships should return empty slice of relationships")
}

// 18.6.1. Get relationship types
func TestGetRelationshipTypes(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	relTypes := []string{}
	for i := 0; i < 10; i++ {
		relTypes = append(relTypes, rndStr(t))
	}
	// Create relationships
	n0, _ := db.CreateNode(Props{})
	n1, _ := db.CreateNode(Props{})
	rels := []*Relationship{}
	for _, rt := range relTypes {
		aRel, _ := n0.Relate(rt, n1.Id(), Props{})
		rels = append(rels, aRel)
	}
	// Get all relationship types, and confirm the list of types contains at least
	// all those randomly-generated values in relTypes.  It cannot be guaranteed
	// that the database will not contain other relationship types beyond these.
	foundRelTypes, err := db.RelTypes()
	if err != nil {
		t.Error(err)
	}
	for _, rt := range relTypes {
		assert.True(t, sort.SearchStrings(foundRelTypes, rt) < len(foundRelTypes),
			"Could not find expected relationship type: "+rt)
	}
}

func TestRelationshipStartEnd(t *testing.T) {
	db := connectTest(t)
	defer cleanup(t, db)
	// Create
	start, _ := db.CreateNode(Props{})
	end, _ := db.CreateNode(Props{})
	r0, _ := start.Relate("knows", end.Id(), Props{})
	//
	n, err := r0.Start()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, start, n)
	n, err = r0.End()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, end, n)
}
