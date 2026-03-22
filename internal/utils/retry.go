package utils

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"
)

// stopRetryError is a sentinel error used to stop retries.
type stopRetryError struct {
	err error
}

// Error implements the error interface.
func (e *stopRetryError) Error() string {
	return e.err.Error()
}

// Unwrap returns the underlying error.
func (e *stopRetryError) Unwrap() error {
	return e.err
}

// StopRetry wraps an error to indicate that retries should stop.
func StopRetry(err error) error {
	return &stopRetryError{err: err}
}

// RetryConfig configures retry behavior.
type RetryConfig struct {
	MaxRetries       int
	BaseDelay        time.Duration
	MaxDelay         time.Duration
	Multiplier       float64
	RetryableErrors  []string
	EnableRetryAfter bool // Whether to respect Retry-After header
}

// DefaultRetryConfig returns a default retry configuration.
func DefaultRetryConfig() *RetryConfig {
	return &RetryConfig{
		MaxRetries:       5,
		BaseDelay:        time.Second,
		MaxDelay:         32 * time.Second,
		Multiplier:       2.0,
		EnableRetryAfter: true,
	}
}

// RetryableFunc is a function that can be retried.
type RetryableFunc func() error

// DoRetry executes the given function with exponential backoff retry logic.
func DoRetry(ctx context.Context, config *RetryConfig, fn RetryableFunc) error {
	if config == nil {
		config = DefaultRetryConfig()
	}

	var lastErr error
	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		err := fn()
		if err == nil {
			return nil
		}

		// Check if this is a stop retry error
		var stopErr *stopRetryError
		if errors.As(err, &stopErr) {
			return stopErr.err
		}

		lastErr = err

		if attempt == config.MaxRetries {
			break
		}

		delay := calculateDelay(config, attempt)

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-timer.C:
		}
	}

	return fmt.Errorf("failed after %d attempts: %w", config.MaxRetries+1, lastErr)
}

func calculateDelay(config *RetryConfig, attempt int) time.Duration {
	delay := float64(config.BaseDelay) * math.Pow(config.Multiplier, float64(attempt))
	if delay > float64(config.MaxDelay) {
		delay = float64(config.MaxDelay)
	}
	return time.Duration(delay)
}
