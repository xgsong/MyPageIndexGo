package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// pageListToGroupText groups pages into text chunks with token-based limits.
// Python: page_list_to_group_text in page_index.py:426-459
// Uses averaging formula for uniform distribution.
func (mp *MetaProcessor) pageListToGroupText(pageTexts []string, startIndex int) []string {
	// Build page contents with tags and estimate token counts
	type pageInfo struct {
		content string
		tokens  int
	}
	pages := make([]pageInfo, 0, len(pageTexts))
	totalTokens := 0

	for i, text := range pageTexts {
		pageNum := startIndex + i
		content := addPageTags(text, pageNum)
		// Estimate tokens: ~4 chars per token
		tokens := len(content) / 4
		pages = append(pages, pageInfo{content: content, tokens: tokens})
		totalTokens += tokens
	}

	maxTokens := mp.cfg.MaxTokensPerNode
	if maxTokens <= 0 {
		maxTokens = 20000
	}

	// If all content fits in one group, return as single group
	if totalTokens <= maxTokens {
		var all strings.Builder
		for _, p := range pages {
			all.WriteString(p.content)
		}
		return []string{all.String()}
	}

	// Python averaging formula for uniform distribution
	expectedPartsNum := (totalTokens + maxTokens - 1) / maxTokens // math.Ceil equivalent
	averageTokensPerPart := ((totalTokens / expectedPartsNum) + maxTokens + 1) / 2

	overlapPage := 1
	groups := make([]string, 0)
	var currentGroup strings.Builder
	currentTokenCount := 0

	for i, p := range pages {
		if currentTokenCount+p.tokens > averageTokensPerPart {
			if currentGroup.Len() > 0 {
				groups = append(groups, currentGroup.String())
			}
			// Start new group from overlap
			overlapStart := i - overlapPage
			if overlapStart < 0 {
				overlapStart = 0
			}
			currentGroup.Reset()
			currentTokenCount = 0
			for j := overlapStart; j < i; j++ {
				currentGroup.WriteString(pages[j].content)
				currentTokenCount += pages[j].tokens
			}
		}
		currentGroup.WriteString(p.content)
		currentTokenCount += p.tokens
	}

	if currentGroup.Len() > 0 {
		groups = append(groups, currentGroup.String())
	}

	return groups
}

func (mp *MetaProcessor) splitContentIntoGroups(content string, maxTokens int, _ int) []string {
	maxChars := maxTokens * 4
	groups := make([]string, 0)

	for len(content) > 0 {
		if len(content) <= maxChars {
			groups = append(groups, content)
			break
		}

		breakPoint := maxChars
		if breakPoint < len(content) {
			breakPoint = findValidBreakPoint(content, maxChars)
		}

		groups = append(groups, content[:breakPoint])
		content = content[breakPoint:]
	}

	return groups
}

func findValidBreakPoint(content string, maxChars int) int {
	breakPoint := maxChars
	if breakPoint >= len(content) {
		return len(content)
	}

	for i := breakPoint; i > breakPoint/2 && i > 0; i-- {
		if content[i] == '\n' {
			return i + 1
		}
	}

	for i := breakPoint; i < len(content) && i < maxChars+100; i++ {
		if content[i] == '>' {
			nextI := i + 1
			if nextI < len(content) && (content[nextI] == '\n' || content[nextI] == ' ' || content[nextI] == '<') {
				return i + 1
			}
		}
	}

	for i := breakPoint - 1; i >= breakPoint/2; i-- {
		if content[i] == '>' {
			return i + 1
		}
	}

	return breakPoint
}

// addPageNumberToTOC uses LLM to find where each TOC section appears in the content.
// Python: add_page_number_to_toc in page_index.py:461-491
// Passes the full existing structure so LLM fills in current part only.
func (mp *MetaProcessor) addPageNumberToTOC(ctx context.Context, toc []TOCItem, content string, _ int) []TOCItem {
	if len(toc) == 0 {
		return toc
	}

	structureJSON, _ := json.Marshal(toc)

	prompt := fmt.Sprintf(`You are given an JSON structure of a document and a partial part of the document. Your task is to check if the title that is described in the structure is started in the partial given document.

The provided text contains tags like <physical_index_X> and <physical_index_X> to indicate the physical location of the page X.

If the full target section starts in the partial given document, insert the given JSON structure with the "start": "yes", and "start_index": "<physical_index_X>".

If the full target section does not start in the partial given document, insert "start": "no",  "start_index": None.

The response should be in the following format.
    [
        {
            "structure": "structure index, x.x.x or None (string)",
            "title": "title of the section",
            "start": "yes or no",
            "physical_index": "<physical_index_X> (keep the format)" or None
        },
        ...
    ]
The given structure contains the result of the previous part, you need to fill the result of the current part, do not change the previous result.
Directly return the final JSON structure. Do not output anything else.

Current Partial Document:
%s

Given Structure
%s
`, content, string(structureJSON))

	response, err := mp.llmClient.GenerateSimple(ctx, prompt)
	if err != nil {
		return toc
	}

	// Parse response
	var result []struct {
		Structure     string      `json:"structure"`
		Title         string      `json:"title"`
		Start         string      `json:"start"`
		PhysicalIndex interface{} `json:"physical_index"`
	}

	if err := parseLLMJSONResponse(response, &result); err != nil {
		return toc
	}

	// Update TOC items with found physical indices
	if len(result) == len(toc) {
		for i := range result {
			if result[i].PhysicalIndex != nil {
				physStr := fmt.Sprintf("%v", result[i].PhysicalIndex)
				if idx, err := convertPhysicalIndexToInt(physStr); err == nil {
					toc[i].PhysicalIndex = &idx
				}
			}
		}
	}

	return toc
}

// processNonePageNumbers fills in missing physical_index for items after offset.
// Python: process_none_page_numbers in page_index.py:656-691
func (mp *MetaProcessor) processNonePageNumbers(ctx context.Context, items []TOCItem, pageTexts []string, startIndex int) []TOCItem {
	for i := range items {
		if items[i].PhysicalIndex != nil {
			continue
		}

		// Find previous valid physical_index
		prevPhysicalIndex := 0
		for j := i - 1; j >= 0; j-- {
			if items[j].PhysicalIndex != nil {
				prevPhysicalIndex = *items[j].PhysicalIndex
				break
			}
		}

		// Find next valid physical_index
		nextPhysicalIndex := -1
		for j := i + 1; j < len(items); j++ {
			if items[j].PhysicalIndex != nil {
				nextPhysicalIndex = *items[j].PhysicalIndex
				break
			}
		}
		if nextPhysicalIndex == -1 {
			continue
		}

		// Build page content in range [prev, next]
		var pageContents strings.Builder
		for pageNum := prevPhysicalIndex; pageNum <= nextPhysicalIndex; pageNum++ {
			listIndex := pageNum - startIndex
			if listIndex >= 0 && listIndex < len(pageTexts) {
				pageContents.WriteString(addPageTags(pageTexts[listIndex], pageNum))
			}
		}

		// Ask LLM to find the section location
		itemJSON, _ := json.Marshal(items[i])
		prompt := fmt.Sprintf(`You are given an JSON structure of a document and a partial part of the document. Your task is to check if the title that is described in the structure is started in the partial given document.

The provided text contains tags like <physical_index_X> and <physical_index_X> to indicate the physical location of the page X.

If the full target section starts in the partial given document, insert the given JSON structure with the "start": "yes", and "start_index": "<physical_index_X>".

If the full target section does not start in the partial given document, insert "start": "no", "start_index": None.

The response should be in the following format.
    [
        {
            "structure": "structure index",
            "title": "title of the section",
            "start": "yes or no",
            "physical_index": "<physical_index_X> (keep the format)" or None
        }
    ]
Directly return the final JSON structure. Do not output anything else.

Current Partial Document:
%s

Given Structure
%s
`, pageContents.String(), string(itemJSON))

		response, err := mp.llmClient.GenerateSimple(ctx, prompt)
		if err != nil {
			continue
		}

		var result []struct {
			PhysicalIndex interface{} `json:"physical_index"`
		}
		if err := parseLLMJSONResponse(response, &result); err != nil || len(result) == 0 {
			continue
		}

		if result[0].PhysicalIndex != nil {
			physStr := fmt.Sprintf("%v", result[0].PhysicalIndex)
			if idx, err := convertPhysicalIndexToInt(physStr); err == nil {
				items[i].PhysicalIndex = &idx
				items[i].Page = nil
			}
		}
	}

	return items
}
