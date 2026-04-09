package unit

// related_navigate_cache_detail_init_test.go — Tests for the bug where
// handleRelatedNavigate's cache-hit branch never dispatches related checks.
//
// Bug (app_related.go:88-98 and 122-131): when TargetID or a single RelatedID
// matches an entry in resourceCache, the code pushes a detail view and returns
// (m, nil). It never calls NeedsRelatedCheck() / dispatches RelatedCheckStartedMsg.
// This leaves the right column in permanent loading state for those navigations.
//
// TestRelatedNavigate_CachedTargetID_DispatchesRelatedCheck — FAILS with current code.
// TestRelatedNavigate_CachedTargetID_UsesCachedResults       — FAILS with current code.
// TestRelatedNavigate_SingleRelatedID_CacheHit_DispatchesRelatedCheck — FAILS with current code.

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// setupEC2ListWithCache navigates to the EC2 list, loads all EC2 resources, and
// returns the model with the resourceCache populated for "ec2".
func setupEC2ListWithCache(t *testing.T) (tui.Model, []resource.Resource) {
	t.Helper()

	m := tui.New("demo", "us-east-1", tui.WithDemo(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	ec2Client := fakes.NewEC2()
	ec2Res, err := awsclient.FetchEC2Instances(context.Background(), ec2Client)
	if err != nil || len(ec2Res) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err, len(ec2Res))
	}

	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    ec2Res,
	})

	return m, ec2Res
}

// containsRelatedCheckStartedMsg returns true if any message in msgs is a
// RelatedCheckStartedMsg.
func containsRelatedCheckStartedMsg(msgs []tea.Msg) bool {
	for _, msg := range msgs {
		if _, ok := msg.(messages.RelatedCheckStartedMsg); ok {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// TestRelatedNavigate_CachedTargetID_DispatchesRelatedCheck
//
// Given: EC2 resources are loaded into resourceCache["ec2"].
// When:  RelatedNavigateMsg{TargetType:"ec2", TargetID: ec2[0].ID} is sent.
// Then:  The returned cmd is non-nil and produces a RelatedCheckStartedMsg.
//
// This FAILS now: the cache-hit branch returns (m, nil).
// ---------------------------------------------------------------------------

func TestRelatedNavigate_CachedTargetID_DispatchesRelatedCheck(t *testing.T) {
	defs := resource.GetRelated("ec2")
	if len(defs) == 0 {
		t.Fatal("no ec2 related defs registered — internal/aws import should register them")
	}

	m, ec2Res := setupEC2ListWithCache(t)

	// Send a RelatedNavigateMsg using TargetID — the cache-hit branch fires.
	navMsg := messages.RelatedNavigateMsg{
		TargetType: "ec2",
		TargetID:   ec2Res[0].ID,
		SourceResource: resource.Resource{
			ID:   "rds-db-1",
			Name: "source-db",
		},
		SourceType: "rds",
	}

	m, cmd := rootApplyMsg(m, navMsg)
	if cmd == nil {
		t.Fatal("BUG: RelatedNavigateMsg with cached TargetID returned nil cmd — " +
			"must dispatch RelatedCheckStartedMsg so the detail right column loads")
	}

	// Drain one level of the cmd chain to find RelatedCheckStartedMsg.
	_, msgs := drainCmds(t, m, cmd, 3)

	if !containsRelatedCheckStartedMsg(msgs) {
		types := make([]string, len(msgs))
		for i, msg := range msgs {
			types[i] = fmt.Sprintf("%T", msg)
		}
		t.Fatalf("BUG: RelatedNavigateMsg (cached TargetID) must produce RelatedCheckStartedMsg "+
			"in cmd chain; got: %v", types)
	}
}

// ---------------------------------------------------------------------------
// TestRelatedNavigate_CachedTargetID_UsesCachedResults
//
// Given: EC2 detail has been opened, related results delivered (Count=2), then Esc'd.
//        The relatedCache now holds the results for ec2[0].
// When:  RelatedNavigateMsg{TargetID: ec2[0].ID} is sent again for same resource.
// Then:  The detail view immediately shows cached counts (e.g. "(2)") without
//        waiting for async re-dispatch.
//
// This FAILS now: the cache-hit branch in handleRelatedNavigate returns (m, nil)
// WITHOUT applying any cached results to the new detail view. The right column
// is always in loading state after re-navigation via RelatedNavigateMsg.
// ---------------------------------------------------------------------------

func TestRelatedNavigate_CachedTargetID_UsesCachedResults(t *testing.T) {
	defs := resource.GetRelated("ec2")
	if len(defs) == 0 {
		t.Fatal("no ec2 related defs registered — internal/aws import should register them")
	}

	// Use setupEC2DetailWithResults to build state with results cached (Count=2 per type).
	m := setupEC2DetailWithResults(t)

	// Esc back to EC2 list.
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	ec2Client2 := fakes.NewEC2()
	ec2Res, err2 := awsclient.FetchEC2Instances(context.Background(), ec2Client2)
	if err2 != nil || len(ec2Res) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err2, len(ec2Res))
	}

	// Navigate again to the same resource via RelatedNavigateMsg (cache-hit branch).
	navMsg := messages.RelatedNavigateMsg{
		TargetType: "ec2",
		TargetID:   ec2Res[0].ID,
		SourceResource: resource.Resource{
			ID:   "rds-db-1",
			Name: "source-db",
		},
		SourceType: "rds",
	}

	m, _ = rootApplyMsg(m, navMsg)

	view := stripANSI(rootViewContent(m))

	// After the fix: cached results (Count=2) must appear immediately in the view.
	// BUG: the view shows the right column in loading state because cached results
	// are not applied when navigating via RelatedNavigateMsg (cache-hit branch).
	if !strings.Contains(view, "(2)") {
		t.Fatalf("BUG: after RelatedNavigateMsg to a resource with cached related results, "+
			"the detail view must immediately show '(2)' — but it was not found.\nView:\n%s", view)
	}
}

// ---------------------------------------------------------------------------
// TestRelatedNavigate_SingleRelatedID_CacheHit_DispatchesRelatedCheck
//
// Given: EC2 resources are loaded into resourceCache["ec2"].
// When:  RelatedNavigateMsg{TargetType:"ec2", RelatedIDs:[]string{ec2[0].ID}} is sent.
// Then:  The returned cmd is non-nil and produces a RelatedCheckStartedMsg.
//
// This FAILS now: the single-RelatedID cache-hit branch also returns (m, nil).
// ---------------------------------------------------------------------------

func TestRelatedNavigate_SingleRelatedID_CacheHit_DispatchesRelatedCheck(t *testing.T) {
	defs := resource.GetRelated("ec2")
	if len(defs) == 0 {
		t.Fatal("no ec2 related defs registered — internal/aws import should register them")
	}

	m, ec2Res := setupEC2ListWithCache(t)

	// Single RelatedID — uses the len==1 branch in handleRelatedNavigate.
	navMsg := messages.RelatedNavigateMsg{
		TargetType: "ec2",
		RelatedIDs: []string{ec2Res[0].ID},
		SourceResource: resource.Resource{
			ID:   "rds-db-1",
			Name: "source-db",
		},
		SourceType: "rds",
	}

	m, cmd := rootApplyMsg(m, navMsg)
	if cmd == nil {
		t.Fatal("BUG: RelatedNavigateMsg with single cached RelatedID returned nil cmd — " +
			"must dispatch RelatedCheckStartedMsg so the detail right column loads")
	}

	// Drain one level to find RelatedCheckStartedMsg.
	_, msgs := drainCmds(t, m, cmd, 3)

	if !containsRelatedCheckStartedMsg(msgs) {
		types := make([]string, len(msgs))
		for i, msg := range msgs {
			types[i] = fmt.Sprintf("%T", msg)
		}
		t.Fatalf("BUG: RelatedNavigateMsg (single cached RelatedID) must produce "+
			"RelatedCheckStartedMsg in cmd chain; got: %v", types)
	}
}
