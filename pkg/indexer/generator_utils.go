package indexer

import (
	"regexp"
	"strings"
)

func ptr[T any](v T) *T {
	return &v
}

func normalizeArabicToChinese(s string) string {
	arabicToChinese := map[rune]rune{
		'1': '一',
		'2': '二',
		'3': '三',
		'4': '四',
		'5': '五',
		'6': '六',
		'7': '七',
		'8': '八',
		'9': '九',
	}

	if s == "10" {
		return "十"
	}

	if len(s) == 2 && s[0] == '1' {
		return "十" + string(arabicToChinese[rune(s[1])])
	}

	if len(s) == 2 && s[1] == '0' {
		return string(arabicToChinese[rune(s[0])]) + "十"
	}

	if len(s) == 2 {
		return string(arabicToChinese[rune(s[0])]) + "十" + string(arabicToChinese[rune(s[1])])
	}

	if len(s) == 1 {
		return string(arabicToChinese[rune(s[0])])
	}

	return s
}

func isChapterTitle(title string) bool {
	if title == "" {
		return false
	}
	chapterPattern := regexp.MustCompile(`^第[零一二三四五六七八九十百千万\d]+章`)
	return chapterPattern.MatchString(title)
}

func extractContentPreview(pageTextMap map[int]string, startPage, endPage int, maxChars int) string {
	if pageTextMap == nil || startPage > endPage {
		return ""
	}

	var content strings.Builder
	charsCollected := 0

	for pageNum := startPage; pageNum <= endPage && charsCollected < maxChars; pageNum++ {
		if text, ok := pageTextMap[pageNum]; ok && text != "" {
			trimmed := strings.TrimSpace(text)
			if len(trimmed) < 50 {
				continue
			}

			remaining := maxChars - charsCollected
			if len(text) <= remaining {
				content.WriteString(text)
				charsCollected += len(text)
			} else {
				content.WriteString(text[:remaining])
				charsCollected = maxChars
			}

			if charsCollected < maxChars {
				content.WriteString(" ")
			}
		}
	}

	preview := strings.TrimSpace(content.String())
	if len(preview) > maxChars {
		if lastSpace := strings.LastIndex(preview[:maxChars], " "); lastSpace > maxChars/2 {
			preview = preview[:lastSpace] + "..."
		} else {
			preview = preview[:maxChars-3] + "..."
		}
	} else if preview != "" {
		preview += "..."
	}

	return preview
}

func enrichTitleWithPreview(title string, preview string) string {
	return title
}
