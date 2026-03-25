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

func TestIndexTree_FindOverlappingNodes(t *testing.T) {
	root := NewNode("Root", 1, 20)
	child1 := NewNode("Chapter 1", 1, 5)
	child2 := NewNode("Chapter 2", 6, 15)
	child3 := NewNode("Chapter 3", 16, 20)
	root.AddChild(child1)
	root.AddChild(child2)
	root.AddChild(child3)

	tree := NewIndexTree(root, 20)

	t.Run("find overlapping with middle chapter", func(t *testing.T) {
		nodes := tree.FindOverlappingNodes(6, 10)
		assert.Len(t, nodes, 2) // Root and Chapter 2
		assert.Contains(t, nodes, root)
		assert.Contains(t, nodes, child2)
	})

	t.Run("find overlapping spanning multiple chapters", func(t *testing.T) {
		nodes := tree.FindOverlappingNodes(3, 12)
		assert.Len(t, nodes, 3) // Root, Chapter 1 (ends at 5, overlaps), Chapter 2
		assert.Contains(t, nodes, root)
		assert.Contains(t, nodes, child1)
		assert.Contains(t, nodes, child2)
	})

	t.Run("find overlapping with single page range", func(t *testing.T) {
		nodes := tree.FindOverlappingNodes(6, 6)
		assert.Len(t, nodes, 2) // Root and Chapter 2
	})

	t.Run("find overlapping with no match", func(t *testing.T) {
		nodes := tree.FindOverlappingNodes(25, 30)
		assert.Len(t, nodes, 0)
	})

	t.Run("find overlapping entire tree", func(t *testing.T) {
		nodes := tree.FindOverlappingNodes(1, 20)
		assert.Len(t, nodes, 4) // All nodes
	})

	t.Run("nil root", func(t *testing.T) {
		emptyTree := NewIndexTree(nil, 0)
		nodes := emptyTree.FindOverlappingNodes(1, 10)
		assert.Len(t, nodes, 0)
	})
}

func TestIndexTree_Merge(t *testing.T) {
	t.Run("merge with nil other", func(t *testing.T) {
		tree1 := NewIndexTree(NewNode("Root", 1, 10), 10)
		result := tree1.Merge(nil)
		assert.Equal(t, tree1, result)
		assert.Equal(t, 10, tree1.TotalPages)
	})

	t.Run("merge with nil root", func(t *testing.T) {
		tree1 := NewIndexTree(NewNode("Root", 1, 10), 10)
		tree2 := &IndexTree{Root: nil, TotalPages: 5}
		result := tree1.Merge(tree2)
		assert.Equal(t, tree1, result)
	})

	t.Run("merge two trees with roots", func(t *testing.T) {
		root1 := NewNode("Doc 1", 1, 10)
		tree1 := NewIndexTree(root1, 10)
		tree1.DocumentInfo = "Document 1"

		root2 := NewNode("Doc 2", 1, 5)
		tree2 := NewIndexTree(root2, 5)
		tree2.DocumentInfo = "Document 2"

		result := tree1.Merge(tree2)

		assert.Equal(t, 15, result.TotalPages)
		assert.Equal(t, 1, result.Version)
		assert.Contains(t, result.DocumentInfo, "Document 1")
		assert.Contains(t, result.DocumentInfo, "Document 2")
		assert.NotNil(t, result.Root)
		assert.Len(t, result.Root.Children, 2)

		assert.Equal(t, 5, tree2.TotalPages)
		assert.Equal(t, 0, tree2.Version)
	})

	t.Run("merge into empty tree", func(t *testing.T) {
		tree1 := NewIndexTree(nil, 0)
		root2 := NewNode("Doc 2", 1, 5)
		tree2 := NewIndexTree(root2, 5)

		result := tree1.Merge(tree2)

		assert.Equal(t, 5, result.TotalPages)
		assert.Equal(t, root2, result.Root)
	})

	t.Run("page offset applied correctly", func(t *testing.T) {
		root1 := NewNode("Doc 1", 1, 10)
		tree1 := NewIndexTree(root1, 10)

		child2 := NewNode("Section", 1, 3)
		root2 := NewNode("Doc 2", 1, 3)
		root2.AddChild(child2)
		tree2 := NewIndexTree(root2, 3)

		result := tree1.Merge(tree2)

		combinedRoot := result.Root
		doc1Child := combinedRoot.Children[0]
		doc2Child := combinedRoot.Children[1]

		assert.Equal(t, 1, doc1Child.StartPage)
		assert.Equal(t, 10, doc1Child.EndPage)
		assert.Equal(t, 11, doc2Child.StartPage)
		assert.Equal(t, 13, doc2Child.EndPage)
	})
}

func TestCloneNode(t *testing.T) {
	t.Run("clone nil node", func(t *testing.T) {
		result := CloneNode(nil)
		assert.Nil(t, result)
	})

	t.Run("clone node without children", func(t *testing.T) {
		node := NewNode("Test", 1, 5)
		node.Summary = "Test summary"

		cloned := CloneNode(node)

		assert.Equal(t, node.ID, cloned.ID)
		assert.Equal(t, node.Title, cloned.Title)
		assert.Equal(t, node.StartPage, cloned.StartPage)
		assert.Equal(t, node.EndPage, cloned.EndPage)
		assert.Equal(t, node.Summary, cloned.Summary)
		assert.Len(t, cloned.Children, 0)
		assert.NotSame(t, node, cloned)
	})

	t.Run("clone node with children", func(t *testing.T) {
		root := NewNode("Root", 1, 10)
		child1 := NewNode("Child 1", 1, 5)
		child2 := NewNode("Child 2", 6, 10)
		root.AddChild(child1)
		root.AddChild(child2)

		cloned := CloneNode(root)

		assert.Equal(t, root.ID, cloned.ID)
		assert.Equal(t, root.Title, cloned.Title)
		assert.Len(t, cloned.Children, 2)
		assert.Equal(t, child1.ID, cloned.Children[0].ID)
		assert.Equal(t, child2.ID, cloned.Children[1].ID)
		assert.NotSame(t, root, cloned)
		assert.NotSame(t, root.Children[0], cloned.Children[0])
		assert.NotSame(t, root.Children[1], cloned.Children[1])
	})

	t.Run("deep clone is independent", func(t *testing.T) {
		root := NewNode("Root", 1, 10)
		child := NewNode("Child", 1, 5)
		root.AddChild(child)

		cloned := CloneNode(root)
		cloned.Title = "Modified"
		cloned.Children[0].Title = "Modified Child"

		assert.Equal(t, "Root", root.Title)
		assert.Equal(t, "Child", root.Children[0].Title)
	})
}

func TestIndexTree_Clone(t *testing.T) {
	t.Run("clone nil tree", func(t *testing.T) {
		result := ((*IndexTree)(nil)).Clone()
		assert.Nil(t, result)
	})

	t.Run("clone empty tree", func(t *testing.T) {
		tree := NewIndexTree(nil, 0)
		cloned := tree.Clone()

		assert.Nil(t, cloned.Root)
		assert.Equal(t, 0, cloned.TotalPages)
	})

	t.Run("clone tree with structure", func(t *testing.T) {
		root := NewNode("Root", 1, 10)
		child := NewNode("Child", 1, 5)
		root.AddChild(child)
		tree := NewIndexTree(root, 10)
		tree.DocumentInfo = "Test Document"
		tree.Version = 5

		cloned := tree.Clone()

		assert.Equal(t, tree.Root.ID, cloned.Root.ID)
		assert.Equal(t, tree.TotalPages, cloned.TotalPages)
		assert.Equal(t, tree.DocumentInfo, cloned.DocumentInfo)
		assert.Equal(t, tree.Version, cloned.Version)
		assert.Equal(t, tree.Root.Children[0].ID, cloned.Root.Children[0].ID)
		assert.NotSame(t, tree.Root, cloned.Root)
		assert.NotSame(t, tree.Root.Children[0], cloned.Root.Children[0])

		assert.Equal(t, cloned.Root, cloned.FindNodeByID(cloned.Root.ID))
		assert.Equal(t, cloned.Root.Children[0], cloned.FindNodeByID(cloned.Root.Children[0].ID))
	})

	t.Run("cloned tree is independent", func(t *testing.T) {
		root := NewNode("Root", 1, 10)
		tree := NewIndexTree(root, 10)

		cloned := tree.Clone()
		cloned.TotalPages = 20
		cloned.Root.Title = "Modified"

		assert.Equal(t, 10, tree.TotalPages)
		assert.Equal(t, "Root", tree.Root.Title)
	})
}

func TestIndexTree_CountAllNodes(t *testing.T) {
	t.Run("count nodes in tree", func(t *testing.T) {
		root := NewNode("Root", 1, 10)
		root.AddChild(NewNode("Child 1", 1, 5))
		root.AddChild(NewNode("Child 2", 6, 10))

		tree := NewIndexTree(root, 10)
		assert.Equal(t, 3, tree.CountAllNodes())
	})

	t.Run("count nodes with deep hierarchy", func(t *testing.T) {
		root := NewNode("Root", 1, 20)
		child1 := NewNode("Child 1", 1, 10)
		child2 := NewNode("Child 2", 11, 20)
		child1.AddChild(NewNode("Grandchild 1", 1, 5))
		child1.AddChild(NewNode("Grandchild 2", 6, 10))
		root.AddChild(child1)
		root.AddChild(child2)

		tree := NewIndexTree(root, 20)
		assert.Equal(t, 5, tree.CountAllNodes())
	})

	t.Run("count nodes with nil root", func(t *testing.T) {
		tree := NewIndexTree(nil, 0)
		assert.Equal(t, 0, tree.CountAllNodes())
	})

	t.Run("count nodes with only root", func(t *testing.T) {
		root := NewNode("Root", 1, 10)
		tree := NewIndexTree(root, 10)
		assert.Equal(t, 1, tree.CountAllNodes())
	})
}
