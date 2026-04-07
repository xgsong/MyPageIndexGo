package indexer

import (
	"regexp"
	"sort"
	"strings"
)

func scanAndAddMissingSubsections(tocItems []TOCItem, pageTexts []string, startIndex int) []TOCItem {
	subsectionPattern := regexp.MustCompile(`###\s*(\d+\.\d+)\s*(.+)`)

	existingItems := make(map[string]*TOCItem)
	for i := range tocItems {
		existingItems[tocItems[i].Structure] = &tocItems[i]
	}

	var addedItems []TOCItem

	for pageNum, text := range pageTexts {
		matches := subsectionPattern.FindAllStringSubmatch(text, -1)
		for _, match := range matches {
			if len(match) < 3 {
				continue
			}
			structure := match[1]
			title := strings.TrimSpace(match[2])
			actualPage := pageNum + startIndex

			if existing, found := existingItems[structure]; found {
				if *existing.PhysicalIndex != actualPage {
					existing.PhysicalIndex = &actualPage
					existing.Page = &actualPage
				}
			} else {
				addedItems = append(addedItems, TOCItem{
					Structure:     structure,
					Title:         title,
					PhysicalIndex: &actualPage,
					Page:          &actualPage,
					ListIndex:     ptr(len(tocItems) + len(addedItems)),
				})
				existingItems[structure] = &addedItems[len(addedItems)-1]
			}
		}
	}

	if len(addedItems) > 0 {
		merged := make([]TOCItem, len(tocItems)+len(addedItems))
		copy(merged, tocItems)
		copy(merged[len(tocItems):], addedItems)
		return merged
	}

	return tocItems
}

func getParentStructure(structure string) string {
	if structure == "" {
		return ""
	}
	parts := strings.Split(structure, ".")
	if len(parts) <= 1 {
		return ""
	}
	return strings.Join(parts[:len(parts)-1], ".")
}

func prepareTOCItems(items []TOCItem) []TOCItem {
	for i := range items {
		if items[i].PhysicalIndex == nil && items[i].Page != nil {
			items[i].PhysicalIndex = items[i].Page
		}
	}
	return items
}

func sortTOCItemsByPage(items []TOCItem) []TOCItem {
	sort.Slice(items, func(i, j int) bool {
		if items[i].PhysicalIndex == nil {
			return false
		}
		if items[j].PhysicalIndex == nil {
			return true
		}
		if *items[i].PhysicalIndex != *items[j].PhysicalIndex {
			return *items[i].PhysicalIndex < *items[j].PhysicalIndex
		}
		if items[i].ListIndex != nil && items[j].ListIndex != nil {
			return *items[i].ListIndex < *items[j].ListIndex
		}
		if items[i].ListIndex != nil {
			return true
		}
		if items[j].ListIndex != nil {
			return false
		}
		return false
	})
	return items
}

func calculatePageRanges(items []TOCItem, totalPages int) []TOCItem {
	for i := range items {
		if items[i].PhysicalIndex == nil {
			continue
		}

		startPage := *items[i].PhysicalIndex
		nextDifferentPage := -1
		for j := i + 1; j < len(items); j++ {
			if items[j].PhysicalIndex != nil && *items[j].PhysicalIndex > startPage {
				nextDifferentPage = *items[j].PhysicalIndex
				break
			}
		}

		if nextDifferentPage > startPage {
			items[i].EndPage = ptr(nextDifferentPage)
		} else {
			items[i].EndPage = ptr(totalPages)
		}

		samePageNext := false
		if i < len(items)-1 {
			nextItem := items[i+1]
			if nextItem.PhysicalIndex != nil && *nextItem.PhysicalIndex == startPage {
				samePageNext = true
			}
		}
		if samePageNext {
			items[i].EndPage = ptr(startPage)
		}
	}
	return items
}
