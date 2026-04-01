package indexer

import (
	"fmt"

	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/llm"
)

// TOCItem represents a single TOC entry
type TOCItem struct {
	Structure     string `json:"structure"`
	Title         string `json:"title"`
	Page          *int   `json:"page,omitempty"`
	PhysicalIndex *int   `json:"physical_index,omitempty"`
	ListIndex     *int   `json:"list_index,omitempty"`
	AppearStart   string `json:"appear_start,omitempty"`
	EndPage       *int   `json:"-"` // Temporary field for end page calculation, not serialized
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

// BatchTOCDetectorResult is LLM response for batch TOC detection
type BatchTOCDetectorResult struct {
	TOCPages []int `json:"toc_pages"`
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
	cfg       *config.Config
}

// NewTOCDetector creates a new TOCDetector
func NewTOCDetector(client llm.LLMClient, cfg *config.Config) *TOCDetector {
	return &TOCDetector{llmClient: client, cfg: cfg}
}
