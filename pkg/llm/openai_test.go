package llm

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/language"
)

func TestNewOpenAIClient(t *testing.T) {
	t.Run("basic config", func(t *testing.T) {
		cfg := &config.Config{
			OpenAIAPIKey: "test-key",
			OpenAIModel:  "gpt-4o-mini",
		}

		client := NewOpenAIClient(cfg)

		assert.NotNil(t, client)
		assert.Equal(t, "gpt-4o-mini", client.model)
	})

	t.Run("with base url", func(t *testing.T) {
		cfg := &config.Config{
			OpenAIAPIKey:   "test-key",
			OpenAIModel:    "gpt-4o-mini",
			OpenAIBaseURL:  "https://api.example.com",
		}

		client := NewOpenAIClient(cfg)

		assert.NotNil(t, client)
		assert.NotNil(t, client.client)
	})

	t.Run("base url with trailing slash", func(t *testing.T) {
		cfg := &config.Config{
			OpenAIAPIKey:   "test-key",
			OpenAIModel:    "gpt-4o-mini",
			OpenAIBaseURL:  "https://api.example.com/",
		}

		client := NewOpenAIClient(cfg)

		assert.NotNil(t, client)
	})
}

func TestCreateLanguageSystemMessage(t *testing.T) {
	tests := []struct {
		name     string
		lang     language.Language
		expected string
	}{
		{
			name: "English",
			lang: language.Language{
				Code:        "en",
				Name:        "English",
				Confidence:  0.9,
			},
			expected: "English",
		},
		{
			name: "Chinese",
			lang: language.Language{
				Code:        "zh",
				Name:        "Chinese",
				Confidence:  0.9,
			},
			expected: "Chinese",
		},
		{
			name: "Unknown language",
			lang: language.Language{
				Code:        "en",
				Name:        "English",
				Confidence:  0.3,
			},
			expected: "reasoning",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := createLanguageSystemMessage(tt.lang)
			assert.Contains(t, msg, tt.expected)
		})
	}
}

func TestFindNodesByID(t *testing.T) {
	root := document.NewNode("Root", 1, 10)
	child1 := document.NewNode("Child 1", 1, 5)
	child2 := document.NewNode("Child 2", 6, 10)
	root.AddChild(child1)
	root.AddChild(child2)

	tree := document.NewIndexTree(root, 10)

	t.Run("find existing nodes", func(t *testing.T) {
		ids := []string{child1.ID, child2.ID}
		result := findNodesByID(tree, ids)

		assert.Len(t, result, 2)
		assert.Contains(t, result, child1)
		assert.Contains(t, result, child2)
	})

	t.Run("find non-existent node", func(t *testing.T) {
		ids := []string{"non-existent-id"}
		result := findNodesByID(tree, ids)

		assert.Empty(t, result)
	})

	t.Run("mixed existing and non-existing", func(t *testing.T) {
		ids := []string{child1.ID, "non-existent"}
		result := findNodesByID(tree, ids)

		assert.Len(t, result, 1)
		assert.Equal(t, child1, result[0])
	})

	t.Run("empty ids", func(t *testing.T) {
		result := findNodesByID(tree, []string{})
		assert.Empty(t, result)
	})
}

func TestOpenAIClient_GenerateBatchSummaries_EmptyRequests(t *testing.T) {
	cfg := &config.Config{
		OpenAIAPIKey: "test-key",
		OpenAIModel:  "gpt-4o-mini",
	}

	client := NewOpenAIClient(cfg)
	ctx := context.Background()
	lang := language.Language{Code: "en", Name: "English", Confidence: 0.9}

	requests := []*BatchSummaryRequest{}
	result, err := client.GenerateBatchSummaries(ctx, requests, lang)

	assert.NoError(t, err)
	assert.Empty(t, result)
	assert.NotNil(t, result)
}

func TestOpenAIClient_fallbackToIndividualCalls(t *testing.T) {
	cfg := &config.Config{
		OpenAIAPIKey: "test-key",
		OpenAIModel:  "gpt-4o-mini",
	}

	client := NewOpenAIClient(cfg)
	// Use a short timeout context so the test doesn't hang on real API calls
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	lang := language.Language{Code: "en", Name: "English", Confidence: 0.9}

	requests := []*BatchSummaryRequest{
		{NodeID: "1", NodeTitle: "Chapter 1", Text: "Content 1"},
		{NodeID: "2", NodeTitle: "Chapter 2", Text: "Content 2"},
	}

	// This will fail without a valid API key, but tests the fallback logic
	result := client.fallbackToIndividualCalls(ctx, requests, lang)

	assert.Len(t, result, 2)
	assert.Equal(t, "1", result[0].NodeID)
	assert.Equal(t, "2", result[1].NodeID)
	// Results should have errors since API calls will fail
	assert.NotEmpty(t, result[0].Error)
	assert.NotEmpty(t, result[1].Error)
}

func TestNewOpenAIOCRClient(t *testing.T) {
	t.Run("basic config", func(t *testing.T) {
		cfg := &config.Config{
			OCRAPIKey: "test-ocr-key",
			OCRModel:  "test-model",
		}

		client := NewOpenAIOCRClient(cfg)

		assert.NotNil(t, client)
		assert.Equal(t, "test-model", client.model)
		assert.NotNil(t, client.systemPrompt)
		assert.Contains(t, client.systemPrompt, "OCR")
	})

	t.Run("default model", func(t *testing.T) {
		cfg := &config.Config{
			OCRAPIKey: "test-ocr-key",
		}

		client := NewOpenAIOCRClient(cfg)

		assert.NotNil(t, client)
		assert.Equal(t, "GLM-OCR-Q8_0", client.model)
	})

	t.Run("with base url", func(t *testing.T) {
		cfg := &config.Config{
			OCRAPIKey:          "test-ocr-key",
			OpenAIOCRBaseURL:   "https://ocr.example.com",
		}

		client := NewOpenAIOCRClient(cfg)

		assert.NotNil(t, client)
	})
}

func TestNewOpenAIOCRClientWithModel(t *testing.T) {
	cfg := &config.Config{
		OCRAPIKey: "test-ocr-key",
		OCRModel:  "default-model",
	}

	t.Run("override model", func(t *testing.T) {
		client := NewOpenAIOCRClientWithModel(cfg, "custom-model")

		assert.NotNil(t, client)
		assert.Equal(t, "custom-model", client.model)
	})

	t.Run("empty model override", func(t *testing.T) {
		client := NewOpenAIOCRClientWithModel(cfg, "")

		assert.NotNil(t, client)
		assert.Equal(t, "default-model", client.model)
	})
}

func TestOpenAIOCRClient_Recognize_EmptyImageData(t *testing.T) {
	cfg := &config.Config{
		OCRAPIKey: "test-ocr-key",
	}

	client := NewOpenAIOCRClient(cfg)
	ctx := context.Background()

	req := &document.OCRRequest{
		ImageData: []byte{},
		PageNum:   1,
	}

	resp, err := client.Recognize(ctx, req)

	assert.Error(t, err)
	assert.NotNil(t, resp)
	assert.Contains(t, resp.Error, "empty image data")
}

func TestOpenAIOCRClient_RecognizeBatch_EmptyRequests(t *testing.T) {
	cfg := &config.Config{
		OCRAPIKey: "test-ocr-key",
	}

	client := NewOpenAIOCRClient(cfg)
	ctx := context.Background()

	reqs := []*document.OCRRequest{}
	result, err := client.RecognizeBatch(ctx, reqs)

	assert.NoError(t, err)
	assert.Empty(t, result)
	assert.NotNil(t, result)
}
