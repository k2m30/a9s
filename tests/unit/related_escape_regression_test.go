package unit

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// Regression guard for related navigation UX:
// from related-filtered list, Esc should return to source detail (not clear filter first).
func TestRelatedNavigate_FilteredList_EscReturnsToDetail(t *testing.T) {
	m := tui.New("demo", "us-east-1", tui.WithDemo(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})

	ec2Client := fakes.NewEC2()
	ec2, err := awsclient.FetchEC2Instances(context.Background(), ec2Client)
	if err != nil || len(ec2) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err, len(ec2))
	}
	amis, err := awsclient.FetchAMIs(context.Background(), ec2Client)
	if err != nil || len(amis) == 0 {
		t.Fatalf("demo ami fixtures missing (err=%v, len=%d)", err, len(amis))
	}

	// Source detail view.
	m, _ = rootApplyMsg(m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2[0],
	})

	// Open related target list using exact target ID (produces filtered title like ami(1/4)).
	imageID := amis[0].ID
	m, _ = rootApplyMsg(m, messages.RelatedNavigateMsg{
		TargetType:     "ami",
		SourceType:     "ec2",
		SourceResource: ec2[0],
		TargetID:       imageID,
	})

	// Simulate loaded target list.
	m, _ = rootApplyMsg(m, messages.ResourcesLoadedMsg{
		ResourceType: "ami",
		Resources:    amis,
	})

	beforeEsc := stripANSI(rootViewContent(m))
	if !strings.Contains(beforeEsc, "ami(1/") {
		t.Fatalf("expected related filtered list title before Esc; got:\n%s", beforeEsc)
	}

	// Esc should pop back to detail directly.
	m, _ = rootApplyMsg(m, rootSpecialKey(tea.KeyEscape))
	afterEsc := stripANSI(rootViewContent(m))
	if !strings.Contains(afterEsc, "detail --") {
		t.Fatalf("Esc from related filtered list should return to detail view; got:\n%s", afterEsc)
	}
}
