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
	inst := testInstance{ID: new("i-abc123")}

	val, err := fieldpath.ExtractValue(inst, "instanceId")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// After extraction the pointer should be dereferenced to the underlying string.
	if val.Kind() == reflect.Pointer {
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
	if val.Kind() == reflect.Pointer {
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
	inst := testInstance{ID: new("i-abc123")}

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
	if val.Kind() == reflect.Pointer {
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
	inst := testInstance{MultiAZ: new(true)}

	got := fieldpath.ExtractSubtree(inst, "multiAZ")
	if got != "Yes" {
		t.Errorf("expected %q for bool true, got %q", "Yes", got)
	}
}

func TestExtractSubtree_TimeScalar(t *testing.T) {
	ts := time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)
	inst := testInstance{LaunchTime: new(ts)}

	got := fieldpath.ExtractSubtree(inst, "launchTime")
	expected := "2025-06-15 10:30"
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
// T-JSON — JSON-in-string detection via ExtractSubtree and ToSafeValue
// ---------------------------------------------------------------------------

// testJSONHolder has a *string field that can hold JSON content.
type testJSONHolder struct {
	Name     string  `json:"name"`
	JSONData *string `json:"jsonData"`
}

func TestExtractSubtree_JSONStringObject(t *testing.T) {
	raw := `{"eventVersion":"1.08","userIdentity":{"type":"AssumedRole","accountId":"123456789012"},"eventName":"CreateBucket"}`
	holder := testJSONHolder{Name: "test", JSONData: &raw}

	got := fieldpath.ExtractSubtree(holder, "jsonData")

	for _, want := range []string{"eventVersion", "1.08", "userIdentity", "AssumedRole", "CreateBucket"} {
		if !containsSubstring(got, want) {
			t.Errorf("ExtractSubtree JSON object: expected %q in output, got:\n%s", want, got)
		}
	}
	// Must NOT return the raw JSON blob as a single line
	if containsSubstring(got, `{"eventVersion"`) {
		t.Errorf("ExtractSubtree JSON object: output contains raw JSON blob, expected structured YAML:\n%s", got)
	}
}

func TestExtractSubtree_JSONStringArray(t *testing.T) {
	raw := `[{"name":"bucket1"},{"name":"bucket2"}]`
	holder := testJSONHolder{Name: "test", JSONData: &raw}

	got := fieldpath.ExtractSubtree(holder, "jsonData")

	for _, want := range []string{"bucket1", "bucket2"} {
		if !containsSubstring(got, want) {
			t.Errorf("ExtractSubtree JSON array: expected %q in output, got:\n%s", want, got)
		}
	}
}

func TestExtractSubtree_JSONStringMalformed(t *testing.T) {
	raw := `{not valid json}`
	holder := testJSONHolder{Name: "test", JSONData: &raw}

	got := fieldpath.ExtractSubtree(holder, "jsonData")

	if got != raw {
		t.Errorf("ExtractSubtree malformed JSON: expected raw string %q, got %q", raw, got)
	}
}

func TestExtractSubtree_PlainStringUnchanged(t *testing.T) {
	raw := "t3.medium"
	holder := testJSONHolder{Name: "test", JSONData: &raw}

	got := fieldpath.ExtractSubtree(holder, "jsonData")

	if got != raw {
		t.Errorf("ExtractSubtree plain string: expected %q unchanged, got %q", raw, got)
	}
}

func TestExtractSubtree_JSONStringEmpty(t *testing.T) {
	raw := ""
	holder := testJSONHolder{Name: "test", JSONData: &raw}

	got := fieldpath.ExtractSubtree(holder, "jsonData")

	if got != "" {
		t.Errorf("ExtractSubtree empty string: expected %q, got %q", "", got)
	}
}

func TestExtractSubtree_JSONStringNilPointer(t *testing.T) {
	holder := testJSONHolder{Name: "test", JSONData: nil}

	got := fieldpath.ExtractSubtree(holder, "jsonData")

	if got != "" {
		t.Errorf("ExtractSubtree nil pointer: expected empty string, got %q", got)
	}
}

func TestExtractSubtree_JSONStringDeeplyNested(t *testing.T) {
	raw := `{"a":{"b":{"c":{"d":{"e":"deep"}}}}}`
	holder := testJSONHolder{Name: "test", JSONData: &raw}

	got := fieldpath.ExtractSubtree(holder, "jsonData")

	if !containsSubstring(got, "deep") {
		t.Errorf("ExtractSubtree deeply nested JSON: expected %q in nested YAML output, got:\n%s", "deep", got)
	}
}

func TestExtractSubtree_JSONStringWithNulls(t *testing.T) {
	raw := `{"key1":"value1","key2":null}`
	holder := testJSONHolder{Name: "test", JSONData: &raw}

	got := fieldpath.ExtractSubtree(holder, "jsonData")

	if !containsSubstring(got, "key1") {
		t.Errorf("ExtractSubtree JSON with nulls: expected %q in output, got:\n%s", "key1", got)
	}
	if !containsSubstring(got, "value1") {
		t.Errorf("ExtractSubtree JSON with nulls: expected %q in output, got:\n%s", "value1", got)
	}
	// key2 null is acceptable as empty or omitted — no assertion on it
}

func TestExtractSubtree_JSONStringWhitespace(t *testing.T) {
	raw := `  {"key":"value"}  `
	holder := testJSONHolder{Name: "test", JSONData: &raw}

	got := fieldpath.ExtractSubtree(holder, "jsonData")

	for _, want := range []string{"key", "value"} {
		if !containsSubstring(got, want) {
			t.Errorf("ExtractSubtree whitespace-padded JSON: expected %q in output, got:\n%s", want, got)
		}
	}
	// Should be structured YAML, not the padded raw JSON
	if containsSubstring(got, `  {"key"`) {
		t.Errorf("ExtractSubtree whitespace-padded JSON: output contains unstripped JSON blob:\n%s", got)
	}
}

func TestToSafeValue_JSONStringParsedToMap(t *testing.T) {
	raw := `{"eventVersion":"1.08","eventName":"CreateBucket"}`
	holder := testJSONHolder{Name: "test", JSONData: &raw}

	result := fieldpath.ToSafeValue(reflect.ValueOf(holder))

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("ToSafeValue: expected map[string]interface{}, got %T", result)
	}

	jsonDataVal, exists := m["jsonData"]
	if !exists {
		t.Fatalf("ToSafeValue: expected 'jsonData' key in result map, got keys: %v", mapKeys(m))
	}

	parsed, ok := jsonDataVal.(map[string]any)
	if !ok {
		t.Fatalf("ToSafeValue JSON string: expected jsonData to be map[string]interface{} (parsed JSON), got %T: %v", jsonDataVal, jsonDataVal)
	}

	if _, hasVersion := parsed["eventVersion"]; !hasVersion {
		t.Errorf("ToSafeValue JSON string: parsed map missing 'eventVersion' key, got: %v", parsed)
	}
	if _, hasName := parsed["eventName"]; !hasName {
		t.Errorf("ToSafeValue JSON string: parsed map missing 'eventName' key, got: %v", parsed)
	}
}

func TestToSafeValue_JSONStringMalformedFallback(t *testing.T) {
	raw := `{broken`
	holder := testJSONHolder{Name: "test", JSONData: &raw}

	result := fieldpath.ToSafeValue(reflect.ValueOf(holder))

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("ToSafeValue: expected map[string]interface{}, got %T", result)
	}

	jsonDataVal, exists := m["jsonData"]
	if !exists {
		t.Fatalf("ToSafeValue malformed JSON: expected 'jsonData' key in result map")
	}

	if _, isMap := jsonDataVal.(map[string]any); isMap {
		t.Errorf("ToSafeValue malformed JSON: expected string fallback, got a map")
	}
	if _, isString := jsonDataVal.(string); !isString {
		t.Errorf("ToSafeValue malformed JSON: expected string fallback, got %T: %v", jsonDataVal, jsonDataVal)
	}
}

func TestToSafeValue_PlainStringUnchanged(t *testing.T) {
	raw := "just-a-string"
	holder := testJSONHolder{Name: "test", JSONData: &raw}

	result := fieldpath.ToSafeValue(reflect.ValueOf(holder))

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("ToSafeValue: expected map[string]interface{}, got %T", result)
	}

	jsonDataVal, exists := m["jsonData"]
	if !exists {
		t.Fatalf("ToSafeValue plain string: expected 'jsonData' key in result map")
	}

	s, ok := jsonDataVal.(string)
	if !ok {
		t.Fatalf("ToSafeValue plain string: expected string, got %T: %v", jsonDataVal, jsonDataVal)
	}
	if s != "just-a-string" {
		t.Errorf("ToSafeValue plain string: expected %q, got %q", "just-a-string", s)
	}
}

func TestToSafeValue_JSONStringArrayParsed(t *testing.T) {
	raw := `["a","b","c"]`
	holder := testJSONHolder{Name: "test", JSONData: &raw}

	result := fieldpath.ToSafeValue(reflect.ValueOf(holder))

	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("ToSafeValue: expected map[string]interface{}, got %T", result)
	}

	jsonDataVal, exists := m["jsonData"]
	if !exists {
		t.Fatalf("ToSafeValue JSON array: expected 'jsonData' key in result map")
	}

	if _, isSlice := jsonDataVal.([]any); !isSlice {
		t.Errorf("ToSafeValue JSON array: expected []interface{} (parsed JSON array), got %T: %v", jsonDataVal, jsonDataVal)
	}
}

// mapKeys returns the keys of a map for use in error messages.
func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ---------------------------------------------------------------------------
// TestExtractFirstListScalar — behavior tests for the list-scalar walker
// ---------------------------------------------------------------------------

// Structs used only by ExtractFirstListScalar tests — kept local to avoid
// polluting the shared test-struct namespace.

type flsSubnet struct {
	SubnetId *string
}

type flsVpc struct {
	Subnets []flsSubnet
}

type flsItem struct {
	Name string
}

type flsContainer struct {
	Items []flsItem
}

type flsInner struct {
	ID string
}

type flsOuter struct {
	// *[]flsInner — pointer-to-slice intermediate
	Children *[]flsInner
}

type flsScalarOnly struct {
	Region string
}

func flsPtr(s string) *string { return &s }

func TestExtractFirstListScalar(t *testing.T) {
	emptySubnets := []flsSubnet{}
	innerSlice := []flsInner{{ID: "inner-1"}, {ID: "inner-2"}}

	tests := []struct {
		name  string
		obj   any
		path  string
		want  string
	}{
		{
			name: "list of structs with pointer field — returns first element value",
			obj: flsVpc{
				Subnets: []flsSubnet{
					{SubnetId: flsPtr("subnet-abc")},
					{SubnetId: flsPtr("subnet-xyz")},
				},
			},
			path: "Subnets.SubnetId",
			want: "subnet-abc",
		},
		{
			name: "list of structs with value (non-pointer) field — returns first element value",
			obj: flsContainer{
				Items: []flsItem{
					{Name: "alpha"},
					{Name: "beta"},
				},
			},
			path: "Items.Name",
			want: "alpha",
		},
		{
			name: "pointer-to-slice intermediate — walked transparently",
			obj: flsOuter{
				Children: &innerSlice,
			},
			path: "Children.ID",
			want: "inner-1",
		},
		{
			name: "empty slice — returns empty string",
			obj: flsVpc{
				Subnets: emptySubnets,
			},
			path: "Subnets.SubnetId",
			want: "",
		},
		{
			name: "nil intermediate pointer — returns empty string",
			obj:  flsOuter{Children: nil},
			path: "Children.ID",
			want: "",
		},
		{
			name: "path with no list — behaves like ExtractScalar",
			obj:  flsScalarOnly{Region: "us-east-1"},
			path: "Region",
			want: "us-east-1",
		},
		{
			name: "empty path — returns empty string",
			obj:  flsScalarOnly{Region: "us-east-1"},
			path: "",
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := fieldpath.ExtractFirstListScalar(tc.obj, tc.path)
			if got != tc.want {
				t.Errorf("ExtractFirstListScalar(%T, %q) = %q; want %q", tc.obj, tc.path, got, tc.want)
			}
		})
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
