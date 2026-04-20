package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// Reveals regression from real user journey:
// main menu -> filter "ec2" -> Enter list -> Enter detail.
// At wide width, EC2 detail MUST show RELATED column.
func TestBugReveal_MainMenuToEC2Detail_MustShowRelatedColumn(t *testing.T) {
	for _, profile := range []string{"demo", "test-profile"} {
		t.Run("profile="+profile, func(t *testing.T) {
			m := tui.New(profile, "us-east-1",
				tui.WithClients(demo.NewServiceClients()),
				tui.WithIsDemo(true),
				tui.WithNoCache(true),
				tui.WithProfile(profile),
				tui.WithRegion(demo.DemoRegion))
			m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 170, Height: 50})
			ec2 := mustDemoEC2(t)
			m, _ = rootApplyMsg(m, messages.NavigateMsg{
				Target:       messages.TargetDetail,
				ResourceType: "ec2",
				Resource:     &ec2[0],
			})
			view := stripANSI(rootViewContent(m))
			if !strings.Contains(view, "RELATED") {
				t.Fatalf("EC2 detail missing RELATED column with profile=%s; got:\n%s", profile, view)
			}
		})
	}
}
