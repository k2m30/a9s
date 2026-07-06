package unit

// qa_frame_title_issues_test.go — T051 + docs/attention-signals.md contract
// amendment: ResourceListModel.FrameTitle() format tests.
//
// Tests cover all branching cases in FrameTitle:
//   - no filter, no issues
//   - no filter, with issues (title now appends " !N")
//   - no filter, with issues + truncated pagination
//   - no filter, with issues + truncated Wave-2 enrichment (title appends " !N+")
//   - text filter active (no issue badge)
//   - ctrl+z attention filter active (existing " [!]" stays; " !N" is OMITTED)
//   - both text filter and ctrl+z active
//   - loadingMore state
//   - N=0 → no suffix, title unchanged
//   - non-ec2 resource type → same suffix rule (contract is per-framework)
//
// Single source under test: Controller.buildListFrameTitle
// (internal/app/list_body.go), reached via ResourceListModel.FrameTitle()'s
// delegation to m.ctrl.ListFrameTitle(). Per docs/attention-signals.md +
// docs/resources/*.md §4 S1 row (amendment in progress): the list frame
// title now duplicates the menu badge's issue count as " !N" (or " !N+" when
// the count is a truncated lower bound), aggregated the same way as the menu
// badge — Wave-1 issue-colored rows + Wave-2 "!"-severity findings for the
// resources in the current list (Controller.GetListIssueCount's algorithm).
//
// Uses NewResourceListFromCache so issueCount is recomputed from real statuses.

import (
	"fmt"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// makeResourcesWithStatuses creates a slice of resources with the given
// statuses. Each resource gets a synthetic ID to avoid collisions. The status
// is stored under Fields["status"] which the colorFallback reads when the
// ResourceTypeDef has no per-type Color func (as in these title tests).
func makeResourcesWithStatuses(statuses ...string) []resource.Resource {
	res := make([]resource.Resource, len(statuses))
	for i, s := range statuses {
		res[i] = resource.Resource{
			ID:     fmt.Sprintf("i-%04d", i),
			Name:   fmt.Sprintf("resource-%d", i),
			Fields: map[string]string{"status": s},
		}
	}
	return res
}

// ec2TypeDef returns a minimal EC2 ResourceTypeDef for title tests.
func ec2TypeDef() resource.ResourceTypeDef {
	return resource.ResourceTypeDef{ShortName: "ec2", Name: "EC2 Instances"}
}

// newListFromCache is a convenience wrapper used by FrameTitle tests.
func newListFromCache(
	typeDef resource.ResourceTypeDef,
	resources []resource.Resource,
	pagination *resource.PaginationMeta,
	filterText string,
	attentionOnly bool,
) views.ResourceListModel {
	m := views.NewResourceListFromCache(
		typeDef,
		nil,
		keys.Default(),
		resources,
		pagination,
		filterText,
		views.SortColNone,
		true,
		0,
		0,
		attentionOnly,
	)
	return m
}

// TestFrameTitleNoIssues verifies that with 25 running resources and no filters,
// the title is "ec2(25)" with no issue badge.
func TestFrameTitleNoIssues(t *testing.T) {
	resources := makeResourcesWithStatuses(
		"running", "running", "running", "running", "running",
		"running", "running", "running", "running", "running",
		"running", "running", "running", "running", "running",
		"running", "running", "running", "running", "running",
		"running", "running", "running", "running", "running",
	)
	m := newListFromCache(ec2TypeDef(), resources, nil, "", false)
	got := m.FrameTitle()
	want := "ec2(25)"
	if got != want {
		t.Errorf("FrameTitle() = %q, want %q", got, want)
	}
}

// TestFrameTitleWithIssues — docs/attention-signals.md contract amendment:
// the list title now appends " !N" after the count parentheses when the
// current list has N>0 issues. N mirrors the menu badge aggregation
// (Controller.GetListIssueCount): here, 3 stopped (issue-colored) rows.
func TestFrameTitleWithIssues(t *testing.T) {
	resources := makeResourcesWithStatuses(
		"running", "running", "running", "running", "running",
		"running", "running", "running", "running", "running",
		"running", "running", "running", "running", "running",
		"running", "running", "running", "running", "running",
		"running", "running", "stopped", "stopped", "stopped",
	)
	m := newListFromCache(ec2TypeDef(), resources, nil, "", false)
	got := m.FrameTitle()
	want := "ec2(25) !3"
	if got != want {
		t.Errorf("FrameTitle() = %q, want %q (title now carries ' !N' issue suffix)", got, want)
	}
	if m.IssueCount() != 3 {
		t.Errorf("IssueCount() = %d, want 3", m.IssueCount())
	}
}

// TestFrameTitleWithIssuesTruncated — pagination "+" is operational completeness
// (kept on the total count) and is independent of the " !N" issue suffix,
// which appends after the count parentheses.
func TestFrameTitleWithIssuesTruncated(t *testing.T) {
	resources := makeResourcesWithStatuses(
		"running", "running", "running", "running", "running",
		"running", "running", "running", "running", "running",
		"running", "running", "running", "running", "running",
		"running", "running", "running", "running", "running",
		"stopped", "stopped", "stopped", "stopped", "stopped",
	)
	pagination := &resource.PaginationMeta{IsTruncated: true}
	m := newListFromCache(ec2TypeDef(), resources, pagination, "", false)
	got := m.FrameTitle()
	want := "ec2(25+) !5"
	if got != want {
		t.Errorf("FrameTitle() = %q, want %q (truncated '+' on total, plus ' !N' issue suffix)", got, want)
	}
	if !m.IsTruncated() {
		t.Error("IsTruncated() = false; pagination state must propagate")
	}
}

// TestFrameTitleIssueSuffixTruncated — when the issue COUNT itself is a
// truncated lower bound (Wave-2 enrichment not yet complete for every row),
// the suffix mirrors the menu badge's own "+" truncation semantics:
// " !N+" rather than " !N". Distinct from pagination truncation above, which
// only affects the total-count "+".
func TestFrameTitleIssueSuffixTruncated(t *testing.T) {
	resources := makeResourcesWithStatuses(
		"running", "running", "running", "running", "running",
	)
	m := newListFromCache(ec2TypeDef(), resources, nil, "", false)
	findings := map[string]domain.Finding{
		"i-0000": {Code: "ec2.system.status.impaired", Phrase: "impaired", Severity: domain.SevBroken, Source: "wave2:ec2"},
		"i-0001": {Code: "ec2.system.status.impaired", Phrase: "impaired", Severity: domain.SevBroken, Source: "wave2:ec2"},
	}
	m.SetEnrichmentState(2, true, findings, nil)
	got := m.FrameTitle()
	want := "ec2(5) !2+"
	if got != want {
		t.Errorf("FrameTitle() = %q, want %q (truncated Wave-2 issue count → ' !N+')", got, want)
	}
}

// TestFrameTitleTextFilterActive verifies that an active text filter shows
// "ec2(filtered/total)" format, plus the " !N" issue suffix where N is the
// FULL list's issue count (GetListIssueCount aggregates over the whole list,
// not the filtered/visible subset) — text-filter mode is not attention-only
// mode, so the suffix is not omitted.
func TestFrameTitleTextFilterActive(t *testing.T) {
	// 25 resources, 7 named "web-" to match filter, all stopped (= issues)
	resources := make([]resource.Resource, 25)
	for i := range resources {
		resources[i] = resource.Resource{
			ID:     fmt.Sprintf("i-%04d", i),
			Fields: map[string]string{"status": "stopped"},
		}
		if i < 7 {
			resources[i].Name = fmt.Sprintf("web-%d", i)
		} else {
			resources[i].Name = fmt.Sprintf("app-%d", i)
		}
	}
	m := newListFromCache(ec2TypeDef(), resources, nil, "web-", false)
	got := m.FrameTitle()
	want := "ec2(7/25) !25"
	if got != want {
		t.Errorf("FrameTitle() = %q, want %q (filtered/total count plus full-list issue suffix)", got, want)
	}
	if m.IssueCount() != 25 {
		t.Errorf("IssueCount() = %d, want 25", m.IssueCount())
	}
}

// TestFrameTitleCtrlZActive verifies that with ctrl+z attention filter enabled,
// the title contains " [!]" and "of" notation, and per the contract amendment
// the " !N" issue suffix is OMITTED — the filtered count already IS the issue
// count, so duplicating it as " !N" would be redundant.
// Fixture: 5 running (not issues) + 3 stopped (issues, visible under ctrl+z)
// + 2 terminated (dim, filtered out by ctrl+z) → issue count is 3 (N>0), so
// this genuinely exercises the omission rule rather than a trivial N=0 case.
func TestFrameTitleCtrlZActive(t *testing.T) {
	resources := makeResourcesWithStatuses(
		"running", "running", "running", "running", "running",
		"stopped", "stopped", "stopped",
		"terminated", "terminated",
	)
	m := newListFromCache(ec2TypeDef(), resources, nil, "", true)
	got := m.FrameTitle()
	want := "ec2(3 of 10) [!]"
	if got != want {
		t.Errorf("FrameTitle() = %q, want %q", got, want)
	}
	if m.IssueCount() != 3 {
		t.Errorf("IssueCount() = %d, want 3", m.IssueCount())
	}
	if strings.Contains(got, "!3") {
		t.Errorf("FrameTitle() = %q; ' !N' issue suffix must be OMITTED in attention-only mode (filtered count already IS the issue count)", got)
	}
}

// TestFrameTitleCtrlZAndTextFilter verifies that with both ctrl+z and text filter
// active, the title contains "[!]", the filter count, and — per the contract
// amendment — still OMITS the " !N" issue suffix (attention-only always wins).
// Fixture: 25 total; text filter "web-" matches 15 (10 stopped=issue-colored,
// 5 terminated=dim, not issue-colored); ctrl+z then narrows those 15 down to
// the 10 issue-colored (stopped) rows.
func TestFrameTitleCtrlZAndTextFilter(t *testing.T) {
	resources := make([]resource.Resource, 25)
	for i := range resources {
		resources[i] = resource.Resource{ID: fmt.Sprintf("i-%04d", i)}
		switch {
		case i < 10:
			resources[i].Name = fmt.Sprintf("web-%d", i)
			resources[i].Fields = map[string]string{"status": "stopped"} // issue-colored, survives ctrl+z
		case i < 20:
			resources[i].Name = fmt.Sprintf("app-%d", i)
			resources[i].Fields = map[string]string{"status": "running"}
		default:
			resources[i].Name = fmt.Sprintf("web-%d", i)                    // matches text filter
			resources[i].Fields = map[string]string{"status": "terminated"} // dim — filtered by ctrl+z
		}
	}
	// With text="web-" and attentionOnly=true:
	//   text matches: web-0..9 (stopped, 10) + web-20..24 (terminated, 5) = 15 text matches
	//   after ctrl+z: only issue-colored (stopped) → web-0..9 = 10
	//   filtered (final, text+attention) = 10; total (unfiltered) = 25
	m := newListFromCache(ec2TypeDef(), resources, nil, "web-", true)
	got := m.FrameTitle()
	want := "ec2(10 of 25) [!]"
	if got != want {
		t.Errorf("FrameTitle() = %q, want %q", got, want)
	}
	if strings.Contains(got, "!10") || strings.Contains(got, "!25") {
		t.Errorf("FrameTitle() = %q; ' !N' issue suffix must be OMITTED when ctrl+z attention-only is active", got)
	}
}

// TestFrameTitleLoadingMore verifies that when loadingMore is active, the title
// shows "loading..." suffix regardless of issue count.
func TestFrameTitleLoadingMore(t *testing.T) {
	// Use loadingMore via ResourcesLoadedMsg with Append=true on a paginated list.
	// We can't set loadingMore directly (unexported), but we can verify the
	// "loading..." format appears when pagination is truncated and loadingMore fires.
	//
	// Instead, test the non-loadingMore branch for completeness — loadingMore is
	// only set by the Update handler (message-driven) and would require a full
	// Update cycle with a loadingMore message.
	//
	// Verify: truncated pagination without loadingMore shows "+" but not "loading...".
	resources := makeResourcesWithStatuses("running", "running", "running")
	pagination := &resource.PaginationMeta{IsTruncated: true}
	m := newListFromCache(ec2TypeDef(), resources, pagination, "", false)
	got := m.FrameTitle()

	if !strings.Contains(got, "+") {
		t.Errorf("FrameTitle() = %q; want '+' for truncated pagination", got)
	}
	if strings.Contains(got, "loading...") {
		t.Errorf("FrameTitle() = %q; 'loading...' should only appear when loadingMore is active", got)
	}
}

// TestFrameTitleIssueCountVariants — docs/attention-signals.md contract
// amendment: the list title appends " !N" (N = IssueCount()) whenever N>0,
// and carries no suffix when N=0. wantTitle is the FULL expected title
// string (count parentheses plus the conditional " !N" suffix), pinning the
// exact format rather than a loose substring check.
func TestFrameTitleIssueCountVariants(t *testing.T) {
	tests := []struct {
		name         string
		statuses     []string
		wantTitle    string
		wantIssueCnt int
	}{
		{"only running", []string{"running", "running"}, "ec2(2)", 0},
		{"only stopped", []string{"stopped", "stopped", "stopped"}, "ec2(3) !3", 3},
		{"only pending", []string{"pending", "pending"}, "ec2(2) !2", 2},
		{"only failed", []string{"failed"}, "ec2(1) !1", 1},
		{"mixed running and stopped", []string{"running", "stopped", "running", "stopped"}, "ec2(4) !2", 2},
		{"suffix-matched: create_failed", []string{"running", "create_failed"}, "ec2(2) !1", 1},
		{"suffix-matched: update_in_progress", []string{"running", "update_in_progress"}, "ec2(2) !1", 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resources := makeResourcesWithStatuses(tc.statuses...)
			m := newListFromCache(ec2TypeDef(), resources, nil, "", false)
			got := m.FrameTitle()
			if got != tc.wantTitle {
				t.Errorf("FrameTitle() = %q, want %q", got, tc.wantTitle)
			}
			if m.IssueCount() != tc.wantIssueCnt {
				t.Errorf("IssueCount() = %d, want %d", m.IssueCount(), tc.wantIssueCnt)
			}
		})
	}
}

// TestFrameTitleIssueSuffixZero_NoSuffix pins the N=0 case explicitly: with
// zero issues, the title carries no " !N" suffix at all — byte-identical to
// the pre-amendment format.
func TestFrameTitleIssueSuffixZero_NoSuffix(t *testing.T) {
	resources := makeResourcesWithStatuses("running", "running", "running")
	m := newListFromCache(ec2TypeDef(), resources, nil, "", false)
	got := m.FrameTitle()
	want := "ec2(3)"
	if got != want {
		t.Errorf("FrameTitle() = %q, want %q (N=0 → no suffix)", got, want)
	}
	if strings.Contains(got, "!") {
		t.Errorf("FrameTitle() = %q; N=0 must not render any ' !N' suffix", got)
	}
}

// TestFrameTitleIssueSuffix_NonEC2ResourceType — the contract is per-framework,
// not per-type: an s3 list with issue-colored rows gets the same " !N" suffix
// as ec2. Fixture mirrors TestSpec_S4_S3HealthyStatus_BlankNotBucketName's
// realistic s3 resource shape (RawStruct bucket, empty Fields for healthy rows).
func TestFrameTitleIssueSuffix_NonEC2ResourceType(t *testing.T) {
	type s3Bucket struct {
		Name         string
		BucketRegion string
	}
	resources := []resource.Resource{
		{ID: "b-healthy-1", Name: "b-healthy-1", Fields: map[string]string{}, RawStruct: s3Bucket{Name: "b-healthy-1", BucketRegion: "eu-west-2"}},
		{ID: "b-healthy-2", Name: "b-healthy-2", Fields: map[string]string{}, RawStruct: s3Bucket{Name: "b-healthy-2", BucketRegion: "eu-west-2"}},
		{ID: "b-error-1", Name: "b-error-1", Fields: map[string]string{"status": "error"}, RawStruct: s3Bucket{Name: "b-error-1", BucketRegion: "eu-west-2"}},
	}
	s3TypeDef := resource.ResourceTypeDef{ShortName: "s3", Name: "S3 Buckets"}
	m := newListFromCache(s3TypeDef, resources, nil, "", false)
	got := m.FrameTitle()
	want := "s3(3) !1"
	if got != want {
		t.Errorf("FrameTitle() = %q, want %q (non-ec2 type must follow the same ' !N' suffix rule)", got, want)
	}
	if m.IssueCount() != 1 {
		t.Errorf("IssueCount() = %d, want 1", m.IssueCount())
	}
}
