package unit

// related_navigate_cache_enter_child_test.go — Pin for the "single-result
// auto-drill mirrors Enter" invariant on the CACHE-HIT fast path.
//
// Rule (user, 2026-04-24): a related pivot that narrows to Count=1 must do
// EXACTLY what pressing Enter would do in the target type's list view —
// if the type registers Children[Key="enter"], drill INTO the child view;
// otherwise open the generic detail view. The slow path (cache miss →
// NavigationKindFilteredList + autoOpenSingleDetail) has been doing this since
// commit e6dfbc9 via (ResourceListModel).enterChildFor. The fast path
// (cache hit → NavigationKindDetail in internal/runtime/handlers_related.go)
// was never updated and silently stranded the operator on the generic detail
// when the target cache was already populated.
//
// This test pins the fast-path fix using s3 (Children[Key="enter"] →
// s3_objects, ContextKeys={"bucket":"ID"}).

import (
	"context"
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
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

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})

	s3Client := fakes.NewS3()
	s3Res, err := awsclient.FetchS3Buckets(context.Background(), s3Client)
	if err != nil || len(s3Res) == 0 {
		t.Fatalf("demo s3 fixtures missing (err=%v, len=%d)", err, len(s3Res))
	}

	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "s3",
		Resources:    s3Res,
	})

	return m, s3Res
}

// containsEnterChildViewMsg returns true when any message in msgs is an
// EnterChildViewMsg for the given child type.
func containsEnterChildViewMsg(msgs []tea.Msg, childType string) (messages.EnterChildViewMsg, bool) {
	for _, msg := range msgs {
		if m, ok := msg.(messages.EnterChildViewMsg); ok && m.ChildType == childType {
			return m, true
		}
	}
	return messages.EnterChildViewMsg{}, false
}

// containsNavigateMsgTargetDetail reports whether any message in msgs is a
// NavigateMsg{Target: TargetDetail} for the given resource type. Used by the
// negative assertion: when the target registers an enter-child, the fast
// path must NOT emit a plain detail navigation.
func containsNavigateMsgTargetDetail(msgs []tea.Msg, resourceType string) bool {
	for _, msg := range msgs {
		if m, ok := msg.(messages.NavigateMsg); ok &&
			m.Target == messages.TargetDetail &&
			m.ResourceType == resourceType {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Pin: RelatedNavigateMsg with a single cached RelatedID targeting s3 must
// dispatch EnterChildViewMsg{ChildType:"s3_objects"}, NOT a plain detail
// NavigateMsg. Mirrors (ResourceListModel).enterChildFor in the fast path.
// ---------------------------------------------------------------------------

func TestRelatedNavigate_CacheHit_SingleRelatedID_S3_EntersChildView(t *testing.T) {
	m, s3Res := setupS3ListWithCache(t)
	if len(s3Res) == 0 {
		t.Fatal("no s3 fixtures loaded")
	}
	bucket := s3Res[0]

	navMsg := messages.RelatedNavigateMsg{
		TargetType: "s3",
		RelatedIDs: []string{bucket.ID},
		SourceResource: resource.Resource{
			ID:   "source-resource",
			Name: "source",
		},
		SourceType: "cfn",
	}

	_, cmd := rootApplyMsg(m, navMsg)
	if cmd == nil {
		t.Fatal("RelatedNavigateMsg returned nil cmd; expected EnterChildViewMsg for s3_objects")
	}
	_, msgs := drainCmds(t, m, cmd, 4)

	ecv, ok := containsEnterChildViewMsg(msgs, "s3_objects")
	if !ok {
		types := make([]string, len(msgs))
		for i, msg := range msgs {
			types[i] = fmt.Sprintf("%T", msg)
		}
		t.Fatalf("expected EnterChildViewMsg{ChildType:\"s3_objects\"} on cache-hit single-RelatedID drill into s3; got: %v",
			types)
	}
	if got := ecv.ParentContext["bucket"]; got != bucket.ID {
		t.Errorf("EnterChildViewMsg.ParentContext[\"bucket\"] = %q, want %q (s3 Children ContextKeys {\"bucket\":\"ID\"})",
			got, bucket.ID)
	}
	if containsNavigateMsgTargetDetail(msgs, "s3") {
		t.Errorf("fast path also emitted NavigateMsg{TargetDetail, s3} — must not push detail when an enter-child exists; got messages: %v", msgs)
	}
}

// ---------------------------------------------------------------------------
// Pin: RelatedNavigateMsg with TargetID on s3 (cache hit) also enters child.
// Covers the TargetID path of the NavigationKindDetail branch
// (internal/runtime/handlers_related.go) in addition to the single-RelatedID path.
// ---------------------------------------------------------------------------

func TestRelatedNavigate_CacheHit_TargetID_S3_EntersChildView(t *testing.T) {
	m, s3Res := setupS3ListWithCache(t)
	bucket := s3Res[0]

	navMsg := messages.RelatedNavigateMsg{
		TargetType: "s3",
		TargetID:   bucket.ID,
		SourceResource: resource.Resource{
			ID:   "source-resource",
			Name: "source",
		},
		SourceType: "cfn",
	}

	_, cmd := rootApplyMsg(m, navMsg)
	if cmd == nil {
		t.Fatal("RelatedNavigateMsg{TargetID} returned nil cmd")
	}
	_, msgs := drainCmds(t, m, cmd, 4)

	ecv, ok := containsEnterChildViewMsg(msgs, "s3_objects")
	if !ok {
		types := make([]string, len(msgs))
		for i, msg := range msgs {
			types[i] = fmt.Sprintf("%T", msg)
		}
		t.Fatalf("expected EnterChildViewMsg{ChildType:\"s3_objects\"} on cache-hit TargetID drill into s3; got: %v",
			types)
	}
	if got := ecv.ParentContext["bucket"]; got != bucket.ID {
		t.Errorf("EnterChildViewMsg.ParentContext[\"bucket\"] = %q, want %q", got, bucket.ID)
	}
	if containsNavigateMsgTargetDetail(msgs, "s3") {
		t.Errorf("fast path also emitted NavigateMsg{TargetDetail, s3} — must not push detail when an enter-child exists")
	}
}
