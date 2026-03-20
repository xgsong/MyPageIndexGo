package llm

import (
	"context"

	"github.com/xgsong/mypageindexgo/pkg/document"
)

// LLMClient is the interface for LLM providers.
// Implementations: OpenAI, Anthropic, etc.
type LLMClient interface {
	// GenerateStructure generates a hierarchical tree structure from raw page text.
	// Returns a root Node with children representing the semantic structure.
	GenerateStructure(ctx context.Context, text string) (*document.Node, error)

	// GenerateSummary generates a concise summary for a node that captures its key content.
	GenerateSummary(ctx context.Context, nodeTitle string, text string) (string, error)

	// Search performs reasoning-based retrieval on the index tree given a query.
	// Returns a search result with answer and relevant nodes.
	Search(ctx context.Context, query string, tree *document.IndexTree) (*document.SearchResult, error)
}
