package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
)

// transformDotsToColon transforms dots like "....." to ": "
func transformDotsToColon(text string) string {
	text = regexp.MustCompile(`\.{5,}`).ReplaceAllString(text, ": ")
	text = regexp.MustCompile(`(?:\. ){5,}\.?`).ReplaceAllString(text, ": ")
	return text
}

// tocDetectorPrompt creates prompt for TOC detection
func tocDetectorPrompt(content string) string {
	return fmt.Sprintf(`Your job is to detect if there is a table of content provided in the given text.

Given text: %s

return the following JSON format:
{
    "toc_detected": "yes or no"
}

Directly return the final JSON structure. Do not output anything else.
Please note: abstract, summary, notation list, figure list, table list, etc. are not table of contents.`, content)
}

// tocTransformerPrompt creates prompt for TOC transformation
func tocTransformerPrompt(tocContent string) string {
	return fmt.Sprintf(`你现在需要将给定的目录内容转换为标准的JSON格式。
请严格按照以下要求返回结果：
1. 只返回JSON格式内容，不要任何其他解释、说明、或者额外文本
2. JSON结构必须严格符合下面的格式要求：
{
    "table_of_contents": [
        {
            "structure": "目录层级编号，字符串类型，比如"1", "1.1", "2.3.1"，没有则填"None"",
            "title": "章节标题，字符串类型",
            "page": 页码，数字类型，没有则填null
        }
    ]
}
3. 确保JSON格式正确，没有语法错误

现在要转换的目录内容是：
%s`, tocContent)
}

// parseLLMJSONResponse parses JSON from LLM response
func parseLLMJSONResponse(response string, target interface{}) error {
	content := response
	originalResponse := response

	// Remove all leading non-JSON characters (BOM, control characters, etc.)
	content = strings.TrimFunc(content, func(r rune) bool {
		return r < ' ' || r == '\ufeff' || r == 'ï' || r == '»' || r == '¿' || r == 'p' || r == 'n' || r == 't'
	})

	// Remove markdown code blocks
	start := strings.Index(content, "```json")
	if start != -1 {
		content = content[start+7:]
		end := strings.LastIndex(content, "```")
		if end != -1 {
			content = content[:end]
		}
	}

	content = strings.TrimSpace(content)
	if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	// Remove any leading text before JSON starts
	// Handle cases like "json\n{...}" or "Here is the JSON:\n{...}"
	jsonStart := strings.Index(content, "{")
	if jsonStart == -1 {
		jsonStart = strings.Index(content, "[")
	}
	if jsonStart > 0 {
		content = content[jsonStart:]
	}

	// Again remove any leading non-JSON characters
	content = strings.TrimLeftFunc(content, func(r rune) bool {
		return r != '{' && r != '['
	})

	// Find the first '{' and last '}' to extract JSON content
	// This handles cases where JSON is surrounded by other text
	firstBrace := strings.Index(content, "{")
	lastBrace := strings.LastIndex(content, "}")
	if firstBrace != -1 && lastBrace != -1 && lastBrace > firstBrace {
		content = content[firstBrace : lastBrace+1]
	} else {
		// If no braces found, try to find JSON array
		firstBracket := strings.Index(content, "[")
		lastBracket := strings.LastIndex(content, "]")
		if firstBracket != -1 && lastBracket != -1 && lastBracket > firstBracket {
			content = content[firstBracket : lastBracket+1]
		}
	}

	// Replace common invalid patterns
	content = strings.ReplaceAll(content, "None", "null")
	content = strings.ReplaceAll(content, "none", "null")
	content = strings.ReplaceAll(content, "'", "\"") // Replace single quotes with double quotes
	content = strings.ReplaceAll(content, "“", "\"") // Replace smart quotes
	content = strings.ReplaceAll(content, "”", "\"")
	content = strings.ReplaceAll(content, "‘", "'")
	content = strings.ReplaceAll(content, "’", "'")
	content = strings.ReplaceAll(content, "，", ",") // Replace Chinese commas
	content = strings.ReplaceAll(content, "：", ":") // Replace Chinese colons

	// Try parsing
	if err := json.Unmarshal([]byte(content), target); err != nil {
		// Try cleaning trailing commas
		cleaned := regexp.MustCompile(`,\s*([}\]])`).ReplaceAllString(content, "$1")
		if err2 := json.Unmarshal([]byte(cleaned), target); err2 != nil {
			// Try to fix unquoted keys
			cleaned = regexp.MustCompile(`([{\s,])\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*:`).ReplaceAllString(cleaned, `$1"$2":`)
			if err3 := json.Unmarshal([]byte(cleaned), target); err3 == nil {
				return nil
			}
			// Last resort: try to extract JSON using regex
			jsonRegex := regexp.MustCompile(`(?s)\{.*\}`)
			matches := jsonRegex.FindString(originalResponse)
			if matches != "" {
				if err3 := json.Unmarshal([]byte(matches), target); err3 == nil {
					return nil
				}
			}
			// Try array regex
			arrayRegex := regexp.MustCompile(`(?s)\[.*\]`)
			matches = arrayRegex.FindString(originalResponse)
			if matches != "" {
				if err3 := json.Unmarshal([]byte(matches), target); err3 == nil {
					return nil
				}
			}
			log.Error().Str("raw_response", originalResponse).Msg("JSON parsing failed")
			return fmt.Errorf("failed to parse JSON: %w", err)
		}
	}

	return nil
}

// addPageTags wraps content with physical index tags
func addPageTags(content string, pageIndex int) string {
	return fmt.Sprintf("<physical_index_%d>\n%s\n<physical_index_%d>\n\n",
		pageIndex, content, pageIndex)
}

// buildContentWithTags builds document with page tags
func buildContentWithTags(pages []string, startIndex int) string {
	var content strings.Builder
	for i, page := range pages {
		pageNum := startIndex + i
		content.WriteString(addPageTags(page, pageNum))
	}
	return content.String()
}

// convertPhysicalIndexToInt converts "<physical_index_5>" to 5
func convertPhysicalIndexToInt(physicalIndex string) (int, error) {
	physicalIndex = strings.TrimSpace(physicalIndex)

	if strings.HasPrefix(physicalIndex, "<physical_index_") {
		physicalIndex = strings.TrimPrefix(physicalIndex, "<physical_index_")
		physicalIndex = strings.TrimSuffix(physicalIndex, ">")
	} else if strings.HasPrefix(physicalIndex, "physical_index_") {
		physicalIndex = strings.TrimPrefix(physicalIndex, "physical_index_")
	}

	return strconv.Atoi(strings.TrimSpace(physicalIndex))
}

// detectTOCPage asks LLM if page contains TOC
func (d *TOCDetector) detectTOCPage(ctx context.Context, content string) (bool, error) {
	prompt := tocDetectorPrompt(content)

	response, err := d.llmClient.GenerateSimple(ctx, prompt)
	if err != nil {
		return false, fmt.Errorf("failed to detect TOC: %w", err)
	}

	var result TOCPromptResult
	if err := parseLLMJSONResponse(response, &result); err != nil {
		log.Warn().Err(err).Str("response", response).Msg("Failed to parse TOC detection response")
		return false, nil
	}

	return strings.ToLower(result.TOCDetected) == "yes", nil
}

// findTOCPages scans pages to find TOC pages starting from startPageIndex.
// Python: find_toc_pages in page_index.py:341-366
// Only stops at maxPages if not currently finding consecutive TOC pages.
func (d *TOCDetector) findTOCPages(ctx context.Context, pages []string, maxPages int, startPageIndex int) ([]int, error) {
	var tocPages []int
	lastPageWasTOC := false

	for i := startPageIndex; i < len(pages); i++ {
		// Only enforce maxPages limit when not in the middle of finding TOC pages
		if i >= maxPages && !lastPageWasTOC {
			break
		}

		isTOC, err := d.detectTOCPage(ctx, pages[i])
		if err != nil {
			log.Warn().Err(err).Int("page", i).Msg("TOC detection failed")
			continue
		}

		if isTOC {
			log.Info().Int("page", i).Msg("Page has TOC")
			tocPages = append(tocPages, i)
			lastPageWasTOC = true
		} else if lastPageWasTOC {
			log.Info().Int("page", i-1).Msg("Found last TOC page")
			break
		}
	}

	return tocPages, nil
}

// CheckTOC performs full TOC detection
// Python: check_toc in page_index.py:696-732
func (d *TOCDetector) CheckTOC(ctx context.Context, pages []string, tocCheckPageNum int) (*TOCResult, error) {
	tocPages, err := d.findTOCPages(ctx, pages, tocCheckPageNum, 0)
	if err != nil {
		return nil, err
	}

	if len(tocPages) == 0 {
		log.Info().Msg("No TOC found")
		return &TOCResult{
			TOCContent:     "",
			TOCPageList:    []int{},
			PageIndexGiven: false,
			Items:          []TOCItem{},
		}, nil
	}

	log.Info().Msg("TOC found")
	tocContent := d.extractTOCContent(pages, tocPages)
	hasPageIndex, err := d.detectPageIndex(ctx, tocContent)
	if err != nil {
		return nil, err
	}

	if hasPageIndex {
		log.Info().Msg("Page index found in TOC")
		return &TOCResult{
			TOCContent:     tocContent,
			TOCPageList:    tocPages,
			PageIndexGiven: true,
			Items:          []TOCItem{},
		}, nil
	}

	// Python: when first TOC has no page index, continue searching for another TOC
	// that might have page index (page_index.py:709-732)
	currentStartIndex := tocPages[len(tocPages)-1] + 1
	for !hasPageIndex && currentStartIndex < len(pages) && currentStartIndex < tocCheckPageNum {
		additionalTOCPages, err := d.findTOCPages(ctx, pages, tocCheckPageNum, currentStartIndex)
		if err != nil || len(additionalTOCPages) == 0 {
			break
		}

		additionalTOCContent := d.extractTOCContent(pages, additionalTOCPages)
		additionalHasPageIndex, err := d.detectPageIndex(ctx, additionalTOCContent)
		if err != nil {
			break
		}

		if additionalHasPageIndex {
			log.Info().Msg("Page index found in additional TOC")
			return &TOCResult{
				TOCContent:     additionalTOCContent,
				TOCPageList:    additionalTOCPages,
				PageIndexGiven: true,
				Items:          []TOCItem{},
			}, nil
		}

		currentStartIndex = additionalTOCPages[len(additionalTOCPages)-1] + 1
	}

	log.Info().Msg("Page index not found in any TOC")
	return &TOCResult{
		TOCContent:     tocContent,
		TOCPageList:    tocPages,
		PageIndexGiven: false,
		Items:          []TOCItem{},
	}, nil
}
