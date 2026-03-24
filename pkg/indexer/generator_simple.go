package indexer

import (
	"strings"

	"github.com/xgsong/mypageindexgo/pkg/document"
)

// generateTreeFromTOC generates a tree structure from TOC items
// Python equivalent: post_processing + list_to_tree in utils.py:319-358, 428-447
// This is a simplified version that directly mirrors the Python implementation
func (g *IndexGenerator) generateTreeFromTOC(items []TOCItem, totalPages int) *document.Node {
	if len(items) == 0 {
		return nil
	}

	// First pass: Set start_index (PhysicalIndex) for each item
	// Python: post_processing in utils.py:430
	for i := range items {
		if items[i].PhysicalIndex == nil && items[i].Page != nil {
			items[i].PhysicalIndex = items[i].Page
		}
	}

	// Second pass: Calculate end_index for each item
	// Python: post_processing in utils.py:432-438
	for i := range items {
		if items[i].PhysicalIndex == nil {
			continue
		}

		startPage := *items[i].PhysicalIndex

		if i < len(items)-1 && items[i+1].PhysicalIndex != nil {
			// Check if next item appears at start (appear_start == "yes")
			if items[i+1].AppearStart == "yes" {
				items[i].EndPage = *items[i+1].PhysicalIndex - 1
			} else {
				items[i].EndPage = *items[i+1].PhysicalIndex
			}
		} else {
			// Last item
			items[i].EndPage = totalPages
		}

		// Clamp: end_page must be at least start_page
		if items[i].EndPage < startPage {
			items[i].EndPage = startPage
		}
	}

	// Third pass: Build tree structure
	// Python: list_to_tree in utils.py:327-353
	nodes := make(map[string]*document.Node)
	var rootNodes []*document.Node

	for _, item := range items {
		// Skip duplicate structures (keep first occurrence)
		if _, exists := nodes[item.Structure]; exists {
			continue
		}

		startPage := 1
		if item.PhysicalIndex != nil {
			startPage = *item.PhysicalIndex
		}

		node := document.NewNode(item.Title, startPage, item.EndPage)
		nodes[item.Structure] = node

		// Find parent
		parentStructure := getParentStructure(item.Structure)

		if parentStructure != "" {
			// Add as child to parent if parent exists
			if parent, ok := nodes[parentStructure]; ok {
				parent.AddChild(node)
			} else {
				// Parent not found yet, add to root nodes temporarily
				rootNodes = append(rootNodes, node)
			}
		} else {
			// No parent, this is a root node
			rootNodes = append(rootNodes, node)
		}
	}

	// Clean empty children arrays (Python: clean_node in utils.py:356-362)
	var cleanNode func(n *document.Node)
	cleanNode = func(n *document.Node) {
		if len(n.Children) == 0 {
			n.Children = nil
		} else {
			for _, child := range n.Children {
				cleanNode(child)
			}
		}
	}
	for _, node := range rootNodes {
		cleanNode(node)
	}

	// Python: post_processing fallback (utils.py:440-447)
	// If list_to_tree returns empty, return flat structure
	if len(rootNodes) == 0 {
		// Fallback: create flat nodes from items
		root := document.NewNode("Document", 1, totalPages)
		for _, item := range items {
			startPage := 1
			if item.PhysicalIndex != nil {
				startPage = *item.PhysicalIndex
			}
			node := document.NewNode(item.Title, startPage, item.EndPage)
			root.AddChild(node)
		}
		return root
	}

	// If single root node, return it directly
	if len(rootNodes) == 1 {
		return rootNodes[0]
	}

	// Multiple root nodes - create wrapper root
	root := document.NewNode("Document", 1, totalPages)
	for _, node := range rootNodes {
		root.AddChild(node)
	}

	return root
}

// getParentStructure gets the parent structure number
// e.g., "1.2.3" -> "1.2", "1.2" -> "1", "1" -> ""
// Python: get_parent_structure in utils.py:320-325
func getParentStructure(structure string) string {
	if structure == "" {
		return ""
	}
	parts := strings.Split(structure, ".")
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[:len(parts)-1], ".")
}
