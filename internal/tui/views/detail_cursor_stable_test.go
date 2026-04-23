package views

// detail_cursor_stable_test.go — reveal test for the P3 cursor-jump bug.
//
// When Wave 2 enrichment arrives after the operator has opened a detail view
// and moved the cursor off row 0, SetEnrichmentFinding prepends an Attention
// section to fieldList but leaves fieldCursor untouched. The cursor then
// points to a different logical row (often inside the Attention section or
// on its spacer). Enter/copy/etc. act on the wrong value.
//
// Invariant being pinned: SetEnrichmentFinding must preserve the operator's
// cursor position on the SAME logical field — i.e. the FieldItem whose Path
// + Key identity the cursor referred to before the rebuild.

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
)

// viewportSelectedLine returns the 0-based line index of the rendered
// content that reflects m.fieldCursor. It maps fieldCursor to a content
// line by consulting the currently rendered viewport content (ANSI-stripped).
// The cursor highlight is painted at that line; what matters for this test
// is that the cursor row's TEXT matches the item at m.fieldCursor — proving
// the render used the new cursor, not the stale one.
func viewportSelectedLineText(m *DetailModel) string {
	content := ansi.Strip(m.viewport.View())
	lines := strings.Split(content, "\n")
	if m.fieldCursor < 0 || m.fieldCursor >= len(lines) {
		return ""
	}
	return lines[m.fieldCursor]
}

func TestDetail_SetEnrichmentFinding_PreservesCursorIdentity(t *testing.T) {
	// Build a detail model with several path-form fields.
	res := resource.Resource{
		ID:   "db-1",
		Name: "prod-db",
		Fields: map[string]string{
			"db_identifier":     "prod-db",
			"engine":            "postgres",
			"engine_version":    "15.4",
			"status":            "no automated backups",
			"instance_class":    "db.r6g.large",
			"endpoint":          "prod-db.aws.com:5432",
			"multi_az":          "false",
			"publicly_accessible": "true",
		},
	}

	m := NewDetail(res, "dbi", nil, keys.Default())
	m.SetSize(120, 40)
	// Prime the viewport: build fieldList without any enrichment finding.
	m.refreshViewportContent()

	// Operator moves cursor onto a meaningful field (say index 5, whatever
	// logical item that is after the default layout).
	if len(m.fieldList) < 6 {
		t.Fatalf("precondition: fieldList too short (len=%d)", len(m.fieldList))
	}
	m.fieldCursor = 5
	before := m.fieldList[m.fieldCursor]

	// Wave 2 enrichment lands after the operator moved cursor.
	finding := resource.EnrichmentFinding{
		Severity: "!",
		Summary:  "encryption key unavailable",
		Rows: []resource.FindingRow{
			{Label: "KMS Key", Value: "arn:aws:kms:…"},
			{Label: "Reason", Value: "key is pending deletion"},
		},
	}
	m.SetEnrichmentFinding(&finding)

	// After injection, the cursor must still point at the SAME logical item.
	if m.fieldCursor < 0 || m.fieldCursor >= len(m.fieldList) {
		t.Fatalf("fieldCursor out of range after SetEnrichmentFinding: %d / %d",
			m.fieldCursor, len(m.fieldList))
	}
	after := m.fieldList[m.fieldCursor]
	if after.Key != before.Key || after.Path != before.Path {
		t.Errorf("cursor jumped to different field after Attention injection:\n  before: key=%q path=%q\n  after:  key=%q path=%q",
			before.Key, before.Path, after.Key, after.Path)
	}
}

// TestDetail_SetEnrichmentFinding_RenderedSelectionFollowsCursor asserts
// that the RENDERED viewport content reflects the relocated cursor position,
// not the stale pre-rebuild index. Reviewer P3 second-pass finding: even
// with the cursor field corrected on the model, refreshViewportContent
// runs BEFORE relocation in the naive fix, so the paint highlights the
// old row for at least one frame. This test catches that.
func TestDetail_SetEnrichmentFinding_RenderedSelectionFollowsCursor(t *testing.T) {
	res := resource.Resource{
		ID:   "db-3",
		Name: "prod-db-3",
		Fields: map[string]string{
			"db_identifier":       "prod-db-3",
			"engine":              "postgres",
			"engine_version":      "15.4",
			"status":              "no automated backups",
			"instance_class":      "db.r6g.large",
			"endpoint":            "prod-db-3.aws.com:5432",
			"multi_az":            "false",
			"publicly_accessible": "true",
		},
	}
	m := NewDetail(res, "dbi", nil, keys.Default())
	m.SetSize(120, 40)
	m.refreshViewportContent()

	if len(m.fieldList) < 6 {
		t.Fatalf("precondition: fieldList too short (len=%d)", len(m.fieldList))
	}
	// Pick a cursor position that unambiguously identifies a row via Key.
	cursorIdx := 5
	m.fieldCursor = cursorIdx
	wantKey := m.fieldList[cursorIdx].Key
	if wantKey == "" {
		t.Fatalf("precondition: expected non-empty Key at fieldCursor=%d", cursorIdx)
	}

	// Enrichment fires.
	finding := resource.EnrichmentFinding{
		Severity: "!",
		Summary:  "encryption key unavailable",
		Rows:     []resource.FindingRow{{Label: "KMS Key", Value: "arn:aws:kms:…"}},
	}
	m.SetEnrichmentFinding(&finding)

	// Rendered viewport content at the CURRENT fieldCursor index must carry
	// the same Key as before. If renderContent ran before relocation, the
	// line at this index would reflect a different row (Attention entry).
	gotLine := viewportSelectedLineText(&m)
	if !strings.Contains(gotLine, wantKey) {
		t.Errorf("rendered selection does not follow relocated cursor:\n  want line containing key %q\n  got  line: %q",
			wantKey, gotLine)
	}
}

// TestDetail_ClearEnrichmentFinding_PreservesCursorIdentity is the symmetric
// case: when a finding is REMOVED (e.g. re-enrichment reports a previously
// Broken row as now Healthy), the Attention section shrinks. Cursor should
// stay on the same logical field, not shift backwards onto a resource field
// it wasn't on.
func TestDetail_ClearEnrichmentFinding_PreservesCursorIdentity(t *testing.T) {
	res := resource.Resource{
		ID:   "db-2",
		Name: "prod-db-2",
		Fields: map[string]string{
			"db_identifier":  "prod-db-2",
			"engine":         "postgres",
			"instance_class": "db.r6g.large",
			"endpoint":       "prod-db-2.aws.com:5432",
		},
	}
	finding := resource.EnrichmentFinding{
		Severity: "!",
		Summary:  "pending maintenance",
		Rows:     []resource.FindingRow{{Label: "Action", Value: "os-upgrade"}},
	}

	m := NewDetail(res, "dbi", nil, keys.Default())
	m.SetSize(120, 40)
	m.SetEnrichmentFinding(&finding)
	m.refreshViewportContent()

	if len(m.fieldList) < 8 {
		t.Fatalf("precondition: fieldList too short (len=%d)", len(m.fieldList))
	}
	// Cursor ON a resource field that sits after the Attention section.
	m.fieldCursor = 7
	before := m.fieldList[m.fieldCursor]

	// Finding clears (e.g. next enrichment cycle reports healthy).
	m.SetEnrichmentFinding(nil)

	if m.fieldCursor < 0 || m.fieldCursor >= len(m.fieldList) {
		t.Fatalf("fieldCursor out of range after clear: %d / %d",
			m.fieldCursor, len(m.fieldList))
	}
	after := m.fieldList[m.fieldCursor]
	if after.Key != before.Key || after.Path != before.Path {
		t.Errorf("cursor jumped on finding clear:\n  before: key=%q path=%q\n  after:  key=%q path=%q",
			before.Key, before.Path, after.Key, after.Path)
	}
}
