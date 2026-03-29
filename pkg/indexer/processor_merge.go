package indexer

import (
	"sort"

	"github.com/xgsong/mypageindexgo/pkg/document"
)

// MergeNodes merges multiple node trees from different page groups into a single coherent tree.
// Input nodes are cloned to avoid mutating the original trees.
func MergeNodes(groups []*document.Node) *document.Node {
	if len(groups) == 0 {
		return nil
	}

	if len(groups) == 1 {
		return document.CloneNode(groups[0])
	}

	allChildren := make([]*document.Node, 0)
	minStartPage := groups[0].StartPage
	maxEndPage := 0

	for _, group := range groups {
		if group.StartPage < minStartPage {
			minStartPage = group.StartPage
		}
		if group.EndPage > maxEndPage {
			maxEndPage = group.EndPage
		}
		if len(group.Children) > 0 {
			for _, child := range group.Children {
				allChildren = append(allChildren, document.CloneNode(child))
			}
		} else {
			allChildren = append(allChildren, document.CloneNode(group))
		}
	}

	sort.Slice(allChildren, func(i, j int) bool {
		return allChildren[i].StartPage < allChildren[j].StartPage
	})

	for i := 0; i < len(allChildren); i++ {
		if i < len(allChildren)-1 {
			nextStart := allChildren[i+1].StartPage
			if allChildren[i].EndPage >= nextStart {
				allChildren[i].EndPage = nextStart - 1
			}
		}
	}

	merged := document.NewNode("Document", minStartPage, maxEndPage)
	for _, child := range allChildren {
		merged.AddChild(child)
	}

	return merged
}
