package indexer

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/language"
)

// GenerateWithTOC generates an index tree using TOC-based processing.
// This is the main entry point for PDF indexing with TOC detection and processing.
// Python equivalent: tree_parser in page_index.py:1029-1063
func (g *IndexGenerator) GenerateWithTOC(ctx context.Context, doc *document.Document) (*document.IndexTree, error) {
	startTime := time.Now()
	log.Info().
		Int("pages", len(doc.Pages)).
		Str("language", doc.Language.Name).
		Msg("Starting index generation with TOC")

	// Store reference to original document for summary generation
	g.doc = doc

	// Detect document language from first page sample
	if doc.Language.Code == "" {
		doc.Language = language.Detect(doc.GetFullText())
		log.Info().Str("detected_language", doc.Language.Name).Msg("Detected document language")
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

	// Check if document has TOC
	// Python: check_toc in page_index.py:696-732
	tocDetector := NewTOCDetector(g.llmClient)
	tocResult, err := tocDetector.CheckTOC(ctx, pageTexts, g.cfg.TOCheckPageNum)
	if err != nil {
		log.Warn().Err(err).Msg("TOC detection failed, proceeding without TOC")
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
		log.Info().Msg("Processing mode: TOC with page numbers")
	} else {
		mode = ModeNoTOC
		log.Info().Msg("Processing mode: No TOC (or TOC without page index)")
	}

	// Process document with meta processor
	items, err := mp.Process(ctx, pageTexts, mode, tocResult.TOCContent, tocResult.TOCPageList, 1)
	if err != nil {
		return nil, fmt.Errorf("meta processor failed: %w", err)
	}

	log.Info().Int("toc_items", len(items)).Msg("Meta processor complete")

	// Python: add_preface_if_needed (utils.py:367-378)
	items = addPrefaceIfNeeded(items)

	// Python: check_title_appearance_in_start_concurrent (page_index.py:1051)
	ac := NewAppearanceChecker(g.llmClient, g.cfg)
	items = ac.CheckAllItemsAppearanceInStart(ctx, items, pageTexts)

	// Convert TOC items to tree structure using Python-equivalent logic
	root := g.generateTreeFromTOC(items, len(doc.Pages))
	if root == nil {
		return nil, fmt.Errorf("failed to generate tree from TOC")
	}

	// Python: process_large_node_recursively (page_index.py:1057-1061)
	// In Python, process_large_node_recursively is called on each node in the
	// tree LIST (not on a wrapper root). We process children of root, not root itself.
	for _, child := range root.Children {
		g.processLargeNodesWithMetaProcessor(ctx, child, mp, pageTexts)
	}

	// Count total nodes
	totalNodes := root.CountNodes()
	log.Info().Int("total_nodes", totalNodes).Msg("Tree structure created")

	// Create the index tree
	tree := document.NewIndexTree(root, len(doc.Pages))
	tree.DocumentInfo = fmt.Sprintf("Document with %d pages", len(doc.Pages))

	// Generate summaries if enabled
	if g.cfg.GenerateSummaries {
		stepStart := time.Now()
		log.Info().Int("nodes", totalNodes).Msg("Starting summary generation")
		if err := g.generateAllSummaries(ctx, root); err != nil {
			return nil, fmt.Errorf("failed to generate summaries: %w", err)
		}
		log.Info().
			Dur("duration", time.Since(stepStart)).
			Msg("Summary generation complete")
	}

	log.Info().
		Dur("total_duration", time.Since(startTime)).
		Int("total_nodes", totalNodes).
		Msg("Index generation complete")

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
func (g *IndexGenerator) processLargeNodesWithMetaProcessor(ctx context.Context, node *document.Node, mp *MetaProcessor, pageTexts []string) {
	if node == nil {
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

	// Check if node is large enough to split
	// Python: end_index - start_index > max_page_num_each_node AND token_num >= max_token_num_each_node
	if pageCount > g.cfg.MaxPagesPerNode && tokenNum >= g.cfg.MaxTokenNumEachNode {
		log.Info().
			Str("title", node.Title).
			Int("start", node.StartPage).
			Int("end", node.EndPage).
			Int("tokens", tokenNum).
			Msg("Processing large node recursively")

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
				if len(validItems) > 1 && validItems[1].PhysicalIndex != nil {
					node.EndPage = *validItems[1].PhysicalIndex
				}
			}
		} else {
			childRoot := g.generateTreeFromTOC(validItems, node.EndPage)
			if childRoot != nil {
				node.Children = childRoot.Children
				if validItems[0].PhysicalIndex != nil {
					node.EndPage = *validItems[0].PhysicalIndex
				}
			}
		}
	}

	// Recurse into children
	for _, child := range node.Children {
		g.processLargeNodesWithMetaProcessor(ctx, child, mp, pageTexts)
	}
}
