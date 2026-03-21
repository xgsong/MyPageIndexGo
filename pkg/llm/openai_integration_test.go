package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sashabaranov/go-openai"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/document"
)

func TestGenerateStructure_Success(t *testing.T) {
	// Create a test server that returns a valid response
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request path and method
		assert.Equal(t, "/chat/completions", r.URL.Path)
		assert.Equal(t, "POST", r.Method)

		// Parse request body
		var req openai.ChatCompletionRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		// Verify request parameters
		assert.Equal(t, "gpt-4o", req.Model)
		assert.Len(t, req.Messages, 1)

		// Return mock response
		resp := openai.ChatCompletionResponse{
			Choices: []openai.ChatCompletionChoice{
				{
					Message: openai.ChatCompletionMessage{
						Content: `{
							"title": "Test Document",
							"start_page": 1,
							"end_page": 10,
							"children": [
								{
									"title": "Introduction",
									"start_page": 1,
									"end_page": 2
								},
								{
									"title": "Implementation",
									"start_page": 3,
									"end_page": 10
								}
							]
						}`,
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer testServer.Close()

	// Create client with custom base URL pointing to test server
	cfg := config.DefaultConfig()
	cfg.OpenAIAPIKey = "test-key"
	cfg.OpenAIBaseURL = testServer.URL
	cfg.OpenAIModel = "gpt-4o"

	client := NewOpenAIClient(cfg)
	require.NotNil(t, client)

	// Call GenerateStructure
	ctx := context.Background()
	node, err := client.GenerateStructure(ctx, "Test document content")
	assert.NoError(t, err)
	assert.NotNil(t, node)

	// Verify returned node
	assert.Equal(t, "Test Document", node.Title)
	assert.Equal(t, 1, node.StartPage)
	assert.Equal(t, 10, node.EndPage)
	assert.Len(t, node.Children, 2)
	assert.Equal(t, "Introduction", node.Children[0].Title)
	assert.Equal(t, "Implementation", node.Children[1].Title)
}

func TestGenerateStructure_InvalidJSON(t *testing.T) {
	// Create a test server that returns invalid JSON
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openai.ChatCompletionResponse{
			Choices: []openai.ChatCompletionChoice{
				{
					Message: openai.ChatCompletionMessage{
						Content: "This is not valid JSON",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer testServer.Close()

	cfg := config.DefaultConfig()
	cfg.OpenAIAPIKey = "test-key"
	cfg.OpenAIBaseURL = testServer.URL

	client := NewOpenAIClient(cfg)

	ctx := context.Background()
	node, err := client.GenerateStructure(ctx, "Test content")
	assert.Error(t, err)
	assert.Nil(t, node)
	assert.Contains(t, err.Error(), "failed to parse structure json")
}

func TestGenerateStructure_APIError(t *testing.T) {
	// Create a test server that returns an error
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Internal Server Error"))
	}))
	defer testServer.Close()

	cfg := config.DefaultConfig()
	cfg.OpenAIAPIKey = "test-key"
	cfg.OpenAIBaseURL = testServer.URL

	client := NewOpenAIClient(cfg)

	ctx := context.Background()
	node, err := client.GenerateStructure(ctx, "Test content")
	assert.Error(t, err)
	assert.Nil(t, node)
	assert.Contains(t, err.Error(), "openai api call failed")
}

func TestGenerateSummary_Success(t *testing.T) {
	// Create a test server that returns a valid summary response
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/chat/completions", r.URL.Path)

		// Return mock response
		resp := openai.ChatCompletionResponse{
			Choices: []openai.ChatCompletionChoice{
				{
					Message: openai.ChatCompletionMessage{
						Content: "This is a concise summary of the section content.",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer testServer.Close()

	cfg := config.DefaultConfig()
	cfg.OpenAIAPIKey = "test-key"
	cfg.OpenAIBaseURL = testServer.URL

	client := NewOpenAIClient(cfg)

	ctx := context.Background()
	summary, err := client.GenerateSummary(ctx, "Test Section", "This is the full content of the test section that needs summarization.")
	assert.NoError(t, err)
	assert.Equal(t, "This is a concise summary of the section content.", summary)
}

func TestGenerateSummary_EmptyResponse(t *testing.T) {
	// Create a test server that returns empty response
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openai.ChatCompletionResponse{
			Choices: []openai.ChatCompletionChoice{
				{
					Message: openai.ChatCompletionMessage{
						Content: "",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer testServer.Close()

	cfg := config.DefaultConfig()
	cfg.OpenAIAPIKey = "test-key"
	cfg.OpenAIBaseURL = testServer.URL

	client := NewOpenAIClient(cfg)

	ctx := context.Background()
	summary, err := client.GenerateSummary(ctx, "Test Section", "Test content")
	assert.NoError(t, err)
	assert.Empty(t, summary)
}

func TestSearch_Success(t *testing.T) {
	// Create a test server that returns a valid search response
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/chat/completions", r.URL.Path)

		// Return mock response
		resp := openai.ChatCompletionResponse{
			Choices: []openai.ChatCompletionChoice{
				{
					Message: openai.ChatCompletionMessage{
						Content: `{
							"answer": "The revenue in 2023 was $10 million with 15% year-over-year growth.",
							"node_ids": ["child1", "child2"]
						}`,
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer testServer.Close()

	cfg := config.DefaultConfig()
	cfg.OpenAIAPIKey = "test-key"
	cfg.OpenAIBaseURL = testServer.URL

	client := NewOpenAIClient(cfg)

	// Create test tree
	root := document.NewNode("Root", 1, 10)
	child1 := document.NewNode("Financial Results 2023", 5, 7)
	child1.ID = "child1"
	child2 := document.NewNode("Growth Metrics", 8, 10)
	child2.ID = "child2"
	root.AddChild(child1)
	root.AddChild(child2)

	tree := document.NewIndexTree(root, 10)

	ctx := context.Background()
	result, err := client.Search(ctx, "What was the revenue in 2023?", tree)
	assert.NoError(t, err)
	assert.NotNil(t, result)

	assert.Equal(t, "What was the revenue in 2023?", result.Query)
	assert.Equal(t, "The revenue in 2023 was $10 million with 15% year-over-year growth.", result.Answer)
	assert.Len(t, result.Nodes, 2)
	assert.Contains(t, result.Nodes, child1)
	assert.Contains(t, result.Nodes, child2)
}

func TestSearch_InvalidJSON(t *testing.T) {
	// Create a test server that returns invalid JSON
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openai.ChatCompletionResponse{
			Choices: []openai.ChatCompletionChoice{
				{
					Message: openai.ChatCompletionMessage{
						Content: "Invalid JSON response",
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer testServer.Close()

	cfg := config.DefaultConfig()
	cfg.OpenAIAPIKey = "test-key"
	cfg.OpenAIBaseURL = testServer.URL

	client := NewOpenAIClient(cfg)

	root := document.NewNode("Root", 1, 10)
	tree := document.NewIndexTree(root, 10)

	ctx := context.Background()
	result, err := client.Search(ctx, "Test query", tree)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to parse search response json")
}

func TestSearch_MissingNodeIDs(t *testing.T) {
	// Create a test server that returns node IDs that don't exist
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := openai.ChatCompletionResponse{
			Choices: []openai.ChatCompletionChoice{
				{
					Message: openai.ChatCompletionMessage{
						Content: `{
							"answer": "Test answer",
							"node_ids": ["non-existent-id"]
						}`,
					},
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
	}))
	defer testServer.Close()

	cfg := config.DefaultConfig()
	cfg.OpenAIAPIKey = "test-key"
	cfg.OpenAIBaseURL = testServer.URL

	client := NewOpenAIClient(cfg)

	root := document.NewNode("Root", 1, 10)
	tree := document.NewIndexTree(root, 10)

	ctx := context.Background()
	result, err := client.Search(ctx, "Test query", tree)
	assert.NoError(t, err) // Should not error, just return empty nodes list
	assert.NotNil(t, result)
	assert.Equal(t, "Test answer", result.Answer)
	assert.Len(t, result.Nodes, 0) // Non-existent IDs are ignored
}
