package unit_test

// runtime_executor_depth_refetch_test.go — RED regression tests for DEF-9.
//
// Contract: docs/design/cache-requirements.md C1/C2/C5, Goal 3. A warm list
// open that previously persisted an exact total (e.g. 55 rows across 2
// pages) must not have its background verify-refetch (KindFetchResources)
// silently downgrade that total to a single truncated first page (e.g. 50
// rows). The executor must keep paginating via FetchMoreResources up to the
// previously-cached depth (Core.CachedListDepth) before returning
// messages.ResourcesLoaded.
//
// See core/runtime/executor.go KindFetchResources and
// core/runtime/probes.go CachedListDepth.

import (
	"context"
	"errors"
	"testing"

	"github.com/k2m30/a9s/v3/core/cache"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// ────────────────────────────────────────────────────────────────────────────
// helpers
// ────────────────────────────────────────────────────────────────────────────

// depthTestProfile/depthTestRegion are fake, non-real credentials — used only
// to build a deterministic on-disk cache directory under a temp
// A9S_CONFIG_FOLDER for these tests.
const (
	depthTestProfile = "pilot-prof"
	depthTestRegion  = "us-east-1"
)

// newDepthExecutorCore builds a Core identical in spirit to newExecutorCore
// (see runtime_executor_test.go) but with an explicit non-demo profile/region
// pair so tests can control the on-disk cache directory precisely via
// A9S_CONFIG_FOLDER. NoCache defaults to false (caller may override) since
// these tests exercise the cache-store read path.
func newDepthExecutorCore(t *testing.T, noCache bool) *runtime.Core {
	t.Helper()
	c := runtime.Bootstrap(depthTestProfile, depthTestRegion, catalog.All())
	c.SetNoCache(noCache)
	c.SetIsDemo(true)
	fakeClients := demo.NewServiceClients()
	c.SetPreSuppliedClients(fakeClients)
	c.HandleClientsReady(runtime.ClientsReadyEvent{ //nolint:errcheck // intentional — we only need side-effect
		Clients:    nil,
		Gen:        c.ConnectGen(),
		StackDepth: 1,
	})
	return c
}

// seedCachedRows writes a TypeFile with N rows for shortName under the
// A9S_CONFIG_FOLDER-scoped cache dir for (depthTestProfile, depthTestRegion),
// mirroring the production Put/SaveType seam in core/cache/cache.go.
func seedCachedRows(t *testing.T, shortName string, count int, exact bool) {
	t.Helper()
	store := cache.LoadDir(depthTestProfile, depthTestRegion)
	rows := make([]cache.Row, count)
	for i := range rows {
		rows[i] = cache.Row{ID: bucketID(i), Name: bucketID(i)}
	}
	store.Put(shortName, cache.TypeFile{
		HasResources: true,
		Count:        count,
		Exact:        exact,
		Rows:         rows,
	})
	if err := store.SaveType(shortName); err != nil {
		t.Fatalf("seedCachedRows: SaveType(%s): %v", shortName, err)
	}
}

func bucketID(i int) string {
	const digits = "0123456789"
	n := i + 1
	s := ""
	for n > 0 {
		s = string(digits[n%10]) + s
		n /= 10
	}
	for len(s) < 3 {
		s = "0" + s
	}
	return "bucket-" + s
}

// page1Resources builds n realistic resources for the first page.
func page1Resources(n int) []resource.Resource {
	out := make([]resource.Resource, n)
	for i := range out {
		out[i] = resource.Resource{ID: bucketID(i), Name: bucketID(i), Type: "s3pilot"}
	}
	return out
}

// page2Resources builds n realistic resources for the second page,
// continuing the ID sequence from offset.
func page2Resources(offset, n int) []resource.Resource {
	out := make([]resource.Resource, n)
	for i := range out {
		out[i] = resource.Resource{ID: bucketID(offset + i), Name: bucketID(offset + i), Type: "s3pilot"}
	}
	return out
}

// registerDepthFetcherWithPage2 registers a paginated fetcher for shortName
// that returns page1 (50 resources, truncated, NextToken="p2") on an empty
// token, and defers to page2 for the "p2" token. Any other token is an
// error. Cleanup restores prior state.
//
// page2 lets each call site control the follow-up page's outcome without
// duplicating the (identical) page-1 setup: registerDepthFetcher's default
// (page2Resources(50, 5), untruncated) is reused via
// registerDepthFetcher below; registerDepthFetcherErrorOnPage2 supplies a
// page2 that returns an error instead.
func registerDepthFetcherWithPage2(t *testing.T, shortName string, page2 func() (resource.FetchResult, error)) {
	t.Helper()
	resource.SetPaginatedForTest(shortName, func(_ context.Context, _ any, token string) (resource.FetchResult, error) {
		switch token {
		case "":
			return resource.FetchResult{
				Resources: page1Resources(50),
				Pagination: &resource.PaginationMeta{
					IsTruncated: true,
					NextToken:   "p2",
					TotalHint:   -1,
					PageSize:    50,
				},
			}, nil
		case "p2":
			return page2()
		default:
			return resource.FetchResult{}, errors.New("depth-test fetcher: unexpected token " + token)
		}
	})
	t.Cleanup(func() { resource.CleanupPaginatedForTest(shortName) })
}

// registerDepthFetcher registers the standard depth-test fetcher: page1 (50
// resources, truncated) followed by page2 (5 resources, not truncated, 55
// total) on token "p2".
func registerDepthFetcher(t *testing.T, shortName string) {
	t.Helper()
	registerDepthFetcherWithPage2(t, shortName, func() (resource.FetchResult, error) {
		return resource.FetchResult{
			Resources: page2Resources(50, 5),
			Pagination: &resource.PaginationMeta{
				IsTruncated: false,
				NextToken:   "",
				TotalHint:   55,
				PageSize:    5,
			},
		}, nil
	})
}

// registerDepthFetcherErrorOnPage2 registers the depth-test fetcher with the
// same page1 as registerDepthFetcher, but the "p2" token returns an error
// instead of a second page.
func registerDepthFetcherErrorOnPage2(t *testing.T, shortName string) {
	t.Helper()
	registerDepthFetcherWithPage2(t, shortName, func() (resource.FetchResult, error) {
		return resource.FetchResult{}, errors.New("simulated follow-up page failure")
	})
}

// ────────────────────────────────────────────────────────────────────────────
// Test 1 — RefetchesToCachedDepth
// ────────────────────────────────────────────────────────────────────────────

// TestExecuteTask_FetchResources_RefetchesToCachedDepth pins DEF-9 / C1 / C2 /
// C5: when 55 rows were previously persisted for this type, a background
// verify-refetch (KindFetchResources) must keep paginating past the
// truncated 50-row first page until it reaches the previously-shown depth
// (55), and the final Pagination must reflect the LAST page fetched
// (IsTruncated=false) so the frame title renders exact ("s3(55)"), not
// truncated ("s3(50+)").
func TestExecuteTask_FetchResources_RefetchesToCachedDepth(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const shortName = "s3pilot"

	registerDepthFetcher(t, shortName)
	seedCachedRows(t, shortName, 55, true)

	c := newDepthExecutorCore(t, false)
	ev, err := c.ExecuteTask(context.Background(), runtime.TaskRequest{
		Key: runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: shortName},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := ev.(messages.ResourcesLoaded)
	if !ok {
		t.Fatalf("expected messages.ResourcesLoaded, got %T", ev)
	}
	if got.Err != nil {
		t.Errorf("Err = %v, want nil", got.Err)
	}
	if len(got.Resources) != 55 {
		t.Errorf("len(Resources) = %d, want 55 (must not downgrade cached exact total)", len(got.Resources))
	}
	if got.Pagination == nil {
		t.Fatal("Pagination must not be nil after a full re-walk")
	}
	if got.Pagination.IsTruncated {
		t.Error("Pagination.IsTruncated = true, want false — a full re-walk to cached depth must end exact (C5)")
	}

	// Verify concatenation preserved both pages' IDs in order (bug-catching:
	// a naive re-slice or dedup-by-mistake would still pass a bare len check).
	if got.Resources[0].ID != "bucket-001" {
		t.Errorf("Resources[0].ID = %q, want %q", got.Resources[0].ID, "bucket-001")
	}
	if got.Resources[49].ID != "bucket-050" {
		t.Errorf("Resources[49].ID = %q, want %q (last row of page 1)", got.Resources[49].ID, "bucket-050")
	}
	if got.Resources[50].ID != "bucket-051" {
		t.Errorf("Resources[50].ID = %q, want %q (first row of page 2)", got.Resources[50].ID, "bucket-051")
	}
	if got.Resources[54].ID != "bucket-055" {
		t.Errorf("Resources[54].ID = %q, want %q (last row of page 2)", got.Resources[54].ID, "bucket-055")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 2 — NoCachedDepth_SinglePageUnchanged
// ────────────────────────────────────────────────────────────────────────────

// TestExecuteTask_FetchResources_NoCachedDepth_SinglePageUnchanged pins the
// cold-start / no-prior-cache behavior: with no TypeFile persisted for this
// type, CachedListDepth returns 0 and the depth loop's guard
// (len(Resources) < CachedListDepth) is never true on page 1 (50 < 0 is
// false), so the executor returns exactly the first page, truncated.
func TestExecuteTask_FetchResources_NoCachedDepth_SinglePageUnchanged(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const shortName = "s3pilot"

	registerDepthFetcher(t, shortName)
	// No seedCachedRows call — cache dir has no file for shortName.

	c := newDepthExecutorCore(t, false)
	ev, err := c.ExecuteTask(context.Background(), runtime.TaskRequest{
		Key: runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: shortName},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got, ok := ev.(messages.ResourcesLoaded)
	if !ok {
		t.Fatalf("expected messages.ResourcesLoaded, got %T", ev)
	}
	if got.Err != nil {
		t.Errorf("Err = %v, want nil", got.Err)
	}
	if len(got.Resources) != 50 {
		t.Errorf("len(Resources) = %d, want 50 (cold behavior must stay first-page-only)", len(got.Resources))
	}
	if got.Pagination == nil {
		t.Fatal("Pagination must not be nil")
	}
	if !got.Pagination.IsTruncated {
		t.Error("Pagination.IsTruncated = false, want true — no cached depth means no follow-up page fetch")
	}
	if got.Pagination.NextToken != "p2" {
		t.Errorf("Pagination.NextToken = %q, want %q", got.Pagination.NextToken, "p2")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 3 — FollowUpPageError_PartialWithErr
// ────────────────────────────────────────────────────────────────────────────

// TestExecuteTask_FetchResources_FollowUpPageError_PartialWithErr pins the
// partial-success contract (HandleResourcesLoaded, existing behavior at
// core/runtime/handlers_resources.go:84-89): when the depth-loop's
// follow-up FetchMoreResources call errors, the executor must still return
// the resources accumulated so far (page 1's 50) with Err set — never an
// messages.APIError, since some resources DID make it back.
func TestExecuteTask_FetchResources_FollowUpPageError_PartialWithErr(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const shortName = "s3pilot"

	registerDepthFetcherErrorOnPage2(t, shortName)
	seedCachedRows(t, shortName, 55, true)

	c := newDepthExecutorCore(t, false)
	ev, err := c.ExecuteTask(context.Background(), runtime.TaskRequest{
		Key: runtime.TaskKey{Kind: runtime.KindFetchResources, Scope: shortName},
	})
	if err != nil {
		t.Fatalf("unexpected error from ExecuteTask itself: %v", err)
	}
	got, ok := ev.(messages.ResourcesLoaded)
	if !ok {
		t.Fatalf("expected messages.ResourcesLoaded (partial-success contract), got %T", ev)
	}
	if got.Err == nil {
		t.Error("Err must be non-nil when the follow-up page fetch fails")
	}
	if len(got.Resources) != 50 {
		t.Errorf("len(Resources) = %d, want 50 (page 1's resources accumulated before the failure)", len(got.Resources))
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 4 — CachedListDepth accessor direct pins
// ────────────────────────────────────────────────────────────────────────────

// TestCachedListDepth_UnknownType_ReturnsZero pins the "type absent from
// store" branch.
func TestCachedListDepth_UnknownType_ReturnsZero(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	c := newDepthExecutorCore(t, false)
	got := c.CachedListDepth("nonexistent-depth-type-000")
	if got != 0 {
		t.Errorf("CachedListDepth(unknown) = %d, want 0", got)
	}
}

// TestCachedListDepth_NoCache_ReturnsZero pins the NoCache short-circuit —
// even with rows on disk, NoCache=true must return 0 without touching the
// store (mirrors EnsureCacheStore's own NoCache guard).
func TestCachedListDepth_NoCache_ReturnsZero(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const shortName = "s3pilot"
	seedCachedRows(t, shortName, 55, true)

	c := newDepthExecutorCore(t, true) // NoCache=true
	got := c.CachedListDepth(shortName)
	if got != 0 {
		t.Errorf("CachedListDepth with NoCache=true = %d, want 0", got)
	}
}

// TestCachedListDepth_StoredRows_ReturnsCount pins the primary contract:
// depth equals the exact number of persisted Rows.
func TestCachedListDepth_StoredRows_ReturnsCount(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const shortName = "s3pilot"
	seedCachedRows(t, shortName, 55, true)

	c := newDepthExecutorCore(t, false)
	got := c.CachedListDepth(shortName)
	if got != 55 {
		t.Errorf("CachedListDepth(%s) = %d, want 55", shortName, got)
	}
}

// TestCachedListDepth_AliasResolvesToCanonical pins alias canonicalization:
// "rds" is a registered alias of the "dbi" resource type (see
// core/aws/catalog_databases.go ShortName:"dbi", Aliases includes "rds").
// A TypeFile persisted under the canonical name "dbi" must be found when
// CachedListDepth is called with the alias "rds".
func TestCachedListDepth_AliasResolvesToCanonical(t *testing.T) {
	if resource.FindResourceType("rds") == nil {
		t.Skip("no registered type has alias \"rds\" in this build; alias case not testable")
	}
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	seedCachedRows(t, "dbi", 12, true)

	c := newDepthExecutorCore(t, false)
	got := c.CachedListDepth("rds")
	if got != 12 {
		t.Errorf("CachedListDepth(alias %q) = %d, want 12 (must canonicalize to %q)", "rds", got, "dbi")
	}
}
