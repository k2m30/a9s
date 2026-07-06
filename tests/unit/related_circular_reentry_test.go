package unit

// related_circular_reentry_test.go — Pins for the live, deterministic bug:
// a circular related drill (detail A -> drill count-1 row -> detail B ->
// drill count-1 row back to A) leaves the re-entered A's RELATED panel with
// ALL rows bare (no badges, not even a loading state) because the cache
// replay in the NavigationKindDetail branch of handleRelatedNavigate
// (internal/tui/runtime_adapter_related.go) runs BEFORE the first render of
// the re-pushed detail screen, and nothing re-seeds the panel on a cache
// MISS either (no RelatedCheckStarted dispatch).
//
// Harness follows related_cache_bug_test.go / related_navigate_cache_enter_child_test.go:
// build a demo root model, drive it via rootApplyMsg/drainCmds, and assert
// on the ANSI-stripped rendered view. Package unit (not unit_test) is
// required to reach those harness helpers.
//
// Pin 4 (header/version, on coordinator guidance): the invariant that holds
// regardless of what the parallel coder determines "[N]" to be (deliberate
// depth badge vs. accidental corruption) is that the header of a
// drill-entered detail screen must contain the resolved buildinfo version
// string. tui.Version is a package var (set from cmd/a9s/main.go at real
// run time) so the test sets it directly to a sentinel and asserts its
// well-formed presence ("v"+sentinel) in the rendered header — no assertion
// is made here about whether "[N]" also appears, since that is the coder's
// call to make.

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// circularReentrySetup builds a demo root model, loads ec2 and vpc resource
// caches, and opens the ec2 instance's DETAIL view (A) via a real Navigate
// message (mirrors chainNavigateToEC2Detail in ec2_stories_nav_chains_test.go).
// Returns the model plus the ec2 instance resource and the vpc resource used
// as the B hop.
func circularReentrySetup(t *testing.T) (tui.Model, resource.Resource, resource.Resource) {
	t.Helper()

	m := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	ec2Client := fakes.NewEC2()
	ec2Res, err := awsclient.FetchEC2Instances(t.Context(), ec2Client)
	if err != nil || len(ec2Res) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err, len(ec2Res))
	}
	instance := ec2Res[0]

	vpcID, ok := instance.Fields["vpc_id"]
	if !ok || vpcID == "" {
		t.Fatalf("demo ec2 fixture %q has no vpc_id field to drive the A->B hop", instance.ID)
	}
	vpcRes := resource.Resource{
		ID:   vpcID,
		Name: "production-vpc",
		Fields: map[string]string{
			"vpc_id": vpcID,
			"status": "available",
		},
	}

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    ec2Res,
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "vpc",
		Resources:    []resource.Resource{vpcRes},
	})

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &instance,
	})

	return m, instance, vpcRes
}

// feedEC2RelatedResults delivers a RelatedCheckResult with Count=2 for every
// registered ec2 related def (mirrors setupEC2DetailWithResults in
// related_cache_bug_test.go), which drives the PatchRelatedCache intent
// (internal/runtime/handlers_resources.go) that populates
// session.RelatedCacheLRU keyed by RelatedCacheKey("ec2", instance.ID).
func feedEC2RelatedResults(m tui.Model, instanceID string) tui.Model {
	for _, def := range resource.GetRelated("ec2") {
		m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
			ResourceType: "ec2",
			Result: resource.RelatedCheckResult{
				TargetType:  def.TargetType,
				Count:       2,
				ResourceIDs: []string{"related-id-1", "related-id-2"},
			},
		})
	}
	return m
}

// ---------------------------------------------------------------------------
// Pin 1: circular drill A -> B -> A must show CACHED count badges on
// re-entry, not bare rows.
// ---------------------------------------------------------------------------

// TestRelatedCircularReentry_CacheHit_ShowsCachedBadges verifies that after
// A (ec2) accumulates related-check results, drilling A -> B (vpc) -> back to
// the SAME A renders A's RELATED panel with the cached count badges
// immediately, exactly as a direct Esc-based re-entry would (see
// TestBug_RelatedCheckResults_RightColShowsCachedCounts in
// related_cache_bug_test.go for the non-circular analog). RED today: the
// panel renders every row bare.
func TestRelatedCircularReentry_CacheHit_ShowsCachedBadges(t *testing.T) {
	m, instance, vpc := circularReentrySetup(t)
	m = feedEC2RelatedResults(m, instance.ID)

	// A -> B: drill into vpc via the count-1 TargetID pivot.
	m, cmd := rootApplyMsg(m, messages.RelatedNavigate{
		TargetType: "vpc",
		TargetID:   vpc.ID,
		SourceResource: resource.Resource{
			ID:   instance.ID,
			Name: instance.Name,
		},
		SourceType: "ec2",
	})
	m, _ = drainCmds(t, m, cmd, 4)

	viewB := stripANSI(rootViewContent(m))
	if !strings.Contains(viewB, "detail -- "+vpc.ID) {
		t.Fatalf("A->B hop did not land on vpc detail (\"detail -- %s\"); got:\n%s", vpc.ID, viewB)
	}

	// B -> A: drill back into the SAME ec2 instance.
	m, cmd = rootApplyMsg(m, messages.RelatedNavigate{
		TargetType: "ec2",
		TargetID:   instance.ID,
		SourceResource: resource.Resource{
			ID:   vpc.ID,
			Name: vpc.Name,
		},
		SourceType: "vpc",
	})
	m, reentryMsgs := drainCmds(t, m, cmd, 4)

	viewA := stripANSI(rootViewContent(m))
	if !strings.Contains(viewA, "detail -- "+instance.ID) {
		t.Fatalf("B->A hop did not land back on ec2 detail (\"detail -- %s\"); got:\n%s", instance.ID, viewA)
	}

	defs := resource.GetRelated("ec2")
	if len(defs) == 0 {
		t.Fatal("test setup: ec2 has no registered related defs to assert a cached badge against")
	}
	foundBadge := false
	for _, def := range defs {
		if strings.Contains(viewA, def.DisplayName+" (2)") {
			foundBadge = true
			break
		}
	}
	if !foundBadge {
		t.Fatalf("BUG: re-entering ec2 detail A via a circular A->B->A drill must show the CACHED related count badge (e.g. %q) immediately, not bare rows; got:\n%s",
			defs[0].DisplayName+" (2)", viewA)
	}

	for _, msg := range reentryMsgs {
		if _, ok := msg.(messages.RelatedCheckStarted); ok {
			t.Errorf("BUG: circular re-entry into a resource with CACHED related results must not re-dispatch RelatedCheckStarted (cache hit); got one in the cmd chain")
		}
	}
}

// ---------------------------------------------------------------------------
// Pin 2: circular drill with an EMPTY cache for A (fresh session) must show
// the loading state and dispatch checks, not silently render bare rows.
// ---------------------------------------------------------------------------

// TestRelatedCircularReentry_CacheMiss_DispatchesChecks verifies the
// cache-MISS half of the same circular drill: when A has never accumulated
// related results before the B hop, drilling back into A must dispatch a
// RelatedCheckStarted to populate the panel. This is the only reliable,
// falsifiable signal available: renderRelatedPanel
// (internal/tui/views/detail_helpers.go) renders a Loading row as
// "  "+blk.Name — byte-identical, after ANSI stripping, to a bare/never-set
// row (styles.DimText carries no plain-text glyph such as "..."), so a
// rendered-view assertion cannot distinguish "loading" from "bare" and would
// be accidentally green either way. The dispatched-check assertion is the
// real, non-accidental pin for "miss => checks fire".
func TestRelatedCircularReentry_CacheMiss_DispatchesChecks(t *testing.T) {
	m, instance, vpc := circularReentrySetup(t)
	// Deliberately do NOT feed any RelatedCheckResult for A — fresh session,
	// empty cache for RelatedCacheKey("ec2", instance.ID).

	m, cmd := rootApplyMsg(m, messages.RelatedNavigate{
		TargetType: "vpc",
		TargetID:   vpc.ID,
		SourceResource: resource.Resource{
			ID:   instance.ID,
			Name: instance.Name,
		},
		SourceType: "ec2",
	})
	m, _ = drainCmds(t, m, cmd, 4)

	m, cmd = rootApplyMsg(m, messages.RelatedNavigate{
		TargetType: "ec2",
		TargetID:   instance.ID,
		SourceResource: resource.Resource{
			ID:   vpc.ID,
			Name: vpc.Name,
		},
		SourceType: "vpc",
	})
	m, reentryMsgs := drainCmds(t, m, cmd, 4)

	viewA := stripANSI(rootViewContent(m))
	if !strings.Contains(viewA, "detail -- "+instance.ID) {
		t.Fatalf("B->A hop did not land back on ec2 detail (\"detail -- %s\"); got:\n%s", instance.ID, viewA)
	}

	dispatchedCheck := false
	for _, msg := range reentryMsgs {
		if _, ok := msg.(messages.RelatedCheckStarted); ok {
			dispatchedCheck = true
			break
		}
	}

	if !dispatchedCheck {
		t.Fatalf("BUG: circular re-entry into A with an EMPTY related cache must dispatch RelatedCheckStarted so the panel is populated instead of staying bare; got no RelatedCheckStarted in the cmd chain. View:\n%s", viewA)
	}
}

// ---------------------------------------------------------------------------
// Pin 4: header version invariant on the drill-entered detail screen.
// ---------------------------------------------------------------------------

// TestRelatedCircularReentry_HeaderShowsResolvedVersion verifies that after
// the A->B->A circular drill, the header of the re-entered detail screen
// contains the resolved buildinfo version string. tui.Version is a package
// var normally set by cmd/a9s/main.go from internal/buildinfo at real
// startup; the test sets it directly so the assertion is independent of the
// build-time injection mechanism. This pins the direction-agnostic half of
// the header contract: whatever the coder decides "[N]" should be (present
// alongside the version or not at all), the version substring itself must
// never be displaced.
func TestRelatedCircularReentry_HeaderShowsResolvedVersion(t *testing.T) {
	const sentinelVersion = "9.9.9-test"
	prevVersion := tui.Version
	tui.Version = sentinelVersion
	t.Cleanup(func() { tui.Version = prevVersion })

	m, instance, vpc := circularReentrySetup(t)
	m = feedEC2RelatedResults(m, instance.ID)

	m, cmd := rootApplyMsg(m, messages.RelatedNavigate{
		TargetType: "vpc",
		TargetID:   vpc.ID,
		SourceResource: resource.Resource{
			ID:   instance.ID,
			Name: instance.Name,
		},
		SourceType: "ec2",
	})
	m, _ = drainCmds(t, m, cmd, 4)

	m, cmd = rootApplyMsg(m, messages.RelatedNavigate{
		TargetType: "ec2",
		TargetID:   instance.ID,
		SourceResource: resource.Resource{
			ID:   vpc.ID,
			Name: vpc.Name,
		},
		SourceType: "vpc",
	})
	m, _ = drainCmds(t, m, cmd, 4)

	view := stripANSI(rootViewContent(m))
	if !strings.Contains(view, "detail -- "+instance.ID) {
		t.Fatalf("B->A hop did not land back on ec2 detail (\"detail -- %s\"); got:\n%s", instance.ID, view)
	}

	wantVersionSubstr := "v" + sentinelVersion
	if !strings.Contains(view, wantVersionSubstr) {
		t.Fatalf("header on the drill-entered detail screen must contain the resolved version %q; got view:\n%s",
			wantVersionSubstr, view)
	}
}
