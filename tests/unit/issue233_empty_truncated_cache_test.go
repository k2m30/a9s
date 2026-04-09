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
// see IsTruncated=false and report Count=0 instead of Count=-1 (unknown).
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

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// setupLiveModeEC2Detail creates a NON-demo root model (so the real checker path runs),
// navigates to EC2 detail for the first fixture, and returns the model plus EC2 fixtures.
//
// Non-demo mode is required because demo mode's handleRelatedCheckStarted bypasses
// buildResourceCacheSnapshot() entirely and uses registered demo checkers instead.
// Only the live-mode path calls the real checker with the cache snapshot.
func setupLiveModeEC2Detail(t *testing.T) (tui.Model, []resource.Resource) {
	t.Helper()

	// Non-demo model: no WithDemo option.
	// This makes handleRelatedCheckStarted use the real checker path (not demo fixtures).
	m := tui.New("test-profile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	ec2Client := fakes.NewEC2()
	ec2Res, err := awsclient.FetchEC2Instances(context.Background(), ec2Client)
	if err != nil || len(ec2Res) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err, len(ec2Res))
	}

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    ec2Res,
	})

	// Enter first EC2 detail. In non-demo mode this produces RelatedCheckStartedMsg
	// which triggers live-mode checker dispatch. We drain but ignore those cmds —
	// we only care about the subsequent write-back path.
	m, firstCmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	m, _ = drainCmds(t, m, firstCmd, 3)

	return m, ec2Res
}

// execRelatedCheckAndCollectTGResult feeds a RelatedCheckStartedMsg to the model
// and synchronously executes all resulting cmds, collecting the "tg" RelatedCheckResultMsg.
// Returns (count, found).
//
// In non-demo mode, handleRelatedCheckStarted:
//  1. Calls buildResourceCacheSnapshot() — reads from m.resourceCache (set by write-back)
//  2. For each RelatedDef: if target is already in cache, calls the real checker directly
//  3. Returns a tea.BatchMsg of cmds, each returning RelatedCheckResultMsg
//
// Executing the "tg" cmd reveals what IsTruncated the write-back persisted:
//   IsTruncated=true (correct)  → checker returns Count=-1 (unknown)
//   IsTruncated=false (bug)     → checker returns Count=0 (wrong definitive zero)
func execRelatedCheckAndCollectTGResult(t *testing.T, m tui.Model, sourceResource resource.Resource) (count int, found bool) {
	t.Helper()

	_, batchCmd := rootApplyMsg(m, messages.RelatedCheckStartedMsg{
		ResourceType:   "ec2",
		SourceResource: sourceResource,
	})
	if batchCmd == nil {
		t.Fatal("handleRelatedCheckStarted returned nil cmd — expected batch of checker cmds")
	}

	// Execute the batch. tea.Batch returns a cmd that, when called, returns tea.BatchMsg.
	rawMsg := batchCmd()
	if rawMsg == nil {
		return -1, false
	}

	batchMsg, ok := rawMsg.(tea.BatchMsg)
	if !ok {
		// Single cmd, not a batch — handle it.
		if result, ok2 := rawMsg.(messages.RelatedCheckResultMsg); ok2 && result.Result.TargetType == "tg" {
			return result.Result.Count, true
		}
		return -1, false
	}

	for _, cmd := range batchMsg {
		if cmd == nil {
			continue
		}
		msg := cmd()
		if msg == nil {
			continue
		}
		if result, ok2 := msg.(messages.RelatedCheckResultMsg); ok2 && result.Result.TargetType == "tg" {
			return result.Result.Count, true
		}
	}
	return -1, false
}

// TestContract_EmptyTruncatedPage_PreservesIsTruncated is the core bug test for issue #233.
//
// Contract: When CachedPages carries {Resources:[], IsTruncated:true}, the write-back
// in app.go:378-387 MUST persist IsTruncated=true so that the next checker call
// (via buildResourceCacheSnapshot) returns Count=-1 (unknown), not Count=0.
//
// Execution path:
//   RelatedCheckResultMsg{CachedPages:{"tg":{[],true}}} → app.go:378-387 write-back
//   → RelatedCheckStartedMsg → handleRelatedCheckStarted → buildResourceCacheSnapshot()
//   → checkEC2TargetGroups sees cache["tg"].IsTruncated → must be true → Count=-1
//
// This test FAILS with current code because app.go:383 guards persistence with
// `len(entry.Resources) > 0`, silently dropping IsTruncated on empty pages.
func TestContract_EmptyTruncatedPage_PreservesIsTruncated(t *testing.T) {
	m, ec2Res := setupLiveModeEC2Detail(t)
	firstInstance := ec2Res[0]

	// Step 1: Write-back. Feed RelatedCheckResultMsg with empty-but-truncated CachedPages.
	// This simulates what app_related.go:67-79 produces on a cold-miss first page
	// where the paginated fetcher returned an empty page with more pages behind it.
	m, _ = rootApplyMsg(m, messages.RelatedCheckResultMsg{
		ResourceType:     "ec2",
		SourceResourceID: firstInstance.ID,
		Result: resource.RelatedCheckResult{
			TargetType: "tg",
			Count:      -1, // unknown because the page was truncated
		},
		CachedPages: map[string]resource.ResourceCacheEntry{
			"tg": {
				Resources:   []resource.Resource{}, // empty page: zero TGs on this page
				IsTruncated: true,                  // but truncated: more pages exist
			},
		},
	})

	// Step 2: Trigger a fresh related check to observe what the write-back persisted.
	// handleRelatedCheckStarted calls buildResourceCacheSnapshot() which reads
	// m.resourceCache["tg"].pagination to reconstruct IsTruncated.
	// If the write-back preserved it: IsTruncated=true → Count=-1 (correct)
	// If the write-back dropped it:   IsTruncated=false → Count=0 (bug)
	count, found := execRelatedCheckAndCollectTGResult(t, m, firstInstance)
	if !found {
		t.Fatal("TG-related checker did not produce a RelatedCheckResultMsg — cannot verify write-back contract")
	}

	// EXPECTED after fix: Count=-1 (empty page was truncated; can't conclude zero)
	// ACTUAL with bug:    Count=0 (IsTruncated dropped; empty list treated as complete)
	if count != -1 {
		t.Fatalf("BUG #233: empty-but-truncated CachedPages write-back corrupted IsTruncated. "+
			"Expected TG checker Count=-1 (unknown — IsTruncated=true preserved from write-back), "+
			"got Count=%d. "+
			"Root cause: app.go:383 guard `len(entry.Resources) > 0` drops pagination "+
			"when the first page is empty, so buildResourceCacheSnapshot reconstructs IsTruncated=false.",
			count)
	}
}

// TestContract_NonEmptyTruncatedPage_PreservesIsTruncated is a control test.
//
// Contract: When CachedPages has 1+ Resources AND IsTruncated=true, the write-back
// correctly persists IsTruncated (app.go:383 guard passes because len > 0).
// The TG checker must return Count=-1 when no match is found in the partial list.
//
// This test PASSES with current code — the bug only affects the empty-page case.
func TestContract_NonEmptyTruncatedPage_PreservesIsTruncated(t *testing.T) {
	m, ec2Res := setupLiveModeEC2Detail(t)
	firstInstance := ec2Res[0]

	// Write-back: 1 resource + IsTruncated=true.
	// The TG has no relationship to firstInstance. Without truncation → Count=0.
	// With IsTruncated=true → Count=-1 (can't rule out a match in unseen pages).
	m, _ = rootApplyMsg(m, messages.RelatedCheckResultMsg{
		ResourceType:     "ec2",
		SourceResourceID: firstInstance.ID,
		Result: resource.RelatedCheckResult{
			TargetType: "tg",
			Count:      -1,
		},
		CachedPages: map[string]resource.ResourceCacheEntry{
			"tg": {
				Resources:   []resource.Resource{{ID: "tg-unrelated-ctrl-001"}},
				IsTruncated: true, // non-empty + truncated: app.go:383 guard passes
			},
		},
	})

	count, found := execRelatedCheckAndCollectTGResult(t, m, firstInstance)
	if !found {
		t.Fatal("TG-related checker did not produce a RelatedCheckResultMsg")
	}
	if count != -1 {
		t.Errorf("control: non-empty truncated cache must produce Count=-1 (unknown); got Count=%d", count)
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
	m, _ = rootApplyMsg(m, messages.RelatedCheckResultMsg{
		ResourceType:     "ec2",
		SourceResourceID: firstInstance.ID,
		Result: resource.RelatedCheckResult{
			TargetType: "tg",
			Count:      0,
		},
		CachedPages: map[string]resource.ResourceCacheEntry{
			"tg": {
				Resources:   []resource.Resource{},
				IsTruncated: false, // complete: all TGs fetched, none found
			},
		},
	})

	count, found := execRelatedCheckAndCollectTGResult(t, m, firstInstance)
	if !found {
		t.Fatal("TG-related checker did not produce a RelatedCheckResultMsg")
	}
	if count != 0 {
		t.Errorf("negative control: complete empty cache must produce Count=0 (definitive zero); got Count=%d", count)
	}
}

// TestContract_EmptyTruncatedPage_CheckerBehavior_Direct directly validates
// the TG checker's IsTruncated handling in isolation. This test confirms that
// the checker itself correctly returns Count=-1 on an empty-but-truncated entry,
// and Count=0 on an empty-but-complete entry.
//
// When this test passes but TestContract_EmptyTruncatedPage_PreservesIsTruncated fails,
// the bug is definitively in the write-back (app.go:383), not in the checker.
//
// This test PASSES with current code.
func TestContract_EmptyTruncatedPage_CheckerBehavior_Direct(t *testing.T) {
	ec2Client := fakes.NewEC2()
	ec2Res, err := awsclient.FetchEC2Instances(context.Background(), ec2Client)
	if err != nil || len(ec2Res) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err, len(ec2Res))
	}
	instance := ec2Res[0]
	checker := ec2CheckerByTarget(t, "tg")

	// Subtest 1: empty + truncated → must return Count=-1 (unknown)
	truncatedEmptyCache := resource.ResourceCache{
		"tg": {
			Resources:   []resource.Resource{},
			IsTruncated: true,
		},
	}
	gotTruncated := checker(context.Background(), nil, instance, truncatedEmptyCache)
	if gotTruncated.Count != -1 {
		t.Errorf("checker with empty+truncated cache must return Count=-1; got Count=%d", gotTruncated.Count)
	}

	// Subtest 2: empty + complete → must return Count=0 (definitive zero)
	completeEmptyCache := resource.ResourceCache{
		"tg": {
			Resources:   []resource.Resource{},
			IsTruncated: false,
		},
	}
	gotComplete := checker(context.Background(), nil, instance, completeEmptyCache)
	if gotComplete.Count != 0 {
		t.Errorf("checker with empty+complete cache must return Count=0; got Count=%d", gotComplete.Count)
	}
}
