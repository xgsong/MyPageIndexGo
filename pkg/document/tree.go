package document

import (
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
	Root         *Node     `json:"root"`
	TotalPages   int       `json:"total_pages"`
	DocumentInfo string    `json:"document_info"`
	GeneratedAt  time.Time `json:"generated_at"`
}

// NewIndexTree creates a new empty IndexTree.
func NewIndexTree(root *Node, totalPages int) *IndexTree {
	return &IndexTree{
		Root:        root,
		TotalPages:  totalPages,
		GeneratedAt: time.Now(),
	}
}

// CountAllNodes returns the total number of nodes in the entire tree.
func (t *IndexTree) CountAllNodes() int {
	if t.Root == nil {
		return 0
	}
	return t.Root.CountNodes()
}

// SearchResult represents a search result from querying the index tree.
type SearchResult struct {
	Query  string  `json:"query"`
	Answer string  `json:"answer"`
	Nodes  []*Node `json:"nodes"`
}
