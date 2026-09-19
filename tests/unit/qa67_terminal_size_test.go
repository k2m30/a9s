package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// A terminal exactly 60 columns wide renders normally, not "too narrow".
func TestQa67_H1_MinimumWidth60_RendersNormally(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 60, Height: 24})

	plain := stripANSI(rootViewContent(m))
	if strings.Contains(plain, "too narrow") || strings.Contains(plain, "narrow") {
		t.Errorf("H.1: at exactly 60 columns, should NOT show 'too narrow', got: %s", plain[:min(200, len(plain))])
	}
	if plain == "" {
		t.Error("H.1: View() should return non-empty output at minimum width 60")
	}
}

// A terminal exactly 7 lines tall renders normally, not "too short".
func TestQa67_H2_MinimumHeight7_RendersNormally(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 7})

	plain := stripANSI(rootViewContent(m))
	if strings.Contains(plain, "too short") || strings.Contains(plain, "short") {
		t.Errorf("H.2: at exactly 7 lines, should NOT show 'too short', got: %s", plain[:min(200, len(plain))])
	}
	if plain == "" {
		t.Error("H.2: View() should return non-empty output at minimum height 7")
	}
}

// A 59-column terminal shows the "too narrow" error.
func TestQa67_H3_Width59_ShowsTooNarrow(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 59, Height: 24})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "narrow") {
		t.Errorf("H.3: at 59 columns, should show 'narrow' error, got: %s", plain[:min(200, len(plain))])
	}
}

// A 6-line terminal shows the "too short" error.
func TestQa67_H4_Height6_ShowsTooShort(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 6})

	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "short") {
		t.Errorf("H.4: at 6 lines, should show 'short' error, got: %s", plain[:min(200, len(plain))])
	}
}

// Resizing from below the minimum to above it restores the full UI.
func TestQa67_H5_ResizeFromBelowToAboveMinimum_RestoresUI(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "testprofile", "us-east-1")

	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 50, Height: 24})
	plain := stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "narrow") {
		t.Errorf("H.5: pre-condition: expected 'narrow' at width=50, got: %s", plain[:min(150, len(plain))])
	}

	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	plain = stripANSI(rootViewContent(m))
	if strings.Contains(plain, "narrow") {
		t.Errorf("H.5: after resizing to 80 cols, should NOT show 'narrow', got: %s", plain[:min(200, len(plain))])
	}
	if plain == "" {
		t.Error("H.5: View() should return non-empty output after resize to 80 columns")
	}
}

// Resizing from above the minimum to below it shows the error message.
func TestQa67_H6_ResizeFromAboveToBelowMinimum_ShowsError(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "testprofile", "us-east-1")

	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	plain := stripANSI(rootViewContent(m))
	if strings.Contains(plain, "narrow") {
		t.Errorf("H.6: pre-condition: unexpected 'narrow' at width=120, got: %s", plain[:min(150, len(plain))])
	}

	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 50, Height: 30})
	plain = stripANSI(rootViewContent(m))
	if !strings.Contains(plain, "narrow") {
		t.Errorf("H.6: after resizing to 50 cols, should show 'narrow', got: %s", plain[:min(200, len(plain))])
	}
}

// A resize during the detail view re-renders without crash.
func TestQa67_H7_ResizeDuringDetailView_NoCrash(t *testing.T) {
	m := newRootSizedModel()
	res := &resource.Resource{
		ID:   "i-detail-resize",
		Name: "resize-test-instance",
		Fields: map[string]string{
			"instance_id": "i-detail-resize",
			"name":        "resize-test-instance",
			"state":       "running",
			"type":        "t3.large",
			"private_ip":  "10.0.0.1",
			"public_ip":   "52.1.2.3",
			"launch_time": "2025-03-01",
			"lifecycle":   "normal",
		},
	}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetDetail,
		Resource: res,
	})

	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 30})
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 24})

	out := rootViewContent(m)
	if out == "" {
		t.Error("H.7: detail view should not be empty after resize")
	}
	plain := stripANSI(out)
	if !strings.Contains(plain, "resize-test-instance") {
		t.Errorf("H.7: detail view should show resource name 'resize-test-instance' after resize, got: %s", plain[:min(300, len(plain))])
	}
}

// A resize during the YAML view re-renders without crash.
func TestQa67_H8_ResizeDuringYAMLView_NoCrash(t *testing.T) {
	m := newRootSizedModel()
	res := &resource.Resource{
		ID:     "i-yaml-resize",
		Name:   "yaml-resize-test",
		Fields: map[string]string{},
	}
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:   messages.TargetYAML,
		Resource: res,
	})

	for _, width := range []int{120, 200, 60, 80} {
		m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: width, Height: 24})
	}

	out := rootViewContent(m)
	if out == "" {
		t.Error("H.8: YAML view should not be empty after multiple resizes")
	}
}

// A resize during the help screen re-renders without crash.
func TestQa67_H9_ResizeDuringHelpScreen_NoCrash(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target: messages.TargetHelp,
	})

	for _, width := range []int{120, 80, 200} {
		m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: width, Height: 30})
	}

	out := rootViewContent(m)
	if out == "" {
		t.Error("H.9: help view should not be empty after resize")
	}
	plain := stripANSI(out)
	if !strings.Contains(plain, "NAVIGATION") {
		t.Errorf("H.9: help view should show 'NAVIGATION' section after resize, got: %s", plain[:min(300, len(plain))])
	}
}

// A resize during a child view re-renders without crash.
func TestQa67_H10_ResizeDuringChildView_NoCrash(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, messages.Navigate{
		Target:       messages.TargetResourceList,
		ResourceType: "s3",
	})
	buckets := []resource.Resource{
		{
			ID:   "resize-bucket",
			Name: "resize-bucket",
			Fields: map[string]string{
				"name":          "resize-bucket",
				"region":        "us-east-1",
				"creation_date": "2025-01-01",
			},
		},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "s3", Resources: buckets})

	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			m, _ = rootApplyMsg(m, msg)
		}
	}

	objects := []resource.Resource{
		{ID: "file.txt", Name: "file.txt", Fields: map[string]string{
			"key": "file.txt", "size": "1024", "last_modified": "2025-01-01", "storage_class": "STANDARD",
		}},
	}
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceChild, ResourceType: "s3_objects", Resources: objects})

	for _, width := range []int{80, 120, 60, 200} {
		m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: width, Height: 24})
	}

	out := rootViewContent(m)
	if out == "" {
		t.Error("H.10: child view should not be empty after resize")
	}
	plain := stripANSI(out)
	if !strings.Contains(plain, "resize-bucket") {
		t.Errorf("H.10: child view should show bucket name 'resize-bucket' after resize, got: %s", plain[:min(300, len(plain))])
	}
}

// A 300-column terminal renders without overflow or crash.
func TestQa67_H11_ExtremelyWide_NoCrash(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 300, Height: 40})

	out := rootViewContent(m)
	if out == "" {
		t.Error("H.11: View() should not be empty at 300 columns width")
	}
	plain := stripANSI(out)
	if strings.Contains(plain, "narrow") {
		t.Error("H.11: at 300 columns, should NOT show 'too narrow' error")
	}
}

// A 200-line terminal renders without crash.
func TestQa67_H12_ExtremelyTall_NoCrash(t *testing.T) {
	tui.Version = "test"
	m := newBlessedModel(t, "testprofile", "us-east-1")
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 200})

	out := rootViewContent(m)
	if out == "" {
		t.Error("H.12: View() should not be empty at 200 lines height")
	}
	plain := stripANSI(out)
	if strings.Contains(plain, "short") {
		t.Error("H.12: at 200 lines, should NOT show 'too short' error")
	}
}

// Extreme sizes on the resource list view do not crash.
func TestQa67_H11_H12_ExtremeSizes_WithResourceList_NoCrash(t *testing.T) {
	sizes := []struct {
		name          string
		width, height int
	}{
		{"narrow_tall", 60, 100},
		{"wide_short", 300, 7},
		{"huge", 300, 200},
		{"square_large", 200, 200},
	}
	for _, sz := range sizes {
		t.Run(sz.name, func(t *testing.T) {
			tui.Version = "test"
			m := newBlessedModel(t, "testprofile", "us-east-1")
			m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: sz.width, Height: sz.height})
			m, _ = rootApplyMsg(m, messages.Navigate{
				Target:       messages.TargetResourceList,
				ResourceType: "ec2",
			})
			resources := []resource.Resource{
				{ID: "i-size-test", Name: "size-test", Fields: map[string]string{
					"instance_id": "i-size-test",
					"name":        "size-test",
					"state":       "running",
					"type":        "t3.micro",
					"private_ip":  "10.0.0.1",
					"public_ip":   "",
					"launch_time": "2025-01-01",
					"lifecycle":   "",
				}},
			}
			m, _ = rootApplyMsg(m, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: resources})
			out := rootViewContent(m)
			if out == "" {
				t.Errorf("[%s] H.11/H.12: View() returned empty at %dx%d", sz.name, sz.width, sz.height)
			}
		})
	}
}
