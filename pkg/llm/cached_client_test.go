package llm

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/language"
)

// MockLLMClient is a mock implementation of LLMClient for testing
type MockLLMClient struct {
	mock.Mock
}

func (m *MockLLMClient) GenerateStructure(ctx context.Context, text string, lang language.Language) (*document.Node, error) {
	args := m.Called(ctx, text, lang)
	return args.Get(0).(*document.Node), args.Error(1)
}

func (m *MockLLMClient) GenerateSummary(ctx context.Context, nodeTitle string, text string, lang language.Language) (string, error) {
	args := m.Called(ctx, nodeTitle, text, lang)
	return args.String(0), args.Error(1)
}

func (m *MockLLMClient) Search(ctx context.Context, query string, tree *document.IndexTree) (*document.SearchResult, error) {
	args := m.Called(ctx, query, tree)
	return args.Get(0).(*document.SearchResult), args.Error(1)
}

func (m *MockLLMClient) GenerateBatchSummaries(ctx context.Context, requests []*BatchSummaryRequest, lang language.Language) ([]*BatchSummaryResponse, error) {
	args := m.Called(ctx, requests, lang)
	return args.Get(0).([]*BatchSummaryResponse), args.Error(1)
}

func (m *MockLLMClient) GenerateSimple(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func TestCachedLLMClient_GenerateStructure_CacheHit(t *testing.T) {
	mockClient := new(MockLLMClient)
	cachedClient := NewCachedLLMClient(mockClient, 1*time.Hour, false)

	testText := "test content"
	expectedNode := document.NewNode("Test", 1, 1)

	// First call - should call the underlying client
	mockClient.On("GenerateStructure", mock.Anything, testText, language.LanguageEnglish).Return(expectedNode, nil).Once()

	node, err := cachedClient.GenerateStructure(context.Background(), testText, language.LanguageEnglish)
	assert.NoError(t, err)
	assert.Equal(t, expectedNode, node)
	mockClient.AssertExpectations(t)

	// Second call - should hit cache, no underlying call
	node, err = cachedClient.GenerateStructure(context.Background(), testText, language.LanguageEnglish)
	assert.NoError(t, err)
	assert.Equal(t, expectedNode, node)
	// mockClient should only have been called once
	mockClient.AssertNumberOfCalls(t, "GenerateStructure", 1)
}

func TestCachedLLMClient_GenerateStructure_CacheExpired(t *testing.T) {
	mockClient := new(MockLLMClient)
	// Use very short TTL
	cachedClient := NewCachedLLMClient(mockClient, 1*time.Millisecond, false)

	testText := "test content"
	expectedNode1 := document.NewNode("Test", 1, 1)
	expectedNode2 := document.NewNode("Test 2", 1, 1)

	// First call
	mockClient.On("GenerateStructure", mock.Anything, testText, language.LanguageEnglish).Return(expectedNode1, nil).Once()

	node, err := cachedClient.GenerateStructure(context.Background(), testText, language.LanguageEnglish)
	assert.NoError(t, err)
	assert.Equal(t, expectedNode1, node)

	// Wait for cache to expire
	time.Sleep(10 * time.Millisecond)

	// Second call - should miss cache and call underlying client again
	mockClient.On("GenerateStructure", mock.Anything, testText, language.LanguageEnglish).Return(expectedNode2, nil).Once()

	node, err = cachedClient.GenerateStructure(context.Background(), testText, language.LanguageEnglish)
	assert.NoError(t, err)
	assert.Equal(t, expectedNode2, node)
	mockClient.AssertNumberOfCalls(t, "GenerateStructure", 2)
}

func TestCachedLLMClient_GenerateSummary_CacheHit(t *testing.T) {
	mockClient := new(MockLLMClient)
	cachedClient := NewCachedLLMClient(mockClient, 1*time.Hour, false)

	testTitle := "Test Title"
	testText := "test content"
	expectedSummary := "This is a test summary"

	// First call
	mockClient.On("GenerateSummary", mock.Anything, testTitle, testText, language.LanguageEnglish).Return(expectedSummary, nil).Once()

	summary, err := cachedClient.GenerateSummary(context.Background(), testTitle, testText, language.LanguageEnglish)
	assert.NoError(t, err)
	assert.Equal(t, expectedSummary, summary)
	mockClient.AssertExpectations(t)

	// Second call - cache hit
	summary, err = cachedClient.GenerateSummary(context.Background(), testTitle, testText, language.LanguageEnglish)
	assert.NoError(t, err)
	assert.Equal(t, expectedSummary, summary)
	mockClient.AssertNumberOfCalls(t, "GenerateSummary", 1)
}

func TestCachedLLMClient_Search_NotCached(t *testing.T) {
	mockClient := new(MockLLMClient)
	cachedClient := NewCachedLLMClient(mockClient, 1*time.Hour, false)

	testQuery := "test query"
	testTree := document.NewIndexTree(document.NewNode("Root", 1, 10), 10)
	expectedResult := &document.SearchResult{
		Query:  testQuery,
		Answer: "test answer",
	}

	// First call
	mockClient.On("Search", mock.Anything, testQuery, testTree).Return(expectedResult, nil).Once()

	result, err := cachedClient.Search(context.Background(), testQuery, testTree)
	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result)
	mockClient.AssertExpectations(t)

	// Second call - search should not be cached, so call again
	mockClient.On("Search", mock.Anything, testQuery, testTree).Return(expectedResult, nil).Once()

	result, err = cachedClient.Search(context.Background(), testQuery, testTree)
	assert.NoError(t, err)
	assert.Equal(t, expectedResult, result)
	mockClient.AssertNumberOfCalls(t, "Search", 2)
}

func TestCachedLLMClient_DifferentInputs_DifferentCache(t *testing.T) {
	mockClient := new(MockLLMClient)
	cachedClient := NewCachedLLMClient(mockClient, 1*time.Hour, false)

	text1 := "content 1"
	text2 := "content 2"
	node1 := document.NewNode("Node 1", 1, 1)
	node2 := document.NewNode("Node 2", 2, 2)

	mockClient.On("GenerateStructure", mock.Anything, text1, language.LanguageEnglish).Return(node1, nil).Once()
	mockClient.On("GenerateStructure", mock.Anything, text2, language.LanguageEnglish).Return(node2, nil).Once()

	// Call with first text
	result1, err := cachedClient.GenerateStructure(context.Background(), text1, language.LanguageEnglish)
	assert.NoError(t, err)
	assert.Equal(t, node1, result1)

	// Call with second text
	result2, err := cachedClient.GenerateStructure(context.Background(), text2, language.LanguageEnglish)
	assert.NoError(t, err)
	assert.Equal(t, node2, result2)

	// Both should be cached
	result1Again, err := cachedClient.GenerateStructure(context.Background(), text1, language.LanguageEnglish)
	assert.NoError(t, err)
	assert.Equal(t, node1, result1Again)

	result2Again, err := cachedClient.GenerateStructure(context.Background(), text2, language.LanguageEnglish)
	assert.NoError(t, err)
	assert.Equal(t, node2, result2Again)

	mockClient.AssertNumberOfCalls(t, "GenerateStructure", 2)
}
