package indexer

import (
	"context"
	"fmt"

	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/language"
	"github.com/xgsong/mypageindexgo/pkg/llm"
)

// ProcessingMode represents the different processing modes
type ProcessingMode string

const (
	// ModeTOCWithPageNumbers processes TOC that has explicit page numbers
	ModeTOCWithPageNumbers ProcessingMode = "process_toc_with_page_numbers"
	// ModeTOCNoPageNumbers processes TOC without page numbers
	ModeTOCNoPageNumbers ProcessingMode = "process_toc_no_page_numbers"
	// ModeNoTOC generates structure without TOC
	ModeNoTOC ProcessingMode = "process_no_toc"
)

// MetaProcessor handles the main processing logic with mode switching
// Python: meta_processor in page_index.py:959-997
type MetaProcessor struct {
	llmClient   llm.LLMClient
	cfg         *config.Config
	tocDetector *TOCDetector
	docLanguage language.Language // Document language for consistent output
}

// NewMetaProcessor creates a new MetaProcessor
func NewMetaProcessor(client llm.LLMClient, cfg *config.Config, docLanguage language.Language) *MetaProcessor {
	return &MetaProcessor{
		llmClient:   client,
		cfg:         cfg,
		tocDetector: NewTOCDetector(client),
		docLanguage: docLanguage,
	}
}

// Process processes pages according to the specified mode
// Python: meta_processor in page_index.py:959-997
func (mp *MetaProcessor) Process(ctx context.Context, pageTexts []string, mode ProcessingMode, tocContent string, tocPageList []int, startIndex int) ([]TOCItem, error) {
	var result []TOCItem
	var err error

	switch mode {
	case ModeTOCWithPageNumbers:
		result, err = mp.processTOCWithPageNumbers(ctx, pageTexts, tocContent, tocPageList, startIndex)
		if err != nil {
			return mp.Process(ctx, pageTexts, ModeTOCNoPageNumbers, tocContent, tocPageList, startIndex)
		}
	case ModeTOCNoPageNumbers:
		result, err = mp.processTOCNoPageNumbers(ctx, pageTexts, tocContent, tocPageList, startIndex)
		if err != nil {
			return mp.Process(ctx, pageTexts, ModeNoTOC, "", []int{}, startIndex)
		}
	case ModeNoTOC:
		result, err = mp.processNoTOC(ctx, pageTexts, startIndex, tocPageList, false)
		if err != nil {
			return mp.generateSimpleFlatStructure(pageTexts, startIndex), nil
		}
	default:
		return nil, fmt.Errorf("unknown processing mode: %s", mode)
	}

	if err != nil {
		return nil, err
	}

	// Filter items with nil physical_index
	result = mp.filterValidItems(result)

	// Validate and truncate physical indices
	result = mp.validateAndTruncatePhysicalIndices(result, len(pageTexts), startIndex)

	// Verify TOC accuracy
	accuracy, incorrectResults, err := mp.verifyTOC(ctx, pageTexts, result, startIndex)
	if err != nil {
		return result, nil
	}

	// Handle verification results
	if accuracy == 1.0 && len(incorrectResults) == 0 {
		return result, nil
	}

	if accuracy > 0.6 && len(incorrectResults) > 0 {
		if !mp.cfg.SkipTOCFix {
			fixedResult, _, err := mp.fixIncorrectTOCWithRetries(ctx, result, pageTexts, incorrectResults, startIndex, 3)
			if err == nil {
				return fixedResult, nil
			}
		}
		return result, nil
	}

	// Accuracy too low, fallback to simpler mode
	switch mode {
	case ModeTOCWithPageNumbers:
		return mp.Process(ctx, pageTexts, ModeTOCNoPageNumbers, tocContent, tocPageList, startIndex)
	case ModeTOCNoPageNumbers:
		return mp.Process(ctx, pageTexts, ModeNoTOC, "", []int{}, startIndex)
	case ModeNoTOC:
		return result, nil
	}

	return result, nil
}

// processTOCWithPageNumbers processes TOC with explicit page numbers
// Python: process_toc_with_page_numbers in page_index.py:622-652
func (mp *MetaProcessor) processTOCWithPageNumbers(ctx context.Context, pageTexts []string, tocContent string, tocPageList []int, startIndex int) ([]TOCItem, error) {

	// Step 1: Transform raw TOC to structured JSON
	tocItems, err := mp.tocDetector.extractTOCFromLLM(ctx, tocContent)
	if err != nil {
		return nil, fmt.Errorf("failed to transform TOC: %w", err)
	}

	// Step 2: Extract physical index mapping from sample pages
	// Python: start_page_index = toc_page_list[-1] + 1
	tocNoPageNumber := mp.deepCopyTOCItems(tocItems)
	for i := range tocNoPageNumber {
		tocNoPageNumber[i].Page = nil
	}

	sampleStart := startIndex
	if len(tocPageList) > 0 {
		sampleStart = tocPageList[len(tocPageList)-1] + 1
	}
	mainContent := mp.samplePages(pageTexts, sampleStart, mp.cfg.TOCheckPageNum)

	contentPages := []string{mainContent}
	tocWithPhysicalIndex, err := mp.tocDetector.addPhysicalIndexToTOC(ctx, tocNoPageNumber, contentPages, sampleStart)
	if err != nil {
		return nil, fmt.Errorf("failed to extract physical indices: %w", err)
	}

	// Step 3: Match TOC page numbers to physical indices
	matchingPairs := mp.extractMatchingPagePairs(tocItems, tocWithPhysicalIndex, sampleStart)
	offset := mp.calculatePageOffset(matchingPairs)

	// Step 4: Apply offset to convert logical page to physical index
	if offset != nil {
		for i := range tocItems {
			if tocItems[i].Page != nil {
				physicalIdx := *tocItems[i].Page + *offset
				tocItems[i].PhysicalIndex = &physicalIdx
			}
		}
	}

	// Step 5: Fix items that still lack physical_index after offset
	// Python: process_none_page_numbers in page_index.py:656-691
	tocItems = mp.processNonePageNumbers(ctx, tocItems, pageTexts, startIndex)

	return tocItems, nil
}

// processTOCNoPageNumbers processes TOC without page numbers
// Python: process_toc_no_page_numbers in page_index.py:597-618
func (mp *MetaProcessor) processTOCNoPageNumbers(ctx context.Context, pageTexts []string, tocContent string, tocPageList []int, startIndex int) ([]TOCItem, error) {

	// Step 1: Transform TOC to structured format
	tocItems, err := mp.tocDetector.extractTOCFromLLM(ctx, tocContent)
	if err != nil {
		return nil, fmt.Errorf("failed to transform TOC: %w", err)
	}

	// Step 2: Group pages by token limit
	groupTexts := mp.pageListToGroupText(pageTexts, startIndex)

	// Step 3: For each group, find where TOC sections appear
	for _, groupText := range groupTexts {
		tocItems = mp.addPageNumberToTOC(ctx, tocItems, groupText, startIndex)
	}

	return tocItems, nil
}

// generateSimpleFlatStructure generates the simplest possible structure: one item per page
func (mp *MetaProcessor) generateSimpleFlatStructure(pageTexts []string, startIndex int) []TOCItem {
	var items []TOCItem
	for i := range pageTexts {
		pageNum := startIndex + i // pageNum is 1-based, matching Page.Number in document
		physicalIdx := pageNum    // Create copy to avoid pointer reuse

		// Generate title based on document language
		var title string
		if mp.docLanguage.Code == "zh" {
			title = fmt.Sprintf("第%d页", pageNum)
		} else {
			title = fmt.Sprintf("Page %d", pageNum)
		}

		items = append(items, TOCItem{
			Structure:     fmt.Sprintf("%d", i+1),
			Title:         title,
			PhysicalIndex: &physicalIdx,
			AppearStart:   "yes",
		})
	}
	return items
}

// processNoTOC generates structure without TOC
// Python: process_no_toc in page_index.py:576-595
func (mp *MetaProcessor) processNoTOC(ctx context.Context, pageTexts []string, startIndex int, tocPageList []int, pageIndexGiven bool) ([]TOCItem, error) {

	// Only skip TOC pages if we have high confidence they are real TOC pages with page numbers
	// If PageIndexGiven is false, the detected "TOC pages" are likely false positives
	// and should not be skipped
	if pageIndexGiven && len(tocPageList) > 0 {
		skipPages := make(map[int]bool)
		for _, p := range tocPageList {
			if p >= 0 && p < len(pageTexts) {
				skipPages[p] = true
			}
		}

		filteredTexts := make([]string, 0, len(pageTexts))
		actualStartIndex := startIndex
		adjustedStart := startIndex
		for i, text := range pageTexts {
			pageNum := startIndex + i
			if skipPages[pageNum] {
				continue
			}
			filteredTexts = append(filteredTexts, text)
			if adjustedStart == startIndex && len(filteredTexts) == 1 {
				actualStartIndex = pageNum
			}
		}
		if len(filteredTexts) == 0 {
			filteredTexts = pageTexts
			actualStartIndex = startIndex
		}

		contentWithTags := buildContentWithTags(filteredTexts, actualStartIndex)
		groupTexts := mp.splitContentIntoGroups(contentWithTags, mp.cfg.MaxTokensPerNode, mp.cfg.MaxPagesPerNode)

		if len(groupTexts) == 0 {
			return nil, fmt.Errorf("no content to process")
		}

		tocItems, err := mp.generateTOCInit(ctx, groupTexts[0], actualStartIndex, mp.docLanguage)
		if err != nil {
			return nil, fmt.Errorf("failed to generate initial TOC: %w", err)
		}

		for _, groupText := range groupTexts[1:] {
			additional, err := mp.generateTOCContinue(ctx, tocItems, groupText, actualStartIndex, mp.docLanguage)
			if err != nil {
				continue
			}
			tocItems = mp.mergeTOCItems(tocItems, additional)
		}

		return tocItems, nil
	}

	// No confident TOC found, process all pages
	contentWithTags := buildContentWithTags(pageTexts, startIndex)
	groupTexts := mp.splitContentIntoGroups(contentWithTags, mp.cfg.MaxTokensPerNode, mp.cfg.MaxPagesPerNode)

	if len(groupTexts) == 0 {
		return nil, fmt.Errorf("no content to process")
	}

	tocItems, err := mp.generateTOCInit(ctx, groupTexts[0], startIndex, mp.docLanguage)
	if err != nil {
		return nil, fmt.Errorf("failed to generate initial TOC: %w", err)
	}

	for _, groupText := range groupTexts[1:] {
		additional, err := mp.generateTOCContinue(ctx, tocItems, groupText, startIndex, mp.docLanguage)
		if err != nil {
			continue
		}
		tocItems = mp.mergeTOCItems(tocItems, additional)
	}

	return tocItems, nil
}
