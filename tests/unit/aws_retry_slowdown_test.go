package unit

// aws_retry_slowdown_test.go — issue #456: "SlowDown" is a legitimate S3
// (and DynamoDB) throttle error code that ClassifyAWSError does not
// currently recognize, so RetryOnThrottle gives up on the first attempt
// instead of backing off and retrying like it does for Throttling /
// ThrottlingException / TooManyRequestsException / RequestLimitExceeded.
//
// Reuses MockAPIError from mocks_test.go (same package unit) — no new
// mocks needed.

import (
	"context"
	"testing"
	"time"

	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// TestClassifyAWSError_SlowDown pins retryable=true for Code "SlowDown".
func TestClassifyAWSError_SlowDown(t *testing.T) {
	err := &MockAPIError{Code: "SlowDown", Message: "Please reduce your request rate.", Fault: smithy.FaultClient}
	code, message, retryable := awsclient.ClassifyAWSError(err)
	if code != "SlowDown" {
		t.Errorf("expected code %q, got %q", "SlowDown", code)
	}
	if message != "Please reduce your request rate." {
		t.Errorf("expected message %q, got %q", "Please reduce your request rate.", message)
	}
	if !retryable {
		t.Error("expected retryable=true for SlowDown (S3/DynamoDB throttle code)")
	}
}

// TestRetryOnThrottle_RetriesSlowDownThenSucceeds pins the end-to-end
// behavior: RetryOnThrottle must back off and retry a SlowDown error
// instead of returning it on the first attempt.
func TestRetryOnThrottle_RetriesSlowDownThenSucceeds(t *testing.T) {
	slowDownErr := &smithy.GenericAPIError{Code: "SlowDown", Message: "Please reduce your request rate."}
	calls := 0
	result, err := awsclient.RetryOnThrottle(context.Background(), awsclient.RetryConfig{
		MaxAttempts: 3,
		BaseDelay:   1 * time.Millisecond,
		MaxDelay:    10 * time.Millisecond,
		Jitter:      false,
	}, func() (int, error) {
		calls++
		if calls == 1 {
			return 0, slowDownErr
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
		t.Errorf("expected 2 calls (retry after SlowDown), got %d", calls)
	}
}
