package unit_test

import (
	"context"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

func makePreviewEC2Detail(t *testing.T, w, h int) views.DetailModel {
	t.Helper()
	ec2Client := fakes.NewEC2()
	ec2, err := awsclient.FetchEC2Instances(context.Background(), ec2Client)
	if err != nil || len(ec2) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err, len(ec2))
	}
	k := keys.Default()
	cfg := config.DefaultConfig()
	d := views.NewDetail(ec2[0], "ec2", cfg, k)
	d.SetSize(w, h)
	return d
}

func TestPreviewLeft_FirstSelectedRowShowsInstanceIDLabel(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	d := makePreviewEC2Detail(t, 120, 35)
	view := d.View()
	lines := strings.Split(view, "\n")
	if len(lines) == 0 {
		t.Fatal("detail view returned no lines")
	}

	first := lines[0]
	if !strings.Contains(first, "InstanceId:") {
		t.Errorf("selected first row must include 'InstanceId:' label; got first line:\n%q", stripAnsi(first))
	}
}

func TestPreviewLeft_SecurityGroupsSubFields_NoYAMLBullets(t *testing.T) {
	d := makePreviewEC2Detail(t, 120, 35)
	plain := stripAnsi(d.View())

	if strings.Contains(plain, "- GroupId:") {
		t.Errorf("SecurityGroups sub-fields should render as indented key/value rows, not YAML bullets; got:\n%s", plain)
	}
}

func TestPreviewLeft_IamInstanceProfile_RendersArnOnIndentedSubFieldLine(t *testing.T) {
	d := makePreviewEC2Detail(t, 120, 35)
	plain := stripAnsi(d.View())

	if !strings.Contains(plain, "IamInstanceProfile:") {
		t.Fatalf("precondition failed: expected IamInstanceProfile section in detail view; got:\n%s", plain)
	}
	if strings.Contains(plain, "IamInstanceProfile: Arn:") {
		t.Errorf("IamInstanceProfile should be a section header with Arn on its own indented line; got:\n%s", plain)
	}
	if !strings.Contains(plain, "Arn:") {
		t.Errorf("IamInstanceProfile section should include Arn sub-field line; got:\n%s", plain)
	}
}

func TestPreviewLeft_DetailTitleIncludesDetailContextAndResourceID(t *testing.T) {
	m := tui.New("demo", "us-east-1", tui.WithDemo(true))
	m2, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 35})
	m = m2.(tui.Model)

	ec2 := mustDemoEC2(t)
	m3, _ := m.Update(messages.NavigateMsg{
		Target:       messages.TargetDetail,
		ResourceType: "ec2",
		Resource:     &ec2[0],
	})
	m = m3.(tui.Model)
	plain := stripAnsi(m.View().Content)

	if !strings.Contains(plain, "detail --") {
		t.Errorf("detail frame title should include 'detail --' context; got:\n%s", plain)
	}
	if !strings.Contains(plain, ec2[0].ID) {
		t.Errorf("detail frame title should include resource id %q; got:\n%s", ec2[0].ID, plain)
	}
}

func mustDemoEC2(t *testing.T) []resource.Resource {
	t.Helper()
	ec2Client := fakes.NewEC2()
	ec2, err := awsclient.FetchEC2Instances(context.Background(), ec2Client)
	if err != nil || len(ec2) == 0 {
		t.Fatalf("demo ec2 fixtures missing (err=%v, len=%d)", err, len(ec2))
	}
	return ec2
}
