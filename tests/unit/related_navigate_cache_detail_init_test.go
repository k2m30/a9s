package unit

// HandleRelatedNavigate's
// cache-hit branch (TargetID or a single RelatedID matching a cached resource)
// pushes a detail view and dispatches the related-check task, so the right
// column leaves its loading state.

import (
	"context"
	"fmt"
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

// setupEC2ListWithCache navigates to the EC2 list, loads all EC2 resources, and
// returns the model with the resourceCache populated for "ec2".
func setupEC2ListWithCache(t *testing.T) (tui.Model, []resource.Resource) {
	t.Helper()

	m := newBlessedModel(t, "demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	ec2Client := fakes.NewEC2()
	ec2Res, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEC2InstancesPage(context.Background(), ec2Client, token)
	})
	if err != nil || len(ec2Res) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err, len(ec2Res))
	}

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: "ec2",
		Resources:    ec2Res,
	})

	return m, ec2Res
}

// containsRelatedCheckResultMsg returns true if any message in msgs is a
// RelatedCheckResult — the signal that the related-check fan-out actually
// dispatched and ran.
func containsRelatedCheckResultMsg(msgs []tea.Msg) bool {
	for _, msg := range msgs {
		if _, ok := msg.(messages.RelatedCheckResult); ok {
			return true
		}
	}
	return false
}

func TestRelatedNavigate_CachedTargetID_DispatchesRelatedCheck(t *testing.T) {
	defs := resource.GetRelated("ec2")
	if len(defs) == 0 {
		t.Fatal("no ec2 related defs registered — core/aws import should register them")
	}

	m, ec2Res := setupEC2ListWithCache(t)

	// Send a RelatedNavigateMsg using TargetID — the cache-hit branch fires.
	navMsg := messages.RelatedNavigate{
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
			"must dispatch the related-check fan-out so the detail right column loads")
	}

	_, msgs := drainCmds(t, m, cmd, 3)

	if !containsRelatedCheckResultMsg(msgs) {
		types := make([]string, len(msgs))
		for i, msg := range msgs {
			types[i] = fmt.Sprintf("%T", msg)
		}
		t.Fatalf("BUG: RelatedNavigateMsg (cached TargetID) must produce a RelatedCheckResult "+
			"in cmd chain; got: %v", types)
	}
}

// A repeat RelatedNavigate shows cached counts immediately: openRelatedDetail
// (core/app/navigate.go) merges RelatedCacheGet's entries into the freshly
// pushed DetailState.

func TestRelatedNavigate_CachedTargetID_UsesCachedResults(t *testing.T) {
	defs := resource.GetRelated("ec2")
	if len(defs) == 0 {
		t.Fatal("no ec2 related defs registered — core/aws import should register them")
	}

	m := setupEC2DetailWithResults(t)

	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))

	ec2Client2 := fakes.NewEC2()
	ec2Res, err2 := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEC2InstancesPage(context.Background(), ec2Client2, token)
	})
	if err2 != nil || len(ec2Res) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err2, len(ec2Res))
	}

	// Navigate again to the same resource via RelatedNavigateMsg (cache-hit branch).
	navMsg := messages.RelatedNavigate{
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

	if !strings.Contains(view, "(7)") {
		t.Fatalf("BUG: after RelatedNavigateMsg to a resource with cached related results, "+
			"the detail view must immediately show '(7)' — but it was not found.\nView:\n%s", view)
	}
}

func TestRelatedNavigate_SingleRelatedID_CacheHit_DispatchesRelatedCheck(t *testing.T) {
	defs := resource.GetRelated("ec2")
	if len(defs) == 0 {
		t.Fatal("no ec2 related defs registered — core/aws import should register them")
	}

	m, ec2Res := setupEC2ListWithCache(t)

	// Single RelatedID — uses the len==1 branch in handleRelatedNavigate.
	navMsg := messages.RelatedNavigate{
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
			"must dispatch the related-check fan-out so the detail right column loads")
	}

	_, msgs := drainCmds(t, m, cmd, 3)

	if !containsRelatedCheckResultMsg(msgs) {
		types := make([]string, len(msgs))
		for i, msg := range msgs {
			types[i] = fmt.Sprintf("%T", msg)
		}
		t.Fatalf("BUG: RelatedNavigateMsg (single cached RelatedID) must produce "+
			"a RelatedCheckResult in cmd chain; got: %v", types)
	}
}
