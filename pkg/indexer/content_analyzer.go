package indexer

import (
	"regexp"
	"strings"
)

func analyzeSectionEndPage(pageTextMap map[int]string, startPage int, title string, allTOCItems []TOCItem) int {
	if pageTextMap == nil || startPage < 1 {
		return startPage
	}

	maxPages := len(pageTextMap)

	nextSectionNum := findNextSectionNumberInTOC(title, allTOCItems)

	if nextSectionNum == "" {
		nextSectionTitle := ""
		for _, item := range allTOCItems {
			if item.Title == title || (item.PhysicalIndex != nil && *item.PhysicalIndex > startPage) {
				if item.PhysicalIndex != nil && *item.PhysicalIndex > startPage {
					return *item.PhysicalIndex
				}
			}
		}
		_ = nextSectionTitle
		return maxPages
	}

	nextSectionTitle := findSectionTitleByNum(nextSectionNum, allTOCItems)

	for currentPage := startPage; currentPage <= maxPages; currentPage++ {
		text, exists := pageTextMap[currentPage]
		if !exists || strings.TrimSpace(text) == "" {
			if currentPage == startPage {
				return currentPage
			}
			return currentPage - 1
		}

		if currentPage > startPage {
			if nextSectionTitle != "" && strings.Contains(text, nextSectionTitle) {
				return currentPage
			}
		}
	}

	if nextSectionNum != "" {
		for _, item := range allTOCItems {
			itemNum := extractSectionNumber(item.Title)
			if itemNum == nextSectionNum && item.PhysicalIndex != nil {
				return *item.PhysicalIndex
			}
		}
	}

	return maxPages
}

func findNextSectionNumberInTOC(title string, allTOCItems []TOCItem) string {
	currentNum := extractSectionNumber(title)
	if currentNum == "" {
		return ""
	}

	currentParts := strings.Split(currentNum, ".")
	if len(currentParts) != 2 {
		return ""
	}

	for _, item := range allTOCItems {
		itemNum := extractSectionNumber(item.Title)
		if itemNum == "" {
			continue
		}

		itemParts := strings.Split(itemNum, ".")
		if len(itemParts) != 2 {
			continue
		}

		if itemParts[0] == currentParts[0] && itemParts[1] > currentParts[1] {
			return itemNum
		}
	}

	return ""
}

func findSectionTitleByNum(sectionNum string, allTOCItems []TOCItem) string {
	for _, item := range allTOCItems {
		itemNum := extractSectionNumber(item.Title)
		if itemNum == sectionNum {
			return item.Title
		}
	}
	return ""
}

func findNextSectionNumber(currentSection string, allTOCItems []TOCItem) string {
	if len(allTOCItems) == 0 {
		return ""
	}

	parts := strings.Split(currentSection, ".")
	if len(parts) != 2 {
		return ""
	}

	currentIdx := -1
	for i, item := range allTOCItems {
		itemNum := extractSectionNumber(item.Title)
		if itemNum == currentSection {
			currentIdx = i
			break
		}
	}

	if currentIdx == -1 {
		return ""
	}

	for i := currentIdx + 1; i < len(allTOCItems); i++ {
		item := allTOCItems[i]
		itemNum := extractSectionNumber(item.Title)
		if itemNum == "" {
			continue
		}

		itemParts := strings.Split(itemNum, ".")
		if len(itemParts) != 2 {
			continue
		}

		if itemParts[0] > parts[0] {
			return itemNum
		}
		if itemParts[0] == parts[0] && itemParts[1] > parts[1] {
			return itemNum
		}
	}

	return ""
}

func extractSectionNumber(title string) string {
	pattern := regexp.MustCompile(`^(\d+\.\d+)`)
	matches := pattern.FindStringSubmatch(title)
	if len(matches) >= 2 {
		return matches[1]
	}
	return ""
}

func findSectionTitle(sectionNum string, allTOCItems []TOCItem) string {
	for _, item := range allTOCItems {
		itemNum := extractSectionNumber(item.Title)
		if itemNum == sectionNum {
			return item.Title
		}
	}
	return ""
}

func containsSectionTitle(text string, sectionTitle string) bool {
	if text == "" || sectionTitle == "" {
		return false
	}

	cleanText := strings.ReplaceAll(text, "\n", " ")
	cleanText = strings.TrimSpace(cleanText)

	cleanTitle := strings.TrimSpace(sectionTitle)

	return strings.Contains(cleanText, cleanTitle)
}
