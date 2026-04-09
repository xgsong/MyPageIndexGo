package indexer

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/language"
)

// GenerateWithTOC generates an index tree using TOC-based processing.
// This is the main entry point for PDF indexing with TOC detection and processing.
// Python equivalent: tree_parser in page_index.py:1029-1063
func (g *IndexGenerator) GenerateWithTOC(ctx context.Context, doc *document.Document, progressCb ProgressCallback) (*document.IndexTree, error) {

	// Clear instance-level cache for each new document
	g.nodeTextCache = make(map[string]*nodeTextCacheEntry)

	// Detect document language from first page sample
	if doc.Language.Code == "" {
		detector := language.NewDetector()
		doc.Language = detector.DetectWithSampleSize(doc.GetFullText(), g.cfg.LanguageDetectSampleSize)
	}

	// Create meta processor
	mp := NewMetaProcessor(g.llmClient, g.cfg, doc.Language)

	// Convert document pages to page texts
	pageTexts := make([]string, len(doc.Pages))
	for i, page := range doc.Pages {
		pageTexts[i] = page.Text
	}

	// Precompute page text map for summary generation (1-based)
	pageTextMap := make(map[int]string, len(doc.Pages))
	for i, text := range pageTexts {
		pageNum := i + 1 // Pages are 1-based
		pageTextMap[pageNum] = text
	}

	// Check if document has TOC (Stage 1)
	// Python: check_toc in page_index.py:696-732
	if progressCb != nil {
		progressCb(1, 5, "Detecting TOC")
	}
	tocDetector := NewTOCDetector(g.llmClient, g.cfg)
	tocResult, err := tocDetector.CheckTOC(ctx, pageTexts, g.cfg.TOCheckPageNum)
	if err != nil {
		log.Warn().Err(err).Msg("TOC detection failed, using empty result")
		tocResult = &TOCResult{
			TOCContent:     "",
			TOCPageList:    []int{},
			PageIndexGiven: false,
			Items:          []TOCItem{},
		}
	}

	// Determine processing mode
	// Python tree_parser (page_index.py:1029-1063):
	// - TOC with page index -> process_toc_with_page_numbers
	// - TOC without page index OR no TOC -> process_no_toc
	var mode ProcessingMode
	if tocResult.TOCContent != "" && strings.TrimSpace(tocResult.TOCContent) != "" && tocResult.PageIndexGiven {
		mode = ModeTOCWithPageNumbers
	} else {
		mode = ModeNoTOC
	}

	// Debug logging for OCR investigation
	log.Debug().
		Str("mode", string(mode)).
		Int("toc_pages", len(tocResult.TOCPageList)).
		Bool("page_index_given", tocResult.PageIndexGiven).
		Int("page_texts", len(pageTexts)).
		Msg("TOC detection complete")
	// if len(tocResult.TOCPageList) > 0 {
	// 	log.Debug().Ints("toc_page_list", tocResult.TOCPageList).Msg("TOC pages detected")
	// }

	// Process document with meta processor (Stage 2)
	if progressCb != nil {
		progressCb(2, 5, "Processing document structure")
	}
	items, err := mp.Process(ctx, pageTexts, mode, tocResult.TOCContent, tocResult.TOCPageList, 1)
	if err != nil {
		return nil, fmt.Errorf("meta processor failed: %w", err)
	}

	// Debug logging for OCR investigation
	log.Debug().
		Str("mode", string(mode)).
		Int("toc_items", len(items)).
		Msg("Meta processor complete")

	// Python: add_preface_if_needed (utils.py:367-378)
	items = addPrefaceIfNeeded(items)

	// Python: check_title_appearance_in_start_concurrent (page_index.py:1051) (Stage 3)
	if progressCb != nil {
		progressCb(3, 5, "Verifying TOC items")
	}
	ac := NewAppearanceChecker(g.llmClient, g.cfg)
	items = ac.CheckAllItemsAppearanceInStart(ctx, items, pageTexts)

	// Convert TOC items to tree structure using Python-equivalent logic
	root := g.generateTreeFromTOC(items, pageTexts, len(doc.Pages), pageTextMap)
	if root == nil {
		return nil, fmt.Errorf("failed to generate tree from TOC")
	}

	// Count total nodes
	root.CountNodes()

	// Create the index tree
	tree := document.NewIndexTree(root, len(doc.Pages))
	tree.DocumentInfo = fmt.Sprintf("Document with %d pages", len(doc.Pages))

	// Generate summaries if enabled (Stage 5)
	if g.cfg.GenerateSummaries {
		if err := g.generateAllSummaries(ctx, root, progressCb, 80, 100, doc, pageTextMap); err != nil {
			return nil, fmt.Errorf("failed to generate summaries: %w", err)
		}
	}

	// Ensure progress is marked as complete
	if progressCb != nil {
		progressCb(100, 100, "Index generation complete")
	}

	return tree, nil
}

// addPrefaceIfNeeded adds a Preface node if first item doesn't start at page 1
// Python: add_preface_if_needed in utils.py:367-378
func addPrefaceIfNeeded(items []TOCItem) []TOCItem {
	if len(items) == 0 {
		return items
	}

	if items[0].PhysicalIndex != nil && *items[0].PhysicalIndex > 1 {
		prefacePage := 1
		preface := TOCItem{
			Structure:     "0",
			Title:         "Preface",
			PhysicalIndex: &prefacePage,
		}
		items = append([]TOCItem{preface}, items...)
	}

	return items
}
