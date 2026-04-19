package unit_test

import (
	"context"
	"os"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/demo"
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

func TestPreviewLeft_SecurityGroupsSubFields_YAMLStructure(t *testing.T) {
	d := makePreviewEC2Detail(t, 120, 35)
	plain := stripAnsi(d.View())

	// Issue #265: detail view sub-fields should render with YAML structure
	// including dashes for array items, matching the YAML view's formatting.
	if !strings.Contains(plain, "GroupId:") {
		t.Errorf("SecurityGroups sub-fields should contain GroupId key; got:\n%s", plain)
	}

	// Verify hierarchical indentation: SecurityGroups sub-field lines must be
	// indented deeper than the section header (which sits at 1-space indent).
	inSG := false
	var subFieldIndents []int
	for line := range strings.SplitSeq(plain, "\n") {
		stripped := strings.TrimSpace(line)
		if strings.HasPrefix(stripped, "SecurityGroups:") {
			inSG = true
			continue
		}
		if inSG {
			if stripped == "" || (len(line) > 0 && line[0] != ' ') {
				break // left the section
			}
			spaces := len(line) - len(strings.TrimLeft(line, " "))
			if spaces <= 1 {
				break // hit next top-level field
			}
			if strings.Contains(stripped, ":") {
				subFieldIndents = append(subFieldIndents, spaces)
			}
		}
	}
	if len(subFieldIndents) == 0 {
		t.Errorf("expected SecurityGroups sub-field lines with indentation > 1; got:\n%s", plain)
	}
	for _, indent := range subFieldIndents {
		if indent < 5 {
			t.Errorf("SecurityGroups sub-field has indent %d (expected >= 5); got:\n%s", indent, plain)
		}
	}
}

// TestPreviewLeft_PlainTextIdenticalAcrossCursorPositions verifies that the
// plain-text form of every detail line is the same regardless of which row the
// cursor is on. This guards the search-indexing contract: detail search indexes
// ansi.Strip(renderContent()) and uses literal substring matching, so the plain
// text must not change when the cursor moves (e.g., no extra spaces on selected rows).
func TestPreviewLeft_PlainTextIdenticalAcrossCursorPositions(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	d := makePreviewEC2Detail(t, 120, 35)
	baseline := stripAnsi(d.View())
	baseLines := strings.Split(baseline, "\n")

	// Move cursor through every position and verify non-cursor lines stay identical.
	for step := range 20 {
		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
		current := stripAnsi(d.View())
		curLines := strings.Split(current, "\n")

		if len(curLines) != len(baseLines) {
			t.Fatalf("step %d: line count changed from %d to %d", step, len(baseLines), len(curLines))
		}

		cursor := d.FieldCursor()
		for i, got := range curLines {
			if i == cursor {
				continue // cursor row may differ (selection indicator)
			}
			if got != baseLines[i] {
				t.Errorf("step %d cursor=%d: line %d changed\n  baseline: %q\n  current:  %q",
					step, cursor, i, baseLines[i], got)
			}
		}
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
	m := tui.New("demo", "us-east-1",
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithNoCache(true),
		tui.WithProfile(demo.DemoProfile),
		tui.WithRegion(demo.DemoRegion))
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
