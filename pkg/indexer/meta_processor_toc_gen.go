package indexer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog/log"
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
func (mp *MetaProcessor) generateTOCInit(ctx context.Context, content string, startIndex int, lang language.Language) ([]TOCItem, error) {
	// Create language-specific system message
	languageInstruction := getLanguageInstructionForTOC(lang)

	prompt := fmt.Sprintf(`%s

Extract a hierarchical tree structure from the given document content.

CRITICAL PAGE NUMBER EXTRACTION:
The document has pages marked with 【第X页开始】 and 【第X页结束】 tags.
Each section between these tags represents page X of the PDF.

For example:
【第1页开始】
content of page 1...
【第1页结束】

IMPORTANT RULES:
1. When "第一章" appears between 【第1页开始】 and 【第1页结束】, its page number is 1
2. When "第二章" appears between 【第3页开始】 and 【第3页结束】, its page number is 3
3. The page numbers may not be sequential - always look at the ACTUAL tag numbers
4. DO NOT guess page numbers - use the tag numbers exactly

DOCUMENT STRUCTURE:
- Top-level sections: 1, 2, 3, ... (e.g., "第一章", "第二章")
- Child sections: 1.1, 1.2, ... (e.g., "1.1", "1.2")
- Top-level sections are FLAT siblings

Return JSON:
{
    "table_of_contents": [
        {
            "structure": "1",
            "title": "第一章...",
            "physical_index": "1"
        }
    ]
}

Document content:
%s`, languageInstruction, content)

	response, err := mp.llmClient.GenerateSimple(ctx, prompt)
	if err != nil {
		return nil, err
	}

	log.Info().Str("response", response).Msg("LLM response for generateTOCInit")

	var result TOCTransformerResult
	if err := parseLLMJSONResponse(response, &result); err != nil {
		return nil, err
	}

	items := make([]TOCItem, len(result.TableOfContents))
	for i, entry := range result.TableOfContents {
		items[i] = TOCItem{
			Structure:   normalizeStructure(entry.Structure),
			Title:       entry.Title,
			ListIndex:   i,
			AppearStart: "yes",
		}
		// Convert interface{} to string first
		physicalIndexStr := result.GetPhysicalIndexAsString(i)
		if physicalIndexStr != "" {
			idx, err := convertPhysicalIndexToInt(physicalIndexStr)
			if err != nil {
				log.Warn().Int("item", i).Str("physical_index", physicalIndexStr).Err(err).Msg("Failed to parse physical_index, skipping")
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
func (mp *MetaProcessor) generateTOCContinue(ctx context.Context, existingTOC []TOCItem, content string, startIndex int, lang language.Language) ([]TOCItem, error) {
	existingJSON, _ := json.Marshal(existingTOC)

	// Create language-specific system message
	languageInstruction := getLanguageInstructionForTOC(lang)

	prompt := fmt.Sprintf(`%s

Continue extracting hierarchical tree structure from additional document content.

Existing TOC:
%s

New content:
%s

CRITICAL REQUIREMENTS - MUST FOLLOW:
1. DO NOT return any sections that already exist in the Existing TOC above
2. DO NOT repeat any structure numbers (e.g., if "7" exists, do NOT return "7" again)
3. DO NOT repeat any section titles - extract only NEW sections not in Existing TOC
4. Structure numbering for Chinese legal documents:
   - Top-level sections: 1, 2, 3, ... (e.g., "第一条", "第二条", "第三条")
   - Child of top-level: 1.1, 1.2, ... (e.g., "（一）", "（二）" which are 子条款 of 条)
   - Sub-sub-level: 1.1.1, 1.1.2, ... (e.g., nested content under 子条款)
   - CRITICAL: 条(1, 2, 3...) are FLAT siblings - 条 2 is NOT a child of 条 1!
   - Only （一）（二）... under 条 are children of that 条
5. Continue numbering from where the existing TOC left off
6. Each structure value must be UNIQUE across the entire document
7. CRITICAL - PAGE NUMBER ACCURACY:
   - The physical_index MUST match the ACTUAL page where the section STARTS in the document
   - Look for <physical_index_X> tags in the content - extract the X value accurately
   - DO NOT guess or make up page numbers - only use page numbers explicitly marked in the content
   - Sequential sections (siblings) should have SEQUENTIAL or NON-OVERLAPPING page numbers
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
