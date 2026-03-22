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
	tokenizer    *tokenizer.Tokenizer
	maxTokens    int
	overlapPages int // Number of pages to overlap between groups
}

// NewPageGrouper creates a new PageGrouper with the given tokenizer and max tokens per group.
func NewPageGrouper(t *tokenizer.Tokenizer, maxTokens int) *PageGrouper {
	return &PageGrouper{
		tokenizer:    t,
		maxTokens:    maxTokens,
		overlapPages: 2, // Default: overlap 2 pages between groups
	}
}

// NewPageGrouperWithOverlap creates a new PageGrouper with custom overlap.
func NewPageGrouperWithOverlap(t *tokenizer.Tokenizer, maxTokens int, overlapPages int) *PageGrouper {
	return &PageGrouper{
		tokenizer:    t,
		maxTokens:    maxTokens,
		overlapPages: overlapPages,
	}
}

// pageWithTokens wraps a page with its pre-computed token count
type pageWithTokens struct {
	page   document.Page
	tokens int
}

// GroupPages groups document pages into chunks that don't exceed the token limit.
// Uses overlapping to ensure sections spanning group boundaries are complete.
// OPTIMIZED: Pre-computes token counts to avoid redundant counting
func (g *PageGrouper) GroupPages(doc *document.Document) ([]*PageGroup, error) {
	if len(doc.Pages) == 0 {
		return nil, fmt.Errorf("document has no pages")
	}

	// OPTIMIZATION: Pre-compute token counts ONCE per page
	pagesWithTokens := make([]pageWithTokens, len(doc.Pages))
	for i, page := range doc.Pages {
		pagesWithTokens[i] = pageWithTokens{
			page:   page,
			tokens: g.tokenizer.Count(page.Text),
		}
	}

	// Small document optimization - but first check if any page exceeds maxTokens
	if len(doc.Pages) <= g.overlapPages*2 {
		// Check if any single page exceeds maxTokens
		anyPageTooLarge := false
		for _, pwt := range pagesWithTokens {
			if pwt.tokens > g.maxTokens {
				anyPageTooLarge = true
				break
			}
		}

		// Only use small document optimization if all pages fit within limit
		if !anyPageTooLarge {
			var allText strings.Builder
			totalTokens := 0
			for i, pwt := range pagesWithTokens {
				if i > 0 {
					allText.WriteByte('\n')
				}
				allText.WriteString(pwt.page.Text)
				totalTokens += pwt.tokens
			}
			return []*PageGroup{{
				Text:       allText.String(),
				StartPage:  doc.Pages[0].Number,
				EndPage:    doc.Pages[len(doc.Pages)-1].Number,
				TokenCount: totalTokens,
			}}, nil
		}
	}

	var groups []*PageGroup
	var currentGroup *PageGroup
	var currentText strings.Builder
	var currentTokens int
	var overlapBuffer []pageWithTokens

	for i, pwt := range pagesWithTokens {
		pageTokens := pwt.tokens

		if pageTokens > g.maxTokens {
			if currentGroup != nil {
				groups = append(groups, &PageGroup{
					Text:       currentText.String(),
					StartPage:  currentGroup.StartPage,
					EndPage:    pagesWithTokens[i-1].page.Number,
					TokenCount: currentTokens,
				})
				currentGroup = nil
				currentText.Reset()
				currentTokens = 0
				overlapBuffer = nil
			}

			_, truncated := g.tokenizer.CountWithTruncate(pwt.page.Text, g.maxTokens)
			groups = append(groups, &PageGroup{
				Text:       truncated,
				StartPage:  pwt.page.Number,
				EndPage:    pwt.page.Number,
				TokenCount: g.maxTokens,
			})
			continue
		}

		if currentGroup != nil && currentTokens+pageTokens > g.maxTokens {
			groups = append(groups, &PageGroup{
				Text:       currentText.String(),
				StartPage:  currentGroup.StartPage,
				EndPage:    pagesWithTokens[i-1].page.Number,
				TokenCount: currentTokens,
			})

			currentGroup = nil
			currentText.Reset()
			currentTokens = 0

			if len(overlapBuffer) > 0 {
				for _, overlapPwt := range overlapBuffer {
					if currentText.Len() > 0 {
						currentText.WriteByte('\n')
					}
					currentText.WriteString(overlapPwt.page.Text)
					currentTokens += overlapPwt.tokens
				}
				currentGroup = &PageGroup{
					StartPage: overlapBuffer[0].page.Number,
				}
			}
			overlapBuffer = nil
		}

		if currentGroup == nil {
			currentGroup = &PageGroup{
				StartPage: pwt.page.Number,
			}
		}

		if currentText.Len() > 0 {
			currentText.WriteByte('\n')
		}
		currentText.WriteString(pwt.page.Text)
		currentTokens += pageTokens

		overlapBuffer = append(overlapBuffer, pwt)
		if len(overlapBuffer) > g.overlapPages {
			overlapBuffer = overlapBuffer[len(overlapBuffer)-g.overlapPages:]
		}
	}

	if currentGroup != nil && currentText.Len() > 0 {
		groups = append(groups, &PageGroup{
			Text:       currentText.String(),
			StartPage:  currentGroup.StartPage,
			EndPage:    pagesWithTokens[len(pagesWithTokens)-1].page.Number,
			TokenCount: currentTokens,
		})
	}

	return groups, nil
}

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
		if group.StartPage < merged.StartPage {
			merged.StartPage = group.StartPage
		}
		if group.EndPage > endPage {
			endPage = group.EndPage
		}
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
