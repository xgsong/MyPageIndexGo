package llm

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
	"golang.org/x/sync/errgroup"

	"github.com/xgsong/mypageindexgo/internal/utils"
	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/language"
)

// RateLimitInfo contains rate limit information from OpenAI API responses.
type RateLimitInfo struct {
	Remaining int       // Number of remaining requests
	Reset     time.Time // Time when the rate limit resets
}

// RateLimitCallback is a function that is called when rate limit information is received.
type RateLimitCallback func(info RateLimitInfo)

// OpenAIClient implements LLMClient for OpenAI API.
type OpenAIClient struct {
	client          *openai.Client
	model           string
	jsonCleaner     *utils.JSONCleaner
	OnRateLimitInfo RateLimitCallback // Optional callback for rate limit information
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

// createLanguageSystemMessage creates a system message that enforces language consistency.
// This is critical for ensuring summaries and structure titles match the document language.
func createLanguageSystemMessage(lang language.Language) string {
	if lang.Code == "en" && lang.Confidence < 0.5 {
		// Unknown language, default to English without strong enforcement
		return "IMPORTANT: Do NOT output any thinking/reasoning process. Directly output only the final result in the required format."
	}

	// Strong language enforcement for detected languages
	return fmt.Sprintf(
		"You MUST respond in %s. The document is written in %s. "+
			"ALL output including titles, summaries, and structure MUST be in %s. "+
			"Do NOT use English or any other language. "+
			"CRITICAL: This is a strict requirement - any output in wrong language will be rejected.",
		lang.GetLanguageName(),
		lang.GetLanguageName(),
		lang.GetLanguageName(),
	)
}

// prepareSystemMessage prepends the anti-thinking system message to the request messages.
// This is done once before the retry loop to avoid duplication on retries.
func prepareSystemMessage(req *openai.ChatCompletionRequest) {
	systemContent := "IMPORTANT: Do NOT output any thinking/reasoning process, <think/> tags, or chain-of-thought content. Directly output only the final result in the required format. Any thought content must be completely excluded from your response."

	if len(req.Messages) == 0 {
		req.Messages = []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemContent,
			},
		}
	} else if req.Messages[0].Role == openai.ChatMessageRoleSystem {
		req.Messages[0].Content = systemContent + "\n" + req.Messages[0].Content
	} else {
		req.Messages = append([]openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemContent,
			},
		}, req.Messages...)
	}

	// Disable streaming to ensure we get complete response
	req.Stream = false
}

// createChatCompletion sends a chat completion request with retry logic.
func (c *OpenAIClient) createChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (string, error) {
	// Prepare system message once BEFORE the retry loop to avoid duplication on retries.
	prepareSystemMessage(&req)

	var content string
	err := utils.DoRetry(ctx, utils.DefaultRetryConfig(), func() error {
		resp, err := c.client.CreateChatCompletion(ctx, req)
		if err != nil {
			// Check if this is an API error
			var apiErr *openai.APIError
			if errors.As(err, &apiErr) {
				switch apiErr.HTTPStatusCode {
				case 429, 500, 502, 503, 504:
					// These are retryable errors
					return err // Continue retrying
				default:
					// Non-retryable errors, stop retrying
					return utils.StopRetry(err)
				}
			}

			// For other error types, check if they are retryable
			var reqErr *openai.RequestError
			if errors.As(err, &reqErr) {
				// Request errors (connection issues, timeouts) are retryable
				return err
			}

			// All other errors are non-retryable
			return utils.StopRetry(err)
		}
		if len(resp.Choices) == 0 {
			return fmt.Errorf("no choices returned from OpenAI")
		}

		// Extract rate limit information from successful response
		if c.OnRateLimitInfo != nil {
			header := resp.Header()
			remainingStr := header.Get("X-RateLimit-Remaining")
			resetStr := header.Get("X-RateLimit-Reset")
			if remainingStr != "" && resetStr != "" {
				remaining, _ := strconv.Atoi(remainingStr)
				resetUnix, _ := strconv.ParseInt(resetStr, 10, 64)
				if resetUnix > 0 {
					c.OnRateLimitInfo(RateLimitInfo{
						Remaining: remaining,
						Reset:     time.Unix(resetUnix, 0),
					})
				}
			}
		}

		content = resp.Choices[0].Message.Content
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("openai api call failed: %w", err)
	}
	return content, nil
}

// GenerateStructure generates a hierarchical tree structure from raw page text.
func (c *OpenAIClient) GenerateStructure(ctx context.Context, text string, lang language.Language) (*document.Node, error) {
	if text == "" {
		return nil, fmt.Errorf("empty input text")
	}

	// Create system message to enforce language consistency
	systemMsg := createLanguageSystemMessage(lang)

	prompt := GenerateStructurePrompt() + text

	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemMsg,
			},
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

	// Ensure node has an ID if not set by LLM, and recursively fix all children
	ensureNodeIDs(&node)

	return &node, nil
}

// ensureNodeIDs recursively ensures all nodes in the tree have valid IDs
func ensureNodeIDs(node *document.Node) {
	if node == nil {
		return
	}
	// Generate ID if empty
	if node.ID == "" {
		node.ID = document.NewNode(node.Title, node.StartPage, node.EndPage).ID
	}
	// Recursively fix children
	for _, child := range node.Children {
		ensureNodeIDs(child)
	}
}

// GenerateSummary generates a concise summary for a node that captures its key content.
func (c *OpenAIClient) GenerateSummary(ctx context.Context, nodeTitle string, text string, lang language.Language) (string, error) {
	if text == "" {
		return "", fmt.Errorf("empty input text")
	}

	prompt, err := RenderSummaryPrompt(nodeTitle, text)
	if err != nil {
		return "", fmt.Errorf("failed to render summary prompt: %w", err)
	}

	// Create system message to enforce language consistency
	systemMsg := createLanguageSystemMessage(lang)

	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemMsg,
			},
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
	nodes := findNodesByID(tree, sr.NodeIDs)

	result := &document.SearchResult{
		Query:  query,
		Answer: sr.Answer,
		Nodes:  nodes,
	}

	return result, nil
}

// findNodesByID looks up nodes by ID using the index tree's built-in map for O(1) lookups
func findNodesByID(tree *document.IndexTree, ids []string) []*document.Node {
	var result []*document.Node
	for _, id := range ids {
		if node := tree.FindNodeByID(id); node != nil {
			result = append(result, node)
		}
	}
	return result
}

// GenerateBatchSummaries generates summaries for multiple nodes in a single batch call.
func (c *OpenAIClient) GenerateBatchSummaries(ctx context.Context, requests []*BatchSummaryRequest, lang language.Language) ([]*BatchSummaryResponse, error) {
	if len(requests) == 0 {
		return []*BatchSummaryResponse{}, nil
	}

	// Render the batch summary prompt
	prompt, err := RenderBatchSummaryPrompt(requests)
	if err != nil {
		return nil, fmt.Errorf("failed to render batch summary prompt: %w", err)
	}

	// Create system message to enforce language consistency
	systemMsg := createLanguageSystemMessage(lang)

	// Create chat completion request
	req := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: systemMsg,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: prompt,
			},
		},
	}

	// Call OpenAI API
	content, err := c.createChatCompletion(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate batch summaries: %w", err)
	}

	// Parse the JSON response
	var responses []*BatchSummaryResponse
	err = c.jsonCleaner.ParseJSON(content, &responses)
	if err != nil {
		return c.fallbackToIndividualCalls(ctx, requests, lang), nil
	}

	// Validate responses match request count
	if len(responses) != len(requests) {
		return c.fallbackToIndividualCalls(ctx, requests, lang), nil
	}

	// Validate node IDs match
	requestIDMap := make(map[string]int)
	for i, req := range requests {
		requestIDMap[req.NodeID] = i
	}

	for _, resp := range responses {
		if _, ok := requestIDMap[resp.NodeID]; !ok {
			return c.fallbackToIndividualCalls(ctx, requests, lang), nil
		}
	}

	// Sort responses to match the original request order
	sortedResponses := make([]*BatchSummaryResponse, len(requests))
	for _, resp := range responses {
		idx := requestIDMap[resp.NodeID]
		sortedResponses[idx] = resp
	}

	return sortedResponses, nil
}

// fallbackToIndividualCalls falls back to generating summaries one by one when batch fails.
// Uses errgroup to respect context cancellation and propagate errors.
func (c *OpenAIClient) fallbackToIndividualCalls(ctx context.Context, requests []*BatchSummaryRequest, lang language.Language) []*BatchSummaryResponse {
	responses := make([]*BatchSummaryResponse, len(requests))
	var mu sync.Mutex

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(5) // Limit concurrency for fallback calls

	for i, req := range requests {
		i, req := i, req
		eg.Go(func() error {
			summary, err := c.GenerateSummary(ctx, req.NodeTitle, req.Text, lang)
			resp := &BatchSummaryResponse{
				NodeID: req.NodeID,
			}
			if err != nil {
				// Record the error but don't fail the entire fallback;
				// other nodes can still get their summaries.
				resp.Error = err.Error()
			} else {
				resp.Summary = summary
			}

			mu.Lock()
			responses[i] = resp
			mu.Unlock()
			return nil
		})
	}

	// Wait for all goroutines; if context is cancelled, in-flight API calls
	// will return early via ctx. Already-completed responses are preserved.
	_ = eg.Wait()

	return responses
}

// GenerateSimple generates a simple text response from a prompt.
func (c *OpenAIClient) GenerateSimple(ctx context.Context, prompt string) (string, error) {
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
		return "", fmt.Errorf("failed to generate simple response: %w", err)
	}

	return content, nil
}
