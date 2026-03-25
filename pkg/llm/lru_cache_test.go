package llm

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLRUCache_NewLRUCache_DefaultMaxSize(t *testing.T) {
	cache := NewLRUCache(0, 0)
	assert.Equal(t, DefaultMaxCacheEntries, cache.maxSize)
	assert.NotNil(t, cache.entries)
	assert.NotNil(t, cache.lruList)
}

func TestLRUCache_Len(t *testing.T) {
	cache := NewLRUCache(100, 0)

	assert.Equal(t, 0, cache.Len())

	cache.Set("key1", "value1")
	assert.Equal(t, 1, cache.Len())

	cache.Set("key2", "value2")
	assert.Equal(t, 2, cache.Len())

	cache.Delete("key1")
	assert.Equal(t, 1, cache.Len())

	cache.Delete("non-existent")
	assert.Equal(t, 1, cache.Len())
}

func TestLRUCache_Len_Concurrent(t *testing.T) {
	cache := NewLRUCache(1000, 0)

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			for j := 0; j < 100; j++ {
				key := string(rune('a'+idx)) + string(rune('0'+j%10))
				cache.Set(key, idx*100+j)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	assert.Equal(t, 100, cache.Len())
}

func TestLRUCache_Set_UpdateExisting(t *testing.T) {
	cache := NewLRUCache(100, 0)

	cache.Set("key1", "value1")
	cache.Set("key1", "value2")

	val, found := cache.Get("key1")
	assert.True(t, found)
	assert.Equal(t, "value2", val)
	assert.Equal(t, 1, cache.Len())
}

func TestLRUCache_Set_Eviction(t *testing.T) {
	cache := NewLRUCache(3, 0)

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	assert.Equal(t, 3, cache.Len())

	cache.Set("key4", "value4")

	assert.Equal(t, 3, cache.Len())

	_, found := cache.Get("key1")
	assert.False(t, found)

	_, found = cache.Get("key4")
	assert.True(t, found)
}

func TestLRUCache_Set_LRUEviction(t *testing.T) {
	cache := NewLRUCache(3, 0)

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	cache.Get("key1")

	cache.Set("key4", "value4")

	_, found1 := cache.Get("key1")
	assert.True(t, found1)

	_, found2 := cache.Get("key2")
	assert.False(t, found2)
}

func TestLRUCache_Get_TTLExpiration(t *testing.T) {
	cache := NewLRUCache(100, 50*time.Millisecond)

	cache.Set("key1", "value1")

	val, found := cache.Get("key1")
	assert.True(t, found)
	assert.Equal(t, "value1", val)

	time.Sleep(60 * time.Millisecond)

	val, found = cache.Get("key1")
	assert.False(t, found)
	assert.Nil(t, val)
}

func TestLRUCache_Get_NoTTL(t *testing.T) {
	cache := NewLRUCache(100, 0)

	cache.Set("key1", "value1")
	time.Sleep(10 * time.Millisecond)

	val, found := cache.Get("key1")
	assert.True(t, found)
	assert.Equal(t, "value1", val)
}

func TestLRUCache_Delete(t *testing.T) {
	cache := NewLRUCache(100, 0)

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")

	_, found := cache.Get("key1")
	assert.True(t, found)

	cache.Delete("key1")

	_, found = cache.Get("key1")
	assert.False(t, found)

	_, found = cache.Get("key2")
	assert.True(t, found)

	assert.Equal(t, 1, cache.Len())
}

func TestLRUCache_Delete_NonExistent(t *testing.T) {
	cache := NewLRUCache(100, 0)

	cache.Set("key1", "value1")
	cache.Delete("non-existent")

	assert.Equal(t, 1, cache.Len())
}

func TestLRUCache_Delete_AfterGet(t *testing.T) {
	cache := NewLRUCache(100, 0)

	cache.Set("key1", "value1")
	cache.Get("key1")
	cache.Delete("key1")

	_, found := cache.Get("key1")
	assert.False(t, found)
}

func TestLRUCache_evictOldest(t *testing.T) {
	cache := NewLRUCache(100, 0)

	cache.Set("key1", "value1")
	cache.Set("key2", "value2")
	cache.Set("key3", "value3")

	cache.evictOldest()

	assert.Equal(t, 2, cache.Len())

	_, found := cache.Get("key1")
	assert.False(t, found)

	_, found = cache.Get("key2")
	assert.True(t, found)

	_, found = cache.Get("key3")
	assert.True(t, found)
}

func TestLRUCache_evictOldest_Empty(t *testing.T) {
	cache := NewLRUCache(100, 0)

	cache.evictOldest()

	assert.Equal(t, 0, cache.Len())
}

func TestLRUCache_removeEntry(t *testing.T) {
	cache := NewLRUCache(100, 0)

	cache.Set("key1", "value1")

	cache.removeEntry(cache.entries["key1"])

	assert.Equal(t, 0, cache.Len())
	_, found := cache.Get("key1")
	assert.False(t, found)
}