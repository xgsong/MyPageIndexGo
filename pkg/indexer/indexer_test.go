package indexer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/tokenizer"
)

func TestTransformDotsToColon(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "multiple dots to colon",
			input:    "Chapter 1..... Section 1.1",
			expected: "Chapter 1:  Section 1.1",
		},
		{
			name:     "dots with spaces to colon",
			input:    "Section 1. . . . .  Chapter 2",
			expected: "Section 1:  Chapter 2",
		},
		{
			name:     "no change needed",
			input:    "Normal text without dots",
			expected: "Normal text without dots",
		},
		{
			name:     "single dot no change",
			input:    "Section 1.2",
			expected: "Section 1.2",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := transformDotsToColon(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAddPageTags(t *testing.T) {
	content := "This is page content"
	result := addPageTags(content, 5)
	expected := "【第5页开始】\nThis is page content\n【第5页结束】\n\n"
	assert.Equal(t, expected, result)
}

func TestBuildContentWithTags(t *testing.T) {
	pages := []string{"Page 1 content", "Page 2 content", "Page 3 content"}
	result := buildContentWithTags(pages, 1)

	assert.Contains(t, result, "【第1页开始】")
	assert.Contains(t, result, "Page 1 content")
	assert.Contains(t, result, "【第2页开始】")
	assert.Contains(t, result, "Page 2 content")
	assert.Contains(t, result, "【第3页开始】")
	assert.Contains(t, result, "Page 3 content")
}

func TestBuildContentWithTags_StartingIndex(t *testing.T) {
	pages := []string{"Page 5 content", "Page 6 content"}
	result := buildContentWithTags(pages, 5)

	assert.Contains(t, result, "【第5页开始】")
	assert.Contains(t, result, "【第6页开始】")
}

func TestConvertPhysicalIndexToInt(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    int
		expectError bool
	}{
		{"standard format", "<physical_index_5>", 5, false},
		{"without brackets", "physical_index_5", 5, false},
		{"with whitespace", "  <physical_index_10>  ", 10, false},
		{"plain number", "42", 42, false},
		{"Chinese format", "【第7页开始】", 7, false},
		{"Chinese format end", "【第8页结束】", 8, false},
		{"invalid format", "invalid", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertPhysicalIndexToInt(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestFilterValidItems(t *testing.T) {
	mp := &MetaProcessor{}

	idx1 := 1
	idx2 := 2
	page1 := 10

	items := []TOCItem{
		{Title: "Valid 1", PhysicalIndex: &idx1},
		{Title: "Nil Physical", PhysicalIndex: nil, Page: &page1},
		{Title: "Both Nil", PhysicalIndex: nil, Page: nil},
		{Title: "Valid 2", PhysicalIndex: &idx2},
	}

	result := mp.filterValidItems(items)

	assert.Len(t, result, 3)
	assert.Equal(t, "Valid 1", result[0].Title)
	assert.Equal(t, "Nil Physical", result[1].Title)
	assert.Equal(t, "Valid 2", result[2].Title)
}

func TestValidateAndTruncatePhysicalIndices(t *testing.T) {
	mp := &MetaProcessor{}

	idx1 := 5
	idx2 := 15
	idx3 := 25

	items := []TOCItem{
		{Title: "Normal", PhysicalIndex: &idx1},
		{Title: "Below Range", PhysicalIndex: &idx2},
		{Title: "Above Range", PhysicalIndex: &idx3},
	}

	result := mp.validateAndTruncatePhysicalIndices(items, 20, 1)

	assert.Equal(t, 5, *result[0].PhysicalIndex)
	assert.Equal(t, 15, *result[1].PhysicalIndex)
	assert.Equal(t, 20, *result[2].PhysicalIndex)
}

func TestDeepCopyTOCItems(t *testing.T) {
	mp := &MetaProcessor{}

	idx1 := 1
	idx2 := 2
	page1 := 10

	items := []TOCItem{
		{
			Title:         "Item 1",
			Structure:     "1",
			PhysicalIndex: &idx1,
			Page:          &page1,
		},
		{
			Title:         "Item 2",
			Structure:     "2",
			PhysicalIndex: &idx2,
		},
	}

	copy := mp.deepCopyTOCItems(items)

	assert.Len(t, copy, 2)
	assert.Equal(t, items[0].Title, copy[0].Title)
	assert.Equal(t, *items[0].PhysicalIndex, *copy[0].PhysicalIndex)
	assert.Equal(t, *items[0].Page, *copy[0].Page)

	*copy[0].PhysicalIndex = 100
	assert.Equal(t, idx1, *items[0].PhysicalIndex)
}

func TestSamplePages(t *testing.T) {
	mp := &MetaProcessor{}

	pages := []string{"Page 1", "Page 2", "Page 3", "Page 4", "Page 5"}

	result := mp.samplePages(pages, 2, 3)

	assert.Contains(t, result, "【第2页开始】")
	assert.Contains(t, result, "Page 2")
	assert.Contains(t, result, "【第3页开始】")
	assert.Contains(t, result, "Page 3")
	assert.Contains(t, result, "【第4页开始】")
	assert.Contains(t, result, "Page 4")
	assert.Contains(t, result, "【第5页开始】")
	assert.Contains(t, result, "Page 5")
}

func TestSamplePages_ExceedsBounds(t *testing.T) {
	mp := &MetaProcessor{}

	pages := []string{"Page 1", "Page 2", "Page 3"}

	result := mp.samplePages(pages, 2, 10)

	assert.Contains(t, result, "Page 2")
	assert.Contains(t, result, "Page 3")
	assert.NotContains(t, result, "Page 4")
}

func TestExtractMatchingPagePairs(t *testing.T) {
	mp := &MetaProcessor{}

	page1 := 1
	page2 := 5
	phyIdx1 := 10
	phyIdx2 := 50

	tocWithPages := []TOCItem{
		{Title: "Chapter 1", Page: &page1},
		{Title: "Chapter 2", Page: &page2},
		{Title: "Chapter 3", Page: nil},
	}

	tocWithPhysical := []TOCItem{
		{Title: "Chapter 1", PhysicalIndex: &phyIdx1},
		{Title: "Chapter 2", PhysicalIndex: &phyIdx2},
		{Title: "Chapter 3", PhysicalIndex: nil},
	}

	pairs := mp.extractMatchingPagePairs(tocWithPages, tocWithPhysical, 1)

	assert.Len(t, pairs, 2)
	assert.Equal(t, "Chapter 1", pairs[0].Title)
	assert.Equal(t, 1, pairs[0].Page)
	assert.Equal(t, 10, pairs[0].PhysicalIndex)
	assert.Equal(t, "Chapter 2", pairs[1].Title)
}

func TestCalculatePageOffset(t *testing.T) {
	mp := &MetaProcessor{}

	t.Run("no pairs", func(t *testing.T) {
		result := mp.calculatePageOffset([]PageIndexPair{}, 100)
		assert.Nil(t, result)
	})

	t.Run("single pair", func(t *testing.T) {
		pairs := []PageIndexPair{
			{Page: 10, PhysicalIndex: 15},
		}
		result := mp.calculatePageOffset(pairs, 100)
		assert.NotNil(t, result)
		assert.Equal(t, 5, *result)
	})

	t.Run("multiple pairs with consistent offset", func(t *testing.T) {
		pairs := []PageIndexPair{
			{Page: 10, PhysicalIndex: 15},
			{Page: 20, PhysicalIndex: 25},
			{Page: 30, PhysicalIndex: 35},
		}
		result := mp.calculatePageOffset(pairs, 100)
		assert.NotNil(t, result)
		assert.Equal(t, 5, *result)
	})

	t.Run("multiple pairs with different offsets", func(t *testing.T) {
		pairs := []PageIndexPair{
			{Page: 10, PhysicalIndex: 15},
			{Page: 20, PhysicalIndex: 22},
			{Page: 30, PhysicalIndex: 35},
		}
		result := mp.calculatePageOffset(pairs, 100)
		assert.NotNil(t, result)
		assert.Equal(t, 5, *result)
	})
}

func TestTOCItem_Structure(t *testing.T) {
	item := TOCItem{
		Title:     "Test",
		Structure: "1.2.3",
	}

	idx := 5
	item.PhysicalIndex = &idx

	assert.Equal(t, "Test", item.Title)
	assert.Equal(t, "1.2.3", item.Structure)
	assert.Equal(t, 5, *item.PhysicalIndex)
}

func TestMergeTOCItems(t *testing.T) {
	mp := &MetaProcessor{}

	existing := []TOCItem{
		{Title: "Existing 1", Structure: "1"},
		{Title: "Existing 2", Structure: "2"},
	}

	new := []TOCItem{
		{Title: "New 1", Structure: "3"},
		{Title: "New 2", Structure: "4"},
	}

	result := mp.mergeTOCItems(existing, new)

	assert.Len(t, result, 4)
	assert.Equal(t, "Existing 1", result[0].Title)
	assert.Equal(t, "New 2", result[3].Title)
}

func TestMergeTOCItems_WithDuplicates(t *testing.T) {
	mp := &MetaProcessor{}

	existing := []TOCItem{
		{Title: "Item 1", Structure: "1"},
		{Title: "Item 2", Structure: "2"},
	}

	new := []TOCItem{
		{Title: "Item 2", Structure: "2"},
		{Title: "Item 3", Structure: "3"},
	}

	result := mp.mergeTOCItems(existing, new)

	assert.Len(t, result, 3)
}

func TestCalculateOptimalBatchSize(t *testing.T) {
	tests := []struct {
		name           string
		nodeCount      int
		totalTokens    int
		expectedBatch  int
		expectedTokens int
	}{
		{"few nodes", 3, 5000, 3, 100000},
		{"many nodes", 100, 100000, 10, 10000},
		{"very many nodes", 1000, 100000, 50, 10000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batchSize, tokensPerBatch := calculateOptimalBatchSize(tt.nodeCount, tt.totalTokens)
			assert.Equal(t, tt.expectedBatch, batchSize)
			assert.Equal(t, tt.expectedTokens, tokensPerBatch)
		})
	}
}

func TestTOCDetector_NewTOCDetector(t *testing.T) {
	mockLLM := &mockLLMClient{}
	cfg := config.DefaultConfig()
	detector := NewTOCDetector(mockLLM, cfg)
	assert.NotNil(t, detector)
	assert.Equal(t, mockLLM, detector.llmClient)
	assert.Equal(t, cfg, detector.cfg)
}

func TestNewPageGrouperWithOverlap(t *testing.T) {
	tok, err := newTokenizerForTest("gpt-4o")
	if err != nil {
		t.Skip("Cannot create tokenizer for test")
	}

	grouper := NewPageGrouperWithOverlap(tok, 1000, 5)
	assert.NotNil(t, grouper)
	assert.Equal(t, 5, grouper.overlapPages)
}

func newTokenizerForTest(model string) (*tokenizer.Tokenizer, error) {
	return tokenizer.NewTokenizer(model)
}
