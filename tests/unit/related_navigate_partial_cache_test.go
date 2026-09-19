package unit

// Tests for the partial-cache /
// pagination contract in handleRelatedNavigate's multi-RelatedIDs branch
// (internal/tui/runtime_adapter_related.go, NavigationKindFilteredList +
// RelatedIDs path).
//
// The runtime is the sole decision-maker for fetch tasks
// (core/runtime/handlers_related.go relatedFetchTasks). The adapter must
// translate the emitted []TaskRequest into tea.Cmd values and never silently
// drop them, regardless of how much the cache already covers.

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// setupEC2ListWithTruncatedCache navigates to the EC2 list and loads only the
// first EC2 resource with IsTruncated=true, simulating a partial first page.
// Returns the model and the full EC2 resource list (for ID references).
func setupEC2ListWithTruncatedCache(t *testing.T) (tui.Model, []resource.Resource) {
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
	if err != nil || len(ec2Res) < 2 {
		t.Fatalf("demo ec2 fixtures need at least 2 resources (err=%v, len=%d)", err, len(ec2Res))
	}

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
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
	if err != nil || len(ec2Res) < 2 {
		t.Fatalf("demo ec2 fixtures need at least 2 resources (err=%v, len=%d)", err, len(ec2Res))
	}

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
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

func TestRelatedNavigate_PartialCache_Truncated_FetchesMissing(t *testing.T) {
	m, ec2Res := setupEC2ListWithTruncatedCache(t)

	navMsg := messages.RelatedNavigate{
		TargetType: "ec2",
		RelatedIDs: []string{ec2Res[0].ID, ec2Res[1].ID},
		SourceResource: resource.Resource{
			ID:   "i-source",
			Name: "source-instance",
		},
		SourceType: "ec2",
	}

	_, cmd := rootApplyMsg(m, navMsg)

	if !cmdIsNonNilFetch(cmd) {
		t.Fatal("BUG: RelatedNavigateMsg with partial truncated cache must initiate a fetch " +
			"(cmd should be non-nil) — missing IDs may be on later pages")
	}
}

func TestRelatedNavigate_AllRelatedIDs_InCache_NoFetch(t *testing.T) {
	m, ec2Res := setupEC2ListWithCompleteCache(t)

	navMsg := messages.RelatedNavigate{
		TargetType: "ec2",
		RelatedIDs: []string{ec2Res[0].ID, ec2Res[1].ID},
		SourceResource: resource.Resource{
			ID:   "i-source",
			Name: "source-instance",
		},
		SourceType: "ec2",
	}

	_, cmd := rootApplyMsg(m, navMsg)

	if cmd != nil {
		t.Fatal("all RelatedIDs are in a complete (non-truncated) cache — cmd should be nil " +
			"(no fetch needed for a pure cache hit)")
	}
}

// When any RelatedID is missing from cache and the cache cannot page further
// (Pagination == nil OR IsTruncated == false), the runtime emits a
// KindFetchResources task (core/runtime/handlers_related.go relatedFetchTasks).
// The adapter must honor it — a missing ID may genuinely not exist OR may
// simply not have been observed yet, and the runtime is the sole
// decision-maker. The pre-populated cached row stays visible while the fetch
// is in flight (the view is built before the fetch cmd is returned).

func TestRelatedNavigate_PartialCache_NotTruncated_FetchesFullList(t *testing.T) {
	m, ec2Res := setupEC2ListWithCompleteCache(t)

	navMsg := messages.RelatedNavigate{
		TargetType: "ec2",
		RelatedIDs: []string{ec2Res[0].ID, "ec2-nonexistent-xxxxxxxxxxx"},
		SourceResource: resource.Resource{
			ID:   "i-source",
			Name: "source-instance",
		},
		SourceType: "ec2",
	}

	_, cmd := rootApplyMsg(m, navMsg)

	if !cmdIsNonNilFetch(cmd) {
		t.Fatal("BUG: RelatedNavigateMsg with partial coverage on a non-truncated cache " +
			"must initiate a full re-fetch (cmd should be non-nil) — runtime is the SSOT " +
			"for the fetch decision and emits KindFetchResources here " +
			"(see core/runtime/handlers_related.go relatedFetchTasks)")
	}
}
