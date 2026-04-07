package indexer

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"sync"

	"golang.org/x/sync/errgroup"

	"github.com/rs/zerolog/log"
	"github.com/xgsong/mypageindexgo/pkg/prompts"
)

const (
	minTOCAccuracy     = 0.6 // Accept TOC if 60%+ items verified
	lowAccuracyWarning = 0.3 // Warn if accuracy below 30%
)

// verifyTOC verifies TOC accuracy using check_title_appearance approach.
// Python: verify_toc in page_index.py:900-952
// Note: error return value is always nil, kept for interface compatibility
func (mp *MetaProcessor) verifyTOC(ctx context.Context, pageTexts []string, items []TOCItem, startIndex int) (float64, []TOCItem, error) { //nolint:unparam
	if len(items) == 0 {
		return 0, nil, nil
	}

	// Early exit: if last physical_index < totalPages/2, return accuracy=0
	// Python: page_index.py:910
	lastPhysicalIndex := 0
	for i := len(items) - 1; i >= 0; i-- {
		if items[i].PhysicalIndex != nil {
			lastPhysicalIndex = *items[i].PhysicalIndex
			break
		}
	}

	if lastPhysicalIndex == 0 || lastPhysicalIndex < len(pageTexts)/2 {
		log.Debug().
			Int("lastPhysicalIndex", lastPhysicalIndex).
			Int("totalPages", len(pageTexts)).
			Msg("lastPhysicalIndex < totalPages/2, accepting TOC without verification")
		return 1.0, items, nil
	}

	// Check all items concurrently using check_title_appearance
	ac := NewAppearanceChecker(mp.llmClient, mp.cfg)

	type verifyResult struct {
		itemIndex int
		appears   bool
		item      TOCItem
	}

	results := make([]verifyResult, 0, len(items))
	var mu sync.Mutex

	// Use errgroup to limit concurrency and handle errors properly
	// Limit to avoid overwhelming LLM with too many concurrent requests
	eg, ctx := errgroup.WithContext(ctx)
	verifyConcurrency := min(runtime.NumCPU()*2, 10)
	eg.SetLimit(verifyConcurrency)

	for i, item := range items {
		if item.PhysicalIndex == nil {
			continue
		}

		i := i
		item := item
		eg.Go(func() error {
			itemCopy := item
			itemCopy.ListIndex = ptr(i)
			appears, err := ac.CheckTitleAppearance(ctx, itemCopy, pageTexts, startIndex)
			if err != nil {
				return err
			}

			mu.Lock()
			results = append(results, verifyResult{itemIndex: i, appears: appears, item: itemCopy})
			mu.Unlock()
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		log.Warn().Err(err).Msg("TOC verification encountered errors")
	}

	correctCount := 0
	var incorrectItems []TOCItem
	for _, r := range results {
		if r.appears {
			correctCount++
		} else {
			incorrectItems = append(incorrectItems, r.item)
		}
	}

	checkedCount := correctCount + len(incorrectItems)
	if checkedCount == 0 {
		return 0, nil, nil
	}

	accuracy := float64(correctCount) / float64(checkedCount)

	log.Debug().
		Int("correctCount", correctCount).
		Int("checkedCount", checkedCount).
		Float64("accuracy", accuracy).
		Msg("TOC verification complete")

	switch {
	case accuracy >= minTOCAccuracy:
		log.Debug().
			Int("incorrectCount", len(incorrectItems)).
			Msg("TOC accuracy >= threshold, returning all items for fixing")
		return accuracy, incorrectItems, nil

	case accuracy >= lowAccuracyWarning:
		log.Warn().
			Float64("accuracy", accuracy).
			Int("correctCount", correctCount).
			Int("incorrectCount", len(incorrectItems)).
			Msg("TOC accuracy borderline, returning all items without fixing")
		return accuracy, incorrectItems, nil

	default:
		log.Warn().
			Float64("accuracy", accuracy).
			Msg("TOC accuracy too low, triggering fallback mode")
		return accuracy, nil, nil
	}
}

func (mp *MetaProcessor) fixIncorrectTOCWithRetries(ctx context.Context, items []TOCItem, pageTexts []string, incorrectItems []TOCItem, startIndex int, maxRetries int) ([]TOCItem, []TOCItem, error) {
	currentItems := items
	currentIncorrect := incorrectItems

	for attempt := 0; attempt < maxRetries && len(currentIncorrect) > 0; attempt++ {
		newItems, stillIncorrect, err := mp.fixIncorrectTOC(ctx, currentItems, pageTexts, startIndex, currentIncorrect)
		if err != nil {
			return currentItems, currentIncorrect, err
		}
		currentItems = newItems
		currentIncorrect = stillIncorrect
	}

	return currentItems, currentIncorrect, nil
}

func (mp *MetaProcessor) fixIncorrectTOC(ctx context.Context, items []TOCItem, pageTexts []string, startIndex int, incorrectItems []TOCItem) ([]TOCItem, []TOCItem, error) { //nolint:unparam // error return value is always nil, kept for interface compatibility
	// Create set of incorrect indices
	incorrectSet := make(map[int]bool)
	for _, item := range incorrectItems {
		incorrectSet[*item.ListIndex] = true
	}

	// Fix each incorrect item
	stillIncorrect := make([]TOCItem, 0)

	for _, item := range incorrectItems {
		newItem, err := mp.fixSingleItem(ctx, item, items, incorrectSet, pageTexts, startIndex)
		if err != nil {
			stillIncorrect = append(stillIncorrect, item)
			continue
		}

		// Update the item in the main list
		for i := range items {
			if items[i].ListIndex == newItem.ListIndex {
				items[i] = newItem
				break
			}
		}
	}

	return items, stillIncorrect, nil
}

func (mp *MetaProcessor) fixSingleItem(ctx context.Context, incorrectItem TOCItem, allItems []TOCItem, incorrectSet map[int]bool, pageTexts []string, startIndex int) (TOCItem, error) {
	endIndex := len(pageTexts) + startIndex - 1

	// Find previous correct item
	prevCorrect := startIndex - 1
	for i := *incorrectItem.ListIndex - 1; i >= 0; i-- {
		if !incorrectSet[i] && i < len(allItems) {
			if allItems[i].PhysicalIndex != nil {
				prevCorrect = *allItems[i].PhysicalIndex
				break
			}
		}
	}

	// Find next correct item
	nextCorrect := endIndex
	for i := *incorrectItem.ListIndex + 1; i < len(allItems); i++ {
		if !incorrectSet[i] {
			if allItems[i].PhysicalIndex != nil {
				nextCorrect = *allItems[i].PhysicalIndex
				break
			}
		}
	}

	// Build content for search range
	var content strings.Builder
	for pageNum := prevCorrect; pageNum <= nextCorrect && pageNum <= endIndex; pageNum++ {
		pageIdx := pageNum - startIndex
		if pageIdx >= 0 && pageIdx < len(pageTexts) {
			content.WriteString(addPageTags(pageTexts[pageIdx], pageNum))
		}
	}

	// Ask LLM to find the section
	// Python: single_toc_item_index_fixer in page_index.py:740-756
	prompt := prompts.FindSectionLocationPrompt(incorrectItem.Title, content.String())

	response, err := mp.llmClient.GenerateSimple(ctx, prompt)
	if err != nil {
		return incorrectItem, err
	}

	var result struct {
		PhysicalIndex string `json:"physical_index"`
	}
	if err := parseLLMJSONResponse(response, &result); err != nil {
		return incorrectItem, err
	}

	idx, err := convertPhysicalIndexToInt(result.PhysicalIndex)
	if err != nil {
		return incorrectItem, err
	}

	// Python: verify the fix with check_title_appearance (page_index.py:826-834)
	fixedItem := incorrectItem
	fixedItem.PhysicalIndex = &idx

	ac := NewAppearanceChecker(mp.llmClient, mp.cfg)
	isValid, err := ac.CheckTitleAppearance(ctx, fixedItem, pageTexts, startIndex)
	if err != nil || !isValid {
		return incorrectItem, fmt.Errorf("fix verification failed for %s", incorrectItem.Title)
	}

	return fixedItem, nil
}
