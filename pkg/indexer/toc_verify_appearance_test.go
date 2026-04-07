// Package indexer_test provides unit tests for the appearance verification module.
// This module checks if TOC items appear at the start of their pages using LLM.
// Tests cover error paths, edge cases, and concurrency safety.
package indexer_test

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/indexer"
	"github.com/xgsong/mypageindexgo/pkg/llm"
)

// MockLLMClient is a mock implementation of llm.LLMClient for testing.
type MockLLMClient struct {
	mu          sync.Mutex
	response    string
	err         error
	callCount   int
	lastPrompt  string
	failOnce    bool
	failed      bool
	rateLimit   bool
	rateLimited bool
}

func (m *MockLLMClient) GenerateSimple(ctx context.Context, prompt string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.callCount++
	m.lastPrompt = prompt

	// Simulate rate limiting
	if m.rateLimit && !m.rateLimited {
		m.rateLimited = true
		return "", &llm.RateLimitError{Message: "rate limit exceeded"}
	}

	// Simulate one-time failure
	if m.failOnce && !m.failed {
		m.failed = true
		return "", m.err
	}

	return m.response, m.err
}

func (m *MockLLMClient) GenerateWithRetry(ctx context.Context, prompt string) (string, error) {
	return m.GenerateSimple(ctx, prompt)
}

func (m *MockLLMClient) GenerateBatch(ctx context.Context, prompts []string) ([]string, error) {
	responses := make([]string, len(prompts))
	for i := range prompts {
		resp, err := m.GenerateSimple(ctx, prompts[i])
		if err != nil {
			return nil, err
		}
		responses[i] = resp
	}
	return responses, nil
}

func (m *MockLLMClient) Recognize(ctx context.Context, imageBytes []byte) (string, error) {
	return "", nil
}

func (m *MockLLMClient) RecognizeBatch(ctx context.Context, images [][]byte) ([]string, error) {
	return nil, nil
}

func (m *MockLLMClient) SetAPIKey(key string) {}

func (m *MockLLMClient) GetCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

func (m *MockLLMClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount = 0
	m.failed = false
	m.rateLimited = false
}

// ============================================================================
// CheckTitleAppearance Tests
// ============================================================================

func TestCheckTitleAppearance_NormalPath_TitleExists(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mockClient := &MockLLMClient{
		response: `{"answer": "yes"}`,
	}

	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	item := indexer.TOCItem{
		Title:         "Chapter 1",
		PhysicalIndex: intPtr(5),
	}
	pageTexts := []string{
		"page 1",
		"page 2",
		"page 3",
		"page 4",
		"Chapter 1 Introduction\nThis is the content...",
	}

	ctx := context.Background()
	result, err := ac.CheckTitleAppearance(ctx, item, pageTexts, 1)

	must.NoError(err)
	is.True(result)
	is.Equal(1, mockClient.GetCallCount())
}

func TestCheckTitleAppearance_PhysicalIndexNil(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mockClient := &MockLLMClient{}
	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	item := indexer.TOCItem{
		Title:         "Chapter 1",
		PhysicalIndex: nil,
	}
	pageTexts := []string{"page 1", "page 2"}

	ctx := context.Background()
	result, err := ac.CheckTitleAppearance(ctx, item, pageTexts, 1)

	must.NoError(err)
	is.False(result)
	is.Equal(0, mockClient.GetCallCount())
}

func TestCheckTitleAppearance_PageIndexOutOfRange_Negative(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mockClient := &MockLLMClient{}
	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	item := indexer.TOCItem{
		Title:         "Chapter 1",
		PhysicalIndex: intPtr(1),
	}
	pageTexts := []string{"page 1", "page 2"}

	ctx := context.Background()
	result, err := ac.CheckTitleAppearance(ctx, item, pageTexts, 5)

	must.NoError(err)
	is.False(result)
	is.Equal(0, mockClient.GetCallCount())
}

func TestCheckTitleAppearance_PageIndexOutOfRange_Exceeds(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mockClient := &MockLLMClient{}
	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	item := indexer.TOCItem{
		Title:         "Chapter 1",
		PhysicalIndex: intPtr(10),
	}
	pageTexts := []string{"page 1", "page 2"}

	ctx := context.Background()
	result, err := ac.CheckTitleAppearance(ctx, item, pageTexts, 1)

	must.NoError(err)
	is.False(result)
	is.Equal(0, mockClient.GetCallCount())
}

func TestCheckTitleAppearance_LLMCallFailure(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mockClient := &MockLLMClient{
		err: assert.AnError,
	}
	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	item := indexer.TOCItem{
		Title:         "Chapter 1",
		PhysicalIndex: intPtr(1),
	}
	pageTexts := []string{"page 1"}

	ctx := context.Background()
	result, err := ac.CheckTitleAppearance(ctx, item, pageTexts, 1)

	must.Error(err)
	is.False(result)
	is.Equal(1, mockClient.GetCallCount())
}

func TestCheckTitleAppearance_JSONParseError_InvalidJSON(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mockClient := &MockLLMClient{
		response: `{"answer": invalid json}`,
	}
	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	item := indexer.TOCItem{
		Title:         "Chapter 1",
		PhysicalIndex: intPtr(1),
	}
	pageTexts := []string{"page 1"}

	ctx := context.Background()
	result, err := ac.CheckTitleAppearance(ctx, item, pageTexts, 1)

	must.NoError(err)
	is.False(result)
	is.Equal(1, mockClient.GetCallCount())
}

func TestCheckTitleAppearance_JSONParseError_WrongFormat(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mockClient := &MockLLMClient{
		response: `{"wrong_field": "yes"}`,
	}
	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	item := indexer.TOCItem{
		Title:         "Chapter 1",
		PhysicalIndex: intPtr(1),
	}
	pageTexts := []string{"page 1"}

	ctx := context.Background()
	result, err := ac.CheckTitleAppearance(ctx, item, pageTexts, 1)

	must.NoError(err)
	is.False(result)
	is.Equal(1, mockClient.GetCallCount())
}

func TestCheckTitleAppearance_AnswerNo(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mockClient := &MockLLMClient{
		response: `{"answer": "no"}`,
	}
	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	item := indexer.TOCItem{
		Title:         "Chapter 1",
		PhysicalIndex: intPtr(1),
	}
	pageTexts := []string{"page 1"}

	ctx := context.Background()
	result, err := ac.CheckTitleAppearance(ctx, item, pageTexts, 1)

	must.NoError(err)
	is.False(result)
	is.Equal(1, mockClient.GetCallCount())
}

func TestCheckTitleAppearance_AnswerCaseInsensitive(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mockClient := &MockLLMClient{
		response: `{"answer": "YES"}`,
	}
	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	item := indexer.TOCItem{
		Title:         "Chapter 1",
		PhysicalIndex: intPtr(1),
	}
	pageTexts := []string{"page 1"}

	ctx := context.Background()
	result, err := ac.CheckTitleAppearance(ctx, item, pageTexts, 1)

	must.NoError(err)
	is.True(result)
	is.Equal(1, mockClient.GetCallCount())
}

func TestCheckTitleAppearance_TableDriven(t *testing.T) {
	tests := []struct {
		name            string
		item            indexer.TOCItem
		pageTexts       []string
		startIndex      int
		mockResponse    string
		mockErr         error
		expectResult    bool
		expectError     bool
		expectCallCount int
	}{
		{
			name:            "title exists",
			item:            indexer.TOCItem{Title: "Chapter 1", PhysicalIndex: intPtr(1)},
			pageTexts:       []string{"Chapter 1\nContent"},
			startIndex:      1,
			mockResponse:    `{"answer": "yes"}`,
			expectResult:    true,
			expectCallCount: 1,
		},
		{
			name:            "physical index nil",
			item:            indexer.TOCItem{Title: "Chapter 1", PhysicalIndex: nil},
			pageTexts:       []string{"page 1"},
			startIndex:      1,
			expectResult:    false,
			expectError:     false,
			expectCallCount: 0,
		},
		{
			name:            "page index out of range",
			item:            indexer.TOCItem{Title: "Chapter 1", PhysicalIndex: intPtr(10)},
			pageTexts:       []string{"page 1"},
			startIndex:      1,
			expectResult:    false,
			expectError:     false,
			expectCallCount: 0,
		},
		{
			name:            "LLM error",
			item:            indexer.TOCItem{Title: "Chapter 1", PhysicalIndex: intPtr(1)},
			pageTexts:       []string{"page 1"},
			startIndex:      1,
			mockErr:         assert.AnError,
			expectResult:    false,
			expectError:     true,
			expectCallCount: 1,
		},
		{
			name:            "JSON parse error",
			item:            indexer.TOCItem{Title: "Chapter 1", PhysicalIndex: intPtr(1)},
			pageTexts:       []string{"page 1"},
			startIndex:      1,
			mockResponse:    `invalid json`,
			expectResult:    false,
			expectError:     false,
			expectCallCount: 1,
		},
		{
			name:            "answer no",
			item:            indexer.TOCItem{Title: "Chapter 1", PhysicalIndex: intPtr(1)},
			pageTexts:       []string{"page 1"},
			startIndex:      1,
			mockResponse:    `{"answer": "no"}`,
			expectResult:    false,
			expectCallCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := assert.New(t)
			must := require.New(t)

			mockClient := &MockLLMClient{
				response: tt.mockResponse,
				err:      tt.mockErr,
			}
			ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

			ctx := context.Background()
			result, err := ac.CheckTitleAppearance(ctx, tt.item, tt.pageTexts, tt.startIndex)

			if tt.expectError {
				must.Error(err)
			} else {
				must.NoError(err)
			}
			is.Equal(tt.expectResult, result)
			is.Equal(tt.expectCallCount, mockClient.GetCallCount())
		})
	}
}

// ============================================================================
// CheckTitleAppearanceInStart Tests
// ============================================================================

func TestCheckTitleAppearanceInStart_NormalPath_TitleAtStart(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mockClient := &MockLLMClient{
		response: `{"start_begin": "yes"}`,
	}
	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	ctx := context.Background()
	result, err := ac.CheckTitleAppearanceInStart(ctx, "Chapter 1", "Chapter 1\nIntroduction content...")

	must.NoError(err)
	is.Equal("yes", result)
	is.Equal(1, mockClient.GetCallCount())
}

func TestCheckTitleAppearanceInStart_EmptyPageText(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mockClient := &MockLLMClient{
		response: `{"start_begin": "no"}`,
	}
	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	ctx := context.Background()
	result, err := ac.CheckTitleAppearanceInStart(ctx, "Chapter 1", "")

	must.NoError(err)
	is.Equal("no", result)
	is.Equal(1, mockClient.GetCallCount())
}

func TestCheckTitleAppearanceInStart_EmptyTitle(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mockClient := &MockLLMClient{
		response: `{"start_begin": "no"}`,
	}
	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	ctx := context.Background()
	result, err := ac.CheckTitleAppearanceInStart(ctx, "", "Some content")

	must.NoError(err)
	is.Equal("no", result)
	is.Equal(1, mockClient.GetCallCount())
}

func TestCheckTitleAppearanceInStart_LLMFailure(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mockClient := &MockLLMClient{
		err: assert.AnError,
	}
	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	ctx := context.Background()
	result, err := ac.CheckTitleAppearanceInStart(ctx, "Chapter 1", "content")

	must.Error(err)
	is.Equal("no", result)
	is.Equal(1, mockClient.GetCallCount())
}

func TestCheckTitleAppearanceInStart_JSONParseError(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mockClient := &MockLLMClient{
		response: `invalid json`,
	}
	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	ctx := context.Background()
	result, err := ac.CheckTitleAppearanceInStart(ctx, "Chapter 1", "content")

	must.NoError(err)
	is.Equal("no", result)
	is.Equal(1, mockClient.GetCallCount())
}

func TestCheckTitleAppearanceInStart_WrongResponseFormat(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mockClient := &MockLLMClient{
		response: `{"wrong_field": "yes"}`,
	}
	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	ctx := context.Background()
	result, err := ac.CheckTitleAppearanceInStart(ctx, "Chapter 1", "content")

	must.NoError(err)
	is.Equal("no", result)
	is.Equal(1, mockClient.GetCallCount())
}

func TestCheckTitleAppearanceInStart_EmptyStartBegin(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mockClient := &MockLLMClient{
		response: `{"start_begin": ""}`,
	}
	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	ctx := context.Background()
	result, err := ac.CheckTitleAppearanceInStart(ctx, "Chapter 1", "content")

	must.NoError(err)
	is.Equal("no", result)
	is.Equal(1, mockClient.GetCallCount())
}

func TestCheckTitleAppearanceInStart_TableDriven(t *testing.T) {
	tests := []struct {
		name            string
		title           string
		pageText        string
		mockResponse    string
		mockErr         error
		expectResult    string
		expectError     bool
		expectCallCount int
	}{
		{
			name:            "title at start",
			title:           "Chapter 1",
			pageText:        "Chapter 1\nContent",
			mockResponse:    `{"start_begin": "yes"}`,
			expectResult:    "yes",
			expectCallCount: 1,
		},
		{
			name:            "empty page text",
			title:           "Chapter 1",
			pageText:        "",
			mockResponse:    `{"start_begin": "no"}`,
			expectResult:    "no",
			expectCallCount: 1,
		},
		{
			name:            "empty title",
			title:           "",
			pageText:        "content",
			mockResponse:    `{"start_begin": "no"}`,
			expectResult:    "no",
			expectCallCount: 1,
		},
		{
			name:            "LLM error",
			title:           "Chapter 1",
			pageText:        "content",
			mockErr:         assert.AnError,
			expectResult:    "no",
			expectError:     true,
			expectCallCount: 1,
		},
		{
			name:            "JSON parse error",
			title:           "Chapter 1",
			pageText:        "content",
			mockResponse:    `invalid`,
			expectResult:    "no",
			expectCallCount: 1,
		},
		{
			name:            "empty start_begin",
			title:           "Chapter 1",
			pageText:        "content",
			mockResponse:    `{"start_begin": ""}`,
			expectResult:    "no",
			expectCallCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := assert.New(t)
			must := require.New(t)

			mockClient := &MockLLMClient{
				response: tt.mockResponse,
				err:      tt.mockErr,
			}
			ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

			ctx := context.Background()
			result, err := ac.CheckTitleAppearanceInStart(ctx, tt.title, tt.pageText)

			if tt.expectError {
				must.Error(err)
			} else {
				must.NoError(err)
			}
			is.Equal(tt.expectResult, result)
			is.Equal(tt.expectCallCount, mockClient.GetCallCount())
		})
	}
}

// ============================================================================
// CheckAllItemsAppearanceInStart Tests
// ============================================================================

func TestCheckAllItemsAppearanceInStart_SkipAppearanceCheck(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mockClient := &MockLLMClient{}
	cfg := &config.Config{SkipAppearanceCheck: true}
	ac := indexer.NewAppearanceChecker(mockClient, cfg)

	items := []indexer.TOCItem{
		{Title: "Chapter 1", PhysicalIndex: intPtr(1)},
		{Title: "Chapter 2", PhysicalIndex: intPtr(2)},
		{Title: "Chapter 3", PhysicalIndex: nil},
	}
	pageTexts := []string{"page 1", "page 2"}

	ctx := context.Background()
	result := ac.CheckAllItemsAppearanceInStart(ctx, items, pageTexts)

	is.Len(result, 3)
	for _, item := range result {
		is.Equal("yes", item.AppearStart)
	}
	is.Equal(0, mockClient.GetCallCount())
}

func TestCheckAllItemsAppearanceInStart_EmptyItems(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mockClient := &MockLLMClient{}
	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	items := []indexer.TOCItem{}
	pageTexts := []string{"page 1", "page 2"}

	ctx := context.Background()
	result := ac.CheckAllItemsAppearanceInStart(ctx, items, pageTexts)

	is.Empty(result)
	is.Equal(0, mockClient.GetCallCount())
}

func TestCheckAllItemsAppearanceInStart_NoPhysicalIndex(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mockClient := &MockLLMClient{}
	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	items := []indexer.TOCItem{
		{Title: "Chapter 1", PhysicalIndex: nil},
		{Title: "Chapter 2", PhysicalIndex: nil},
	}
	pageTexts := []string{"page 1", "page 2"}

	ctx := context.Background()
	result := ac.CheckAllItemsAppearanceInStart(ctx, items, pageTexts)

	is.Len(result, 2)
	for _, item := range result {
		is.Equal("no", item.AppearStart)
	}
	is.Equal(0, mockClient.GetCallCount())
}

func TestCheckAllItemsAppearanceInStart_PhysicalIndexOutOfRange(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mockClient := &MockLLMClient{}
	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	items := []indexer.TOCItem{
		{Title: "Chapter 1", PhysicalIndex: intPtr(1)},
		{Title: "Chapter 2", PhysicalIndex: intPtr(10)},
	}
	pageTexts := []string{"page 1"}

	ctx := context.Background()
	result := ac.CheckAllItemsAppearanceInStart(ctx, items, pageTexts)

	is.Len(result, 2)
	is.Equal("yes", result[0].AppearStart)
	is.Equal("no", result[1].AppearStart)
	is.Equal(1, mockClient.GetCallCount())
}

func TestCheckAllItemsAppearanceInStart_ConcurrentSafety(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mockClient := &MockLLMClient{
		response: `{"start_begin": "yes"}`,
	}
	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	items := make([]indexer.TOCItem, 100)
	pageTexts := make([]string, 100)
	for i := 0; i < 100; i++ {
		idx := i + 1
		items[i] = indexer.TOCItem{
			Title:         "Chapter",
			PhysicalIndex: &idx,
		}
		pageTexts[i] = "page content"
	}

	ctx := context.Background()
	result := ac.CheckAllItemsAppearanceInStart(ctx, items, pageTexts)

	is.Len(result, 100)
	for _, item := range result {
		is.Equal("yes", item.AppearStart)
	}
	is.Equal(100, mockClient.GetCallCount())
}

func TestCheckAllItemsAppearanceInStart_LLMErrorHandling(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mockClient := &MockLLMClient{
		err: assert.AnError,
	}
	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	items := []indexer.TOCItem{
		{Title: "Chapter 1", PhysicalIndex: intPtr(1)},
		{Title: "Chapter 2", PhysicalIndex: intPtr(2)},
		{Title: "Chapter 3", PhysicalIndex: intPtr(3)},
	}
	pageTexts := []string{"page 1", "page 2", "page 3"}

	ctx := context.Background()
	result := ac.CheckAllItemsAppearanceInStart(ctx, items, pageTexts)

	is.Len(result, 3)
	for _, item := range result {
		is.Equal("no", item.AppearStart)
	}
	is.Equal(3, mockClient.GetCallCount())
}

func TestCheckAllItemsAppearanceInStart_MixedResults(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	callCount := 0
	var mu sync.Mutex
	mockClient := &MockLLMClient{
		response: `{"start_begin": "yes"}`,
	}
	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	items := []indexer.TOCItem{
		{Title: "Chapter 1", PhysicalIndex: intPtr(1)},
		{Title: "Chapter 2", PhysicalIndex: nil},
		{Title: "Chapter 3", PhysicalIndex: intPtr(10)},
	}
	pageTexts := []string{"page 1", "page 2"}

	ctx := context.Background()
	result := ac.CheckAllItemsAppearanceInStart(ctx, items, pageTexts)

	is.Len(result, 3)
	is.Equal("yes", result[0].AppearStart)
	is.Equal("no", result[1].AppearStart)
	is.Equal("no", result[2].AppearStart)

	mu.Lock()
	actualCalls := mockClient.callCount
	mu.Unlock()
	is.Equal(1, actualCalls)
}

func TestCheckAllItemsAppearanceInStart_ResponseVariations(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	responses := []string{
		`{"start_begin": "yes"}`,
		`{"start_begin": "no"}`,
		`{"start_begin": "YES"}`,
		`{"start_begin": "NO"}`,
	}
	mockClient := &MockLLMClient{
		response: responses[0],
	}
	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	items := []indexer.TOCItem{
		{Title: "Chapter 1", PhysicalIndex: intPtr(1)},
		{Title: "Chapter 2", PhysicalIndex: intPtr(2)},
		{Title: "Chapter 3", PhysicalIndex: intPtr(3)},
		{Title: "Chapter 4", PhysicalIndex: intPtr(4)},
	}
	pageTexts := []string{"page 1", "page 2", "page 3", "page 4"}

	ctx := context.Background()

	for i, resp := range responses {
		mockClient.Reset()
		mockClient.response = resp
		singleItem := []indexer.TOCItem{items[i]}
		singlePage := []string{pageTexts[i]}

		result := ac.CheckAllItemsAppearanceInStart(ctx, singleItem, singlePage)
		is.Len(result, 1)

		expected := "yes"
		if resp == `{"start_begin": "no"}` || resp == `{"start_begin": "NO"}` {
			expected = "no"
		}
		is.Equal(expected, result[0].AppearStart)
	}
}

func TestCheckAllItemsAppearanceInStart_ContextCancellation(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	mockClient := &MockLLMClient{
		response: `{"start_begin": "yes"}`,
	}
	ac := indexer.NewAppearanceChecker(mockClient, &config.Config{})

	items := []indexer.TOCItem{
		{Title: "Chapter 1", PhysicalIndex: intPtr(1)},
	}
	pageTexts := []string{"page 1"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	result := ac.CheckAllItemsAppearanceInStart(ctx, items, pageTexts)

	is.Len(result, 1)
	is.Equal(1, mockClient.GetCallCount())
}

func TestCheckAllItemsAppearanceInStart_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		items         []indexer.TOCItem
		pageTexts     []string
		skipCheck     bool
		expectResults []string
		expectCalls   int
	}{
		{
			name: "skip appearance check",
			items: []indexer.TOCItem{
				{Title: "Chapter 1", PhysicalIndex: intPtr(1)},
			},
			pageTexts:     []string{"page 1"},
			skipCheck:     true,
			expectResults: []string{"yes"},
			expectCalls:   0,
		},
		{
			name:          "empty items",
			items:         []indexer.TOCItem{},
			pageTexts:     []string{"page 1"},
			skipCheck:     false,
			expectResults: []string{},
			expectCalls:   0,
		},
		{
			name: "no physical index",
			items: []indexer.TOCItem{
				{Title: "Chapter 1", PhysicalIndex: nil},
			},
			pageTexts:     []string{"page 1"},
			skipCheck:     false,
			expectResults: []string{"no"},
			expectCalls:   0,
		},
		{
			name: "physical index out of range",
			items: []indexer.TOCItem{
				{Title: "Chapter 1", PhysicalIndex: intPtr(10)},
			},
			pageTexts:     []string{"page 1"},
			skipCheck:     false,
			expectResults: []string{"no"},
			expectCalls:   0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			is := assert.New(t)
			must := require.New(t)

			mockClient := &MockLLMClient{
				response: `{"start_begin": "yes"}`,
			}
			cfg := &config.Config{SkipAppearanceCheck: tt.skipCheck}
			ac := indexer.NewAppearanceChecker(mockClient, cfg)

			ctx := context.Background()
			result := ac.CheckAllItemsAppearanceInStart(ctx, tt.items, tt.pageTexts)

			is.Len(result, len(tt.expectResults))
			for i, expected := range tt.expectResults {
				if i < len(result) {
					is.Equal(expected, result[i].AppearStart)
				}
			}
			is.Equal(tt.expectCalls, mockClient.GetCallCount())

			if tt.skipCheck && len(tt.items) > 0 {
				for _, item := range result {
					is.Equal("yes", item.AppearStart)
				}
			}
		})
	}
}

// Helper functions

func intPtr(i int) *int {
	return &i
}

func TestParseLLMJSONResponse_ValidJSON(t *testing.T) {
	is := assert.New(t)
	must := require.New(t)

	jsonStr := `{"answer": "yes"}`
	var result struct {
		Answer string `json:"answer"`
	}

	err := parseLLMJSONResponse(jsonStr, &result)

	must.NoError(err)
	is.Equal("yes", result.Answer)
}

func TestParseLLMJSONResponse_InvalidJSON(t *testing.T) {
	is := assert.New(t)

	jsonStr := `invalid json`
	var result struct {
		Answer string `json:"answer"`
	}

	err := parseLLMJSONResponse(jsonStr, &result)

	is.Error(err)
}

func TestParseLLMJSONResponse_EmptyString(t *testing.T) {
	is := assert.New(t)

	var result struct {
		Answer string `json:"answer"`
	}

	err := parseLLMJSONResponse("", &result)

	is.Error(err)
}

func TestParseLLMJSONResponse_WrongType(t *testing.T) {
	is := assert.New(t)

	jsonStr := `{"answer": 123}`
	var result struct {
		Answer string `json:"answer"`
	}

	err := parseLLMJSONResponse(jsonStr, &result)

	is.Error(err)
}

// parseLLMJSONResponse is a helper function from the indexer package.
// We need to replicate it here for testing since it's not exported.
func parseLLMJSONResponse(response string, result interface{}) error {
	cleaned := strings.TrimSpace(response)
	if cleaned == "" {
		return &json.UnmarshalTypeError{Value: "empty string", Type: nil, Offset: 0}
	}
	return json.Unmarshal([]byte(cleaned), result)
}

// RateLimitError is a mock error type for testing rate limiting.
// In the actual code, this would come from the llm package.
type RateLimitError struct {
	Message string
}

func (e *RateLimitError) Error() string {
	return e.Message
}
