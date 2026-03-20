package llm

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
	"github.com/xgsong/mypageindexgo/internal/utils"
	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/document"
)

// OpenAIClient implements LLMClient for OpenAI API.
type OpenAIClient struct {
	client      *openai.Client
	model       string
	jsonCleaner *utils.JSONCleaner
}

// NewOpenAIClient creates a new OpenAI client from configuration.
func NewOpenAIClient(cfg *config.Config) *OpenAIClient {
	clientConfig := openai.DefaultConfig(cfg.OpenAIAPIKey)
	if cfg.OpenAIBaseURL != "" {
		// Ensure BaseURL ends with trailing slash for proper path joining
		baseURL := cfg.OpenAIBaseURL
		if len(baseURL) > 0 && baseURL[len(baseURL)-1] != '/' {
			baseURL += "/"
		}
		clientConfig.BaseURL = baseURL
	}

	return &OpenAIClient{
		client:      openai.NewClientWithConfig(clientConfig),
		model:       cfg.OpenAIModel,
		jsonCleaner: utils.NewJSONCleaner(),
	}
}

// createChatCompletion sends a chat completion request and returns the content.
func (c *OpenAIClient) createChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (string, error) {
	resp, err := c.client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("openai api call failed: %w", err)
	}

	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices returned from OpenAI")
	}

	return resp.Choices[0].Message.Content, nil
}

// GenerateStructure generates a hierarchical tree structure from raw page text.
func (c *OpenAIClient) GenerateStructure(ctx context.Context, text string) (*document.Node, error) {
	if text == "" {
		return nil, fmt.Errorf("empty input text")
	}

	prompt := GenerateStructurePrompt() + text

	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
	}

	content, err := c.createChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate structure: %w", err)
	}

	var node document.Node
	err = c.jsonCleaner.ParseJSON(content, &node)
	if err != nil {
		return nil, fmt.Errorf("failed to parse structure json: %w", err)
	}

	// Ensure node has an ID if not set by LLM
	if node.ID == "" {
		// We need to recreate it to get a proper UUID
		newNode := document.NewNode(node.Title, node.StartPage, node.EndPage)
		newNode.Summary = node.Summary
		newNode.Children = node.Children
		return newNode, nil
	}

	return &node, nil
}

// GenerateSummary generates a concise summary for a node that captures its key content.
func (c *OpenAIClient) GenerateSummary(ctx context.Context, nodeTitle string, text string) (string, error) {
	if text == "" {
		return "", fmt.Errorf("empty input text")
	}

	prompt, err := RenderSummaryPrompt(nodeTitle, text)
	if err != nil {
		return "", fmt.Errorf("failed to render summary prompt: %w", err)
	}

	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
	}

	content, err := c.createChatCompletion(ctx, req)
	if err != nil {
		return "", fmt.Errorf("failed to generate summary: %w", err)
	}

	content = c.jsonCleaner.Clean(content)
	return content, nil
}

// searchResponse represents the JSON response from OpenAI for search.
type searchResponse struct {
	Answer  string   `json:"answer"`
	NodeIDs []string `json:"node_ids"`
}

// Search performs reasoning-based retrieval on the index tree given a query.
func (c *OpenAIClient) Search(ctx context.Context, query string, tree *document.IndexTree) (*document.SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("empty search query")
	}
	if tree == nil || tree.Root == nil {
		return nil, fmt.Errorf("invalid index tree")
	}

	prompt, err := SearchPrompt(query, tree)
	if err != nil {
		return nil, fmt.Errorf("failed to render search prompt: %w", err)
	}

	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
	}

	content, err := c.createChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to perform search: %w", err)
	}

	var sr searchResponse
	err = c.jsonCleaner.ParseJSON(content, &sr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse search response json: %w", err)
	}

	// Collect the actual node pointers from the tree based on IDs
	nodes := findNodesByID(tree.Root, sr.NodeIDs)

	result := &document.SearchResult{
		Query:  query,
		Answer: sr.Answer,
		Nodes:  nodes,
	}

	return result, nil
}

// findNodesByID recursively searches the tree for nodes with the given IDs.
func findNodesByID(root *document.Node, ids []string) []*document.Node {
	idSet := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		idSet[id] = struct{}{}
	}

	var result []*document.Node
	var search func(*document.Node)
	search = func(node *document.Node) {
		if node == nil {
			return
		}
		if _, ok := idSet[node.ID]; ok {
			result = append(result, node)
			delete(idSet, node.ID) // each ID matched once
		}
		for _, child := range node.Children {
			search(child)
		}
	}

	search(root)
	return result
}
