package indexer

import (
	"context"
	"strings"
	"sync"

	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/llm"
	"github.com/xgsong/mypageindexgo/pkg/prompts"
)

// AppearanceChecker checks if TOC items appear at the start of their pages.
// Python: check_title_appearance_in_start_concurrent in page_index.py:74-101
type AppearanceChecker struct {
	llmClient           llm.LLMClient
	skipAppearanceCheck bool
}

// NewAppearanceChecker creates a new AppearanceChecker.
func NewAppearanceChecker(client llm.LLMClient, cfg *config.Config) *AppearanceChecker {
	skipCheck := false
	if cfg != nil {
		skipCheck = cfg.SkipAppearanceCheck
	}
	return &AppearanceChecker{
		llmClient:           client,
		skipAppearanceCheck: skipCheck,
	}
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
	prompt := prompts.TitleAppearancePrompt(item.Title, pageText)

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
	prompt := prompts.TitleAppearanceInStartPrompt(title, pageText)

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
// OPTIMIZED: If skipAppearanceCheck is true, skip LLM calls and set all to "yes".
func (ac *AppearanceChecker) CheckAllItemsAppearanceInStart(ctx context.Context, items []TOCItem, pageTexts []string) []TOCItem {
	if ac.skipAppearanceCheck {
		for i := range items {
			items[i].AppearStart = "yes"
		}
		return items
	}

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
