package indexer

import (
	"context"
	"fmt"
	"strings"
)

// extractTOCContent extracts and cleans TOC content from pages
// Python: extract_toc_content in page_index.py:160-200 + toc_extractor in page_index.py:222-238
// First concatenates raw content and transforms dots, then the caller may use LLM for further extraction
func (d *TOCDetector) extractTOCContent(pages []string, tocPageIndices []int) string {
	// Pre-calculate total size for efficiency
	totalLen := 0
	for _, idx := range tocPageIndices {
		if idx < len(pages) {
			totalLen += len(pages[idx]) + 1
		}
	}

	var content strings.Builder
	if totalLen > 0 {
		content.Grow(totalLen)
	}
	for _, idx := range tocPageIndices {
		if idx < len(pages) {
			content.WriteString(pages[idx])
			content.WriteString("\n")
		}
	}
	return transformDotsToColon(content.String())
}

// checkTOCTransformationComplete checks if TOC transformation is complete
// Python: check_if_toc_transformation_is_complete in page_index.py:143-158
func (d *TOCDetector) checkTOCTransformationComplete(ctx context.Context, rawContent, transformedContent string) bool {
	prompt := fmt.Sprintf(`请检查整理后的目录是否完整，包含了原始目录的所有内容。
请严格按照JSON格式返回结果，不要任何其他内容：
{
    "completed": "yes或者no"
}

原始目录内容：
%s

整理后的目录内容：
%s`, rawContent, transformedContent)

	response, err := d.llmClient.GenerateSimple(ctx, prompt)
	if err != nil {
		return true // Assume complete on error
	}

	var result struct {
		Completed string `json:"completed"`
	}
	if err := parseLLMJSONResponse(response, &result); err != nil {
		return true
	}

	return strings.ToLower(result.Completed) == "yes"
}

// extractTOCFromLLM extracts TOC structure from LLM response
// Python: toc_transformer in page_index.py:273-336
// Includes completeness checking and continuation for long TOCs
func (d *TOCDetector) extractTOCFromLLM(ctx context.Context, tocContent string) ([]TOCItem, error) {
	prompt := tocTransformerPrompt(tocContent)

	response, err := d.llmClient.GenerateSimple(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to extract TOC: %w", err)
	}

	// Check if transformation is complete
	if d.checkTOCTransformationComplete(ctx, tocContent, response) {
		return d.parseTOCTransformerResponse(response)
	}

	// Handle truncated JSON — find last complete object
	jsonContent := getJSONContent(response)

	// Continue generation if incomplete (max 5 retries)
	const maxAttempts = 5
	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Truncate at last complete JSON object
		lastBrace := strings.LastIndex(jsonContent, "}")
		if lastBrace != -1 {
			jsonContent = jsonContent[:lastBrace+2]
		}

		continuePrompt := fmt.Sprintf(`Your task is to continue the table of contents json structure, directly output the remaining part of the json structure.

The raw table of contents json structure is:
%s

The incomplete transformed table of contents json structure is:
%s

Please continue the json structure, directly output the remaining part of the json structure.`, tocContent, jsonContent)

		newResponse, err := d.llmClient.GenerateSimple(ctx, continuePrompt)
		if err != nil {
			break
		}

		newContent := getJSONContent(newResponse)
		jsonContent = jsonContent + newContent

		if d.checkTOCTransformationComplete(ctx, tocContent, jsonContent) {
			break
		}
	}

	return d.parseTOCTransformerResponse(jsonContent)
}

// getJSONContent extracts JSON content from a response, removing markdown code blocks
func getJSONContent(response string) string {
	startIdx := strings.Index(response, "```json")
	if startIdx != -1 {
		response = response[startIdx+7:]
	}
	endIdx := strings.LastIndex(response, "```")
	if endIdx != -1 {
		response = response[:endIdx]
	}
	return strings.TrimSpace(response)
}

// parseTOCTransformerResponse parses TOC transformer response into TOCItems
func (d *TOCDetector) parseTOCTransformerResponse(response string) ([]TOCItem, error) {
	var result TOCTransformerResult
	if err := parseLLMJSONResponse(response, &result); err != nil {
		return nil, fmt.Errorf("failed to parse TOC extraction response: %w", err)
	}

	var items []TOCItem
	for _, entry := range result.TableOfContents {
		item := TOCItem{
			Structure: entry.Structure,
			Title:     entry.Title,
			Page:      entry.Page,
		}
		// Convert page string to int if needed (Python: convert_page_to_int)
		items = append(items, item)
	}

	return items, nil
}
