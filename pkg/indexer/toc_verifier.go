package indexer

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
	"github.com/xgsong/mypageindexgo/pkg/llm"
)

// VerificationResult represents the result of TOC verification
type VerificationResult struct {
	Verified bool     `json:"verified"`
	Errors   []string `json:"errors,omitempty"`
}

// TOCVerifier handles TOC verification and fixing
type TOCVerifier struct {
	llmClient llm.LLMClient
}

// NewTOCVerifier creates a new TOCVerifier
func NewTOCVerifier(client llm.LLMClient) *TOCVerifier {
	return &TOCVerifier{llmClient: client}
}

// verifyTOCEntry asks LLM to verify a single TOC entry
func (v *TOCVerifier) verifyTOCEntry(ctx context.Context, item TOCItem, pageContent string) (*VerificationResult, error) {
	prompt := fmt.Sprintf(`You are given a TOC entry and the actual page content from a document.
Your job is to verify if the TOC entry is correct.

TOC entry:
- Structure: %s
- Title: %s
- Page: %v

Page content:
%s

Respond in the following JSON format:
{
    "verified": true or false,
    "errors": ["list of errors if not verified"]
}

Directly return the final JSON structure. Do not output anything else.`,
		item.Structure, item.Title, item.Page, pageContent)

	response, err := v.llmClient.GenerateSimple(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to verify TOC entry: %w", err)
	}

	var result VerificationResult
	if err := parseLLMJSONResponse(response, &result); err != nil {
		log.Warn().Err(err).Str("response", response).Msg("Failed to parse verification response")
		return &VerificationResult{Verified: false}, nil
	}

	return &result, nil
}

// VerifyTOC verifies all TOC entries against document content
func (v *TOCVerifier) VerifyTOC(ctx context.Context, toc *TOCResult, pages []string) (*VerificationResult, error) {
	if len(toc.Items) == 0 {
		return &VerificationResult{Verified: true}, nil
	}

	var allErrors []string
	verified := true

	for i, item := range toc.Items {
		if item.PhysicalIndex == nil {
			log.Warn().Int("index", i).Str("title", item.Title).Msg("TOC item has no physical index, skipping verification")
			continue
		}

		// Get the page content for this TOC entry
		pageIdx := *item.PhysicalIndex
		if pageIdx < 1 || pageIdx > len(pages) {
			errMsg := fmt.Sprintf("TOC item '%s' has invalid physical index %d (document has %d pages)",
				item.Title, pageIdx, len(pages))
			allErrors = append(allErrors, errMsg)
			verified = false
			continue
		}

		pageContent := pages[pageIdx-1] // Convert to 0-based index

		result, err := v.verifyTOCEntry(ctx, item, pageContent)
		if err != nil {
			log.Warn().Err(err).Str("title", item.Title).Msg("Failed to verify TOC entry")
			// Don't fail entire verification for single entry errors
			continue
		}

		if !result.Verified {
			verified = false
			allErrors = append(allErrors, result.Errors...)
		}
	}

	return &VerificationResult{
		Verified: verified,
		Errors:   allErrors,
	}, nil
}

// fixIncorrectTOCEntry asks LLM to fix an incorrect TOC entry
func (v *TOCVerifier) fixIncorrectTOCEntry(ctx context.Context, item TOCItem, pageContent string) (*TOCItem, error) {
	prompt := fmt.Sprintf(`You are given an incorrect TOC entry and the actual page content from a document.
Your job is to fix the TOC entry to match the actual content.

Incorrect TOC entry:
- Structure: %s
- Title: %s

Page content:
%s

Respond in the following JSON format:
{
    "structure": "correct structure",
    "title": "correct title"
}

Directly return the final JSON structure. Do not output anything else.`,
		item.Structure, item.Title, pageContent)

	response, err := v.llmClient.GenerateSimple(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to fix TOC entry: %w", err)
	}

	var fixedItem struct {
		Structure string `json:"structure"`
		Title     string `json:"title"`
	}
	if err := parseLLMJSONResponse(response, &fixedItem); err != nil {
		return nil, fmt.Errorf("failed to parse fix response: %w", err)
	}

	return &TOCItem{
		Structure:     fixedItem.Structure,
		Title:         fixedItem.Title,
		Page:          item.Page,
		PhysicalIndex: item.PhysicalIndex,
	}, nil
}

// FixIncorrectTOC fixes all incorrect TOC entries
func (v *TOCVerifier) FixIncorrectTOC(ctx context.Context, toc *TOCResult, pages []string) error {
	if len(toc.Items) == 0 {
		return nil
	}

	for i := range toc.Items {
		item := &toc.Items[i]
		if item.PhysicalIndex == nil {
			continue
		}

		pageIdx := *item.PhysicalIndex
		if pageIdx < 1 || pageIdx > len(pages) {
			continue
		}

		pageContent := pages[pageIdx-1]

		// Verify this entry
		verification, err := v.verifyTOCEntry(ctx, *item, pageContent)
		if err != nil {
			log.Warn().Err(err).Str("title", item.Title).Msg("Failed to verify TOC entry during fix")
			continue
		}

		// Only fix if verification failed
		if !verification.Verified {
			fixedItem, err := v.fixIncorrectTOCEntry(ctx, *item, pageContent)
			if err != nil {
				log.Warn().Err(err).Str("title", item.Title).Msg("Failed to fix TOC entry")
				continue
			}

			log.Info().
				Str("old_title", item.Title).
				Str("new_title", fixedItem.Title).
				Msg("Fixed TOC entry")
			toc.Items[i] = *fixedItem
		}
	}

	return nil
}

// FixIncorrectTOCWithRetries fixes incorrect TOC entries with multiple retry attempts
func (v *TOCVerifier) FixIncorrectTOCWithRetries(ctx context.Context, toc *TOCResult, pages []string, maxRetries int) error {
	if len(toc.Items) == 0 {
		return nil
	}

	if maxRetries <= 0 {
		maxRetries = 3 // Default to 3 retries
	}

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Info().Int("attempt", attempt).Msg("Attempting to fix incorrect TOC entries")

		// Fix all incorrect entries
		if err := v.FixIncorrectTOC(ctx, toc, pages); err != nil {
			return fmt.Errorf("failed to fix incorrect TOC on attempt %d: %w", attempt, err)
		}

		// Verify if all entries are now correct
		verification, err := v.VerifyTOC(ctx, toc, pages)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to verify TOC after fix attempt")
			continue
		}

		if verification.Verified {
			log.Info().Int("attempt", attempt).Msg("TOC verification passed")
			return nil
		}

		if attempt < maxRetries {
			log.Warn().
				Int("attempt", attempt).
				Int("max_retries", maxRetries).
				Strs("errors", verification.Errors).
				Msg("TOC verification failed, will retry")
		}
	}

	return fmt.Errorf("failed to fix TOC after %d attempts", maxRetries)
}
