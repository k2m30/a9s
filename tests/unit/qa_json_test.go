package unit

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/layout"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ════════════════════════════════════════════════════════════════════════════
// QA: JSON View — mirrors qa_yaml_test.go patterns
// ════════════════════════════════════════════════════════════════════════════

// testJSONStruct is a simple struct for RawStruct tests.
type testJSONStruct struct {
	Name  string
	Count int
}

// jsonModel creates a JSONModel from a resource, sets size, and returns it.
func jsonModel(res resource.Resource, w, h int) views.JSONModel {
	k := keys.Default()
	m := views.NewJSON(res, "", k)
	m.SetSize(w, h)
	return m
}

// TestJSON_RawContent_WithRawStruct verifies RawContent returns valid 2-space-indented JSON
// with the struct fields when RawStruct is set.
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
	raw := m.RawContent()
	if raw == "" {
		t.Fatal("RawContent() returned empty for resource with RawStruct")
	}
	// Must be valid JSON
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("RawContent() is not valid JSON: %v\ncontent: %s", err, raw)
	}
	// Must contain field values
	if !strings.Contains(raw, "test-bucket") {
		t.Errorf("RawContent() missing Name value 'test-bucket'\ncontent: %s", raw)
	}
	if !strings.Contains(raw, "42") {
		t.Errorf("RawContent() missing Count value '42'\ncontent: %s", raw)
	}
	// Must use 2-space indent
	if !strings.Contains(raw, "  ") {
		t.Errorf("RawContent() does not use 2-space indentation\ncontent: %s", raw)
	}
}

// TestJSON_RawContent_WithFields verifies RawContent returns valid JSON from Fields map
// when no RawStruct is set.
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
	raw := m.RawContent()
	if raw == "" {
		t.Fatal("RawContent() returned empty for resource with Fields")
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("RawContent() is not valid JSON: %v\ncontent: %s", err, raw)
	}
	if !strings.Contains(raw, "instance_type") {
		t.Errorf("RawContent() missing field key 'instance_type'\ncontent: %s", raw)
	}
	if !strings.Contains(raw, "t3.medium") {
		t.Errorf("RawContent() missing field value 't3.medium'\ncontent: %s", raw)
	}
}

// TestJSON_RawContent_Empty verifies RawContent returns "" when no RawStruct and empty Fields.
func TestJSON_RawContent_Empty(t *testing.T) {
	res := resource.Resource{
		ID:   "empty-id",
		Name: "empty-resource",
	}
	m := jsonModel(res, 80, 24)
	raw := m.RawContent()
	if raw != "" {
		t.Errorf("RawContent() = %q, want empty string for resource with no data", raw)
	}
}

// TestJSON_FrameTitle_WithName verifies FrameTitle returns "<Name> json".
func TestJSON_FrameTitle_WithName(t *testing.T) {
	res := resource.Resource{
		ID:   "i-unused",
		Name: "my-bucket",
	}
	m := jsonModel(res, 80, 24)
	title := m.FrameTitle()
	if title != "my-bucket json" {
		t.Errorf("FrameTitle() = %q, want %q", title, "my-bucket json")
	}
}

// TestJSON_FrameTitle_WithoutName verifies FrameTitle falls back to ID when Name is empty.
func TestJSON_FrameTitle_WithoutName(t *testing.T) {
	res := resource.Resource{
		ID: "i-123",
	}
	m := jsonModel(res, 80, 24)
	title := m.FrameTitle()
	if title != "i-123 json" {
		t.Errorf("FrameTitle() = %q, want %q", title, "i-123 json")
	}
}

// TestJSON_CopyContent verifies CopyContent returns JSON text and correct label.
func TestJSON_CopyContent(t *testing.T) {
	res := resource.Resource{
		ID:   "sg-abc",
		Name: "my-sg",
		Fields: map[string]string{
			"vpc_id": "vpc-123",
		},
	}
	m := jsonModel(res, 80, 24)
	content, label := m.CopyContent()
	if content == "" {
		t.Error("CopyContent() returned empty content for resource with Fields")
	}
	if label != "Copied JSON to clipboard" {
		t.Errorf("CopyContent() label = %q, want %q", label, "Copied JSON to clipboard")
	}
	// content must be valid JSON
	var out any
	if err := json.Unmarshal([]byte(content), &out); err != nil {
		t.Errorf("CopyContent() returned invalid JSON: %v\ncontent: %s", err, content)
	}
}

// TestJSON_CopyContent_Empty verifies CopyContent returns "", "" for empty resource.
func TestJSON_CopyContent_Empty(t *testing.T) {
	res := resource.Resource{
		ID:   "empty-id",
		Name: "empty",
	}
	m := jsonModel(res, 80, 24)
	content, label := m.CopyContent()
	if content != "" || label != "" {
		t.Errorf("CopyContent() = (%q, %q), want (\"\", \"\") for empty resource", content, label)
	}
}

// TestJSON_GetHelpContext verifies GetHelpContext returns HelpFromJSON.
func TestJSON_GetHelpContext(t *testing.T) {
	res := resource.Resource{ID: "help-id", Name: "help-resource"}
	m := jsonModel(res, 80, 24)
	got := m.GetHelpContext()
	if got != views.HelpFromJSON {
		t.Errorf("GetHelpContext() = %v, want HelpFromJSON (%v)", got, views.HelpFromJSON)
	}
}

// TestJSON_BottomHints verifies the JSON view exposes Wrap and Copy hints.
func TestJSON_BottomHints(t *testing.T) {
	res := resource.Resource{ID: "hints-id", Name: "hints-resource"}
	m := jsonModel(res, 80, 24)
	hints := m.BottomHints()

	want := []layout.KeyHint{
		{Key: "w", Desc: "Wrap"},
		{Key: "c", Desc: "Copy"},
	}
	wantMap := map[string]string{}
	for _, h := range want {
		wantMap[h.Key] = h.Desc
	}
	for _, wh := range want {
		found := false
		for _, gh := range hints {
			if gh.Key == wh.Key && gh.Desc == wh.Desc {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("BottomHints() missing {Key:%q, Desc:%q}; got %v", wh.Key, wh.Desc, hints)
		}
	}
}

// TestJSON_View_NotReady verifies View() returns "Initializing..." before SetSize is called.
func TestJSON_View_NotReady(t *testing.T) {
	res := resource.Resource{ID: "nr-id", Name: "not-ready"}
	k := keys.Default()
	m := views.NewJSON(res, "", k)
	// Do NOT call SetSize — model is not ready
	out := m.View()
	if out != "Initializing..." {
		t.Errorf("View() before SetSize = %q, want %q", out, "Initializing...")
	}
}

// TestJSON_ResourceID verifies ResourceID returns the resource's ID field.
func TestJSON_ResourceID(t *testing.T) {
	res := resource.Resource{
		ID:   "i-resourceid-test",
		Name: "resourceid-resource",
		Fields: map[string]string{
			"state": "running",
		},
	}
	m := jsonModel(res, 80, 24)
	got := m.ResourceID()
	if got != "i-resourceid-test" {
		t.Errorf("ResourceID() = %q, want %q", got, "i-resourceid-test")
	}
}

// TestJSON_ValidJSON_WithRawStruct verifies RawContent produces valid JSON when RawStruct is set.
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
	raw := m.RawContent()
	if raw == "" {
		t.Fatal("RawContent() returned empty for resource with RawStruct")
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("json.Unmarshal(RawContent()) failed: %v\ncontent: %s", err, raw)
	}
	// Direct json.MarshalIndent preserves native JSON types (no fieldpath.ToSafeValue).
	if parsed["Region"] != "us-east-1" {
		t.Errorf("parsed[\"Region\"] = %v, want %q", parsed["Region"], "us-east-1")
	}
	// json.Unmarshal decodes numbers as float64
	if parsed["Count"] != float64(7) {
		t.Errorf("parsed[\"Count\"] = %v, want %v", parsed["Count"], float64(7))
	}
	if parsed["Active"] != true {
		t.Errorf("parsed[\"Active\"] = %v, want %v", parsed["Active"], true)
	}
}

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
	raw := m.RawContent()
	if raw == "" {
		t.Fatal("RawContent() returned empty for resource with zero values")
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		t.Fatalf("json.Unmarshal(RawContent()) failed: %v\ncontent: %s", err, raw)
	}
	if parsed["Count"] != float64(0) {
		t.Errorf("parsed[\"Count\"] = %v, want %v", parsed["Count"], float64(0))
	}
	if parsed["Active"] != false {
		t.Errorf("parsed[\"Active\"] = %v, want %v", parsed["Active"], false)
	}
}
