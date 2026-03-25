package llm

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/sashabaranov/go-openai"
	"github.com/xgsong/mypageindexgo/pkg/config"
	"github.com/xgsong/mypageindexgo/pkg/document"
)

// OpenAIOCRClient implements document.OCRClient using OpenAI-compatible API.
// This can be used with llama.cpp server or any OpenAI-compatible endpoint.
type OpenAIOCRClient struct {
	client       *openai.Client
	model        string
	systemPrompt string
}

// NewOpenAIOCRClient creates a new OCR client from configuration.
func NewOpenAIOCRClient(cfg *config.Config) *OpenAIOCRClient {
	clientConfig := openai.DefaultConfig(cfg.OCRAPIKey)
	if cfg.OpenAIOCRBaseURL != "" {
		baseURL := cfg.OpenAIOCRBaseURL
		if len(baseURL) > 0 && baseURL[len(baseURL)-1] != '/' {
			baseURL += "/"
		}
		// Ensure URL ends with /v1 for OpenAI compatibility
		if !strings.HasSuffix(baseURL, "v1/") {
			baseURL += "v1/"
		}
		clientConfig.BaseURL = baseURL
	}

	model := cfg.OCRModel
	if model == "" {
		model = "GLM-OCR-Q8_0"
	}

	return &OpenAIOCRClient{
		client: openai.NewClientWithConfig(clientConfig),
		model:  model,
		systemPrompt: `You are an OCR (Optical Character Recognition) engine.
Your task is to extract all text content from the provided image accurately.
Preserve the original layout and formatting as much as possible.
Do not add any explanations, summaries, or additional text.
Output only the extracted text content.`,
	}
}

// NewOpenAIOCRClientWithModel creates a new OCR client with a specific model.
func NewOpenAIOCRClientWithModel(cfg *config.Config, model string) *OpenAIOCRClient {
	client := NewOpenAIOCRClient(cfg)
	if model != "" {
		client.model = model
	}
	return client
}

// Recognize performs OCR on the given image data.
func (c *OpenAIOCRClient) Recognize(ctx context.Context, req *document.OCRRequest) (*document.OCRResponse, error) {
	if len(req.ImageData) == 0 {
		return &document.OCRResponse{Error: "empty image data"}, fmt.Errorf("empty image data")
	}

	base64Image := base64.StdEncoding.EncodeToString(req.ImageData)

	chatReq := openai.ChatCompletionRequest{
		Model: c.model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: c.systemPrompt,
			},
			{
				Role:    openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{
						Type: openai.ChatMessagePartTypeText,
						Text: "Extract all text from this image:",
					},
					{
						Type: openai.ChatMessagePartTypeImageURL,
						ImageURL: &openai.ChatMessageImageURL{
							URL: fmt.Sprintf("data:image/png;base64,%s", base64Image),
						},
					},
				},
			},
		},
		Stream: false,
	}

	resp, err := c.client.CreateChatCompletion(ctx, chatReq)
	if err != nil {
		return &document.OCRResponse{Error: err.Error()}, fmt.Errorf("OCR API call failed for page %d: %w", req.PageNum, err)
	}

	if len(resp.Choices) == 0 {
		return &document.OCRResponse{Error: "no response from OCR model"}, fmt.Errorf("no response from OCR model for page %d", req.PageNum)
	}

	return &document.OCRResponse{
		Text: resp.Choices[0].Message.Content,
	}, nil
}

// RecognizeBatch performs OCR on multiple images in batch.
func (c *OpenAIOCRClient) RecognizeBatch(ctx context.Context, reqs []*document.OCRRequest) ([]*document.OCRResponse, error) {
	if len(reqs) == 0 {
		return []*document.OCRResponse{}, nil
	}

	responses := make([]*document.OCRResponse, len(reqs))
	for i, req := range reqs {
		resp, err := c.Recognize(ctx, req)
		if err != nil {
			responses[i] = &document.OCRResponse{
				Error: err.Error(),
			}
		} else {
			responses[i] = resp
		}
	}
	return responses, nil
}
