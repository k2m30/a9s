// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/aws/smithy-go"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
)

// The backoff after the last permitted attempt is slept for nothing: there is
// no attempt left to space out, so the caller waits the whole delay only to be
// handed the error that was already known.
func TestRetryOnThrottle_LastAttemptDoesNotSleepItsBackoff(t *testing.T) {
	cfg := awsclient.RetryConfig{MaxAttempts: 2, BaseDelay: 300 * time.Millisecond, MaxDelay: 10 * time.Second}
	restore := awsclient.SetRetryConfigForTest(&cfg)
	defer restore()

	throttle := &smithy.GenericAPIError{Code: "Throttling", Message: "Rate exceeded"}

	calls := 0
	start := time.Now()
	_, err := awsclient.RetryOnThrottle(context.Background(), awsclient.DefaultRetryConfig(), func() (int, error) {
		calls++
		return 0, throttle
	})
	elapsed := time.Since(start)

	if err == nil || !strings.Contains(err.Error(), "max retries") {
		t.Fatalf("err = %v, want a max-retries error after %d throttled attempts", err, cfg.MaxAttempts)
	}
	if calls != cfg.MaxAttempts {
		t.Errorf("the call ran %d times, want %d", calls, cfg.MaxAttempts)
	}
	if want := 2 * cfg.BaseDelay; elapsed >= want {
		t.Errorf("RetryOnThrottle took %v, want under %v — %d attempts are spaced by %d backoff, not %d", elapsed, want, cfg.MaxAttempts, cfg.MaxAttempts-1, cfg.MaxAttempts)
	}
}
