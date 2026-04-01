package indexer

import (
	"sort"
	"strings"
	"unicode"
)

const minMatchPairs = 3

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

func (mp *MetaProcessor) validateAndTruncatePhysicalIndices(items []TOCItem, totalPages int, _ int) []TOCItem {
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

func (mp *MetaProcessor) extractMatchingPagePairs(tocWithPages []TOCItem, tocWithPhysical []TOCItem, _ int) []PageIndexPair {
	pairs := make([]PageIndexPair, 0)

	for _, phyItem := range tocWithPhysical {
		if phyItem.PhysicalIndex == nil {
			continue
		}
		for _, pageItem := range tocWithPages {
			if titlesMatch(phyItem.Title, pageItem.Title) && pageItem.Page != nil {
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

func (mp *MetaProcessor) calculatePageOffset(pairs []PageIndexPair, totalPages int) *int {
	if len(pairs) == 0 {
		return nil
	}

	medianOffset := mp.calculateMedianOffset(pairs, totalPages)
	if medianOffset != nil {
		return medianOffset
	}

	differences := make(map[int]int)
	for _, pair := range pairs {
		diff := pair.PhysicalIndex - pair.Page
		differences[diff]++
	}

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

func (mp *MetaProcessor) calculateMedianOffset(pairs []PageIndexPair, totalPages int) *int {
	if len(pairs) < minMatchPairs {
		return nil
	}

	diffs := make([]int, len(pairs))
	for i, pair := range pairs {
		diffs[i] = pair.PhysicalIndex - pair.Page
	}

	sort.Ints(diffs)

	var median int
	n := len(diffs)
	if n%2 == 0 {
		median = (diffs[n/2-1] + diffs[n/2]) / 2
	} else {
		median = diffs[n/2]
	}

	if abs(median) > totalPages/2 {
		return nil
	}

	return &median
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// normalizeTitleForComparison normalizes title for string comparison (lowercase, no punctuation)
func normalizeTitleForComparison(title string) string {
	title = strings.TrimSpace(title)
	title = strings.ToLower(title)
	var result strings.Builder
	for _, r := range title {
		if !unicode.IsPunct(r) {
			result.WriteRune(r)
		}
	}
	normalized := result.String()
	normalized = strings.Join(strings.Fields(normalized), " ")
	return normalized
}

// cleanTitleForOutput cleans title for output (removes newlines, invalid chars, truncates long titles)
func cleanTitleForOutput(title string) string {
	if title == "" {
		return ""
	}

	// Remove all newline characters
	title = strings.ReplaceAll(title, "\n", " ")
	title = strings.ReplaceAll(title, "\r", " ")

	// Truncate at first dash or bullet point that starts a new line
	if idx := strings.Index(title, " - "); idx > 0 {
		title = title[:idx]
	}
	if idx := strings.Index(title, " — "); idx > 0 {
		title = title[:idx]
	}

	// Convert to rune array for safe UTF-8 handling
	runes := []rune(title)
	valid := make([]rune, 0, len(runes))

	// Remove invalid UTF-8 characters and replacement characters
	for _, r := range runes {
		if r == 0xFFFD || !unicode.IsPrint(r) { // Skip replacement char and non-printable chars
			continue
		}
		valid = append(valid, r)
	}

	// Trim extra spaces from rune array
	start := 0
	for start < len(valid) && unicode.IsSpace(valid[start]) {
		start++
	}
	end := len(valid) - 1
	for end >= start && unicode.IsSpace(valid[end]) {
		end--
	}
	if start > end {
		return ""
	}
	valid = valid[start : end+1]

	// Truncate long titles (max 30 runes/characters)
	maxRunes := 30
	if len(valid) > maxRunes {
		// Try to truncate at last space before max length
		truncateAt := maxRunes
		for i := maxRunes - 1; i > maxRunes/2; i-- {
			if unicode.IsSpace(valid[i]) || valid[i] == '、' || valid[i] == '，' || valid[i] == '：' {
				truncateAt = i
				break
			}
		}
		valid = valid[:truncateAt]

		// Trim trailing punctuation and spaces
		for len(valid) > 0 && (unicode.IsPunct(valid[len(valid)-1]) || unicode.IsSpace(valid[len(valid)-1])) {
			valid = valid[:len(valid)-1]
		}
	}

	// Convert back to string and final cleanup
	title = string(valid)
	title = strings.TrimSuffix(title, ":")
	title = strings.TrimSuffix(title, "：")
	title = strings.TrimSpace(title)

	return title
}

func levenshteinDistance(s1, s2 string, max int) int {
	len1, len2 := len(s1), len(s2)

	if abs(len1-len2) > max {
		return max + 1
	}

	if len1 == 0 {
		return len2
	}
	if len2 == 0 {
		return len1
	}

	if len1 > len2 {
		s1, s2 = s2, s1
		len1, len2 = len2, len1
	}

	column := make([]int, len1+1)
	for i := range column {
		column[i] = i
	}

	for j := 1; j <= len2; j++ {
		diagonalUpLeft := column[0]
		column[0] = j

		for i := 1; i <= len1; i++ {
			substitutionCost := 0
			if s1[i-1] != s2[j-1] {
				substitutionCost = 1
			}

			current := column[i]
			column[i] = min(
				column[i]+1,
				min(
					column[i-1]+1,
					diagonalUpLeft+substitutionCost,
				),
			)

			diagonalUpLeft = current
		}

		if column[1] > max {
			return max + 1
		}
	}

	return column[len1]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func countDigitChanges(s1, s2 string) int {
	count := 0
	minLen := len(s1)
	if len(s2) < minLen {
		minLen = len(s2)
	}

	for i := 0; i < minLen; i++ {
		if isDigitByte(s1[i]) && isDigitByte(s2[i]) && s1[i] != s2[i] {
			count++
		}
	}

	return count
}

// detectDuplicatePhysicalIndices detects duplicate PhysicalIndex values in TOC items
// func (mp *MetaProcessor) detectDuplicatePhysicalIndices(items []TOCItem) map[int][]string {
// 	duplicates := make(map[int][]string)
// 	physicalIndexMap := make(map[int][]string)

// 	for _, item := range items {
// 		if item.PhysicalIndex != nil {
// 			idx := *item.PhysicalIndex
// 			physicalIndexMap[idx] = append(physicalIndexMap[idx], item.Title)
// 		}
// 	}

// 	for idx, titles := range physicalIndexMap {
// 		if len(titles) > 1 {
// 			duplicates[idx] = titles
// 		}
// 	}

// 	return duplicates
// }

func isDigitByte(c byte) bool {
	return c >= '0' && c <= '9'
}

func titlesMatch(title1, title2 string) bool {
	normalized1 := normalizeTitleForComparison(title1)
	normalized2 := normalizeTitleForComparison(title2)

	if normalized1 == normalized2 {
		return true
	}

	maxDistance := 3
	distance := levenshteinDistance(normalized1, normalized2, maxDistance)
	if distance > maxDistance {
		return false
	}

	digitChanges := countDigitChanges(normalized1, normalized2)
	return digitChanges <= 1
}
