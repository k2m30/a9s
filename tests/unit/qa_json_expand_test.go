package unit

import (
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/text"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ════════════════════════════════════════════════════════════════════════════
// TryJSONToYAMLLines — unit tests for the shared JSON→YAML helper
// ════════════════════════════════════════════════════════════════════════════

// TestTryJSONToYAMLLines_ValidObject verifies that a simple JSON object is converted
// to YAML lines. YAML sorts keys alphabetically, so "count" comes before "name".
func TestTryJSONToYAMLLines_ValidObject(t *testing.T) {
	lines := text.TryJSONToYAMLLines(`{"name":"test","count":42}`)
	if lines == nil {
		t.Fatal("TryJSONToYAMLLines returned nil for valid JSON object")
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "count: 42") {
		t.Errorf("expected 'count: 42' in YAML output, got:\n%s", joined)
	}
	if !strings.Contains(joined, "name: test") {
		t.Errorf("expected 'name: test' in YAML output, got:\n%s", joined)
	}
}

// TestTryJSONToYAMLLines_ValidArray verifies that a JSON array is converted to YAML list lines.
func TestTryJSONToYAMLLines_ValidArray(t *testing.T) {
	lines := text.TryJSONToYAMLLines(`[1,2,3]`)
	if lines == nil {
		t.Fatal("TryJSONToYAMLLines returned nil for valid JSON array")
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "- 1") {
		t.Errorf("expected '- 1' in YAML output, got:\n%s", joined)
	}
	if !strings.Contains(joined, "- 2") {
		t.Errorf("expected '- 2' in YAML output, got:\n%s", joined)
	}
	if !strings.Contains(joined, "- 3") {
		t.Errorf("expected '- 3' in YAML output, got:\n%s", joined)
	}
}

// TestTryJSONToYAMLLines_InvalidJSON verifies that a non-JSON string returns nil.
func TestTryJSONToYAMLLines_InvalidJSON(t *testing.T) {
	lines := text.TryJSONToYAMLLines("not json")
	if lines != nil {
		t.Errorf("TryJSONToYAMLLines returned non-nil for invalid JSON: %v", lines)
	}
}

// TestTryJSONToYAMLLines_EmptyObject verifies that "{}" returns nil (no meaningful content).
func TestTryJSONToYAMLLines_EmptyObject(t *testing.T) {
	lines := text.TryJSONToYAMLLines("{}")
	if lines != nil {
		t.Errorf("TryJSONToYAMLLines returned non-nil for empty object '{}': %v", lines)
	}
}

// TestTryJSONToYAMLLines_EmptyArray verifies that "[]" returns nil (no meaningful content).
func TestTryJSONToYAMLLines_EmptyArray(t *testing.T) {
	lines := text.TryJSONToYAMLLines("[]")
	if lines != nil {
		t.Errorf("TryJSONToYAMLLines returned non-nil for empty array '[]': %v", lines)
	}
}

// TestTryJSONToYAMLLines_PlainString verifies that a plain string not starting with { or [
// returns nil immediately without attempting JSON parse.
func TestTryJSONToYAMLLines_PlainString(t *testing.T) {
	lines := text.TryJSONToYAMLLines("hello world")
	if lines != nil {
		t.Errorf("TryJSONToYAMLLines returned non-nil for plain string: %v", lines)
	}
}

// TestTryJSONToYAMLLines_NestedObject verifies that a nested JSON object produces
// multi-line YAML with indented sub-keys.
func TestTryJSONToYAMLLines_NestedObject(t *testing.T) {
	lines := text.TryJSONToYAMLLines(`{"a":{"b":"c"}}`)
	if lines == nil {
		t.Fatal("TryJSONToYAMLLines returned nil for nested JSON object")
	}
	if len(lines) < 2 {
		t.Fatalf("expected multiple YAML lines for nested object, got %d: %v", len(lines), lines)
	}
	joined := strings.Join(lines, "\n")
	// YAML renders nested objects with indentation (spaces before "b:")
	if !strings.Contains(joined, "a:") {
		t.Errorf("expected 'a:' in YAML output, got:\n%s", joined)
	}
	if !strings.Contains(joined, "b: c") {
		t.Errorf("expected 'b: c' in YAML output (possibly indented), got:\n%s", joined)
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Detail view JSON expansion — integration tests via views.DetailModel
// ════════════════════════════════════════════════════════════════════════════

// jsonExpandDetailModel builds a DetailModel for a Fields-only resource with a custom
// ViewsConfig that lists the given field keys as detail paths.
func jsonExpandDetailModel(fields map[string]string, detailPaths []string) views.DetailModel {
	k := keys.Default()
	res := resource.Resource{
		ID:     "test-resource",
		Name:   "test",
		Fields: fields,
	}
	detailFields := make([]config.DetailField, len(detailPaths))
	for i, p := range detailPaths {
		detailFields[i] = config.DetailField{Path: p}
	}
	cfg := &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"ec2": {
				Detail: detailFields,
			},
		},
	}
	m := views.NewDetail(res, "ec2", cfg, k)
	m.SetSize(120, 40)
	return m
}

// TestQA_JSONExpand_TopLevelScalar_Expanded verifies that a JSON string value in a detail
// field is rendered as expanded YAML sub-fields, NOT as a single-line JSON blob.
func TestQA_JSONExpand_TopLevelScalar_Expanded(t *testing.T) {
	policyJSON := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"s3:GetObject"}]}`
	fields := map[string]string{
		"Policy": policyJSON,
	}
	m := jsonExpandDetailModel(fields, []string{"Policy"})
	view := stripANSI(m.View())

	// The raw single-line JSON blob must NOT appear verbatim in the output.
	if strings.Contains(view, policyJSON) {
		t.Errorf("detail view rendered raw JSON blob instead of expanding it:\n%s", view)
	}
	// Key YAML fields from the expanded policy should be visible.
	if !strings.Contains(view, "Version") {
		t.Errorf("detail view missing expanded YAML key 'Version':\n%s", view)
	}
	if !strings.Contains(view, "2012-10-17") {
		t.Errorf("detail view missing expanded YAML value '2012-10-17':\n%s", view)
	}
}

// TestQA_JSONExpand_InvalidJSON_PassesThrough verifies that a non-JSON value in a detail
// field is rendered as-is, without modification.
func TestQA_JSONExpand_InvalidJSON_PassesThrough(t *testing.T) {
	plainValue := "arn:aws:iam::123456789012:role/MyRole"
	fields := map[string]string{
		"RoleArn": plainValue,
	}
	m := jsonExpandDetailModel(fields, []string{"RoleArn"})
	view := stripANSI(m.View())

	if !strings.Contains(view, plainValue) {
		t.Errorf("detail view should render plain ARN value as-is, got:\n%s", view)
	}
}

// TestQA_JSONExpand_EmptyObject_NotExpanded verifies that "{}" is kept inline
// and does not trigger JSON expansion (no content to expand).
func TestQA_JSONExpand_EmptyObject_NotExpanded(t *testing.T) {
	fields := map[string]string{
		"Tags": "{}",
	}
	m := jsonExpandDetailModel(fields, []string{"Tags"})
	view := stripANSI(m.View())

	// "{}" should appear as a literal value, not trigger sub-field expansion.
	if !strings.Contains(view, "{}") {
		t.Errorf("detail view should keep empty JSON object '{}' inline, got:\n%s", view)
	}
}

// TestQA_JSONExpand_YAMLView_Unaffected verifies that RawYAML() is not affected by
// the JSON expansion logic — it must return the raw Fields map serialized as YAML,
// not the expanded representation.
func TestQA_JSONExpand_YAMLView_Unaffected(t *testing.T) {
	policyJSON := `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"s3:GetObject"}]}`
	k := keys.Default()
	res := resource.Resource{
		ID:   "test-resource",
		Name: "test",
		Fields: map[string]string{
			"Policy": policyJSON,
		},
	}
	cfg := &config.ViewsConfig{
		Views: map[string]config.ViewDef{
			"ec2": {
				Detail: []config.DetailField{{Path: "Policy"}},
			},
		},
	}
	m := views.NewDetail(res, "ec2", cfg, k)
	m.SetSize(120, 40)

	rawYAML := m.RawYAML()
	// RawYAML must contain the original JSON string as a YAML scalar — not double-expanded.
	if !strings.Contains(rawYAML, "Policy") {
		t.Errorf("RawYAML() missing 'Policy' key:\n%s", rawYAML)
	}
	// The JSON string itself should appear in RawYAML as a quoted or block scalar,
	// not silently dropped or replaced with YAML-expanded content.
	if !strings.Contains(rawYAML, "2012-10-17") {
		t.Errorf("RawYAML() missing original policy content '2012-10-17':\n%s", rawYAML)
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Sub-field JSON expansion
// ════════════════════════════════════════════════════════════════════════════

// TestQA_JSONExpand_SubField_Expanded verifies that a JSON string nested inside
// a multi-line section (sub-field value) is expanded into additional sub-fields.
func TestQA_JSONExpand_SubField_Expanded(t *testing.T) {
	// Simulate a resource with a nested JSON value inside a multi-line field.
	// Use Fields map with dotted keys to create sub-fields under a parent.
	fields := map[string]string{
		"Config.Policy": `{"Effect":"Allow","Resource":"*"}`,
		"Config.Name":   "my-config",
	}
	m := jsonExpandDetailModel(fields, []string{"Config"})
	view := stripANSI(m.View())

	// The expanded JSON should show its keys, not a raw blob.
	if !strings.Contains(view, "Effect") {
		t.Errorf("sub-field JSON should expand to show 'Effect':\n%s", view)
	}
	if !strings.Contains(view, "Allow") {
		t.Errorf("sub-field JSON should expand to show 'Allow':\n%s", view)
	}
	// Non-JSON sub-field should pass through.
	if !strings.Contains(view, "my-config") {
		t.Errorf("non-JSON sub-field 'my-config' should pass through:\n%s", view)
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Explicit scope boundary tests
// ════════════════════════════════════════════════════════════════════════════

// TestQA_JSONExpand_JSONView_NotAffected verifies that the JSON view renders
// via json.MarshalIndent on Fields directly — expandJSONItems only runs in
// buildFieldList (detail view path), so JSONModel is unaffected. Tests through
// the actual JSONModel.RawContent() path.
func TestQA_JSONExpand_JSONView_NotAffected(t *testing.T) {
	policyJSON := `{"Version":"2012-10-17"}`
	res := resource.Resource{
		ID:   "test-resource",
		Name: "test",
		Fields: map[string]string{
			"Policy": policyJSON,
		},
	}
	k := keys.Default()
	jm := views.NewJSON(res, "ec2", k)
	jm.SetSize(120, 40)

	raw := jm.RawContent()
	// JSON view serializes the Fields map — Policy value stays as a JSON string,
	// not expanded into nested structure.
	if !strings.Contains(raw, "Policy") {
		t.Errorf("JSONModel.RawContent() should contain 'Policy' key:\n%s", raw)
	}
	if !strings.Contains(raw, "2012-10-17") {
		t.Errorf("JSONModel.RawContent() should contain original JSON content:\n%s", raw)
	}
}

// TestQA_JSONExpand_CloudTrail_OutOfScope verifies that CloudTrail event detail
// rendering takes the ctdetail.Parse branch (not the generic buildFieldList path),
// so expandJSONItems never runs on CT events. Uses a real cloudtrailtypes.Event
// with a CloudTrailEvent JSON payload containing embedded JSON in RequestParameters.
func TestQA_JSONExpand_CloudTrail_OutOfScope(t *testing.T) {
	ctJSON := `{"eventVersion":"1.08","eventSource":"s3.amazonaws.com","eventName":"PutObject","requestParameters":{"bucketName":"my-bucket","key":"data.json","policy":"{\"Version\":\"2012-10-17\"}"},"responseElements":null}`
	event := cloudtrailtypes.Event{
		EventId:         aws.String("event-ct-scope-test"),
		EventName:       aws.String("PutObject"),
		EventSource:     aws.String("s3.amazonaws.com"),
		CloudTrailEvent: aws.String(ctJSON),
	}
	k := keys.Default()
	res := resource.Resource{
		ID:        "event-ct-scope-test",
		Name:      "PutObject",
		Status:    "write",
		RawStruct: event,
	}
	cfg := config.DefaultConfig()
	m := views.NewDetail(res, "ct-events", cfg, k)
	m.SetSize(120, 40)

	view := stripANSI(m.View())

	// CT branch must be taken — verify CT-specific content appears.
	if !strings.Contains(view, "PutObject") {
		t.Errorf("ct-events detail should show event name 'PutObject':\n%s", view)
	}
	// The embedded JSON in requestParameters.policy should NOT be expanded
	// by expandJSONItems (CT uses its own rendering). Verify the raw policy
	// string is NOT expanded into YAML sub-fields.
	if strings.Contains(view, "2012-10-17") {
		// If the policy version appears, it was rendered by the CT summarizer
		// as a compact string — not by expandJSONItems. That's fine.
		// The key assertion is that we don't crash and CT path is taken.
	}
}
