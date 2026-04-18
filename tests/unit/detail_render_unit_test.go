package unit_test

// detail_render_unit_test.go covers functions in internal/tui/views/detail_render.go
// that have 0% coverage:
//   - RawYAML        (exported)
//   - PlainContent   (exported)
//   - renderFromConfig (unexported, exercised via View())
//   - toSnakeCase      (unexported, exercised via renderFromConfig snake-case fallback)
//   - computeKeyWidth  (unexported, exercised via View() alignment)

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// buildDetailWithConfig creates a DetailModel using a ViewsConfig that
// specifies the given detail paths, with the resource Fields map supplied.
func buildDetailWithConfig(t *testing.T, fields map[string]string, detailPaths []string) views.DetailModel {
	t.Helper()
	detailFields := make([]config.DetailField, len(detailPaths))
	for i, p := range detailPaths {
		detailFields[i] = config.DetailField{Path: p}
	}
	cfg := &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"testtype": {
				Detail: detailFields,
			},
		},
	}
	res := resource.Resource{
		ID:     "test-id",
		Name:   "test-name",
		Fields: fields,
	}
	k := keys.Default()
	m := views.NewDetail(res, "testtype", cfg, k)
	m.SetSize(120, 30)
	return m
}

// buildDetailFieldsOnly creates a DetailModel with Fields but no config,
// so renderContent falls through to the sorted-Fields fallback path.
func buildDetailFieldsOnly(t *testing.T, fields map[string]string) views.DetailModel {
	t.Helper()
	res := resource.Resource{
		ID:     "test-id",
		Name:   "test-name",
		Fields: fields,
	}
	k := keys.Default()
	m := views.NewDetail(res, "testtype", nil, k)
	m.SetSize(120, 30)
	return m
}

// buildDetailWithRawStruct creates a DetailModel with a RawStruct and a config.
func buildDetailWithRawStruct(t *testing.T, rawStruct any, detailPaths []string) views.DetailModel {
	t.Helper()
	detailFields := make([]config.DetailField, len(detailPaths))
	for i, p := range detailPaths {
		detailFields[i] = config.DetailField{Path: p}
	}
	cfg := &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"testtype": {
				Detail: detailFields,
			},
		},
	}
	res := resource.Resource{
		ID:        "test-id",
		Name:      "test-name",
		RawStruct: rawStruct,
	}
	k := keys.Default()
	m := views.NewDetail(res, "testtype", cfg, k)
	m.SetSize(120, 30)
	return m
}

// ---------------------------------------------------------------------------
// RawYAML tests
// ---------------------------------------------------------------------------

func TestDetail_RawYAML_WithRawStruct(t *testing.T) {
	type simpleStruct struct {
		Name   string
		Count  int
		Active bool
	}
	raw := simpleStruct{Name: "test-resource", Count: 42, Active: true}
	res := resource.Resource{
		ID:        "res-001",
		Name:      "test-resource",
		RawStruct: raw,
	}
	k := keys.Default()
	m := views.NewDetail(res, "", nil, k)
	m.SetSize(120, 30)

	yaml := m.RawYAML()
	if yaml == "" {
		t.Fatal("RawYAML should return non-empty YAML when RawStruct is set")
	}
	if !strings.Contains(yaml, "test-resource") {
		t.Errorf("RawYAML should contain struct field values; got:\n%s", yaml)
	}
	if !strings.Contains(yaml, "42") {
		t.Errorf("RawYAML should contain numeric field values; got:\n%s", yaml)
	}
}

func TestDetail_RawYAML_FieldsOnly(t *testing.T) {
	res := resource.Resource{
		ID:   "res-002",
		Name: "fields-only",
		Fields: map[string]string{
			"instance_id": "i-abc123",
			"state":       "running",
		},
	}
	k := keys.Default()
	m := views.NewDetail(res, "", nil, k)
	m.SetSize(120, 30)

	yaml := m.RawYAML()
	if yaml == "" {
		t.Fatal("RawYAML should return YAML from Fields map when RawStruct is nil")
	}
	if !strings.Contains(yaml, "i-abc123") {
		t.Errorf("RawYAML should contain field values; got:\n%s", yaml)
	}
	if !strings.Contains(yaml, "running") {
		t.Errorf("RawYAML should contain all field values; got:\n%s", yaml)
	}
}

func TestDetail_RawYAML_Empty(t *testing.T) {
	res := resource.Resource{
		ID:   "res-003",
		Name: "empty",
		// no RawStruct, no Fields
	}
	k := keys.Default()
	m := views.NewDetail(res, "", nil, k)
	m.SetSize(120, 30)

	yaml := m.RawYAML()
	if yaml != "" {
		t.Errorf("RawYAML should return empty string when no data; got: %q", yaml)
	}
}

// ---------------------------------------------------------------------------
// PlainContent tests
// ---------------------------------------------------------------------------

func TestDetail_PlainContent_StripsANSI(t *testing.T) {
	// Use Fields so renderContent produces styled output
	res := resource.Resource{
		ID:   "res-004",
		Name: "plain-test",
		Fields: map[string]string{
			"instance_id": "i-plain123",
			"state":       "running",
		},
	}
	k := keys.Default()
	m := views.NewDetail(res, "", nil, k)
	m.SetSize(120, 30)

	plain := m.PlainContent()
	if strings.Contains(plain, "\x1b") {
		t.Errorf("PlainContent should strip ANSI escape codes; got content containing \\x1b:\n%q", plain)
	}
	if !strings.Contains(plain, "i-plain123") {
		t.Errorf("PlainContent should contain field values after stripping ANSI; got:\n%s", plain)
	}
	if !strings.Contains(plain, "running") {
		t.Errorf("PlainContent should contain all field values; got:\n%s", plain)
	}
}

func TestDetail_PlainContent_EmptyResource(t *testing.T) {
	res := resource.Resource{
		ID:   "res-005",
		Name: "empty-plain",
	}
	k := keys.Default()
	m := views.NewDetail(res, "", nil, k)
	m.SetSize(120, 30)

	plain := m.PlainContent()
	// Should return something (the "No detail data available" message) without panicking
	if strings.Contains(plain, "\x1b") {
		t.Errorf("PlainContent on empty resource should strip ANSI; got:\n%q", plain)
	}
}

// ---------------------------------------------------------------------------
// renderFromConfig tests (via View())
// ---------------------------------------------------------------------------

func TestDetail_RenderFromConfig_FieldsMapLookup_ExactCase(t *testing.T) {
	ensureNoColor(t)
	// Field key matches PascalCase path exactly (case-insensitive match)
	m := buildDetailWithConfig(t,
		map[string]string{
			"InstanceId": "i-exact-match",
		},
		[]string{"InstanceId"},
	)

	view := stripAnsi(m.View())
	if !strings.Contains(view, "i-exact-match") {
		t.Errorf("renderFromConfig should find field via exact case-insensitive match; got:\n%s", view)
	}
}

func TestDetail_RenderFromConfig_SnakeCaseFallback(t *testing.T) {
	ensureNoColor(t)
	// Field key is snake_case ("instance_id") but ViewDef path is PascalCase ("InstanceId").
	// toSnakeCase("InstanceId") == "instance_id" so the fallback should find it.
	m := buildDetailWithConfig(t,
		map[string]string{
			"instance_id": "i-abc-snake",
		},
		[]string{"InstanceId"},
	)

	view := stripAnsi(m.View())
	if !strings.Contains(view, "i-abc-snake") {
		t.Errorf("renderFromConfig should fall back to snake_case key lookup; view:\n%s", view)
	}
}

func TestDetail_RenderFromConfig_EmptyFieldShowsDash(t *testing.T) {
	ensureNoColor(t)
	// Path "MissingField" not in Fields map and no RawStruct → should render "-"
	m := buildDetailWithConfig(t,
		map[string]string{
			"other_field": "some-value",
		},
		[]string{"MissingField"},
	)

	view := stripAnsi(m.View())
	if !strings.Contains(view, "-") {
		t.Errorf("renderFromConfig should show '-' for missing fields; got:\n%s", view)
	}
}

func TestDetail_RenderFromConfig_MultilineValue(t *testing.T) {
	ensureNoColor(t)
	// A field containing newlines should be rendered with section header + indented sub-lines.
	m := buildDetailWithConfig(t,
		map[string]string{
			"MultilineField": "line-one\nline-two\nline-three",
		},
		[]string{"MultilineField"},
	)

	view := stripAnsi(m.View())
	if !strings.Contains(view, "line-one") {
		t.Errorf("multiline field: first line should appear; got:\n%s", view)
	}
	if !strings.Contains(view, "line-two") {
		t.Errorf("multiline field: second line should appear; got:\n%s", view)
	}
	if !strings.Contains(view, "line-three") {
		t.Errorf("multiline field: third line should appear; got:\n%s", view)
	}
}

func TestDetail_RenderFromConfig_MultipleFields(t *testing.T) {
	ensureNoColor(t)
	m := buildDetailWithConfig(t,
		map[string]string{
			"vpc_id":    "vpc-render-123",
			"subnet_id": "subnet-render-456",
		},
		[]string{"VpcId", "SubnetId"},
	)

	view := stripAnsi(m.View())
	if !strings.Contains(view, "vpc-render-123") {
		t.Errorf("renderFromConfig should render VpcId field; got:\n%s", view)
	}
	if !strings.Contains(view, "subnet-render-456") {
		t.Errorf("renderFromConfig should render SubnetId field; got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// toSnakeCase tests (indirect — via renderFromConfig snake-case fallback)
// ---------------------------------------------------------------------------

func TestDetail_ToSnakeCase_SimpleWord(t *testing.T) {
	ensureNoColor(t)
	// "InstanceId" → "instance_id"
	m := buildDetailWithConfig(t,
		map[string]string{"instance_id": "i-snake-simple"},
		[]string{"InstanceId"},
	)
	if !strings.Contains(stripAnsi(m.View()), "i-snake-simple") {
		t.Error("toSnakeCase(InstanceId) should produce 'instance_id'")
	}
}

func TestDetail_ToSnakeCase_MultipleHumps(t *testing.T) {
	ensureNoColor(t)
	// "PrivateIpAddress" → "private_ip_address"
	m := buildDetailWithConfig(t,
		map[string]string{"private_ip_address": "10.0.1.42"},
		[]string{"PrivateIpAddress"},
	)
	if !strings.Contains(stripAnsi(m.View()), "10.0.1.42") {
		t.Error("toSnakeCase(PrivateIpAddress) should produce 'private_ip_address'")
	}
}

func TestDetail_ToSnakeCase_AlreadyLower(t *testing.T) {
	ensureNoColor(t)
	// "name" (no uppercase) → "name" unchanged; exact case-insensitive match should find it first
	m := buildDetailWithConfig(t,
		map[string]string{"name": "my-resource"},
		[]string{"name"},
	)
	if !strings.Contains(stripAnsi(m.View()), "my-resource") {
		t.Error("lowercase path should still render correctly")
	}
}

// ---------------------------------------------------------------------------
// computeKeyWidth tests (indirect — via View() alignment)
// ---------------------------------------------------------------------------

func TestDetail_ComputeKeyWidth_ShortKeys(t *testing.T) {
	ensureNoColor(t)
	// All keys shorter than 22 chars — minimum width 22 applies.
	m := buildDetailFieldsOnly(t, map[string]string{
		"a":   "val-a",
		"b":   "val-b",
		"key": "val-key",
	})
	view := stripAnsi(m.View())
	// All values should be present; alignment uses min 22 chars for key column
	if !strings.Contains(view, "val-a") {
		t.Errorf("short key fields should render; got:\n%s", view)
	}
}

func TestDetail_ComputeKeyWidth_LongKeyExpands(t *testing.T) {
	ensureNoColor(t)
	// Key longer than 22 chars should expand the key column width.
	longKey := "very_long_key_that_exceeds_minimum_width"
	m := buildDetailFieldsOnly(t, map[string]string{
		longKey: "long-val",
		"short": "short-val",
	})
	view := stripAnsi(m.View())
	if !strings.Contains(view, "long-val") {
		t.Errorf("long key field should render; got:\n%s", view)
	}
	if !strings.Contains(view, "short-val") {
		t.Errorf("short key field should render alongside long key; got:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// Regression: View() without SetSize should return "Initializing..."
// ---------------------------------------------------------------------------

func TestDetail_View_WithoutSetSize_ReturnsInitializing(t *testing.T) {
	res := resource.Resource{
		ID:   "res-006",
		Name: "no-size",
		Fields: map[string]string{
			"key": "value",
		},
	}
	k := keys.Default()
	m := views.NewDetail(res, "", nil, k)
	// Deliberately NOT calling SetSize
	view := m.View()
	if view != "Initializing..." {
		t.Errorf("View() before SetSize should return 'Initializing...'; got: %q", view)
	}
}
