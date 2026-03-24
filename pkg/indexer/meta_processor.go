package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

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

// TOCItemWithNodes combines TOC item with its child nodes for tree building
type TOCItemWithNodes struct {
	TOCItem
	Children []TOCItemWithNodes
}

// MetaProcessor handles the main processing logic with mode switching
// Python: meta_processor in page_index.py:959-997
type MetaProcessor struct {
	llmClient   llm.LLMClient
	cfg         *config.Config
	tocDetector *TOCDetector
	tocVerifier *TOCVerifier
}

// NewMetaProcessor creates a new MetaProcessor
func NewMetaProcessor(client llm.LLMClient, cfg *config.Config) *MetaProcessor {
	return &MetaProcessor{
		llmClient:   client,
		cfg:         cfg,
		tocDetector: NewTOCDetector(client),
		tocVerifier: NewTOCVerifier(client),
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
	case ModeTOCNoPageNumbers:
		result, err = mp.processTOCNoPageNumbers(ctx, pageTexts, tocContent, tocPageList, startIndex)
	case ModeNoTOC:
		result, err = mp.processNoTOC(ctx, pageTexts, startIndex)
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
	tocNoPageNumber := mp.deepCopyTOCItems(tocItems)
	for i := range tocNoPageNumber {
		tocNoPageNumber[i].Page = nil
	}

	mainContent := mp.samplePages(pageTexts, startIndex, mp.cfg.TOCheckPageNum)

	// Convert mainContent string to []string (single element)
	contentPages := []string{mainContent}

	tocWithPhysicalIndex, err := mp.tocDetector.addPhysicalIndexToTOC(ctx, tocNoPageNumber, contentPages, startIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to extract physical indices: %w", err)
	}

	// Step 3: Match TOC page numbers to physical indices
	matchingPairs := mp.extractMatchingPagePairs(tocItems, tocWithPhysicalIndex, startIndex)
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

// processNoTOC generates structure without TOC
// Python: process_no_toc in page_index.py:576-595
func (mp *MetaProcessor) processNoTOC(ctx context.Context, pageTexts []string, startIndex int) ([]TOCItem, error) {
	log.Info().Msg("Processing without TOC")

	// Step 1: Wrap pages with physical index tags
	contentWithTags := mp.buildContentWithTags(pageTexts, startIndex)

	// Step 2: Group pages by token limit
	groupTexts := mp.splitContentIntoGroups(contentWithTags, mp.cfg.MaxTokensPerNode, mp.cfg.MaxPagesPerNode)

	if len(groupTexts) == 0 {
		return nil, fmt.Errorf("no content to process")
	}

	// Step 3: Generate initial TOC from first group
	tocItems, err := mp.generateTOCInit(ctx, groupTexts[0], startIndex)
	if err != nil {
		return nil, fmt.Errorf("failed to generate initial TOC: %w", err)
	}

	// Step 4: Continue TOC generation for remaining groups
	for i, groupText := range groupTexts[1:] {
		additional, err := mp.generateTOCContinue(ctx, tocItems, groupText, startIndex)
		if err != nil {
			log.Warn().Err(err).Int("group", i+1).Msg("Failed to continue TOC generation")
			continue
		}
		// Deduplicate additional items before merging
		tocItems = mp.mergeTOCItems(tocItems, additional)
	}

	return tocItems, nil
}

// generateTOCInit generates initial TOC from first content group
// Python: generate_toc_init in page_index.py:540-567
func (mp *MetaProcessor) generateTOCInit(ctx context.Context, content string, startIndex int) ([]TOCItem, error) {
	prompt := fmt.Sprintf(`Extract a hierarchical tree structure from the given document content.

IMPORTANT REQUIREMENTS:
1. Use consistent structure numbering: "1", "1.1", "1.2", "2", "2.1", etc. (no leading zeros, no trailing dots)
2. Each structure value must be UNIQUE within the document
3. Start from "1" for the first top-level section
4. CRITICAL - PAGE NUMBER ACCURACY:
   - The physical_index MUST match the ACTUAL page where the section STARTS in the document
   - Look for <physical_index_X> tags in the content - extract the X value accurately
   - DO NOT guess or make up page numbers - only use page numbers explicitly marked in the content
   - Child sections (e.g., 1.1, 1.2) must have page numbers WITHIN their parent's range
   - Sequential sections should have SEQUENTIAL page numbers (no gaps, no overlaps between siblings)
5. Verify each extracted page number by checking it against the <physical_index_X> tag in the content

Return the result in the following JSON format:
{
    "table_of_contents": [
        {
            "structure": "structure index (e.g., 1, 1.1, 1.2)",
            "title": "section title",
            "physical_index": "<physical_index_X>"
        }
    ]
}

Document content:
%s`, content)

	response, err := mp.llmClient.GenerateSimple(ctx, prompt)
	if err != nil {
		return nil, err
	}

	var result TOCTransformerResult
	if err := parseLLMJSONResponse(response, &result); err != nil {
		return nil, err
	}

	items := make([]TOCItem, len(result.TableOfContents))
	for i, entry := range result.TableOfContents {
		items[i] = TOCItem{
			Structure: normalizeStructure(entry.Structure),
			Title:     entry.Title,
			ListIndex: i,
		}
		// Convert interface{} to string first
		physicalIndexStr := result.GetPhysicalIndexAsString(i)
		if physicalIndexStr != "" {
			idx, _ := convertPhysicalIndexToInt(physicalIndexStr)
			items[i].PhysicalIndex = &idx
		}
	}

	return items, nil
}

// generateTOCContinue continues TOC generation for additional content
// Python: generate_toc_continue in page_index.py (implied)
func (mp *MetaProcessor) generateTOCContinue(ctx context.Context, existingTOC []TOCItem, content string, startIndex int) ([]TOCItem, error) {
	existingJSON, _ := json.Marshal(existingTOC)

	prompt := fmt.Sprintf(`Continue extracting hierarchical tree structure from additional document content.

Existing TOC:
%s

New content:
%s

CRITICAL REQUIREMENTS - MUST FOLLOW:
1. DO NOT return any sections that already exist in the Existing TOC above
2. DO NOT repeat any structure numbers (e.g., if "7" exists, do NOT return "7" again)
3. DO NOT repeat any section titles - extract only NEW sections not in Existing TOC
4. Use consistent structure numbering: "1", "1.1", "1.2", "2", "2.1", etc. (no leading zeros, no trailing dots)
5. Continue numbering from where the existing TOC left off
6. Each structure value must be UNIQUE across the entire document
7. CRITICAL - PAGE NUMBER ACCURACY:
   - The physical_index MUST match the ACTUAL page where the section STARTS in the document
   - Look for <physical_index_X> tags in the content - extract the X value accurately
   - DO NOT guess or make up page numbers - only use page numbers explicitly marked in the content
   - Child sections (e.g., 7.1, 7.2) must have page numbers WITHIN their parent's range (e.g., if Chapter 7 is pages 15-17, then 7.1, 7.2 must be within 15-17)
   - Sequential sections should have SEQUENTIAL page numbers (no gaps, no overlaps between siblings)
   - The first subsection should start at the parent's start page
   - The last subsection should end at the parent's end page
8. Verify each extracted page number by checking it against the <physical_index_X> tag in the content

Return in the following JSON format:
{
    "table_of_contents": [
        {
            "structure": "structure index (e.g., 1, 1.1, 2, 2.1)",
            "title": "section title",
            "physical_index": "<physical_index_X>"
        }
    ]
}

Return ONLY new sections. If all sections are already in Existing TOC, return an empty array [].`, string(existingJSON), content)

	response, err := mp.llmClient.GenerateSimple(ctx, prompt)
	if err != nil {
		return nil, err
	}

	var result TOCTransformerResult
	if err := parseLLMJSONResponse(response, &result); err != nil {
		return nil, err
	}

	items := make([]TOCItem, len(result.TableOfContents))
	for i, entry := range result.TableOfContents {
		items[i] = TOCItem{
			Structure: normalizeStructure(entry.Structure),
			Title:     entry.Title,
			ListIndex: len(existingTOC) + i,
		}
		// Convert interface{} to string first
		physicalIndexStr := result.GetPhysicalIndexAsString(i)
		if physicalIndexStr != "" {
			idx, _ := convertPhysicalIndexToInt(physicalIndexStr)
			items[i].PhysicalIndex = &idx
		}
	}

	return items, nil
}

// Helper functions

func (mp *MetaProcessor) filterValidItems(items []TOCItem) []TOCItem {
	valid := make([]TOCItem, 0, len(items))
	for _, item := range items {
		// Accept items that have either PhysicalIndex or Page set
		if item.PhysicalIndex != nil || item.Page != nil {
			valid = append(valid, item)
		}
	}
	return valid
}

func (mp *MetaProcessor) validateAndTruncatePhysicalIndices(items []TOCItem, totalPages int, startIndex int) []TOCItem {
	for i := range items {
		if items[i].PhysicalIndex != nil {
			idx := *items[i].PhysicalIndex
			// Ensure within bounds
			if idx < startIndex {
				idx = startIndex
			}
			if idx > totalPages {
				idx = totalPages
			}
			items[i].PhysicalIndex = &idx
		}
	}
	return items
}

func (mp *MetaProcessor) deepCopyTOCItems(items []TOCItem) []TOCItem {
	copy := make([]TOCItem, len(items))
	for i, item := range items {
		copy[i] = item
		if item.Page != nil {
			pageCopy := *item.Page
			copy[i].Page = &pageCopy
		}
		if item.PhysicalIndex != nil {
			idxCopy := *item.PhysicalIndex
			copy[i].PhysicalIndex = &idxCopy
		}
	}
	return copy
}

func (mp *MetaProcessor) samplePages(pageTexts []string, startIndex int, maxPages int) string {
	var content strings.Builder
	endIndex := startIndex + maxPages
	if endIndex > len(pageTexts) {
		endIndex = len(pageTexts)
	}
	for i := startIndex - 1; i < endIndex; i++ {
		if i >= 0 && i < len(pageTexts) {
			content.WriteString(addPageTags(pageTexts[i], i+1))
		}
	}
	return content.String()
}

func (mp *MetaProcessor) extractMatchingPagePairs(tocWithPages []TOCItem, tocWithPhysical []TOCItem, startIndex int) []PageIndexPair {
	pairs := make([]PageIndexPair, 0)

	for _, phyItem := range tocWithPhysical {
		if phyItem.PhysicalIndex == nil {
			continue
		}
		for _, pageItem := range tocWithPages {
			if phyItem.Title == pageItem.Title && pageItem.Page != nil {
				pairs = append(pairs, PageIndexPair{
					Title:         pageItem.Title,
					Page:          *pageItem.Page,
					PhysicalIndex: *phyItem.PhysicalIndex,
				})
				break
			}
		}
	}
	return pairs
}

func (mp *MetaProcessor) calculatePageOffset(pairs []PageIndexPair) *int {
	if len(pairs) == 0 {
		return nil
	}

	differences := make(map[int]int)
	for _, pair := range pairs {
		diff := pair.PhysicalIndex - pair.Page
		differences[diff]++
	}

	// Find most common difference
	maxCount := 0
	mostCommon := 0
	for diff, count := range differences {
		if count > maxCount {
			maxCount = count
			mostCommon = diff
		}
	}

	if maxCount > 0 {
		return &mostCommon
	}
	return nil
}

func (mp *MetaProcessor) pageListToGroupText(pageTexts []string, startIndex int) []string {
	// Simple implementation: group pages by max token limit
	groups := make([]string, 0)
	var currentGroup strings.Builder

	for i, text := range pageTexts {
		pageNum := startIndex + i
		pageContent := addPageTags(text, pageNum)

		if currentGroup.Len()+len(pageContent) > mp.cfg.MaxTokensPerNode {
			if currentGroup.Len() > 0 {
				groups = append(groups, currentGroup.String())
				currentGroup.Reset()
			}
		}
		currentGroup.WriteString(pageContent)
	}

	if currentGroup.Len() > 0 {
		groups = append(groups, currentGroup.String())
	}

	return groups
}

func (mp *MetaProcessor) splitContentIntoGroups(content string, maxTokens int, overlapPages int) []string {
	// Simple splitting by token estimate (approx 4 chars per token)
	maxChars := maxTokens * 4
	groups := make([]string, 0)

	for len(content) > 0 {
		if len(content) <= maxChars {
			groups = append(groups, content)
			break
		}

		// Find a good break point
		breakPoint := maxChars
		if breakPoint < len(content) {
			// Try to break at newline
			for i := breakPoint; i > breakPoint/2; i-- {
				if content[i] == '\n' {
					breakPoint = i
					break
				}
			}
		}

		groups = append(groups, content[:breakPoint])
		content = content[breakPoint:]
	}

	return groups
}

func (mp *MetaProcessor) addPageNumberToTOC(ctx context.Context, toc []TOCItem, content string, startIndex int) []TOCItem {
	// Call LLM to find where each TOC section appears in the content
	// This matches Python's add_page_number_to_toc behavior

	if len(toc) == 0 {
		return toc
	}

	// Build prompt to find section locations
	var tocList strings.Builder
	for i, item := range toc {
		tocList.WriteString(fmt.Sprintf("%d. %s\n", i+1, item.Title))
	}

	prompt := fmt.Sprintf(`Find the page numbers where each section appears in the document content.

Table of Contents:
%s

Document content (with page tags):
%s

Return the result in the following JSON format:
{
    "section_locations": [
        {
            "section_number": 1,
            "page": <physical_page_number>
        }
    ]
}

If a section is not found, set page to null.`, tocList.String(), content)

	response, err := mp.llmClient.GenerateSimple(ctx, prompt)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get page numbers from LLM")
		return toc
	}

	// Parse the response to extract page numbers
	// Look for JSON pattern in the response
	jsonStart := strings.Index(response, "{")
	jsonEnd := strings.LastIndex(response, "}")
	if jsonStart == -1 || jsonEnd == -1 || jsonEnd <= jsonStart {
		log.Warn().Msg("Invalid JSON response from LLM for page numbers")
		return toc
	}

	jsonStr := response[jsonStart : jsonEnd+1]

	var result struct {
		SectionLocations []struct {
			SectionNumber int `json:"section_number"`
			Page          int `json:"page"`
		} `json:"section_locations"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		log.Warn().Err(err).Msg("Failed to parse page number JSON")
		return toc
	}

	// Update TOC items with found page numbers
	for _, loc := range result.SectionLocations {
		if loc.SectionNumber > 0 && loc.SectionNumber <= len(toc) {
			if loc.Page > 0 {
				toc[loc.SectionNumber-1].PhysicalIndex = &loc.Page
			}
		}
	}

	return toc
}

func (mp *MetaProcessor) buildContentWithTags(pageTexts []string, startIndex int) string {
	var content strings.Builder
	for i, text := range pageTexts {
		pageNum := startIndex + i
		content.WriteString(addPageTags(text, pageNum))
	}
	return content.String()
}

func (mp *MetaProcessor) verifyTOC(ctx context.Context, pageTexts []string, items []TOCItem, startIndex int) (float64, []TOCItem, error) {
	tocResult := &TOCResult{Items: items}
	verifyResult, err := mp.tocVerifier.VerifyTOC(ctx, tocResult, pageTexts)
	if err != nil {
		return 0, nil, err
	}

	// Convert VerificationResult to accuracy metrics
	// This is a simplified version - real implementation would check each item
	if verifyResult.Verified {
		return 1.0, nil, nil
	}

	// Count errors
	errorCount := len(verifyResult.Errors)
	totalCount := len(items)

	if totalCount == 0 {
		return 0, nil, nil
	}

	accuracy := float64(totalCount-errorCount) / float64(totalCount)

	// Build list of incorrect items (simplified)
	incorrectItems := make([]TOCItem, 0)

	return accuracy, incorrectItems, nil
}

func (mp *MetaProcessor) fixIncorrectTOCWithRetries(ctx context.Context, items []TOCItem, pageTexts []string, incorrectItems []TOCItem, startIndex int, maxRetries int) ([]TOCItem, []TOCItem, error) {
	currentItems := items
	currentIncorrect := incorrectItems

	for attempt := 0; attempt < maxRetries && len(currentIncorrect) > 0; attempt++ {
		newItems, stillIncorrect, err := mp.fixIncorrectTOC(ctx, currentItems, pageTexts, startIndex, currentIncorrect)
		if err != nil {
			return currentItems, currentIncorrect, err
		}
		currentItems = newItems
		currentIncorrect = stillIncorrect
	}

	return currentItems, currentIncorrect, nil
}

func (mp *MetaProcessor) fixIncorrectTOC(ctx context.Context, items []TOCItem, pageTexts []string, startIndex int, incorrectItems []TOCItem) ([]TOCItem, []TOCItem, error) {
	// Create set of incorrect indices
	incorrectSet := make(map[int]bool)
	for _, item := range incorrectItems {
		incorrectSet[item.ListIndex] = true
	}

	// Fix each incorrect item
	fixed := make([]TOCItem, 0)
	stillIncorrect := make([]TOCItem, 0)

	for _, item := range incorrectItems {
		newItem, err := mp.fixSingleItem(ctx, item, items, incorrectSet, pageTexts, startIndex)
		if err != nil {
			stillIncorrect = append(stillIncorrect, item)
			continue
		}
		fixed = append(fixed, newItem)

		// Update the item in the main list
		for i := range items {
			if items[i].ListIndex == newItem.ListIndex {
				items[i] = newItem
				break
			}
		}
	}

	return items, stillIncorrect, nil
}

func (mp *MetaProcessor) fixSingleItem(ctx context.Context, incorrectItem TOCItem, allItems []TOCItem, incorrectSet map[int]bool, pageTexts []string, startIndex int) (TOCItem, error) {
	endIndex := len(pageTexts) + startIndex - 1

	// Find previous correct item
	prevCorrect := startIndex - 1
	for i := incorrectItem.ListIndex - 1; i >= 0; i-- {
		if !incorrectSet[i] && i < len(allItems) {
			if allItems[i].PhysicalIndex != nil {
				prevCorrect = *allItems[i].PhysicalIndex
				break
			}
		}
	}

	// Find next correct item
	nextCorrect := endIndex
	for i := incorrectItem.ListIndex + 1; i < len(allItems); i++ {
		if !incorrectSet[i] {
			if allItems[i].PhysicalIndex != nil {
				nextCorrect = *allItems[i].PhysicalIndex
				break
			}
		}
	}

	// Build content for search range
	var content strings.Builder
	for pageNum := prevCorrect; pageNum <= nextCorrect && pageNum <= endIndex; pageNum++ {
		pageIdx := pageNum - startIndex
		if pageIdx >= 0 && pageIdx < len(pageTexts) {
			content.WriteString(addPageTags(pageTexts[pageIdx], pageNum))
		}
	}

	// Ask LLM to find the section
	prompt := fmt.Sprintf(`You are given a section title and several pages of a document, your job is to find the physical index of the start page of the section in the partial document.

The provided pages contains tags like <physical_index_X> and <physical_index_X> to indicate the physical location of the page X.

Reply in a JSON format:
{
    "thinking": "explain which page contains the start of this section",
    "physical_index": "<physical_index_X> (keep the format)"
}
Directly return the final JSON structure. Do not output anything else.

Section Title: %s
Document pages: %s`, incorrectItem.Title, content.String())

	response, err := mp.llmClient.GenerateSimple(ctx, prompt)
	if err != nil {
		return incorrectItem, err
	}

	var result struct {
		PhysicalIndex string `json:"physical_index"`
	}
	if err := parseLLMJSONResponse(response, &result); err != nil {
		return incorrectItem, err
	}

	idx, err := convertPhysicalIndexToInt(result.PhysicalIndex)
	if err != nil {
		return incorrectItem, err
	}

	incorrectItem.PhysicalIndex = &idx
	return incorrectItem, nil
}

// ProcessLargeNodeRecursively processes large nodes recursively
// Python: process_large_node_recursively in page_index.py:1000-1027
func (mp *MetaProcessor) ProcessLargeNodeRecursively(ctx context.Context, item *TOCItemWithNodes, pageTexts []string, startIndex int, lang language.Language) {
	if item == nil {
		return
	}

	// Calculate page count
	startPage := 1
	if item.PhysicalIndex != nil {
		startPage = *item.PhysicalIndex
	}
	endPage := len(pageTexts)

	pageCount := endPage - startPage + 1
	if pageCount < 0 {
		pageCount = 0
	}

	// Check if node is too large
	if pageCount > mp.cfg.MaxPagesPerNode {
		log.Info().
			Str("title", item.Title).
			Int("pages", pageCount).
			Msg("Processing large node recursively")

		// Generate sub-structure for this node
		subItems, err := mp.processNoTOC(ctx, pageTexts[startPage-1:endPage], startPage)
		if err == nil && len(subItems) > 0 {
			// Clear existing children first to avoid duplicates
			item.Children = nil

			// Check if first item matches current item
			if len(subItems) > 0 && subItems[0].Title == item.Title {
				// Remove first item and add rest as children
				for _, subItem := range subItems[1:] {
					child := &TOCItemWithNodes{TOCItem: subItem}
					item.Children = append(item.Children, *child)
				}
			} else {
				// Add all as children
				for _, subItem := range subItems {
					child := &TOCItemWithNodes{TOCItem: subItem}
					item.Children = append(item.Children, *child)
				}
			}
		}
	}
}

// mergeTOCItems merges additional TOC items into existing items with deduplication
// Deduplicates based on structure field OR title, keeping the first occurrence
func (mp *MetaProcessor) mergeTOCItems(existing, additional []TOCItem) []TOCItem {
	seenStructures := make(map[string]bool)
	seenTitles := make(map[string]bool)

	// Build set of existing structures and titles
	for _, item := range existing {
		if item.Structure != "" {
			seenStructures[normalizeStructure(item.Structure)] = true
		}
		// Normalize title for comparison (trim spaces)
		if item.Title != "" {
			seenTitles[strings.TrimSpace(item.Title)] = true
		}
	}

	log.Info().
		Int("existing_count", len(existing)).
		Int("additional_count", len(additional)).
		Int("known_structures", len(seenStructures)).
		Int("known_titles", len(seenTitles)).
		Msg("Merging TOC items")

	merged := make([]TOCItem, len(existing))
	copy(merged, existing)

	for _, item := range additional {
		shouldSkip := false
		skipReason := ""

		// Check structure duplication
		if item.Structure != "" {
			normalized := normalizeStructure(item.Structure)
			if seenStructures[normalized] {
				shouldSkip = true
				skipReason = "duplicate structure"
			} else {
				seenStructures[normalized] = true
				item.Structure = normalized
			}
		}

		// Check title duplication (only if not already skipped)
		if !shouldSkip && item.Title != "" {
			title := strings.TrimSpace(item.Title)
			if seenTitles[title] {
				shouldSkip = true
				skipReason = "duplicate title"
			} else {
				seenTitles[title] = true
			}
		}

		if shouldSkip {
			log.Warn().
				Str("structure", item.Structure).
				Str("title", item.Title).
				Str("reason", skipReason).
				Msg("Skipping duplicate TOC item during merge")
		} else {
			merged = append(merged, item)
		}
	}

	log.Info().
		Int("merged_count", len(merged)).
		Int("removed", len(additional)+len(existing)-len(merged)).
		Msg("TOC merge complete")

	return merged
}

// normalizeStructure normalizes a structure string to a consistent format
// Removes leading/trailing spaces, normalizes multiple dots, removes leading zeros
// Examples: " 1.1 " -> "1.1", "01.02" -> "1.2", "1.." -> "1."
func normalizeStructure(structure string) string {
	if structure == "" {
		return ""
	}

	// Trim spaces
	structure = strings.TrimSpace(structure)

	// Split by dot
	parts := strings.Split(structure, ".")
	normalized := make([]string, 0, len(parts))

	for _, part := range parts {
		if part == "" {
			continue // Skip empty parts (from multiple dots)
		}
		// Remove leading zeros and convert to integer then back to string
		if num, err := strconv.Atoi(part); err == nil {
			normalized = append(normalized, strconv.Itoa(num))
		} else {
			// If not a number, keep the original (after trimming)
			normalized = append(normalized, strings.TrimSpace(part))
		}
	}

	return strings.Join(normalized, ".")
}
