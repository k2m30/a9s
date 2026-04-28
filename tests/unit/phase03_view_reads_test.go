// phase03_view_reads_test.go — TDD red-light tests for PR-03a-views.
//
// Strategy: every resource has Findings populated AND Status/Issues set to a
// different decoy value. Pre-fix views read the legacy field → decoy visible.
// Post-fix views read Findings/AttentionDetails → canonical visible.
//
// Each test MUST fail until PR-03a-views is implemented.
package unit_test

import (
	"os"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// ---------------------------------------------------------------------------
// Helpers local to this file
// ---------------------------------------------------------------------------

// minimalTypeDef builds a ResourceTypeDef with a name column and a "status"
// key column. Color reads Fields["status"] so healthy=running, broken=stopped.
// LifecycleKey is left empty (defaults to "state" at view-read time).
func minimalTypeDef(shortName string) resource.ResourceTypeDef {
	return resource.ResourceTypeDef{
		Name:      shortName,
		ShortName: shortName,
		Columns: []resource.Column{
			{Key: "name", Title: "Name", Width: 24},
			{Key: "status", Title: "Status", Width: 20},
		},
		Color: func(r resource.Resource) resource.Color {
			switch r.Fields["status"] {
			case "stopped", "failed":
				return resource.ColorBroken
			case "running", "available":
				return resource.ColorHealthy
			}
			return resource.ColorHealthy
		},
	}
}

// minimalTypeDefWithLifecycleKey builds a typeDef whose status column key
// ("status") intentionally does NOT match LifecycleKey ("state").
// Pre-fix: extractCellValue for Key:"status" finds Fields["status"] = ""
// (not set) → cell is blank. Post-fix: the view reads Fields[LifecycleKey]
// = Fields["state"] = "running" and shows it in the status cell.
func minimalTypeDefWithLifecycleKey() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{
		Name:      "ec2-lifecycle-test",
		ShortName: "ec2-lifecycle-test",
		Columns: []resource.Column{
			{Key: "name", Title: "Name", Width: 24},
			{Key: "status", Title: "Status", Width: 20},
		},
		LifecycleKey: "state", // explicit: post-fix fallback reads Fields["state"]
		Color: func(r resource.Resource) resource.Color {
			return resource.ColorHealthy
		},
	}
}

// loadList builds a ResourceListModel pre-populated with resources.
func loadList(td resource.ResourceTypeDef, rs []resource.Resource) views.ResourceListModel {
	k := keys.Default()
	m := views.NewResourceList(td, nil, k)
	m.SetSize(120, 30)
	m, _ = m.Update(messages.ResourcesLoadedMsg{Resources: rs, ResourceType: td.ShortName})
	return m
}

// renderList returns the stripANSI-cleaned View() output of the list.
func renderList(m views.ResourceListModel) string {
	return stripAnsi(m.View())
}

// ---------------------------------------------------------------------------
// Test 1 — list Status column reads Findings[0].Phrase, not r.Status
// ---------------------------------------------------------------------------

// TestViews_ListStatusColumn_ReadsFindingsPhrase verifies that when a resource
// carries Findings[0].Phrase = "<canonical>" and Status = "<DECOY>", the list
// view shows the canonical phrase, not the decoy.
//
// Pre-fix: view reads Fields["status"] = DECOY → DECOY is visible.
// Post-fix: view reads Findings[0].Phrase = canonical → canonical is visible.
func TestViews_ListStatusColumn_ReadsFindingsPhrase(t *testing.T) {
	ensureNoColor(t)

	cases := []struct {
		displayName string // human-readable case name
		shortName   string // must NOT match any registered type to keep custom columns
		canonical   string
		decoy       string
		resourceID  string
	}{
		{"ec2", "ec2-findings-test", "pending maintenance", "DECOY-ec2", "i-001"},
		{"s3", "s3-findings-test", "public access enabled", "DECOY-s3", "my-bucket"},
		{"sg", "sg-findings-test", "unrestricted ingress", "DECOY-sg", "sg-001"},
		{"role", "role-findings-test", "admin policy attached", "DECOY-role", "MyRole"},
		{"ng", "ng-findings-test", "node group degraded", "DECOY-ng", "ng-001"},
		{"kms", "kms-findings-test", "key rotation disabled", "DECOY-kms", "key-001"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.displayName, func(t *testing.T) {
			td := minimalTypeDef(tc.shortName)
			r := resource.Resource{
				ID:   tc.resourceID,
				Name: tc.resourceID,
				// Decoy goes into the status field — pre-fix view reads this.
				Status: tc.decoy,
				Fields: map[string]string{
					"status": tc.decoy,
				},
				// Canonical phrase in Findings — post-fix view reads this.
				Findings: []domain.Finding{
					{
						Code:     domain.FindingCode(tc.displayName + ".test"),
						Phrase:   tc.canonical,
						Severity: domain.SevBroken,
						Source:   "wave1",
					},
				},
			}

			m := loadList(td, []resource.Resource{r})
			out := renderList(m)

			if !strings.Contains(out, tc.canonical) {
				t.Errorf("[%s] rendered list does not contain canonical phrase %q; got:\n%s",
					tc.displayName, tc.canonical, out)
			}
			if strings.Contains(out, tc.decoy) {
				t.Errorf("[%s] rendered list contains DECOY phrase %q, which should not appear post-fix; got:\n%s",
					tc.displayName, tc.decoy, out)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test 2 — Status column falls back to Fields[LifecycleKey] when Findings is nil
// ---------------------------------------------------------------------------

// TestViews_ListStatusColumn_FallsBackToLifecycleKey verifies that when Findings
// is nil and Fields[LifecycleKey] ("state") has a value, that value is shown.
//
// Pre-fix: view reads Fields["status"] — which is NOT set, so the cell is
// blank or shows a different value. Post-fix: view reads Fields[LifecycleKey]
// = Fields["state"] = "running".
func TestViews_ListStatusColumn_FallsBackToLifecycleKey(t *testing.T) {
	ensureNoColor(t)

	td := minimalTypeDefWithLifecycleKey()
	r := resource.Resource{
		ID:   "i-lifecycle",
		Name: "my-instance",
		// Fields["status"] is intentionally absent — pre-fix the cell is blank.
		// Fields["state"] = "running" — post-fix the fallback returns this.
		Fields: map[string]string{
			"state": "running",
		},
		Findings: nil,
	}

	m := loadList(td, []resource.Resource{r})
	out := renderList(m)

	if !strings.Contains(out, "running") {
		t.Errorf("list view should display lifecycle fallback value \"running\" when Findings is nil; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Test 3 — row color reads Findings[0].Severity, not r.Status lifecycle
// ---------------------------------------------------------------------------

// TestViews_ListColor_ReadsFindingsSeverity verifies that a resource with
// Findings[0].Severity == SevBroken renders as broken-colored (not healthy)
// even though its Status (and Fields["status"]) says "running".
//
// Pre-fix: view calls td.ResolveColor(r) which reads Fields["status"]="running"
// → ColorHealthy. All non-cursor rows with the same status have identical ANSI
// color prefix → healthy-A and broken-A rows look the same → assertion fails.
// Post-fix: view uses Findings[0].Severity = SevBroken → ColorBroken → rows differ.
//
// Design note: cursor row (position 0) renders with RowSelected style, masking
// color differences. We place a "padding" resource at cursor position 0 and put
// the two resources we care about at positions 1 (healthy) and 2 (broken) so
// both render without cursor overlay and their base styles are comparable.
func TestViews_ListColor_ReadsFindingsSeverity(t *testing.T) {
	// Ensure NO_COLOR is absent so lipgloss emits ANSI escape sequences.
	old, wasSet := os.LookupEnv("NO_COLOR")
	os.Unsetenv("NO_COLOR") //nolint:errcheck
	styles.Reinit()
	t.Cleanup(func() {
		if wasSet {
			os.Setenv("NO_COLOR", old) //nolint:errcheck
		} else {
			os.Unsetenv("NO_COLOR") //nolint:errcheck
		}
		styles.Reinit()
	})

	td := minimalTypeDef("ec2-col-test") // unique name avoids registered-type config override

	// cursor-row resource at position 0: RowSelected style masks color, so we
	// do not use it in comparisons. It just holds the cursor.
	padding := resource.Resource{
		ID:     "i-cursor",
		Name:   "cursor-holder",
		Status: "running",
		Fields: map[string]string{"status": "running"},
	}
	// healthy-A at position 1: Findings=nil → post-fix ColorHealthy.
	healthyA := resource.Resource{
		ID:     "i-healthy-A",
		Name:   "healthy-A",
		Status: "running",
		Fields: map[string]string{"status": "running"},
		Findings: nil,
	}
	// broken-A at position 2: Findings[0].Severity=SevBroken but status="running"
	// → pre-fix ColorHealthy (same as healthyA), post-fix ColorBroken (different).
	brokenA := resource.Resource{
		ID:     "i-broken-A",
		Name:   "broken-A",
		Status: "running", // decoy: Color func reads this as Healthy pre-fix
		Fields: map[string]string{"status": "running"},
		Findings: []domain.Finding{
			{
				Code:     "ec2.broken",
				Phrase:   "instance impaired",
				Severity: domain.SevBroken,
				Source:   "wave1",
			},
		},
	}

	m := loadList(td, []resource.Resource{padding, healthyA, brokenA})

	rawOut := m.View()
	lines := strings.Split(rawOut, "\n")

	var healthyLine, brokenLine string
	for _, l := range lines {
		plain := stripAnsi(l)
		if strings.Contains(plain, "healthy-A") {
			healthyLine = l
		}
		if strings.Contains(plain, "broken-A") {
			brokenLine = l
		}
	}

	if healthyLine == "" {
		t.Fatalf("could not find healthy-A row in rendered list:\n%s", stripAnsi(rawOut))
	}
	if brokenLine == "" {
		t.Fatalf("could not find broken-A row in rendered list:\n%s", stripAnsi(rawOut))
	}

	// Post-fix: broken-A must have a different ANSI color prefix than healthy-A.
	// Pre-fix: both rows are ColorHealthy (status="running") → identical prefix → FAIL.
	healthyANSIPrefix := extractANSIPrefix(healthyLine)
	brokenANSIPrefix := extractANSIPrefix(brokenLine)

	if healthyANSIPrefix == brokenANSIPrefix {
		t.Errorf("broken-A (Findings.Severity=SevBroken) must render with a different color than healthy-A (Findings=nil); "+
			"both rows have identical ANSI prefix — view is not reading Findings.Severity for row color.\n"+
			"healthy-A raw line: %q\n"+
			"broken-A  raw line: %q",
			healthyLine, brokenLine)
	}
}

// extractANSIPrefix returns the leading ANSI escape sequences from a string,
// stopping at the first non-escape character. Used to compare row color codes.
func extractANSIPrefix(s string) string {
	var sb strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// Consume the escape sequence.
			j := i + 2
			for j < len(s) && (s[j] < 'A' || s[j] > 'z') {
				j++
			}
			if j < len(s) {
				j++ // include terminator
			}
			sb.WriteString(s[i:j])
			i = j
		} else {
			break
		}
	}
	return sb.String()
}

// ---------------------------------------------------------------------------
// Test 4 — detail Attention section reads r.AttentionDetails[code].Rows
// ---------------------------------------------------------------------------

// TestViews_DetailAttention_ReadsAttentionDetails verifies that the detail view
// Attention section renders rows from r.AttentionDetails, NOT from
// m.enrichmentFinding (which is left nil/unset).
//
// Pre-fix: injectAttentionSection reads m.enrichmentFinding (nil) → no Rows.
// Post-fix: injectAttentionSection reads r.Findings[i] + r.AttentionDetails[code].Rows.
func TestViews_DetailAttention_ReadsAttentionDetails(t *testing.T) {
	ensureNoColor(t)

	code := domain.FindingCode("ec2.X")
	r := resource.Resource{
		ID:   "i-maint",
		Name: "maint-instance",
		Fields: map[string]string{
			"InstanceId": "i-maint",
		},
		Findings: []domain.Finding{
			{
				Code:     code,
				Phrase:   "pending maintenance",
				Severity: domain.SevBroken,
				Source:   "wave2:ec2",
			},
		},
		AttentionDetails: map[domain.FindingCode]domain.AttentionDetail{
			code: {
				Rows: []domain.DetailRow{
					{Label: "Action", Value: "reboot"},
					{Label: "Earliest", Value: "2026-05-01"},
				},
			},
		},
	}

	k := keys.Default()
	m := views.NewDetail(r, "ec2", nil, k)
	m.SetSize(200, 100)
	// Deliberately do NOT call m.SetEnrichmentFinding — pre-fix path uses only that.

	out := m.PlainContent()

	if !strings.Contains(out, "Action") || !strings.Contains(out, "reboot") {
		t.Errorf("detail Attention section must show AttentionDetail row \"Action: reboot\"; got:\n%s", out)
	}
	if !strings.Contains(out, "Earliest") || !strings.Contains(out, "2026-05-01") {
		t.Errorf("detail Attention section must show AttentionDetail row \"Earliest: 2026-05-01\"; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Test 5 — detail Attention prefers Findings phrase over Issues
// ---------------------------------------------------------------------------

// TestViews_DetailAttention_PrefersFindingsPhraseOverIssues verifies that when
// both r.Findings[0].Phrase and r.Issues are set, the detail view shows the
// Findings phrase, not the Issues phrase.
//
// Pre-fix: injectAttentionSection reads m.res.Issues → shows "legacy decoy".
// Post-fix: reads r.Findings[0].Phrase → shows "canonical phrase".
func TestViews_DetailAttention_PrefersFindingsPhraseOverIssues(t *testing.T) {
	ensureNoColor(t)

	r := resource.Resource{
		ID:   "i-prefer",
		Name: "prefer-instance",
		Fields: map[string]string{
			"status": "running",
		},
		// Legacy field — pre-fix detail view reads this.
		Issues: []string{"legacy decoy"},
		// New field — post-fix detail view reads this.
		Findings: []domain.Finding{
			{
				Code:     "ec2.prefer",
				Phrase:   "canonical phrase",
				Severity: domain.SevBroken,
				Source:   "wave1",
			},
		},
	}

	k := keys.Default()
	m := views.NewDetail(r, "ec2", nil, k)
	m.SetSize(200, 100)

	out := m.PlainContent()

	if !strings.Contains(out, "canonical phrase") {
		t.Errorf("detail Attention section must show Findings phrase \"canonical phrase\"; got:\n%s", out)
	}
	if strings.Contains(out, "legacy decoy") {
		t.Errorf("detail Attention section must NOT show Issues phrase \"legacy decoy\" when Findings is populated; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Test 6 — IssueCount counts Findings where Severity.IsIssue(), not status color
// ---------------------------------------------------------------------------

// TestViews_IssueCount_ReadsFindingsBySeverity verifies that IssueCount() counts
// resources whose Findings contain an IsIssue()-severity finding, not resources
// whose legacy Status/Color is broken.
//
// Setup: 3 resources, all with Fields["status"]="running" (Color = Healthy).
//   A: Findings[0].Severity = SevBroken → should count
//   B: Findings[0].Severity = SevWarn   → should count
//   C: Findings = nil                   → should NOT count
//
// Pre-fix: IssueCount reads td.ResolveColor(r).IsIssue() → all Healthy → count=0.
// Post-fix: IssueCount reads r.Findings → A+B are issues → count=2.
func TestViews_IssueCount_ReadsFindingsBySeverity(t *testing.T) {
	ensureNoColor(t)

	td := minimalTypeDef("ec2-badge-test")

	resA := resource.Resource{
		ID:     "i-A",
		Name:   "instance-A",
		Status: "running",
		Fields: map[string]string{"status": "running"},
		Issues: nil,
		Findings: []domain.Finding{
			{Code: "ec2.A", Phrase: "impaired", Severity: domain.SevBroken, Source: "wave1"},
		},
	}
	resB := resource.Resource{
		ID:     "i-B",
		Name:   "instance-B",
		Status: "running",
		Fields: map[string]string{"status": "running"},
		Issues: nil,
		Findings: []domain.Finding{
			{Code: "ec2.B", Phrase: "degraded", Severity: domain.SevWarn, Source: "wave1"},
		},
	}
	resC := resource.Resource{
		ID:       "i-C",
		Name:     "instance-C",
		Status:   "running",
		Fields:   map[string]string{"status": "running"},
		Issues:   nil,
		Findings: nil,
	}

	m := loadList(td, []resource.Resource{resA, resB, resC})
	got := m.IssueCount()

	if got != 2 {
		t.Errorf("IssueCount() = %d, want 2 (resources with Findings.Severity.IsIssue()); "+
			"pre-fix value is 0 (no legacy Status issues)", got)
	}
}

// ---------------------------------------------------------------------------
// Test 7 — attention filter (ctrl+z) reads r.Findings for visibility
// ---------------------------------------------------------------------------

// TestViews_AttentionFilter_ReadsFindings verifies that enabling the ctrl+z
// attention filter shows only resources that have at least one Findings entry
// with IsIssue() severity, regardless of their legacy Status / Color.
//
// Setup: 2 resources, both with Fields["status"]="running" (Healthy color):
//   A: Findings[0].Severity = SevBroken → must be visible after enabling filter
//   B: Findings = nil                   → must be hidden after enabling filter
//
// Pre-fix: applyFilter reads td.ResolveColor(r).IsIssue() → both Healthy → both hidden.
// Post-fix: applyFilter reads r.Findings → A visible, B hidden.
func TestViews_AttentionFilter_ReadsFindings(t *testing.T) {
	ensureNoColor(t)

	td := minimalTypeDef("ec2-filter-test")

	resA := resource.Resource{
		ID:     "i-filter-A",
		Name:   "filter-A",
		Status: "running",
		Fields: map[string]string{"status": "running"},
		Findings: []domain.Finding{
			{Code: "ec2.filter.A", Phrase: "impaired", Severity: domain.SevBroken, Source: "wave1"},
		},
	}
	resB := resource.Resource{
		ID:       "i-filter-B",
		Name:     "filter-B",
		Status:   "running",
		Fields:   map[string]string{"status": "running"},
		Findings: nil,
	}

	m := loadList(td, []resource.Resource{resA, resB})

	// Enable the attention filter.
	m.SetEnabled(true)
	m.SetFilter("")

	out := renderList(m)

	if !strings.Contains(out, "filter-A") {
		t.Errorf("attention filter must show resource A (Findings.Severity=SevBroken); got:\n%s", out)
	}
	if strings.Contains(out, "filter-B") {
		t.Errorf("attention filter must hide resource B (Findings=nil, Status=running); got:\n%s", out)
	}
}
