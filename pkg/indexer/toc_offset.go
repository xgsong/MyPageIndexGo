package indexer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xgsong/mypageindexgo/pkg/prompts"
)

// addPhysicalIndexToTOC asks LLM to add physical_index to TOC items based on document content
func (d *TOCDetector) addPhysicalIndexToTOC(ctx context.Context, toc []TOCItem, pages []string, startIndex int) ([]TOCItem, error) {
	if len(toc) == 0 {
		return toc, nil
	}

	content := buildContentWithTags(pages, startIndex)
	tocJSON, _ := json.Marshal(toc)
	prompt := prompts.TOCIndexExtractorPrompt(string(tocJSON), content)

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
			items = append(items, TOCItem{
				Structure:     entry.Structure,
				Title:         cleanTitleForOutput(entry.Title),
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

	return items, nil
}
