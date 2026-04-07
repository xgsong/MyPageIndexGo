package indexer

import (
	"github.com/xgsong/mypageindexgo/pkg/document"
)

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
