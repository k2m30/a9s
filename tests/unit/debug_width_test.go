package unit

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
)

func measureView(t *testing.T, label string, m tui.Model) {
	t.Helper()
	view := rootViewContent(m)
	maxW := 0
	widestLine := ""
	for _, line := range strings.Split(view, "\n") {
		if w := lipgloss.Width(line); w > maxW {
			maxW = w
			widestLine = line
		}
	}
	t.Logf("%s: maxW=%d widest=%q", label, maxW, stripANSI(widestLine[:min(len(widestLine), 80)]))
}

func min2(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func TestDebugWidth104(t *testing.T) {
	m := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 36})

	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "ec2",
	})

	clients := demo.NewServiceClients()
	ec2Res, err := awsclient.FetchEC2Instances(context.Background(), clients.EC2)
	if err != nil || len(ec2Res) == 0 {
		t.Fatalf("demo ec2 fixtures missing: err=%v len=%d", err, len(ec2Res))
	}

	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		ResourceType: "ec2",
		Resources:    ec2Res,
	})
	measureView(t, "AFTER ResourcesLoaded", m)

	// Enter → navigate into detail
	m, firstCmd := rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	measureView(t, "AFTER Enter", m)
	m, _ = drainCmds(t, m, firstCmd, 5)
	measureView(t, "AFTER drainCmds", m)

	// Feed results one by one
	defs := resource.GetRelated("ec2")
	t.Logf("EC2 has %d related defs", len(defs))
	for i, def := range defs {
		m, _ = rootApplyMsg(m, messages.RelatedCheckResult{
			ResourceType: "ec2",
			Result: resource.RelatedCheckResult{
				TargetType:  def.TargetType,
				Count:       2,
				ResourceIDs: []string{"related-id-1", "related-id-2"},
			},
		})
		view := rootViewContent(m)
		maxW := 0
		widestLine := ""
		for _, line := range strings.Split(view, "\n") {
			if w := lipgloss.Width(line); w > maxW {
				maxW = w
				widestLine = line
			}
		}
		strip := stripANSI(widestLine)
		if len(strip) > 80 {
			strip = strip[:80]
		}
		t.Logf("After result[%d] %s: maxW=%d widest=%q", i, def.TargetType, maxW, strip)
		if maxW > 80 {
			t.Logf("  -> OVERFLOW at result[%d]", i)
			break
		}
	}
}
