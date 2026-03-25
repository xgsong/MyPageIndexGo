package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/xgsong/mypageindexgo/pkg/document"
	"github.com/xgsong/mypageindexgo/pkg/language"
)

type CachedLLMClient struct {
	llmClient         LLMClient
	cache             *LRUCache
	ttl               time.Duration
	enableSearchCache bool
}

func NewCachedLLMClient(client LLMClient, ttl time.Duration, enableSearchCache bool) LLMClient {
	return &CachedLLMClient{
		llmClient:         client,
		cache:             NewLRUCache(DefaultMaxCacheEntries, ttl),
		ttl:               ttl,
		enableSearchCache: enableSearchCache,
	}
}

func hashText(prefix, text string) string {
	h := sha256.New()
	h.Write([]byte(prefix))
	h.Write([]byte(text))
	return hex.EncodeToString(h.Sum(nil))
}

func (c *CachedLLMClient) GenerateStructure(ctx context.Context, text string, lang language.Language) (*document.Node, error) {
	key := hashText("GenerateStructure", text+lang.Code)

	if value, ok := c.cache.Get(key); ok {
		return value.(*document.Node), nil
	}

	node, err := c.llmClient.GenerateStructure(ctx, text, lang)
	if err != nil {
		return nil, err
	}

	c.cache.Set(key, node)
	return node, nil
}

func (c *CachedLLMClient) GenerateSummary(ctx context.Context, nodeTitle string, text string, lang language.Language) (string, error) {
	key := hashText("GenerateSummary", nodeTitle+"||"+text+"||"+lang.Code)

	if value, ok := c.cache.Get(key); ok {
		return value.(string), nil
	}

	summary, err := c.llmClient.GenerateSummary(ctx, nodeTitle, text, lang)
	if err != nil {
		return "", err
	}

	c.cache.Set(key, summary)
	return summary, nil
}

func (c *CachedLLMClient) Search(ctx context.Context, query string, tree *document.IndexTree) (*document.SearchResult, error) {
	if !c.enableSearchCache {
		return c.llmClient.Search(ctx, query, tree)
	}

	key := hashText("Search", query+"||"+tree.Root.Title)

	if value, ok := c.cache.Get(key); ok {
		return value.(*document.SearchResult), nil
	}

	result, err := c.llmClient.Search(ctx, query, tree)
	if err != nil {
		return nil, err
	}

	c.cache.Set(key, result)
	return result, nil
}

func (c *CachedLLMClient) GenerateBatchSummaries(ctx context.Context, requests []*BatchSummaryRequest, lang language.Language) ([]*BatchSummaryResponse, error) {
	var cachedResponses []*BatchSummaryResponse
	var uncachedRequests []*BatchSummaryRequest
	var requestIndices []int

	for i, req := range requests {
		key := hashText("GenerateSummary", req.NodeTitle+"||"+req.Text+"||"+lang.Code)
		if value, ok := c.cache.Get(key); ok {
			cachedResponses = append(cachedResponses, &BatchSummaryResponse{
				NodeID:  req.NodeID,
				Summary: value.(string),
			})
			continue
		}
		uncachedRequests = append(uncachedRequests, req)
		requestIndices = append(requestIndices, i)
	}

	if len(uncachedRequests) == 0 {
		return cachedResponses, nil
	}

	uncachedResponses, err := c.llmClient.GenerateBatchSummaries(ctx, uncachedRequests, lang)
	if err != nil {
		return nil, err
	}

	responses := make([]*BatchSummaryResponse, len(requests))
	for _, resp := range cachedResponses {
		for i, req := range requests {
			if req.NodeID == resp.NodeID {
				responses[i] = resp
				break
			}
		}
	}
	for i, resp := range uncachedResponses {
		originalIdx := requestIndices[i]
		responses[originalIdx] = resp
		if resp.Error == "" {
			req := uncachedRequests[i]
			key := hashText("GenerateSummary", req.NodeTitle+"||"+req.Text+"||"+lang.Code)
			c.cache.Set(key, resp.Summary)
		}
	}

	return responses, nil
}

// GenerateSimple generates a simple text response with caching
func (c *CachedLLMClient) GenerateSimple(ctx context.Context, prompt string) (string, error) {
	key := hashText("GenerateSimple", prompt)

	if value, ok := c.cache.Get(key); ok {
		return value.(string), nil
	}

	response, err := c.llmClient.GenerateSimple(ctx, prompt)
	if err != nil {
		return "", err
	}

	c.cache.Set(key, response)
	return response, nil
}
