package unit_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/internal/fieldpath"
)

// ---------------------------------------------------------------------------
// Test struct definitions (local to test — no AWS SDK dependency)
// ---------------------------------------------------------------------------

type testState struct {
	Name string `json:"name"`
	Code int32  `json:"code"`
}

type testPlacement struct {
	AZ      string `json:"availabilityZone"`
	Tenancy string `json:"tenancy"`
}

type testTag struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type testInstance struct {
	ID         *string        `json:"instanceId"`
	State      *testState     `json:"state"`
	Type       string         `json:"instanceType"`
	Placement  *testPlacement `json:"placement"`
	Tags       []testTag      `json:"tags"`
	LaunchTime *time.Time     `json:"launchTime"`
	MultiAZ    *bool          `json:"multiAZ"`
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func strPtr(s string) *string { return &s }

func boolPtr(b bool) *bool { return &b }

func timePtr(t time.Time) *time.Time { return &t }

func int32Ptr(i int32) *int32 { return &i }

// ---------------------------------------------------------------------------
// T004 — Dot-path extraction on simple structs
// ---------------------------------------------------------------------------

func TestExtractValue_SimpleStringField(t *testing.T) {
	inst := testInstance{Type: "t3.medium"}

	val, err := fieldpath.ExtractValue(inst, "instanceType")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val.Kind() != reflect.String {
		t.Fatalf("expected String kind, got %v", val.Kind())
	}
	if val.String() != "t3.medium" {
		t.Errorf("expected %q, got %q", "t3.medium", val.String())
	}
}

func TestExtractValue_PointerToString(t *testing.T) {
	inst := testInstance{ID: strPtr("i-abc123")}

	val, err := fieldpath.ExtractValue(inst, "instanceId")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// After extraction the pointer should be dereferenced to the underlying string.
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.String() != "i-abc123" {
		t.Errorf("expected %q, got %q", "i-abc123", val.String())
	}
}

func TestExtractValue_IntegerField(t *testing.T) {
	obj := testState{Name: "running", Code: 16}

	val, err := fieldpath.ExtractValue(obj, "code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val.Kind() != reflect.Int32 {
		t.Fatalf("expected Int32 kind, got %v", val.Kind())
	}
	if val.Int() != 16 {
		t.Errorf("expected 16, got %d", val.Int())
	}
}

func TestExtractValue_NestedStruct(t *testing.T) {
	inst := testInstance{
		State: &testState{Name: "running", Code: 16},
	}

	val, err := fieldpath.ExtractValue(inst, "state.name")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.String() != "running" {
		t.Errorf("expected %q, got %q", "running", val.String())
	}
}

func TestExtractScalar_SimpleStringField(t *testing.T) {
	inst := testInstance{Type: "t3.medium"}

	got := fieldpath.ExtractScalar(inst, "instanceType")
	if got != "t3.medium" {
		t.Errorf("expected %q, got %q", "t3.medium", got)
	}
}

func TestExtractScalar_PointerToString(t *testing.T) {
	inst := testInstance{ID: strPtr("i-abc123")}

	got := fieldpath.ExtractScalar(inst, "instanceId")
	if got != "i-abc123" {
		t.Errorf("expected %q, got %q", "i-abc123", got)
	}
}

func TestExtractScalar_IntegerField(t *testing.T) {
	obj := testState{Name: "running", Code: 16}

	got := fieldpath.ExtractScalar(obj, "code")
	if got != "16" {
		t.Errorf("expected %q, got %q", "16", got)
	}
}

func TestExtractScalar_NestedStruct(t *testing.T) {
	inst := testInstance{
		State: &testState{Name: "running", Code: 16},
	}

	got := fieldpath.ExtractScalar(inst, "state.name")
	if got != "running" {
		t.Errorf("expected %q, got %q", "running", got)
	}
}

// ---------------------------------------------------------------------------
// T006 — Edge cases
// ---------------------------------------------------------------------------

func TestExtractValue_NilPointerField(t *testing.T) {
	inst := testInstance{} // ID is nil *string

	val, err := fieldpath.ExtractValue(inst, "instanceId")
	// Either returns zero Value or an error — both acceptable.
	if err != nil {
		return // error path is fine
	}
	if val.IsValid() && !val.IsZero() {
		t.Errorf("expected zero/invalid Value for nil pointer, got %v", val)
	}
}

func TestExtractScalar_NilPointerField(t *testing.T) {
	inst := testInstance{} // ID is nil

	got := fieldpath.ExtractScalar(inst, "instanceId")
	if got != "" {
		t.Errorf("expected empty string for nil pointer, got %q", got)
	}
}

func TestExtractValue_MissingField(t *testing.T) {
	inst := testInstance{Type: "t3.medium"}

	_, err := fieldpath.ExtractValue(inst, "nonExistentField")
	if err == nil {
		t.Error("expected error for missing field, got nil")
	}
}

func TestExtractScalar_MissingField(t *testing.T) {
	inst := testInstance{Type: "t3.medium"}

	got := fieldpath.ExtractScalar(inst, "nonExistentField")
	if got != "" {
		t.Errorf("expected empty string for missing field, got %q", got)
	}
}

func TestExtractValue_DeeplyNestedThroughPointers(t *testing.T) {
	inst := testInstance{
		Placement: &testPlacement{
			AZ:      "us-east-1a",
			Tenancy: "default",
		},
	}

	val, err := fieldpath.ExtractValue(inst, "placement.availabilityZone")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	if val.String() != "us-east-1a" {
		t.Errorf("expected %q, got %q", "us-east-1a", val.String())
	}
}

func TestExtractValue_NilNestedPointer(t *testing.T) {
	inst := testInstance{} // State is nil *testState

	val, err := fieldpath.ExtractValue(inst, "state.name")
	if err != nil {
		return // error path is acceptable
	}
	if val.IsValid() && !val.IsZero() {
		t.Errorf("expected zero/invalid Value for nil nested pointer, got %v", val)
	}
}

func TestExtractScalar_NilNestedPointer(t *testing.T) {
	inst := testInstance{} // State is nil

	got := fieldpath.ExtractScalar(inst, "state.name")
	if got != "" {
		t.Errorf("expected empty string for nil nested pointer, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// T007 — YAML subtree extraction
// ---------------------------------------------------------------------------

func TestExtractSubtree_ScalarReturnsFormattedValue(t *testing.T) {
	inst := testInstance{Type: "t3.medium"}

	got := fieldpath.ExtractSubtree(inst, "instanceType")
	if got != "t3.medium" {
		t.Errorf("expected %q, got %q", "t3.medium", got)
	}
}

func TestExtractSubtree_NestedStructReturnsYAML(t *testing.T) {
	inst := testInstance{
		State: &testState{Name: "running", Code: 16},
	}

	got := fieldpath.ExtractSubtree(inst, "state")
	if got == "" {
		t.Fatal("expected non-empty YAML output for nested struct")
	}
	// The YAML should contain both fields.
	t.Run("contains_name", func(t *testing.T) {
		if !containsSubstring(got, "name") || !containsSubstring(got, "running") {
			t.Errorf("YAML output missing name field, got:\n%s", got)
		}
	})
	t.Run("contains_code", func(t *testing.T) {
		if !containsSubstring(got, "code") || !containsSubstring(got, "16") {
			t.Errorf("YAML output missing code field, got:\n%s", got)
		}
	})
}

func TestExtractSubtree_SliceReturnsYAML(t *testing.T) {
	inst := testInstance{
		Tags: []testTag{
			{Key: "env", Value: "prod"},
			{Key: "team", Value: "platform"},
		},
	}

	got := fieldpath.ExtractSubtree(inst, "tags")
	if got == "" {
		t.Fatal("expected non-empty YAML output for slice")
	}
	// The YAML should represent the slice with both tags.
	if !containsSubstring(got, "env") {
		t.Errorf("YAML output missing 'env' key, got:\n%s", got)
	}
	if !containsSubstring(got, "platform") {
		t.Errorf("YAML output missing 'platform' value, got:\n%s", got)
	}
}

func TestExtractSubtree_BoolScalar(t *testing.T) {
	inst := testInstance{MultiAZ: boolPtr(true)}

	got := fieldpath.ExtractSubtree(inst, "multiAZ")
	if got != "Yes" {
		t.Errorf("expected %q for bool true, got %q", "Yes", got)
	}
}

func TestExtractSubtree_TimeScalar(t *testing.T) {
	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	inst := testInstance{LaunchTime: timePtr(ts)}

	got := fieldpath.ExtractSubtree(inst, "launchTime")
	expected := "2025-06-15 10:30:00"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestExtractSubtree_MissingFieldReturnsEmpty(t *testing.T) {
	inst := testInstance{}

	got := fieldpath.ExtractSubtree(inst, "nonExistentField")
	if got != "" {
		t.Errorf("expected empty string for missing field, got %q", got)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func containsSubstring(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
