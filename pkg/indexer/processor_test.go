package indexer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/tokenizer"
)

func TestNewPageGrouper(t *testing.T) {
	tok, err := tokenizer.NewTokenizer("gpt-4o")
	assert.NoError(t, err)

	grouper := NewPageGrouper(tok, 1000)
	assert.NotNil(t, grouper)
	assert.Equal(t, 1000, grouper.maxTokens)
}

func TestPageGrouper_GroupPages_Empty(t *testing.T) {
	tok, err := tokenizer.NewTokenizer("gpt-4o")
	assert.NoError(t, err)

	grouper := NewPageGrouper(tok, 1000)
	doc := &document.Document{
		Pages: []document.Page{},
	}
	groups, err := grouper.GroupPages(doc)
	assert.Error(t, err)
	assert.Nil(t, groups)
}

func TestPageGrouper_GroupPages_SinglePage(t *testing.T) {
	tok, err := tokenizer.NewTokenizer("gpt-4o")
	assert.NoError(t, err)

	grouper := NewPageGrouper(tok, 1000)
	doc := &document.Document{
		Pages: []document.Page{
			{
				Number: 1,
				Text:   "This is a test page with some content.",
			},
		},
	}
	groups, err := grouper.GroupPages(doc)
	assert.NoError(t, err)
	assert.Len(t, groups, 1)
	assert.Equal(t, 1, groups[0].StartPage)
	assert.Equal(t, 1, groups[0].EndPage)
	assert.Contains(t, groups[0].Text, "test page")
}

func TestPageGrouper_GroupPages_MultiplePagesWithinLimit(t *testing.T) {
	tok, err := tokenizer.NewTokenizer("gpt-4o")
	assert.NoError(t, err)

	grouper := NewPageGrouper(tok, 1000)
	doc := &document.Document{
		Pages: []document.Page{
			{Number: 1, Text: "Page 1 content."},
			{Number: 2, Text: "Page 2 content."},
			{Number: 3, Text: "Page 3 content."},
		},
	}
	groups, err := grouper.GroupPages(doc)
	assert.NoError(t, err)
	assert.Len(t, groups, 1)
	assert.Equal(t, 1, groups[0].StartPage)
	assert.Equal(t, 3, groups[0].EndPage)
	assert.Contains(t, groups[0].Text, "Page 1")
	assert.Contains(t, groups[0].Text, "Page 2")
	assert.Contains(t, groups[0].Text, "Page 3")
}

func TestPageGrouper_GroupPages_TruncateLargePage(t *testing.T) {
	tok, err := tokenizer.NewTokenizer("gpt-4o")
	assert.NoError(t, err)

	// Create a very large text that exceeds max tokens
	largeText := ""
	for i := 0; i < 1000; i++ {
		largeText += "word "
	}

	grouper := NewPageGrouper(tok, 100)
	doc := &document.Document{
		Pages: []document.Page{
			{Number: 1, Text: largeText},
		},
	}
	groups, err := grouper.GroupPages(doc)
	assert.NoError(t, err)
	assert.Len(t, groups, 1)
	assert.Equal(t, 100, groups[0].TokenCount)
	assert.LessOrEqual(t, tok.Count(groups[0].Text), 100)
}

func TestPageGrouper_GroupPages_MultipleGroups(t *testing.T) {
	tok, err := tokenizer.NewTokenizer("gpt-4o")
	assert.NoError(t, err)

	// Each page ~10 tokens, max 25 tokens per group → should get 2 groups
	// Using 5 pages to ensure multiple groups (more than overlap*2=4)
	grouper := NewPageGrouper(tok, 25)
	doc := &document.Document{
		Pages: []document.Page{
			{Number: 1, Text: "one two three four five six seven eight nine ten"},
			{Number: 2, Text: "eleven twelve thirteen fourteen fifteen sixteen"},
			{Number: 3, Text: "seventeen eighteen nineteen twenty twentyone twentytwo"},
			{Number: 4, Text: "twentythree twentyfour twentyfive twentysix twentyseven"},
			{Number: 5, Text: "twentyeight twentynine thirty thirtyone thirtytwo"},
		},
	}
	groups, err := grouper.GroupPages(doc)
	assert.NoError(t, err)
	assert.True(t, len(groups) >= 2)
}

func TestMergeNodes_Empty(t *testing.T) {
	result := MergeNodes([]*document.Node{})
	assert.Nil(t, result)
}

func TestMergeNodes_Single(t *testing.T) {
	node := document.NewNode("Test", 1, 10)
	result := MergeNodes([]*document.Node{node})
	assert.Same(t, node, result)
}

func TestMergeNodes_Multiple(t *testing.T) {
	node1 := document.NewNode("Chapter 1", 1, 5)
	node1.AddChild(document.NewNode("Section 1.1", 1, 3))

	node2 := document.NewNode("Chapter 2", 6, 10)
	node2.AddChild(document.NewNode("Section 2.1", 6, 8))

	result := MergeNodes([]*document.Node{node1, node2})
	assert.NotNil(t, result)
	assert.Equal(t, "Document", result.Title)
	assert.Equal(t, 1, result.StartPage)
	assert.Equal(t, 10, result.EndPage)
	assert.Len(t, result.Children, 2)
	// MergeNodes extracts children from each group root, so we get the sections not the chapters
	assert.Equal(t, "Section 1.1", result.Children[0].Title)
	assert.Equal(t, "Section 2.1", result.Children[1].Title)
}

func TestMergeNodes_WithChildrenExtraction(t *testing.T) {
	// If group roots already have children, extract them directly
	root1 := document.NewNode("Root1", 1, 5)
	root1.AddChild(document.NewNode("Child A", 1, 3))
	root1.AddChild(document.NewNode("Child B", 3, 5))

	root2 := document.NewNode("Root2", 6, 10)
	root2.AddChild(document.NewNode("Child C", 6, 8))

	result := MergeNodes([]*document.Node{root1, root2})
	assert.NotNil(t, result)
	assert.Len(t, result.Children, 3) // 2 from root1 + 1 from root2 = 3
}
