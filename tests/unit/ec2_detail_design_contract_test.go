package unit_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// TestEC2Detail_Render_MatchesApprovedDesignContract validates the current EC2
// detail rendering against the approved design contract in
// docs/design/related-resources-preview/ (mock #1: EC2 left-focused).
func TestEC2Detail_Render_MatchesApprovedDesignContract(t *testing.T) {
	m := tui.New("demo", "us-east-1", tui.WithDemo(true))
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 36})
	m = m2.(tui.Model)

	ec2Res := resource.Resource{
		ID:   "i-0abc123def456789a",
		Name: "web-prod",
		Fields: map[string]string{
			"InstanceId": "i-0abc123def456789a",
			"InstanceType": "t3.large",
			"State": "running",
			"VpcId": "vpc-0aaa111bbb222cc",
			"SubnetId": "subnet-0bbb222ccc333dd",
			"ImageId": "ami-0aaa111222333",
			"KeyName": "prod-keypair",
			"PrivateIpAddress": "10.0.48.175",
			"PublicIpAddress": "203.0.113.10",
			"LaunchTime": "2026-03-15 09:22:45",
			"Architecture": "x86_64",
			"SecurityGroups": "",
			"SecurityGroups.GroupId.0": "sg-0ccc333ddd444ee",
			"SecurityGroups.GroupName.0": "web-sg",
			"SecurityGroups.GroupId.1": "sg-0ddd444eee555ff",
			"SecurityGroups.GroupName.1": "db-access-sg",
			"IamInstanceProfile": "",
			"IamInstanceProfile.Arn": "arn:aws:iam::123456:role/web-role",
		},
	}

	m2, _ = m.Update(messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2Res,
	})
	m = m2.(tui.Model)

	view := stripAnsi(m.View().Content)
	lines := strings.Split(view, "\n")

	t.Run("title includes detail context", func(t *testing.T) {
		if !strings.Contains(view, "detail --") {
			t.Fatalf("expected frame title to include 'detail --'; got:\n%s", view)
		}
		if !strings.Contains(view, "i-0abc123") {
			t.Fatalf("expected frame title to include EC2 id context; got:\n%s", view)
		}
	})

	t.Run("first selected row is InstanceId", func(t *testing.T) {
		firstContent := ""
		for _, ln := range lines {
			if strings.Contains(ln, "InstanceId:") || strings.Contains(ln, "State:") || strings.Contains(ln, "InstanceType:") {
				firstContent = ln
				break
			}
		}
		if firstContent == "" {
			t.Fatalf("could not find content rows in detail view:\n%s", view)
		}
		if !strings.Contains(firstContent, "InstanceId:") {
			t.Fatalf("design expects first left-column row to be InstanceId; got first content row:\n%s", firstContent)
		}
	})

	t.Run("security groups render as indented sub-fields with YAML structure", func(t *testing.T) {
		if !strings.Contains(view, "SecurityGroups:") {
			t.Fatalf("missing SecurityGroups section header; got:\n%s", view)
		}
		if !strings.Contains(view, "GroupId:") {
			t.Fatalf("missing SecurityGroups GroupId sub-field; got:\n%s", view)
		}
		if !strings.Contains(view, "GroupName:") {
			t.Fatalf("missing SecurityGroups GroupName sub-field; got:\n%s", view)
		}
	})

	t.Run("iam instance profile arn on its own indented sub-field line", func(t *testing.T) {
		if !strings.Contains(view, "IamInstanceProfile:") {
			t.Fatalf("missing IamInstanceProfile section header; got:\n%s", view)
		}
		if !strings.Contains(view, "     Arn:") {
			t.Fatalf("design expects Arn as indented sub-field line; got:\n%s", view)
		}
		if strings.Contains(view, "IamInstanceProfile:   Arn:") {
			t.Fatalf("design forbids inline IamInstanceProfile+Arn rendering; got:\n%s", view)
		}
	})
}
