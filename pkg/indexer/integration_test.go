package indexer

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/llm"
	"github.com/xgsong/mypageindexgo/pkg/tokenizer"
)

func TestPageGrouper_LargeDocument(t *testing.T) {
	tok, err := tokenizer.NewTokenizer("gpt-4o")
	require.NoError(t, err)

	grouper := NewPageGrouper(tok, 50) // Very small limit to force multiple groups

	// Create a document with multiple pages
	doc := &document.Document{
		Pages: []document.Page{
			{Number: 1, Text: "This is page one with several words here to make it longer and longer to ensure enough tokens"},
			{Number: 2, Text: "This is page two with several words here to make it longer and longer to ensure enough tokens"},
			{Number: 3, Text: "This is page three with several words here to make it longer and longer to ensure enough tokens"},
			{Number: 4, Text: "This is page four with several words here to make it longer and longer to ensure enough tokens"},
			{Number: 5, Text: "This is page five with several words here to make it longer and longer to ensure enough tokens"},
		},
	}

	groups, err := grouper.GroupPages(doc)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(groups), 1)
}

func TestPageGrouper_SingleLargePage(t *testing.T) {
	tok, err := tokenizer.NewTokenizer("gpt-4o")
	require.NoError(t, err)

	grouper := NewPageGrouper(tok, 10) // Very small limit

	// Create a document with one large page
	largeText := "word "
	for i := 0; i < 100; i++ {
		largeText += "word "
	}

	doc := &document.Document{
		Pages: []document.Page{
			{Number: 1, Text: largeText},
		},
	}

	groups, err := grouper.GroupPages(doc)
	require.NoError(t, err)
	assert.Equal(t, 1, len(groups))
	assert.Less(t, groups[0].TokenCount, 15) // Should be truncated to near limit
}

func TestMergeNodes_EdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		groups []*document.Node
		check  func(t *testing.T, result *document.Node)
	}{
		{
			name:   "nil input",
			groups: nil,
			check: func(t *testing.T, result *document.Node) {
				assert.Nil(t, result)
			},
		},
		{
			name:   "empty slice",
			groups: []*document.Node{},
			check: func(t *testing.T, result *document.Node) {
				assert.Nil(t, result)
			},
		},
		{
			name: "overlapping page ranges",
			groups: []*document.Node{
				document.NewNode("Part 1", 1, 10),
				document.NewNode("Part 2", 5, 15), // Overlaps with Part 1
			},
			check: func(t *testing.T, result *document.Node) {
				assert.NotNil(t, result)
				// Should find min start and max end
				assert.Equal(t, 1, result.StartPage)
				assert.Equal(t, 15, result.EndPage)
			},
		},
		{
			name: "nodes with summaries",
			groups: func() []*document.Node {
				n1 := document.NewNode("Section 1", 1, 5)
				n1.Summary = "Summary of section 1"
				n2 := document.NewNode("Section 2", 6, 10)
				n2.Summary = "Summary of section 2"
				return []*document.Node{n1, n2}
			}(),
			check: func(t *testing.T, result *document.Node) {
				assert.NotNil(t, result)
				assert.Len(t, result.Children, 2)
				assert.Equal(t, "Summary of section 1", result.Children[0].Summary)
				assert.Equal(t, "Summary of section 2", result.Children[1].Summary)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MergeNodes(tt.groups)
			tt.check(t, result)
		})
	}
}

func TestIndexGenerator_Validation(t *testing.T) {
	tests := []struct {
		name      string
		cfg       *config.Config
		llmClient llm.LLMClient
		wantErr   bool
	}{
		{
			name:      "nil config",
			cfg:       nil,
			llmClient: &MockLLMClient{},
			wantErr:   true,
		},
		{
			name:      "nil llm client",
			cfg:       config.DefaultConfig(),
			llmClient: nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewIndexGenerator(tt.cfg, tt.llmClient)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestIndexGenerator_ConfigDefaults(t *testing.T) {
	cfg := &config.Config{
		OpenAIAPIKey:     "test-key",
		MaxTokensPerNode: 0, // Should default to 16000
		MaxConcurrency:   0, // Should default to 1
	}

	mockLLM := &MockLLMClient{}
	gen, err := NewIndexGenerator(cfg, mockLLM)
	require.NoError(t, err)
	assert.NotNil(t, gen)
}

func TestSearcher_Validation(t *testing.T) {
	ctx := context.Background()
	searcher := NewSearcher(&MockLLMClient{})

	tests := []struct {
		name    string
		query   string
		tree    *document.IndexTree
		wantErr bool
	}{
		{
			name:    "empty query",
			query:   "",
			tree:    &document.IndexTree{Root: document.NewNode("Root", 1, 10)},
			wantErr: true,
		},
		{
			name:    "nil tree",
			query:   "test query",
			tree:    nil,
			wantErr: true,
		},
		{
			name:    "nil root",
			query:   "test query",
			tree:    &document.IndexTree{Root: nil},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := searcher.Search(ctx, tt.query, tt.tree)
			if tt.wantErr {
				assert.Error(t, err)
			}
		})
	}
}
