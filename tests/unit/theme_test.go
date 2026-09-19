package unit

import (
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/internal/tui/styles"
	"github.com/k2m30/a9s/v3/internal/tui/styles/themes"
	"github.com/k2m30/a9s/v3/internal/tui/views"
)

func TestDefaultTheme_AllFieldsMatchPalette(t *testing.T) {
	th := styles.DefaultTheme()

	if th.Name != "Tokyo Night Dark" {
		t.Errorf("DefaultTheme Name: expected %q, got %q", "Tokyo Night Dark", th.Name)
	}

	checks := []struct {
		name    string
		got     color.Color
		wantHex string
	}{
		{"HeaderFg", th.HeaderFg, "#c0caf5"},
		{"Accent", th.Accent, "#7aa2f7"},
		{"Dim", th.Dim, "#565f89"},
		{"Border", th.Border, "#414868"},
		{"RowSelectedBg", th.RowSelectedBg, "#7aa2f7"},
		{"RowSelectedFg", th.RowSelectedFg, "#1a1b26"},
		{"RowAltBg", th.RowAltBg, "#1e2030"},
		{"Running", th.Running, "#9ece6a"},
		{"Stopped", th.Stopped, "#f7768e"},
		{"Pending", th.Pending, "#e0af68"},
		{"Terminated", th.Terminated, "#565f89"},
		{"DetailKey", th.DetailKey, "#7aa2f7"},
		{"DetailVal", th.DetailVal, "#c0caf5"},
		{"DetailSec", th.DetailSec, "#e0af68"},
		{"YAMLKey", th.YAMLKey, "#7aa2f7"},
		{"YAMLStr", th.YAMLStr, "#9ece6a"},
		{"YAMLNum", th.YAMLNum, "#ff9e64"},
		{"YAMLBool", th.YAMLBool, "#bb9af7"},
		{"YAMLNull", th.YAMLNull, "#565f89"},
		{"YAMLTree", th.YAMLTree, "#414868"},
		{"HelpKey", th.HelpKey, "#9ece6a"},
		{"HelpCat", th.HelpCat, "#e0af68"},
		{"Filter", th.Filter, "#e0af68"},
		{"Success", th.Success, "#9ece6a"},
		{"Error", th.Error, "#f7768e"},
		{"Spinner", th.Spinner, "#7aa2f7"},
		{"Scroll", th.Scroll, "#414868"},
		{"Warning", th.Warning, "#e0af68"},
		{"KeyHintKey", th.KeyHintKey, "#7aa2f7"},
		{"KeyHintBg", th.KeyHintBg, "#24283b"},
		{"KeyHintFg", th.KeyHintFg, "#565f89"},
		{"OverlayBg", th.OverlayBg, "#1a1b26"},
		{"OverlayBorder", th.OverlayBorder, "#7aa2f7"},
		{"SearchHighlightFg", th.SearchHighlightFg, "#1a1b26"},
		{"SearchHighlightBg", th.SearchHighlightBg, "#e0af68"},
	}

	for _, c := range checks {
		want := lipgloss.Color(c.wantHex)
		if !colorsEqual(c.got, want) {
			t.Errorf("DefaultTheme %s: expected %s, got %v", c.name, c.wantHex, c.got)
		}
	}
}

func TestApplyTheme_UpdatesPaletteAndComposedStyles(t *testing.T) {
	original := styles.DefaultTheme()
	defer styles.ApplyTheme(original)

	custom := styles.DefaultTheme()
	custom.Accent = lipgloss.Color("#ff0000")
	custom.RowSelectedBg = lipgloss.Color("#00ff00")
	custom.HeaderFg = lipgloss.Color("#0000ff")

	styles.ApplyTheme(custom)

	if !colorsEqual(styles.ColAccent, lipgloss.Color("#ff0000")) {
		t.Errorf("ApplyTheme: ColAccent not updated, got %v", styles.ColAccent)
	}

	bg := styles.RowSelected.GetBackground()
	if !colorsEqual(bg, lipgloss.Color("#00ff00")) {
		t.Errorf("ApplyTheme: RowSelected background not updated to #00ff00, got %v", bg)
	}

	fg := styles.TableHeader.GetForeground()
	if !colorsEqual(fg, lipgloss.Color("#ff0000")) {
		t.Errorf("ApplyTheme: TableHeader foreground not updated to accent #ff0000, got %v", fg)
	}
}

func TestActiveTheme_ReturnsCurrentThemeName(t *testing.T) {
	original := styles.DefaultTheme()
	defer styles.ApplyTheme(original)

	custom := styles.DefaultTheme()
	custom.Name = "My Custom Theme"
	styles.ApplyTheme(custom)

	active := styles.ActiveTheme()
	if active.Name != "My Custom Theme" {
		t.Errorf("ActiveTheme Name: expected %q, got %q", "My Custom Theme", active.Name)
	}
}

func TestApplyTheme_RebuildsRowColorCache(t *testing.T) {
	original := styles.DefaultTheme()
	defer styles.ApplyTheme(original)

	beforeFg := styles.ColorStyle(resource.ColorHealthy).GetForeground()

	custom := styles.DefaultTheme()
	custom.Running = lipgloss.Color("#aabbcc")
	styles.ApplyTheme(custom)

	afterFg := styles.ColorStyle(resource.ColorHealthy).GetForeground()

	if colorsEqual(beforeFg, afterFg) {
		t.Error("ApplyTheme: ColorStyle(ColorHealthy) foreground unchanged after applying new Running color")
	}
	if !colorsEqual(afterFg, lipgloss.Color("#aabbcc")) {
		t.Errorf("ApplyTheme: ColorStyle(ColorHealthy) expected #aabbcc, got %v", afterFg)
	}
}

func TestThemeFromYAML_FullTheme(t *testing.T) {
	data := []byte(`
name: "Test Theme"
colors:
  header_fg: "#ffffff"
  accent: "#ff0000"
  dim: "#888888"
  border: "#333333"
  row_selected_bg: "#ff0000"
  row_selected_fg: "#000000"
  row_alt_bg: "#111111"
  running: "#00ff00"
  stopped: "#ff0000"
  pending: "#ffff00"
  terminated: "#888888"
  detail_key: "#ff0000"
  detail_val: "#ffffff"
  detail_sec: "#ffff00"
  yaml_key: "#ff0000"
  yaml_str: "#00ff00"
  yaml_num: "#ff9900"
  yaml_bool: "#ff00ff"
  yaml_null: "#888888"
  yaml_tree: "#333333"
  help_key: "#00ff00"
  help_cat: "#ffff00"
  filter: "#ffff00"
  success: "#00ff00"
  error: "#ff0000"
  spinner: "#ff0000"
  scroll: "#333333"
  warning: "#ffff00"
  key_hint_key: "#ff0000"
  key_hint_bg: "#222222"
  key_hint_fg: "#888888"
  overlay_bg: "#000000"
  overlay_border: "#ff0000"
  search_highlight_fg: "#000000"
  search_highlight_bg: "#ffff00"
`)

	th, err := styles.ThemeFromYAML(data)
	if err != nil {
		t.Fatalf("ThemeFromYAML: unexpected error: %v", err)
	}

	if th.Name != "Test Theme" {
		t.Errorf("ThemeFromYAML Name: expected %q, got %q", "Test Theme", th.Name)
	}

	checks := []struct {
		name    string
		got     color.Color
		wantHex string
	}{
		{"HeaderFg", th.HeaderFg, "#ffffff"},
		{"Accent", th.Accent, "#ff0000"},
		{"Dim", th.Dim, "#888888"},
		{"Border", th.Border, "#333333"},
		{"RowSelectedBg", th.RowSelectedBg, "#ff0000"},
		{"RowSelectedFg", th.RowSelectedFg, "#000000"},
		{"RowAltBg", th.RowAltBg, "#111111"},
		{"Running", th.Running, "#00ff00"},
		{"Stopped", th.Stopped, "#ff0000"},
		{"Pending", th.Pending, "#ffff00"},
		{"Terminated", th.Terminated, "#888888"},
		{"DetailKey", th.DetailKey, "#ff0000"},
		{"DetailVal", th.DetailVal, "#ffffff"},
		{"DetailSec", th.DetailSec, "#ffff00"},
		{"YAMLKey", th.YAMLKey, "#ff0000"},
		{"YAMLStr", th.YAMLStr, "#00ff00"},
		{"YAMLNum", th.YAMLNum, "#ff9900"},
		{"YAMLBool", th.YAMLBool, "#ff00ff"},
		{"YAMLNull", th.YAMLNull, "#888888"},
		{"YAMLTree", th.YAMLTree, "#333333"},
		{"HelpKey", th.HelpKey, "#00ff00"},
		{"HelpCat", th.HelpCat, "#ffff00"},
		{"Filter", th.Filter, "#ffff00"},
		{"Success", th.Success, "#00ff00"},
		{"Error", th.Error, "#ff0000"},
		{"Spinner", th.Spinner, "#ff0000"},
		{"Scroll", th.Scroll, "#333333"},
		{"Warning", th.Warning, "#ffff00"},
		{"KeyHintKey", th.KeyHintKey, "#ff0000"},
		{"KeyHintBg", th.KeyHintBg, "#222222"},
		{"KeyHintFg", th.KeyHintFg, "#888888"},
		{"OverlayBg", th.OverlayBg, "#000000"},
		{"OverlayBorder", th.OverlayBorder, "#ff0000"},
		{"SearchHighlightFg", th.SearchHighlightFg, "#000000"},
		{"SearchHighlightBg", th.SearchHighlightBg, "#ffff00"},
	}

	for _, c := range checks {
		want := lipgloss.Color(c.wantHex)
		if !colorsEqual(c.got, want) {
			t.Errorf("ThemeFromYAML %s: expected %s, got %v", c.name, c.wantHex, c.got)
		}
	}
}

func TestThemeFromYAML_PartialThemeInheritsDefaults(t *testing.T) {
	data := []byte(`
name: "Partial Theme"
colors:
  accent: "#ff0000"
`)

	th, err := styles.ThemeFromYAML(data)
	if err != nil {
		t.Fatalf("ThemeFromYAML partial: unexpected error: %v", err)
	}

	if !colorsEqual(th.Accent, lipgloss.Color("#ff0000")) {
		t.Errorf("ThemeFromYAML partial: Accent expected #ff0000, got %v", th.Accent)
	}

	def := styles.DefaultTheme()

	nonAccentChecks := []struct {
		name string
		got  color.Color
		want color.Color
	}{
		{"HeaderFg", th.HeaderFg, def.HeaderFg},
		{"Dim", th.Dim, def.Dim},
		{"Border", th.Border, def.Border},
		{"RowSelectedBg", th.RowSelectedBg, def.RowSelectedBg},
		{"RowSelectedFg", th.RowSelectedFg, def.RowSelectedFg},
		{"Running", th.Running, def.Running},
		{"Stopped", th.Stopped, def.Stopped},
		{"Pending", th.Pending, def.Pending},
		{"SearchHighlightBg", th.SearchHighlightBg, def.SearchHighlightBg},
		{"SearchHighlightFg", th.SearchHighlightFg, def.SearchHighlightFg},
	}
	for _, c := range nonAccentChecks {
		if !colorsEqual(c.got, c.want) {
			t.Errorf("ThemeFromYAML partial %s: expected default %v, got %v", c.name, c.want, c.got)
		}
	}
}

func TestThemeFromYAML_InvalidHexReturnsError(t *testing.T) {
	data := []byte(`
name: "Bad Theme"
colors:
  accent: "not-a-color"
`)

	_, err := styles.ThemeFromYAML(data)
	if err == nil {
		t.Error("ThemeFromYAML: expected error for invalid hex value, got nil")
	}
}

func TestThemeFromYAML_UnknownKeysIgnored(t *testing.T) {
	data := []byte(`
name: "Unknown Keys Theme"
colors:
  unknown_key: "#ff0000"
  another_unknown: "#00ff00"
`)

	th, err := styles.ThemeFromYAML(data)
	if err != nil {
		t.Fatalf("ThemeFromYAML unknown keys: unexpected error: %v", err)
	}

	def := styles.DefaultTheme()
	if !colorsEqual(th.Accent, def.Accent) {
		t.Errorf("ThemeFromYAML unknown keys: Accent should be default %v, got %v", def.Accent, th.Accent)
	}
}

func TestApplyTheme_SearchHighlightStylesUpdated(t *testing.T) {
	original := styles.DefaultTheme()
	defer styles.ApplyTheme(original)

	custom := styles.DefaultTheme()
	custom.SearchHighlightFg = lipgloss.Color("#112233")
	custom.SearchHighlightBg = lipgloss.Color("#aabbcc")
	styles.ApplyTheme(custom)

	currentBg := styles.SearchCurrentStyle.GetBackground()
	if !colorsEqual(currentBg, lipgloss.Color("#aabbcc")) {
		t.Errorf("SearchCurrentStyle background: expected #aabbcc, got %v", currentBg)
	}

	otherFg := styles.SearchOtherStyle.GetForeground()
	if !colorsEqual(otherFg, lipgloss.Color("#aabbcc")) {
		t.Errorf("SearchOtherStyle foreground: expected #aabbcc, got %v", otherFg)
	}
}

func TestEmbeddedThemes_AllParseWithAllFieldsSet(t *testing.T) {
	entries, err := themes.FS.ReadDir(".")
	if err != nil {
		t.Fatalf("themes.FS.ReadDir: %v", err)
	}

	var yamlFiles []string
	for _, e := range entries {
		if !e.IsDir() {
			yamlFiles = append(yamlFiles, e.Name())
		}
	}

	if len(yamlFiles) == 0 {
		t.Fatal("no YAML files found in embedded themes FS")
	}

	def := styles.DefaultTheme()

	for _, name := range yamlFiles {
		t.Run(name, func(t *testing.T) {
			data, readErr := themes.FS.ReadFile(name)
			if readErr != nil {
				t.Fatalf("ReadFile(%q): %v", name, readErr)
			}

			th, parseErr := styles.ThemeFromYAML(data)
			if parseErr != nil {
				t.Fatalf("ThemeFromYAML(%q): %v", name, parseErr)
			}

			if th.Name == "" {
				t.Errorf("%s: Name is empty", name)
			}

			// A color set to "" is the zero lipgloss.Color result.
			zeroColor := lipgloss.Color("")

			colorFields := []struct {
				fieldName string
				got       color.Color
			}{
				{"HeaderFg", th.HeaderFg},
				{"Accent", th.Accent},
				{"Dim", th.Dim},
				{"Border", th.Border},
				{"RowSelectedBg", th.RowSelectedBg},
				{"RowSelectedFg", th.RowSelectedFg},
				{"RowAltBg", th.RowAltBg},
				{"Running", th.Running},
				{"Stopped", th.Stopped},
				{"Pending", th.Pending},
				{"Terminated", th.Terminated},
				{"DetailKey", th.DetailKey},
				{"DetailVal", th.DetailVal},
				{"DetailSec", th.DetailSec},
				{"YAMLKey", th.YAMLKey},
				{"YAMLStr", th.YAMLStr},
				{"YAMLNum", th.YAMLNum},
				{"YAMLBool", th.YAMLBool},
				{"YAMLNull", th.YAMLNull},
				{"YAMLTree", th.YAMLTree},
				{"HelpKey", th.HelpKey},
				{"HelpCat", th.HelpCat},
				{"Filter", th.Filter},
				{"Success", th.Success},
				{"Error", th.Error},
				{"Spinner", th.Spinner},
				{"Scroll", th.Scroll},
				{"Warning", th.Warning},
				{"KeyHintKey", th.KeyHintKey},
				{"KeyHintBg", th.KeyHintBg},
				{"KeyHintFg", th.KeyHintFg},
				{"OverlayBg", th.OverlayBg},
				{"OverlayBorder", th.OverlayBorder},
				{"SearchHighlightFg", th.SearchHighlightFg},
				{"SearchHighlightBg", th.SearchHighlightBg},
			}

			for _, f := range colorFields {
				if colorsEqual(f.got, zeroColor) {
					// Embedded built-in themes set every field explicitly.
					_ = def
					t.Errorf("%s: field %s is zero/empty", name, f.fieldName)
				}
			}
		})
	}
}

func TestApplyTheme_DraculaUpdatesComposedStyles(t *testing.T) {
	original := styles.DefaultTheme()
	defer styles.ApplyTheme(original)

	data, err := themes.FS.ReadFile("dracula.yaml")
	if err != nil {
		t.Fatalf("ReadFile dracula.yaml: %v", err)
	}

	dracula, err := styles.ThemeFromYAML(data)
	if err != nil {
		t.Fatalf("ThemeFromYAML dracula.yaml: %v", err)
	}

	styles.ApplyTheme(dracula)

	rowBg := styles.RowSelected.GetBackground()
	if colorsEqual(rowBg, original.RowSelectedBg) {
		t.Errorf("RowSelected background unchanged after applying dracula theme")
	}
	if !colorsEqual(rowBg, dracula.RowSelectedBg) {
		t.Errorf("RowSelected background: expected dracula RowSelectedBg %v, got %v", dracula.RowSelectedBg, rowBg)
	}

	headerFg := styles.TableHeader.GetForeground()
	if colorsEqual(headerFg, original.Accent) {
		t.Errorf("TableHeader foreground unchanged after applying dracula theme")
	}
	if !colorsEqual(headerFg, dracula.Accent) {
		t.Errorf("TableHeader foreground: expected dracula Accent %v, got %v", dracula.Accent, headerFg)
	}

	errFg := styles.FlashError.GetForeground()
	if colorsEqual(errFg, original.Error) {
		t.Errorf("FlashError foreground unchanged after applying dracula theme")
	}
	if !colorsEqual(errFg, dracula.Error) {
		t.Errorf("FlashError foreground: expected dracula Error %v, got %v", dracula.Error, errFg)
	}
}

func TestEnsureThemesDir_WritesMissingSkipsExisting(t *testing.T) {
	dir := t.TempDir()

	if err := themes.EnsureThemesDir(dir); err != nil {
		t.Fatalf("EnsureThemesDir (first call): %v", err)
	}

	entries, _ := themes.FS.ReadDir(".")
	var yamlNames []string
	for _, e := range entries {
		if !e.IsDir() {
			yamlNames = append(yamlNames, e.Name())
		}
	}

	for _, name := range yamlNames {
		dest := filepath.Join(dir, name)
		if _, err := os.Stat(dest); err != nil {
			t.Errorf("EnsureThemesDir: expected file %s to exist after first call, got: %v", name, err)
		}
	}

	if len(yamlNames) == 0 {
		t.Skip("no embedded themes to test")
	}
	modified := yamlNames[0]
	modifiedPath := filepath.Join(dir, modified)
	sentinel := []byte("# user-modified content\n")
	if err := os.WriteFile(modifiedPath, sentinel, 0644); err != nil {
		t.Fatalf("writing modified content: %v", err)
	}

	if err := themes.EnsureThemesDir(dir); err != nil {
		t.Fatalf("EnsureThemesDir (second call): %v", err)
	}

	got, err := os.ReadFile(modifiedPath)
	if err != nil {
		t.Fatalf("reading modified file after second call: %v", err)
	}
	if string(got) != string(sentinel) {
		t.Errorf("EnsureThemesDir overwrote user-modified file %s; expected sentinel content preserved", modified)
	}
}

func TestEnsureThemesDir_PreservesExactBytes(t *testing.T) {
	dir := t.TempDir()

	entries, _ := themes.FS.ReadDir(".")
	type fileCheck struct {
		name    string
		content []byte
	}
	var checks []fileCheck
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		custom := []byte("# custom: " + e.Name() + "\n")
		if err := os.WriteFile(filepath.Join(dir, e.Name()), custom, 0644); err != nil {
			t.Fatalf("pre-populate %s: %v", e.Name(), err)
		}
		checks = append(checks, fileCheck{e.Name(), custom})
	}

	if err := themes.EnsureThemesDir(dir); err != nil {
		t.Fatalf("EnsureThemesDir: %v", err)
	}

	for _, c := range checks {
		got, err := os.ReadFile(filepath.Join(dir, c.name))
		if err != nil {
			t.Errorf("reading %s after EnsureThemesDir: %v", c.name, err)
			continue
		}
		if string(got) != string(c.content) {
			t.Errorf("EnsureThemesDir overwrote %s: expected %q, got %q", c.name, c.content, got)
		}
	}
}

func TestApplyTheme_WithNoColorSet_ProducesMonochrome(t *testing.T) {
	original := styles.DefaultTheme()
	defer func() {
		os.Unsetenv("NO_COLOR")
		styles.ApplyTheme(original)
	}()

	t.Setenv("NO_COLOR", "1")

	// Build a non-default theme to ensure the palette change is irrelevant.
	nonDefault := original
	nonDefault.Accent = lipgloss.Color("#ff0000")
	nonDefault.RowSelectedBg = lipgloss.Color("#ff0000")

	styles.ApplyTheme(nonDefault)

	zeroCol := lipgloss.NoColor{}
	rowBg := styles.RowSelected.GetBackground()
	if !colorsEqual(rowBg, zeroCol) {
		t.Errorf("RowSelected.GetBackground(): expected no color in NO_COLOR mode, got %v", rowBg)
	}
	if !styles.RowSelected.GetReverse() {
		t.Errorf("RowSelected.GetReverse(): expected true in NO_COLOR mode, got false")
	}

	headerFg := styles.TableHeader.GetForeground()
	if !colorsEqual(headerFg, zeroCol) {
		t.Errorf("TableHeader.GetForeground(): expected no color in NO_COLOR mode, got %v", headerFg)
	}
}

func TestApplyTheme_WithNoColorEmpty_ProducesMonochrome(t *testing.T) {
	original := styles.DefaultTheme()
	defer func() {
		os.Unsetenv("NO_COLOR")
		styles.ApplyTheme(original)
	}()

	// NO_COLOR spec: presence of the variable (even empty) activates monochrome.
	t.Setenv("NO_COLOR", "")

	nonDefault := original
	nonDefault.Accent = lipgloss.Color("#00ff00")
	nonDefault.RowSelectedBg = lipgloss.Color("#00ff00")

	styles.ApplyTheme(nonDefault)

	zeroCol := lipgloss.NoColor{}
	rowBg := styles.RowSelected.GetBackground()
	if !colorsEqual(rowBg, zeroCol) {
		t.Errorf("RowSelected.GetBackground(): expected no color in NO_COLOR='' mode, got %v", rowBg)
	}
	if !styles.RowSelected.GetReverse() {
		t.Errorf("RowSelected.GetReverse(): expected true in NO_COLOR='' mode, got false")
	}

	headerFg := styles.TableHeader.GetForeground()
	if !colorsEqual(headerFg, zeroCol) {
		t.Errorf("TableHeader.GetForeground(): expected no color in NO_COLOR='' mode, got %v", headerFg)
	}
}

func TestDefaultActiveTheme_TokyoNightMarkedCurrent(t *testing.T) {
	themeFiles := []string{"tokyo-night.yaml", "dracula.yaml"}
	body := app.SelectorBody{
		Items:      themeFiles,
		Selected:   0,
		AllItems:   themeFiles,
		ActiveItem: "tokyo-night.yaml",
		Title:      "themes",
	}
	m := views.NewTransientSelector(80, 24)

	plain := stripANSI(m.RenderSelector(body))

	found := false
	for line := range strings.SplitSeq(plain, "\n") {
		if strings.Contains(line, "tokyo-night.yaml") && strings.Contains(line, "(current)") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("tokyo-night.yaml (default active theme) should have (current) marker; got:\n%s", plain)
	}
}
