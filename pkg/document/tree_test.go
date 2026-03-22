package document

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIndexTree_FindNodeByID(t *testing.T) {
	// Build a test tree
	root := NewNode("Root", 1, 20)
	child1 := NewNode("Chapter 1", 1, 5)
	child2 := NewNode("Chapter 2", 6, 15)
	child21 := NewNode("Section 2.1", 6, 10)
	child22 := NewNode("Section 2.2", 11, 15)
	child3 := NewNode("Chapter 3", 16, 20)

	child2.AddChild(child21)
	child2.AddChild(child22)
	root.AddChild(child1)
	root.AddChild(child2)
	root.AddChild(child3)

	tree := NewIndexTree(root, 20)

	// Test finding existing nodes
	t.Run("find existing nodes", func(t *testing.T) {
		node := tree.FindNodeByID(child1.ID)
		assert.Equal(t, child1, node)

		node = tree.FindNodeByID(child21.ID)
		assert.Equal(t, child21, node)

		node = tree.FindNodeByID(root.ID)
		assert.Equal(t, root, node)
	})

	// Test finding non-existent node
	t.Run("find non-existent node", func(t *testing.T) {
		node := tree.FindNodeByID("non-existent-id")
		assert.Nil(t, node)
	})

	// Test empty string ID
	t.Run("find empty ID", func(t *testing.T) {
		node := tree.FindNodeByID("")
		assert.Nil(t, node)
	})
}

func TestIndexTree_BuildNodeMap(t *testing.T) {
	// Build initial tree
	root := NewNode("Root", 1, 10)
	child1 := NewNode("Child 1", 1, 5)
	root.AddChild(child1)

	tree := NewIndexTree(root, 10)

	// Verify initial nodes are in map
	assert.Equal(t, child1, tree.FindNodeByID(child1.ID))
	assert.Equal(t, root, tree.FindNodeByID(root.ID))

	// Add a new child after initial build
	child2 := NewNode("Child 2", 6, 10)
	root.AddChild(child2)

	// New node should not be in map yet
	assert.Nil(t, tree.FindNodeByID(child2.ID))

	// Rebuild node map
	tree.BuildNodeMap()

	// Now it should be found
	assert.Equal(t, child2, tree.FindNodeByID(child2.ID))
}

func TestIndexTree_NodeMap_AfterLoad(t *testing.T) {
	// This simulates what happens when loading a tree from JSON
	// When unmarshaling, the nodeMap is nil, so we need to rebuild it

	root := NewNode("Root", 1, 10)
	child1 := NewNode("Child 1", 1, 5)
	child2 := NewNode("Child 2", 6, 10)
	root.AddChild(child1)
	root.AddChild(child2)

	// Create a tree "manually" like JSON unmarshaling would
	tree := &IndexTree{
		Root:       root,
		TotalPages: 10,
		// nodeMap is nil here, as it would be after JSON unmarshal
	}

	// Before building map, FindNodeByID should panic or return nil?
	// It should return nil because nodeMap is nil
	node := tree.FindNodeByID(child1.ID)
	assert.Nil(t, node)

	// Build the map
	tree.BuildNodeMap()

	// Now nodes should be found
	node = tree.FindNodeByID(child1.ID)
	assert.Equal(t, child1, node)

	node = tree.FindNodeByID(child2.ID)
	assert.Equal(t, child2, node)
}
