package indexer

import (
	"context"
	"fmt"
	"strings"

	"github.com/rs/zerolog/log"
	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/tokenizer"
)

// MergeNodes merges multiple node trees from different page groups into a single coherent tree.
// Input nodes are cloned to avoid mutating the original trees.
func MergeNodes(groups []*document.Node) *document.Node {
	if len(groups) == 0 {
		return nil
	}

	if len(groups) == 1 {
		return document.CloneNode(groups[0])
	}

	merged := document.NewNode("Document", 1, 0)
	endPage := 0

	for _, group := range groups {
		merged.StartPage = min(group.StartPage, merged.StartPage)
		endPage = max(group.EndPage, endPage)
		if len(group.Children) > 0 {
			for _, child := range group.Children {
				merged.AddChild(document.CloneNode(child))
			}
		} else {
			merged.AddChild(document.CloneNode(group))
		}
	}

	merged.EndPage = endPage
	return merged
}

// ProcessLargeNodeRecursively processes a large node by recursively splitting it into smaller nodes.
// This is used when a node exceeds the maximum token size.
func ProcessLargeNodeRecursively(
	ctx context.Context,
	node *document.Node,
	pages []document.Page,
	tokenizer *tokenizer.Tokenizer,
	maxTokens int,
) (*document.Node, error) {
	if node == nil {
		return nil, fmt.Errorf("nil node")
	}

	// Calculate token count for this node
	var nodeText strings.Builder
	for _, page := range pages {
		if page.Number >= node.StartPage && page.Number <= node.EndPage {
			nodeText.WriteString(page.Text)
			nodeText.WriteString("\n\n")
		}
	}

	tokenCount := tokenizer.Count(nodeText.String())

	// If node is small enough, return as is
	if tokenCount <= maxTokens {
		return node, nil
	}

	log.Info().
		Str("node_id", node.ID).
		Str("title", node.Title).
		Int("tokens", tokenCount).
		Int("max_tokens", maxTokens).
		Msg("Node too large, will recursively process")

	// Calculate number of sub-nodes needed
	numSubNodes := (tokenCount / maxTokens) + 1
	pagesPerNode := len(pages) / numSubNodes
	if pagesPerNode < 1 {
		pagesPerNode = 1
	}

	// Split into smaller nodes
	var children []*document.Node
	currentStartPage := node.StartPage

	for i := 0; i < numSubNodes && currentStartPage <= node.EndPage; i++ {
		endPage := min(currentStartPage+pagesPerNode-1, node.EndPage)

		subNode := document.NewNode(
			fmt.Sprintf("%s (Part %d)", node.Title, i+1),
			currentStartPage,
			endPage,
		)

		children = append(children, subNode)
		currentStartPage = endPage + 1
	}

	// Replace node's children with the split nodes
	node.Children = children
	node.StartPage = children[0].StartPage
	node.EndPage = children[len(children)-1].EndPage

	log.Info().
		Str("node_id", node.ID).
		Int("sub_nodes", len(children)).
		Msg("Split large node into sub-nodes")

	return node, nil
}

// ProcessLargeNodesInTree recursively processes all large nodes in a tree.
func ProcessLargeNodesInTree(
	ctx context.Context,
	root *document.Node,
	pages []document.Page,
	tokenizer *tokenizer.Tokenizer,
	maxTokens int,
) error {
	if root == nil {
		return nil
	}

	// Process this node if it's large
	processedNode, err := ProcessLargeNodeRecursively(ctx, root, pages, tokenizer, maxTokens)
	if err != nil {
		return fmt.Errorf("failed to process large node %s: %w", root.ID, err)
	}

	// Recursively process children
	for _, child := range processedNode.Children {
		if err := ProcessLargeNodesInTree(ctx, child, pages, tokenizer, maxTokens); err != nil {
			return err
		}
	}

	return nil
}
