package indexer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/xgsong/mypageindexgo/pkg/language"
)

// getLanguageInstructionForTOC returns language-specific instruction for TOC generation
func getLanguageInstructionForTOC(lang language.Language) string {
	if lang.Code == "zh" {
		return `IMPORTANT: The document is written in Chinese. ALL section titles MUST be in Chinese (中文). Do NOT use English or any other language.`
	}
	if lang.Code == "ja" {
		return `IMPORTANT: The document is written in Japanese. ALL section titles MUST be in Japanese (日本語). Do NOT use English or any other language.`
	}
	if lang.Code == "ko" {
		return `IMPORTANT: The document is written in Korean. ALL section titles MUST be in Korean (한국어). Do NOT use English or any other language.`
	}
	if lang.Code == "ru" {
		return `IMPORTANT: The document is written in Russian. ALL section titles MUST be in Russian. Do NOT use English or any other language.`
	}
	if lang.Code == "fr" {
		return `IMPORTANT: The document is written in French. ALL section titles MUST be in French. Do NOT use English or any other language.`
	}
	if lang.Code == "de" {
		return `IMPORTANT: The document is written in German. ALL section titles MUST be in German. Do NOT use English or any other language.`
	}
	if lang.Code == "es" {
		return `IMPORTANT: The document is written in Spanish. ALL section titles MUST be in Spanish. Do NOT use English or any other language.`
	}
	// Default for English or unknown languages
	return ``
}

// generateTOCInit generates initial TOC from first content group
// Python: generate_toc_init in page_index.py:540-567
func (mp *MetaProcessor) generateTOCInit(ctx context.Context, content string, _ int, lang language.Language) ([]TOCItem, error) {
	languageInstruction := getLanguageInstructionForTOC(lang)

	prompt := fmt.Sprintf(`%s

Extract a HIERARCHICAL tree structure from the given document content.

CRITICAL PAGE NUMBER EXTRACTION - HIGHEST PRIORITY:
The document has pages marked with【第 X 页开始】 and【第 X 页结束】tags.
Each section between these tags represents page X of the PDF.

For example:
【第 1 页开始】
content of page 1...
【第 1 页结束】

IMPORTANT RULES:
1. When "第一章" or "第一条" appears between【第 1 页开始】and【第 1 页结束】, its page number is 1
2. When "第二章" or "第二条" appears between【第 3 页开始】and【第 3 页结束】, its page number is 3
3. The page numbers may not be sequential - always look at the ACTUAL tag numbers
4. DO NOT guess page numbers - use the tag numbers exactly
5. CRITICAL: Extract page numbers EXACTLY as they appear in the tags - NO estimation, NO inference
6. Before returning, VERIFY each physical_index matches the actual page tag in the content

CRITICAL - HIERARCHICAL STRUCTURE EXTRACTION:
You MUST identify and extract ALL levels of the document hierarchy, not just top-level sections.

STRUCTURE NUMBERING SYSTEM:
- Level 1 (top-level): 1, 2, 3, ... (e.g., "第一章", "第二章", "第一条", "第二条")
- Level 2 (children of level 1): 1.1, 1.2, 2.1, 2.2, ... (e.g., "1.1", "（一）", "（二）", "第一节")
- Level 3 (children of level 2): 1.1.1, 1.1.2, ... (e.g., nested subsections)
- Level 4+: Continue the pattern (1.1.1.1, etc.)

HIERARCHY EXAMPLES:
Example 1 - Chinese document:
{
    "table_of_contents": [
        {"structure": "1", "title": "第一章 总论", "physical_index": "1"},
        {"structure": "1.1", "title": "第一节 研究背景", "physical_index": "1"},
        {"structure": "1.2", "title": "第二节 研究目的", "physical_index": "3"},
        {"structure": "2", "title": "第二章 文献综述", "physical_index": "5"},
        {"structure": "2.1", "title": "第一节 国内研究", "physical_index": "5"},
        {"structure": "2.2", "title": "第二节 国外研究", "physical_index": "7"}
    ]
}

Example 2 - Document with subsections:
{
    "table_of_contents": [
        {"structure": "1", "title": "1. Introduction", "physical_index": "1"},
        {"structure": "1.1", "title": "1.1 Background", "physical_index": "1"},
        {"structure": "1.2", "title": "1.2 Objectives", "physical_index": "2"},
        {"structure": "2", "title": "2. Methodology", "physical_index": "4"},
        {"structure": "2.1", "title": "2.1 Data Collection", "physical_index": "4"},
        {"structure": "2.2", "title": "2.2 Analysis", "physical_index": "6"}
    ]
}

CRITICAL - RETURN ITEMS IN PAGE ORDER:
- Sort sections by their physical_index (page number) in ASCENDING order
- DO NOT return in structure order (1, 1.1, 1.2, 2...) - return in PAGE ORDER
- This ensures correct page range calculation

Document content:
%s`, languageInstruction, content)

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
			Structure:   normalizeStructure(entry.Structure),
			Title:       cleanTitleForOutput(entry.Title),
			ListIndex:   i,
			AppearStart: "yes",
		}
		// Convert interface{} to string first
		physicalIndexStr := result.GetPhysicalIndexAsString(i)
		if physicalIndexStr != "" {
			idx, err := convertPhysicalIndexToInt(physicalIndexStr)
			if err != nil {
				continue
			}
			if idx > 0 {
				items[i].PhysicalIndex = &idx
			}
		}
	}

	return items, nil
}

// generateTOCContinue continues TOC generation for additional content
// Python: generate_toc_continue in page_index.py (implied)
func (mp *MetaProcessor) generateTOCContinue(ctx context.Context, existingTOC []TOCItem, content string, _ int, lang language.Language) ([]TOCItem, error) {
	existingJSON, _ := json.Marshal(existingTOC)

	languageInstruction := getLanguageInstructionForTOC(lang)

	prompt := fmt.Sprintf(`%s

Continue extracting HIERARCHICAL tree structure from additional document content.

Existing TOC:
%s

New content:
%s

CRITICAL REQUIREMENTS - MUST FOLLOW:
1. DO NOT return any sections that already exist in the Existing TOC above
2. DO NOT repeat any structure numbers (e.g., if "7" exists, do NOT return "7" again)
3. DO NOT repeat any section titles - extract only NEW sections not in Existing TOC

CRITICAL - HIERARCHICAL STRUCTURE CONTINUATION:
You MUST continue extracting ALL levels of the document hierarchy.

STRUCTURE NUMBERING SYSTEM:
- Level 1 (top-level): 1, 2, 3, ... (e.g., "第一章", "第一条")
- Level 2 (children of level 1): 1.1, 1.2, 2.1, 2.2, ... (e.g., "第一节", "（一）", "（二）")
- Level 3 (children of level 2): 1.1.1, 1.1.2, ... (e.g., nested subsections)
- Level 4+: Continue the pattern (1.1.1.1, etc.)

IMPORTANT: If Existing TOC ends with "2.1", new sections should continue appropriately:
- If new content is a subsection of "2.1", use "2.1.1", "2.1.2", etc.
- If new content is a sibling of "2.1", use "2.2", "2.3", etc.
- If new content is a new top-level section, use "3", "4", etc.

CRITICAL - PAGE NUMBER ACCURACY:
- The physical_index MUST match the ACTUAL page where the section STARTS
- Look for【第 X 页开始】tags in the content - extract the X value EXACTLY
- DO NOT guess or make up page numbers

CRITICAL - RETURN ITEMS IN PAGE ORDER:
- Sort ALL new sections by their physical_index in ASCENDING order
- DO NOT return in structure order - return in PAGE ORDER

Return in the following JSON format:
{
    "table_of_contents": [
        {"structure": "2.2", "title": "section title", "physical_index": "8"},
        {"structure": "2.2.1", "title": "subsection title", "physical_index": "8"},
        {"structure": "3", "title": "new chapter", "physical_index": "10"}
    ]
}

Return ONLY new sections. If all sections are already in Existing TOC, return an empty array [].`, languageInstruction, string(existingJSON), content)

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
			Title:     cleanTitleForOutput(entry.Title),
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
