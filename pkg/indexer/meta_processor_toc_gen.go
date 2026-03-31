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
7. CRITICAL FOR SUBSECTIONS: If section 1.1 starts on page 1 and 1.2 starts on page 1 but continues to page 2, mark 1.2 with physical_index: 1 (its START page)
8. CRITICAL: Different subsections often start on DIFFERENT pages - do NOT assign the same page number to all subsections
9. Look carefully: if 1.1 title appears on page 1, but 1.2 title appears on page 2, their physical_index MUST be different (1 vs 2)

CRITICAL - HIERARCHICAL STRUCTURE EXTRACTION:
You MUST identify and extract ALL levels of the document hierarchy, not just top-level sections.
DO NOT MISS ANY SUBSECTIONS - even if a section appears short, include ALL its subsections.
DO NOT MISS ANY SUBSECTIONS - even if a section appears short, include ALL its subsections.
Extract ALL numbered sections (1.1, 1.2, 1.3, 1.4, ...) that appear in the content.
Scrutinize the entire content carefully to ensure no subsection is omitted. Count the subsections to verify you have all of them.

CRITICAL - SUBSECTION COMPLETENESS CHECK:
Before returning, perform this verification:
1. If you find sections 1.1 and 1.2, CHECK if there is a 1.3, 1.4, etc.
2. Look for patterns like "1.3", "1.4", "第三节", "第四节" in the content
3. Many documents have 3-5 subsections per chapter - do NOT stop at 1.2
4. Missing subsections will cause incorrect page ranges - VERIFY completeness
5. CRITICAL: Search the ENTIRE content for ALL numbered patterns - scan from beginning to end
6. If you only find 2 subsections (1.1, 1.2), re-scan the content - there are likely more (1.3, 1.4, etc.)

CRITICAL MARKDOWN HEADER DETECTION - FINAL VERIFICATION STEP:
BEFORE returning your response, perform this FINAL scan:
1. Search for ALL lines containing "### 数字。数字" pattern (e.g., "### 1.1", "### 1.2", "### 1.3")
2. Search for ALL lines containing "### 第 [一二三四五] 节" pattern
3. For EACH match, create a TOC entry - DO NOT SKIP ANY
4. If you find "### 1.3" anywhere in the content, it MUST be in your TOC
5. If "### 1.3" appears at the BOTTOM of a page, it is STILL VALID - include it with that page number

CRITICAL - MARKDOWN FORMAT HEADERS:
The document may use Markdown header format (###) for subsections:
- "### 1.1 Title" or "### 1.2 Title" or "### 1.3 Title"
- These MUST all be extracted as subsections at the same hierarchy level
- Do NOT skip subsections that appear at the bottom of a page - they are valid sections
- Even if the content is cut off at page boundary, the SECTION TITLE is still valid

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
DO NOT MISS ANY SUBSECTIONS - even if a section appears short, include ALL its subsections.
DO NOT MISS ANY SUBSECTIONS - even if a section appears short, include ALL its subsections.
Extract ALL numbered sections (1.1, 1.2, 1.3, 1.4, ...) that appear in the content.
Scrutinize the entire content carefully to ensure no subsection is omitted. Count the subsections to verify you have all of them.

CRITICAL - SUBSECTION COMPLETENESS CHECK:
Before returning, perform this verification:
1. If you find sections 2.1 and 2.2, CHECK if there is a 2.3, 2.4, etc.
2. Look for patterns like "2.3", "2.4", "第三节", "第四节" in the content
3. Many documents have 3-5 subsections per chapter - do NOT stop at 2.2
4. Missing subsections will cause incorrect page ranges - VERIFY completeness
5. CRITICAL: Search the ENTIRE content for ALL numbered patterns - scan from beginning to end
6. If you only find 2 subsections (2.1, 2.2), re-scan the content - there are likely more (2.3, 2.4, etc.)

STRUCTURE NUMBERING SYSTEM:
- Level 1 (top-level): 1, 2, 3, ... (e.g., "第一章", "第一条")
- Level 2 (children of level 1): 1.1, 1.2, 2.1, 2.2, ... (e.g., "第一节", "（一）", "（二）")
- Level 3 (children of level 2): 1.1.1, 1.1.2, ... (e.g., nested subsections)
- Level 4+: Continue the pattern (1.1.1.1, etc.)

IMPORTANT: If Existing TOC ends with "2.1", new sections should continue appropriately:
- If new content is a subsection of "2.1", use "2.1.1", "2.1.2", etc.
- If new content is a sibling of "2.1", use "2.2", "2.3", etc.
- If new content is a new top-level section, use "3", "4", etc.

CRITICAL - PAGE NUMBER ACCURACY - MOST COMMON ERROR:
The most common error is assigning the SAME page number to all subsections (e.g., 1.1, 1.2, 1.3 all get physical_index: 1).
This is WRONG! Each subsection title appears on a SPECIFIC page - find that page!

EXAMPLE OF CORRECT BEHAVIOR:
- If "1.1 XXX" title appears on page 1 → physical_index: 1
- If "1.2 YYY" title appears on page 1 (continued from previous) → physical_index: 1  
- If "1.3 ZZZ" title appears on page 2 → physical_index: 2 (NOT 1!)
- If "1.4 AAA" title appears on page 3 → physical_index: 3 (NOT 1!)

BEFORE SUBMITTING, VERIFY:
1. Do all subsections have the SAME physical_index? If yes, YOU MADE A MISTAKE - re-scan!
2. Look at each subsection title individually - which page does it appear on?
3. Use the【第 X 页开始】tags to find the EXACT page for each title

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
