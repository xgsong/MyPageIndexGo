package indexer

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
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
	log.Info().Str("mode", string(mode)).Int("start_index", startIndex).Msg("Starting meta processor")

	var result []TOCItem
	var err error

	switch mode {
	case ModeTOCWithPageNumbers:
		result, err = mp.processTOCWithPageNumbers(ctx, pageTexts, tocContent, tocPageList, startIndex)
		if err != nil {
			log.Warn().Err(err).Msg("TOC with page numbers processing failed, falling back to TOC no page numbers mode")
			return mp.Process(ctx, pageTexts, ModeTOCNoPageNumbers, tocContent, tocPageList, startIndex)
		}
	case ModeTOCNoPageNumbers:
		result, err = mp.processTOCNoPageNumbers(ctx, pageTexts, tocContent, tocPageList, startIndex)
		if err != nil {
			log.Warn().Err(err).Msg("TOC without page numbers processing failed, falling back to no TOC mode")
			return mp.Process(ctx, pageTexts, ModeNoTOC, "", []int{}, startIndex)
		}
	case ModeNoTOC:
		result, err = mp.processNoTOC(ctx, pageTexts, startIndex)
		if err != nil {
			log.Warn().Err(err).Msg("No TOC processing failed, returning simple flat structure")
			// Fallback to simplest possible structure: one item per page
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
		log.Warn().Err(err).Msg("TOC verification failed")
		return result, nil
	}

	log.Info().
		Str("mode", string(mode)).
		Float64("accuracy", accuracy).
		Int("incorrect_count", len(incorrectResults)).
		Msg("TOC verification complete")

	// Handle verification results
	if accuracy == 1.0 && len(incorrectResults) == 0 {
		// Perfect accuracy
		return result, nil
	}

	if accuracy > 0.6 && len(incorrectResults) > 0 {
		// Try to fix incorrect items
		fixedResult, _, err := mp.fixIncorrectTOCWithRetries(ctx, result, pageTexts, incorrectResults, startIndex, 3)
		if err == nil {
			return fixedResult, nil
		}
		log.Warn().Err(err).Msg("Failed to fix incorrect TOC")
		return result, nil
	}

	// Accuracy too low, fallback to simpler mode
	log.Warn().Float64("accuracy", accuracy).Str("current_mode", string(mode)).Msg("Accuracy too low, falling back")

	switch mode {
	case ModeTOCWithPageNumbers:
		// Fallback to ModeTOCNoPageNumbers
		return mp.Process(ctx, pageTexts, ModeTOCNoPageNumbers, tocContent, tocPageList, startIndex)
	case ModeTOCNoPageNumbers:
		// Fallback to ModeNoTOC
		return mp.Process(ctx, pageTexts, ModeNoTOC, "", []int{}, startIndex)
	case ModeNoTOC:
		// Already at simplest mode
		return result, nil
	}

	return result, nil
}

// processTOCWithPageNumbers processes TOC with explicit page numbers
// Python: process_toc_with_page_numbers in page_index.py:622-652
func (mp *MetaProcessor) processTOCWithPageNumbers(ctx context.Context, pageTexts []string, tocContent string, tocPageList []int, startIndex int) ([]TOCItem, error) {
	log.Info().Msg("Processing TOC with page numbers")

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
		log.Info().Int("offset", *offset).Msg("Applying page offset")
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
	log.Info().Msg("Processing TOC without page numbers")

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
			PhysicalIndex: &physicalIdx, // Use 1-based index matching Page.Number
			AppearStart:   "yes",        // Each section starts at the beginning of the page
		})
	}
	log.Info().Int("items", len(items)).Msg("Generated simple flat structure as fallback")
	return items
}

// processNoTOC generates structure without TOC
// Python: process_no_toc in page_index.py:576-595
func (mp *MetaProcessor) processNoTOC(ctx context.Context, pageTexts []string, startIndex int) ([]TOCItem, error) {
	log.Info().Msg("Processing without TOC")

	// Step 1: Wrap pages with physical index tags
	contentWithTags := buildContentWithTags(pageTexts, startIndex)

	// Step 2: Group pages by token limit
	groupTexts := mp.splitContentIntoGroups(contentWithTags, mp.cfg.MaxTokensPerNode, mp.cfg.MaxPagesPerNode)

	if len(groupTexts) == 0 {
		return nil, fmt.Errorf("no content to process")
	}

	// Step 3: Generate initial TOC from first group
	tocItems, err := mp.generateTOCInit(ctx, groupTexts[0], startIndex, mp.docLanguage)
	if err != nil {
		return nil, fmt.Errorf("failed to generate initial TOC: %w", err)
	}

	// Step 4: Continue TOC generation for remaining groups
	for i, groupText := range groupTexts[1:] {
		additional, err := mp.generateTOCContinue(ctx, tocItems, groupText, startIndex, mp.docLanguage)
		if err != nil {
			log.Warn().Err(err).Int("group", i+1).Msg("Failed to continue TOC generation")
			continue
		}
		// Deduplicate additional items before merging
		tocItems = mp.mergeTOCItems(tocItems, additional)
	}

	return tocItems, nil
}
