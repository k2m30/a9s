package unit

// ctevent_summarizer_test.go covers RegisterSummarizer in
// internal/semantics/ctevent/summarizer.go.
//
// The only uncovered branch is the duplicate-registration panic.
// The happy path (successful registration) is already exercised by the
// init() calls in summarize_ec2.go, summarize_iam.go, and summarize_s3.go
// which run before any test.

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/semantics/ctevent"
)

// TestRegisterSummarizer_DuplicatePanics verifies that registering a summarizer
// for an event source that is already registered causes a panic.
// This guards against accidentally overwriting a summarizer during init.
func TestRegisterSummarizer_DuplicatePanics(t *testing.T) {
	noop := func(_ string, _ map[string]any) []ctevent.Row { return nil }

	// Register once under a unique test-only key.
	const testKey = "test.duplicate.panic.guard"
	ctevent.RegisterSummarizer(testKey, noop)

	// A second registration for the same key must panic.
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("RegisterSummarizer: expected panic on duplicate registration, got none")
		}
	}()
	ctevent.RegisterSummarizer(testKey, noop)
}
