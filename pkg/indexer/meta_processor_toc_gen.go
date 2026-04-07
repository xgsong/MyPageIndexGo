package indexer

import (
	"context"
	"encoding/json"

	"github.com/xgsong/mypageindexgo/pkg/language"
	"github.com/xgsong/mypageindexgo/pkg/prompts"
)

// generateTOCInit generates initial TOC from first content group
// Python: generate_toc_init in page_index.py:540-567
func (mp *MetaProcessor) generateTOCInit(ctx context.Context, content string, _ int, lang language.Language) ([]TOCItem, error) {
	languageInstruction := prompts.GetLanguageInstructionForTOC(lang.Code)
	prompt := prompts.TOCInitPrompt(languageInstruction, content)

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
			ListIndex:     ptr(i),
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
	languageInstruction := prompts.GetLanguageInstructionForTOC(lang.Code)
	prompt := prompts.TOCContinuePromptWithExisting(languageInstruction, string(existingJSON), content)

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
			ListIndex:     ptr(len(existingTOC) + i),
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
