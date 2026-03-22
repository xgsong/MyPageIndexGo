package llm

import (
	"context"

	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/language"
)

// BatchSummaryRequest represents a single summary request in a batch.
type BatchSummaryRequest struct {
	NodeID    string `json:"node_id"`
	NodeTitle string `json:"node_title"`
	Text      string `json:"text"`
}

// BatchSummaryResponse represents a single summary response in a batch.
type BatchSummaryResponse struct {
	NodeID  string `json:"node_id"`
	Summary string `json:"summary"`
	Error   string `json:"error,omitempty"`
}

// LLMClient is the interface for LLM providers.
// Implementations: OpenAI, Anthropic, etc.
type LLMClient interface {
	// GenerateStructure generates a hierarchical tree structure from raw page text.
	// Returns a root Node with children representing the semantic structure.
	// The lang parameter ensures the LLM responds in the document's language.
	GenerateStructure(ctx context.Context, text string, lang language.Language) (*document.Node, error)

	// GenerateSummary generates a concise summary for a node that captures its key content.
	// The lang parameter ensures the summary is written in the document's language.
	GenerateSummary(ctx context.Context, nodeTitle string, text string, lang language.Language) (string, error)

	// Search performs reasoning-based retrieval on the index tree given a query.
	// Returns a search result with answer and relevant nodes.
	Search(ctx context.Context, query string, tree *document.IndexTree) (*document.SearchResult, error)

	// GenerateBatchSummaries generates summaries for multiple nodes in a single batch call.
	// Returns a slice of responses matching the order of requests.
	// The lang parameter ensures all summaries are written in the document's language.
	GenerateBatchSummaries(ctx context.Context, requests []*BatchSummaryRequest, lang language.Language) ([]*BatchSummaryResponse, error)
}
