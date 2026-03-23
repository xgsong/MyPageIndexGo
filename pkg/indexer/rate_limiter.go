package indexer

import (
	"context"
	"sync/atomic"
	"time"

	"golang.org/x/time/rate"
)

const (
	defaultInitialConcurrency = 10
	defaultMinConcurrency     = 5
	defaultMaxConcurrency     = 40
)

type DynamicRateLimiter struct {
	limiter        *rate.Limiter
	minConcurrency atomic.Int32
	maxConcurrency atomic.Int32
	currentLimit   atomic.Int32
}

func NewDynamicRateLimiter(initialConcurrency, minConcurrency, maxConcurrency int) *DynamicRateLimiter {
	minC := int32(max(1, minConcurrency))
	maxC := int32(max(int(minC), maxConcurrency))
	initialC := int32(min(max(initialConcurrency, int(minC)), int(maxC)))

	d := &DynamicRateLimiter{
		limiter: rate.NewLimiter(rate.Limit(initialC), int(initialC)),
	}
	d.minConcurrency.Store(minC)
	d.maxConcurrency.Store(maxC)
	d.currentLimit.Store(initialC)

	return d
}

func (d *DynamicRateLimiter) Wait(ctx context.Context) error {
	return d.limiter.Wait(ctx)
}

func (d *DynamicRateLimiter) AdjustRate(remaining int, reset time.Time) {
	timeUntilReset := time.Until(reset)
	if timeUntilReset <= 0 {
		return
	}

	current := int(d.currentLimit.Load())
	minC := int(d.minConcurrency.Load())
	maxC := int(d.maxConcurrency.Load())

	var newLimit int
	if remaining < current/2 {
		newLimit = max(current/2, minC)
	} else if remaining > current*2 {
		newLimit = current * 3 / 2
		if newLimit == current {
			newLimit++
		}
		newLimit = min(newLimit, maxC)
	} else {
		return
	}

	if newLimit != current {
		d.currentLimit.Store(int32(newLimit))
		d.limiter.SetLimit(rate.Limit(newLimit))
		d.limiter.SetBurst(newLimit)
	}
}

func (d *DynamicRateLimiter) CurrentLimit() int {
	return int(d.currentLimit.Load())
}
