package progress

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/schollz/progressbar/v3"
	"github.com/stretchr/testify/assert"
)

func TestNewBar(t *testing.T) {
	t.Run("creates progress bar", func(t *testing.T) {
		bar := NewBar(100, "test")
		assert.NotNil(t, bar)
	})
}

func TestProgress(t *testing.T) {
	t.Run("new progress", func(t *testing.T) {
		p := New(100, "test")
		assert.NotNil(t, p)
	})

	t.Run("add progress", func(t *testing.T) {
		p := New(100, "test")
		assert.NotPanics(t, func() {
			p.Add(10)
		})
	})

	t.Run("set progress", func(t *testing.T) {
		p := New(100, "test")
		assert.NotPanics(t, func() {
			p.Set(50)
		})
	})

	t.Run("set description", func(t *testing.T) {
		p := New(100, "test")
		assert.NotPanics(t, func() {
			p.SetDescription("new description")
		})
	})

	t.Run("finish progress", func(t *testing.T) {
		p := New(100, "test")
		assert.NotPanics(t, func() {
			p.Finish()
		})
	})
}

func TestTracker(t *testing.T) {
	t.Run("new tracker", func(t *testing.T) {
		var callbackCalled bool
		tracker := NewTracker(100, "test", func(done, total int64, desc string) {
			callbackCalled = true
		})
		assert.NotNil(t, tracker)
		assert.Equal(t, int64(0), tracker.Current())
		assert.False(t, callbackCalled)
	})

	t.Run("add calls callback", func(t *testing.T) {
		var lastDone int64
		tracker := NewTracker(100, "test", func(done, total int64, desc string) {
			lastDone = done
		})
		tracker.Add(10)
		assert.Equal(t, int64(10), lastDone)
		assert.Equal(t, int64(10), tracker.Current())
	})

	t.Run("add multiple times", func(t *testing.T) {
		var callCount int
		tracker := NewTracker(100, "test", func(done, total int64, desc string) {
			callCount++
		})
		tracker.Add(10)
		tracker.Add(20)
		tracker.Add(30)
		assert.Equal(t, 3, callCount)
		assert.Equal(t, int64(60), tracker.Current())
	})

	t.Run("done calls callback with total", func(t *testing.T) {
		var lastDone, lastTotal int64
		tracker := NewTracker(100, "test", func(done, total int64, desc string) {
			lastDone = done
			lastTotal = total
		})
		tracker.Done()
		assert.Equal(t, int64(100), lastDone)
		assert.Equal(t, int64(100), lastTotal)
	})

	t.Run("nil callback does not panic", func(t *testing.T) {
		tracker := NewTracker(100, "test", nil)
		assert.NotPanics(t, func() {
			tracker.Add(10)
			tracker.Done()
		})
		assert.Equal(t, int64(10), tracker.Current())
	})
}

func TestMultiStageTracker(t *testing.T) {
	t.Run("new multi-stage tracker", func(t *testing.T) {
		stages := []*Stage{
			{Name: "Stage 1", Start: 0, End: 50},
			{Name: "Stage 2", Start: 50, End: 100},
		}
		bar := progressbar.New(100)
		mst := NewMultiStageTracker(stages, bar)
		assert.NotNil(t, mst)
		assert.Equal(t, int64(100), mst.total)
	})

	t.Run("current tracker returns first stage", func(t *testing.T) {
		stages := []*Stage{
			{Name: "Stage 1", Start: 0, End: 50},
		}
		bar := progressbar.New(100)
		mst := NewMultiStageTracker(stages, bar)
		tracker := mst.CurrentTracker()
		assert.NotNil(t, tracker)
	})

	t.Run("next stage", func(t *testing.T) {
		stages := []*Stage{
			{Name: "Stage 1", Start: 0, End: 50},
			{Name: "Stage 2", Start: 50, End: 100},
		}
		bar := progressbar.New(100)
		mst := NewMultiStageTracker(stages, bar)

		currentTracker := mst.CurrentTracker()
		assert.NotNil(t, currentTracker)

		mst.NextStage()
		newTracker := mst.CurrentTracker()
		assert.NotNil(t, newTracker)
		assert.NotSame(t, currentTracker, newTracker)
	})

	t.Run("next stage beyond range returns nil", func(t *testing.T) {
		stages := []*Stage{
			{Name: "Stage 1", Start: 0, End: 50},
		}
		bar := progressbar.New(100)
		mst := NewMultiStageTracker(stages, bar)

		mst.NextStage()
		tracker := mst.CurrentTracker()
		assert.Nil(t, tracker)
	})

	t.Run("finish does not panic", func(t *testing.T) {
		stages := []*Stage{
			{Name: "Stage 1", Start: 0, End: 50},
		}
		bar := progressbar.New(100)
		mst := NewMultiStageTracker(stages, bar)
		assert.NotPanics(t, func() {
			mst.Finish()
		})
	})

	t.Run("finish with nil bar does not panic", func(t *testing.T) {
		stages := []*Stage{
			{Name: "Stage 1", Start: 0, End: 50},
		}
		mst := NewMultiStageTracker(stages, nil)
		assert.NotPanics(t, func() {
			mst.Finish()
		})
	})
}

func TestCtxWithProgress(t *testing.T) {
	t.Run("empty items returns nil", func(t *testing.T) {
		ctx := context.Background()
		items := []string{}
		err := CtxWithProgress(ctx, items, "test", func(ctx context.Context, item string, tracker *Tracker) error {
			return nil
		}, 1)
		assert.NoError(t, err)
	})

	t.Run("single item processed", func(t *testing.T) {
		ctx := context.Background()
		items := []string{"item1"}
		var processed bool
		err := CtxWithProgress(ctx, items, "test", func(ctx context.Context, item string, tracker *Tracker) error {
			processed = true
			assert.Equal(t, "item1", item)
			assert.NotNil(t, tracker)
			return nil
		}, 1)
		assert.NoError(t, err)
		assert.True(t, processed)
	})

	t.Run("multiple items processed", func(t *testing.T) {
		ctx := context.Background()
		items := []string{"item1", "item2", "item3"}
		var count int
		err := CtxWithProgress(ctx, items, "test", func(ctx context.Context, item string, tracker *Tracker) error {
			count++
			return nil
		}, 2)
		assert.NoError(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("error stops processing", func(t *testing.T) {
		ctx := context.Background()
		items := []string{"item1", "item2", "item3"}
		var count int
		err := CtxWithProgress(ctx, items, "test", func(ctx context.Context, item string, tracker *Tracker) error {
			count++
			if item == "item2" {
				return assert.AnError
			}
			return nil
		}, 1)
		assert.Error(t, err)
		assert.Equal(t, 3, count)
	})

	t.Run("concurrent processing", func(t *testing.T) {
		ctx := context.Background()
		items := make([]int, 10)
		for i := range items {
			items[i] = i
		}

		var count atomic.Int32
		err := CtxWithProgress(ctx, items, "test", func(ctx context.Context, item int, tracker *Tracker) error {
			time.Sleep(1 * time.Millisecond)
			count.Add(1)
			return nil
		}, 5)

		assert.NoError(t, err)
		assert.Equal(t, int32(10), count.Load())
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		items := make([]int, 5)
		for i := range items {
			items[i] = i
		}

		var count int
		err := CtxWithProgress(ctx, items, "test", func(ctx context.Context, item int, tracker *Tracker) error {
			count++
			if count >= 2 {
				cancel()
			}
			return nil
		}, 1)

		assert.NoError(t, err)
		assert.GreaterOrEqual(t, count, 2)
	})
}
