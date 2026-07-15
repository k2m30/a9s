package unit

// related_navigate_cache_enter_child_test.go — Pin for the "single-result
// pivot always opens the target's DETAIL view" invariant on the CACHE-HIT
// fast path.
//
// Rule (owner, 2026-07-06 — supersedes the 2026-04-24 rule): a related pivot
// that narrows to exactly ONE resource must open that resource's DETAIL view
// (fields + related), for EVERY target type, in both lanes (TUI and web) —
// never the target's enter-keyed child view. The old rule ("do exactly what
// Enter would do" — child view for ~19 types with Children[Key="enter"])
// made the web lane diverge from the TUI lane, since the web lane always
// rendered detail. Child views stay reachable exactly as before by pressing
// Enter inside the target's own list.
//
// This test pins the fast-path fix using s3 (Children[Key="enter"] →
// s3_objects, ContextKeys={"bucket":"ID"}): a cache-hit pivot to s3 must land
// on the s3 bucket's DETAIL view, not s3_objects.

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// setupS3ListWithCache primes the root model's resourceCache["s3"] so a
// subsequent RelatedNavigateMsg with a matching RelatedID will take the
// NavigationKindDetail cache-hit branch.
func setupS3ListWithCache(t *testing.T) (tui.Model, []resource.Resource) {
	t.Helper()

	m := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})

	s3Client := fakes.NewS3()
	s3Res, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchS3BucketsPage(context.Background(), s3Client, token)
	})
	if err != nil || len(s3Res) == 0 {
		t.Fatalf("demo s3 fixtures missing (err=%v, len=%d)", err, len(s3Res))
	}

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "s3",
		Resources:    s3Res,
	})

	return m, s3Res
}

// containsEnterChildViewMsg returns true when any message in msgs is an
// EnterChildViewMsg for the given child type. Retained for the negative
// assertion below: the new rule requires this NEVER fires on the related-
// panel Count=1 pivot.
func containsEnterChildViewMsg(msgs []tea.Msg, childType string) (messages.EnterChildView, bool) {
	for _, msg := range msgs {
		if m, ok := msg.(messages.EnterChildView); ok && m.ChildType == childType {
			return m, true
		}
	}
	return messages.EnterChildView{}, false
}

// ---------------------------------------------------------------------------
// Pin: RelatedNavigateMsg with a single cached RelatedID targeting s3 must
// land on the s3 bucket's DETAIL view (frame title "detail -- <id>"), NOT
// the s3_objects child view.
// ---------------------------------------------------------------------------

func TestRelatedNavigate_CacheHit_SingleRelatedID_S3_OpensDetail(t *testing.T) {
	m, s3Res := setupS3ListWithCache(t)
	if len(s3Res) == 0 {
		t.Fatal("no s3 fixtures loaded")
	}
	bucket := s3Res[0]

	navMsg := messages.RelatedNavigate{
		TargetType: "s3",
		RelatedIDs: []string{bucket.ID},
		SourceResource: resource.Resource{
			ID:   "source-resource",
			Name: "source",
		},
		SourceType: "cfn",
	}

	m, cmd := rootApplyMsg(m, navMsg)
	_, msgs := drainCmds(t, m, cmd, 4)

	view := stripANSI(rootViewContent(m))
	if !strings.Contains(view, "detail -- "+bucket.ID) {
		t.Fatalf("expected s3 bucket DETAIL view (\"detail -- %s\") on cache-hit single-RelatedID pivot into s3; got:\n%s",
			bucket.ID, view)
	}
	if _, ok := containsEnterChildViewMsg(msgs, "s3_objects"); ok {
		t.Errorf("fast path emitted EnterChildViewMsg{ChildType:\"s3_objects\"} — a related-panel Count=1 pivot must open detail, not the enter-child view")
	}
}

// ---------------------------------------------------------------------------
// Pin: RelatedNavigateMsg with TargetID on s3 (cache hit) also opens detail.
// Covers the TargetID path of the NavigationKindDetail branch
// (core/runtime/handlers_related.go) in addition to the single-RelatedID path.
// ---------------------------------------------------------------------------

func TestRelatedNavigate_CacheHit_TargetID_S3_OpensDetail(t *testing.T) {
	m, s3Res := setupS3ListWithCache(t)
	bucket := s3Res[0]

	navMsg := messages.RelatedNavigate{
		TargetType: "s3",
		TargetID:   bucket.ID,
		SourceResource: resource.Resource{
			ID:   "source-resource",
			Name: "source",
		},
		SourceType: "cfn",
	}

	m, cmd := rootApplyMsg(m, navMsg)
	_, msgs := drainCmds(t, m, cmd, 4)

	view := stripANSI(rootViewContent(m))
	if !strings.Contains(view, "detail -- "+bucket.ID) {
		t.Fatalf("expected s3 bucket DETAIL view (\"detail -- %s\") on cache-hit TargetID pivot into s3; got:\n%s",
			bucket.ID, view)
	}
	if _, ok := containsEnterChildViewMsg(msgs, "s3_objects"); ok {
		t.Errorf("fast path emitted EnterChildViewMsg{ChildType:\"s3_objects\"} — a related-panel Count=1 pivot must open detail, not the enter-child view")
	}
}
