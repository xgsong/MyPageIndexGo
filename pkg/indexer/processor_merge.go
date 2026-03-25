package indexer

import (
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

	merged := document.NewNode("Document", 1, 0)
	endPage := 0

	for _, group := range groups {
		merged.StartPage = min(group.StartPage, merged.StartPage)
		endPage = max(group.EndPage, endPage)
		if len(group.Children) > 0 {
			for _, child := range group.Children {
				merged.AddChild(document.CloneNode(child))
			}
		} else {
			merged.AddChild(document.CloneNode(group))
		}
	}

	merged.EndPage = endPage
	return merged
}
