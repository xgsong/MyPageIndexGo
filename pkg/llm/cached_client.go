package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
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

var (
	hashCache     = make(map[string]string)
	hashCacheLock sync.RWMutex
)

func hashText(prefix, text string) string {
	// For short texts, compute directly
	if len(text) < 1024 {
		h := sha256.New()
		h.Write([]byte(prefix))
		h.Write([]byte(text))
		return hex.EncodeToString(h.Sum(nil))
	}
	
	// For long texts, use cache
	cacheKey := prefix + ":" + text
	hashCacheLock.RLock()
	if hash, ok := hashCache[cacheKey]; ok {
		hashCacheLock.RUnlock()
		return hash
	}
	hashCacheLock.RUnlock()
	
	h := sha256.New()
	h.Write([]byte(prefix))
	h.Write([]byte(text))
	hash := hex.EncodeToString(h.Sum(nil))
	
	hashCacheLock.Lock()
	// Limit cache size to prevent memory leak
	if len(hashCache) < 1000 {
		hashCache[cacheKey] = hash
	}
	hashCacheLock.Unlock()
	
	return hash
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
	// Pre-compute all cache keys to avoid duplicate hash calculations
	type requestWithKey struct {
		req *BatchSummaryRequest
		key string
		idx int
	}
	
	requestsWithKeys := make([]requestWithKey, len(requests))
	for i, req := range requests {
		requestsWithKeys[i] = requestWithKey{
			req: req,
			key: hashText("GenerateSummary", req.NodeTitle+"||"+req.Text+"||"+lang.Code),
			idx: i,
		}
	}
	
	// Separate cached and uncached requests
	var cachedResponses []*BatchSummaryResponse
	var uncachedRequests []requestWithKey
	
	for _, rwk := range requestsWithKeys {
		if value, ok := c.cache.Get(rwk.key); ok {
			cachedResponses = append(cachedResponses, &BatchSummaryResponse{
				NodeID:  rwk.req.NodeID,
				Summary: value.(string),
			})
			continue
		}
		uncachedRequests = append(uncachedRequests, rwk)
	}
	
	if len(uncachedRequests) == 0 {
		return cachedResponses, nil
	}
	
	// Extract just the requests for the LLM call
	llmRequests := make([]*BatchSummaryRequest, len(uncachedRequests))
	for i, rwk := range uncachedRequests {
		llmRequests[i] = rwk.req
	}
	
	uncachedResponses, err := c.llmClient.GenerateBatchSummaries(ctx, llmRequests, lang)
	if err != nil {
		return nil, err
	}
	
	// Build final responses array
	responses := make([]*BatchSummaryResponse, len(requests))
	
	// First, fill in cached responses
	for _, resp := range cachedResponses {
		for i, req := range requests {
			if req.NodeID == resp.NodeID {
				responses[i] = resp
				break
			}
		}
	}
	
	// Then, fill in uncached responses and update cache
	for i, resp := range uncachedResponses {
		rwk := uncachedRequests[i]
		responses[rwk.idx] = resp
		if resp.Error == "" {
			c.cache.Set(rwk.key, resp.Summary)
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
