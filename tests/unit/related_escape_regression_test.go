package unit

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

// Regression guard for related navigation UX:
// from related-filtered list, Esc should return to source detail (not clear filter first).
func TestRelatedNavigate_FilteredList_EscReturnsToDetail(t *testing.T) {
	m := newBlessedModel(t, "demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfileForTest(demo.DemoProfile),
		tui.WithRegionForTest(demo.DemoRegion))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	ec2Client := fakes.NewEC2()
	ec2, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchEC2InstancesPage(context.Background(), ec2Client, token)
	})
	if err != nil || len(ec2) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err, len(ec2))
	}
	amis, err := collectAllPages(func(token string) (resource.FetchResult, error) {
		return awsclient.FetchAMIsPage(context.Background(), ec2Client, token)
	})
	if err != nil || len(amis) == 0 {
		t.Fatalf("demo ami fixtures missing (err=%v, len=%d)", err, len(amis))
	}

	// Source detail view.
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2[0],
	})

	// Open related target list using exact target ID (produces filtered title like ami(1/4)).
	imageID := amis[0].ID
	m, _ = rootApplyMsg(m, messages.RelatedNavigate{
		TargetType:     "ami",
		SourceType:     "ec2",
		SourceResource: ec2[0],
		TargetID:       imageID,
	})

	// Simulate loaded target list.
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceByID,
		ResourceType: "ami",
		Resources:    amis,
	})

	beforeEsc := stripANSI(rootViewContent(m))
	// A related drill's count is what its own check named, not the type's
	// population with the drill's rows filtered out of it: this screen holds
	// one AMI because one AMI is what the instance was built from. An
	// "ami(1/4)" would read as a filter over the account's four AMIs, which
	// is the same claim that would let a refreshed drill title ten instances
	// as the account's whole fleet.
	if !strings.Contains(beforeEsc, "ami(1)") {
		t.Fatalf("expected the related drill's own count in the title before Esc; got:\n%s", beforeEsc)
	}

	// Esc should pop back to detail directly.
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))
	afterEsc := stripANSI(rootViewContent(m))
	if !strings.Contains(afterEsc, "detail --") {
		t.Fatalf("Esc from related filtered list should return to detail view; got:\n%s", afterEsc)
	}
}
