package unit

// issue233_empty_truncated_cache_test.go — an empty-but-truncated cold-miss
// page keeps its pagination metadata.
//
// When a cold-cache fetch returns zero resources but IsTruncated=true, the
// cache write-back must persist IsTruncated so buildResourceCacheSnapshot()
// reconstructs it and every subsequent related check reports the honest
// lower bound {Count:0, Truncated:true} (relatedResultTrunc), never a
// definitive Count=0. See related.go and ValidateRelatedResult.

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
// detail screen and collects the "tg" RelatedCheckResult from the resulting
// (possibly nested) tea.Batch. Returns (result, found).
//
// handleActionRefresh (core/app/actions_list.go) invalidates RelatedCache and
// begins a fresh DetailOperation unconditionally on Ctrl+R, so the checker
// runs against the current buildResourceCacheSnapshot() state rather than
// replaying a cached result. The "tg" leaf shows what IsTruncated the
// write-back persisted:
//
//	IsTruncated=true  → {Count:0, Truncated:true} (honest lower bound)
//	IsTruncated=false → {Count:0, Truncated:false} (wrong definitive zero)
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

// TestContract_EmptyTruncatedPage_PreservesIsTruncated is the core case:
// when CachedPages carries {Resources:[], IsTruncated:true}, the write-back
// MUST persist IsTruncated=true so that the next checker call (via
// buildResourceCacheSnapshot) returns {Count:0, Truncated:true}
// (relatedResultTrunc — the honest lower bound), not a definitive Count=0.
//
// Execution path:
//
//	RelatedCheckResultMsg{CachedPages:{"tg":{[],true}}} → write-back
//	→ Ctrl+R refresh → fresh DetailOperation → buildResourceCacheSnapshot()
//	→ checkEC2TargetGroups sees cache["tg"].IsTruncated → must be true → {Count:0, Truncated:true}
func TestContract_EmptyTruncatedPage_PreservesIsTruncated(t *testing.T) {
	m, ec2Res := setupLiveModeEC2Detail(t)
	firstInstance := ec2Res[0]

	// A cold-miss first page whose paginated fetcher returned an empty page with
	// more pages behind it.
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

	// Ctrl+R's checker fan-out calls buildResourceCacheSnapshot(), which reads
	// m.ResourceCache["tg"].pagination to reconstruct IsTruncated.
	got, found := execRelatedCheckAndCollectTGResult(t, m)
	if !found {
		t.Fatal("TG-related checker did not produce a RelatedCheckResultMsg — cannot verify write-back contract")
	}

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

// TestContract_NonEmptyTruncatedPage_PreservesIsTruncated is a control: with
// 1+ Resources and IsTruncated=true, the TG checker returns {Count:0,
// Truncated:true} (relatedResultTrunc) when no match is found in the partial
// list.
func TestContract_NonEmptyTruncatedPage_PreservesIsTruncated(t *testing.T) {
	m, ec2Res := setupLiveModeEC2Detail(t)
	firstInstance := ec2Res[0]

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

// TestContract_EmptyCompletePage_IsTruncatedFalse is a negative control: with
// 0 Resources and IsTruncated=false (a complete list) the checker returns the
// definitive Count=0.
func TestContract_EmptyCompletePage_IsTruncatedFalse(t *testing.T) {
	m, ec2Res := setupLiveModeEC2Detail(t)
	firstInstance := ec2Res[0]

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

// TestContract_EmptyTruncatedPage_CheckerBehavior_Direct validates the TG
// checker's IsTruncated handling in isolation from the write-back:
// {Count:0, Truncated:true} (relatedResultTrunc) on an empty-but-truncated
// entry, and a definitive {Count:0, Truncated:false} on an empty-but-complete
// entry.
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
