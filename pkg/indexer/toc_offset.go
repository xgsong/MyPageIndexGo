package indexer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog/log"
)

// calculatePageOffset calculates the offset between physical page numbers and logical page numbers
func calculatePageOffset(toc *TOCResult) (int, error) {
	if len(toc.Items) == 0 {
		return 0, nil
	}

	// Find the first TOC entry with a page number and physical index
	for _, item := range toc.Items {
		if item.Page != nil && item.PhysicalIndex != nil {
			// Calculate offset: physical_index - page_number
			offset := *item.PhysicalIndex - *item.Page
			log.Info().
				Int("physical_index", *item.PhysicalIndex).
				Int("page_number", *item.Page).
				Int("offset", offset).
				Msg("Calculated page offset from first valid TOC entry")
			return offset, nil
		}
	}

	// If no entry with both page and physical index, assume no offset
	log.Info().Msg("No TOC entry with both page and physical index found, assuming no offset")
	return 0, nil
}

// addPageOffsetToTOC adds page offset to TOC items by converting physical_index to logical page numbers
func addPageOffsetToTOC(toc *TOCResult, offset int) {
	if len(toc.Items) == 0 {
		return
	}

	log.Info().Int("offset", offset).Msg("Adding page offset to TOC")

	for i := range toc.Items {
		item := &toc.Items[i]
		if item.PhysicalIndex != nil {
			// Calculate logical page number: physical_index - offset
			logicalPage := *item.PhysicalIndex - offset
			item.Page = &logicalPage

			log.Debug().
				Str("title", item.Title).
				Int("physical_index", *item.PhysicalIndex).
				Int("logical_page", logicalPage).
				Msg("Mapped physical index to logical page")
		}
	}
}

// tocIndexExtractorPrompt creates prompt for adding physical index to TOC
func tocIndexExtractorPrompt(toc []TOCItem, content string) string {
	tocJSON, _ := json.Marshal(toc)
	return fmt.Sprintf(`You are given a table of contents in a json format and several pages of a document, your job is to add the physical_index to the table of contents in the json format.

The provided pages contains tags like <physical_index_X> and <physical_index_X> to indicate the physical location of the page X.

The structure variable is the numeric system which represents the index of the hierarchy section in the table of contents. For example, the first section has structure index 1, the first subsection has structure index 1.1, the second subsection has structure index 1.2, etc.

The response should be in the following JSON format:
[
    {
        "structure": "structure index, x.x.x or None (string)",
        "title": "title of the section",
        "physical_index": "<physical_index_X>" (keep the format)
    }
]

Only add the physical_index to the sections that are in the provided pages.
If the section is not in the provided pages, do not add the physical_index to it.
Directly return the final JSON structure. Do not output anything else.

Table of contents:
%s

Document pages:
%s`, string(tocJSON), content)
}

// addPhysicalIndexToTOC asks LLM to add physical_index to TOC items based on document content
func (d *TOCDetector) addPhysicalIndexToTOC(ctx context.Context, toc []TOCItem, pages []string, startIndex int) ([]TOCItem, error) {
	if len(toc) == 0 {
		return toc, nil
	}

	// Build content with physical index tags
	content := buildContentWithTags(pages, startIndex)

	prompt := tocIndexExtractorPrompt(toc, content)

	response, err := d.llmClient.GenerateSimple(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to add physical index to TOC: %w", err)
	}

	var result []TOCIndexExtractorResult
	if err := parseLLMJSONResponse(response, &result); err != nil {
		return nil, fmt.Errorf("failed to parse physical index response: %w", err)
	}

	// Convert result back to TOCItem with physical_index
	var items []TOCItem
	for _, entry := range result {
		// Convert interface{} to string first
		physicalIndexStr := entry.GetPhysicalIndexAsString()
		physicalIndex, err := convertPhysicalIndexToInt(physicalIndexStr)
		if err != nil {
			log.Warn().Err(err).Str("value", physicalIndexStr).Msg("Failed to convert physical index")
			items = append(items, TOCItem{
				Structure:     entry.Structure,
				Title:         entry.Title,
				PhysicalIndex: nil,
			})
		} else {
			items = append(items, TOCItem{
				Structure:     entry.Structure,
				Title:         entry.Title,
				PhysicalIndex: &physicalIndex,
			})
		}
	}

	log.Info().Int("items", len(items)).Msg("Added physical index to TOC items")
	return items, nil
}
