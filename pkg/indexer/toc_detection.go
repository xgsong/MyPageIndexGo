package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"

	"github.com/xgsong/mypageindexgo/pkg/prompts"
)

// Pre-compiled regular expressions for performance
var (
	// transformDotsToColon patterns
	fiveDotsRegex = regexp.MustCompile(`\.{5,}`)
	dotSpaceRegex = regexp.MustCompile(`(?:\. ){5,}\.?`)

	// parseLLMJSONResponse patterns
	trailingCommaRegex = regexp.MustCompile(`,\s*([}\]])`)
	unquotedKeyRegex   = regexp.MustCompile(`([{\s,])\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*:`)
	jsonExtractRegex   = regexp.MustCompile(`(?s)\{.*\}`)
	arrayExtractRegex  = regexp.MustCompile(`(?s)\[.*\]`)

	// convertPhysicalIndexToInt patterns
	chinesePageRegex = regexp.MustCompile(`第(\d+)页`)
)

// transformDotsToColon transforms dots like "....." to ": "
func transformDotsToColon(text string) string {
	text = fiveDotsRegex.ReplaceAllString(text, ": ")
	text = dotSpaceRegex.ReplaceAllString(text, ": ")
	return text
}

// parseLLMJSONResponse parses JSON from LLM response
func parseLLMJSONResponse(response string, target interface{}) error {
	content := response
	originalResponse := response

	log.Debug().Str("response_length", strconv.Itoa(len(response))).Msg("parsing LLM JSON response")

	// Remove all leading non-JSON characters (BOM, control characters, etc.)
	content = strings.TrimFunc(content, func(r rune) bool {
		return r < ' ' || r == '\ufeff' || r == 'ï' || r == '»' || r == '¿'
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
		cleaned := trailingCommaRegex.ReplaceAllString(content, "$1")
		if err2 := json.Unmarshal([]byte(cleaned), target); err2 != nil {
			// Try to fix unquoted keys
			cleaned = unquotedKeyRegex.ReplaceAllString(cleaned, `$1"$2":`)
			if err3 := json.Unmarshal([]byte(cleaned), target); err3 == nil {
				return nil
			}
			// Last resort: try to extract JSON using regex
			matches := jsonExtractRegex.FindString(originalResponse)
			if matches != "" {
				if err3 := json.Unmarshal([]byte(matches), target); err3 == nil {
					return nil
				}
			}
			// Try array regex
			matches = arrayExtractRegex.FindString(originalResponse)
			if matches != "" {
				if err3 := json.Unmarshal([]byte(matches), target); err3 == nil {
					return nil
				}
			}
			return fmt.Errorf("failed to parse JSON: %w", err)
		}
	}

	return nil
}

// addPageTags wraps content with physical index tags
func addPageTags(content string, pageIndex int) string {
	return fmt.Sprintf("【第%d页开始】\n%s\n【第%d页结束】\n\n",
		pageIndex, content, pageIndex)
}

// buildContentWithTags builds document with page tags
func buildContentWithTags(pages []string, startIndex int) string {
	// Pre-calculate approximate size for efficiency (tags add ~50 chars per page)
	totalLen := 0
	for _, page := range pages {
		totalLen += len(page) + 50
	}

	var content strings.Builder
	content.Grow(totalLen)
	for i, page := range pages {
		pageNum := startIndex + i
		content.WriteString(addPageTags(page, pageNum))
	}
	return content.String()
}

// convertPhysicalIndexToInt converts various formats to int
// Supports: "<physical_index_5>", "physical_index_5", "5", "【第 5 页开始】"
func convertPhysicalIndexToInt(physicalIndex string) (int, error) {
	physicalIndex = strings.TrimSpace(physicalIndex)

	// Try to extract number from Chinese format【第 X 页开始】
	if strings.Contains(physicalIndex, "第") && strings.Contains(physicalIndex, "页") {
		// Remove Chinese brackets and text first
		cleaned := strings.ReplaceAll(physicalIndex, "【", "")
		cleaned = strings.ReplaceAll(cleaned, "】", "")
		cleaned = strings.ReplaceAll(cleaned, "开始", "")
		cleaned = strings.ReplaceAll(cleaned, "结束", "")

		matches := chinesePageRegex.FindStringSubmatch(cleaned)
		if len(matches) >= 2 {
			return strconv.Atoi(matches[1])
		}
	}

	// Try standard <physical_index_X> format
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
	prompt := prompts.TOCDetectorPrompt(content)

	log.Debug().Int("content_length", len(content)).Msg("detecting TOC page")

	response, err := d.llmClient.GenerateSimple(ctx, prompt)
	if err != nil {
		return false, fmt.Errorf("failed to detect TOC: %w", err)
	}

	var result TOCPromptResult
	if err := parseLLMJSONResponse(response, &result); err != nil {
		log.Debug().Str("response", response).Msg("TOC detection JSON parse failed")
		return false, nil
	}

	isTOC := strings.ToLower(result.TOCDetected) == "yes"
	log.Debug().Bool("is_toc", isTOC).Msg("TOC detection result")
	return isTOC, nil
}

// findTOCPages scans pages to find TOC pages starting from startPageIndex.
// Python: find_toc_pages in page_index.py:341-366
// Only stops at maxPages if not currently finding consecutive TOC pages.
// Uses per-page detection for reliability.
func (d *TOCDetector) findTOCPages(ctx context.Context, pages []string, maxPages int, startPageIndex int) ([]int, error) {
	return d.findTOCPagesPerPage(ctx, pages, maxPages, startPageIndex)
}

// findTOCPagesPerPage performs per-page TOC detection (original implementation)
// Used as fallback when batch detection fails
func (d *TOCDetector) findTOCPagesPerPage(ctx context.Context, pages []string, maxPages int, startPageIndex int) ([]int, error) {
	var tocPages []int
	lastPageWasTOC := false

	log.Debug().Int("total_pages", len(pages)).Int("start_index", startPageIndex).Int("max_pages", maxPages).Msg("starting TOC page detection")

	for i := startPageIndex; i < len(pages); i++ {
		select {
		case <-ctx.Done():
			log.Debug().Int("cancelled_at_page", i).Msg("TOC detection cancelled")
			return tocPages, ctx.Err()
		default:
		}

		if i >= maxPages && !lastPageWasTOC {
			log.Debug().Int("stopped_at_page", i).Msg("TOC detection stopped")
			break
		}

		isTOC, err := d.detectTOCPage(ctx, pages[i])
		if err != nil {
			log.Debug().Int("page", i).Err(err).Msg("TOC detection error")
			continue
		}

		if isTOC {
			tocPages = append(tocPages, i)
			lastPageWasTOC = true
			log.Debug().Int("page", i).Msg("found TOC page")
		} else if lastPageWasTOC {
			log.Debug().Int("page", i).Msg("TOC sequence ended")
			break
		}
	}

	log.Debug().Ints("toc_pages", tocPages).Msg("TOC detection complete")
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
		return &TOCResult{
			TOCContent:     "",
			TOCPageList:    []int{},
			PageIndexGiven: false,
			Items:          []TOCItem{},
		}, nil
	}

	tocContent := d.extractTOCContent(pages, tocPages)
	return &TOCResult{
		TOCContent:     tocContent,
		TOCPageList:    tocPages,
		PageIndexGiven: false,
		Items:          []TOCItem{},
	}, nil
}
