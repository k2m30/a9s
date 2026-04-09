package unit

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

func applyRootAndCmd(t *testing.T, m tui.Model, msg tea.Msg) tui.Model {
	t.Helper()
	next, cmd := rootApplyMsg(m, msg)
	if cmd != nil {
		produced := cmd()
		if produced != nil {
			next, _ = rootApplyMsg(next, produced)
		}
	}
	return next
}

// One-enter-away smoke: EC2 list -> Enter -> detail should immediately show RELATED.
func TestFirstScreen_EC2EnterToDetail_ShowsRelatedColumn(t *testing.T) {
	m := tui.New("demo", "us-east-1", tui.WithDemo(true))
	m = applyRootAndCmd(t, m, tea.WindowSizeMsg{Width: 120, Height: 36})
	m = applyRootAndCmd(t, m, messages.NavigateMsg{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	ec2Client := fakes.NewEC2()
	ec2, err := awsclient.FetchEC2Instances(context.Background(), ec2Client)
	if err != nil || len(ec2) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err, len(ec2))
	}
	m = applyRootAndCmd(t, m, messages.ResourcesLoadedMsg{
		ResourceType: "ec2",
		Resources:    ec2,
	})

	m = applyRootAndCmd(t, m, rootSpecialKey(tea.KeyEnter))
	view := stripANSI(rootViewContent(m))
	if !strings.Contains(view, "detail --") {
		t.Fatalf("expected detail view after Enter from ec2 list; got:\n%s", view)
	}
	if !strings.Contains(view, "RELATED") {
		t.Fatalf("expected RELATED column on first detail screen; got:\n%s", view)
	}
}

// One-enter-away navigation flow: detail Enter on ImageId -> related ami detail
// when there is exactly one related match; Esc must return to source detail in one hit.
func TestFirstScreen_DetailEnterRelatedList_EscReturnsToDetail(t *testing.T) {
	m := tui.New("demo", "us-east-1", tui.WithDemo(true))
	m = applyRootAndCmd(t, m, tea.WindowSizeMsg{Width: 120, Height: 36})

	ec2Client2 := fakes.NewEC2()
	ec2, err2 := awsclient.FetchEC2Instances(context.Background(), ec2Client2)
	if err2 != nil || len(ec2) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err2, len(ec2))
	}
	amis, err3 := awsclient.FetchAMIs(context.Background(), ec2Client2)
	if err3 != nil || len(amis) == 0 {
		t.Fatalf("demo ami fixtures missing (err=%v, len=%d)", err3, len(amis))
	}

	m = applyRootAndCmd(t, m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2[0],
	})

	// Move cursor to ImageId row (default EC2 detail: index 4).
	for range 4 {
		m = applyRootAndCmd(t, m, rootKeyPress("j"))
	}
	m = applyRootAndCmd(t, m, rootSpecialKey(tea.KeyEnter))

	// Simulate loaded target resources so single-match related navigation can resolve.
	m = applyRootAndCmd(t, m, messages.ResourcesLoadedMsg{
		ResourceType: "ami",
		Resources:    amis,
	})

	beforeEsc := stripANSI(rootViewContent(m))
	if !strings.Contains(beforeEsc, "detail --") || !strings.Contains(beforeEsc, "ami-") {
		t.Fatalf("expected ami detail after Enter on single-related ImageId; got:\n%s", beforeEsc)
	}

	m = applyRootAndCmd(t, m, rootSpecialKey(tea.KeyEscape))
	afterEsc := stripANSI(rootViewContent(m))
	if !strings.Contains(afterEsc, "detail --") {
		t.Fatalf("Esc from related list should return to source detail; got:\n%s", afterEsc)
	}
}

// Regression: if a detail route is invoked without ResourceType but with EC2-shaped
// fields, detail must still render as EC2 (including RELATED column).
func TestFirstScreen_DetailMissingType_StillShowsRelatedForEC2Shape(t *testing.T) {
	m := tui.New("demo", "us-east-1", tui.WithDemo(true))
	m = applyRootAndCmd(t, m, tea.WindowSizeMsg{Width: 120, Height: 36})

	ec2Client3 := fakes.NewEC2()
	ec2, err4 := awsclient.FetchEC2Instances(context.Background(), ec2Client3)
	if err4 != nil || len(ec2) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err4, len(ec2))
	}
	ec2Res := ec2[0]
	m = applyRootAndCmd(t, m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "",
		Resource:     &ec2Res,
	})

	view := stripANSI(rootViewContent(m))
	if !strings.Contains(view, "detail --") {
		t.Fatalf("expected detail view when opening EC2-shaped resource; got:\n%s", view)
	}
	if !strings.Contains(view, "RELATED") {
		t.Fatalf("expected RELATED column for EC2-shaped detail even when ResourceType is empty; got:\n%s", view)
	}
}

// Bug reveal: real-image navigation can fall into an empty AMI list when the
// target image exists in AWS but is not present in the owned-AMI list fetch.
func TestFirstScreen_DetailEnterExternalImageID_DoesNotEndInEmptyAMIList(t *testing.T) {
	m := tui.New("demo", "us-east-1", tui.WithDemo(true))
	m = applyRootAndCmd(t, m, tea.WindowSizeMsg{Width: 120, Height: 36})

	ec2Res := resource.Resource{
		ID:     "i-external-ami",
		Name:   "vpn-like-host",
		Status: "running",
		Fields: map[string]string{
			"InstanceId":         "i-external-ami",
			"State":              "running",
			"InstanceType":       "t3.large",
			"InstanceLifecycle":  "on-demand",
			"ImageId":            "ami-external-123",
			"VpcId":              "vpc-123",
			"SubnetId":           "subnet-123",
			"SecurityGroups":     "sg-123",
			"PrivateIpAddress":   "10.0.0.10",
			"PrivateDnsName":     "ip-10-0-0-10.internal",
			"PublicIpAddress":    "1.2.3.4",
			"IamInstanceProfile": "-",
		},
	}

	m = applyRootAndCmd(t, m, messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2Res,
	})

	for range 4 {
		m = applyRootAndCmd(t, m, rootKeyPress("j"))
	}
	m = applyRootAndCmd(t, m, rootSpecialKey(tea.KeyEnter))

	// Simulate the current live failure mode: the generic ami list fetch returns
	// no rows for a referenced external image ID.
	m = applyRootAndCmd(t, m, messages.ResourcesLoadedMsg{
		ResourceType: "ami",
		Resources:    nil,
	})

	view := stripANSI(rootViewContent(m))
	if strings.Contains(view, "No resources found") || strings.Contains(view, "ami(0)") {
		t.Fatalf("Enter on ImageId should not strand the user in an empty ami list for an exact target image ID; got:\n%s", view)
	}
}
