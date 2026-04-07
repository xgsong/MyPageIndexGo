package indexer

import (
	"github.com/xgsong/mypageindexgo/pkg/document"
)

func (g *IndexGenerator) generateTreeFromTOC(items []TOCItem, pageTexts []string, totalPages int) *document.Node {
	if len(items) == 0 {
		return nil
	}

	items = prepareTOCItems(items)
	items = sortTOCItemsByPage(items)
	items = calculatePageRanges(items, totalPages)
	nodes, rootNodes := buildTreeStructure(items, g.pageTextMap, totalPages)
	nodes = fillPlaceholderTitles(nodes, items)
	rootNodes = reorganizeRootNodes(nodes, rootNodes)
	rootNodes = cleanNodeStructure(rootNodes)
	rootNodes, _ = mergeDuplicateChapters(rootNodes, nodes)
	rootNodes = recalculateParentPageRanges(rootNodes)

	if len(rootNodes) == 0 {
		return createFlatStructure(items, totalPages)
	}

	return createRootNode(rootNodes, totalPages)
}
