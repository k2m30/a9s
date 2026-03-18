package unit

import (
	"image/color"
	"os"
	"testing"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/internal/tui/styles"
)

// colorsEqual compares two color.Color values by their RGBA components.
func colorsEqual(a, b color.Color) bool {
	r1, g1, b1, a1 := a.RGBA()
	r2, g2, b2, a2 := b.RGBA()
	return r1 == r2 && g1 == g2 && b1 == b2 && a1 == a2
}

// isStyleInitialized checks if a style has been set (has any foreground, background,
// bold, or padding). A zero-value lipgloss.Style has NoColor{} for fg/bg.
func isStyleInitialized(s lipgloss.Style) bool {
	fg := s.GetForeground()
	bg := s.GetBackground()
	noCol := lipgloss.NoColor{}
	fgSet := !colorsEqual(fg, noCol)
	bgSet := !colorsEqual(bg, noCol)
	boldSet := s.GetBold()
	top, right, bottom, left := s.GetPadding()
	padSet := top != 0 || right != 0 || bottom != 0 || left != 0
	return fgSet || bgSet || boldSet || padSet
}

// ===========================================================================
// RowColorStyle — status coverage
// ===========================================================================

func TestRowColorStyle_GreenStatuses(t *testing.T) {
	greenStatuses := []string{"running", "available", "active", "in-use"}
	for _, s := range greenStatuses {
		style := styles.RowColorStyle(s)
		fg := style.GetForeground()
		if !colorsEqual(fg, styles.ColRunning) {
			t.Errorf("RowColorStyle(%q): expected foreground ColRunning, got different color", s)
		}
	}
}

func TestRowColorStyle_RedStatuses(t *testing.T) {
	redStatuses := []string{"stopped", "failed", "error", "deleting", "deleted"}
	for _, s := range redStatuses {
		style := styles.RowColorStyle(s)
		fg := style.GetForeground()
		if !colorsEqual(fg, styles.ColStopped) {
			t.Errorf("RowColorStyle(%q): expected foreground ColStopped, got different color", s)
		}
	}
}

func TestRowColorStyle_YellowStatuses(t *testing.T) {
	yellowStatuses := []string{"pending", "creating", "modifying", "updating"}
	for _, s := range yellowStatuses {
		style := styles.RowColorStyle(s)
		fg := style.GetForeground()
		if !colorsEqual(fg, styles.ColPending) {
			t.Errorf("RowColorStyle(%q): expected foreground ColPending, got different color", s)
		}
	}
}

func TestRowColorStyle_DimStatuses(t *testing.T) {
	dimStatuses := []string{"terminated", "shutting-down"}
	for _, s := range dimStatuses {
		style := styles.RowColorStyle(s)
		fg := style.GetForeground()
		if !colorsEqual(fg, styles.ColTerminated) {
			t.Errorf("RowColorStyle(%q): expected foreground ColTerminated, got different color", s)
		}
	}
}

func TestRowColorStyle_UnknownStatus(t *testing.T) {
	unknownStatuses := []string{"unknown", "", "something-weird", "n/a"}
	for _, s := range unknownStatuses {
		style := styles.RowColorStyle(s)
		fg := style.GetForeground()
		if !colorsEqual(fg, styles.ColHeaderFg) {
			t.Errorf("RowColorStyle(%q): expected foreground ColHeaderFg, got different color", s)
		}
	}
}

// ===========================================================================
// RowColorStyle — case insensitivity
// ===========================================================================

func TestRowColorStyle_CaseInsensitive(t *testing.T) {
	cases := []struct {
		input    string
		expected color.Color
	}{
		{"Running", styles.ColRunning},
		{"RUNNING", styles.ColRunning},
		{"running", styles.ColRunning},
		{"rUnNiNg", styles.ColRunning},
		{"Available", styles.ColRunning},
		{"AVAILABLE", styles.ColRunning},
		{"Active", styles.ColRunning},
		{"ACTIVE", styles.ColRunning},
		{"In-Use", styles.ColRunning},
		{"IN-USE", styles.ColRunning},
		{"Stopped", styles.ColStopped},
		{"STOPPED", styles.ColStopped},
		{"Failed", styles.ColStopped},
		{"FAILED", styles.ColStopped},
		{"Error", styles.ColStopped},
		{"ERROR", styles.ColStopped},
		{"Deleting", styles.ColStopped},
		{"DELETING", styles.ColStopped},
		{"Deleted", styles.ColStopped},
		{"DELETED", styles.ColStopped},
		{"Pending", styles.ColPending},
		{"PENDING", styles.ColPending},
		{"Creating", styles.ColPending},
		{"CREATING", styles.ColPending},
		{"Modifying", styles.ColPending},
		{"MODIFYING", styles.ColPending},
		{"Updating", styles.ColPending},
		{"UPDATING", styles.ColPending},
		{"Terminated", styles.ColTerminated},
		{"TERMINATED", styles.ColTerminated},
		{"Shutting-Down", styles.ColTerminated},
		{"SHUTTING-DOWN", styles.ColTerminated},
	}
	for _, tc := range cases {
		style := styles.RowColorStyle(tc.input)
		fg := style.GetForeground()
		if !colorsEqual(fg, tc.expected) {
			t.Errorf("RowColorStyle(%q): expected matching color, got different", tc.input)
		}
	}
}

// ===========================================================================
// RowColorStyle — returns non-nil for all inputs
// ===========================================================================

func TestRowColorStyle_NeverReturnsZeroStyle(t *testing.T) {
	allStatuses := []string{
		"running", "available", "active", "in-use",
		"stopped", "failed", "error", "deleting", "deleted",
		"pending", "creating", "modifying", "updating",
		"terminated", "shutting-down",
		"unknown", "", "anything", "RUNNING", "Stopped",
	}
	for _, s := range allStatuses {
		style := styles.RowColorStyle(s)
		if !isStyleInitialized(style) {
			t.Errorf("RowColorStyle(%q): returned uninitialized Style", s)
		}
	}
}

// ===========================================================================
// NoColorActive
// ===========================================================================

func TestNoColorActive_WithEnvSet(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	if !styles.NoColorActive() {
		t.Error("NoColorActive() should return true when NO_COLOR=1")
	}
}

func TestNoColorActive_WithEnvEmpty(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	if styles.NoColorActive() {
		t.Error("NoColorActive() should return false when NO_COLOR is empty")
	}
}

func TestNoColorActive_WithEnvUnset(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	if styles.NoColorActive() {
		t.Error("NoColorActive() should return false when NO_COLOR is unset")
	}
}

// ===========================================================================
// Composed styles — non-zero after init
// ===========================================================================

func TestComposedStyles_NonZeroAfterInit(t *testing.T) {
	// Ensure NO_COLOR is unset so styles actually get initialized.
	os.Unsetenv("NO_COLOR")
	// Force re-init since init() may have already run with NO_COLOR set.
	styles.Reinit()

	namedStyles := map[string]lipgloss.Style{
		"HeaderStyle":   styles.HeaderStyle,
		"TableHeader":   styles.TableHeader,
		"RowSelected":   styles.RowSelected,
		"RowNormal":     styles.RowNormal,
		"RowAlt":        styles.RowAlt,
		"BorderNormal":  styles.BorderNormal,
		"BorderFocused": styles.BorderFocused,
		"DetailKey":     styles.DetailKey,
		"DetailVal":     styles.DetailVal,
		"DetailSection": styles.DetailSection,
		"FlashSuccess":  styles.FlashSuccess,
		"FlashError":    styles.FlashError,
		"FilterActive":  styles.FilterActive,
		"DimText":       styles.DimText,
		"SpinnerStyle":  styles.SpinnerStyle,
	}
	for name, s := range namedStyles {
		if !isStyleInitialized(s) {
			t.Errorf("composed style %q is zero-value after init (NO_COLOR not set)", name)
		}
	}
}

// ===========================================================================
// RowSelected style properties
// ===========================================================================

func TestRowSelected_Properties(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	s := styles.RowSelected
	bg := s.GetBackground()
	fg := s.GetForeground()
	bold := s.GetBold()

	if !colorsEqual(bg, styles.ColRowSelectedBg) {
		t.Errorf("RowSelected background: expected ColRowSelectedBg, got different color")
	}
	if !colorsEqual(fg, styles.ColRowSelectedFg) {
		t.Errorf("RowSelected foreground: expected ColRowSelectedFg, got different color")
	}
	if !bold {
		t.Error("RowSelected should be bold")
	}
}

// ===========================================================================
// TableHeader style properties
// ===========================================================================

func TestTableHeader_Properties(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	s := styles.TableHeader
	fg := s.GetForeground()
	bold := s.GetBold()

	if !colorsEqual(fg, styles.ColAccent) {
		t.Errorf("TableHeader foreground: expected ColAccent, got different color")
	}
	if !bold {
		t.Error("TableHeader should be bold")
	}
}

// ===========================================================================
// Palette completeness — design.md cross-reference
// ===========================================================================

func TestPalette_DesignSpecColors(t *testing.T) {
	// Verify all named palette colors have the correct hex values from design.md.
	checks := []struct {
		name    string
		got     color.Color
		wantHex string
	}{
		{"ColHeaderFg", styles.ColHeaderFg, "#c0caf5"},
		{"ColAccent", styles.ColAccent, "#7aa2f7"},
		{"ColDim", styles.ColDim, "#565f89"},
		{"ColBorder", styles.ColBorder, "#414868"},
		{"ColRowSelectedBg", styles.ColRowSelectedBg, "#7aa2f7"},
		{"ColRowSelectedFg", styles.ColRowSelectedFg, "#1a1b26"},
		{"ColRowAltBg", styles.ColRowAltBg, "#1e2030"},
		{"ColRunning", styles.ColRunning, "#9ece6a"},
		{"ColStopped", styles.ColStopped, "#f7768e"},
		{"ColPending", styles.ColPending, "#e0af68"},
		{"ColTerminated", styles.ColTerminated, "#565f89"},
		{"ColDetailKey", styles.ColDetailKey, "#7aa2f7"},
		{"ColDetailVal", styles.ColDetailVal, "#c0caf5"},
		{"ColDetailSec", styles.ColDetailSec, "#e0af68"},
		{"ColYAMLKey", styles.ColYAMLKey, "#7aa2f7"},
		{"ColYAMLStr", styles.ColYAMLStr, "#9ece6a"},
		{"ColYAMLNum", styles.ColYAMLNum, "#ff9e64"},
		{"ColYAMLBool", styles.ColYAMLBool, "#bb9af7"},
		{"ColYAMLNull", styles.ColYAMLNull, "#565f89"},
		{"ColYAMLTree", styles.ColYAMLTree, "#414868"},
		{"ColHelpKey", styles.ColHelpKey, "#9ece6a"},
		{"ColHelpCat", styles.ColHelpCat, "#e0af68"},
		{"ColFilter", styles.ColFilter, "#e0af68"},
		{"ColSuccess", styles.ColSuccess, "#9ece6a"},
		{"ColError", styles.ColError, "#f7768e"},
		{"ColSpinner", styles.ColSpinner, "#7aa2f7"},
		{"ColScroll", styles.ColScroll, "#414868"},
		{"ColKeyHintKey", styles.ColKeyHintKey, "#7aa2f7"},
		{"ColKeyHintBg", styles.ColKeyHintBg, "#24283b"},
		{"ColKeyHintFg", styles.ColKeyHintFg, "#565f89"},
		{"ColWarning", styles.ColWarning, "#e0af68"},
		{"ColOverlayBg", styles.ColOverlayBg, "#1a1b26"},
		{"ColOverlayBorder", styles.ColOverlayBorder, "#7aa2f7"},
	}
	for _, c := range checks {
		want := lipgloss.Color(c.wantHex)
		if !colorsEqual(c.got, want) {
			t.Errorf("palette %s: hex value mismatch (expected %s)", c.name, c.wantHex)
		}
	}
}

// ===========================================================================
// FlashSuccess and FlashError style properties
// ===========================================================================

func TestFlashSuccess_Properties(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	s := styles.FlashSuccess
	fg := s.GetForeground()
	bold := s.GetBold()

	if !colorsEqual(fg, styles.ColSuccess) {
		t.Errorf("FlashSuccess foreground: expected ColSuccess, got different color")
	}
	if !bold {
		t.Error("FlashSuccess should be bold")
	}
}

func TestFlashError_Properties(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	s := styles.FlashError
	fg := s.GetForeground()
	bold := s.GetBold()

	if !colorsEqual(fg, styles.ColError) {
		t.Errorf("FlashError foreground: expected ColError, got different color")
	}
	if !bold {
		t.Error("FlashError should be bold")
	}
}

// ===========================================================================
// DetailSection style properties
// ===========================================================================

func TestDetailSection_Properties(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	s := styles.DetailSection
	fg := s.GetForeground()
	bold := s.GetBold()

	if !colorsEqual(fg, styles.ColDetailSec) {
		t.Errorf("DetailSection foreground: expected ColDetailSec, got different color")
	}
	if !bold {
		t.Error("DetailSection should be bold")
	}
}

// ===========================================================================
// FilterActive style properties
// ===========================================================================

func TestFilterActive_Properties(t *testing.T) {
	os.Unsetenv("NO_COLOR")
	styles.Reinit()

	s := styles.FilterActive
	fg := s.GetForeground()
	bold := s.GetBold()

	if !colorsEqual(fg, styles.ColFilter) {
		t.Errorf("FilterActive foreground: expected ColFilter, got different color")
	}
	if !bold {
		t.Error("FilterActive should be bold")
	}
}

// ===========================================================================
// NO_COLOR leaves composed styles at zero
// ===========================================================================

func TestComposedStyles_ZeroWithNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	styles.Reinit()

	// When NO_COLOR is set, all composed styles should remain zero
	// (Reinit resets then returns early before assigning them).
	namedStyles := map[string]lipgloss.Style{
		"TableHeader":   styles.TableHeader,
		"RowSelected":   styles.RowSelected,
		"RowNormal":     styles.RowNormal,
		"RowAlt":        styles.RowAlt,
		"BorderNormal":  styles.BorderNormal,
		"BorderFocused": styles.BorderFocused,
		"DetailKey":     styles.DetailKey,
		"DetailVal":     styles.DetailVal,
		"DetailSection": styles.DetailSection,
		"FlashSuccess":  styles.FlashSuccess,
		"FlashError":    styles.FlashError,
		"FilterActive":  styles.FilterActive,
		"DimText":       styles.DimText,
		"SpinnerStyle":  styles.SpinnerStyle,
	}
	for name, s := range namedStyles {
		if isStyleInitialized(s) {
			t.Errorf("composed style %q should be zero when NO_COLOR is set, but it is not", name)
		}
	}

	// Restore for other tests.
	os.Unsetenv("NO_COLOR")
	styles.Reinit()
}
