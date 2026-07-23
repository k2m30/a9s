package unit

// runtime_handle_api_error_nilerr_test.go — fresh coverage for
// Core.HandleAPIError's nil-Err guard (core/runtime/handlers.go): calling
// ev.Err.Error() on a nil error interface panics (invalid memory address /
// nil pointer dereference) — the exact call HandleAPIError makes today when
// ClassifyAWSError(nil) returns an empty, unclassified code. The nil-Err
// guard must short-circuit that branch to a fixed "unknown API error" flash
// text instead, so a caller that (mistakenly or not) dispatches
// TaskKindEmitAPIError / drives HandleAPIError with a nil Err never crashes
// the session.
//
// The other two branches of the same text-selection switch — a classified
// smithy.APIError producing "[code] message", and an unclassified non-nil
// error falling back to Err.Error() — are already covered by core/runtime's
// own white-box handlers_test.go; the table test below re-pins that same
// selection logic from tests/unit's black-box surface so the three branches
// are asserted together as one contract instead of scattered across two
// packages.

import (
	"errors"
	"testing"

	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// TestHandleAPIError_NilErr_NoPanic drives HandleAPIError with a nil Err and
// asserts: no panic, a FlashIntent with IsError=true and the fixed
// "unknown API error" text, and the same FlashTick task HandleAPIError always
// schedules — the nil-Err path must degrade gracefully, not skip the rest of
// the handler's contract.
func TestHandleAPIError_NilErr_NoPanic(t *testing.T) {
	c := runtime.New(session.New(), nil)

	var intents []runtime.UIIntent
	var tasks []runtime.TaskRequest
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("HandleAPIError panicked on a nil Err: %v", r)
			}
		}()
		intents, tasks = c.HandleAPIError(runtime.APIErrorEvent{Err: nil, NewGen: 3})
	}()

	var flash runtime.FlashIntent
	found := false
	for _, in := range intents {
		if fi, ok := in.(runtime.FlashIntent); ok {
			flash = fi
			found = true
		}
	}
	if !found {
		t.Fatal("HandleAPIError(nil Err) returned no FlashIntent")
	}
	if flash.Text != "unknown API error" {
		t.Errorf("FlashIntent.Text = %q, want %q", flash.Text, "unknown API error")
	}
	if !flash.IsError {
		t.Error("FlashIntent.IsError = false, want true")
	}

	hasTick := false
	for _, task := range tasks {
		if task.Key.Kind == runtime.TaskKindFlashTick {
			hasTick = true
		}
	}
	if !hasTick {
		t.Error("HandleAPIError(nil Err) returned no TaskKindFlashTick task")
	}
}

// TestHandleAPIError_MessageSelectionTable pins the full three-way text
// selection: nil Err -> fixed fallback; a non-nil error that ClassifyAWSError
// cannot classify as a smithy.APIError -> Err.Error() verbatim; a matched
// smithy.APIError -> "[code] message". A matched APIError with an empty
// code (Code:"") is included as the boundary row: ClassifyAWSError only
// special-cases a FAILED errors.As match ("Unknown"), so a successful match
// with a genuinely empty ErrorCode() must still fall through to Err.Error(),
// exactly like the unclassified-error row.
func TestHandleAPIError_MessageSelectionTable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "nil Err falls back to the fixed unknown-API-error text",
			err:  nil,
			want: "unknown API error",
		},
		{
			name: "unclassified non-nil error uses Err.Error() verbatim",
			err:  errors.New("boom"),
			want: "boom",
		},
		{
			name: "classified smithy.APIError uses [code] message",
			err:  &MockAPIError{Code: "AccessDenied", Message: "nope"},
			want: "[AccessDenied] nope",
		},
		{
			name: "matched APIError with an empty code falls back to Err.Error()",
			err:  &MockAPIError{Code: "", Message: "weird"},
			want: "weird",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := runtime.New(session.New(), nil)
			intents, _ := c.HandleAPIError(runtime.APIErrorEvent{Err: tt.err, NewGen: 1})

			var flash runtime.FlashIntent
			found := false
			for _, in := range intents {
				if fi, ok := in.(runtime.FlashIntent); ok {
					flash = fi
					found = true
				}
			}
			if !found {
				t.Fatal("HandleAPIError returned no FlashIntent")
			}
			if flash.Text != tt.want {
				t.Errorf("FlashIntent.Text = %q, want %q", flash.Text, tt.want)
			}
		})
	}
}
