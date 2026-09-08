// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// related_cold_fetch_roundtrip_test.go — end-to-end coverage for the
// permanent-"Loading…" regression fixed by FetchResourcesPayload/
// FetchMorePayload's Provenance override (core/runtime/handlers_related.go)
// and activeListRefreshTasks' matching fix (core/app/actions_list.go).
//
// Every prior test in this area either stopped at the TaskRequest
// HandleRelatedNavigate returns (tests/unit/runtime_handlers_related_test.go)
// or started from an already-populated cache — neither shape can catch a
// wrong Provenance stamp, because that defect only manifests once the task's
// RESULT reaches handleResourcesLoadedEvent's symmetric canonical/non-
// canonical gate (core/app/handle.go). A live account hit permanent
// "Loading…" on an uncached related drill that 12 shuffled runs of the
// existing suite never reproduced.
//
// These tests drive the FULL round trip a real cold-cache related navigate
// takes: dispatch (Apply) -> app.DrainSync (the real Core.ExecuteTaskAt,
// same executor a live session uses) -> Handle -> Snapshot. The assertion in
// every case is the rendered ListBody's Rows/Loading/LoadingMore — the one
// thing a permanently-stuck "Loading…" screen cannot fake.
package app_test

import (
	"context"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
)

// newColdFetchController returns a Controller wired to a fresh session (with
// non-nil, unconnected Clients — Core.FetchMoreResources errors on a nil
// pointer, and a real dispatch always has Clients set by the time it can
// reach a related-navigate; a registered resource.SetPaginatedForTest fetcher
// ignores the value) and a detail screen already open on src, ready to
// receive a seeded DetailRelatedRow. Returns the session too, so callers that
// need a cold-cache PARTIAL RowStore entry (the KindFetchMore branch) can
// seed it directly, mirroring tests/unit/runtime_handlers_related_test.go's
// own seeding idiom.
func newColdFetchController(t *testing.T, src resource.Resource, srcType string) (*app.Controller, *session.Session) {
	t.Helper()
	s := session.New()
	s.Clients = &awsclient.ServiceClients{}
	core := runtime.New(s, catalog.All())
	c := app.New(core)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{
		ID:      runtime.ScreenDetail,
		Context: runtime.ScreenContext{ResourceType: srcType, ResourceID: src.ID},
	}})
	c.EnsureDetailState(src, srcType)
	return c, s
}

// selectFirstRelatedRow drives ActionRelatedSelect at visible index 0 — the
// same click path handleActionRelatedSelect implements, and (via the shared
// dispatchRelatedNavigate tail) identical to the TUI/keyboard Enter path — and
// returns the tasks it dispatched.
func selectFirstRelatedRow(t *testing.T, c *app.Controller) []runtime.TaskRequest {
	t.Helper()
	_, tasks := c.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: "0"})
	return tasks
}

// fakePaginatedFetcher registers a resource.PaginatedFetcher for shortName
// that answers exactly the continuation tokens in pages (the empty string is
// the first-page token), restoring whatever fetcher (real or none) was
// previously registered when the test ends. An unrecognised token fails the
// test immediately rather than silently returning a zero FetchResult, so a
// test never mistakes "my fake wasn't wired the way I expected" for "the
// controller round trip is broken".
func fakePaginatedFetcher(t *testing.T, shortName string, pages map[string]resource.FetchResult) {
	t.Helper()
	original := resource.GetPaginatedFetcher(shortName)
	resource.SetPaginatedForTest(shortName, resource.PaginatedFetcher(
		func(_ context.Context, _ any, token string) (resource.FetchResult, error) {
			res, ok := pages[token]
			if !ok {
				t.Fatalf("fake %s fetcher: unexpected continuation token %q", shortName, token)
			}
			return res, nil
		}))
	t.Cleanup(func() {
		if original != nil {
			resource.SetPaginatedForTest(shortName, original)
		} else {
			resource.CleanupPaginatedForTest(shortName)
		}
	})
}

// drillRows fetches the current Snapshot and fails the test immediately if
// the controller is not sitting on a list screen — every cold-fetch branch
// under test here resolves to NavigationKindFilteredList, which always pushes
// a ScreenResourceList placeholder (pushByIDPlaceholderList).
func drillRows(t *testing.T, c *app.Controller) *app.ListBody {
	t.Helper()
	snap := c.Snapshot()
	if snap.Body.Kind != app.BodyKindList {
		t.Fatalf("expected a list screen after the cold fetch, got %q", snap.Body.Kind)
	}
	if snap.Body.List == nil {
		t.Fatal("expected a non-nil ListBody on a list screen")
	}
	return snap.Body.List
}

func resourceIDSet(rows []app.ListRow) map[string]bool {
	set := make(map[string]bool, len(rows))
	for _, r := range rows {
		set[r.ResourceID] = true
	}
	return set
}

// TestColdFetch_TargetIDCacheMiss_NoFetchByIDs_RowsLandOnScreen covers branch
// 1: a single-ID related row whose target type has no registered FetchByIDs
// (lambda — see tests/unit/runtime_handlers_related_test.go's
// TestHandleRelatedNavigate_NonByIDType_CacheMiss_EmitsFetchResources for why
// lambda is the type used to pin this else-branch). HandleRelatedNavigate
// emits a bare KindFetchResources task; pre-fix that task's
// FetchResourcesPayload.Provenance defaulted to the zero value, which the
// executor then stamped FetchProvenanceCanonicalList — permanently rejected
// by handleResourcesLoadedEvent's gate against this non-canonical placeholder
// screen, so the ResourcesLoaded result was silently dropped and the screen
// never left "Loading…".
func TestColdFetch_TargetIDCacheMiss_NoFetchByIDs_RowsLandOnScreen(t *testing.T) {
	const targetARN = "arn:aws:lambda:us-east-1:123456789012:function:cold-fetch-target"

	src := resource.Resource{ID: "i-coldfetch0001", Name: "cold-fetch-src", Type: "ec2"}
	c, _ := newColdFetchController(t, src, "ec2")
	c.ApplyDetailRelated([]app.DetailRelatedRow{{
		TargetType:  "lambda",
		DisplayName: "Lambda Functions",
		Count:       1,
		ResourceIDs: []string{targetARN},
	}})

	fakePaginatedFetcher(t, "lambda", map[string]resource.FetchResult{
		"": {Resources: []resource.Resource{{ID: targetARN, Name: "cold-fetch-target", Type: "lambda"}}},
	})

	tasks := selectFirstRelatedRow(t, c)
	foundFetch := false
	for _, tk := range tasks {
		if tk.Key.Kind == runtime.KindFetchByIDDetail {
			t.Fatalf("lambda has no registered FetchByIDs; must not dispatch KindFetchByIDDetail, got tasks=%+v", tasks)
		}
		if tk.Key.Kind == runtime.KindFetchResources && tk.Key.Scope == "lambda" {
			foundFetch = true
		}
	}
	if !foundFetch {
		t.Fatalf("expected a KindFetchResources task for lambda, got tasks=%+v", tasks)
	}

	app.DrainSync(c, tasks)

	list := drillRows(t, c)
	if list.Loading {
		t.Fatal("drill screen still Loading after the cold fetch executed — permanent-Loading regression " +
			"(mismatched Provenance rejected by handleResourcesLoadedEvent's gate)")
	}
	if len(list.Rows) != 1 || list.Rows[0].ResourceID != targetARN {
		t.Fatalf("drill screen rows = %+v, want exactly one row with ResourceID %q", list.Rows, targetARN)
	}
}

// TestColdFetch_TruncatedReverseScan_RowsLandOnScreen covers branch 2: a
// "(N+)" truncated reverse-scan row. HandleRelatedNavigate's Truncated branch
// (checked before any TargetID/RelatedIDs cache-hit fast path) always emits a
// bare KindFetchResources population fetch. Same pre-fix failure mode as
// above: a defaulted-canonical Provenance stamp permanently rejected against
// the non-canonical scoped-scan screen.
func TestColdFetch_TruncatedReverseScan_RowsLandOnScreen(t *testing.T) {
	const foundID = "i-truncated-0001"

	src := resource.Resource{ID: "sg-coldfetch0001", Name: "cold-fetch-src", Type: "sg"}
	c, _ := newColdFetchController(t, src, "sg")
	c.ApplyDetailRelated([]app.DetailRelatedRow{{
		TargetType:  "ec2",
		DisplayName: "EC2 Instances",
		Count:       1,
		Truncated:   true,
		ResourceIDs: []string{foundID},
	}})

	fakePaginatedFetcher(t, "ec2", map[string]resource.FetchResult{
		"": {
			Resources:  []resource.Resource{{ID: foundID, Name: "cold-fetch-target", Type: "ec2"}},
			Pagination: &resource.PaginationMeta{IsTruncated: false},
		},
	})

	tasks := selectFirstRelatedRow(t, c)
	if len(tasks) != 1 || tasks[0].Key.Kind != runtime.KindFetchResources || tasks[0].Key.Scope != "ec2" {
		t.Fatalf("tasks = %+v, want a single KindFetchResources task for ec2 (truncated reverse-scan population fetch)", tasks)
	}

	app.DrainSync(c, tasks)

	list := drillRows(t, c)
	if list.Loading {
		t.Fatal("drill screen still Loading after the truncated-scan population fetch executed — permanent-Loading " +
			"regression (mismatched Provenance rejected by handleResourcesLoadedEvent's gate)")
	}
	if len(list.Rows) != 1 || list.Rows[0].ResourceID != foundID {
		t.Fatalf("drill screen rows = %+v, want exactly one row with ResourceID %q", list.Rows, foundID)
	}
}

// TestColdFetch_RelatedIDs_CacheMiss_RowsLandOnScreen covers the first half
// of branch 3: multiple RelatedIDs with a full RowStore cache miss (no entry
// at all for the target type) — relatedFetchTasks' "missing == len(RowStore
// rows)" path, which emits a bare KindFetchResources population fetch.
func TestColdFetch_RelatedIDs_CacheMiss_RowsLandOnScreen(t *testing.T) {
	ids := []string{"i-relids-0001", "i-relids-0002"}

	src := resource.Resource{ID: "sg-coldfetch0002", Name: "cold-fetch-src-2", Type: "sg"}
	c, _ := newColdFetchController(t, src, "sg")
	c.ApplyDetailRelated([]app.DetailRelatedRow{{
		TargetType:  "ec2",
		DisplayName: "EC2 Instances",
		Count:       len(ids),
		ResourceIDs: ids,
	}})

	fakePaginatedFetcher(t, "ec2", map[string]resource.FetchResult{
		"": {
			Resources: []resource.Resource{
				{ID: ids[0], Name: "cold-fetch-target-1", Type: "ec2"},
				{ID: ids[1], Name: "cold-fetch-target-2", Type: "ec2"},
			},
			Pagination: &resource.PaginationMeta{IsTruncated: false},
		},
	})

	tasks := selectFirstRelatedRow(t, c)
	if len(tasks) != 1 || tasks[0].Key.Kind != runtime.KindFetchResources || tasks[0].Key.Scope != "ec2" {
		t.Fatalf("tasks = %+v, want a single KindFetchResources task for ec2 (RelatedIDs full cache miss)", tasks)
	}

	app.DrainSync(c, tasks)

	list := drillRows(t, c)
	if list.Loading {
		t.Fatal("drill screen still Loading after the RelatedIDs cache-miss fetch executed — permanent-Loading " +
			"regression (mismatched Provenance rejected by handleResourcesLoadedEvent's gate)")
	}
	got := resourceIDSet(list.Rows)
	for _, id := range ids {
		if !got[id] {
			t.Errorf("drill screen missing expected related row %q; rows=%+v", id, list.Rows)
		}
	}
}

// TestColdFetch_RelatedIDs_PartialCoverageTruncated_RowsLandOnScreen covers
// the second half of branch 3: multiple RelatedIDs where the RowStore already
// has ONE of them cached behind a truncated first page — relatedFetchTasks'
// "tr.Pagination.IsTruncated" path, which emits a KindFetchMore continuation
// instead of a full KindFetchResources refetch. This is the exact shape the
// mechanical pin fix in tests/unit/runtime_handlers_related_test.go's Case H
// covers at the TaskRequest level; this test instead executes that
// KindFetchMore task for real and asserts the missing ID actually lands.
func TestColdFetch_RelatedIDs_PartialCoverageTruncated_RowsLandOnScreen(t *testing.T) {
	const cachedID = "i-partial-0001"
	const pendingID = "i-partial-0002"
	const nextToken = "ec2-partial-page-2"

	src := resource.Resource{ID: "sg-coldfetch0003", Name: "cold-fetch-src-3", Type: "sg"}
	c, s := newColdFetchController(t, src, "sg")
	s.RowStore.Observe("ec2", []resource.Resource{{ID: cachedID, Name: "cached-target", Type: "ec2"}},
		&resource.PaginationMeta{IsTruncated: true, NextToken: nextToken}, session.OriginFetch, false)

	c.ApplyDetailRelated([]app.DetailRelatedRow{{
		TargetType:  "ec2",
		DisplayName: "EC2 Instances",
		Count:       2,
		ResourceIDs: []string{cachedID, pendingID},
	}})

	fakePaginatedFetcher(t, "ec2", map[string]resource.FetchResult{
		nextToken: {
			Resources:  []resource.Resource{{ID: pendingID, Name: "pending-target", Type: "ec2"}},
			Pagination: &resource.PaginationMeta{IsTruncated: false},
		},
	})

	tasks := selectFirstRelatedRow(t, c)
	if len(tasks) != 1 || tasks[0].Key.Kind != runtime.KindFetchMore || tasks[0].Key.Scope != "ec2" {
		t.Fatalf("tasks = %+v, want a single KindFetchMore task for ec2 (partial coverage + truncated cache)", tasks)
	}

	// Precondition: the cache-hit half (seedRelatedExactRows) already seeded
	// the cached row before any task runs — the "zero visible rows" shape is
	// a different bug than the one under test here.
	if pre := drillRows(t, c); len(pre.Rows) != 1 || pre.Rows[0].ResourceID != cachedID {
		t.Fatalf("precondition failed: expected the cache-hit half to seed exactly [%s] before the page-2 fetch runs; got %+v",
			cachedID, pre.Rows)
	}

	app.DrainSync(c, tasks)

	list := drillRows(t, c)
	got := resourceIDSet(list.Rows)
	if !got[cachedID] || !got[pendingID] {
		t.Fatalf("drill screen missing an expected related row after page 2 landed — permanent-Loading regression "+
			"(mismatched Provenance rejected by handleResourcesLoadedEvent's gate); want [%s %s], got rows=%+v",
			cachedID, pendingID, list.Rows)
	}
}

// TestColdFetch_CtrlROnAlreadyOpenDrillList_RowsSurviveRefresh covers the
// sibling defect the same fix caught: Ctrl+R (ActionRefresh) issued while
// already sitting on an open related-navigation drill list.
// activeListRefreshTasks (core/app/actions_list.go) used to stamp its
// FetchResourcesPayload with the zero Provenance unconditionally — a
// permanent mismatch against a non-canonical drill screen's
// isTopLevelCanonicalList()==false, rejected forever by
// handleResourcesLoadedEvent's gate. This was found by generalizing the root
// cause, not by a failing smoke test, so nothing was watching it before this.
func TestColdFetch_CtrlROnAlreadyOpenDrillList_RowsSurviveRefresh(t *testing.T) {
	ids := []string{"i-ctrlr-0001", "i-ctrlr-0002"}

	src := resource.Resource{ID: "sg-coldfetch0004", Name: "cold-fetch-src-4", Type: "sg"}
	c, _ := newColdFetchController(t, src, "sg")
	c.ApplyDetailRelated([]app.DetailRelatedRow{{
		TargetType:  "ec2",
		DisplayName: "EC2 Instances",
		Count:       len(ids),
		ResourceIDs: ids,
	}})

	fakePaginatedFetcher(t, "ec2", map[string]resource.FetchResult{
		"": {
			Resources: []resource.Resource{
				{ID: ids[0], Name: "ctrlr-target-1", Type: "ec2"},
				{ID: ids[1], Name: "ctrlr-target-2", Type: "ec2"},
			},
			Pagination: &resource.PaginationMeta{IsTruncated: false},
		},
	})

	// Land on the drill screen for real first (the same round trip
	// TestColdFetch_RelatedIDs_CacheMiss_RowsLandOnScreen exercises) — Ctrl+R's
	// own regression only shows once a non-canonical drill screen is already
	// open and receives a SECOND fetch result.
	initialTasks := selectFirstRelatedRow(t, c)
	app.DrainSync(c, initialTasks)
	if pre := drillRows(t, c); pre.Loading || len(pre.Rows) != len(ids) {
		t.Fatalf("precondition failed: expected the initial cold fetch to land %d rows before testing Ctrl+R; got loading=%v rows=%+v",
			len(ids), pre.Loading, pre.Rows)
	}

	_, refreshTasks := c.Apply(app.Action{Kind: app.ActionRefresh})
	if len(refreshTasks) == 0 {
		t.Fatal("ActionRefresh on the open drill list returned no tasks")
	}

	app.DrainSync(c, refreshTasks)

	list := drillRows(t, c)
	if list.Loading {
		t.Fatal("drill screen stuck Loading after Ctrl+R — activeListRefreshTasks' Provenance regression")
	}
	if list.Refreshing {
		t.Fatal("drill screen stuck Refreshing after Ctrl+R — activeListRefreshTasks' Provenance regression")
	}
	got := resourceIDSet(list.Rows)
	for _, id := range ids {
		if !got[id] {
			t.Errorf("drill screen missing expected related row %q after Ctrl+R; rows=%+v", id, list.Rows)
		}
	}
}
