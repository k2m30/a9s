package unit_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
)

// Regression guard: selected rows must keep labels readable and avoid carrying
// navigable underline into the selected state.
func TestDetail_SelectedRow_LabelVisible_NoNestedKeyTint(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	d := makePreviewEC2Detail(t, 120, 35)
	line := findLineContaining(d.View(), "InstanceId:")
	if line == "" {
		t.Fatalf("selected row line not found")
	}
	if !strings.Contains(line, "\x1b[48;") {
		t.Fatalf("selected row must include background highlight; line=%q", line)
	}
	if strings.Contains(line, "\x1b[38;2;122;162;247mInstanceId:") {
		t.Fatalf("selected row must not render key with default key tint (low contrast risk); line=%q", line)
	}
}

func TestDetail_SelectedNavigableRow_DropsUnderline(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	styles.Reinit()
	t.Cleanup(styles.Reinit)
	withIssue140EC2RelatedDefs(t)

	d := makePreviewEC2Detail(t, 120, 35)
	// Move to VpcId.
	for range 80 {
		if strings.Contains(findSelectedLine(d.View()), "VpcId:") {
			break
		}
		d, _ = d.Update(tea.KeyPressMsg{Code: -1, Text: "j"})
	}

	line := findSelectedLine(d.View())
	if line == "" {
		t.Fatalf("VpcId row not found")
	}
	if !strings.Contains(line, "VpcId:") {
		t.Fatalf("selected line should be VpcId row; line=%q", line)
	}
	if strings.Contains(line, "\x1b[4;") || strings.Contains(line, "\x1b[4m") {
		t.Fatalf("selected navigable row must not be underlined; line=%q", line)
	}
}

func TestDetail_SelectedRow_FillsViewportWidth(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	styles.Reinit()
	t.Cleanup(styles.Reinit)

	// Disable related to keep single-column width deterministic.
	oldDefs := resource.GetRelated("ec2")
	resource.RegisterRelated("ec2", nil)
	t.Cleanup(func() { resource.RegisterRelated("ec2", oldDefs) })

	d := makePreviewEC2Detail(t, 90, 30)
	line := findSelectedLine(d.View())
	if line == "" {
		t.Fatal("selected line not found")
	}
	if !strings.Contains(line, "\x1b[48;") {
		t.Fatalf("selected line must include background highlight; line=%q", line)
	}
	if got := lipgloss.Width(stripAnsi(line)); got != 90 {
		t.Fatalf("selected row width should fill viewport width: got=%d want=90 line=%q", got, stripAnsi(line))
	}
}
