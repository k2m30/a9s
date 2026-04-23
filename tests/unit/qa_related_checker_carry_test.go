package unit

// qa_related_checker_carry_test.go — reveal tests for checker-carry on
// related navigation. When a related-panel pivot is reverse-scan-based over
// a truncated cache, the initial ResourceIDs set is a LOWER BOUND. Further
// pages of the target type may contain additional matching resources. The
// fix: navigate carries the original checker forward; each new
// ResourcesLoadedMsg re-runs the checker against the delta, and newly
// matched IDs merge into the filter set.
//
// This is universal behavior across all approximate pivots — (0+) / (10+) /
// (25+) only differ in the initial match count, not in how m-loads-more
// should extend it.

import (
	"context"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// testReapplyChecker returns a RelatedChecker that matches any role whose
// Fields["policy_resources"] contains the source bucket's ID (ARN fragment).
// Mirrors the shape of real checkers (e.g. checkS3Role).
func testReapplyChecker(_ context.Context, _ any, src resource.Resource, cache resource.ResourceCache) resource.RelatedCheckResult {
	entry, ok := cache["role"]
	if !ok {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	var matched []string
	for _, r := range entry.Resources {
		if strings.Contains(r.Fields["policy_resources"], src.ID) {
			matched = append(matched, r.ID)
		}
	}
	if len(matched) == 0 {
		return resource.RelatedCheckResult{TargetType: "role", Count: 0}
	}
	return resource.RelatedCheckResult{
		TargetType:  "role",
		Count:       len(matched),
		ResourceIDs: matched,
	}
}

// TestSpec_RelatedCheckerCarry_ApproxZero_GrowsOnLoadMore verifies that a
// pivot that opened with `(0+)` (approximate zero) picks up matches as
// additional pages of the target type load in via m-loads-more.
func TestSpec_RelatedCheckerCarry_ApproxZero_GrowsOnLoadMore(t *testing.T) {
	src := resource.Resource{ID: "bucket-X", Name: "bucket-X"}
	typeDef := resource.ResourceTypeDef{ShortName: "role", Name: "IAM Roles"}

	// Initial state: zero known IDs, approximate=true (reverse-scan cache
	// was truncated; more pages pending).
	m := views.NewResourceListFromCache(
		typeDef, nil, keys.Default(),
		nil, // no resources loaded yet
		nil, "", views.SortColNone, true, 0, 0, false,
	)
	m.SetRelatedIDFilter(nil)             // 0 initial IDs
	m.SetReapplyChecker(testReapplyChecker, src)

	// Precondition: empty filter → nothing matches whatever is loaded.
	if got := m.RelatedIDFilterSize(); got != 0 {
		t.Fatalf("precondition: relatedIDSet size = %d, want 0", got)
	}

	// First page loads: two roles, neither matches.
	page1 := []resource.Resource{
		{ID: "role-1", Fields: map[string]string{"policy_resources": "s3:::other-bucket"}},
		{ID: "role-2", Fields: map[string]string{"policy_resources": ""}},
	}
	m.ReapplyCheckerAgainst(page1)
	if got := m.RelatedIDFilterSize(); got != 0 {
		t.Errorf("after page1 (no matches): relatedIDSet size = %d, want 0", got)
	}

	// Second page (m-loads-more): new role that DOES mention bucket-X.
	page2 := []resource.Resource{
		{ID: "role-3", Fields: map[string]string{"policy_resources": "arn:aws:s3:::bucket-X/*"}},
		{ID: "role-4", Fields: map[string]string{"policy_resources": "s3:::other"}},
	}
	m.ReapplyCheckerAgainst(page2)
	if got := m.RelatedIDFilterSize(); got != 1 {
		t.Errorf("after page2 (role-3 matches): relatedIDSet size = %d, want 1", got)
	}

	// Third page: two more matches — filter grows to 3.
	page3 := []resource.Resource{
		{ID: "role-5", Fields: map[string]string{"policy_resources": "arn:aws:s3:::bucket-X/key1"}},
		{ID: "role-6", Fields: map[string]string{"policy_resources": "arn:aws:s3:::bucket-X/key2"}},
	}
	m.ReapplyCheckerAgainst(page3)
	if got := m.RelatedIDFilterSize(); got != 3 {
		t.Errorf("after page3: relatedIDSet size = %d, want 3", got)
	}
}

// TestSpec_RelatedCheckerCarry_NonApprox_StillExtends verifies that even a
// non-approximate pivot (e.g. (25)) keeps extending on load-more — the
// initial "25 IDs" set is just a starting point; new pages can reveal more
// matches the original cache didn't hold. The behavior is identical to
// approximate pivots, per user spec ("no difference between 0+/10+/25+").
func TestSpec_RelatedCheckerCarry_NonApprox_StillExtends(t *testing.T) {
	src := resource.Resource{ID: "bucket-Y", Name: "bucket-Y"}
	typeDef := resource.ResourceTypeDef{ShortName: "role", Name: "IAM Roles"}

	m := views.NewResourceListFromCache(
		typeDef, nil, keys.Default(),
		nil, nil, "", views.SortColNone, true, 0, 0, false,
	)
	// Seed with the checker's initial output: 2 known matches.
	m.SetRelatedIDFilter([]string{"role-a", "role-b"})
	m.SetReapplyChecker(testReapplyChecker, src)

	if got := m.RelatedIDFilterSize(); got != 2 {
		t.Fatalf("precondition: relatedIDSet size = %d, want 2", got)
	}

	// m-loads-more delivers a new page with one additional match.
	page := []resource.Resource{
		{ID: "role-c", Fields: map[string]string{"policy_resources": "arn:aws:s3:::bucket-Y/*"}},
	}
	m.ReapplyCheckerAgainst(page)
	if got := m.RelatedIDFilterSize(); got != 3 {
		t.Errorf("after page: relatedIDSet size = %d, want 3 (role-a, role-b, role-c)", got)
	}
}

// TestSpec_RelatedCheckerCarry_ZeroInitial_FiltersAwayUnrelated verifies
// that a zero-initial (0+) navigation still HIDES unrelated rows even
// before the checker has produced any matches. Without this, the operator
// navigates to a pivot expecting "related roles" and is shown every role
// in the account (bug reported against the live ./a9s binary on 2026-04-23).
//
// The invariant: carrying a checker implies "the list is filtered", even
// if the filter-match set is empty. Nil filter ≠ empty filter.
func TestSpec_RelatedCheckerCarry_ZeroInitial_FiltersAwayUnrelated(t *testing.T) {
	src := resource.Resource{ID: "bucket-Z", Name: "bucket-Z"}
	typeDef := resource.ResourceTypeDef{ShortName: "role", Name: "IAM Roles"}

	// First page of 3 roles, NONE mention bucket-Z. After navigation from a
	// (0+) pivot, checker-carry is set BEFORE resources load.
	page1 := []resource.Resource{
		{ID: "role-a", Name: "unrelated-a", Fields: map[string]string{"policy_resources": "arn:aws:s3:::other"}},
		{ID: "role-b", Name: "unrelated-b", Fields: map[string]string{"policy_resources": ""}},
		{ID: "role-c", Name: "unrelated-c", Fields: map[string]string{"policy_resources": "s3:::nothing"}},
	}

	// Simulate the navigation: list created with empty ID filter (0+ pivot)
	// and a reapply checker. Resources load in subsequently via cache.
	m := views.NewResourceListFromCache(
		typeDef, nil, keys.Default(),
		page1,                       // resources loaded
		nil, "", views.SortColNone, true, 0, 0, false,
	)
	m.SetReapplyChecker(testReapplyChecker, src)
	// Trigger the re-apply as the app handler would on ResourcesLoadedMsg.
	m.ReapplyCheckerAgainst(page1)

	// Expected: 0 rows visible (none match the bucket predicate).
	// Bug behavior: 3 rows visible (filter inert when relatedIDSet is nil).
	if got := len(m.VisibleResources()); got != 0 {
		t.Errorf("zero-initial carry must hide unrelated rows; got %d visible, want 0", got)
	}

	// Now page 2 arrives via m-loads-more: one matching role.
	page2 := []resource.Resource{
		{ID: "role-match", Name: "related", Fields: map[string]string{"policy_resources": "arn:aws:s3:::bucket-Z/*"}},
	}
	m.AppendResourcesForTest(page2)
	m.ReapplyCheckerAgainst(page2)
	// Expected: 1 row visible (the match).
	if got := len(m.VisibleResources()); got != 1 {
		t.Errorf("after page 2 load-more: want 1 visible, got %d", got)
	}
}

// TestSpec_RelatedCheckerCarry_PreservesSort verifies that when the
// checker-carry merge grows the ID set, the active sort is re-applied so
// newly visible rows land in the correct sorted position (not appended
// to the bottom).
func TestSpec_RelatedCheckerCarry_PreservesSort(t *testing.T) {
	src := resource.Resource{ID: "bucket-S", Name: "bucket-S"}
	typeDef := resource.ResourceTypeDef{
		ShortName: "role",
		Name:      "IAM Roles",
		Columns: []resource.Column{
			{Key: "name", Title: "Name", Width: 30, Sortable: true},
		},
	}

	// All three rows match the predicate; names are in reverse-alpha order.
	// If sort is re-applied after the merge, visible order should be
	// alphabetical ascending.
	page := []resource.Resource{
		{ID: "r-zeta", Name: "zeta", Fields: map[string]string{"policy_resources": "arn:aws:s3:::bucket-S/*", "name": "zeta"}},
		{ID: "r-mu", Name: "mu", Fields: map[string]string{"policy_resources": "arn:aws:s3:::bucket-S/*", "name": "mu"}},
		{ID: "r-alpha", Name: "alpha", Fields: map[string]string{"policy_resources": "arn:aws:s3:::bucket-S/*", "name": "alpha"}},
	}
	m := views.NewResourceListFromCache(
		typeDef, nil, keys.Default(),
		page, nil, "", 0, true, 0, 0, false,
	)
	m.SetReapplyChecker(testReapplyChecker, src)
	m.ReapplyCheckerAgainst(page)

	vis := m.VisibleResources()
	if len(vis) != 3 {
		t.Fatalf("want 3 visible rows, got %d", len(vis))
	}
	// Sort column 0 (Name) ascending → alpha, mu, zeta.
	if vis[0].Name != "alpha" || vis[1].Name != "mu" || vis[2].Name != "zeta" {
		t.Errorf("sort not re-applied after checker merge; got order: %s, %s, %s",
			vis[0].Name, vis[1].Name, vis[2].Name)
	}
}

// TestSpec_RelatedCheckerCarry_NoChecker_Inert verifies that a list
// without a carried checker is unaffected — the feature is opt-in and does
// not regress other navigation paths.
func TestSpec_RelatedCheckerCarry_NoChecker_Inert(t *testing.T) {
	typeDef := resource.ResourceTypeDef{ShortName: "ec2", Name: "EC2 Instances"}
	m := views.NewResourceListFromCache(
		typeDef, nil, keys.Default(),
		nil, nil, "", views.SortColNone, true, 0, 0, false,
	)
	m.SetRelatedIDFilter([]string{"i-1", "i-2"})
	// No SetReapplyChecker call.

	page := []resource.Resource{
		{ID: "i-3", Fields: map[string]string{"policy_resources": "anything"}},
	}
	m.ReapplyCheckerAgainst(page) // should be a no-op

	if got := m.RelatedIDFilterSize(); got != 2 {
		t.Errorf("without a carried checker, filter must remain untouched; size = %d, want 2", got)
	}
}
