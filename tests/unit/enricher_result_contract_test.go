package unit

// enricher_result_contract_test.go — Structural contract tests for IssueEnricherResult.
//
// These tests pin the required fields of IssueEnricherResult. They fail with
// compile errors until the field exists in internal/aws/interfaces.go.
//
// Tests:
//   1. TruncatedIDs field exists with type map[string]bool.
//   2. Truncated (bool) back-compat — still present with unchanged type.
//   3. Zero-value construction is legal; callers must init maps before writing.

import (
	"reflect"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// TestIssueEnricherResult_HasTruncatedIDsField verifies that IssueEnricherResult carries a
// TruncatedIDs field of type map[string]bool. Fails with a compile error on the
// field reference until the coder adds it.
func TestIssueEnricherResult_HasTruncatedIDsField(t *testing.T) {
	rt := reflect.TypeOf(awsclient.IssueEnricherResult{})
	f, ok := rt.FieldByName("TruncatedIDs")
	if !ok {
		t.Fatal("IssueEnricherResult is missing required field TruncatedIDs — add it to internal/aws/interfaces.go")
	}
	want := reflect.TypeOf(map[string]bool{})
	if f.Type != want {
		t.Errorf("TruncatedIDs has type %v, want %v", f.Type, want)
	}
}

// TestIssueEnricherResult_Truncated_BackCompat verifies that the existing Truncated bool
// field is still present with its original type. This guards against regressions
// where a refactor removes or renames the field used by all existing enrichers.
func TestIssueEnricherResult_Truncated_BackCompat(t *testing.T) {
	rt := reflect.TypeOf(awsclient.IssueEnricherResult{})
	f, ok := rt.FieldByName("Truncated")
	if !ok {
		t.Fatal("IssueEnricherResult.Truncated field is missing — must not be removed (back-compat)")
	}
	if f.Type.Kind() != reflect.Bool {
		t.Errorf("Truncated has kind %v, want bool", f.Type.Kind())
	}
}

// TestIssueEnricherResult_ZeroValueNilMaps_IsLegalConstruction documents that constructing a
// zero-value IssueEnricherResult{} is legal (does not panic) but callers MUST initialize
// TruncatedIDs before writing to it. This is a compile-time legality guard — no
// assertions are needed beyond successful construction and return.
func TestIssueEnricherResult_ZeroValueNilMaps_IsLegalConstruction(t *testing.T) {
	// Zero-value construction must not panic.
	_ = func() awsclient.IssueEnricherResult {
		r := awsclient.IssueEnricherResult{}
		// Reading nil map is legal in Go.
		_ = r.TruncatedIDs["any-key"]
		_ = r.Truncated
		return r
	}()
	// If this test compiles and runs without panic, the contract is satisfied.
}
