package indexer

import (
	"strconv"
	"strings"
	"unicode"
)

// mergeTOCItems merges additional TOC items into existing items with deduplication
// Deduplicates based on structure field OR title, keeping the first occurrence
func (mp *MetaProcessor) mergeTOCItems(existing, additional []TOCItem) []TOCItem {
	seenStructures := make(map[string]bool)
	seenTitles := make(map[string]bool)

	// Build set of existing structures and titles
	for _, item := range existing {
		if item.Structure != "" {
			seenStructures[normalizeStructure(item.Structure)] = true
		}
		// Normalize title for comparison (trim spaces)
		if item.Title != "" {
			seenTitles[strings.TrimSpace(item.Title)] = true
		}
	}

	merged := make([]TOCItem, len(existing))
	copy(merged, existing)

	for _, item := range additional {
		shouldSkip := false

		// Check structure duplication
		if item.Structure != "" {
			normalized := normalizeStructure(item.Structure)
			if seenStructures[normalized] {
				shouldSkip = true
			} else {
				seenStructures[normalized] = true
				item.Structure = normalized
			}
		}

		// Check title duplication (only if not already skipped)
		if !shouldSkip && item.Title != "" {
			title := strings.TrimSpace(item.Title)
			if seenTitles[title] {
				shouldSkip = true
			} else {
				seenTitles[title] = true
			}
		}

		if !shouldSkip {
			merged = append(merged, item)
		}
	}

	return merged
}

// normalizeStructure normalizes a structure string to a consistent format
// Removes leading/trailing spaces, normalizes multiple dots, removes leading zeros
// Fixes common LLM errors: "31" -> "3.1", "321" -> "3.2.1", etc.
// Examples: " 1.1 " -> "1.1", "01.02" -> "1.2", "31" -> "3.1", "1.." -> "1"
func normalizeStructure(structure string) string {
	if structure == "" {
		return ""
	}

	// Trim spaces
	structure = strings.TrimSpace(structure)

	// Remove any non-digit characters except dots
	cleaned := make([]rune, 0, len(structure))
	for _, r := range structure {
		if unicode.IsDigit(r) || r == '.' {
			cleaned = append(cleaned, r)
		}
	}
	structure = string(cleaned)

	// Fix common LLM error: no dots between numbers (e.g., "31" -> "3.1", "321" -> "3.2.1")
	// Only split if length > 2 to avoid breaking two-digit chapter numbers (e.g., "10" -> keep as "10")
	if !strings.Contains(structure, ".") && len(structure) > 2 {
		// Split into individual digits and join with dots
		digits := strings.Split(structure, "")
		structure = strings.Join(digits, ".")
	}

	// Split by dot
	parts := strings.Split(structure, ".")
	normalized := make([]string, 0, len(parts))

	for _, part := range parts {
		if part == "" {
			continue // Skip empty parts (from multiple dots)
		}
		// Remove leading zeros and convert to integer then back to string
		if num, err := strconv.Atoi(part); err == nil {
			normalized = append(normalized, strconv.Itoa(num))
		} else {
			// If not a number, keep the original (after trimming)
			normalized = append(normalized, strings.TrimSpace(part))
		}
	}

	return strings.Join(normalized, ".")
}


