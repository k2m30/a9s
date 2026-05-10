package unit

// related_navigate_partial_cache_test.go — Tests for the partial-cache /
// pagination contract in handleRelatedNavigate's multi-RelatedIDs branch
// (internal/tui/runtime_adapter_related.go, NavigationKindFilteredList +
// RelatedIDs path).
//
// The runtime is the sole decision-maker for fetch tasks
// (internal/runtime/handlers_related.go relatedFetchTasks). The adapter must
// translate the emitted []TaskRequest into tea.Cmd values and never silently
// drop them, regardless of how much the cache already covers.
//
// TestRelatedNavigate_PartialCache_Truncated_FetchesMissing       — partial + truncated → fetch (KindFetchMore translation).
// TestRelatedNavigate_AllRelatedIDs_InCache_NoFetch                — full coverage → nil cmd.
// TestRelatedNavigate_PartialCache_NotTruncated_FetchesFullList    — partial + not truncated → fetch (KindFetchResources translation).
//   AS-216 / AS-245 (PR #345 R2) regression guard.

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// setupEC2ListWithTruncatedCache navigates to the EC2 list and loads only the
// first EC2 resource with IsTruncated=true, simulating a partial first page.
// Returns the model and the full EC2 resource list (for ID references).
func setupEC2ListWithTruncatedCache(t *testing.T) (tui.Model, []resource.Resource) {
	t.Helper()

	m := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	ec2Client := fakes.NewEC2()
	ec2Res, err := awsclient.FetchEC2Instances(context.Background(), ec2Client)
	if err != nil || len(ec2Res) < 2 {
		t.Fatalf("demo ec2 fixtures need at least 2 resources (err=%v, len=%d)", err, len(ec2Res))
	}

	// Load only the FIRST resource with truncated pagination.
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    ec2Res[0:1],
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "token123"},
	})

	return m, ec2Res
}

// setupEC2ListWithCompleteCache navigates to the EC2 list and loads all
// resources with no truncation (complete cache).
func setupEC2ListWithCompleteCache(t *testing.T) (tui.Model, []resource.Resource) {
	t.Helper()

	m := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	ec2Client := fakes.NewEC2()
	ec2Res, err := awsclient.FetchEC2Instances(context.Background(), ec2Client)
	if err != nil || len(ec2Res) < 2 {
		t.Fatalf("demo ec2 fixtures need at least 2 resources (err=%v, len=%d)", err, len(ec2Res))
	}

	// Load ALL resources with no truncation.
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    ec2Res,
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
	})

	return m, ec2Res
}

// cmdIsNonNilFetch returns true if cmd is non-nil. Used to check whether the
// model initiated an async fetch instead of returning a pure cache hit.
func cmdIsNonNilFetch(cmd tea.Cmd) bool {
	return cmd != nil
}

// ---------------------------------------------------------------------------
// TestRelatedNavigate_PartialCache_Truncated_FetchesMissing
//
// Given: resourceCache["ec2"] has only ec2[0], IsTruncated=true.
// When:  RelatedNavigateMsg{RelatedIDs: [ec2[0].ID, ec2[1].ID]} is sent.
//        ec2[1] is NOT in the cache; the cache is truncated (more pages exist).
// Then:  The returned cmd is non-nil (a fetch was initiated).
//
// This FAILS now: the branch finds ec2[0], misses ec2[1], and silently shows
// the incomplete filtered list instead of fetching the remaining pages.
// ---------------------------------------------------------------------------

func TestRelatedNavigate_PartialCache_Truncated_FetchesMissing(t *testing.T) {
	m, ec2Res := setupEC2ListWithTruncatedCache(t)

	// Request two IDs: one in cache (ec2[0]), one NOT in cache (ec2[1]).
	navMsg := messages.RelatedNavigateMsg{
		TargetType: "ec2",
		RelatedIDs: []string{ec2Res[0].ID, ec2Res[1].ID},
		SourceResource: resource.Resource{
			ID:   "i-source",
			Name: "source-instance",
		},
		SourceType: "ec2",
	}

	_, cmd := rootApplyMsg(m, navMsg)

	// The cache is truncated and a requested ID is missing → must fetch.
	if !cmdIsNonNilFetch(cmd) {
		t.Fatal("BUG: RelatedNavigateMsg with partial truncated cache must initiate a fetch " +
			"(cmd should be non-nil) — missing IDs may be on later pages")
	}
}

// ---------------------------------------------------------------------------
// TestRelatedNavigate_AllRelatedIDs_InCache_NoFetch
//
// Given: resourceCache["ec2"] has ALL ec2 resources, IsTruncated=false.
// When:  RelatedNavigateMsg{RelatedIDs: [ec2[0].ID, ec2[1].ID]} is sent.
//        Both IDs are in the cache.
// Then:  The returned cmd is nil (pure cache hit, no fetch needed).
//
// This PASSES with current code (correct behavior).
// ---------------------------------------------------------------------------

func TestRelatedNavigate_AllRelatedIDs_InCache_NoFetch(t *testing.T) {
	m, ec2Res := setupEC2ListWithCompleteCache(t)

	// Request two IDs: both are in the complete cache.
	navMsg := messages.RelatedNavigateMsg{
		TargetType: "ec2",
		RelatedIDs: []string{ec2Res[0].ID, ec2Res[1].ID},
		SourceResource: resource.Resource{
			ID:   "i-source",
			Name: "source-instance",
		},
		SourceType: "ec2",
	}

	_, cmd := rootApplyMsg(m, navMsg)

	// All IDs are in the complete (non-truncated) cache → no fetch needed.
	if cmd != nil {
		t.Fatal("all RelatedIDs are in a complete (non-truncated) cache — cmd should be nil " +
			"(no fetch needed for a pure cache hit)")
	}
}

// ---------------------------------------------------------------------------
// TestRelatedNavigate_PartialCache_NotTruncated_FetchesFullList
//
// Given: resourceCache["ec2"] holds the loaded EC2 list, IsTruncated=false.
// When:  RelatedNavigateMsg{RelatedIDs: [ec2[0].ID, "ec2-nonexistent-id"]} is sent.
//        One ID is in the cache; the other is not — coverage is partial.
// Then:  The returned cmd is non-nil (a full re-fetch is issued).
//
// Contract: per internal/runtime/handlers_related.go relatedFetchTasks() and
// the pinning runtime test
// TestRelatedFetchTasks_PartialCoverage_NotTruncated_FetchAll, when any
// RelatedID is missing from cache and the cache cannot page further
// (Pagination == nil OR IsTruncated == false), the runtime emits a
// KindFetchResources task. The adapter must honor that task — a missing ID may
// genuinely not exist OR may simply not have been observed yet, and the
// runtime is the sole decision-maker. Returning nil here would silently strand
// the user on an incomplete list and divergence the adapter from the runtime
// SSOT. The pre-populated cached row stays visible while the fetch is in
// flight (the view is built before the fetch cmd is returned).
//
// This is the AS-216 / AS-245 (PR #345 R2) regression guard for
// internal/tui/runtime_adapter_related.go's RelatedIDs branch.
// ---------------------------------------------------------------------------

func TestRelatedNavigate_PartialCache_NotTruncated_FetchesFullList(t *testing.T) {
	m, ec2Res := setupEC2ListWithCompleteCache(t)

	// Request ec2[0] (in cache) + a nonexistent ID — partial coverage.
	navMsg := messages.RelatedNavigateMsg{
		TargetType: "ec2",
		RelatedIDs: []string{ec2Res[0].ID, "ec2-nonexistent-xxxxxxxxxxx"},
		SourceResource: resource.Resource{
			ID:   "i-source",
			Name: "source-instance",
		},
		SourceType: "ec2",
	}

	_, cmd := rootApplyMsg(m, navMsg)

	// Partial coverage + non-truncated cache → runtime emits
	// KindFetchResources and the adapter must propagate it, even though the
	// cached row is already pre-populated in the view.
	if !cmdIsNonNilFetch(cmd) {
		t.Fatal("BUG: RelatedNavigateMsg with partial coverage on a non-truncated cache " +
			"must initiate a full re-fetch (cmd should be non-nil) — runtime is the SSOT " +
			"for the fetch decision and emits KindFetchResources here " +
			"(see internal/runtime/handlers_related.go relatedFetchTasks)")
	}
}
