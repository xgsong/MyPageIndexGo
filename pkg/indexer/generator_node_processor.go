package indexer

import (
	"fmt"

	"github.com/xgsong/mypageindexgo/pkg/document"
)

func fillPlaceholderTitles(nodes map[string]*document.Node, items []TOCItem) map[string]*document.Node {
	for structure, node := range nodes {
		if node.Title == "" && structure != "" {
			var inferredTitle string
			for _, item := range items {
				if item.Structure == structure {
					inferredTitle = item.Title
					break
				}
			}

			if inferredTitle == "" {
				inferredTitle = fmt.Sprintf("第%s章", structure)
			}
			node.Title = inferredTitle
		}
	}
	return nodes
}

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

func createFlatStructure(items []TOCItem, totalPages int) *document.Node {
	root := document.NewNode("Document", 1, totalPages)

	for _, item := range items {
		startPage := 1
		if item.PhysicalIndex != nil {
			startPage = *item.PhysicalIndex
		}
		node := document.NewNode(item.Title, startPage, *item.EndPage)
		root.AddChild(node)
	}

	return root
}

func createRootNode(rootNodes []*document.Node, totalPages int) *document.Node {
	if len(rootNodes) == 0 {
		return nil
	}

	if len(rootNodes) == 1 {
		root := rootNodes[0]

		if isChapterTitle(root.Title) {
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

			wrapperRoot := document.NewNode("Document", 1, totalPages)
			wrapperRoot.AddChild(root)
			if wrapperRoot.EndPage < root.EndPage {
				wrapperRoot.EndPage = root.EndPage
			}
			return wrapperRoot
		}

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

	root := document.NewNode("Document", 1, totalPages)
	for _, node := range rootNodes {
		root.AddChild(node)
	}

	if len(root.Children) > 0 && root.EndPage < root.Children[len(root.Children)-1].EndPage {
		root.EndPage = root.Children[len(root.Children)-1].EndPage
	}

	return root
}
