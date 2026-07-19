package unit

// mainmenu_renderbody_truncated_test.go — live-seam replacement for
// issue236_truncated_zero_nav_test.go's TestIssue236_DisplayShowsZeroPlusForTruncatedZero
// (022-codebase-cleanup wave 3). MainMenuModel.View()/SetAvailability/
// SetTruncated/SelectedItem are production-dead (Model.RenderBody(body) is
// the only reachable render entry — see internal/tui/renderer.go:renderMenu).
// The cursor-lands-on / Enter-navigates-on truncated-zero business rules
// from the retired file are already pinned on the controller path by
// TestMenuIntent_PatchMenuAvailability_TruncatedCount and
// TestMenuAction_Select_AllowedForTruncatedZero (app_menu_test.go); only the
// "(0+)" vs bare "(0)" render contract had no live-seam pin.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/views"
	"github.com/k2m30/a9s/v3/tests/unit/tuitest"
)

// TestMainMenuRenderBody_TruncatedZero_ShowsPlusSuffix pins that
// RenderBody renders "(0+)" — never a bare "(0)" — for an entry whose
// availability probe found zero items on a truncated (more-pages-exist) page.
func TestMainMenuRenderBody_TruncatedZero_ShowsPlusSuffix(t *testing.T) {
	tuitest.NoColor(t)

	m := views.NewMainMenu(keys.Default())
	m.SetSize(80, 40)

	body := app.MenuBody{
		Entries: []app.MenuEntry{
			{ShortName: "ec2", Display: "EC2 Instances", Availability: 0, AvailKnown: true, AvailTruncated: true},
		},
	}

	plain := stripANSI(m.RenderBody(body))
	if !strings.Contains(plain, "(0+)") {
		t.Errorf("RenderBody should render '(0+)' for a truncated-zero entry, got:\n%s", plain)
	}
	withoutZeroPlus := strings.ReplaceAll(plain, "(0+)", "")
	if strings.Contains(withoutZeroPlus, "(0)") {
		t.Errorf("RenderBody must NOT render bare '(0)' for a truncated-zero entry, got:\n%s", plain)
	}
}

// TestMainMenuRenderBody_ConfirmedZero_ShowsBareZero is the negative case:
// a confirmed-empty entry (Truncated=false) renders bare "(0)", never "(0+)".
func TestMainMenuRenderBody_ConfirmedZero_ShowsBareZero(t *testing.T) {
	tuitest.NoColor(t)

	m := views.NewMainMenu(keys.Default())
	m.SetSize(80, 40)

	body := app.MenuBody{
		Entries: []app.MenuEntry{
			{ShortName: "lambda", Display: "Lambda Functions", Availability: 0, AvailKnown: true, AvailTruncated: false},
		},
	}

	plain := stripANSI(m.RenderBody(body))
	if strings.Contains(plain, "(0+)") {
		t.Errorf("RenderBody must NOT render '(0+)' for a confirmed-empty entry, got:\n%s", plain)
	}
	if !strings.Contains(plain, "(0)") {
		t.Errorf("RenderBody should render bare '(0)' for a confirmed-empty entry, got:\n%s", plain)
	}
}
