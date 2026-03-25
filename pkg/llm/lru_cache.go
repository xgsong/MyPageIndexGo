package llm

import (
	"container/list"
	"sync"
	"time"
)

const DefaultMaxCacheEntries = 10000

type lruEntry struct {
	value     any
	timestamp time.Time
	key       string
	element   *list.Element
}

type LRUCache struct {
	mu      sync.RWMutex
	entries map[string]*lruEntry
	lruList *list.List
	maxSize int
	ttl     time.Duration
}

func NewLRUCache(maxSize int, ttl time.Duration) *LRUCache {
	if maxSize <= 0 {
		maxSize = DefaultMaxCacheEntries
	}
	return &LRUCache{
		entries: make(map[string]*lruEntry),
		lruList: list.New(),
		maxSize: maxSize,
		ttl:     ttl,
	}
}

func (c *LRUCache) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, exists := c.entries[key]
	if !exists {
		return nil, false
	}

	if c.ttl > 0 && time.Since(entry.timestamp) > c.ttl {
		c.removeEntry(entry)
		return nil, false
	}

	if entry.element != nil {
		c.lruList.MoveToFront(entry.element)
	}

	return entry.value, true
}

func (c *LRUCache) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, exists := c.entries[key]; exists {
		entry.value = value
		entry.timestamp = time.Now()
		c.lruList.MoveToFront(entry.element)
		return
	}

	for len(c.entries) >= c.maxSize {
		c.evictOldest()
	}

	entry := &lruEntry{
		value:     value,
		timestamp: time.Now(),
		key:       key,
	}
	entry.element = c.lruList.PushFront(entry)
	c.entries[key] = entry
}

func (c *LRUCache) Delete(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, exists := c.entries[key]; exists {
		c.removeEntry(entry)
	}
}

func (c *LRUCache) evictOldest() {
	elem := c.lruList.Back()
	if elem == nil {
		return
	}
	entry := elem.Value.(*lruEntry)
	c.removeEntry(entry)
}

func (c *LRUCache) removeEntry(entry *lruEntry) {
	delete(c.entries, entry.key)
	if entry.element != nil {
		c.lruList.Remove(entry.element)
	}
}

func (c *LRUCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}
