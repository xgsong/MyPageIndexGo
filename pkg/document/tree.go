package document

import (
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
)

// Node represents a node in the index tree.
type Node struct {
	ID        string  `json:"id"`
	Title     string  `json:"title"`
	StartPage int     `json:"start_page"`
	EndPage   int     `json:"end_page"`
	Summary   string  `json:"summary,omitempty"`
	Children  []*Node `json:"children,omitempty"`
}

// NewNode creates a new Node with a generated UUID.
func NewNode(title string, startPage, endPage int) *Node {
	return &Node{
		ID:        uuid.New().String()[:12],
		Title:     title,
		StartPage: startPage,
		EndPage:   endPage,
		Children:  []*Node{},
	}
}

// AddChild adds a child node to this node.
func (n *Node) AddChild(child *Node) {
	n.Children = append(n.Children, child)
	if child.StartPage < n.StartPage || n.StartPage == 0 {
		n.StartPage = child.StartPage
	}
	if child.EndPage > n.EndPage {
		n.EndPage = child.EndPage
	}
}

// CountNodes returns the total number of nodes in the subtree rooted at this node.
func (n *Node) CountNodes() int {
	count := 1
	for _, child := range n.Children {
		count += child.CountNodes()
	}
	return count
}

// IndexTree represents the complete index tree for a document.
type IndexTree struct {
	Root         *Node            `json:"root"`
	TotalPages   int              `json:"total_pages"`
	DocumentInfo string           `json:"document_info"`
	GeneratedAt  time.Time        `json:"generated_at"`
	Version      int              `json:"version,omitempty"` // Version number for incremental updates
	LastModified time.Time        `json:"last_modified"`     // Last modification time
	nodeMap      map[string]*Node `json:"-"`                 // ID to node mapping for fast lookups, not serialized
}

// NewIndexTree creates a new empty IndexTree.
func NewIndexTree(root *Node, totalPages int) *IndexTree {
	tree := &IndexTree{
		Root:        root,
		TotalPages:  totalPages,
		GeneratedAt: time.Now(),
		nodeMap:     make(map[string]*Node),
	}
	tree.BuildNodeMap()
	return tree
}

// BuildNodeMap recursively builds the node ID to node mapping
func (t *IndexTree) BuildNodeMap() {
	if t.nodeMap == nil {
		t.nodeMap = make(map[string]*Node)
	} else {
		clear(t.nodeMap)
	}

	var traverse func(*Node)
	traverse = func(node *Node) {
		if node == nil {
			return
		}
		t.nodeMap[node.ID] = node
		for _, child := range node.Children {
			traverse(child)
		}
	}
	traverse(t.Root)
}

// FindNodeByID returns the node with the given ID, or nil if not found
func (t *IndexTree) FindNodeByID(id string) *Node {
	return t.nodeMap[id]
}

// CountAllNodes returns the total number of nodes in the entire tree.
func (t *IndexTree) CountAllNodes() int {
	if t.Root == nil {
		return 0
	}
	return t.Root.CountNodes()
}

// FindOverlappingNodes returns all nodes that overlap with the given page range (inclusive).
func (t *IndexTree) FindOverlappingNodes(startPage, endPage int) []*Node {
	var overlapping []*Node

	var traverse func(*Node)
	traverse = func(node *Node) {
		if node == nil {
			return
		}
		// Check for overlap: [node.StartPage, node.EndPage] intersects with [startPage, endPage]
		if node.StartPage <= endPage && node.EndPage >= startPage {
			overlapping = append(overlapping, node)
		}
		for _, child := range node.Children {
			traverse(child)
		}
	}

	traverse(t.Root)
	return overlapping
}

// Merge merges another index tree into this one.
// The new tree's pages are appended after the existing tree's pages.
// Returns the merged tree (this tree is modified in place).
// The other tree is cloned before modification to preserve its original state.
func (t *IndexTree) Merge(other *IndexTree) *IndexTree {
	if other == nil || other.Root == nil {
		return t
	}

	// Clone the other tree to avoid modifying the original
	otherClone := other.Clone()

	// Calculate page offset for the new tree
	pageOffset := t.TotalPages

	// Adjust page numbers in the cloned tree
	var adjustPageNumbers func(*Node)
	adjustPageNumbers = func(node *Node) {
		if node == nil {
			return
		}
		node.StartPage += pageOffset
		node.EndPage += pageOffset
		for _, child := range node.Children {
			adjustPageNumbers(child)
		}
	}
	adjustPageNumbers(otherClone.Root)

	// Merge the root nodes
	if t.Root == nil {
		t.Root = otherClone.Root
	} else {
		// Create a combined root node
		combinedRoot := NewNode(
			"Combined Document",
			1,
			t.TotalPages+otherClone.TotalPages,
		)
		combinedRoot.AddChild(t.Root)
		combinedRoot.AddChild(otherClone.Root)
		t.Root = combinedRoot
	}

	// Update total pages and metadata
	t.TotalPages += otherClone.TotalPages
	t.Version++
	t.LastModified = time.Now()
	t.DocumentInfo = fmt.Sprintf("Combined document: %s + %s", t.DocumentInfo, otherClone.DocumentInfo)

	// Rebuild the node map
	t.BuildNodeMap()

	return t
}

// CloneNode creates a deep copy of a node and all its children.
func CloneNode(node *Node) *Node {
	if node == nil {
		return nil
	}
	cloned := &Node{
		ID:        node.ID,
		Title:     node.Title,
		StartPage: node.StartPage,
		EndPage:   node.EndPage,
		Summary:   node.Summary,
		Children:  slices.Clone(node.Children),
	}
	for i, child := range node.Children {
		cloned.Children[i] = CloneNode(child)
	}
	return cloned
}

// Clone creates a deep copy of the index tree.
func (t *IndexTree) Clone() *IndexTree {
	if t == nil {
		return nil
	}

	clonedRoot := CloneNode(t.Root)
	clonedTree := NewIndexTree(clonedRoot, t.TotalPages)
	clonedTree.DocumentInfo = t.DocumentInfo
	clonedTree.GeneratedAt = t.GeneratedAt
	clonedTree.Version = t.Version
	clonedTree.LastModified = t.LastModified

	return clonedTree
}

// SearchResult represents a search result from querying the index tree.
type SearchResult struct {
	Query  string  `json:"query"`
	Answer string  `json:"answer"`
	Nodes  []*Node `json:"nodes"`
}
