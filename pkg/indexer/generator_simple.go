package indexer

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/xgsong/mypageindexgo/pkg/document"
)

func normalizeArabicToChinese(s string) string {
	arabicToChinese := map[rune]rune{
		'1': '一',
		'2': '二',
		'3': '三',
		'4': '四',
		'5': '五',
		'6': '六',
		'7': '七',
		'8': '八',
		'9': '九',
	}

	if s == "10" {
		return "十"
	}

	if len(s) == 2 && s[0] == '1' {
		return "十" + string(arabicToChinese[rune(s[1])])
	}

	if len(s) == 2 && s[1] == '0' {
		return string(arabicToChinese[rune(s[0])]) + "十"
	}

	if len(s) == 2 {
		return string(arabicToChinese[rune(s[0])]) + "十" + string(arabicToChinese[rune(s[1])])
	}

	if len(s) == 1 {
		return string(arabicToChinese[rune(s[0])])
	}

	return s
}

// isChapterTitle checks if a title is a chapter title (e.g., "第一章 总则", "第1章 引言")
func isChapterTitle(title string) bool {
	if title == "" {
		return false
	}
	// 检测中文章节标题模式：第X章，支持中文数字和阿拉伯数字
	chapterPattern := regexp.MustCompile(`^第[零一二三四五六七八九十百千万\d]+章`)
	return chapterPattern.MatchString(title)
}

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
func (g *IndexGenerator) generateTreeFromTOC(items []TOCItem, pageTexts []string, totalPages int) *document.Node {
	if len(items) == 0 {
		return nil
	}

	// Step 1: Prepare TOC items
	items = prepareTOCItems(items)

	// Step 2: Sort items by page number
	items = sortTOCItemsByPage(items)

	// Step 3: Calculate page ranges
	items = calculatePageRanges(items, totalPages)

	// Step 4: Build tree structure
	nodes, rootNodes := buildTreeStructure(items, g.pageTextMap, totalPages)

	// Step 5: Fill placeholder titles
	nodes = fillPlaceholderTitles(nodes, items)

	// Step 6: Reorganize root nodes
	rootNodes = reorganizeRootNodes(nodes, rootNodes)

	// Step 7: Clean node structure
	rootNodes = cleanNodeStructure(rootNodes)

	// Step 8: Merge duplicate chapters
	rootNodes, _ = mergeDuplicateChapters(rootNodes, nodes)

	// Step 9: Recalculate parent page ranges
	rootNodes = recalculateParentPageRanges(rootNodes)

	// Step 10: Create root node
	if len(rootNodes) == 0 {
		// Fallback: create flat structure
		return createFlatStructure(items, totalPages)
	}

	return createRootNode(rootNodes, totalPages)
}

func scanAndAddMissingSubsections(tocItems []TOCItem, pageTexts []string, startIndex int) []TOCItem {
	subsectionPattern := regexp.MustCompile(`###\s*(\d+\.\d+)\s*(.+)`)

	existingItems := make(map[string]*TOCItem)
	for i := range tocItems {
		existingItems[tocItems[i].Structure] = &tocItems[i]
	}

	var addedItems []TOCItem

	for pageNum, text := range pageTexts {
		matches := subsectionPattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) < 3 {
				continue
			}
			structure := match[1]
			title := strings.TrimSpace(match[2])
			actualPage := pageNum + startIndex

			if existing, found := existingItems[structure]; found {
				if *existing.PhysicalIndex != actualPage {
					existing.PhysicalIndex = &actualPage
					existing.Page = &actualPage
				}
			} else {
				addedItems = append(addedItems, TOCItem{
					Structure:     structure,
					Title:         title,
					PhysicalIndex: &actualPage,
					Page:          &actualPage,
					ListIndex:     len(tocItems) + len(addedItems),
				})
				existingItems[structure] = &addedItems[len(addedItems)-1]
			}
		}
	}

	if len(addedItems) > 0 {
		merged := make([]TOCItem, len(tocItems)+len(addedItems))
		copy(merged, tocItems)
		copy(merged[len(tocItems):], addedItems)
		return merged
	}

	return tocItems
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

// prepareTOCItems prepares TOC items by setting PhysicalIndex from Page if not set
func prepareTOCItems(items []TOCItem) []TOCItem {
	for i := range items {
		if items[i].PhysicalIndex == nil && items[i].Page != nil {
			items[i].PhysicalIndex = items[i].Page
		}
	}
	return items
}

// sortTOCItemsByPage sorts TOC items by PhysicalIndex (page number) and ListIndex
func sortTOCItemsByPage(items []TOCItem) []TOCItem {
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
	return items
}

// calculatePageRanges calculates the EndPage for each TOC item based on page order
func calculatePageRanges(items []TOCItem, totalPages int) []TOCItem {
	for i := range items {
		if items[i].PhysicalIndex == nil {
			continue
		}

		startPage := *items[i].PhysicalIndex
		nextDifferentPage := -1
		for j := i + 1; j < len(items); j++ {
			if items[j].PhysicalIndex != nil && *items[j].PhysicalIndex > startPage {
				nextDifferentPage = *items[j].PhysicalIndex
				break
			}
		}

		if nextDifferentPage > startPage {
			items[i].EndPage = nextDifferentPage

		} else {
			items[i].EndPage = totalPages

		}

		samePageNext := false
		if i < len(items)-1 {
			nextItem := items[i+1]
			if nextItem.PhysicalIndex != nil && *nextItem.PhysicalIndex == startPage {
				samePageNext = true

			}
		}
		if samePageNext {
			items[i].EndPage = startPage

		}
	}
	return items
}

// buildTreeStructure builds a tree structure from TOC items
func buildTreeStructure(items []TOCItem, pageTextMap map[int]string, totalPages int) (map[string]*document.Node, []*document.Node) {
	nodes := make(map[string]*document.Node)
	var rootNodes []*document.Node

	// Helper function to create or get parent node
	var getOrCreateParentNode func(structure string) *document.Node
	getOrCreateParentNode = func(structure string) *document.Node {
		if structure == "" {
			return nil
		}
		if node, ok := nodes[structure]; ok {
			return node
		}
		// Parent doesn't exist yet, create a placeholder
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
			if existingNode.Title == "" {
				startPage := 1
				if item.PhysicalIndex != nil {
					startPage = *item.PhysicalIndex

				}
				existingNode.StartPage = startPage

				existingNode.EndPage = item.EndPage

				preview := extractContentPreview(pageTextMap, startPage, item.EndPage, 100)

				existingNode.Title = enrichTitleWithPreview(item.Title, preview)

			}
			// If title already exists, skip (first occurrence wins - deduplication)

			continue

		}

		startPage := 1
		if item.PhysicalIndex != nil {
			startPage = *item.PhysicalIndex

		}

		preview := extractContentPreview(pageTextMap, startPage, item.EndPage, 100)

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

	return nodes, rootNodes

}

// fillPlaceholderTitles fills placeholder titles for missing parent nodes
func fillPlaceholderTitles(nodes map[string]*document.Node, items []TOCItem) map[string]*document.Node {
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

	return nodes

}

// cleanNodeStructure cleans up node structure by removing empty children slices
func cleanNodeStructure(rootNodes []*document.Node) []*document.Node {
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

	return rootNodes

}

// createFlatStructure creates a flat structure as fallback when tree building fails
func createFlatStructure(items []TOCItem, totalPages int) *document.Node {
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

// createRootNode creates a root node for the tree structure
func createRootNode(rootNodes []*document.Node, totalPages int) *document.Node {
	// If no root nodes, return nil
	if len(rootNodes) == 0 {
		return nil

	}

	// If single root node, ensure its EndPage covers all descendants
	if len(rootNodes) == 1 {
		root := rootNodes[0]

		// Check if the root node is a chapter title
		if isChapterTitle(root.Title) {
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

			// Create a wrapper root node with generic title
			wrapperRoot := document.NewNode("Document", 1, totalPages)

			wrapperRoot.AddChild(root)

			// Ensure wrapper root covers all pages
			if wrapperRoot.EndPage < root.EndPage {
				wrapperRoot.EndPage = root.EndPage

			}

			return wrapperRoot

		}

		// Original logic for non-chapter single root node
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

// mergeDuplicateChapters merges duplicate chapter nodes with matching Chinese numeral
func mergeDuplicateChapters(rootNodes []*document.Node, nodes map[string]*document.Node) ([]*document.Node, map[string]*document.Node) {
	chapterTitleToNode := make(map[string]*document.Node)
	for _, node := range rootNodes {
		if matches := regexp.MustCompile(`第(.+?)章`).FindStringSubmatch(node.Title); len(matches) > 1 {
			chapterNum := strings.TrimSpace(matches[1])
			if allArabic := func(s string) bool {
				for _, r := range s {
					if r < '0' || r > '9' {
						return false

					}

				}

				return true

			}(chapterNum); allArabic {
				chapterNum = normalizeArabicToChinese(chapterNum)

			}

			if existing, ok := chapterTitleToNode[chapterNum]; ok {
				if len(node.Title) > len(existing.Title) {
					chapterTitleToNode[chapterNum] = node

				}

			} else {
				chapterTitleToNode[chapterNum] = node

			}

		}

	}

	deduplicatedRoots := make([]*document.Node, 0, len(rootNodes))

	skippedNodes := make(map[*document.Node]bool)

	for _, node := range rootNodes {
		if skippedNodes[node] {
			continue

		}

		if matches := regexp.MustCompile(`第(.+?)章`).FindStringSubmatch(node.Title); len(matches) > 1 {
			chapterNum := strings.TrimSpace(matches[1])

			if allArabic := func(s string) bool {
				for _, r := range s {
					if r < '0' || r > '9' {
						return false

					}

				}

				return true

			}(chapterNum); allArabic {
				chapterNum = normalizeArabicToChinese(chapterNum)

			}

			preferredNode := chapterTitleToNode[chapterNum]

			if preferredNode != node {
				for _, child := range node.Children {
					preferredNode.AddChild(child)

				}

				if node.StartPage < preferredNode.StartPage {
					preferredNode.StartPage = node.StartPage

				}

				if node.EndPage > preferredNode.EndPage {
					preferredNode.EndPage = node.EndPage

				}

				skippedNodes[node] = true

				continue

			}

		}

		deduplicatedRoots = append(deduplicatedRoots, node)

	}

	return deduplicatedRoots, nodes

}

// recalculateParentPageRanges recalculates page ranges for all parent nodes based on their children
func recalculateParentPageRanges(rootNodes []*document.Node) []*document.Node {
	var recalculatePageRanges func(*document.Node) (int, int)
	recalculatePageRanges = func(n *document.Node) (int, int) {
		if len(n.Children) == 0 {
			return n.StartPage, n.EndPage

		}

		firstMin, firstMax := recalculatePageRanges(n.Children[0])

		minPage, maxPage := firstMin, firstMax

		for _, child := range n.Children[1:] {
			childMin, childMax := recalculatePageRanges(child)

			if childMin < minPage {
				minPage = childMin

			}

			if childMax > maxPage {
				maxPage = childMax

			}

		}

		n.StartPage = minPage

		n.EndPage = maxPage

		return minPage, maxPage

	}

	for _, node := range rootNodes {
		recalculatePageRanges(node)

	}

	return rootNodes

}

// reorganizeRootNodes moves non-root nodes to their correct parent nodes
func reorganizeRootNodes(nodes map[string]*document.Node, rootNodes []*document.Node) []*document.Node {
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

	return rootNodes

}
