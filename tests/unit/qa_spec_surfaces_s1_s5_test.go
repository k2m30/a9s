package unit

// qa_spec_surfaces_s1_s5_test.go — reveal tests that pin docs/attention-signals.md
// and docs/resources/<type>.md §4 allowed surfaces S1–S5.
//
// Spec §4: "No other UI is allowed." The five authorized surfaces are:
//   S1 | Menu `issues:N` badge — integer, no suffix, no plus.
//   S2 | Row color by state bucket.
//   S3 | `!` or `~` glyph prefix on Healthy rows only. No `?`, no others.
//   S4 | Status column text — §4 phrase, or blank for healthy. No bucket-name fallback.
//   S5 | Detail Attention section — every finding rendered, no ceremony.
//
// These tests MUST fail against the buggy baseline so a reader can see the
// exact invented UI being removed. Each test comment cites the spec line.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// stripANSI strips ANSI escape sequences for substring inspection.
// Many rendered frames carry color codes; plain-text assertions need the raw text.
func stripANSISpec(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// -----------------------------------------------------------------------------
// S1 — Menu badge must be "issues:N" with no "+" suffix.
// Spec §4 S1: "Aggregated count of `!`-severity findings." The format is an
// integer; truncation state is behavioral (ctrl+z visibility), never rendered.
// -----------------------------------------------------------------------------

func TestSpec_S1_MenuBadge_NoPlusSuffixWhenTruncated(t *testing.T) {
	m := views.NewMainMenu(keys.Default())
	// Seed: 3 ec2 issues with a truncated count — buggy code renders "issues:3+".
	m.SetIssues("ec2", 3, true)
	m.SetAvailability("ec2", 50)
	m.SetSize(120, 40)
	view := stripANSISpec(m.View())
	if !strings.Contains(view, "issues:3") {
		t.Fatalf("badge missing entirely; menu must render 'issues:3'. view:\n%s", view)
	}
	if strings.Contains(view, "issues:3+") {
		t.Errorf("spec §4 S1 violation: menu badge must be 'issues:N' with no '+' suffix; got '+' in:\n%s", view)
	}
}

// -----------------------------------------------------------------------------
// S1 / banner — No ⓘ info banner in the list view.
// Spec §4: "No other UI is allowed." Any `ⓘ`-prefixed derived banner is
// invented UI — in particular "count is a lower bound (truncated)" and
// "N background-check finding(s) off-viewport" are both illegal.
// -----------------------------------------------------------------------------

func TestSpec_NoBanner_WhenEnrichmentTruncated(t *testing.T) {
	m := views.NewResourceListFromCache(
		resource.ResourceTypeDef{ShortName: "ec2", Name: "EC2 Instances"},
		nil, keys.Default(),
		[]resource.Resource{
			{ID: "i-001", Name: "web-01", Status: "running"},
		},
		nil, "", views.SortColNone, true, 0, 0, false,
	)
	m.SetShowIssueBadge(true)
	m.SetSize(120, 40)
	// Buggy code: truncated=true causes findingsBanner to emit the ⓘ banner.
	m.SetEnrichmentState(0, true, nil)
	view := stripANSISpec(m.View())
	for _, forbidden := range []string{
		"ⓘ",
		"count is a lower bound",
		"(truncated)",
		"background-check finding",
		"off-viewport",
	} {
		if strings.Contains(view, forbidden) {
			t.Errorf("spec §4 violation: list view must not render invented banner text %q. view:\n%s", forbidden, view)
		}
	}
}

func TestSpec_NoBanner_WhenFindingsOffViewport(t *testing.T) {
	res := []resource.Resource{
		{ID: "i-001", Name: "web-01", Status: "running"},
		{ID: "i-002", Name: "web-02", Status: "running"},
	}
	m := views.NewResourceListFromCache(
		resource.ResourceTypeDef{ShortName: "ec2", Name: "EC2 Instances"},
		nil, keys.Default(), res, nil, "", views.SortColNone, true, 0, 0, false,
	)
	m.SetShowIssueBadge(true)
	m.SetSize(120, 5) // tiny viewport — forces hidden findings
	// Attach findings to rows that would be off-viewport after sort/filter.
	findings := map[string]domain.Finding{
		"i-001": {Code: "ec2.system.status.impaired", Phrase: "some finding", Severity: domain.SevBroken, Source: "wave2:ec2"},
		"i-002": {Code: "ec2.system.status.impaired", Phrase: "some finding", Severity: domain.SevBroken, Source: "wave2:ec2"},
	}
	m.SetEnrichmentState(2, false, findings)
	view := stripANSISpec(m.View())
	if strings.Contains(view, "background-check finding") || strings.Contains(view, "ⓘ") {
		t.Errorf("spec §4 violation: no off-viewport banner. view:\n%s", view)
	}
}

// -----------------------------------------------------------------------------
// S3 — Only `!` and `~` glyphs. No `?`, no others.
// Spec §4 S3: "`!` / `~` glyph prefix appears only on Healthy (green) rows".
// The `?` glyph (currently emitted for truncated per-resource enrichment) is
// invented UI.
// -----------------------------------------------------------------------------

func TestSpec_S3_NoQuestionGlyph_OnTruncatedEnrichment(t *testing.T) {
	res := []resource.Resource{
		{ID: "i-001", Name: "web-01", Status: "running"},
	}
	m := views.NewResourceListFromCache(
		resource.ResourceTypeDef{ShortName: "ec2", Name: "EC2 Instances"},
		nil, keys.Default(), res, nil, "", views.SortColNone, true, 0, 0, false,
	)
	m.SetShowIssueBadge(true)
	m.SetSize(120, 20)
	// Buggy code: truncatedByID renders a "? " prefix on the identity column.
	m.SetTruncatedIDs(map[string]bool{"i-001": true})
	view := stripANSISpec(m.View())
	if strings.Contains(view, "? web-01") || strings.Contains(view, "? i-001") {
		t.Errorf("spec §4 S3 violation: only `!` and `~` glyphs are allowed; `?` is invented. view:\n%s", view)
	}
}

// -----------------------------------------------------------------------------
// S4 — Healthy row Status cell is blank, never the bucket/resource name.
// Spec §4 S4: "Healthy rows render blank — no `OK` / `available` / `ACTIVE`
// / `running`. Empty means 'nothing to see.'" A Path fallback on the Status
// column resolves to RawStruct.Name on every healthy row — a violation.
// -----------------------------------------------------------------------------

func TestSpec_S4_S3HealthyStatus_BlankNotBucketName(t *testing.T) {
	// Construct an s3 resource with no Fields["status"] set (healthy, as a
	// real AWS fetcher produces). The list view must render the Status cell
	// as BLANK. The current `defaults_databases.go` s3 Status column has
	// `Path: "Name"`, which falls through to the bucket name. That is what
	// this test reveals.
	cfg := &config.ViewsConfig{Views: map[string]config.ViewDef{
		"s3": config.DefaultViewDef("s3"),
	}}
	type s3Bucket struct {
		Name         string
		BucketRegion string
	}
	bucket := s3Bucket{Name: "authservice-prod-state", BucketRegion: "eu-west-2"}
	res := []resource.Resource{{
		ID:        "authservice-prod-state",
		Name:      "authservice-prod-state",
		Status:    "",
		Fields:    map[string]string{}, // no "status" key — healthy bucket, no enrichment yet
		RawStruct: bucket,
	}}
	m := views.NewResourceListFromCache(
		resource.ResourceTypeDef{ShortName: "s3", Name: "S3 Buckets"},
		cfg, keys.Default(), res, nil, "", views.SortColNone, true, 0, 0, false,
	)
	m.SetShowIssueBadge(true)
	m.SetSize(200, 20)
	view := stripANSISpec(m.View())
	// Find the row for our bucket.
	var rowLine string
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "authservice-prod-state") {
			rowLine = line
			break
		}
	}
	if rowLine == "" {
		t.Fatalf("row not rendered. view:\n%s", view)
	}
	// Count occurrences of the bucket name in the row. A spec-compliant row
	// has exactly ONE occurrence (the identity column). Two or more means
	// the Status column fell through to Path: Name — i.e. the bug.
	n := strings.Count(rowLine, "authservice-prod-state")
	if n > 1 {
		t.Errorf("spec §4 S4 violation: healthy s3 row rendered bucket name in Status column (appears %dx). Row:\n%s", n, rowLine)
	}
}

// -----------------------------------------------------------------------------
// Frame title — No `+` suffix, no "(N/M issue)" duplicate of S1.
// Spec §4: S1 is the MENU badge. Re-exposing the issue count inside the list
// title duplicates the signal to an unauthorized surface, and `+` is the
// same invented truncation marker as S1 (banned).
// -----------------------------------------------------------------------------

// Note: "+" on the TOTAL count in the frame title is an operational
// completeness signal (count is a lower bound), NOT a spec §4 attention
// signal. It is allowed. What IS illegal is "issues:N+" on the menu badge
// (tested elsewhere) and the "(N/M issue)" S1 duplicate.

func TestSpec_ListTitle_NoIssueBadge_DuplicatedFromMenu(t *testing.T) {
	// Four broken instances → issueCount = 4 after applySortAndFilter.
	res := []resource.Resource{
		{ID: "i-001", Name: "a", Status: "stopped"},
		{ID: "i-002", Name: "b", Status: "stopped"},
		{ID: "i-003", Name: "c", Status: "stopped"},
		{ID: "i-004", Name: "d", Status: "stopped"},
	}
	m := views.NewResourceListFromCache(
		resource.ResourceTypeDef{ShortName: "ec2", Name: "EC2 Instances"},
		nil, keys.Default(), res, nil, "", views.SortColNone, true, 0, 0, false,
	)
	m.SetShowIssueBadge(true)
	title := m.FrameTitle()
	// Spec §4: S1 is the MENU badge. The list title is NOT S1.
	// Acceptable: "ec2(4)". Illegal: "ec2(4/4 issues)".
	if strings.Contains(title, "issue") || strings.Contains(title, "issues") {
		t.Errorf("spec §4 violation: list title must not duplicate S1 'issues' count; got %q", title)
	}
}
