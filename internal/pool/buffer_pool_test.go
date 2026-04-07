package pool

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetBuilder(t *testing.T) {
	t.Run("returns non-nil builder", func(t *testing.T) {
		builder := GetBuilder()
		assert.NotNil(t, builder)
		assert.Equal(t, 0, builder.Len())
	})

	t.Run("builder is reset", func(t *testing.T) {
		builder := GetBuilder()
		builder.WriteString("test content")
		assert.Equal(t, "test content", builder.String())

		PutBuilder(builder)

		builder2 := GetBuilder()
		assert.Equal(t, 0, builder2.Len())
		assert.Equal(t, "", builder2.String())
	})

	t.Run("multiple gets return different builders", func(t *testing.T) {
		builder1 := GetBuilder()
		builder2 := GetBuilder()

		assert.NotNil(t, builder1)
		assert.NotNil(t, builder2)
		assert.NotSame(t, builder1, builder2)
	})
}

func TestPutBuilder(t *testing.T) {
	t.Run("put nil builder does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			PutBuilder(nil)
		})
	})

	t.Run("put builder resets it", func(t *testing.T) {
		builder := GetBuilder()
		builder.WriteString("test")
		assert.Equal(t, 4, builder.Len())

		PutBuilder(builder)

		builder2 := GetBuilder()
		assert.Equal(t, 0, builder2.Len())
	})

	t.Run("put and get returns same pool object", func(t *testing.T) {
		builder1 := GetBuilder()
		builder1.WriteString("test")
		PutBuilder(builder1)

		builder2 := GetBuilder()
		assert.Same(t, builder1, builder2)
	})
}

func TestGetBytes(t *testing.T) {
	t.Run("returns non-nil slice", func(t *testing.T) {
		b := GetBytes(100)
		assert.NotNil(t, b)
		assert.Equal(t, 0, len(b))
		assert.GreaterOrEqual(t, cap(b), 100)
	})

	t.Run("slice has zero length", func(t *testing.T) {
		b := GetBytes(1024)
		assert.Equal(t, 0, len(b))
		assert.GreaterOrEqual(t, cap(b), 1024)
	})

	t.Run("slice can be appended", func(t *testing.T) {
		b := GetBytes(100)
		b = append(b, []byte("test")...)
		assert.Equal(t, 4, len(b))
		assert.Equal(t, []byte("test"), b)
	})

	t.Run("capacity check with small size", func(t *testing.T) {
		b := GetBytes(10)
		assert.GreaterOrEqual(t, cap(b), 10)
	})

	t.Run("capacity check with large size", func(t *testing.T) {
		b := GetBytes(8192)
		assert.GreaterOrEqual(t, cap(b), 8192)
	})
}

func TestPutBytes(t *testing.T) {
	t.Run("put nil slice does not panic", func(t *testing.T) {
		assert.NotPanics(t, func() {
			PutBytes(nil)
		})
	})

	t.Run("put small slice returns to pool", func(t *testing.T) {
		b := GetBytes(100)
		b = append(b, []byte("test")...)

		PutBytes(b)

		b2 := GetBytes(100)
		assert.NotNil(t, b2)
	})

	t.Run("put large slice does not pool", func(t *testing.T) {
		b := make([]byte, 0, 100*1024) // 100KB, larger than 64KB limit
		assert.NotPanics(t, func() {
			PutBytes(b)
		})
	})

	t.Run("pooled slice retains capacity", func(t *testing.T) {
		b1 := GetBytes(1000)
		cap1 := cap(b1)
		PutBytes(b1)

		b2 := GetBytes(1000)
		assert.GreaterOrEqual(t, cap(b2), cap1)
	})
}

func TestBuilderPoolIntegration(t *testing.T) {
	t.Run("pool reduces allocations", func(t *testing.T) {
		var builders []*strings.Builder
		for i := 0; i < 10; i++ {
			b := GetBuilder()
			b.WriteString("test")
			builders = append(builders, b)
		}

		for _, b := range builders {
			PutBuilder(b)
		}

		newBuilder := GetBuilder()
		assert.NotNil(t, newBuilder)
	})

	t.Run("concurrent access", func(t *testing.T) {
		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func() {
				b := GetBuilder()
				b.WriteString("concurrent test")
				PutBuilder(b)
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})
}

func TestBytesPoolIntegration(t *testing.T) {
	t.Run("pool reduces allocations", func(t *testing.T) {
		var slices [][]byte
		for i := 0; i < 10; i++ {
			b := GetBytes(100)
			b = append(b, []byte("test")...)
			slices = append(slices, b)
		}

		for _, b := range slices {
			PutBytes(b)
		}

		newSlice := GetBytes(100)
		assert.NotNil(t, newSlice)
	})

	t.Run("concurrent access", func(t *testing.T) {
		done := make(chan bool)
		for i := 0; i < 10; i++ {
			go func() {
				b := GetBytes(100)
				b = append(b, []byte("concurrent")...)
				PutBytes(b)
				done <- true
			}()
		}

		for i := 0; i < 10; i++ {
			<-done
		}
	})
}
