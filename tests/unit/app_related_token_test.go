package unit

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

// Without AWS clients both fetch paths return APIErrorMsg, so only cmd != nil
// is observable here.
func TestHandleRelatedNavigate_TruncatedCache_InitiatesFetch(t *testing.T) {
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
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-stored-001"},
	})

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

	if cmd == nil {
		t.Fatal("truncated cache with missing RelatedID: cmd must be non-nil (a fetch is required to find the missing resource)")
	}

	msg := cmd()
	if loaded, ok := msg.(messages.ResourcesLoaded); ok {
		if !loaded.Append {
			t.Errorf("token resumption: fetch should use Append=true (fetchMoreResources), got Append=false (fetchResources); " +
				"fix: replace fetchResources with fetchMoreResources at app_related.go:199")
		}
	}
}

func TestHandleRelatedNavigate_CompleteCache_NoFetch(t *testing.T) {
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
		t.Fatal("complete (non-truncated) cache with all IDs present: cmd should be nil, no fetch needed")
	}
}
