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

// CacheEntry represents an entry in the LLM cache
type CacheEntry struct {
	value     interface{}
	timestamp time.Time
}

// CachedLLMClient wraps an LLMClient with caching functionality
type CachedLLMClient struct {
	llmClient         LLMClient
	cache             *sync.Map
	ttl               time.Duration
	enableSearchCache bool
}

// NewCachedLLMClient creates a new cached LLM client
func NewCachedLLMClient(client LLMClient, ttl time.Duration, enableSearchCache bool) LLMClient {
	return &CachedLLMClient{
		llmClient:         client,
		cache:             &sync.Map{},
		ttl:               ttl,
		enableSearchCache: enableSearchCache,
	}
}

// hashText generates a hash key for the given prefix and text
func hashText(prefix, text string) string {
	h := sha256.New()
	h.Write([]byte(prefix))
	h.Write([]byte(text))
	return hex.EncodeToString(h.Sum(nil))
}

// isExpired checks if a cache entry is expired
func (c *CachedLLMClient) isExpired(entry CacheEntry) bool {
	if c.ttl <= 0 {
		return false
	}
	return time.Since(entry.timestamp) > c.ttl
}

// GenerateStructure generates a hierarchical tree structure from raw page text, using cache if available
func (c *CachedLLMClient) GenerateStructure(ctx context.Context, text string, lang language.Language) (*document.Node, error) {
	key := hashText("GenerateStructure", text+lang.Code)

	if entry, ok := c.cache.Load(key); ok {
		cacheEntry := entry.(CacheEntry)
		if !c.isExpired(cacheEntry) {
			return cacheEntry.value.(*document.Node), nil
		}
		// Remove expired entry
		c.cache.Delete(key)
	}

	node, err := c.llmClient.GenerateStructure(ctx, text, lang)
	if err != nil {
		return nil, err
	}

	c.cache.Store(key, CacheEntry{
		value:     node,
		timestamp: time.Now(),
	})
	return node, nil
}

// GenerateSummary generates a concise summary for a node, using cache if available
func (c *CachedLLMClient) GenerateSummary(ctx context.Context, nodeTitle string, text string, lang language.Language) (string, error) {
	key := hashText("GenerateSummary", nodeTitle+"||"+text+"||"+lang.Code)

	if entry, ok := c.cache.Load(key); ok {
		cacheEntry := entry.(CacheEntry)
		if !c.isExpired(cacheEntry) {
			return cacheEntry.value.(string), nil
		}
		// Remove expired entry
		c.cache.Delete(key)
	}

	summary, err := c.llmClient.GenerateSummary(ctx, nodeTitle, text, lang)
	if err != nil {
		return "", err
	}

	c.cache.Store(key, CacheEntry{
		value:     summary,
		timestamp: time.Now(),
	})
	return summary, nil
}

// Search performs reasoning-based retrieval on the index tree, using cache if enabled
func (c *CachedLLMClient) Search(ctx context.Context, query string, tree *document.IndexTree) (*document.SearchResult, error) {
	if !c.enableSearchCache {
		return c.llmClient.Search(ctx, query, tree)
	}

	// Create cache key from query and tree content
	key := hashText("Search", query+"||"+tree.Root.Title)

	if entry, ok := c.cache.Load(key); ok {
		cacheEntry := entry.(CacheEntry)
		if !c.isExpired(cacheEntry) {
			return cacheEntry.value.(*document.SearchResult), nil
		}
		c.cache.Delete(key)
	}

	result, err := c.llmClient.Search(ctx, query, tree)
	if err != nil {
		return nil, err
	}

	c.cache.Store(key, CacheEntry{
		value:     result,
		timestamp: time.Now(),
	})
	return result, nil
}

// GenerateBatchSummaries generates summaries for multiple nodes in batch, using cache where available
func (c *CachedLLMClient) GenerateBatchSummaries(ctx context.Context, requests []*BatchSummaryRequest, lang language.Language) ([]*BatchSummaryResponse, error) {
	// Split requests into cached and uncached
	var cachedResponses []*BatchSummaryResponse
	var uncachedRequests []*BatchSummaryRequest
	var requestIndices []int // track original indices of uncached requests

	for i, req := range requests {
		key := hashText("GenerateSummary", req.NodeTitle+"||"+req.Text+"||"+lang.Code)
		if entry, ok := c.cache.Load(key); ok {
			cacheEntry := entry.(CacheEntry)
			if !c.isExpired(cacheEntry) {
				// Return cached result
				cachedResponses = append(cachedResponses, &BatchSummaryResponse{
					NodeID:  req.NodeID,
					Summary: cacheEntry.value.(string),
				})
				continue
			}
			// Remove expired entry
			c.cache.Delete(key)
		}
		// Need to fetch this one
		uncachedRequests = append(uncachedRequests, req)
		requestIndices = append(requestIndices, i)
	}

	// If all are cached, return immediately
	if len(uncachedRequests) == 0 {
		return cachedResponses, nil
	}

	// Fetch uncached requests in batch
	uncachedResponses, err := c.llmClient.GenerateBatchSummaries(ctx, uncachedRequests, lang)
	if err != nil {
		return nil, err
	}

	// Cache the new responses and merge with cached ones
	responses := make([]*BatchSummaryResponse, len(requests))
	// Fill cached responses first
	for _, resp := range cachedResponses {
		// Find the original index
		for i, req := range requests {
			if req.NodeID == resp.NodeID {
				responses[i] = resp
				break
			}
		}
	}
	// Fill uncached responses
	for i, resp := range uncachedResponses {
		originalIdx := requestIndices[i]
		responses[originalIdx] = resp
		// Cache the result if no error
		if resp.Error == "" {
			req := uncachedRequests[i]
			key := hashText("GenerateSummary", req.NodeTitle+"||"+req.Text+"||"+lang.Code)
			c.cache.Store(key, CacheEntry{
				value:     resp.Summary,
				timestamp: time.Now(),
			})
		}
	}

	return responses, nil
}
