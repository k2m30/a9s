package unit

// related_navigate_partial_cache_test.go — Tests for the partial cache / truncated
// pagination bug in handleRelatedNavigate's multi-RelatedIDs branch.
//
// Bug (app_related.go:144-168): when RelatedIDs has >1 entries, the code filters
// the resourceCache. If some IDs are missing AND the cache has IsTruncated==true,
// those IDs may be on a later page — but the code silently shows the incomplete
// filtered list without initiating a fetch.
//
// TestRelatedNavigate_PartialCache_Truncated_FetchesMissing  — FAILS with current code.
// TestRelatedNavigate_AllRelatedIDs_InCache_NoFetch           — PASSES (correct behavior).
// TestRelatedNavigate_PartialCache_NotTruncated_ShowsFiltered — PASSES (correct behavior).

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/demo"
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
// TestRelatedNavigate_PartialCache_NotTruncated_ShowsFiltered
//
// Given: resourceCache["ec2"] has only ec2[0..1], IsTruncated=false.
// When:  RelatedNavigateMsg{RelatedIDs: [ec2[0].ID, "ec2-nonexistent-id"]} is sent.
//        One ID is in the cache; the other does not exist anywhere.
// Then:  The returned cmd is nil (cache is complete — missing IDs just don't exist,
//        no fetch is needed, filtered partial result is acceptable).
//
// This PASSES with current code (correct behavior).
// ---------------------------------------------------------------------------

func TestRelatedNavigate_PartialCache_NotTruncated_ShowsFiltered(t *testing.T) {
	m, ec2Res := setupEC2ListWithCompleteCache(t)

	// Request ec2[0] (in cache) + a nonexistent ID.
	// Since the cache is NOT truncated, the nonexistent ID genuinely doesn't exist.
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

	// Non-truncated cache + missing ID = ID simply doesn't exist.
	// Showing the filtered partial result is correct; no fetch needed.
	if cmd != nil {
		t.Fatal("non-truncated cache with a genuinely missing ID should return nil cmd " +
			"(filtered partial result is acceptable — the ID does not exist)")
	}
}
