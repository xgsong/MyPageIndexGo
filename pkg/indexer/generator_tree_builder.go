package indexer

import (
	"github.com/xgsong/mypageindexgo/pkg/document"
)

func buildTreeStructure(items []TOCItem, pageTextMap map[int]string, totalPages int) (map[string]*document.Node, []*document.Node) {
	nodes := make(map[string]*document.Node)
	var rootNodes []*document.Node

	var getOrCreateParentNode func(structure string) *document.Node
	getOrCreateParentNode = func(structure string) *document.Node {
		if structure == "" {
			return nil
		}
		if node, ok := nodes[structure]; ok {
			return node
		}
		placeholderNode := document.NewNode("", 1, totalPages)
		nodes[structure] = placeholderNode

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
			if existingNode.Title == "" {
				startPage := 1
				if item.PhysicalIndex != nil {
					startPage = *item.PhysicalIndex
				}
				existingNode.StartPage = startPage
				existingNode.EndPage = *item.EndPage
				preview := extractContentPreview(pageTextMap, startPage, *item.EndPage, 100)
				existingNode.Title = enrichTitleWithPreview(item.Title, preview)
			}
			continue
		}

		startPage := 1
		if item.PhysicalIndex != nil {
			startPage = *item.PhysicalIndex
		}

		preview := extractContentPreview(pageTextMap, startPage, *item.EndPage, 100)
		enrichedTitle := enrichTitleWithPreview(item.Title, preview)
		node := document.NewNode(enrichedTitle, startPage, *item.EndPage)
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
