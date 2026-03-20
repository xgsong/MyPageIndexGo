package utils

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDoRetry_Success(t *testing.T) {
	callCount := 0
	fn := func() error {
		callCount++
		return nil
	}

	ctx := context.Background()
	config := DefaultRetryConfig()
	config.MaxRetries = 3
	config.BaseDelay = 10 * time.Millisecond

	err := DoRetry(ctx, config, fn)
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
}

func TestDoRetry_Failure(t *testing.T) {
	callCount := 0
	expectedErr := errors.New("permanent failure")
	fn := func() error {
		callCount++
		return expectedErr
	}

	ctx := context.Background()
	config := DefaultRetryConfig()
	config.MaxRetries = 2
	config.BaseDelay = 10 * time.Millisecond

	err := DoRetry(ctx, config, fn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "permanent failure")
	assert.Equal(t, 3, callCount) // initial + 2 retries
}

func TestDoRetry_EventualSuccess(t *testing.T) {
	callCount := 0
	fn := func() error {
		callCount++
		if callCount < 3 {
			return errors.New("temporary failure")
		}
		return nil
	}

	ctx := context.Background()
	config := DefaultRetryConfig()
	config.MaxRetries = 5
	config.BaseDelay = 10 * time.Millisecond

	err := DoRetry(ctx, config, fn)
	assert.NoError(t, err)
	assert.Equal(t, 3, callCount)
}

func TestDoRetry_Cancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	config := DefaultRetryConfig()
	config.BaseDelay = 100 * time.Millisecond

	callCount := 0
	fn := func() error {
		callCount++
		if callCount == 1 {
			cancel() // Cancel after first call
		}
		return errors.New("error")
	}

	err := DoRetry(ctx, config, fn)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestDoRetry_NilConfig(t *testing.T) {
	fn := func() error {
		return nil
	}

	ctx := context.Background()
	err := DoRetry(ctx, nil, fn)
	assert.NoError(t, err)
}

func TestCalculateDelay(t *testing.T) {
	config := &RetryConfig{
		BaseDelay:  100 * time.Millisecond,
		MaxDelay:   1 * time.Second,
		Multiplier: 2.0,
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 100 * time.Millisecond},
		{1, 200 * time.Millisecond},
		{2, 400 * time.Millisecond},
		{3, 800 * time.Millisecond},
		{4, 1 * time.Second},  // capped at MaxDelay
		{10, 1 * time.Second}, // capped at MaxDelay
	}

	for _, tt := range tests {
		t.Run(tt.expected.String(), func(t *testing.T) {
			delay := calculateDelay(config, tt.attempt)
			assert.Equal(t, tt.expected, delay)
		})
	}
}
