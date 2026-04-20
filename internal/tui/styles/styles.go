package styles

import (
	"os"

	lipgloss "charm.land/lipgloss/v2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// Composed styles built from the Tokyo Night Dark palette.
var (
	HeaderStyle    lipgloss.Style
	TableHeader    lipgloss.Style
	RowSelected    lipgloss.Style
	RowNormal      lipgloss.Style
	RowAlt         lipgloss.Style
	BorderNormal   lipgloss.Style
	BorderFocused  lipgloss.Style
	DetailKey      lipgloss.Style
	DetailVal      lipgloss.Style
	DetailSection  lipgloss.Style
	FlashSuccess   lipgloss.Style
	FlashError     lipgloss.Style
	FilterActive   lipgloss.Style
	DimText        lipgloss.Style
	SpinnerStyle   lipgloss.Style
	NavigableField lipgloss.Style
	ColSepDim      lipgloss.Style // │ separator when left column is focused
	ColSepAccent   lipgloss.Style // │ separator when right column is focused

	StatusCheckFailed lipgloss.Style // "!" glyph — RED bold (impaired)
	StatusCheckWarn   lipgloss.Style // "~" glyph — YELLOW (initializing)
	StatusCheckOk     lipgloss.Style // GREEN (ok values in detail view)

	// FindingSection styles for enrichment section headers in the detail view.
	FindingSectionStopped lipgloss.Style // bold + red — used for "!" tier sections
	FindingSectionPending lipgloss.Style // bold + yellow — used for "~" tier sections
	FindingSectionDefault lipgloss.Style // bold — used for sections with no tier

	// BannerInfo is the style for informational banners in the resource list view.
	BannerInfo lipgloss.Style

	HelpCatStyle         lipgloss.Style
	HelpKeyStyle         lipgloss.Style
	HelpDescStyle        lipgloss.Style
	IdentitySectionStyle lipgloss.Style
	IdentityLabelStyle   lipgloss.Style
	IdentityValueStyle   lipgloss.Style
	YAMLKeyStyle         lipgloss.Style
	YAMLStrStyle         lipgloss.Style
	YAMLNumStyle         lipgloss.Style
	YAMLBoolStyle        lipgloss.Style
	YAMLNullStyle        lipgloss.Style
	SearchCurrentStyle   lipgloss.Style
	SearchOtherStyle     lipgloss.Style
)

// NoColorActive reports whether NO_COLOR is set in the environment.
func NoColorActive() bool {
	_, ok := os.LookupEnv("NO_COLOR")
	return ok
}

// ColorStyle maps a resource.Color to the palette's foreground style.
// ColorHealthy → ColRunning (green), ColorWarning → ColPending (yellow),
// ColorBroken → ColStopped (red), ColorDim → ColTerminated (grey).
// Respects NO_COLOR (returns an empty style when active).
func ColorStyle(c resource.Color) lipgloss.Style {
	if NoColorActive() {
		return lipgloss.NewStyle()
	}
	switch c {
	case resource.ColorHealthy:
		return lipgloss.NewStyle().Foreground(ColRunning)
	case resource.ColorWarning:
		return lipgloss.NewStyle().Foreground(ColPending)
	case resource.ColorBroken:
		return lipgloss.NewStyle().Foreground(ColStopped)
	case resource.ColorDim:
		return lipgloss.NewStyle().Foreground(ColTerminated)
	}
	return lipgloss.NewStyle()
}

// TierColorStyle maps a detail-view ColorTier string to a foreground style.
// ColorTier is a free-form string used on FieldItem for detail-row coloring
// (EC2 status checks, enrichment findings). The mapping preserves pre-refactor
// behavior for the known tier values.
func TierColorStyle(tier string) lipgloss.Style {
	if NoColorActive() {
		return lipgloss.NewStyle()
	}
	switch tier {
	case "ok":
		return lipgloss.NewStyle().Foreground(ColRunning)
	case "!", "impaired", "ct-danger":
		return lipgloss.NewStyle().Foreground(ColStopped)
	case "~", "initializing", "ct-attention":
		return lipgloss.NewStyle().Foreground(ColPending)
	case "ct-info":
		return lipgloss.NewStyle().Foreground(ColTerminated)
	}
	return lipgloss.NewStyle().Foreground(ColHeaderFg)
}

func init() {
	applyPalette(DefaultTheme())
	initStyles()
}

// Reinit re-initializes all composed styles. Useful for tests that toggle NO_COLOR.
func Reinit() {
	applyPalette(ActiveTheme())
	initStyles()
}

func initStyles() {
	// Reset all styles to zero values first.
	HeaderStyle = lipgloss.Style{}
	TableHeader = lipgloss.Style{}
	RowSelected = lipgloss.Style{}
	RowNormal = lipgloss.Style{}
	RowAlt = lipgloss.Style{}
	BorderNormal = lipgloss.Style{}
	BorderFocused = lipgloss.Style{}
	DetailKey = lipgloss.Style{}
	DetailVal = lipgloss.Style{}
	DetailSection = lipgloss.Style{}
	FlashSuccess = lipgloss.Style{}
	FlashError = lipgloss.Style{}
	FilterActive = lipgloss.Style{}
	DimText = lipgloss.Style{}
	SpinnerStyle = lipgloss.Style{}
	NavigableField = lipgloss.Style{}
	ColSepDim = lipgloss.Style{}
	ColSepAccent = lipgloss.Style{}
	StatusCheckFailed = lipgloss.Style{}
	StatusCheckWarn = lipgloss.Style{}
	StatusCheckOk = lipgloss.Style{}
	FindingSectionStopped = lipgloss.Style{}
	FindingSectionPending = lipgloss.Style{}
	FindingSectionDefault = lipgloss.Style{}
	BannerInfo = lipgloss.Style{}
	HelpCatStyle = lipgloss.Style{}
	HelpKeyStyle = lipgloss.Style{}
	HelpDescStyle = lipgloss.Style{}
	IdentitySectionStyle = lipgloss.Style{}
	IdentityLabelStyle = lipgloss.Style{}
	IdentityValueStyle = lipgloss.Style{}
	YAMLKeyStyle = lipgloss.Style{}
	YAMLStrStyle = lipgloss.Style{}
	YAMLNumStyle = lipgloss.Style{}
	YAMLBoolStyle = lipgloss.Style{}
	YAMLNullStyle = lipgloss.Style{}
	SearchCurrentStyle = lipgloss.Style{}
	SearchOtherStyle = lipgloss.Style{}

	// These 13 styles were previously package-level vars in view files,
	// initialized once at load time and unaffected by NO_COLOR / Reinit().
	// They are always initialized regardless of NO_COLOR to preserve that behavior.
	HelpCatStyle = lipgloss.NewStyle().Foreground(ColHelpCat).Bold(true)
	HelpKeyStyle = lipgloss.NewStyle().Foreground(ColHelpKey).Bold(true)
	HelpDescStyle = lipgloss.NewStyle().Foreground(ColDetailVal)
	IdentitySectionStyle = lipgloss.NewStyle().Foreground(ColDetailSec).Bold(true)
	IdentityLabelStyle = lipgloss.NewStyle().Foreground(ColDim)
	IdentityValueStyle = lipgloss.NewStyle().Foreground(ColDetailVal)
	YAMLKeyStyle = lipgloss.NewStyle().Foreground(ColYAMLKey)
	YAMLStrStyle = lipgloss.NewStyle().Foreground(ColYAMLStr)
	YAMLNumStyle = lipgloss.NewStyle().Foreground(ColYAMLNum)
	YAMLBoolStyle = lipgloss.NewStyle().Foreground(ColYAMLBool)
	YAMLNullStyle = lipgloss.NewStyle().Foreground(ColYAMLNull)
	SearchCurrentStyle = lipgloss.NewStyle().Foreground(ActiveTheme().SearchHighlightFg).Background(ActiveTheme().SearchHighlightBg)
	SearchOtherStyle = lipgloss.NewStyle().Underline(true).Foreground(ActiveTheme().SearchHighlightBg)

	if NoColorActive() {
		RowSelected = lipgloss.NewStyle().Reverse(true)
		return
	}

	HeaderStyle = lipgloss.NewStyle().Padding(0, 1)
	TableHeader = lipgloss.NewStyle().Foreground(ColAccent).Bold(true)
	RowSelected = lipgloss.NewStyle().Background(ColRowSelectedBg).Foreground(ColRowSelectedFg).Bold(true)
	RowNormal = lipgloss.NewStyle().Foreground(ColHeaderFg)
	RowAlt = lipgloss.NewStyle().Foreground(ColHeaderFg).Background(ColRowAltBg)
	BorderNormal = lipgloss.NewStyle().Foreground(ColBorder)
	BorderFocused = lipgloss.NewStyle().Foreground(ColAccent)
	DetailKey = lipgloss.NewStyle().Foreground(ColDetailKey)
	DetailVal = lipgloss.NewStyle().Foreground(ColDetailVal)
	DetailSection = lipgloss.NewStyle().Foreground(ColDetailSec).Bold(true)
	FlashSuccess = lipgloss.NewStyle().Foreground(ColSuccess).Bold(true)
	FlashError = lipgloss.NewStyle().Foreground(ColError).Bold(true)
	FilterActive = lipgloss.NewStyle().Foreground(ColFilter).Bold(true)
	DimText = lipgloss.NewStyle().Foreground(ColDim)
	SpinnerStyle = lipgloss.NewStyle().Foreground(ColSpinner)
	NavigableField = lipgloss.NewStyle().Foreground(ColAccent).Underline(true)
	ColSepDim = lipgloss.NewStyle().Foreground(ColBorder)
	ColSepAccent = lipgloss.NewStyle().Foreground(ColAccent)
	StatusCheckFailed = lipgloss.NewStyle().Foreground(ColStopped).Bold(true)
	StatusCheckWarn = lipgloss.NewStyle().Foreground(ColPending)
	StatusCheckOk = lipgloss.NewStyle().Foreground(ColRunning)
	FindingSectionStopped = lipgloss.NewStyle().Bold(true).Foreground(ColStopped)
	FindingSectionPending = lipgloss.NewStyle().Bold(true).Foreground(ColPending)
	FindingSectionDefault = lipgloss.NewStyle().Bold(true)
	BannerInfo = lipgloss.NewStyle().Foreground(ColPending).Italic(true)
}
