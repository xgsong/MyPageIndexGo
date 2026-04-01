package indexer

import (
	"fmt"
	"strings"

	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/tokenizer"
)

const (
	MinContentLength = 10
	MinTokenCount    = 3
)

func filterMeaningfulPages(pages []document.Page) []document.Page {
	result := make([]document.Page, 0, len(pages))
	for _, page := range pages {
		contentLength := len(strings.TrimSpace(page.Text))
		if contentLength >= MinContentLength {
			result = append(result, page)
		}
	}
	return result
}

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

	filteredPages := filterMeaningfulPages(doc.Pages)
	if len(filteredPages) == 0 {
		return nil, fmt.Errorf("document has no meaningful content pages")
	}

	pagesWithTokens := make([]pageWithTokens, 0, len(filteredPages))
	for _, page := range filteredPages {
		tokens := g.tokenizer.Count(page.Text)
		if tokens >= MinTokenCount {
			pagesWithTokens = append(pagesWithTokens, pageWithTokens{
				page:   page,
				tokens: tokens,
			})
		}
	}

	if len(pagesWithTokens) == 0 {
		return nil, fmt.Errorf("document has no pages with sufficient content")
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
			// Pre-calculate total size for efficiency
			totalLen := 0
			for _, pwt := range pagesWithTokens {
				totalLen += len(pwt.page.Text) + 1
			}

			var allText strings.Builder
			allText.Grow(totalLen)

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

	estimatedGroups := len(pagesWithTokens)/5 + 1
	groups := make([]*PageGroup, 0, estimatedGroups)
	var currentGroup *PageGroup
	var currentText strings.Builder
	var currentTokens int

	// Pre-allocate buffer for text building (estimate: 500 chars per page)
	currentText.Grow(estimatedGroups * 500)

	// Use pre-allocated overlap buffer with circular indexing
	overlapBuffer := make([]pageWithTokens, g.overlapPages)
	overlapIndex := 0
	overlapCount := 0

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
				overlapIndex = 0
				overlapCount = 0
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

			// Re-add overlap pages from circular buffer
			if overlapCount > 0 {
				for j := 0; j < overlapCount; j++ {
					idx := (overlapIndex - overlapCount + j + g.overlapPages) % g.overlapPages
					overlapPwt := overlapBuffer[idx]
					if currentText.Len() > 0 {
						currentText.WriteByte('\n')
					}
					currentText.WriteString(overlapPwt.page.Text)
					currentTokens += overlapPwt.tokens
				}
				currentGroup = &PageGroup{
					StartPage: overlapBuffer[(overlapIndex-overlapCount+g.overlapPages)%g.overlapPages].page.Number,
				}
			}
			overlapIndex = 0
			overlapCount = 0
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

		// Add to circular buffer
		overlapBuffer[overlapIndex] = pwt
		overlapIndex = (overlapIndex + 1) % g.overlapPages
		if overlapCount < g.overlapPages {
			overlapCount++
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
