package unit

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"

	"github.com/aws/smithy-go"
)

func TestRetryOnThrottle_SuccessOnFirstAttempt(t *testing.T) {
	calls := 0
	result, err := awsclient.RetryOnThrottle(context.Background(), awsclient.RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		Jitter:      false,
	}, func() (string, error) {
		calls++
		return "ok", nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != "ok" {
		t.Errorf("expected result %q, got %q", "ok", result)
	}
	if calls != 1 {
		t.Errorf("expected 1 call, got %d", calls)
	}
}

func TestRetryOnThrottle_RetryAfterThrottleThenSuccess(t *testing.T) {
	throttleErr := &smithy.GenericAPIError{Code: "Throttling", Message: "Rate exceeded"}
	calls := 0
	result, err := awsclient.RetryOnThrottle(context.Background(), awsclient.RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		Jitter:      false,
	}, func() (int, error) {
		calls++
		if calls == 1 {
			return 0, throttleErr
		}
		return 42, nil
	})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result != 42 {
		t.Errorf("expected result 42, got %d", result)
	}
	if calls != 2 {
		t.Errorf("expected 2 calls, got %d", calls)
	}
}

func TestRetryOnThrottle_MaxRetriesExceeded(t *testing.T) {
	throttleErr := &smithy.GenericAPIError{Code: "Throttling", Message: "Rate exceeded"}
	calls := 0
	_, err := awsclient.RetryOnThrottle(context.Background(), awsclient.RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		Jitter:      false,
	}, func() (string, error) {
		calls++
		return "", throttleErr
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 3 {
		t.Errorf("expected 3 calls, got %d", calls)
	}
	// Error message should mention max retries
	if !errors.Is(err, throttleErr) {
		t.Errorf("expected error to wrap throttle error, got %v", err)
	}
	expectedMsg := fmt.Sprintf("max retries (%d) exceeded", 3)
	if got := err.Error(); len(got) == 0 || !containsSubstring(got, expectedMsg) {
		t.Errorf("expected error to contain %q, got %q", expectedMsg, got)
	}
}

func TestRetryOnThrottle_NonRetryableErrorFailsImmediately(t *testing.T) {
	accessDeniedErr := &smithy.GenericAPIError{Code: "AccessDeniedException", Message: "not authorized"}
	calls := 0
	_, err := awsclient.RetryOnThrottle(context.Background(), awsclient.RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		Jitter:      false,
	}, func() (string, error) {
		calls++
		return "", accessDeniedErr
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if calls != 1 {
		t.Errorf("expected 1 call (no retry for non-retryable error), got %d", calls)
	}
	if !errors.Is(err, accessDeniedErr) {
		t.Errorf("expected original access denied error, got %v", err)
	}
}

func TestRetryOnThrottle_ContextCancellation(t *testing.T) {
	throttleErr := &smithy.GenericAPIError{Code: "Throttling", Message: "Rate exceeded"}
	ctx, cancel := context.WithCancel(context.Background())
	calls := 0
	_, err := awsclient.RetryOnThrottle(ctx, awsclient.RetryConfig{
		MaxAttempts: 5,
		BaseDelay:   500 * time.Millisecond, // Long delay so context cancel triggers during wait
		MaxDelay:    10 * time.Second,
		Jitter:      false,
	}, func() (string, error) {
		calls++
		// Cancel the context after the first throttle error so it triggers during backoff
		cancel()
		return "", throttleErr
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled error, got %v", err)
	}
	if calls != 1 {
		t.Errorf("expected 1 call before context cancellation, got %d", calls)
	}
}

func TestDefaultRetryConfig(t *testing.T) {
	cfg := awsclient.DefaultRetryConfig()
	if cfg.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts=3, got %d", cfg.MaxAttempts)
	}
	if cfg.BaseDelay != 500*time.Millisecond {
		t.Errorf("expected BaseDelay=500ms, got %v", cfg.BaseDelay)
	}
	if cfg.MaxDelay != 10*time.Second {
		t.Errorf("expected MaxDelay=10s, got %v", cfg.MaxDelay)
	}
	if !cfg.Jitter {
		t.Error("expected Jitter=true")
	}
}
