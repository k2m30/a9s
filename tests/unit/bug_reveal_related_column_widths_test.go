package unit

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// TestBugReveal_EC2Detail_RelatedVisibleAcrossWidths verifies the RELATED
// panel remains visible on the EC2 detail view across a spectrum of terminal
// widths. Widths 76 and 95 were previously covered by two separate bug-reveal
// files (see issue #248 Batch 3a); 60 (minimum supported boundary), 120, and
// 160 are additional probes. Width 40 is below the app's 60-column floor and
// shows "Terminal too narrow" instead of the detail view, so it is excluded.
func TestBugReveal_EC2Detail_RelatedVisibleAcrossWidths(t *testing.T) {
	ec2 := mustDemoEC2(t)
	for _, w := range []int{60, 76, 95, 120, 160} {
		t.Run(fmt.Sprintf("width=%d", w), func(t *testing.T) {
			m := tui.New("demo", "us-east-1",
				tui.WithClients(demo.NewServiceClients()),
				tui.WithIsDemo(true),
				tui.WithNoCache(true),
				tui.WithProfile(demo.DemoProfile),
				tui.WithRegion(demo.DemoRegion))
			m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: w, Height: 36})
			m, _ = rootApplyMsg(m, messages.NavigateMsg{
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
