// app_footer_hints_test.go — TDD red-phase pin for Finding C: footer hints
// must be renderer-mode-aware.
//
// New contract (core/app/viewstate.go): MenuFooterHints() is replaced by
//
//	func MenuFooterHintsFor(mode string) []KeyHint
//
// where mode is the ViewState Header.Mode value ("" = TUI, "web", "demo").
// The hint choice keys on web-vs-not-web, NOT on demo — a demo-TUI session
// (Header.Mode=="demo") must still get the TUI hint set.
//
//   - non-web (mode == "" or mode == "demo") -> {ctrl+z Issues only, ctrl+r Refresh}
//     (unchanged from the current MenuFooterHints() contract)
//   - web (mode == "web")                    -> {ctrl+z Issues only, R Refresh}
//
// This function does not exist yet on the `app` package — every test in this
// file fails to COMPILE until core/app/viewstate.go defines
// MenuFooterHintsFor. That compile failure is the expected red-phase signal.
package unit_test

import (
	"reflect"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
)

// wantNonWebFooterHints is the unchanged TUI/demo hint set: {ctrl+z, ctrl+r}.
func wantNonWebFooterHints() []app.KeyHint {
	return []app.KeyHint{
		{Key: "ctrl+z", Help: "Issues only"},
		{Key: "ctrl+r", Help: "Refresh"},
	}
}

// wantWebFooterHints is the web hint set: {ctrl+z, R} — web renderers cannot
// bind ctrl+r in-browser, so the refresh hint surfaces the "R" key instead.
func wantWebFooterHints() []app.KeyHint {
	return []app.KeyHint{
		{Key: "ctrl+z", Help: "Issues only"},
		{Key: "R", Help: "Refresh"},
	}
}

// TestMenuFooterHintsFor_TUIMode_UnchangedHints pins the non-web hint set for
// mode="" (bare TUI, no A9S_MODE set).
func TestMenuFooterHintsFor_TUIMode_UnchangedHints(t *testing.T) {
	got := app.MenuFooterHintsFor("")
	want := wantNonWebFooterHints()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MenuFooterHintsFor(\"\") = %+v, want %+v (TUI mode must keep ctrl+r)", got, want)
	}
}

// TestMenuFooterHintsFor_DemoMode_UsesNonWebHints pins the "key on web-vs-not-web,
// not on demo" rule: a demo-TUI session (Header.Mode=="demo") is still a
// terminal renderer and must get ctrl+r, not the web "R" hint.
func TestMenuFooterHintsFor_DemoMode_UsesNonWebHints(t *testing.T) {
	got := app.MenuFooterHintsFor("demo")
	want := wantNonWebFooterHints()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MenuFooterHintsFor(\"demo\") = %+v, want %+v (demo-TUI must use the non-web hint set — mode selection keys on web-vs-not-web, not on demo)", got, want)
	}
}

// TestMenuFooterHintsFor_WebMode_UsesRRefreshHint pins the new web-specific
// hint set: ctrl+r is replaced by "R" because the web renderer cannot bind
// ctrl+r in-browser.
func TestMenuFooterHintsFor_WebMode_UsesRRefreshHint(t *testing.T) {
	got := app.MenuFooterHintsFor("web")
	want := wantWebFooterHints()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MenuFooterHintsFor(\"web\") = %+v, want %+v (web mode must swap ctrl+r for R)", got, want)
	}
}

// TestMenuFooterHintsFor_WebMode_DoesNotContainCtrlR is a targeted negative
// assertion: the web hint set must NOT carry the "ctrl+r" key at all, since
// browsers intercept ctrl+r for page reload.
func TestMenuFooterHintsFor_WebMode_DoesNotContainCtrlR(t *testing.T) {
	got := app.MenuFooterHintsFor("web")
	for _, h := range got {
		if h.Key == "ctrl+r" {
			t.Errorf("MenuFooterHintsFor(\"web\") contains key %q — web renderer must use \"R\" instead, browsers intercept ctrl+r for page reload", "ctrl+r")
		}
	}
}
