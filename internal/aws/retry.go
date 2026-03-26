package aws

import (
	"context"
	"fmt"
	"math/rand"
	"time"
)

// RetryConfig controls retry behavior for AWS API calls.
type RetryConfig struct {
	MaxAttempts int
	BaseDelay   time.Duration
	MaxDelay    time.Duration
	Jitter      bool
}

// DefaultRetryConfig returns the standard retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   500 * time.Millisecond,
		MaxDelay:    10 * time.Second,
		Jitter:      true,
	}
}

// RetryOnThrottle wraps an API call with exponential backoff for throttling errors.
// Only retries on errors where ClassifyAWSError returns retryable=true.
func RetryOnThrottle[T any](ctx context.Context, cfg RetryConfig, fn func() (T, error)) (T, error) {
	var lastErr error
	for attempt := range cfg.MaxAttempts {
		result, err := fn()
		if err == nil {
			return result, nil
		}

		_, _, retryable := ClassifyAWSError(err)
		if !retryable {
			return result, err
		}

		lastErr = err

		delay := cfg.BaseDelay * time.Duration(1<<attempt)
		if delay > cfg.MaxDelay {
			delay = cfg.MaxDelay
		}
		if cfg.Jitter {
			delay = delay/2 + time.Duration(rand.Int63n(int64(delay/2))) //nolint:gosec // jitter does not need crypto rand
		}

		select {
		case <-ctx.Done():
			var zero T
			return zero, ctx.Err()
		case <-time.After(delay):
		}
	}

	var zero T
	return zero, fmt.Errorf("max retries (%d) exceeded: %w", cfg.MaxAttempts, lastErr)
}
