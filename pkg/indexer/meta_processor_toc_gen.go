package indexer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/xgsong/mypageindexgo/pkg/language"
)

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
