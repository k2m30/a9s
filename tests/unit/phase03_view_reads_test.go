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
// Test 3 — row color delegates to td.ResolveColor(r), not Findings.Severity
// ---------------------------------------------------------------------------

// TestViews_ListColor_DelegatesToTypeColor verifies that row color is always
// determined by td.ResolveColor(r) (i.e. the per-type Color func), regardless
// of what Findings[0].Severity says.
//
// Setup: td.Color returns ColorHealthy for Fields["status"]="running".
//   - healthyRow: Findings=nil, status="running" → td.Color → ColorHealthy
//   - findingsBrokenRow: Findings[0].Severity=SevBroken, status="running"
//     → td.Color still returns ColorHealthy (status field wins over Findings)
//
// Post-fix: both non-cursor rows share the same healthy ANSI prefix because
// td.Color is authoritative and both have status="running" → PASS.
// Pre-fix: view reads Findings.Severity for color → findingsBrokenRow gets
// broken color → ANSI prefix differs from healthyRow → FAIL.
//
// Design note: cursor row (position 0) renders with RowSelected style, masking
// color differences. We place a "padding" resource at cursor position 0 and put
// the two resources we care about at positions 1 (healthy) and 2 (findings-broken)
// so both render without cursor overlay and their base styles are comparable.
func TestViews_ListColor_DelegatesToTypeColor(t *testing.T) {
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

	// td.Color returns ColorHealthy for status="running" — any resource with
	// Fields["status"]="running" is green regardless of Findings.
	td := minimalTypeDef("ec2-delegate-color-test")

	// cursor-row at position 0: RowSelected style masks color, do not compare.
	padding := resource.Resource{
		ID:     "i-cursor",
		Name:   "cursor-holder",
		Status: "running",
		Fields: map[string]string{"status": "running"},
	}
	// healthyRow at position 1: Findings=nil, td.Color → ColorHealthy.
	healthyRow := resource.Resource{
		ID:       "i-healthy",
		Name:     "healthy-row",
		Status:   "running",
		Fields:   map[string]string{"status": "running"},
		Findings: nil,
	}
	// findingsBrokenRow at position 2: Findings[0].Severity=SevBroken but
	// td.Color reads Fields["status"]="running" → ColorHealthy.
	// Pre-fix: view reads Findings.Severity → broken color → different from healthyRow.
	// Post-fix: view reads td.Color → healthy color → same as healthyRow.
	findingsBrokenRow := resource.Resource{
		ID:     "i-findings-broken",
		Name:   "findings-broken-row",
		Status: "running",
		Fields: map[string]string{"status": "running"},
		Findings: []domain.Finding{
			{
				Code:     "ec2.impaired",
				Phrase:   "instance impaired",
				Severity: domain.SevBroken,
				Source:   "wave1",
			},
		},
	}

	m := loadList(td, []resource.Resource{padding, healthyRow, findingsBrokenRow})

	rawOut := m.View()
	lines := strings.Split(rawOut, "\n")

	var healthyLine, brokenFindingsLine string
	for _, l := range lines {
		plain := stripAnsi(l)
		if strings.Contains(plain, "healthy-row") {
			healthyLine = l
		}
		if strings.Contains(plain, "findings-broken-row") {
			brokenFindingsLine = l
		}
	}

	if healthyLine == "" {
		t.Fatalf("could not find healthy-row in rendered list:\n%s", stripAnsi(rawOut))
	}
	if brokenFindingsLine == "" {
		t.Fatalf("could not find findings-broken-row in rendered list:\n%s", stripAnsi(rawOut))
	}

	// Post-fix: both rows must share the same healthy ANSI prefix because
	// td.Color is authoritative (status="running" → ColorHealthy for both).
	// Pre-fix: findingsBrokenRow has broken ANSI prefix → differs → FAIL.
	healthyPrefix := extractANSIPrefix(healthyLine)
	findingsBrokenPrefix := extractANSIPrefix(brokenFindingsLine)

	if healthyPrefix != findingsBrokenPrefix {
		t.Errorf("TestViews_ListColor_DelegatesToTypeColor: findingsBrokenRow must render with the SAME color as healthyRow "+
			"because td.Color is authoritative and both have status=\"running\"; "+
			"ANSI prefixes differ — view is reading Findings.Severity instead of td.ResolveColor(r).\n"+
			"healthy-row       raw line: %q\n"+
			"findings-broken-row raw line: %q",
			healthyLine, brokenFindingsLine)
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

// ---------------------------------------------------------------------------
// Test 8 — SetEnrichmentFinding late-update re-derives and shows in detail
// ---------------------------------------------------------------------------

// TestViews_DetailEnrichmentLateUpdatePicksUpFindings verifies that calling
// SetEnrichmentFinding AFTER the detail view is first rendered causes a
// re-derive so the new enrichment finding appears in the Attention section.
//
// Setup: resource has Status="impaired" (real wave1 Finding, so Findings is
// non-empty). Detail is opened and rendered initially (no enrichment finding).
// Then SetEnrichmentFinding is called with a wave2 finding. The second render
// must include "pending maintenance" and "reboot" in the Attention section.
//
// Pre-fix: SetEnrichmentFinding only stores the finding; it does not re-derive
// r.Findings/r.AttentionDetails, so AttentionDetails stays as it was after
// the first load (nil for wave2 data). The new entry does not appear.
// Post-fix: SetEnrichmentFinding triggers re-derive; wave2 entry is present.
func TestViews_DetailEnrichmentLateUpdatePicksUpFindings(t *testing.T) {
	ensureNoColor(t)

	r := resource.Resource{
		ID:   "i-late",
		Name: "late-instance",
		Fields: map[string]string{
			"InstanceId": "i-late",
		},
		// "impaired" is a real issue phrase — wave1 Finding will be derived.
		Status: "impaired",
	}

	k := keys.Default()
	m := views.NewDetail(r, "ec2", nil, k)
	m.SetSize(200, 100)

	// First render: no enrichment finding yet.
	firstOut := m.PlainContent()
	_ = firstOut // only used to confirm we can render

	// Simulate Wave-2 result arriving later.
	ef := resource.EnrichmentFinding{
		Severity: "!",
		Summary:  "pending maintenance",
		Rows:     []resource.FindingRow{{Label: "Action", Value: "reboot"}},
	}
	m.SetEnrichmentFinding(&ef)

	// Second render: enrichment finding must now appear.
	secondOut := m.PlainContent()
	// The view capitalizes the first letter of phrases for display (e.g.
	// "pending maintenance" → "Pending maintenance"). Use case-insensitive
	// check so the test is not brittle to capitalization rules.
	secondOutLower := strings.ToLower(secondOut)

	if !strings.Contains(secondOutLower, "pending maintenance") {
		t.Errorf("detail Attention section must show wave2 phrase \"pending maintenance\" after SetEnrichmentFinding; got:\n%s", secondOut)
	}
	if !strings.Contains(secondOut, "reboot") {
		t.Errorf("detail Attention section must show wave2 row value \"reboot\" after SetEnrichmentFinding; got:\n%s", secondOut)
	}
}

// ---------------------------------------------------------------------------
// Test 9 — list Status column: Wave 2 overrides lifecycle
// ---------------------------------------------------------------------------

// TestViews_ListStatusColumn_Wave2OverridesLifecycle verifies that when a
// resource has Status="running" (lifecycle steady-state, no wave1 Finding) and
// an enrichment finding is present, the list Status column shows the wave2
// summary ("pending maintenance"), NOT the lifecycle phrase ("running").
//
// After DeriveFindings: lifecycle is filtered → Findings[0] = wave2 entry.
// The list extractCellValue reads Findings[0].Phrase = "pending maintenance".
//
// Pre-fix: "running" is emitted as Findings[0] (wave1), or DeriveFindings was
// never called so extractCellValue falls back to Fields["state"]="running".
// Either way the column shows "running".
// Post-fix: lifecycle filtered; wave2 is Findings[0]; column shows "pending maintenance".
func TestViews_ListStatusColumn_Wave2OverridesLifecycle(t *testing.T) {
	ensureNoColor(t)

	td := minimalTypeDef("ec2-w2-lifecycle-test")
	r := resource.Resource{
		ID:   "i-w2-lc",
		Name: "w2-lifecycle",
		// Status is a lifecycle steady-state — must be filtered by DeriveFindings.
		Status: "running",
		Fields: map[string]string{
			"name":  "w2-lifecycle",
			"state": "running",
		},
		// Wave2 Finding populated by DeriveFindings after enrichment is applied.
		Findings: []domain.Finding{
			{
				Code:     "ec2.pending.maintenance",
				Phrase:   "pending maintenance",
				Severity: domain.SevBroken,
				Source:   "wave2:ec2",
			},
		},
	}

	m := loadList(td, []resource.Resource{r})
	out := renderList(m)

	if !strings.Contains(out, "pending maintenance") {
		t.Errorf("list Status column must show wave2 phrase \"pending maintenance\" when Findings[0] is wave2; got:\n%s", out)
	}
	if strings.Contains(out, "running") {
		t.Errorf("list Status column must NOT show lifecycle phrase \"running\" when Findings is non-empty; got:\n%s", out)
	}
}

// ---------------------------------------------------------------------------
// Test 10 — list Status column: LifecycleKey default is "state"
// ---------------------------------------------------------------------------

// TestViews_ListStatusColumn_LifecycleKeyDefaultIsState verifies that when
// Findings is nil and LifecycleKey is empty on the typeDef, the status column
// still resolves to Fields["state"] because the extractCellValue default is
// "state" (not the column key, not r.Status).
//
// Tested across 6 representative type shorts to ensure the default applies
// regardless of which type is used.
//
// Pre-fix: lifecycleKey = typeDef.LifecycleKey = "" means the condition
// `if lifecycleKey == "" { lifecycleKey = "state" }` may not exist; the
// fallback reads Fields[c.key] = Fields["status"] = "" → blank cell.
// Post-fix: default "state" key is used → Fields["state"] = "running" → visible.
func TestViews_ListStatusColumn_LifecycleKeyDefaultIsState(t *testing.T) {
	ensureNoColor(t)

	// These type shorts all share the same structural test — minimalTypeDef
	// always sets LifecycleKey to "" (no explicit lifecycle key), so the
	// default "state" fallback must kick in.
	typeShorts := []string{"ec2", "s3", "sg", "role", "ng", "kms"}

	for _, short := range typeShorts {
		short := short
		t.Run(short, func(t *testing.T) {
			// Use a unique non-registered name to avoid registry overriding columns.
			td := minimalTypeDef(short + "-lkdefault-test")
			// Explicitly confirm LifecycleKey is empty (minimalTypeDef default).
			if td.LifecycleKey != "" {
				t.Fatalf("test precondition failed: minimalTypeDef set LifecycleKey=%q, want empty", td.LifecycleKey)
			}

			r := resource.Resource{
				ID:   short + "-lk-default",
				Name: short + "-lk-default",
				Fields: map[string]string{
					// "status" is absent — pre-fix extractCellValue reads this, gets blank.
					// "state" is present — post-fix fallback reads this.
					"state": "running",
				},
				Findings: nil,
			}

			m := loadList(td, []resource.Resource{r})
			out := renderList(m)

			if !strings.Contains(out, "running") {
				t.Errorf("[%s] Status column must show Fields[\"state\"]=\"running\" when Findings=nil and LifecycleKey is empty (default=\"state\"); got:\n%s",
					short, out)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test 11 — IssueCount respects per-type Color override (CR finding #3)
// ---------------------------------------------------------------------------

// TestViews_IssueCount_RespectsTypeColorOverride verifies that when a type's
// Color func classifies a resource as ColorBroken, IssueCount() counts it —
// even when the resource's Status is a lifecycle terminal keyword that
// FallbackColor would map to ColorDim (not an issue).
//
// This pins that per-type Color is authoritative for issue classification in
// the empty-Findings fallback path: IssueCount must use td.ResolveColor(r),
// NOT FallbackColor(r.Status).
//
// Setup: td.Color maps Fields["state"]="terminated" to ColorBroken. A resource
// with that state and Findings=nil is loaded. IssueCount must return 1.
//
// Pre-fix: IssueCount fallback uses FallbackColor("terminated") → ColorDim →
// IsIssue()=false → count=0 → FAIL.
// Post-fix: IssueCount fallback uses td.ResolveColor(r) → ColorBroken →
// IsIssue()=true → count=1 → PASS.
func TestViews_IssueCount_RespectsTypeColorOverride(t *testing.T) {
	ensureNoColor(t)

	td := minimalTypeDef("ec2-color-override-test")
	// Override Color so "terminated" maps to ColorBroken. This represents a
	// type-specific policy (e.g. a resource that should never be in terminated
	// state in a healthy account). FallbackColor("terminated") would return
	// ColorDim — this test pins that td.Color, not FallbackColor, is used.
	td.Color = func(r resource.Resource) resource.Color {
		if r.Fields["state"] == "terminated" {
			return resource.ColorBroken
		}
		return resource.ColorHealthy
	}
	r := resource.Resource{
		ID:     "i-term-broken",
		Name:   "terminated-broken",
		Status: "terminated",
		Fields: map[string]string{"state": "terminated"},
		// Findings=nil forces the fallback path: IssueCount must use td.ResolveColor.
		Findings: nil,
	}

	m := loadList(td, []resource.Resource{r})
	got := m.IssueCount()

	if got != 1 {
		t.Errorf("IssueCount() = %d, want 1 (td.Color classifies \"terminated\" as ColorBroken → IsIssue()=true; "+
			"pre-fix value is 0 because FallbackColor(\"terminated\") = ColorDim)", got)
	}
}

// ---------------------------------------------------------------------------
// Test 12 — hasIssueFinding scans ALL findings, not only Findings[0]
// ---------------------------------------------------------------------------

// TestViews_HasIssueFinding_ScansAllFindings pins that issue classification
// considers EVERY finding in the slice, not just Findings[0]. Pre-fix
// hasIssueFinding only checks index 0; post-fix it scans all entries.
//
// Reachable via:
//   - ResourceListModel.IssueCount() — must count rows whose Findings has
//     ANY issue-severity entry, not just at index 0.
//   - Attention filter (ctrl+z) — must keep rows whose Findings has any
//     issue-severity entry visible.
//
// Pre-fix: hasIssueFinding returns len>0 && Findings[0].IsIssue().
//   Row A has Findings[0].Severity=SevOK → false → not counted.
//   Only row B (SevWarn at index 0) is counted → IssueCount()=1.
//
// Post-fix: hasIssueFinding scans all entries.
//   Row A has Findings[1].Severity=SevBroken → true → counted.
//   Row B has Findings[0].Severity=SevWarn → true → counted.
//   IssueCount()=2.
//
// Forward-compat note: production paths post-fix filter lifecycle findings
// before populating r.Findings, so a real resource will not carry [SevOK,
// SevBroken] ordering in production today. This test is a defensive regression
// pin for future per-category PRs that may emit lifecycle Findings explicitly,
// or for any code path that appends findings without pre-sorting by severity.
func TestViews_HasIssueFinding_ScansAllFindings(t *testing.T) {
	ensureNoColor(t)

	td := minimalTypeDef("ec2-mixed-findings")

	// Row A — Findings[0] is non-issue (SevOK), Findings[1] is broken.
	// Pre-fix: hasIssueFinding checks only Findings[0].Severity == SevOK → not an issue.
	// Post-fix: scan all; Findings[1].Severity == SevBroken → is an issue.
	resA := resource.Resource{
		ID:   "i-mixed-1",
		Name: "mixed-findings-1",
		Fields: map[string]string{"state": "running"},
		Findings: []domain.Finding{
			{Code: "ec2.lifecycle", Phrase: "running", Severity: domain.SevOK, Source: "wave1"},
			{Code: "ec2.maint", Phrase: "pending maintenance", Severity: domain.SevBroken, Source: "wave2:ec2"},
		},
	}
	// Row B — only Findings[0]; SevWarn — both pre-fix and post-fix count this.
	resB := resource.Resource{
		ID:   "i-mixed-2",
		Name: "mixed-findings-2",
		Fields: map[string]string{"state": "running"},
		Findings: []domain.Finding{
			{Code: "ec2.warn", Phrase: "node group degraded", Severity: domain.SevWarn, Source: "wave2:ec2"},
		},
	}
	// Row C — no findings; must not be counted.
	resC := resource.Resource{
		ID:   "i-mixed-3",
		Name: "mixed-findings-3",
		Fields: map[string]string{"state": "running"},
	}

	m := loadList(td, []resource.Resource{resA, resB, resC})

	if got := m.IssueCount(); got != 2 {
		t.Errorf("IssueCount() = %d, want 2 (A has SevBroken at index 1, B has SevWarn at index 0 — both IsIssue()). "+
			"Pre-fix value is 1 because hasIssueFinding only checks Findings[0] and A.Findings[0].Severity=SevOK.", got)
	}
}

// ---------------------------------------------------------------------------
// Test 13 — ECS INACTIVE classifies as broken via td.Color (CR finding #1)
// ---------------------------------------------------------------------------

// TestViews_ListColor_ECSInactiveIsBroken pins that an ECS service with
// Fields["status"]="INACTIVE" is classified as broken (ColorBroken) even
// though "inactive" / "INACTIVE" is a lifecycle keyword that FallbackColor
// maps to ColorDim (not an issue).
//
// The ECS service type def (ShortName "ecs-svc") has an explicit Color func
// that returns ColorBroken for "INACTIVE" (types_compute.go). IssueCount
// must respect this via td.ResolveColor(r), not fall back to FallbackColor.
//
// Pre-fix: empty-Findings path uses FallbackColor("INACTIVE") → ColorDim →
// IsIssue()=false → IssueCount()=0 → FAIL.
// Post-fix: empty-Findings path uses td.ResolveColor(r) → reads
// Fields["status"]="INACTIVE" → ColorBroken → IsIssue()=true → IssueCount()=1 → PASS.
func TestViews_ListColor_ECSInactiveIsBroken(t *testing.T) {
	ensureNoColor(t)

	td := resource.FindResourceType("ecs-svc")
	if td == nil {
		t.Fatal("ecs-svc type def not registered — update short name if it changed")
	}

	// Confirm the invariant this test relies on: td.Color must classify INACTIVE
	// as ColorBroken. If this assertion fails, the type def has changed and the
	// test needs updating.
	inactiveProbe := resource.Resource{
		ID:     "svc-probe",
		Name:   "probe",
		Fields: map[string]string{"status": "INACTIVE"},
	}
	if got := td.ResolveColor(inactiveProbe); got != resource.ColorBroken {
		t.Fatalf("precondition: ecs-svc.ResolveColor for INACTIVE = %v, want ColorBroken; "+
			"update this test if the type def changed", got)
	}

	r := resource.Resource{
		ID:   "svc-inactive",
		Name: "inactive-service",
		// Status field intentionally matches the ECS status key.
		Status: "INACTIVE",
		Fields: map[string]string{
			"service_name": "inactive-service",
			"status":       "INACTIVE",
		},
		// Findings=nil forces the fallback path: IssueCount must use td.ResolveColor.
		Findings: nil,
	}

	m := loadList(*td, []resource.Resource{r})
	got := m.IssueCount()

	if got != 1 {
		t.Errorf("IssueCount() = %d, want 1 (ECS INACTIVE is ColorBroken per td.Color; "+
			"pre-fix value is 0 because FallbackColor(\"INACTIVE\") = ColorDim)", got)
	}
}

// ---------------------------------------------------------------------------
// Test 14 — empty-Findings fallback uses td.ResolveColor, not FallbackColor
// (CR finding #3)
// ---------------------------------------------------------------------------

// TestViews_IssueCount_FallbackUsesTypeResolveColor pins that the empty-Findings
// fallback in IssueCount uses td.ResolveColor(r) (which reads full Fields), not
// the coarser FallbackColor(r.Status).
//
// Setup: EC2 type def; resource has Status="" (FallbackColor → ColorHealthy) but
// Fields["state"]="stopped" and Fields["state_reason_code"]="Server.InternalError"
// (AWS forced stop). td.ResolveColor reads Fields and returns ColorBroken.
//
// Pre-fix: IssueCount fallback calls FallbackColor("") → ColorHealthy →
// IsIssue()=false → count=0 → FAIL.
// Post-fix: IssueCount fallback calls td.ResolveColor(r) → reads Fields →
// ColorBroken → IsIssue()=true → count=1 → PASS.
func TestViews_IssueCount_FallbackUsesTypeResolveColor(t *testing.T) {
	ensureNoColor(t)

	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("ec2 type def not registered — update short name if it changed")
	}

	// Confirm the invariant: ec2 Color func must return ColorBroken for a stopped
	// instance with a Server.* state_reason_code.
	brokenProbe := resource.Resource{
		ID:     "i-probe",
		Name:   "probe",
		Status: "",
		Fields: map[string]string{
			"state":             "stopped",
			"state_reason_code": "Server.InternalError",
		},
	}
	if got := td.ResolveColor(brokenProbe); got != resource.ColorBroken {
		t.Fatalf("precondition: ec2.ResolveColor for stopped/Server.InternalError = %v, want ColorBroken; "+
			"update Fields if the type def changed", got)
	}

	r := resource.Resource{
		ID:   "i-server-stopped",
		Name: "server-stopped-instance",
		// Status is deliberately empty so FallbackColor("") → ColorHealthy.
		// td.ResolveColor reads Fields["state"]="stopped" + Server.* reason → ColorBroken.
		Status: "",
		Fields: map[string]string{
			"state":             "stopped",
			"state_reason_code": "Server.InternalError",
		},
		// Findings=nil forces the fallback path: IssueCount must use td.ResolveColor.
		Findings: nil,
	}

	m := loadList(*td, []resource.Resource{r})
	got := m.IssueCount()

	if got != 1 {
		t.Errorf("IssueCount() = %d, want 1 (td.ResolveColor reads Fields and returns ColorBroken for "+
			"stopped/Server.InternalError; pre-fix value is 0 because FallbackColor(\"\") = ColorHealthy)", got)
	}
}
