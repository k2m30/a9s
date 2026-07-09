// app_cache_wave_regressions_test.go — regression pins for the Codex+
// CodeRabbit fix wave on internal/app (branch feat/cache).
//
// Covers:
//
//  1. Non-destructive refresh (C8): ActionRefresh over a list with rows
//     already on screen must keep Loading=false/Rows intact immediately
//     after Apply (only the empty-list case blanks to the Loading shell).
//  2. Truncated disk seed shows N+ (C1/C5): a warm list open seeded from a
//     disk cache TypeFile whose Count is a truncated lower bound must render
//     the "N+" title / HasPagination=true BEFORE any refetch lands.
//  3. Badge fallback (S1/DEF-8 regression): a row whose td.ResolveColor is
//     an issue color but whose ONLY finding is a non-badge Wave-2 "~"
//     (SevWarn) finding must still count in listIssueCount/GetListIssueCount
//     — gating the color check on len(r.Findings)==0 undercounts any row
//     that carries ANY finding, badge or not.
//
// All tests are hermetic: A9S_CONFIG_FOLDER redirected to t.TempDir() where
// disk state is involved, no AWS credentials, no network. Fake resource IDs
// only.
package unit_test

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/cache"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
)

// ────────────────────────────────────────────────────────────────────────────
// Test 7 — non-destructive refresh (C8)
// ────────────────────────────────────────────────────────────────────────────

// TestActionRefresh_ListWithRows_StaysNonDestructive pins C8: applying
// ActionRefresh over a list screen that already has rows on it must NOT
// blank Loading=true/Rows=nil in the immediately-returned ViewState — the
// cached content stays visible under the Refreshing marker while the
// refetch runs, and any prior LastFetchError is cleared. Cache invalidation
// (DeleteResourceCache) still happens so the refetch is a genuine re-fetch,
// not a served-from-cache no-op — this test does not assert on that
// invalidation directly (it is a session-cache-internal side effect with no
// public read seam), only on the RENDERED non-destructive contract.
func TestActionRefresh_ListWithRows_StaysNonDestructive(t *testing.T) {
	_, ctrl := newLiveWebStyleController(t, "", "us-east-1")

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	rows := []resource.Resource{
		{ID: "i-0refresh1", Name: "refresh-instance-1", Type: "ec2", Fields: map[string]string{"state": "running"}},
		{ID: "i-0refresh2", Name: "refresh-instance-2", Type: "ec2", Fields: map[string]string{"state": "running"}},
	}
	ctrl.ApplyResourcesLoaded("ec2", rows, nil, false)

	seeded := ctrl.Snapshot().Body.List
	if seeded == nil || len(seeded.Rows) != 2 {
		t.Fatalf("test setup: expected 2 rows on the ec2 list before refresh, got %+v", seeded)
	}

	vs, _ := ctrl.Apply(app.Action{Kind: app.ActionRefresh})
	lb := vs.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after ActionRefresh")
	}
	if lb.Loading {
		t.Error("Loading = true after ActionRefresh over a list with rows already on screen, want false — C8: cached content must stay visible under the refreshing marker, not blank to the Loading shell")
	}
	if len(lb.Rows) != 2 {
		t.Errorf("len(Rows) = %d after ActionRefresh, want unchanged 2 — C8: a refresh over non-empty content must not blank Rows", len(lb.Rows))
	}
	if lb.LastFetchError != "" {
		t.Errorf("LastFetchError = %q after ActionRefresh, want empty — a fresh refresh clears any prior error marker", lb.LastFetchError)
	}
}

// TestActionRefresh_EmptyList_StillShowsLoading pins the non-regression half
// of C8: a list screen with ZERO rows (nothing to keep under a marker) must
// still show the Loading shell on ActionRefresh — there was never any
// content for the refreshing marker to sit under.
func TestActionRefresh_EmptyList_StillShowsLoading(t *testing.T) {
	_, ctrl := newLiveWebStyleController(t, "", "us-east-1")

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	ctrl.ApplyResourcesLoaded("ec2", []resource.Resource{}, nil, false)

	seeded := ctrl.Snapshot().Body.List
	if seeded == nil || len(seeded.Rows) != 0 {
		t.Fatalf("test setup: expected 0 rows on the ec2 list before refresh, got %+v", seeded)
	}

	vs, _ := ctrl.Apply(app.Action{Kind: app.ActionRefresh})
	lb := vs.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after ActionRefresh")
	}
	if !lb.Loading {
		t.Error("Loading = false after ActionRefresh over an EMPTY list, want true — there is no content for a refreshing marker to sit under, so the Loading shell must still show")
	}
	if len(lb.Rows) != 0 {
		t.Errorf("len(Rows) = %d after ActionRefresh over an empty list, want 0", len(lb.Rows))
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 8 — truncated disk seed shows N+ before any refetch lands
// ────────────────────────────────────────────────────────────────────────────

// TestWarmListOpen_TruncatedDiskSeed_ShowsNPlus_BeforeRefetch pins C1/C5's
// "show what you know" contract for a TRUNCATED disk observation: a disk
// cache TypeFile persisted with Count=50 and Exact=false (a truncated first
// page from a prior session) must, once loaded via the real
// AvailabilityCacheLoaded seam and the type's list opened, render "50+" in
// the frame title and HasPagination=true — BEFORE any live refetch task has
// executed. This is the disk-cache-seeded counterpart to the
// already-covered in-session ProbeTruncated seeding path
// (internal/app/navigate.go's NavigateKindPushResourceList branch): here the
// truncation signal must survive a full cold load-from-disk round trip via
// AvailabilityCacheLoaded, not just a same-session probe.
func TestWarmListOpen_TruncatedDiskSeed_ShowsNPlus_BeforeRefetch(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile, region = "npluswarm-prof", "us-east-1"

	rows := make([]cache.Row, 50)
	for i := range rows {
		id := "i-0npluswarm" + string(rune('a'+i%26))
		rows[i] = cache.Row{ID: id, Name: id, Fields: map[string]string{"state": "running"}}
	}
	store := cache.LoadDir(profile, region)
	store.Put("ec2", cache.TypeFile{
		HasResources: true,
		Count:        50,
		Exact:        false, // truncated: a prior session's first-page-only observation
		Rows:         rows,
	})
	if err := store.SaveType("ec2"); err != nil {
		t.Fatalf("seed SaveType(ec2): %v", err)
	}

	core, ctrl := newLiveWebStyleController(t, profile, region)
	loadedStore := core.EnsureCacheStore()
	if loadedStore == nil {
		t.Fatal("EnsureCacheStore returned nil")
	}
	ctrl.Handle(runtime.CacheStoreToEvent(loadedStore))

	vs, _ := ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	lb := vs.Body.List
	if lb == nil {
		t.Fatal("Body.List is nil after opening a warm, truncated-disk-seeded ec2 list")
	}
	if lb.Loading {
		t.Fatal("test setup: Loading=true — expected a warm (disk-cache-seeded) list open with rows already renderable")
	}
	if len(lb.Rows) != 50 {
		t.Fatalf("len(Rows) = %d, want 50 disk-seeded rows before any refetch lands", len(lb.Rows))
	}
	if !lb.Truncated {
		t.Error("ListBody.Truncated = false, want true — a disk-seeded TypeFile with Exact=false must carry its truncation signal into the warm-opened list, before any refetch confirms/replaces it")
	}

	title := ctrl.ListFrameTitle()
	if !strings.Contains(title, "50+") {
		t.Errorf("ListFrameTitle() = %q, want it to contain %q — C1/C5: a truncated disk observation must render the lower-bound suffix immediately on warm open", title, "50+")
	}

	// Non-regression: HasPagination must have Truncated (not something
	// stronger, like a full false positive of "load more" being disabled)
	// disabled — pagination.HasMore mirrors ls.HasPagination in buildListBody.
	if !lb.Pagination.HasMore {
		t.Error("ListBody.Pagination.HasMore = false, want true — the truncated disk seed must also surface as a load-more-eligible pagination state")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Test 10 — S1 badge fallback: color-issue row with only a non-badge finding
// ────────────────────────────────────────────────────────────────────────────

// TestGetListIssueCount_ColorIssueRow_OnlyNonBadgeFinding_StillCounted pins
// the S1 badge fallback regression: an ec2 row whose td.ResolveColor(r)
// resolves to an issue color (state="stopped" -> ColorBroken, per colorEC2)
// but whose ONLY finding is a non-badge Wave-2 "~" (SevWarn) finding — which
// listHasBadgeFinding correctly does NOT count as a badge finding — must
// STILL be counted via the td.ResolveColor(r).IsIssue() fallback. Gating
// that fallback on len(r.Findings)==0 (the pre-fix behavior) undercounts any
// row carrying ANY finding at all, badge or not, even though the row's own
// color is independently an issue.
// A row whose ONLY finding is a non-badge Wave-2 "~" (SevWarn) must NOT bump the
// issue count: per docs/attention-signals.md S1 "~ findings do not bump", and
// colorEC2 is colorFromAnyFinding-only (it does not read Fields["state"]), so
// once runtime.Wave1Only strips the lone Wave-2 warn no issue signal remains.
// This is the reversal of the former DEF-8 behavior — the list frame title now
// matches the menu badge's unifiedIssueCount exactly.
func TestGetListIssueCount_LoneWave2Warn_NotCounted(t *testing.T) {
	_, ctrl := newLiveWebStyleController(t, "", "us-east-1")

	_, _ = ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "ec2"})
	rows := []resource.Resource{
		{
			ID:   "i-0badgefallback1",
			Name: "wave2-warn-only-instance",
			Type: "ec2",
			Fields: map[string]string{
				// Inert: colorEC2 does not read Fields["state"], so the only color
				// signal for this row is the Wave-2 warn finding below.
				"state": "stopped",
			},
			Findings: []domain.Finding{
				{Code: "ec2-idle-hint", Phrase: "instance idle > 7d", Severity: domain.SevWarn, Source: "wave2:ec2"},
			},
		},
		{
			ID:   "i-0badgefallback2",
			Name: "healthy-instance",
			Type: "ec2",
			Fields: map[string]string{
				"state": "running", // colorEC2: no finding -> ColorHealthy
			},
		},
	}
	ctrl.ApplyResourcesLoaded("ec2", rows, nil, false)

	got := ctrl.GetListIssueCount()
	if got != 0 {
		t.Errorf("GetListIssueCount() = %d, want 0 — a lone Wave-2 \"~\" (SevWarn) finding must not bump the count (docs/attention-signals.md S1); Wave1Only strips it and colorEC2 reads no other signal", got)
	}

	title := ctrl.ListFrameTitle()
	if strings.Contains(title, "!") {
		t.Errorf("ListFrameTitle() = %q, want NO issue-count suffix — a lone Wave-2 warn does not bump the count", title)
	}
}
