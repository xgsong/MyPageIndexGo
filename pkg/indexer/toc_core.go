package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/xgsong/mypageindexgo/pkg/llm"
)

// TOCItem represents a single TOC entry
type TOCItem struct {
	Structure     string `json:"structure"`
	Title         string `json:"title"`
	Page          *int   `json:"page,omitempty"`
	PhysicalIndex *int   `json:"physical_index,omitempty"`
	ListIndex     int    `json:"list_index,omitempty"`
	AppearStart   string `json:"appear_start,omitempty"`
	EndPage       int    `json:"-"` // Temporary field for end page calculation, not serialized
}

// TOCResult holds TOC detection result
type TOCResult struct {
	TOCContent     string    `json:"toc_content"`
	TOCPageList    []int     `json:"toc_page_list"`
	PageIndexGiven bool      `json:"page_index_given"`
	Items          []TOCItem `json:"items"`
}

// PageIndexPair represents a matched title-page pair for offset calculation
type PageIndexPair struct {
	Title         string `json:"title"`
	Page          int    `json:"page"`
	PhysicalIndex int    `json:"physical_index"`
}

// TOCPromptResult is LLM response for TOC detection
type TOCPromptResult struct {
	Thinking    string `json:"thinking"`
	TOCDetected string `json:"toc_detected"`
}

// PageIndexDetectorResult is LLM response for page index detection
type PageIndexDetectorResult struct {
	Thinking       string `json:"thinking"`
	PageIndexGiven string `json:"page_index_given_in_toc"`
}

// TOCTransformerResult is LLM response for TOC transformation
type TOCTransformerResult struct {
	TableOfContents []struct {
		Structure     string      `json:"structure"`
		Title         string      `json:"title"`
		Page          *int        `json:"page"`
		PhysicalIndex interface{} `json:"physical_index,omitempty"` // Can be string or number
	} `json:"table_of_contents"`
}

// GetPhysicalIndexAsString converts PhysicalIndex to string regardless of its underlying type
func (t *TOCTransformerResult) GetPhysicalIndexAsString(index int) string {
	if index >= len(t.TableOfContents) {
		return ""
	}

	switch v := t.TableOfContents[index].PhysicalIndex.(type) {
	case string:
		return v
	case float64: // JSON numbers are parsed as float64
		return fmt.Sprintf("%.0f", v)
	case int:
		return fmt.Sprintf("%d", v)
	default:
		return ""
	}
}

// TOCIndexExtractorResult represents an extracted item with physical_index
type TOCIndexExtractorResult struct {
	Structure     string      `json:"structure"`
	Title         string      `json:"title"`
	PhysicalIndex interface{} `json:"physical_index"` // Can be string or number
}

// GetPhysicalIndexAsString converts PhysicalIndex to string regardless of its underlying type
func (t *TOCIndexExtractorResult) GetPhysicalIndexAsString() string {
	switch v := t.PhysicalIndex.(type) {
	case string:
		return v
	case float64: // JSON numbers are parsed as float64
		return fmt.Sprintf("%.0f", v)
	case int:
		return fmt.Sprintf("%d", v)
	default:
		return ""
	}
}

// TOCDetector handles TOC detection and extraction
type TOCDetector struct {
	llmClient llm.LLMClient
}

// NewTOCDetector creates a new TOCDetector
func NewTOCDetector(client llm.LLMClient) *TOCDetector {
	return &TOCDetector{llmClient: client}
}

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
    "thinking": "why do you think there is a table of content in the given text",
    "toc_detected": "yes or no"
}

Directly return the final JSON structure. Do not output anything else.
Please note: abstract, summary, notation list, figure list, table list, etc. are not table of contents.`, content)
}

// tocTransformerPrompt creates prompt for TOC transformation
func tocTransformerPrompt(tocContent string) string {
	return fmt.Sprintf(`You are given a table of contents. Your job is to transform the whole table of content into a JSON format included table_of_contents.

structure is the numeric system which represents the index of the hierarchy section in the table of contents. For example, the first section has structure index 1, the first subsection has structure index 1.1, the second subsection has structure index 1.2, etc.

The response should be in the following JSON format: 
{
    "table_of_contents": [
        {
            "structure": "structure index, x.x.x or None (string)",
            "title": "title of the section",
            "page": page number or None (number)
        }
    ]
}

You should transform the full table of contents in one go.
Directly return the final JSON structure, do not output anything else.

Given table of contents:
%s`, tocContent)
}

// parseLLMJSONResponse parses JSON from LLM response
func parseLLMJSONResponse(response string, target interface{}) error {
	content := response

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

	// Replace Python None with JSON null
	content = strings.ReplaceAll(content, "None", "null")

	// Try parsing
	if err := json.Unmarshal([]byte(content), target); err != nil {
		// Try cleaning trailing commas
		cleaned := regexp.MustCompile(`,\s*([}\]])`).ReplaceAllString(content, "$1")
		if err2 := json.Unmarshal([]byte(cleaned), target); err2 != nil {
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

// CheckTOC performs full TOC detection
func (d *TOCDetector) CheckTOC(ctx context.Context, pages []string, tocCheckPageNum int) (*TOCResult, error) {
	tocPages, err := d.findTOCPages(ctx, pages, tocCheckPageNum)
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
	hasPageIndex, err := d.detectPageIndex(ctx, tocContent)
	if err != nil {
		return nil, err
	}

	return &TOCResult{
		TOCContent:     tocContent,
		TOCPageList:    tocPages,
		PageIndexGiven: hasPageIndex,
		Items:          []TOCItem{},
	}, nil
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

// findTOCPages scans pages to find TOC pages
func (d *TOCDetector) findTOCPages(ctx context.Context, pages []string, maxPages int) ([]int, error) {
	var tocPages []int
	lastPageWasTOC := false

	for i := 0; i < len(pages) && i < maxPages; i++ {
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

// extractTOCContent extracts TOC content from pages
func (d *TOCDetector) extractTOCContent(pages []string, tocPageIndices []int) string {
	var content strings.Builder
	for _, idx := range tocPageIndices {
		if idx < len(pages) {
			content.WriteString(pages[idx])
			content.WriteString("\n")
		}
	}
	return transformDotsToColon(content.String())
}

// detectPageIndex asks LLM if TOC has page numbers
func (d *TOCDetector) detectPageIndex(ctx context.Context, tocContent string) (bool, error) {
	prompt := fmt.Sprintf(`You will be given a table of contents.

Your job is to detect if there are page numbers/indices given within the table of contents.

Given text: %s

Reply format:
{
    "thinking": "why do you think there are page numbers/indices given within the table of contents",
    "page_index_given_in_toc": "yes or no"
}
Directly return the final JSON structure. Do not output anything else.`, tocContent)

	response, err := d.llmClient.GenerateSimple(ctx, prompt)
	if err != nil {
		return false, fmt.Errorf("failed to detect page index: %w", err)
	}

	var result PageIndexDetectorResult
	if err := parseLLMJSONResponse(response, &result); err != nil {
		return false, err
	}

	return strings.ToLower(result.PageIndexGiven) == "yes", nil
}

// extractTOCFromLLM extracts TOC structure from LLM response
func (d *TOCDetector) extractTOCFromLLM(ctx context.Context, tocContent string) ([]TOCItem, error) {
	prompt := tocTransformerPrompt(tocContent)

	response, err := d.llmClient.GenerateSimple(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to extract TOC: %w", err)
	}

	var result TOCTransformerResult
	if err := parseLLMJSONResponse(response, &result); err != nil {
		return nil, fmt.Errorf("failed to parse TOC extraction response: %w", err)
	}

	var items []TOCItem
	for _, entry := range result.TableOfContents {
		items = append(items, TOCItem{
			Structure:     entry.Structure,
			Title:         entry.Title,
			Page:          entry.Page,
			PhysicalIndex: nil, // Will be set later by page mapping
		})
	}

	log.Info().Int("items", len(items)).Msg("Extracted TOC items from LLM")
	return items, nil
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
