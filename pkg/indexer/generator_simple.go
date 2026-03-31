package indexer

import (
	"fmt"
	"sort"
	"strings"

	"github.com/xgsong/mypageindexgo/pkg/document"
)

// extractContentPreview extracts a preview from the page text for a node.
// Returns the first meaningful content (up to maxChars) from the node's page range.
func extractContentPreview(pageTextMap map[int]string, startPage, endPage int, maxChars int) string {
	if pageTextMap == nil || startPage > endPage {
		return ""
	}

	var content strings.Builder
	charsCollected := 0

	for pageNum := startPage; pageNum <= endPage && charsCollected < maxChars; pageNum++ {
		if text, ok := pageTextMap[pageNum]; ok && text != "" {
			// Skip TOC pages (usually have little meaningful content)
			trimmed := strings.TrimSpace(text)
			if len(trimmed) < 50 {
				continue
			}

			remaining := maxChars - charsCollected
			if len(text) <= remaining {
				content.WriteString(text)
				charsCollected += len(text)
			} else {
				content.WriteString(text[:remaining])
				charsCollected = maxChars
			}

			if charsCollected < maxChars {
				content.WriteString(" ")
			}
		}
	}

	preview := strings.TrimSpace(content.String())
	if len(preview) > maxChars {
		// Truncate at word boundary if possible
		if lastSpace := strings.LastIndex(preview[:maxChars], " "); lastSpace > maxChars/2 {
			preview = preview[:lastSpace] + "..."
		} else {
			preview = preview[:maxChars-3] + "..."
		}
	} else if preview != "" {
		preview += "..."
	}

	return preview
}

// enrichTitleWithPreview enriches a node title with content preview if title is too brief.
func enrichTitleWithPreview(title string, preview string) string {
	// Disable title enrichment to avoid long titles with invalid characters
	return title
}

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

	// CRITICAL FIX: Sort items by PhysicalIndex BEFORE EndPage calculation
	// This ensures items are in page order, not structure order (1, 1.1, 1.2, 2...)
	// Without this, EndPage calculation produces wrong results
	sort.Slice(items, func(i, j int) bool {
		if items[i].PhysicalIndex == nil {
			return false
		}
		if items[j].PhysicalIndex == nil {
			return true
		}
		if *items[i].PhysicalIndex != *items[j].PhysicalIndex {
			return *items[i].PhysicalIndex < *items[j].PhysicalIndex
		}
		return items[i].ListIndex < items[j].ListIndex
	})

	// Second pass: Calculate end_index for each item
	// Python: post_processing in utils.py:432-438
	for i := range items {
		if items[i].PhysicalIndex == nil {
			continue
		}

		startPage := *items[i].PhysicalIndex

		if i < len(items)-1 && items[i+1].PhysicalIndex != nil {
			nextPhysicalIndex := *items[i+1].PhysicalIndex
			// If next section starts on a later page, current ends at nextPhysicalIndex - 1
			// If next section starts on the same page, current ends at the same page
			if nextPhysicalIndex > startPage {
				items[i].EndPage = nextPhysicalIndex - 1
			} else {
				// Same page, both sections are on this page
				items[i].EndPage = startPage
			}
		} else {
			// Last item
			items[i].EndPage = totalPages // Page numbers are 1-based
		}

		// Clamp: end_page must be at least start_page
		if items[i].EndPage < startPage {
			items[i].EndPage = startPage
		}
	}

	// Third pass: Build tree structure
	nodes := make(map[string]*document.Node)
	var rootNodes []*document.Node

	// Helper function to create or get parent node
	// If parent doesn't exist, creates a placeholder that will be updated later
	var getOrCreateParentNode func(structure string) *document.Node
	getOrCreateParentNode = func(structure string) *document.Node {
		if structure == "" {
			return nil
		}
		if node, ok := nodes[structure]; ok {
			return node
		}
		// Parent doesn't exist yet, create a placeholder
		// Title will be updated when the real item is processed
		// If no real item comes, we'll generate a reasonable title from structure
		placeholderNode := document.NewNode("", 1, totalPages)
		nodes[structure] = placeholderNode
		
		// Recursively ensure grandparent exists
		grandparentStructure := getParentStructure(structure)
		if grandparentStructure != "" {
			grandparent := getOrCreateParentNode(grandparentStructure)
			if grandparent != nil {
				grandparent.AddChild(placeholderNode)
			} else {
				rootNodes = append(rootNodes, placeholderNode)
			}
		} else {
			rootNodes = append(rootNodes, placeholderNode)
		}
		return placeholderNode
	}

	for _, item := range items {
		if existingNode, exists := nodes[item.Structure]; exists {
			// Node already exists - only update if it's a placeholder (empty title)
			// This handles the case where child appears before parent in page order
			if existingNode.Title == "" {
				startPage := 1
				if item.PhysicalIndex != nil {
					startPage = *item.PhysicalIndex
				}
				existingNode.StartPage = startPage
				existingNode.EndPage = item.EndPage
				preview := extractContentPreview(g.pageTextMap, startPage, item.EndPage, 100)
				existingNode.Title = enrichTitleWithPreview(item.Title, preview)
			}
			// If title already exists, skip (first occurrence wins - deduplication)
			continue
		}

		startPage := 1
		if item.PhysicalIndex != nil {
			startPage = *item.PhysicalIndex
		}

		preview := extractContentPreview(g.pageTextMap, startPage, item.EndPage, 100)
		enrichedTitle := enrichTitleWithPreview(item.Title, preview)

		node := document.NewNode(enrichedTitle, startPage, item.EndPage)
		nodes[item.Structure] = node

		parentStructure := getParentStructure(item.Structure)

		if parentStructure != "" {
			parent := getOrCreateParentNode(parentStructure)
			if parent != nil {
				parent.AddChild(node)
			} else {
				rootNodes = append(rootNodes, node)
			}
		} else {
			rootNodes = append(rootNodes, node)
		}
	}

	// Fill in placeholder titles for missing parent nodes
	// This handles cases where LLM returns subsections (e.g., 10.1, 10.2) without the parent (10)
	for structure, node := range nodes {
		if node.Title == "" && structure != "" {
			// Generate a reasonable title from structure
			// Try to find a child with a meaningful title to use as parent title
			var inferredTitle string
			for _, item := range items {
				if item.Structure == structure {
					inferredTitle = item.Title
					break
				}
			}
			if inferredTitle == "" {
				// No item found, generate title from structure
				inferredTitle = fmt.Sprintf("第%s章", structure)
			}
			node.Title = inferredTitle
		}
	}

	structureForNode := make(map[*document.Node]string, len(nodes))
	for structure, node := range nodes {
		structureForNode[node] = structure
	}

	for i := 0; i < len(rootNodes); {
		node := rootNodes[i]
		nodeStructure := structureForNode[node]

		if nodeStructure == "" {
			i++
			continue
		}

		parentStructure := getParentStructure(nodeStructure)

		if parentStructure != "" {
			if parent, ok := nodes[parentStructure]; ok {
				parent.AddChild(node)
				rootNodes[i] = rootNodes[len(rootNodes)-1]
				rootNodes = rootNodes[:len(rootNodes)-1]
				continue
			}
		}
		i++
	}

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

	// If single root node, ensure its EndPage covers all descendants
	if len(rootNodes) == 1 {
		root := rootNodes[0]
		// Calculate the max end page from all descendants
		var maxEndPage int
		var findMaxEndPage func(*document.Node)
		findMaxEndPage = func(n *document.Node) {
			if n.EndPage > maxEndPage {
				maxEndPage = n.EndPage
			}
			for _, child := range n.Children {
				findMaxEndPage(child)
			}
		}
		findMaxEndPage(root)
		if maxEndPage > root.EndPage {
			root.EndPage = maxEndPage
		}
		return root
	}

	// Multiple root nodes - create wrapper root
	root := document.NewNode("Document", 1, totalPages)
	for _, node := range rootNodes {
		root.AddChild(node)
	}

	// Ensure root.EndPage covers all children if it has any
	if len(root.Children) > 0 && root.EndPage < root.Children[len(root.Children)-1].EndPage {
		root.EndPage = root.Children[len(root.Children)-1].EndPage
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
