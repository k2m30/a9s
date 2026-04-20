package unit_test

// views_detail_render_test.go covers uncovered branches in detail_render.go:
//   - renderContent config path (computeKeyWidthFromFields + renderFromConfig)
//   - renderFromConfig: key-form field found, key-form field missing (shows "-")
//   - renderFromConfig: path-form exact match, snake_case fallback, RawStruct fallback
//   - renderFromConfig: multiline value renders as section header + indented sub-lines
//   - renderFromConfig: key-form with custom label
//
// All tests use PlainContent() before SetSize() to exercise renderContent's
// viewConfig branch — because SetSize triggers buildFieldList() which populates
// fieldList and bypasses renderFromConfig entirely. PlainContent() calls
// renderContent() directly, so calling it on a freshly-constructed model
// (fieldList == nil) exercises the config path.

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

// buildDetailConfigOnly constructs a DetailModel with a viewConfig that has
// the given DetailFields, WITHOUT calling SetSize, so fieldList stays nil and
// PlainContent() exercises renderFromConfig.
func buildDetailConfigOnly(t *testing.T, res resource.Resource, detailFields []config.DetailField) views.DetailModel {
	t.Helper()
	cfg := &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"rcfg": {
				Detail: detailFields,
			},
		},
	}
	k := keys.Default()
	// Deliberately NOT calling SetSize — fieldList stays nil.
	return views.NewDetail(res, "rcfg", cfg, k)
}

// ---------------------------------------------------------------------------
// renderFromConfig — key-form fields
// ---------------------------------------------------------------------------

// TestDetailRender_renderFromConfig_KeyForm_FoundValue verifies that a key-form
// DetailField whose Key is present in Fields is rendered with its actual value.
func TestDetailRender_renderFromConfig_KeyForm_FoundValue(t *testing.T) {
	res := resource.Resource{
		Fields: map[string]string{
			"logging_enabled": "true",
			"bucket_name":     "my-test-bucket",
		},
	}
	m := buildDetailConfigOnly(t, res, []config.DetailField{
		{Key: "logging_enabled"},
		{Key: "bucket_name"},
	})

	plain := m.PlainContent()
	if !strings.Contains(plain, "true") {
		t.Errorf("renderFromConfig should render key-form field value 'true'; got:\n%s", plain)
	}
	if !strings.Contains(plain, "my-test-bucket") {
		t.Errorf("renderFromConfig should render key-form field value 'my-test-bucket'; got:\n%s", plain)
	}
}

// TestDetailRender_renderFromConfig_KeyForm_MissingValue verifies that a key-form
// DetailField whose Key is absent from Fields renders "-" as the placeholder.
func TestDetailRender_renderFromConfig_KeyForm_MissingValue(t *testing.T) {
	res := resource.Resource{
		Fields: map[string]string{
			"present": "yes",
		},
	}
	m := buildDetailConfigOnly(t, res, []config.DetailField{
		{Key: "absent_field"},
	})

	plain := m.PlainContent()
	if !strings.Contains(plain, "-") {
		t.Errorf("renderFromConfig should render '-' for missing key-form field; got:\n%s", plain)
	}
}

// TestDetailRender_renderFromConfig_KeyForm_CustomLabel verifies that when a
// key-form DetailField has a Label, the label appears in the output instead of
// the raw key name.
func TestDetailRender_renderFromConfig_KeyForm_CustomLabel(t *testing.T) {
	res := resource.Resource{
		Fields: map[string]string{
			"is_logging": "enabled",
		},
	}
	m := buildDetailConfigOnly(t, res, []config.DetailField{
		{Key: "is_logging", Label: "Logging Status"},
	})

	plain := m.PlainContent()
	if !strings.Contains(plain, "Logging Status") {
		t.Errorf("renderFromConfig should use custom label; got:\n%s", plain)
	}
	if !strings.Contains(plain, "enabled") {
		t.Errorf("renderFromConfig should render field value under custom label; got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// renderFromConfig — path-form fields (Fields map lookups)
// ---------------------------------------------------------------------------

// TestDetailRender_renderFromConfig_PathForm_ExactCaseInsensitive verifies that a
// path-form DetailField is found via case-insensitive exact match in Fields.
func TestDetailRender_renderFromConfig_PathForm_ExactCaseInsensitive(t *testing.T) {
	res := resource.Resource{
		Fields: map[string]string{
			"TrailARN": "arn:aws:cloudtrail:us-east-1:111111111111:trail/my-trail",
		},
	}
	m := buildDetailConfigOnly(t, res, []config.DetailField{
		{Path: "trailarn"}, // lowercase path, field key is "TrailARN" — case-insensitive match
	})

	plain := m.PlainContent()
	if !strings.Contains(plain, "my-trail") {
		t.Errorf("renderFromConfig path-form should match field case-insensitively; got:\n%s", plain)
	}
}

// TestDetailRender_renderFromConfig_PathForm_SnakeCaseFallback verifies that a
// path-form DetailField using PascalCase path falls back to snake_case key lookup.
func TestDetailRender_renderFromConfig_PathForm_SnakeCaseFallback(t *testing.T) {
	res := resource.Resource{
		Fields: map[string]string{
			"home_region": "us-west-2",
		},
	}
	m := buildDetailConfigOnly(t, res, []config.DetailField{
		{Path: "HomeRegion"}, // "HomeRegion" → snake: "home_region"
	})

	plain := m.PlainContent()
	if !strings.Contains(plain, "us-west-2") {
		t.Errorf("renderFromConfig path-form should fall back to snake_case 'home_region'; got:\n%s", plain)
	}
}

// TestDetailRender_renderFromConfig_PathForm_NoMatchShowsDash verifies that a
// path-form DetailField absent from both Fields and RawStruct renders "-".
func TestDetailRender_renderFromConfig_PathForm_NoMatchShowsDash(t *testing.T) {
	res := resource.Resource{
		Fields: map[string]string{
			"other_key": "other_val",
		},
	}
	m := buildDetailConfigOnly(t, res, []config.DetailField{
		{Path: "NonExistentField"},
	})

	plain := m.PlainContent()
	if !strings.Contains(plain, "-") {
		t.Errorf("renderFromConfig path-form with no match should render '-'; got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// renderFromConfig — path-form with RawStruct fallback
// ---------------------------------------------------------------------------

// TestDetailRender_renderFromConfig_RawStructFallback verifies that when a
// path-form field is absent from Fields but present in RawStruct, the value
// is extracted via fieldpath reflection.
func TestDetailRender_renderFromConfig_RawStructFallback(t *testing.T) {
	type trailRow struct {
		TrailARN string
		Name     string
	}
	raw := trailRow{
		TrailARN: "arn:aws:cloudtrail:us-east-1:222222222222:trail/rawstruct-trail",
		Name:     "rawstruct-trail",
	}
	res := resource.Resource{
		// No Fields — force RawStruct path in renderFromConfig
		RawStruct: raw,
	}
	m := buildDetailConfigOnly(t, res, []config.DetailField{
		{Path: "TrailARN"},
	})

	plain := m.PlainContent()
	if !strings.Contains(plain, "rawstruct-trail") {
		t.Errorf("renderFromConfig should extract TrailARN from RawStruct; got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// renderFromConfig — multiline values
// ---------------------------------------------------------------------------

// TestDetailRender_renderFromConfig_MultilineValue verifies that a field value
// containing newlines is rendered as a section header followed by indented sub-lines.
func TestDetailRender_renderFromConfig_MultilineValue(t *testing.T) {
	res := resource.Resource{
		Fields: map[string]string{
			"event_selectors": "ReadWrite: All\nIncludeManagementEvents: true\nDataResources: []",
		},
	}
	m := buildDetailConfigOnly(t, res, []config.DetailField{
		{Key: "event_selectors", Label: "Event Selectors"},
	})

	plain := m.PlainContent()
	if !strings.Contains(plain, "Event Selectors") {
		t.Errorf("renderFromConfig multiline: section header should show label; got:\n%s", plain)
	}
	if !strings.Contains(plain, "ReadWrite: All") {
		t.Errorf("renderFromConfig multiline: first line should appear; got:\n%s", plain)
	}
	if !strings.Contains(plain, "IncludeManagementEvents: true") {
		t.Errorf("renderFromConfig multiline: second line should appear; got:\n%s", plain)
	}
	if !strings.Contains(plain, "DataResources: []") {
		t.Errorf("renderFromConfig multiline: third line should appear; got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// computeKeyWidthFromFields — label width drives column alignment
// ---------------------------------------------------------------------------

// TestDetailRender_computeKeyWidthFromFields_LongLabel verifies that a DetailField
// with a Label longer than 22 characters expands the key column width,
// causing shorter labels to be padded so values align.
func TestDetailRender_computeKeyWidthFromFields_LongLabel(t *testing.T) {
	res := resource.Resource{
		Fields: map[string]string{
			"a_long_label_field": "alpha-value",
			"short_field":        "beta-value",
		},
	}
	m := buildDetailConfigOnly(t, res, []config.DetailField{
		{Key: "a_long_label_field", Label: "A Very Long Label That Exceeds The Minimum"},
		{Key: "short_field", Label: "Short"},
	})

	plain := m.PlainContent()
	if !strings.Contains(plain, "alpha-value") {
		t.Errorf("long-label field value should appear; got:\n%s", plain)
	}
	if !strings.Contains(plain, "beta-value") {
		t.Errorf("short-label field value should appear; got:\n%s", plain)
	}
}

// TestDetailRender_computeKeyWidthFromFields_MinimumWidth verifies that labels
// shorter than the 22-char minimum still produce a key column of at least 22 chars,
// so both field values are rendered and present in output.
func TestDetailRender_computeKeyWidthFromFields_MinimumWidth(t *testing.T) {
	res := resource.Resource{
		Fields: map[string]string{
			"a": "val-a",
			"b": "val-b",
		},
	}
	m := buildDetailConfigOnly(t, res, []config.DetailField{
		{Key: "a"},
		{Key: "b"},
	})

	plain := m.PlainContent()
	if !strings.Contains(plain, "val-a") {
		t.Errorf("minimum-width alignment: 'val-a' should appear; got:\n%s", plain)
	}
	if !strings.Contains(plain, "val-b") {
		t.Errorf("minimum-width alignment: 'val-b' should appear; got:\n%s", plain)
	}
}

// ---------------------------------------------------------------------------
// renderContent — config path returns empty, falls through to Fields fallback
// ---------------------------------------------------------------------------

// TestDetailRender_renderContent_ConfigPathEmpty_FallsThrough verifies that when
// renderFromConfig returns no lines (e.g., viewConfig exists but Detail is
// empty after GetViewDef lookup), renderContent falls through to the sorted
// Fields fallback.
func TestDetailRender_renderContent_ConfigPathEmpty_FallsThrough(t *testing.T) {
	// Use a resource type that has NO Detail fields in config so renderFromConfig
	// returns nil, causing the fallback path to render from Fields.
	res := resource.Resource{
		Fields: map[string]string{
			"instance_id": "i-fallback",
		},
	}
	cfg := &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"empty_detail_type": {
				// No Detail fields: renderFromConfig returns nil
				// renderContent falls through to sorted Fields render.
			},
		},
	}
	k := keys.Default()
	m := views.NewDetail(res, "empty_detail_type", cfg, k)
	// Do NOT call SetSize — fieldList stays nil.

	plain := m.PlainContent()
	if !strings.Contains(plain, "i-fallback") {
		t.Errorf("renderContent should fall through to Fields map when config has no Detail; got:\n%s", plain)
	}
}
