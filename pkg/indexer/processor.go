package indexer

import (
	"fmt"
	"strings"

	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/tokenizer"
)

// PageGroup represents a group of pages combined into a single chunk for LLM processing.
type PageGroup struct {
	Text       string `json:"text"`
	StartPage  int    `json:"start_page"`
	EndPage    int    `json:"end_page"`
	TokenCount int    `json:"token_count"`
}

// PageGrouper groups document pages into chunks based on token limit.
type PageGrouper struct {
	tokenizer *tokenizer.Tokenizer
	maxTokens int
}

// NewPageGrouper creates a new PageGrouper with the given tokenizer and max tokens per group.
func NewPageGrouper(t *tokenizer.Tokenizer, maxTokens int) *PageGrouper {
	return &PageGrouper{
		tokenizer: t,
		maxTokens: maxTokens,
	}
}

// GroupPages groups document pages into chunks that don't exceed the token limit.
func (g *PageGrouper) GroupPages(doc *document.Document) ([]*PageGroup, error) {
	if len(doc.Pages) == 0 {
		return nil, fmt.Errorf("document has no pages")
	}

	var groups []*PageGroup
	var currentGroup *PageGroup
	var currentText strings.Builder
	var currentTokens int

	for i, page := range doc.Pages {
		pageTokens := g.tokenizer.Count(page.Text)

		// If a single page exceeds max tokens, we need to truncate it
		if pageTokens > g.maxTokens {
			// If we have a current group, finalize it
			if currentGroup != nil {
				groups = append(groups, &PageGroup{
					Text:       currentText.String(),
					StartPage:  currentGroup.StartPage,
					EndPage:    doc.Pages[i-1].Number,
					TokenCount: currentTokens,
				})
				currentGroup = nil
				currentText.Reset()
				currentTokens = 0
			}

			// Truncate the page to fit within max tokens
			_, truncated := g.tokenizer.CountWithTruncate(page.Text, g.maxTokens)
			groups = append(groups, &PageGroup{
				Text:       truncated,
				StartPage:  page.Number,
				EndPage:    page.Number,
				TokenCount: g.maxTokens,
			})
			continue
		}

		// If adding this page would exceed the limit, finalize current group
		if currentGroup != nil && currentTokens+pageTokens > g.maxTokens {
			groups = append(groups, &PageGroup{
				Text:       currentText.String(),
				StartPage:  currentGroup.StartPage,
				EndPage:    doc.Pages[i-1].Number,
				TokenCount: currentTokens,
			})
			currentGroup = nil
			currentText.Reset()
			currentTokens = 0
		}

		// Start a new group if needed
		if currentGroup == nil {
			currentGroup = &PageGroup{
				StartPage: page.Number,
			}
		}

		// Add the page text to current group
		if currentText.Len() > 0 {
			currentText.WriteByte('\n')
		}
		currentText.WriteString(page.Text)
		currentTokens += pageTokens
	}

	// Add the final group if there's anything left
	if currentGroup != nil && currentText.Len() > 0 {
		groups = append(groups, &PageGroup{
			Text:       currentText.String(),
			StartPage:  currentGroup.StartPage,
			EndPage:    doc.Pages[len(doc.Pages)-1].Number,
			TokenCount: currentTokens,
		})
	}

	return groups, nil
}

// MergeNodes merges multiple node trees from different page groups into a single coherent tree.
// It takes the root nodes from each group and combines them under a new document root.
func MergeNodes(groups []*document.Node) *document.Node {
	if len(groups) == 0 {
		return nil
	}

	if len(groups) == 1 {
		return groups[0]
	}

	// Create a new root node that contains all group roots as children
	merged := document.NewNode("Document", 1, 0)
	endPage := 0

	for _, group := range groups {
		if group.StartPage < merged.StartPage {
			merged.StartPage = group.StartPage
		}
		if group.EndPage > endPage {
			endPage = group.EndPage
		}
		// Only add non-empty children
		if len(group.Children) > 0 {
			for _, child := range group.Children {
				merged.AddChild(child)
			}
		} else {
			// If the group has no children, add the root itself
			merged.AddChild(group)
		}
	}

	merged.EndPage = endPage
	return merged
}
