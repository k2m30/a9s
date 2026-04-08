package unit

import (
	"reflect"
	"testing"

	"github.com/k2m30/a9s/v3/internal/aws/ctdetail"
)

// deepCopyParams returns a deep copy of a map[string]any for mutation-guard comparisons.
func deepCopyParams(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		switch val := v.(type) {
		case map[string]any:
			out[k] = deepCopyParams(val)
		case []any:
			cp := make([]any, len(val))
			copy(cp, val)
			out[k] = cp
		default:
			out[k] = v
		}
	}
	return out
}

// TestCTDetailSummarizeGeneric_PurityNoMutation is the load-bearing purity contract test.
// SummarizeGeneric must not mutate the params map it receives. Mutation here would corrupt
// the cleaned-params returned by ExtractTarget (T006 de-dup rule), causing downstream bugs.
func TestCTDetailSummarizeGeneric_PurityNoMutation(t *testing.T) {
	params := map[string]any{
		"foo":    "bar",
		"count":  42,
		"nested": map[string]any{"x": 1, "y": "hello"},
	}
	before := deepCopyParams(params)
	_ = ctdetail.SummarizeGeneric("SomeEvent", params)
	if !reflect.DeepEqual(params, before) {
		t.Fatalf("SummarizeGeneric mutated input params: got %v, want %v", params, before)
	}
}

// TestCTDetailSummarizeGeneric_NilInput verifies that nil params returns a non-nil empty slice.
// BuildSections relies on non-nil for empty-section omission logic.
func TestCTDetailSummarizeGeneric_NilInput(t *testing.T) {
	rows := ctdetail.SummarizeGeneric("SomeEvent", nil)
	if rows == nil {
		t.Fatal("SummarizeGeneric(nil) returned nil slice; want non-nil []Row{}")
	}
}

// TestCTDetailSummarizeGeneric_EmptyInput verifies that an empty params map returns a non-nil empty slice.
func TestCTDetailSummarizeGeneric_EmptyInput(t *testing.T) {
	rows := ctdetail.SummarizeGeneric("SomeEvent", map[string]any{})
	if rows == nil {
		t.Fatal("SummarizeGeneric(empty map) returned nil slice; want non-nil []Row{}")
	}
}

// TestCTDetailSummarizeGeneric_NoNavigableRows verifies that every Row emitted has
// IsNavigable == false and TargetType == "". The generic walker does not know enough to
// mark anything navigable — that is the per-service summarizer's responsibility.
func TestCTDetailSummarizeGeneric_NoNavigableRows(t *testing.T) {
	params := map[string]any{
		"bucketName": "my-bucket",
		"region":     "us-east-1",
		"roleArn":    "arn:aws:iam::123456789012:role/MyRole",
	}
	rows := ctdetail.SummarizeGeneric("SomeEvent", params)
	for i, row := range rows {
		if row.IsNavigable {
			t.Errorf("row[%d] key=%q: IsNavigable=true; generic summarizer must never mark rows navigable", i, row.Key)
		}
		if row.TargetType != "" {
			t.Errorf("row[%d] key=%q: TargetType=%q; want empty string from generic summarizer", i, row.Key, row.TargetType)
		}
	}
}

// TestCTDetailSummarizeGeneric_HeterogeneousTypes verifies that SummarizeGeneric does not
// panic and returns a non-nil slice when params contain mixed Go types.
func TestCTDetailSummarizeGeneric_HeterogeneousTypes(t *testing.T) {
	params := map[string]any{
		"strVal":    "hello",
		"intVal":    42,
		"floatVal":  3.14,
		"boolVal":   true,
		"nilVal":    nil,
		"sliceVal":  []any{"a", "b", 3},
		"nestedMap": map[string]any{"deep": "value"},
	}
	var rows []ctdetail.Row
	// Must not panic.
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("SummarizeGeneric panicked on heterogeneous types: %v", r)
			}
		}()
		rows = ctdetail.SummarizeGeneric("ComplexEvent", params)
	}()
	if rows == nil {
		t.Fatal("SummarizeGeneric returned nil on heterogeneous params; want non-nil slice")
	}
}

// TestCTDetailSummarizeGeneric_EventNameIgnored verifies that passing an empty or arbitrary
// eventName does not affect correctness for the same params. The generic walk does not use
// eventName — it is accepted only so the Summarizer function signature is satisfied.
func TestCTDetailSummarizeGeneric_EventNameIgnored(t *testing.T) {
	params := map[string]any{"key": "value"}

	rowsEmpty := ctdetail.SummarizeGeneric("", params)
	rowsArbitrary := ctdetail.SummarizeGeneric("SomeRandomEvent", params)

	if rowsEmpty == nil {
		t.Fatal("SummarizeGeneric(\"\", params) returned nil; want non-nil slice")
	}
	if rowsArbitrary == nil {
		t.Fatal("SummarizeGeneric(\"SomeRandomEvent\", params) returned nil; want non-nil slice")
	}
	// The output shape must be the same regardless of the eventName passed.
	if len(rowsEmpty) != len(rowsArbitrary) {
		t.Errorf("eventName changes row count: got %d for empty name, %d for non-empty name; generic walk must ignore eventName",
			len(rowsEmpty), len(rowsArbitrary))
	}
}
