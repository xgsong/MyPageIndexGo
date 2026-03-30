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

	// Store reference to original document for summary generation
	g.doc = doc

	// Detect document language from first page sample
	if doc.Language.Code == "" {
		doc.Language = language.Detect(doc.GetFullText())
	}

	// Create meta processor
	mp := NewMetaProcessor(g.llmClient, g.cfg, doc.Language)

	// Convert document pages to page texts
	pageTexts := make([]string, len(doc.Pages))
	for i, page := range doc.Pages {
		pageTexts[i] = page.Text
	}

	// Precompute page text map for summary generation (1-based)
	g.pageTextMap = make(map[int]string, len(doc.Pages))
	for i, text := range pageTexts {
		pageNum := i + 1 // Pages are 1-based
		g.pageTextMap[pageNum] = text
	}

	// Check if document has TOC (Stage 1)
	// Python: check_toc in page_index.py:696-732
	if progressCb != nil {
		progressCb(1, 5, "Detecting TOC")
	}
	tocDetector := NewTOCDetector(g.llmClient)
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
	log.Info().
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
	log.Info().
		Str("mode", string(mode)).
		Int("toc_items", len(items)).
		Msg("Meta processor complete")
	// for i, item := range items {
	// 	if i < 5 {
	// 		pageVal := -1
	// 		if item.PhysicalIndex != nil {
	// 			pageVal = *item.PhysicalIndex
	// 		}
	// 		log.Debug().
	// 			Str("structure", item.Structure).
	// 			Str("title", item.Title).
	// 			Int("page", pageVal).
	// 			Msg("TOC item")
	// 	}
	// }

	// Python: add_preface_if_needed (utils.py:367-378)
	items = addPrefaceIfNeeded(items)

	// Python: check_title_appearance_in_start_concurrent (page_index.py:1051) (Stage 3)
	if progressCb != nil {
		progressCb(3, 5, "Verifying TOC items")
	}
	ac := NewAppearanceChecker(g.llmClient, g.cfg)
	items = ac.CheckAllItemsAppearanceInStart(ctx, items, pageTexts)

	// Convert TOC items to tree structure using Python-equivalent logic
	root := g.generateTreeFromTOC(items, len(doc.Pages))
	if root == nil {
		return nil, fmt.Errorf("failed to generate tree from TOC")
	}

	// Python: process_large_node_recursively (page_index.py:1057-1061) (Stage 4)
	if progressCb != nil {
		progressCb(4, 5, "Processing large sections")
	}
	for _, child := range root.Children {
		g.processLargeNodesWithMetaProcessor(ctx, child, mp, pageTexts)
	}

	// Count total nodes
	root.CountNodes()

	// Create the index tree
	tree := document.NewIndexTree(root, len(doc.Pages))
	tree.DocumentInfo = fmt.Sprintf("Document with %d pages", len(doc.Pages))

	// Generate summaries if enabled (Stage 5)
	if g.cfg.GenerateSummaries {
		if err := g.generateAllSummaries(ctx, root, progressCb, 80, 100); err != nil {
			return nil, fmt.Errorf("failed to generate summaries: %w", err)
		}
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

// processLargeNodesWithMetaProcessor recursively splits large tree nodes using LLM.
// Python: process_large_node_recursively in page_index.py:1000-1027
// MODIFIED: Lowered threshold and added page-based splitting logic
func (g *IndexGenerator) processLargeNodesWithMetaProcessor(ctx context.Context, node *document.Node, mp *MetaProcessor, pageTexts []string) {
	g.processLargeNodesWithMetaProcessorRecursive(ctx, node, mp, pageTexts, 0)
}

// processLargeNodesWithMetaProcessorRecursive is the internal recursive implementation with depth tracking
func (g *IndexGenerator) processLargeNodesWithMetaProcessorRecursive(ctx context.Context, node *document.Node, mp *MetaProcessor, pageTexts []string, depth int) {
	if node == nil {
		return
	}

	// Prevent infinite recursion: max depth of 3 levels
	if depth >= 3 {
		return
	}

	pageCount := node.EndPage - node.StartPage + 1
	if pageCount <= 0 {
		return
	}

	// Calculate token count for this node
	tokenNum := 0
	for pageNum := node.StartPage; pageNum <= node.EndPage; pageNum++ {
		if pageNum >= 1 && pageNum <= len(pageTexts) {
			tokenNum += g.tokenizer.Count(pageTexts[pageNum-1])
		}
	}

	// Disable large node splitting temporarily to avoid performance issues
	// This feature will be re-enabled with optimized LLM batching in future versions
	shouldSplit := false

	if shouldSplit {
		// Get sub-pages
		startIdx := node.StartPage - 1
		endIdx := node.EndPage
		if startIdx < 0 {
			startIdx = 0
		}
		if endIdx > len(pageTexts) {
			endIdx = len(pageTexts)
		}
		subPageTexts := pageTexts[startIdx:endIdx]

		// Generate sub-structure
		// processNoTOC will generate physical indices relative to the entire document
		// because we pass the full pageTexts array to CheckAllItemsAppearanceInStart later
		subItems, err := mp.processNoTOC(ctx, subPageTexts, node.StartPage, nil, false)
		if err != nil || len(subItems) == 0 {
			return
		}

		// Check appear_start for sub-items
		ac := NewAppearanceChecker(g.llmClient, g.cfg)
		subItems = ac.CheckAllItemsAppearanceInStart(ctx, subItems, pageTexts)

		// Filter valid items
		validItems := make([]TOCItem, 0, len(subItems))
		for _, item := range subItems {
			if item.PhysicalIndex != nil {
				validItems = append(validItems, item)
			}
		}

		if len(validItems) == 0 {
			return
		}

		// Python: if first sub-item title matches parent, remove it
		if strings.TrimSpace(node.Title) == strings.TrimSpace(validItems[0].Title) {
			// Build children from items[1:]
			childRoot := g.generateTreeFromTOC(validItems[1:], node.EndPage)
			if childRoot != nil {
				node.Children = childRoot.Children
			}
		} else {
			childRoot := g.generateTreeFromTOC(validItems, node.EndPage)
			if childRoot != nil {
				node.Children = childRoot.Children
			}
		}
	}

	// Recurse into children with incremented depth
	for _, child := range node.Children {
		g.processLargeNodesWithMetaProcessorRecursive(ctx, child, mp, pageTexts, depth+1)
	}
}
