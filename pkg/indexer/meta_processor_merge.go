package indexer

import (
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

// TOCItemWithNodes combines TOC item with its child nodes for tree building
type TOCItemWithNodes struct {
	TOCItem
	Children []TOCItemWithNodes
}

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

	log.Info().
		Int("existing_count", len(existing)).
		Int("additional_count", len(additional)).
		Int("known_structures", len(seenStructures)).
		Int("known_titles", len(seenTitles)).
		Msg("Merging TOC items")

	merged := make([]TOCItem, len(existing))
	copy(merged, existing)

	for _, item := range additional {
		shouldSkip := false
		skipReason := ""

		// Check structure duplication
		if item.Structure != "" {
			normalized := normalizeStructure(item.Structure)
			if seenStructures[normalized] {
				shouldSkip = true
				skipReason = "duplicate structure"
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
				skipReason = "duplicate title"
			} else {
				seenTitles[title] = true
			}
		}

		if shouldSkip {
			log.Warn().
				Str("structure", item.Structure).
				Str("title", item.Title).
				Str("reason", skipReason).
				Msg("Skipping duplicate TOC item during merge")
		} else {
			merged = append(merged, item)
		}
	}

	log.Info().
		Int("merged_count", len(merged)).
		Int("removed", len(additional)+len(existing)-len(merged)).
		Msg("TOC merge complete")

	return merged
}

// normalizeStructure normalizes a structure string to a consistent format
// Removes leading/trailing spaces, normalizes multiple dots, removes leading zeros
// Examples: " 1.1 " -> "1.1", "01.02" -> "1.2", "1.." -> "1."
func normalizeStructure(structure string) string {
	if structure == "" {
		return ""
	}

	// Trim spaces
	structure = strings.TrimSpace(structure)

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
