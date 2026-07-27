package queryplan

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// Walk must visit the root before its children, depth first.
func TestNodeWalkVisitsDepthFirst(t *testing.T) {
	t.Parallel()

	root := &Node{
		NodeType: "Hash Join",
		Children: []Node{
			{NodeType: "Seq Scan"},
			{NodeType: "Hash", Children: []Node{{NodeType: "Index Scan"}}},
		},
	}

	var visited []string
	require.True(t, root.Walk(func(n *Node) bool {
		visited = append(visited, n.NodeType)
		return true
	}))
	require.Equal(t, []string{"Hash Join", "Seq Scan", "Hash", "Index Scan"}, visited)
}

// Returning false from fn stops the traversal and propagates false to the caller.
func TestNodeWalkStopsEarly(t *testing.T) {
	t.Parallel()

	root := &Node{
		NodeType: "Hash Join",
		Children: []Node{{NodeType: "Seq Scan"}, {NodeType: "Index Scan"}},
	}

	var visited []string
	require.False(t, root.Walk(func(n *Node) bool {
		visited = append(visited, n.NodeType)
		return n.NodeType != "Seq Scan"
	}))
	require.Equal(t, []string{"Hash Join", "Seq Scan"}, visited)
}

// A nil plan is walkable and reports completion, so callers need no nil guard.
func TestNodeWalkNil(t *testing.T) {
	t.Parallel()

	var root *Node
	called := false
	require.True(t, root.Walk(func(*Node) bool {
		called = true
		return true
	}))
	require.False(t, called)
}
