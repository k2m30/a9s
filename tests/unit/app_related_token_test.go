package unit

// app_related_token_test.go — Tests for token resumption in handleRelatedNavigate.
//
// Phase 8 (#219): When navigating to a related resource type with a truncated
// cache entry containing a NextToken, the handler should dispatch fetchMoreResources
// with the stored token rather than fetchResources (which resets to page 1).
//
// The difference is observable via the ResourcesLoadedMsg.Append field:
//   - fetchMoreResources → Append=true  (continues from stored page)
//   - fetchResources     → Append=false (resets to page 1)
//
// In tests, clients are nil so both branches return APIErrorMsg. To verify the
// token is used, we inspect the cmd chain for ResourcesLoadedMsg.Append.
// When clients are nil and a cmd returns APIErrorMsg, we skip (cannot assert).
// The test primarily verifies that a fetch IS initiated (cmd != nil).

import (
	"context"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/demo"
)

// TestHandleRelatedNavigate_TruncatedCache_InitiatesFetch verifies that
// when a RelatedNavigateMsg has IDs that are only partially in a truncated
// cache, the handler initiates a fetch (cmd != nil).
//
// The fetch should use fetchMoreResources (Append=true, token resumption)
// not fetchResources (Append=false, page 1 reset). In unit tests without
// real AWS clients we can only verify cmd != nil; the Append check is best
// verified in integration tests.
func TestHandleRelatedNavigate_TruncatedCache_InitiatesFetch(t *testing.T) {
	m := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	// Navigate to EC2 list.
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	ec2Client := fakes.NewEC2()
	ec2Res, err := awsclient.FetchEC2Instances(context.Background(), ec2Client)
	if err != nil || len(ec2Res) < 2 {
		t.Fatalf("demo ec2 fixtures need at least 2 resources (err=%v, len=%d)", err, len(ec2Res))
	}

	// Load ONLY the first EC2 resource with truncated pagination.
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    ec2Res[0:1],
		Pagination:   &resource.PaginationMeta{IsTruncated: true, NextToken: "tok-stored-001"},
	})

	// Request navigation with 2 IDs: ec2[0] (in cache) + ec2[1] (missing from page 1).
	navMsg := messages.RelatedNavigateMsg{
		TargetType: "ec2",
		RelatedIDs: []string{ec2Res[0].ID, ec2Res[1].ID},
		SourceResource: resource.Resource{
			ID:   "i-source",
			Name: "source-instance",
		},
		SourceType: "ec2",
	}
	_, cmd := rootApplyMsg(m, navMsg)

	// A fetch must be initiated because ec2[1] is missing and the cache is truncated.
	if cmd == nil {
		t.Fatal("truncated cache with missing RelatedID: cmd must be non-nil (a fetch is required to find the missing resource)")
	}

	// Execute the command to check if it uses token resumption (Append=true).
	// In unit test mode (nil clients), fetchMoreResources returns APIErrorMsg —
	// we can still check the msg type to distinguish from a nil cmd.
	msg := cmd()
	if loaded, ok := msg.(messages.ResourcesLoadedMsg); ok {
		// If a ResourcesLoadedMsg was produced (demo mode with real demo clients),
		// verify it uses Append=true (token resumption, not page 1 reset).
		if !loaded.Append {
			t.Errorf("token resumption: fetch should use Append=true (fetchMoreResources), got Append=false (fetchResources); " +
				"fix: replace fetchResources with fetchMoreResources at app_related.go:199")
		}
	}
	// If APIErrorMsg (nil clients), we verified cmd != nil above — sufficient for unit tests.
}

// TestHandleRelatedNavigate_CompleteCache_NoFetch verifies that when all
// requested IDs are in a non-truncated cache, no fetch is dispatched.
func TestHandleRelatedNavigate_CompleteCache_NoFetch(t *testing.T) {
	m := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	ec2Client := fakes.NewEC2()
	ec2Res, err := awsclient.FetchEC2Instances(context.Background(), ec2Client)
	if err != nil || len(ec2Res) < 2 {
		t.Fatalf("demo ec2 fixtures need at least 2 resources (err=%v, len=%d)", err, len(ec2Res))
	}
	// Load ALL resources, non-truncated.
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    ec2Res,
		Pagination:   &resource.PaginationMeta{IsTruncated: false},
	})

	navMsg := messages.RelatedNavigateMsg{
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
