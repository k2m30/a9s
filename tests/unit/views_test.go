package unit

import (
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/internal/aws"
	"github.com/k2m30/a9s/internal/views"
)

// ---------------------------------------------------------------------------
// T036 - Test profile selector view
// ---------------------------------------------------------------------------

func TestProfileSelectModel_NewProfileSelect(t *testing.T) {
	profiles := []string{"default", "dev", "prod-sso"}
	m := views.NewProfileSelect(profiles, "dev")

	if m.Cursor != 1 {
		t.Errorf("expected cursor at index 1 (dev), got %d", m.Cursor)
	}
	if m.ActiveProfile != "dev" {
		t.Errorf("expected ActiveProfile = %q, got %q", "dev", m.ActiveProfile)
	}
}

func TestProfileSelectModel_MoveUpDown(t *testing.T) {
	profiles := []string{"default", "dev", "prod-sso"}
	m := views.NewProfileSelect(profiles, "default")

	if m.Cursor != 0 {
		t.Fatalf("expected cursor at 0, got %d", m.Cursor)
	}

	// Move up at top should stay at 0
	m.MoveUp()
	if m.Cursor != 0 {
		t.Errorf("expected cursor at 0 after MoveUp at top, got %d", m.Cursor)
	}

	// Move down
	m.MoveDown()
	if m.Cursor != 1 {
		t.Errorf("expected cursor at 1 after MoveDown, got %d", m.Cursor)
	}

	m.MoveDown()
	if m.Cursor != 2 {
		t.Errorf("expected cursor at 2 after second MoveDown, got %d", m.Cursor)
	}

	// Move down at bottom should stay
	m.MoveDown()
	if m.Cursor != 2 {
		t.Errorf("expected cursor at 2 after MoveDown at bottom, got %d", m.Cursor)
	}
}

func TestProfileSelectModel_SelectedProfile(t *testing.T) {
	profiles := []string{"default", "dev", "prod-sso"}
	m := views.NewProfileSelect(profiles, "default")

	if m.SelectedProfile() != "default" {
		t.Errorf("expected selected profile %q, got %q", "default", m.SelectedProfile())
	}

	m.MoveDown()
	if m.SelectedProfile() != "dev" {
		t.Errorf("expected selected profile %q, got %q", "dev", m.SelectedProfile())
	}
}

func TestProfileSelectModel_ViewContainsProfiles(t *testing.T) {
	profiles := []string{"default", "dev", "prod-sso"}
	m := views.NewProfileSelect(profiles, "dev")

	view := m.View()
	for _, p := range profiles {
		if !strings.Contains(view, p) {
			t.Errorf("expected View to contain profile %q", p)
		}
	}
	// Active profile "dev" should have "*" marker
	if !strings.Contains(view, "* dev") {
		t.Errorf("expected View to contain '* dev' for active profile")
	}
}

// ---------------------------------------------------------------------------
// T037 - Test region selector view
// ---------------------------------------------------------------------------

func TestRegionSelectModel_NewRegionSelect(t *testing.T) {
	regions := awsclient.AllRegions()
	m := views.NewRegionSelect(regions, "eu-west-1")

	// Find the expected index
	expectedIdx := -1
	for i, r := range regions {
		if r.Code == "eu-west-1" {
			expectedIdx = i
			break
		}
	}

	if m.Cursor != expectedIdx {
		t.Errorf("expected cursor at index %d (eu-west-1), got %d", expectedIdx, m.Cursor)
	}
}

func TestRegionSelectModel_MoveUpDown(t *testing.T) {
	regions := []awsclient.AWSRegion{
		{Code: "us-east-1", DisplayName: "US East (N. Virginia)"},
		{Code: "us-west-2", DisplayName: "US West (Oregon)"},
		{Code: "eu-west-1", DisplayName: "Europe (Ireland)"},
	}
	m := views.NewRegionSelect(regions, "us-east-1")

	m.MoveUp()
	if m.Cursor != 0 {
		t.Errorf("expected cursor at 0 after MoveUp at top, got %d", m.Cursor)
	}

	m.MoveDown()
	if m.Cursor != 1 {
		t.Errorf("expected cursor at 1, got %d", m.Cursor)
	}

	m.MoveDown()
	if m.Cursor != 2 {
		t.Errorf("expected cursor at 2, got %d", m.Cursor)
	}

	m.MoveDown()
	if m.Cursor != 2 {
		t.Errorf("expected cursor at 2 after MoveDown at bottom, got %d", m.Cursor)
	}
}

func TestRegionSelectModel_SelectedRegion(t *testing.T) {
	regions := []awsclient.AWSRegion{
		{Code: "us-east-1", DisplayName: "US East (N. Virginia)"},
		{Code: "us-west-2", DisplayName: "US West (Oregon)"},
	}
	m := views.NewRegionSelect(regions, "us-east-1")

	selected := m.SelectedRegion()
	if selected.Code != "us-east-1" {
		t.Errorf("expected selected region %q, got %q", "us-east-1", selected.Code)
	}

	m.MoveDown()
	selected = m.SelectedRegion()
	if selected.Code != "us-west-2" {
		t.Errorf("expected selected region %q, got %q", "us-west-2", selected.Code)
	}
}

func TestRegionSelectModel_ViewContainsRegions(t *testing.T) {
	regions := []awsclient.AWSRegion{
		{Code: "us-east-1", DisplayName: "US East (N. Virginia)"},
		{Code: "eu-west-1", DisplayName: "Europe (Ireland)"},
	}
	m := views.NewRegionSelect(regions, "us-east-1")

	view := m.View()
	if !strings.Contains(view, "us-east-1") {
		t.Error("expected View to contain 'us-east-1'")
	}
	if !strings.Contains(view, "US East (N. Virginia)") {
		t.Error("expected View to contain 'US East (N. Virginia)'")
	}
	if !strings.Contains(view, "eu-west-1") {
		t.Error("expected View to contain 'eu-west-1'")
	}
	// Active region should have "*" marker
	if !strings.Contains(view, "* us-east-1") {
		t.Error("expected View to contain '* us-east-1' for active region")
	}
}
