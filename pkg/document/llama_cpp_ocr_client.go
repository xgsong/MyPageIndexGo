package document

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
)

// LlamaCppOCRClient implements OCRClient for llama.cpp OCR API.
type LlamaCppOCRClient struct {
	serverURL string        // Llama.cpp server URL (e.g., http://localhost:8080)
	modelName string        // OCR model name
	timeout   time.Duration // Request timeout
	client    *http.Client  // HTTP client
}

// LlamaCppOCRConfig holds configuration for LlamaCppOCRClient.
type LlamaCppOCRConfig struct {
	ServerURL string        // Llama.cpp server URL
	ModelName string        // OCR model name
	Timeout   time.Duration // Request timeout (default: 60s)
}

// NewLlamaCppOCRClient creates a new LlamaCppOCRClient with the given configuration.
func NewLlamaCppOCRClient(config LlamaCppOCRConfig) (*LlamaCppOCRClient, error) {
	if config.ServerURL == "" {
		return nil, fmt.Errorf("server URL is required")
	}
	if config.ModelName == "" {
		return nil, fmt.Errorf("model name is required")
	}
	if config.Timeout <= 0 {
		config.Timeout = 60 * time.Second
	}

	return &LlamaCppOCRClient{
		serverURL: config.ServerURL,
		modelName: config.ModelName,
		timeout:   config.Timeout,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}, nil
}

// LlamaCppChatCompletionRequest represents OpenAI compatible chat completion request for multi-modal OCR.
type LlamaCppChatCompletionRequest struct {
	Model    string                  `json:"model"`
	Messages []LlamaCppChatMessage   `json:"messages"`
	Stream   bool                    `json:"stream"`
	MaxTokens int                     `json:"max_tokens,omitempty"`
}

// LlamaCppChatMessage represents a chat message with multi-modal content.
type LlamaCppChatMessage struct {
	Role    string              `json:"role"`
	Content []LlamaCppContentPart `json:"content"`
}

// LlamaCppContentPart represents a part of message content (text or image).
type LlamaCppContentPart struct {
	Type     string          `json:"type"`
	Text     string          `json:"text,omitempty"`
	ImageURL *LlamaCppImageURL `json:"image_url,omitempty"`
}

// LlamaCppImageURL contains the base64 encoded image data.
type LlamaCppImageURL struct {
	URL string `json:"url"`
}

// LlamaCppChatCompletionResponse represents the response from chat completion API.
type LlamaCppChatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Recognize performs OCR on a single image using OpenAI compatible chat completion API.
func (c *LlamaCppOCRClient) Recognize(ctx context.Context, req *OCRRequest) (*OCRResponse, error) {
	if req == nil || len(req.ImageData) == 0 {
		return nil, fmt.Errorf("invalid OCR request: empty image data")
	}

	// Encode image to base64 data URL
	base64Img := "data:image/png;base64," + base64.StdEncoding.EncodeToString(req.ImageData)

	// Create OCR request using OpenAI compatible multi-modal chat format
	ocrReq := LlamaCppChatCompletionRequest{
		Model: c.modelName,
		Messages: []LlamaCppChatMessage{
			{
				Role: "user",
				Content: []LlamaCppContentPart{
					{
						Type: "text",
						Text: "Perform OCR on this image. Extract all text accurately, preserve the original formatting, structure, headings, and paragraphs. Do not include any explanations or extra text, only return the extracted content.",
					},
					{
						Type: "image_url",
						ImageURL: &LlamaCppImageURL{
							URL: base64Img,
						},
					},
				},
			},
		},
		Stream: false,
		MaxTokens: 4096,
	}

	reqBody, err := json.Marshal(ocrReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OCR request: %w", err)
	}

	// Create HTTP request to OpenAI compatible chat completions endpoint
	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.serverURL+"/v1/chat/completions", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("OCR request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read OCR response: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("OCR API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse response
	var ocrResp LlamaCppChatCompletionResponse
	if err := json.Unmarshal(respBody, &ocrResp); err != nil {
		return nil, fmt.Errorf("failed to parse OCR response: %w", err)
	}

	// Check for API errors
	if ocrResp.Error != nil {
		return nil, fmt.Errorf("OCR API error: %s", ocrResp.Error.Message)
	}

	// Extract text
	var text string
	if len(ocrResp.Choices) > 0 {
		text = ocrResp.Choices[0].Message.Content
	}

	// Create OCR response
	result := &OCRResponse{
		Text:       text,
		Structured: map[string]interface{}{"raw_response": ocrResp},
		Confidence: 0.8, // Default confidence for Llama.cpp OCR
		PageNum:    req.PageNum,
	}

	// For now, we don't parse blocks - this can be extended later based on actual API response format
	// If the OCR model returns structured blocks, we can parse them here

	return result, nil
}

// RecognizeBatch performs OCR on multiple images in batch.
func (c *LlamaCppOCRClient) RecognizeBatch(ctx context.Context, reqs []*OCRRequest) ([]*OCRResponse, error) {
	if len(reqs) == 0 {
		return nil, fmt.Errorf("empty batch request")
	}

	results := make([]*OCRResponse, len(reqs))
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(5) // Limit concurrent requests to avoid overwhelming the server

	for i, req := range reqs {
		idx := i
		ocrReq := req
		g.Go(func() error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			resp, err := c.Recognize(ctx, ocrReq)
			if err != nil {
				log.Error().Err(err).Int("page", ocrReq.PageNum).Msg("OCR recognition failed")
				return fmt.Errorf("page %d OCR failed: %w", ocrReq.PageNum, err)
			}

			results[idx] = resp
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, fmt.Errorf("batch OCR failed: %w", err)
	}

	return results, nil
}