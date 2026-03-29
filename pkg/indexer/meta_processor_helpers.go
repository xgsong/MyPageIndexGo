package indexer

import (
	"strings"
)

// Helper functions

func (mp *MetaProcessor) filterValidItems(items []TOCItem) []TOCItem {
	valid := make([]TOCItem, 0, len(items))
	for _, item := range items {
		// Accept items that have either PhysicalIndex or Page set
		if item.PhysicalIndex != nil || item.Page != nil {
			valid = append(valid, item)
		}
	}
	return valid
}

func (mp *MetaProcessor) validateAndTruncatePhysicalIndices(items []TOCItem, totalPages int, startIndex int) []TOCItem {
	for i := range items {
		if items[i].PhysicalIndex != nil {
			idx := *items[i].PhysicalIndex
			// Ensure within bounds
			if idx < 1 {
				idx = 1
			}
			if idx > totalPages {
				idx = totalPages
			}
			items[i].PhysicalIndex = &idx
		}
	}
	return items
}

func (mp *MetaProcessor) deepCopyTOCItems(items []TOCItem) []TOCItem {
	copy := make([]TOCItem, len(items))
	for i, item := range items {
		copy[i] = item
		if item.Page != nil {
			pageCopy := *item.Page
			copy[i].Page = &pageCopy
		}
		if item.PhysicalIndex != nil {
			idxCopy := *item.PhysicalIndex
			copy[i].PhysicalIndex = &idxCopy
		}
	}
	return copy
}

func (mp *MetaProcessor) samplePages(pageTexts []string, startIndex int, maxPages int) string {
	var content strings.Builder
	endIndex := startIndex + maxPages
	if endIndex > len(pageTexts) {
		endIndex = len(pageTexts)
	}
	for i := startIndex - 1; i < endIndex; i++ {
		if i >= 0 && i < len(pageTexts) {
			content.WriteString(addPageTags(pageTexts[i], i+1))
		}
	}
	return content.String()
}

func (mp *MetaProcessor) extractMatchingPagePairs(tocWithPages []TOCItem, tocWithPhysical []TOCItem, startIndex int) []PageIndexPair {
	pairs := make([]PageIndexPair, 0)

	for _, phyItem := range tocWithPhysical {
		if phyItem.PhysicalIndex == nil {
			continue
		}
		for _, pageItem := range tocWithPages {
			if phyItem.Title == pageItem.Title && pageItem.Page != nil {
				pairs = append(pairs, PageIndexPair{
					Title:         pageItem.Title,
					Page:          *pageItem.Page,
					PhysicalIndex: *phyItem.PhysicalIndex,
				})
				break
			}
		}
	}
	return pairs
}

func (mp *MetaProcessor) calculatePageOffset(pairs []PageIndexPair) *int {
	if len(pairs) == 0 {
		return nil
	}

	differences := make(map[int]int)
	for _, pair := range pairs {
		diff := pair.PhysicalIndex - pair.Page
		differences[diff]++
	}

	// Find most common difference
	maxCount := 0
	mostCommon := 0
	for diff, count := range differences {
		if count > maxCount {
			maxCount = count
			mostCommon = diff
		}
	}

	if maxCount > 0 {
		return &mostCommon
	}
	return nil
}

// detectDuplicatePhysicalIndices detects duplicate PhysicalIndex values in TOC items
func (mp *MetaProcessor) detectDuplicatePhysicalIndices(items []TOCItem) map[int][]string {
	duplicates := make(map[int][]string)
	physicalIndexMap := make(map[int][]string)

	for _, item := range items {
		if item.PhysicalIndex != nil {
			idx := *item.PhysicalIndex
			physicalIndexMap[idx] = append(physicalIndexMap[idx], item.Title)
		}
	}

	for idx, titles := range physicalIndexMap {
		if len(titles) > 1 {
			duplicates[idx] = titles
		}
	}

	return duplicates
}
