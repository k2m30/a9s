package unit

// issue233_empty_truncated_cache_test.go — Tests for issue #233.
//
// Bug: When a cold-cache fetch returns zero resources but IsTruncated=true,
// the subsequent cache write-back in app.go:383 drops the pagination metadata
// because of the guard `if entry.IsTruncated && len(entry.Resources) > 0`.
//
// This means buildResourceCacheSnapshot() (app_related.go:304) reconstructs
// IsTruncated from `entry.pagination != nil`, which is nil because step 2 skipped it.
//
// Result: After one empty-but-truncated cold miss, all subsequent related checks
// see IsTruncated=false and report a definitive Count=0 instead of the honest
// lower bound {Count:0, Truncated:true} (relatedResultTrunc).
//
// New contract (Batch B): truncated-zero produces {Count:0, Truncated:true} via
// relatedResultTrunc. See related.go:34-38 and ValidateRelatedResult.
//
// Three-step corruption path (from the issue):
//   1. app_related.go:67-71 correctly captures IsTruncated=true even on empty Resources
//   2. app.go:383 has `if entry.IsTruncated && len(entry.Resources) > 0` — skips persistence
//   3. app_related.go:304 reconstructs IsTruncated from `entry.pagination != nil` — nil
//
// Tests:
//   TestContract_EmptyTruncatedPage_PreservesIsTruncated — FAILS with current code (reveals bug)
//   TestContract_NonEmptyTruncatedPage_PreservesIsTruncated — PASSES with current code (control)
//   TestContract_EmptyCompletePage_IsTruncatedFalse — PASSES with current code (negative control)
//   TestContract_EmptyTruncatedPage_CheckerBehavior_Direct — PASSES (isolates checker from write-back)

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// setupLiveModeEC2Detail creates a NON-demo root model (so the real checker path runs),
// navigates to EC2 detail for the first fixture, and returns the model plus EC2 fixtures.
//
// Non-demo mode is required because demo mode's related-check dispatch bypasses
// buildResourceCacheSnapshot() entirely and uses registered demo checkers instead.
// Only the live-mode path calls the real checker with the cache snapshot.
func setupLiveModeEC2Detail(t *testing.T) (tui.Model, []resource.Resource) {
	t.Helper()

	// Non-demo model: no WithIsDemo option.
	// This makes the related-check dispatch use the real checker path (not demo fixtures).
	m := newBlessedModel(t, "test-profile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	ec2Client := fakes.NewEC2()
	ec2Res, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEC2InstancesPage(context.Background(), ec2Client, token)
	})
	if err != nil || len(ec2Res) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err, len(ec2Res))
	}

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ec2",
		Resources:    ec2Res,
	})

	// Enter first EC2 detail. In non-demo mode this dispatches the related-check
	// tasks directly, triggering live-mode checker dispatch. We drain but ignore
	// those cmds — we only care about the subsequent write-back path.
	m, firstCmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	m, _ = drainCmds(t, m, firstCmd, 3)

	return m, ec2Res
}

// execRelatedCheckAndCollectTGResult presses Ctrl+R on the (already-open) ec2
// detail screen — the real re-dispatch entry point now that the fan-out has
// no standalone trigger message — and collects the "tg" RelatedCheckResult
// from the resulting (possibly nested) tea.Batch. Returns (result, found).
//
// handleActionRefresh (core/app/actions_list.go) invalidates RelatedCache and
// begins a fresh DetailOperation unconditionally on Ctrl+R, so the resulting
// related-check fan-out always calls the real checker against the CURRENT
// buildResourceCacheSnapshot() state (set by the write-back this test
// exercises) rather than replaying a cached result.
//
// Executing the "tg" leaf reveals what IsTruncated the write-back persisted:
//
//	IsTruncated=true (correct)  → checker returns {Count:0, Truncated:true} (honest lower bound)
//	IsTruncated=false (bug)     → checker returns {Count:0, Truncated:false} (wrong definitive zero)
func execRelatedCheckAndCollectTGResult(t *testing.T, m tui.Model) (result resource.RelatedCheckResult, found bool) {
	t.Helper()

	_, refreshCmd := rootApplyMsg(m, tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	if refreshCmd == nil {
		t.Fatal("Ctrl+R on ec2 detail returned nil cmd — expected batch of checker cmds")
	}

	for _, leaf := range extractLeafMsgs(refreshCmd) {
		if r, ok := leaf.(messages.RelatedCheckResult); ok && r.Result.TargetType() == "tg" {
			return r.Result, true
		}
	}
	return resource.UnknownRelated("tg"), false
}

// TestContract_EmptyTruncatedPage_PreservesIsTruncated is the core bug test for issue #233.
//
// Contract: When CachedPages carries {Resources:[], IsTruncated:true}, the write-back
// in app.go:378-387 MUST persist IsTruncated=true so that the next checker call
// (via buildResourceCacheSnapshot) returns {Count:0, Truncated:true}
// (relatedResultTrunc — the honest lower bound), not a definitive Count=0.
//
// New contract (Batch B): truncated-zero path → {Count:0, Truncated:true}.
// See related.go:34-38 (Truncated semantics) and TruncatedResult (related.go:101-114).
//
// Execution path:
//
//	RelatedCheckResultMsg{CachedPages:{"tg":{[],true}}} → app.go:378-387 write-back
//	→ Ctrl+R refresh → fresh DetailOperation → buildResourceCacheSnapshot()
//	→ checkEC2TargetGroups sees cache["tg"].IsTruncated → must be true → {Count:0, Truncated:true}
//
// This test FAILS with current code because app.go:383 guards persistence with
// `len(entry.Resources) > 0`, silently dropping IsTruncated on empty pages.
func TestContract_EmptyTruncatedPage_PreservesIsTruncated(t *testing.T) {
	m, ec2Res := setupLiveModeEC2Detail(t)
	firstInstance := ec2Res[0]

	// Step 1: Write-back. Feed RelatedCheckResultMsg with empty-but-truncated CachedPages.
	// This simulates what app_related.go:67-79 produces on a cold-miss first page
	// where the paginated fetcher returned an empty page with more pages behind it.
	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     "ec2",
		SourceResourceID: firstInstance.ID,
		Result:           resource.KnownRelated("tg", nil, true), // honest lower bound: page was truncated
		CachedPages: map[string]resource.ResourceCacheEntry{
			"tg": {
				Resources:   []resource.Resource{}, // empty page: zero TGs on this page
				IsTruncated: true,                  // but truncated: more pages exist
			},
		},
	})

	// Step 2: Trigger a fresh related check to observe what the write-back persisted.
	// Ctrl+R's checker fan-out calls buildResourceCacheSnapshot() which reads
	// m.ResourceCache["tg"].pagination to reconstruct IsTruncated.
	// If the write-back preserved it: IsTruncated=true → {Count:0, Truncated:true} (correct)
	// If the write-back dropped it:   IsTruncated=false → {Count:0, Truncated:false} (bug)
	got, found := execRelatedCheckAndCollectTGResult(t, m)
	if !found {
		t.Fatal("TG-related checker did not produce a RelatedCheckResultMsg — cannot verify write-back contract")
	}

	// EXPECTED after fix: {Count:0, Truncated:true} (IsTruncated=true preserved from write-back)
	// ACTUAL with bug:    {Count:0, Truncated:false} (IsTruncated dropped; treated as complete)
	if got.Count() != 0 {
		t.Errorf("BUG #233: empty-but-truncated write-back: want Count=0, got Count=%d", got.Count())
	}
	if !got.Truncated() {
		t.Fatalf("BUG #233: empty-but-truncated CachedPages write-back corrupted IsTruncated. " +
			"Expected TG checker Truncated=true (IsTruncated=true preserved from write-back), " +
			"got Truncated=false. " +
			"Root cause: app.go:383 guard `len(entry.Resources) > 0` drops pagination " +
			"when the first page is empty, so buildResourceCacheSnapshot reconstructs IsTruncated=false.")
	}
}

// TestContract_NonEmptyTruncatedPage_PreservesIsTruncated is a control test.
//
// Contract: When CachedPages has 1+ Resources AND IsTruncated=true, the write-back
// correctly persists IsTruncated (app.go:383 guard passes because len > 0).
// The TG checker must return {Count:0, Truncated:true} (relatedResultTrunc)
// when no match is found in the partial list. See related.go:34-38.
//
// This test PASSES with current code — the bug only affects the empty-page case.
func TestContract_NonEmptyTruncatedPage_PreservesIsTruncated(t *testing.T) {
	m, ec2Res := setupLiveModeEC2Detail(t)
	firstInstance := ec2Res[0]

	// Write-back: 1 resource + IsTruncated=true.
	// The TG has no relationship to firstInstance. Without truncation → Count=0.
	// With IsTruncated=true → {Count:0, Truncated:true} (honest lower bound).
	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     "ec2",
		SourceResourceID: firstInstance.ID,
		Result:           resource.KnownRelated("tg", nil, true),
		CachedPages: map[string]resource.ResourceCacheEntry{
			"tg": {
				Resources:   []resource.Resource{{ID: "tg-unrelated-ctrl-001"}},
				IsTruncated: true, // non-empty + truncated: app.go:383 guard passes
			},
		},
	})

	got, found := execRelatedCheckAndCollectTGResult(t, m)
	if !found {
		t.Fatal("TG-related checker did not produce a RelatedCheckResultMsg")
	}
	if got.Count() != 0 {
		t.Errorf("control: non-empty truncated cache must produce Count=0 (truncated lower bound); got Count=%d", got.Count())
	}
	if !got.Truncated() {
		t.Errorf("control: non-empty truncated cache must produce Truncated=true; got false")
	}
}

// TestContract_EmptyCompletePage_IsTruncatedFalse is a negative control test.
//
// Contract: When CachedPages has 0 Resources AND IsTruncated=false (complete list),
// the checker must return Count=0 (definitive: no related resources exist).
//
// This test PASSES with current code.
func TestContract_EmptyCompletePage_IsTruncatedFalse(t *testing.T) {
	m, ec2Res := setupLiveModeEC2Detail(t)
	firstInstance := ec2Res[0]

	// Write-back: empty + complete. Definitive zero — no TGs exist anywhere.
	m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
		ResourceType:     "ec2",
		SourceResourceID: firstInstance.ID,
		Result:           resource.KnownRelated("tg", nil, false),
		CachedPages: map[string]resource.ResourceCacheEntry{
			"tg": {
				Resources:   []resource.Resource{},
				IsTruncated: false, // complete: all TGs fetched, none found
			},
		},
	})

	got, found := execRelatedCheckAndCollectTGResult(t, m)
	if !found {
		t.Fatal("TG-related checker did not produce a RelatedCheckResultMsg")
	}
	if got.Count() != 0 {
		t.Errorf("negative control: complete empty cache must produce Count=0 (definitive zero); got Count=%d", got.Count())
	}
	if got.Truncated() {
		t.Errorf("negative control: complete empty cache must produce Truncated=false; got true")
	}
}

// TestContract_EmptyTruncatedPage_CheckerBehavior_Direct directly validates
// the TG checker's IsTruncated handling in isolation. This test confirms that
// the checker itself correctly returns {Count:0, Truncated:true}
// (relatedResultTrunc) on an empty-but-truncated entry, and a definitive
// {Count:0, Truncated:false} on an empty-but-complete entry.
// See related.go:34-38 and TruncatedResult (related.go:101-114).
//
// When this test passes but TestContract_EmptyTruncatedPage_PreservesIsTruncated fails,
// the bug is definitively in the write-back (app.go:383), not in the checker.
//
// This test PASSES with current code.
func TestContract_EmptyTruncatedPage_CheckerBehavior_Direct(t *testing.T) {
	ec2Client := fakes.NewEC2()
	ec2Res, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEC2InstancesPage(context.Background(), ec2Client, token)
	})
	if err != nil || len(ec2Res) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err, len(ec2Res))
	}
	instance := ec2Res[0]
	checker := ec2CheckerByTarget(t, "tg")

	// Subtest 1: empty + truncated → must return {Count:0, Truncated:true} (honest lower bound)
	truncatedEmptyCache := resource.ResourceCache{
		"tg": {
			Resources:   []resource.Resource{},
			IsTruncated: true,
		},
	}
	gotTruncated := checker(context.Background(), nil, instance, truncatedEmptyCache)
	if gotTruncated.Count() != 0 {
		t.Errorf("checker with empty+truncated cache must return Count=0; got Count=%d", gotTruncated.Count())
	}
	if !gotTruncated.Truncated() {
		t.Errorf("checker with empty+truncated cache must return Truncated=true; got false")
	}

	// Subtest 2: empty + complete → must return {Count:0, Truncated:false} (definitive zero)
	completeEmptyCache := resource.ResourceCache{
		"tg": {
			Resources:   []resource.Resource{},
			IsTruncated: false,
		},
	}
	gotComplete := checker(context.Background(), nil, instance, completeEmptyCache)
	if gotComplete.Count() != 0 {
		t.Errorf("checker with empty+complete cache must return Count=0; got Count=%d", gotComplete.Count())
	}
	if gotComplete.Truncated() {
		t.Errorf("checker with empty+complete cache must return Truncated=false; got true")
	}
}
