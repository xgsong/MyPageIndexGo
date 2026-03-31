package pool

import (
	"strings"
	"sync"
)

// BuilderPool provides a pool of strings.Builder for reuse.
// This reduces memory allocations and GC pressure in high-frequency
// string building operations.
var BuilderPool = sync.Pool{
	New: func() interface{} {
		return &strings.Builder{}
	},
}

// GetBuilder retrieves a strings.Builder from the pool.
// The builder is reset and ready to use.
func GetBuilder() *strings.Builder {
	b := BuilderPool.Get().(*strings.Builder)
	b.Reset()
	return b
}

// PutBuilder returns a strings.Builder to the pool after resetting it.
func PutBuilder(b *strings.Builder) {
	if b != nil {
		b.Reset()
		BuilderPool.Put(b)
	}
}

// BytesPool provides a pool of byte slices for reuse.
// This reduces memory allocations in high-frequency operations.
var BytesPool = sync.Pool{
	New: func() interface{} {
		// Default capacity of 4KB
		b := make([]byte, 0, 4096)
		return &b
	},
}

// GetBytes retrieves a byte slice from the pool.
// The slice has length 0 but retains its capacity.
func GetBytes(capacity int) []byte {
	bPtr := BytesPool.Get().(*[]byte)
	b := *bPtr
	if cap(b) < capacity {
		// If pooled buffer is too small, allocate a new one
		return make([]byte, 0, capacity)
	}
	return b[:0]
}

// PutBytes returns a byte slice to the pool.
// The slice will be reused, so don't modify it after returning.
func PutBytes(b []byte) {
	if b != nil && cap(b) <= 64*1024 {
		// Only pool buffers up to 64KB to avoid excessive memory retention
		bPtr := &b
		BytesPool.Put(bPtr)
	}
}
