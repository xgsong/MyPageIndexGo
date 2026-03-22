package indexer

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// DynamicRateLimiter implements a dynamic rate limiter that adjusts based on API feedback.
// It uses a token bucket algorithm to control the rate of requests.
type DynamicRateLimiter struct {
	mu             sync.Mutex
	limiter        *rate.Limiter
	minConcurrency int
	maxConcurrency int
	currentLimit   int
}

// NewDynamicRateLimiter creates a new DynamicRateLimiter with the given initial, min, and max concurrency limits.
func NewDynamicRateLimiter(initialConcurrency, minConcurrency, maxConcurrency int) *DynamicRateLimiter {
	if minConcurrency < 1 {
		minConcurrency = 1
	}
	if maxConcurrency < minConcurrency {
		maxConcurrency = minConcurrency
	}
	if initialConcurrency < minConcurrency {
		initialConcurrency = minConcurrency
	}
	if initialConcurrency > maxConcurrency {
		initialConcurrency = maxConcurrency
	}

	return &DynamicRateLimiter{
		limiter:        rate.NewLimiter(rate.Limit(initialConcurrency), initialConcurrency),
		minConcurrency: minConcurrency,
		maxConcurrency: maxConcurrency,
		currentLimit:   initialConcurrency,
	}
}

// Wait blocks until a token is available or the context is canceled.
func (d *DynamicRateLimiter) Wait(ctx context.Context) error {
	return d.limiter.Wait(ctx)
}

// AdjustRate adjusts the rate limit based on remaining quota and reset time.
// remaining: number of remaining requests in the current window
// reset: time when the rate limit window resets
func (d *DynamicRateLimiter) AdjustRate(remaining int, reset time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Calculate time until reset
	timeUntilReset := time.Until(reset)
	if timeUntilReset <= 0 {
		// Already reset, no need to adjust
		return
	}

	// If remaining is low, reduce concurrency
	var newLimit int
	if remaining < d.currentLimit/2 {
		// Reduce by 50% but not below min
		newLimit = d.currentLimit / 2
		if newLimit < d.minConcurrency {
			newLimit = d.minConcurrency
		}
	} else if remaining > d.currentLimit*2 {
		// Increase by 50% but not above max
		newLimit = d.currentLimit * 3 / 2
		// Handle case where integer truncation doesn't increase the limit
		if newLimit == d.currentLimit {
			newLimit++
		}
		if newLimit > d.maxConcurrency {
			newLimit = d.maxConcurrency
		}
	} else {
		// Keep current limit
		return
	}

	if newLimit != d.currentLimit {
		d.currentLimit = newLimit
		d.limiter.SetLimit(rate.Limit(newLimit))
		d.limiter.SetBurst(newLimit)
	}
}

// CurrentLimit returns the current concurrency limit.
func (d *DynamicRateLimiter) CurrentLimit() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.currentLimit
}
