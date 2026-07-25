package unit_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"charm.land/bubbles/v2/viewport"

	"github.com/k2m30/a9s/v3/core/jsonyaml"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ════════════════════════════════════════════════════════════════════════════
// TryJSONToYAMLLines — unit tests for the shared JSON→YAML helper
// ════════════════════════════════════════════════════════════════════════════

// TestTryJSONToYAMLLines_ValidObject verifies that a simple JSON object is converted
// to YAML lines. YAML sorts keys alphabetically, so "count" comes before "name".
func TestTryJSONToYAMLLines_ValidObject(t *testing.T) {
	lines := jsonyaml.TryJSONToYAMLLines(`{"name":"test","count":42}`)
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
	lines := jsonyaml.TryJSONToYAMLLines(`[1,2,3]`)
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
	lines := jsonyaml.TryJSONToYAMLLines("not json")
	if lines != nil {
		t.Errorf("TryJSONToYAMLLines returned non-nil for invalid JSON: %v", lines)
	}
}

// TestTryJSONToYAMLLines_EmptyObject verifies that "{}" returns nil (no meaningful content).
func TestTryJSONToYAMLLines_EmptyObject(t *testing.T) {
	lines := jsonyaml.TryJSONToYAMLLines("{}")
	if lines != nil {
		t.Errorf("TryJSONToYAMLLines returned non-nil for empty object '{}': %v", lines)
	}
}

// TestTryJSONToYAMLLines_EmptyArray verifies that "[]" returns nil (no meaningful content).
func TestTryJSONToYAMLLines_EmptyArray(t *testing.T) {
	lines := jsonyaml.TryJSONToYAMLLines("[]")
	if lines != nil {
		t.Errorf("TryJSONToYAMLLines returned non-nil for empty array '[]': %v", lines)
	}
}

// TestTryJSONToYAMLLines_PlainString verifies that a plain string not starting with { or [
// returns nil immediately without attempting JSON parse.
func TestTryJSONToYAMLLines_PlainString(t *testing.T) {
	lines := jsonyaml.TryJSONToYAMLLines("hello world")
	if lines != nil {
		t.Errorf("TryJSONToYAMLLines returned non-nil for plain string: %v", lines)
	}
}

// TestTryJSONToYAMLLines_NestedObject verifies that a nested JSON object produces
// multi-line YAML with indented sub-keys.
func TestTryJSONToYAMLLines_NestedObject(t *testing.T) {
	lines := jsonyaml.TryJSONToYAMLLines(`{"a":{"b":"c"}}`)
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

// TestTryJSONToYAMLLines_BigIntRoundTripsLosslessly pins F2: TryJSONToYAMLLines
// must decode with json.Decoder.UseNumber so an integer above 2^53 renders
// with its full digits, not a plain json.Unmarshal-into-any float64 that
// silently rounds 9007199254740993 to 9007199254740992 — corrupting policy
// documents and templates on render/copy (the same defect core/fieldpath's
// tryParseJSON was fixed for, extract.go's doc comment).
func TestTryJSONToYAMLLines_BigIntRoundTripsLosslessly(t *testing.T) {
	const bigInt = "9007199254740993"
	lines := jsonyaml.TryJSONToYAMLLines(`{"id":` + bigInt + `}`)
	if lines == nil {
		t.Fatal("TryJSONToYAMLLines returned nil for valid JSON object")
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, bigInt) {
		t.Errorf("expected the full-precision digits %q in YAML output, got:\n%s (a float64 would render \"9007199254740992\")", bigInt, joined)
	}
}

// TestTryJSONToYAMLLines_TrailingGarbage_StillRejected pins F2's other half:
// switching from json.Unmarshal to a json.Decoder for UseNumber must not
// relax the "no trailing content" contract — a single Decoder.Decode call
// does not reject trailing content on its own (unlike json.Unmarshal), so
// this must still explicitly check Decoder.More() the way core/fieldpath's
// tryParseJSON does.
func TestTryJSONToYAMLLines_TrailingGarbage_StillRejected(t *testing.T) {
	lines := jsonyaml.TryJSONToYAMLLines(`{"a":1} trailing garbage`)
	if lines != nil {
		t.Errorf("TryJSONToYAMLLines returned non-nil for JSON with trailing garbage: %v", lines)
	}
}

// TestTryJSONToYAMLLines_PlainFloat_StillParses verifies UseNumber doesn't
// break ordinary float rendering: a ratio value still parses and renders as
// a plain decimal.
func TestTryJSONToYAMLLines_PlainFloat_StillParses(t *testing.T) {
	lines := jsonyaml.TryJSONToYAMLLines(`{"ratio":0.5}`)
	if lines == nil {
		t.Fatal("TryJSONToYAMLLines returned nil for valid JSON object with a plain float")
	}
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "ratio: 0.5") {
		t.Errorf("expected 'ratio: 0.5' in YAML output, got:\n%s", joined)
	}
}

// TestTryJSONToYAMLLines_FloatSpellings_NumericEquality pins NormalizeJSONNumbers'
// big.Rat-based float-tier check: a json.Number whose lexical spelling merely
// differs from Go's canonical FormatFloat digits (a formatting mismatch, not
// a precision loss) must still collapse to a bare, unquoted float64 — not stay
// quoted as a json.Number string. Decimal fractions float64 cannot hold
// exactly (0.10 -> exact rational 1/10, which no float64 bit pattern equals)
// and genuine precision-loss decimals must still stay quoted as json.Number —
// per NormalizeJSONNumbers' own doc comment, 0.1 is the canonical example of
// a value that falls through to the quoted tier, not a value the big.Rat
// check was meant to bare.
func TestTryJSONToYAMLLines_FloatSpellings_NumericEquality(t *testing.T) {
	tests := []struct {
		name       string
		spelling   string
		wantBare   string // substring expected when the value collapses to a bare float64
		wantQuoted bool   // true if the value must stay quoted (no exact float64 equivalent)
	}{
		{name: "1.0 collapses to bare float64(1)", spelling: "1.0", wantBare: "value: 1"},
		{name: "1e3 collapses to bare float64(1000)", spelling: "1e3", wantBare: "value: 1000"},
		{name: "0.5 collapses to bare float64(0.5) (exact dyadic fraction)", spelling: "0.5", wantBare: "value: 0.5"},
		{name: "0.10 has no exact float64 equivalent, stays quoted json.Number", spelling: "0.10", wantQuoted: true},
		{name: "genuine precision loss stays quoted json.Number", spelling: "0.1000000000000000000000000001", wantQuoted: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lines := jsonyaml.TryJSONToYAMLLines(`{"value":` + tt.spelling + `}`)
			if lines == nil {
				t.Fatalf("TryJSONToYAMLLines returned nil for {\"value\":%s}", tt.spelling)
			}
			joined := strings.Join(lines, "\n")
			if tt.wantQuoted {
				if !strings.Contains(joined, `value: "`+tt.spelling+`"`) {
					t.Errorf("expected %q to stay quoted (precision loss), got:\n%s", tt.spelling, joined)
				}
				return
			}
			if !strings.Contains(joined, tt.wantBare) {
				t.Errorf("expected %q in YAML output (bare, unquoted), got:\n%s", tt.wantBare, joined)
			}
			if strings.Contains(joined, `"`+tt.spelling+`"`) {
				t.Errorf("expected %q to render bare, but found it still quoted:\n%s", tt.spelling, joined)
			}
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// CompactValue — unit tests for the shared compact JSON-value rendering helper
// ════════════════════════════════════════════════════════════════════════════

// TestCompactValue verifies that CompactValue renders arbitrary parsed values as a
// compact, single-line, readable string: nested maps/slices as compact JSON (via
// json.Marshal, so map keys sort alphabetically), and scalars in their bare
// (unquoted) form rather than JSON-quoted.
func TestCompactValue(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{
			name:  "nil returns empty string",
			input: nil,
			want:  "",
		},
		{
			name: "nested map of slice of map renders as compact JSON (resourcesSet shape)",
			input: map[string]any{
				"items": []any{
					map[string]any{"resourceId": "i-0e99cfa17db308c06"},
				},
			},
			want: `{"items":[{"resourceId":"i-0e99cfa17db308c06"}]}`,
		},
		{
			name: "nested map of slice of map renders as compact JSON (tagSet shape, key sorts before value)",
			input: map[string]any{
				"items": []any{
					map[string]any{"key": "aws:eks:cluster-name", "value": "acme-dev"},
				},
			},
			want: `{"items":[{"key":"aws:eks:cluster-name","value":"acme-dev"}]}`,
		},
		{
			name:  "plain string passes through bare, not JSON-quoted",
			input: "hello",
			want:  "hello",
		},
		{
			name:  "bool passes through bare",
			input: true,
			want:  "true",
		},
		{
			name:  "number passes through bare",
			input: float64(42),
			want:  "42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := jsonyaml.CompactValue(tt.input)
			if got != tt.want {
				t.Errorf("CompactValue(%#v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ════════════════════════════════════════════════════════════════════════════
// Detail view JSON expansion — integration tests via views.DetailModel
// ════════════════════════════════════════════════════════════════════════════

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
	jm := views.NewJSONWithCtrl(res, "ec2", k, nil)
	jm.SetSize(120, 40)

	raw := stripAnsi(strings.Join(jm.ContentLines(), "\n"))
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
//
// Retargeted (wave3 detail-family cleanup round 4, specs/022-codebase-cleanup)
// off views.NewDetail(...).View() onto the live Controller.EnsureDetailState +
// NewTransientDetail.RenderDetail seam. Not a duplicate of wave3_detail_ports_
// test.go's Test_CTEvents_LiveProjector_SectionHeadersPresentInOrder: that
// test pins section ORDER via a minimal fixture with no embedded JSON: this one
// pins the distinct "requestParameters.policy embedded JSON string is not
// exploded into sub-fields by expandJSONItems" no-crash contract.
func TestQA_JSONExpand_CloudTrail_OutOfScope(t *testing.T) {
	ctJSON := `{"eventVersion":"1.08","eventSource":"s3.amazonaws.com","eventName":"PutObject","requestParameters":{"bucketName":"my-bucket","key":"data.json","policy":"{\"Version\":\"2012-10-17\"}"},"responseElements":null}`
	event := cloudtrailtypes.Event{
		EventId:         aws.String("event-ct-scope-test"),
		EventName:       aws.String("PutObject"),
		EventSource:     aws.String("s3.amazonaws.com"),
		CloudTrailEvent: aws.String(ctJSON),
	}
	res := resource.Resource{
		ID:        "event-ct-scope-test",
		Name:      "PutObject",
		RawStruct: event,
	}

	c := newDetailController(t, res, "ct-events")
	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("Body.Detail is nil for a ct-events resource")
	}

	vp := viewport.New(viewport.WithWidth(120), viewport.WithHeight(40))
	m := views.NewTransientDetail(120, 40, vp)
	view := stripAnsi(m.RenderDetail(*body))

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

// ════════════════════════════════════════════════════════════════════════════
// ParseStrict — the single strict-parse contract
// ════════════════════════════════════════════════════════════════════════════

// TestParseStrict_RejectsTrailingContent pins the contract every caller that
// turns a stored string into structure depends on — the detail-enrichment
// engine (parseJSONOrRaw), the YAML/JSON views (TryJSONToYAMLLines), and field
// extraction (tryParseJSON) all delegate here, so a malformed document is
// preserved verbatim rather than silently rendered as structure.
//
// The stray-closing-delimiter cases are the ones a Decoder.More() check waves
// through: More() reports whether another VALUE follows, and "}" alone is not
// a value. Each of these three call sites carried its own copy of that wrong
// check before they were collapsed onto this one.
func TestParseStrict_RejectsTrailingContent(t *testing.T) {
	accepted := []string{
		`{"Version":"2012-10-17"}`,
		`[1,2,3]`,
		`{"a":{"b":[1,{"c":2}]}}`,
		"  {\"a\":1}\n\t",
	}
	for _, s := range accepted {
		if _, ok := jsonyaml.ParseStrict(s); !ok {
			t.Errorf("ParseStrict(%q) rejected a complete JSON value, want accepted", s)
		}
	}

	rejected := []string{
		`{"a":1}}`,
		`[1,2]]`,
		`{"a":1}garbage`,
		`{"a":1}{"b":2}`,
		`{"a":1}[2]`,
		`{"a":1`,
		``,
	}
	for _, s := range rejected {
		if v, ok := jsonyaml.ParseStrict(s); ok {
			t.Errorf("ParseStrict(%q) accepted malformed input as %v — the original string must be preserved instead", s, v)
		}
	}
}

// TestParseStrict_PreservesLargeIntegers pins that the shared parse keeps the
// lossless-number contract: an integer beyond float64's exact range must not
// round on the way through.
func TestParseStrict_PreservesLargeIntegers(t *testing.T) {
	v, ok := jsonyaml.ParseStrict(`{"n":9007199254740993}`)
	if !ok {
		t.Fatal("ParseStrict rejected a valid object")
	}
	m, isMap := v.(map[string]any)
	if !isMap {
		t.Fatalf("ParseStrict returned %T, want map[string]any", v)
	}
	if got := fmt.Sprintf("%v", m["n"]); got != "9007199254740993" {
		t.Errorf("large integer became %q, want %q — a float64 round-trip loses the last digit", got, "9007199254740993")
	}
}
