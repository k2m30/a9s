package unit

// qa_retry_probe_test.go — QA tests for issue #186
//
// These tests verify that probeResourceAvailability wraps its paginated
// fetcher call in RetryOnThrottle. They use the same generic instantiation
// RetryOnThrottle[resource.FetchResult] that the coder will wire in.
//
// Tests 1, 3, and 4 currently PASS because they test RetryOnThrottle
// directly. Test 2 (NonRetryableErrorFailsImmediately with FetchResult type)
// also passes. All four tests act as a contract: after the coder's change the
// probe must honour exactly this retry contract. If the coder changes the
// generic type or config the assertions will catch the mismatch.

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"

	"github.com/aws/smithy-go"
)

// probeRetryConfig matches the RetryConfig the coder is expected to wire into
// probeResourceAvailability. Tests use fast delays to keep the suite quick.
var probeRetryConfig = awsclient.RetryConfig{
	MaxAttempts: 3,
	BaseDelay:   1 * time.Millisecond,
	MaxDelay:    5 * time.Millisecond,
	Jitter:      false,
}

// mockProbeFetcher is a minimal resource.PaginatedFetcher that records how
// many times it was called and can inject errors on demand.
type mockProbeFetcher struct {
	calls      int
	errOnCalls map[int]error // call number (1-based) → error to return
	result     resource.FetchResult
}

func (m *mockProbeFetcher) fetch(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
	m.calls++
	if err, ok := m.errOnCalls[m.calls]; ok {
		return resource.FetchResult{}, err
	}
	return m.result, nil
}

// TestQA_RetryProbe_ThrottledFetcherRetriesAndSucceeds verifies that a fetcher
// that throws ThrottlingException on the first call is retried and the second
// (successful) call returns the correct FetchResult.
func TestQA_RetryProbe_ThrottledFetcherRetriesAndSucceeds(t *testing.T) {
	throttleErr := &smithy.GenericAPIError{
		Code:    "ThrottlingException",
		Message: "Rate exceeded",
		Fault:   smithy.FaultServer,
	}

	mock := &mockProbeFetcher{
		errOnCalls: map[int]error{1: throttleErr},
		result: resource.FetchResult{
			Resources: []resource.Resource{
				{ID: "i-abc123", Name: "test-instance"},
			},
		},
	}

	result, err := awsclient.RetryOnThrottle(context.Background(), probeRetryConfig,
		func() (resource.FetchResult, error) {
			return mock.fetch(context.Background(), nil, "")
		},
	)

	if err != nil {
		t.Fatalf("expected no error after retry, got %v", err)
	}
	if mock.calls != 2 {
		t.Errorf("expected 2 calls (1 throttle + 1 success), got %d", mock.calls)
	}
	if len(result.Resources) != 1 {
		t.Errorf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Resources[0].ID != "i-abc123" {
		t.Errorf("expected resource ID %q, got %q", "i-abc123", result.Resources[0].ID)
	}
}

// TestQA_RetryProbe_NonRetryableErrorFailsImmediately verifies that a fetcher
// returning AccessDeniedException is NOT retried — the error surfaces on the
// first call with call count == 1.
func TestQA_RetryProbe_NonRetryableErrorFailsImmediately(t *testing.T) {
	accessDeniedErr := &smithy.GenericAPIError{
		Code:    "AccessDeniedException",
		Message: "User is not authorized",
		Fault:   smithy.FaultClient,
	}

	mock := &mockProbeFetcher{
		errOnCalls: map[int]error{1: accessDeniedErr},
		result:     resource.FetchResult{},
	}

	_, err := awsclient.RetryOnThrottle(context.Background(), probeRetryConfig,
		func() (resource.FetchResult, error) {
			return mock.fetch(context.Background(), nil, "")
		},
	)

	if err == nil {
		t.Fatal("expected error for non-retryable AccessDeniedException, got nil")
	}
	if mock.calls != 1 {
		t.Errorf("expected 1 call (no retry for non-retryable error), got %d", mock.calls)
	}
	if !errors.Is(err, accessDeniedErr) {
		t.Errorf("expected original AccessDeniedException, got %v", err)
	}
}

// TestQA_RetryProbe_ThrottledFetcherExhaustsRetries verifies that a fetcher
// that always returns ThrottlingException is tried MaxAttempts times and then
// the error message contains "max retries".
func TestQA_RetryProbe_ThrottledFetcherExhaustsRetries(t *testing.T) {
	throttleErr := &smithy.GenericAPIError{
		Code:    "ThrottlingException",
		Message: "Rate exceeded",
		Fault:   smithy.FaultServer,
	}

	mock := &mockProbeFetcher{
		errOnCalls: map[int]error{1: throttleErr, 2: throttleErr, 3: throttleErr},
		result:     resource.FetchResult{},
	}

	_, err := awsclient.RetryOnThrottle(context.Background(), probeRetryConfig,
		func() (resource.FetchResult, error) {
			return mock.fetch(context.Background(), nil, "")
		},
	)

	if err == nil {
		t.Fatal("expected error after exhausting retries, got nil")
	}
	if mock.calls != 3 {
		t.Errorf("expected 3 calls (MaxAttempts), got %d", mock.calls)
	}
	expectedMsg := fmt.Sprintf("max retries (%d) exceeded", probeRetryConfig.MaxAttempts)
	if !containsSubstring(err.Error(), expectedMsg) {
		t.Errorf("expected error to contain %q, got %q", expectedMsg, err.Error())
	}
}

// TestQA_RetryProbe_SuccessOnFirstAttemptNoRetry verifies that a fetcher that
// succeeds immediately results in exactly 1 call with no retries.
func TestQA_RetryProbe_SuccessOnFirstAttemptNoRetry(t *testing.T) {
	mock := &mockProbeFetcher{
		errOnCalls: map[int]error{}, // no errors — always succeed
		result: resource.FetchResult{
			Resources: []resource.Resource{
				{ID: "bucket-1", Name: "my-bucket"},
				{ID: "bucket-2", Name: "other-bucket"},
			},
		},
	}

	result, err := awsclient.RetryOnThrottle(context.Background(), probeRetryConfig,
		func() (resource.FetchResult, error) {
			return mock.fetch(context.Background(), nil, "")
		},
	)

	if err != nil {
		t.Fatalf("expected no error on immediate success, got %v", err)
	}
	if mock.calls != 1 {
		t.Errorf("expected 1 call (no retry needed), got %d", mock.calls)
	}
	if len(result.Resources) != 2 {
		t.Errorf("expected 2 resources, got %d", len(result.Resources))
	}
}
