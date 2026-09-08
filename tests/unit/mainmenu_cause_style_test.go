// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// mainmenu_cause_style_test.go — the reason a menu count is missing is not an
// alias.
//
// A refused probe replaces the type's alias with the word for why (denied,
// throttled, timeout). It was painted in the dim style the alias uses, so the
// one cell on the screen carrying a failure read as the quietest thing on the
// row.
package unit

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/internal/tui/keys"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

// stylePrefixFor returns the escape sequence a style puts in front of its
// content, taken from the style itself so the pin follows the theme.
func stylePrefixFor(t *testing.T, s interface{ Render(...string) string }, word string) string {
	t.Helper()
	rendered := s.Render(word)
	i := strings.Index(rendered, word)
	if i <= 0 {
		t.Fatalf("style renders %q with no escape sequence in front of it: %q", word, rendered)
	}
	return rendered[:i]
}

// TestMainMenuRenderBody_ProbeCauseRendersInTheWarnStyle pins that the cause
// word carries the theme's warn colour on both the selected and unselected
// row, and never the alias's dim one.
func TestMainMenuRenderBody_ProbeCauseRendersInTheWarnStyle(t *testing.T) {
	styles.ReinitForTest()
	t.Cleanup(styles.ReinitForTest)

	const cause = "denied"
	warn := stylePrefixFor(t, styles.TierColorStyle("~"), cause)
	dim := stylePrefixFor(t, styles.DimText, cause)
	if warn == dim {
		t.Fatal("the warn and dim styles are indistinguishable in this theme, so the pin cannot see the difference")
	}

	body := app.MenuBody{
		Entries: []app.MenuEntry{
			{ShortName: "ec2", Display: "EC2 Instances", Alias: "ec2", Cause: cause},
			{ShortName: "s3", Display: "S3 Buckets", Alias: "s3", Cause: cause},
		},
	}

	for _, selected := range []int{0, 1} {
		body.Selected = selected
		m := views.NewMainMenu(keys.Default())
		m.SetSize(80, 40)
		out := m.RenderBody(body)
		if !strings.Contains(out, warn+cause) {
			t.Errorf("with row %d selected, the cause word is not painted in the warn style:\n%q", selected, out)
		}
		if strings.Contains(out, dim+cause) {
			t.Errorf("with row %d selected, the cause word is painted in the alias's dim style:\n%q", selected, out)
		}
	}
}

// TestMainMenuRenderBody_AliasKeepsTheDimStyle is the control: a row whose
// probe answered still shows its alias quietly.
func TestMainMenuRenderBody_AliasKeepsTheDimStyle(t *testing.T) {
	styles.ReinitForTest()
	t.Cleanup(styles.ReinitForTest)

	const alias = "ec2"
	dim := stylePrefixFor(t, styles.DimText, alias)

	m := views.NewMainMenu(keys.Default())
	m.SetSize(80, 40)
	out := m.RenderBody(app.MenuBody{
		Entries: []app.MenuEntry{{ShortName: "ec2", Display: "EC2 Instances", Alias: alias, Availability: 3, AvailKnown: true}},
	})
	if !strings.Contains(out, dim+alias) {
		t.Errorf("the alias of a healthy row lost its dim style:\n%q", out)
	}
}
