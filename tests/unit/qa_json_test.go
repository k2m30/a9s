// qa_json_test.go — JSON view RawContent/colorize correctness.
//
// views.NewJSON/View/RawContent/FrameTitle/CopyContent/GetHelpContext/
// BottomHints/ResourceID/SearchInfo/SetSize-resize are DEAD per
// specs/022-codebase-cleanup/wave3-map-text.md (json.go). Most of that
// surface was accessor-level and is already covered elsewhere on the live
// path (colorize golden + uncolored-copy: wave3_text_ports_test.go's
// TestWave3Port_JSONCopy_UncoloredContent /
// TestWave3Port_JSON_ColorizeGolden_LiveContentLines). What survives here —
// retargeted onto NewJSONWithCtrl + ContentLines() — is JSON-specific
// marshal-correctness behavior with no YAML equivalent: json.MarshalIndent
// (unlike YAML's fieldpath.ToSafeValue) does NOT omit zero/false values, and
// native JSON types (bool/float64) round-trip through json.Unmarshal.
package unit

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// testJSONStruct is a simple struct for RawStruct tests.
type testJSONStruct struct {
	Name  string
	Count int
}

// jsonModel creates a live JSONModel (NewJSONWithCtrl) from a resource and
// sets its size.
func jsonModel(res resource.Resource, w, h int) views.JSONModel {
	k := keys.Default()
	m := views.NewJSONWithCtrl(res, "", k, nil)
	m.SetSize(w, h)
	return m
}

// jsonRawContent returns the uncolored JSON text via the live ContentLines()
// seam — the ANSI-stripped equivalent of the dead RawContent() method.
func jsonRawContent(m views.JSONModel) string {
	return stripANSI(strings.Join(m.ContentLines(), "\n"))
}

// TestJSON_RawContent_WithRawStruct verifies the JSON content is valid
// 2-space-indented JSON with the struct fields when RawStruct is set.
func TestJSON_RawContent_WithRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:   "bucket-1",
		Name: "test-bucket",
		RawStruct: &testJSONStruct{
			Name:  "test-bucket",
			Count: 42,
		},
	}
	m := jsonModel(res, 80, 24)
	raw := jsonRawContent(m)
	if raw == "" {
		t.Fatal("content returned empty for resource with RawStruct")
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("content is not valid JSON: %v\ncontent: %s", err, raw)
	}
	if !strings.Contains(raw, "test-bucket") {
		t.Errorf("content missing Name value 'test-bucket'\ncontent: %s", raw)
	}
	if !strings.Contains(raw, "42") {
		t.Errorf("content missing Count value '42'\ncontent: %s", raw)
	}
	if !strings.Contains(raw, "  ") {
		t.Errorf("content does not use 2-space indentation\ncontent: %s", raw)
	}
}

// TestJSON_RawContent_WithFields verifies the JSON content is valid JSON
// from Fields map when no RawStruct is set.
func TestJSON_RawContent_WithFields(t *testing.T) {
	res := resource.Resource{
		ID:   "i-123abc",
		Name: "my-instance",
		Fields: map[string]string{
			"instance_type": "t3.medium",
			"state":         "running",
		},
	}
	m := jsonModel(res, 80, 24)
	raw := jsonRawContent(m)
	if raw == "" {
		t.Fatal("content returned empty for resource with Fields")
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("content is not valid JSON: %v\ncontent: %s", err, raw)
	}
	if !strings.Contains(raw, "instance_type") {
		t.Errorf("content missing field key 'instance_type'\ncontent: %s", raw)
	}
	if !strings.Contains(raw, "t3.medium") {
		t.Errorf("content missing field value 't3.medium'\ncontent: %s", raw)
	}
}

// TestJSON_ValidJSON_WithRawStruct verifies that direct json.MarshalIndent
// (no fieldpath.ToSafeValue) preserves native JSON types on round-trip.
func TestJSON_ValidJSON_WithRawStruct(t *testing.T) {
	type myStruct struct {
		Region string
		Count  int
		Active bool
	}
	res := resource.Resource{
		ID:   "valid-json-id",
		Name: "valid-json-resource",
		RawStruct: &myStruct{
			Region: "us-east-1",
			Count:  7,
			Active: true,
		},
	}
	m := jsonModel(res, 80, 24)
	raw := jsonRawContent(m)
	if raw == "" {
		t.Fatal("content returned empty for resource with RawStruct")
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("json.Unmarshal(content) failed: %v\ncontent: %s", err, raw)
	}
	if parsed["Region"] != "us-east-1" {
		t.Errorf("parsed[\"Region\"] = %v, want %q", parsed["Region"], "us-east-1")
	}
	if parsed["Count"] != float64(7) {
		t.Errorf("parsed[\"Count\"] = %v, want %v", parsed["Count"], float64(7))
	}
	if parsed["Active"] != true {
		t.Errorf("parsed[\"Active\"] = %v, want %v", parsed["Active"], true)
	}
}

// TestJSON_RawContent_PreservesZeroAndFalseValues verifies that zero/false
// values are NOT omitted — the key asymmetry with YAML's ToSafeValue, which
// drops zero-value fields entirely.
func TestJSON_RawContent_PreservesZeroAndFalseValues(t *testing.T) {
	type zeroStruct struct {
		Count  int
		Active bool
	}

	res := resource.Resource{
		ID:        "zero-json-id",
		RawStruct: zeroStruct{Count: 0, Active: false},
	}
	m := jsonModel(res, 80, 24)
	raw := jsonRawContent(m)
	if raw == "" {
		t.Fatal("content returned empty for resource with zero values")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("json.Unmarshal(content) failed: %v\ncontent: %s", err, raw)
	}
	if parsed["Count"] != float64(0) {
		t.Errorf("parsed[\"Count\"] = %v, want %v", parsed["Count"], float64(0))
	}
	if parsed["Active"] != false {
		t.Errorf("parsed[\"Active\"] = %v, want %v", parsed["Active"], false)
	}
}

// TestJSON_ColorizeStructuralTokens verifies that structural tokens ({, }, [, ])
// in the JSON content are NOT colorized (they pass through unmodified as
// plain text), while string, number, and bool values ARE colored.
func TestJSON_ColorizeStructuralTokens(t *testing.T) {
	type nested struct {
		Tags   []string
		Count  int
		Active bool
		Name   string
	}
	raw := nested{
		Tags:   []string{"prod", "us-east-1"},
		Count:  3,
		Active: true,
		Name:   "my-resource",
	}
	res := resource.Resource{
		ID:        "struct-tokens-test",
		Name:      "struct-tokens-test",
		RawStruct: &raw,
		Fields:    map[string]string{},
	}
	m := jsonModel(res, 120, 40)
	out := strings.Join(m.ContentLines(), "\n")
	plain := stripANSI(out)

	if !strings.Contains(plain, "{") {
		t.Error("JSON content missing '{' structural token")
	}
	if !strings.Contains(plain, "}") {
		t.Error("JSON content missing '}' structural token")
	}
	if !strings.Contains(plain, "[") {
		t.Error("JSON content missing '[' structural token")
	}
	if !strings.Contains(plain, "]") {
		t.Error("JSON content missing ']' structural token")
	}

	if !strings.Contains(plain, "my-resource") {
		t.Error("JSON content missing string value 'my-resource'")
	}
	if !strings.Contains(out, "\x1b[") {
		t.Error("JSON content has no ANSI color codes — syntax coloring missing")
	}
}
