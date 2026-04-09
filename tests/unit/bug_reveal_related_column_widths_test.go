package unit

import (
	"context"
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
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
	widths := []int{60, 76, 95, 120, 160}
	for _, w := range widths {
		t.Run(fmt.Sprintf("width=%d", w), func(t *testing.T) {
			m := tui.New("demo", "us-east-1", tui.WithDemo(true))
			if initCmd := m.Init(); initCmd != nil {
				if initMsg := initCmd(); initMsg != nil {
					m2, _ := rootApplyMsg(m, initMsg)
					m = m2
				}
			}

			m2, _ := rootApplyMsg(m, tea.WindowSizeMsg{Width: w, Height: 36})
			m = m2

			clients := demo.NewServiceClients()
			ec2, err := awsclient.FetchEC2Instances(context.Background(), clients.EC2)
			if err != nil || len(ec2) == 0 {
				t.Fatalf("demo ec2 fixtures missing: err=%v len=%d", err, len(ec2))
			}
			m2, _ = rootApplyMsg(m, messages.NavigateMsg{
				Target:       messages.TargetDetail,
				ResourceType: "ec2",
				Resource:     &ec2[0],
			})
			m = m2

			view := stripANSI(rootViewContent(m))
			if !strings.Contains(view, "detail --") {
				t.Fatalf("precondition failed at width=%d: expected detail view; got:\n%s", w, view)
			}
			if !strings.Contains(view, "RELATED") {
				t.Errorf("BUG REVEALED: RELATED not visible at width=%d; got:\n%s", w, view)
			}
		})
	}
}
