package indexer

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
	"github.com/xgsong/mypageindexgo/pkg/llm"
)

// AppearanceChecker checks if TOC items appear at the start of their pages.
// Python: check_title_appearance_in_start_concurrent in page_index.py:74-101
type AppearanceChecker struct {
	llmClient llm.LLMClient
}

// NewAppearanceChecker creates a new AppearanceChecker.
func NewAppearanceChecker(client llm.LLMClient) *AppearanceChecker {
	return &AppearanceChecker{llmClient: client}
}

// CheckTitleAppearance checks if a title appears in the given page.
// Python: check_title_appearance in page_index.py:13-45
func (ac *AppearanceChecker) CheckTitleAppearance(ctx context.Context, item TOCItem, pageTexts []string, startIndex int) (bool, error) {
	if item.PhysicalIndex == nil {
		return false, nil
	}

	pageIdx := *item.PhysicalIndex - startIndex
	if pageIdx < 0 || pageIdx >= len(pageTexts) {
		return false, nil
	}

	pageText := pageTexts[pageIdx]
	prompt := fmt.Sprintf(`Your job is to check if the given section appears or starts in the given page_text.

Note: do fuzzy matching, ignore any space inconsistency in the page_text.

The given section title is %s.
The given page_text is %s.

Reply format:
{
    "answer": "yes or no"
}
Directly return the final JSON structure. Do not output anything else.`, item.Title, pageText)

	response, err := ac.llmClient.GenerateSimple(ctx, prompt)
	if err != nil {
		return false, err
	}

	var result struct {
		Answer string `json:"answer"`
	}
	if err := parseLLMJSONResponse(response, &result); err != nil {
		return false, nil
	}

	return strings.ToLower(result.Answer) == "yes", nil
}

// CheckTitleAppearanceInStart checks if a title appears at the BEGINNING of a page.
// Python: check_title_appearance_in_start in page_index.py:48-71
func (ac *AppearanceChecker) CheckTitleAppearanceInStart(ctx context.Context, title string, pageText string) (string, error) {
	prompt := fmt.Sprintf(`You will be given the current section title and the current page_text.
Your job is to check if the current section starts in the beginning of the given page_text.
If there are other contents before the current section title, then the current section does not start in the beginning of the given page_text.
If the current section title is the first content in the given page_text, then the current section starts in the beginning of the given page_text.

Note: do fuzzy matching, ignore any space inconsistency in the page_text.

The given section title is %s.
The given page_text is %s.

reply format:
{
    "start_begin": "yes or no"
}
Directly return the final JSON structure. Do not output anything else.`, title, pageText)

	response, err := ac.llmClient.GenerateSimple(ctx, prompt)
	if err != nil {
		return "no", err
	}

	var result struct {
		StartBegin string `json:"start_begin"`
	}
	if err := parseLLMJSONResponse(response, &result); err != nil {
		return "no", nil
	}

	if result.StartBegin == "" {
		return "no", nil
	}
	return strings.ToLower(result.StartBegin), nil
}

// CheckAllItemsAppearanceInStart checks appear_start for all TOC items concurrently.
// Python: check_title_appearance_in_start_concurrent in page_index.py:74-101
// Sets AppearStart field on each item: "yes" if section starts at beginning of page.
func (ac *AppearanceChecker) CheckAllItemsAppearanceInStart(ctx context.Context, items []TOCItem, pageTexts []string) []TOCItem {
	log.Info().Int("items", len(items)).Msg("Checking title appearance in start concurrently")

	// Set default "no" for items without physical_index
	for i := range items {
		if items[i].PhysicalIndex == nil {
			items[i].AppearStart = "no"
		}
	}

	// Collect valid items to check
	type checkTask struct {
		index int
		title string
		text  string
	}
	var tasks []checkTask
	for i, item := range items {
		if item.PhysicalIndex != nil {
			pageIdx := *item.PhysicalIndex - 1
			if pageIdx >= 0 && pageIdx < len(pageTexts) {
				tasks = append(tasks, checkTask{
					index: i,
					title: item.Title,
					text:  pageTexts[pageIdx],
				})
			} else {
				items[i].AppearStart = "no"
			}
		}
	}

	// Run concurrently
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, task := range tasks {
		wg.Add(1)
		go func(t checkTask) {
			defer wg.Done()

			result, err := ac.CheckTitleAppearanceInStart(ctx, t.title, t.text)
			if err != nil {
				log.Warn().Err(err).Str("title", t.title).Msg("Error checking start appearance")
				result = "no"
			}

			mu.Lock()
			items[t.index].AppearStart = result
			mu.Unlock()
		}(task)
	}

	wg.Wait()
	return items
}
