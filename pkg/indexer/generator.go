package indexer

import (
	"context"
	"fmt"
	"strings"

	"golang.org/x/sync/errgroup"

	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/llm"
	"github.com/xgsong/mypageindexgo/pkg/tokenizer"
)

// IndexGenerator coordinates the full index generation process.
type IndexGenerator struct {
	cfg         *config.Config
	llmClient   llm.LLMClient
	tokenizer   *tokenizer.Tokenizer
	pageGrouper *PageGrouper
	doc         *document.Document // Original document for summary generation
}

// NewIndexGenerator creates a new IndexGenerator.
func NewIndexGenerator(cfg *config.Config, llmClient llm.LLMClient) (*IndexGenerator, error) {
	if cfg == nil {
		return nil, fmt.Errorf("nil config")
	}
	if llmClient == nil {
		return nil, fmt.Errorf("nil LLM client")
	}

	tok, err := tokenizer.NewTokenizer(cfg.OpenAIModel)
	if err != nil {
		return nil, fmt.Errorf("failed to create tokenizer: %w", err)
	}

	// Get max tokens per group from config
	maxTokens := cfg.MaxTokensPerNode
	if maxTokens <= 0 {
		maxTokens = 16000
	}


	pageGrouper := NewPageGrouper(tok, maxTokens)

	return &IndexGenerator{
		cfg:         cfg,
		llmClient:   llmClient,
		tokenizer:   tok,
		pageGrouper: pageGrouper,
	}, nil
}

// Generate generates a complete index tree from a parsed document.
// It performs the full process: grouping → parallel structure generation → merging → (optional) summary generation.
func (g *IndexGenerator) Generate(ctx context.Context, doc *document.Document) (*document.IndexTree, error) {
	// Store reference to original document for summary generation
	g.doc = doc

	// Step 1: Group pages into token-limited chunks
	groups, err := g.pageGrouper.GroupPages(doc)
	if err != nil {
		return nil, fmt.Errorf("failed to group pages: %w", err)
	}

	if len(groups) == 0 {
		return nil, fmt.Errorf("no content found in document: all pages are empty or contain no extractable text")
	}

	// Step 2: Generate structure for each group in parallel
	nodes, err := g.generateStructures(ctx, groups)
	if err != nil {
		return nil, fmt.Errorf("failed to generate structures: %w", err)
	}

	// Step 3: Merge all nodes into a single tree
	root := MergeNodes(nodes)
	if root == nil {
		return nil, fmt.Errorf("failed to merge nodes: no root generated")
	}

	// Step 4: Create the index tree
	tree := document.NewIndexTree(root, len(doc.Pages))
	tree.DocumentInfo = fmt.Sprintf("Document with %d pages", len(doc.Pages))

	// Step 5: Generate summaries if enabled (this is optional and can be slow)
	if g.cfg.GenerateSummaries {
		if err := g.generateAllSummaries(ctx, root); err != nil {
			return nil, fmt.Errorf("failed to generate summaries: %w", err)
		}
	}

	return tree, nil
}

// generateStructures generates the tree structure for each page group in parallel.
func (g *IndexGenerator) generateStructures(ctx context.Context, groups []*PageGroup) ([]*document.Node, error) {
	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(g.cfg.MaxConcurrency)

	nodes := make([]*document.Node, len(groups))

	for i, group := range groups {
		i := i
		group := group
		eg.Go(func() error {
			node, err := g.llmClient.GenerateStructure(ctx, group.Text)
			if err != nil {
				return fmt.Errorf("group %d (%d-%d): failed to generate structure: %w", i+1, group.StartPage, group.EndPage, err)
			}
			nodes[i] = node
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	return nodes, nil
}

// generateAllSummaries generates summaries for all nodes in the tree recursively in parallel.
// It extracts the relevant text from the original document for each node based on page ranges.
// First collects all nodes that need summaries, then processes them in parallel with controlled concurrency.
func (g *IndexGenerator) generateAllSummaries(ctx context.Context, root *document.Node) error {
	// Build page number → text map to handle non-contiguous page numbers (e.g. PDFs with null pages)
	pageTextByNum := make(map[int]string, len(g.doc.Pages))
	for _, p := range g.doc.Pages {
		pageTextByNum[p.Number] = p.Text
	}

	// First pass: collect all nodes that need summaries
	var nodesToProcess []*document.Node
	var collect func(*document.Node)
	collect = func(node *document.Node) {
		if node == nil {
			return
		}
		if node.Summary == "" {
			nodesToProcess = append(nodesToProcess, node)
		}
		for _, child := range node.Children {
			collect(child)
		}
	}
	collect(root)

	if len(nodesToProcess) == 0 {
		return nil
	}

	// Second pass: process all nodes in parallel with controlled concurrency
	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(max(1, g.cfg.MaxConcurrency))

	for _, node := range nodesToProcess {
		node := node // capture
		eg.Go(func() error {
			// Extract text for this node using page number lookup
			var nodeText strings.Builder
			for pageNum := node.StartPage; pageNum <= node.EndPage; pageNum++ {
				if text, ok := pageTextByNum[pageNum]; ok {
					nodeText.WriteString(text)
					nodeText.WriteString("\n\n")
				}
			}

			if nodeText.Len() > 0 {
				if err := g.GenerateSummariesForNode(ctx, node, nodeText.String()); err != nil {
					return err
				}
			}
			return nil
		})
	}

	return eg.Wait()
}

// GenerateSummariesForNode generates a summary for a specific node given its text content.
func (g *IndexGenerator) GenerateSummariesForNode(ctx context.Context, node *document.Node, text string) error {
	if node == nil {
		return fmt.Errorf("nil node")
	}
	if text == "" {
		return nil // nothing to summarize
	}

	summary, err := g.llmClient.GenerateSummary(ctx, node.Title, text)
	if err != nil {
		return fmt.Errorf("failed to generate summary: %w", err)
	}

	node.Summary = summary
	return nil
}
