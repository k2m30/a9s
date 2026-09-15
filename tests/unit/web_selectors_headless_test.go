// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package unit_test

// web_selectors_headless_test.go — the profile and region selectors open on
// a host with no adapter, and the theme command says why it does not.

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
)

func TestWebHost_RegionCommandOpensTheSelector(t *testing.T) {
	c := newTestController(t)
	c.SetUIMode("web")
	vs, _ := c.Apply(app.Action{Kind: app.ActionCommand, Arg: "region"})
	if vs.Body.Kind != app.BodyKindSelector || vs.Body.Selector == nil {
		t.Fatalf(":region on the web host shows %q with selector %v, want a selector body", vs.Body.Kind, vs.Body.Selector)
	}
	if !slices.Contains(vs.Body.Selector.Items, "us-east-1") {
		t.Errorf("the region selector lists %v, want the AWS regions", vs.Body.Selector.Items)
	}
}

func TestWebHost_ProfileCommandOpensTheSelector(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "config")
	if err := os.WriteFile(cfg, []byte("[profile example-readonly]\nregion = us-east-1\n\n[profile example-admin]\nregion = eu-west-1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("AWS_CONFIG_FILE", cfg)
	c := newTestController(t)
	c.SetUIMode("web")
	vs, _ := c.Apply(app.Action{Kind: app.ActionCommand, Arg: "profile"})
	if vs.Body.Kind != app.BodyKindSelector || vs.Body.Selector == nil {
		t.Fatalf(":profile on the web host shows %q (flash %q), want the profile selector", vs.Body.Kind, vs.Header.Flash.Text)
	}
	if !slices.Contains(vs.Body.Selector.Items, "example-admin") {
		t.Errorf("the profile selector lists %v, want the config file's profiles", vs.Body.Selector.Items)
	}
}

func TestWebHost_ThemeCommandSaysThemesAreForTheTerminal(t *testing.T) {
	c := newTestController(t)
	c.SetUIMode("web")
	vs, _ := c.Apply(app.Action{Kind: app.ActionCommand, Arg: "theme"})
	if vs.Body.Kind == app.BodyKindSelector {
		t.Fatalf(":theme on the web host opened a selector; the web page has no theme to apply")
	}
	if !strings.Contains(vs.Header.Flash.Text, "terminal") {
		t.Errorf(":theme on the web host flashes %q, want it to say themes are a terminal setting", vs.Header.Flash.Text)
	}
}
