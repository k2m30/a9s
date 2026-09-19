package unit

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

func TestBugReveal_EC2Detail_RelatedVisibleAcrossWidths(t *testing.T) {
	ec2 := mustDemoEC2(t)
	for _, w := range []int{60, 76, 95, 120, 160} {
		t.Run(fmt.Sprintf("width=%d", w), func(t *testing.T) {
			m := newBlessedModel(t, "demo", "us-east-1",
				tui.WithClients(demo.NewServiceClients()),
				tui.WithIsDemo(true),
				tui.WithNoCache(true),
				tui.WithProfileForTest(demo.DemoProfile),
				tui.WithRegionForTest(demo.DemoRegion))
			m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: w, Height: 36})
			m, _ = rootApplyMsg(m, messages.Navigate{
				Target:       messages.TargetDetail,
				ResourceType: "ec2",
				Resource:     &ec2[0],
			})
			view := stripANSI(rootViewContent(m))
			if !strings.Contains(view, "RELATED") {
				t.Errorf("RELATED not visible at width=%d; got:\n%s", w, view)
			}
		})
	}
}
