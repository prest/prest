// Copyright (c) 2012-2013 Jason McVetta.  This is Free Software, released under
// the terms of the GPL v3.  See http://www.gnu.org/copyleft/gpl.html for details.
// Resist intellectual serfdom - the ownership of ideas is akin to slavery.

package neoism

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

// The underlying functions are used for node and relationship indexes.  For
// now we will test only the pieces of code that are relationship-specific.
func TestRelationshipIndexes(t *testing.T) {
	db := connectTest(t)
	name := rndStr(t)
	//
	// Create new index
	//
	idx0, err := db.CreateLegacyRelIndex(name, "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer idx0.Delete()
	assert.Equal(t, idx0.Name, name)
	//
	// Get the index we just created
	//
	idx1, err := db.LegacyRelIndex(name)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, idx0.Name, idx1.Name)
	//
	// See if we get this index, and only this index
	//
	indexes, err := db.LegacyRelIndexes()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 1, len(indexes))
	idx2 := indexes[0]
	assert.Equal(t, idx0.Name, idx2.Name)
}

func TestBadCreateLegacyRelIndex(t *testing.T) {
	db := connectTest(t)
	_, err := db.CreateLegacyRelIndex("", "", "")
	assert.NotEqual(t, nil, err)
}
