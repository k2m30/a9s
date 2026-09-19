// MenuFooterHintsFor(mode) keys on web-vs-not-web, not on demo: mode "" (TUI)
// and "demo" get {ctrl+z Issues only, ctrl+r Refresh}; "web" gets
// {ctrl+z Issues only, R Refresh}, because browsers intercept ctrl+r for page
// reload.
package unit_test

import (
	"reflect"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
)

// wantNonWebFooterHints is the TUI/demo hint set: {ctrl+z, ctrl+r}.
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

func TestMenuFooterHintsFor_TUIMode_UnchangedHints(t *testing.T) {
	got := app.MenuFooterHintsFor("")
	want := wantNonWebFooterHints()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MenuFooterHintsFor(\"\") = %+v, want %+v (TUI mode must keep ctrl+r)", got, want)
	}
}

func TestMenuFooterHintsFor_DemoMode_UsesNonWebHints(t *testing.T) {
	got := app.MenuFooterHintsFor("demo")
	want := wantNonWebFooterHints()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MenuFooterHintsFor(\"demo\") = %+v, want %+v (demo-TUI must use the non-web hint set — mode selection keys on web-vs-not-web, not on demo)", got, want)
	}
}

func TestMenuFooterHintsFor_WebMode_UsesRRefreshHint(t *testing.T) {
	got := app.MenuFooterHintsFor("web")
	want := wantWebFooterHints()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MenuFooterHintsFor(\"web\") = %+v, want %+v (web mode must swap ctrl+r for R)", got, want)
	}
}

func TestMenuFooterHintsFor_WebMode_DoesNotContainCtrlR(t *testing.T) {
	got := app.MenuFooterHintsFor("web")
	for _, h := range got {
		if h.Key == "ctrl+r" {
			t.Errorf("MenuFooterHintsFor(\"web\") contains key %q — web renderer must use \"R\" instead, browsers intercept ctrl+r for page reload", "ctrl+r")
		}
	}
}
