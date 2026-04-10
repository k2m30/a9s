package styles

import "image/color"

// Tokyo Night Dark palette.
var (
	ColHeaderFg color.Color
	ColAccent   color.Color
	ColDim      color.Color
	ColBorder   color.Color

	ColRowSelectedBg color.Color
	ColRowSelectedFg color.Color
	ColRowAltBg      color.Color

	ColRunning    color.Color
	ColStopped    color.Color
	ColPending    color.Color
	ColTerminated color.Color

	ColDetailKey color.Color
	ColDetailVal color.Color
	ColDetailSec color.Color

	ColYAMLKey  color.Color
	ColYAMLStr  color.Color
	ColYAMLNum  color.Color
	ColYAMLBool color.Color
	ColYAMLNull color.Color
	ColYAMLTree color.Color

	ColHelpKey color.Color
	ColHelpCat color.Color

	ColFilter  color.Color
	ColSuccess color.Color
	ColError   color.Color

	ColSpinner color.Color
	ColScroll  color.Color

	ColKeyHintKey color.Color
	ColKeyHintBg  color.Color
	ColKeyHintFg  color.Color

	ColWarning       color.Color
	ColOverlayBg     color.Color
	ColOverlayBorder color.Color
)

func applyPalette(t Theme) {
	ColHeaderFg = t.HeaderFg
	ColAccent = t.Accent
	ColDim = t.Dim
	ColBorder = t.Border
	ColRowSelectedBg = t.RowSelectedBg
	ColRowSelectedFg = t.RowSelectedFg
	ColRowAltBg = t.RowAltBg
	ColRunning = t.Running
	ColStopped = t.Stopped
	ColPending = t.Pending
	ColTerminated = t.Terminated
	ColDetailKey = t.DetailKey
	ColDetailVal = t.DetailVal
	ColDetailSec = t.DetailSec
	ColYAMLKey = t.YAMLKey
	ColYAMLStr = t.YAMLStr
	ColYAMLNum = t.YAMLNum
	ColYAMLBool = t.YAMLBool
	ColYAMLNull = t.YAMLNull
	ColYAMLTree = t.YAMLTree
	ColHelpKey = t.HelpKey
	ColHelpCat = t.HelpCat
	ColFilter = t.Filter
	ColSuccess = t.Success
	ColError = t.Error
	ColSpinner = t.Spinner
	ColScroll = t.Scroll
	ColKeyHintKey = t.KeyHintKey
	ColKeyHintBg = t.KeyHintBg
	ColKeyHintFg = t.KeyHintFg
	ColWarning = t.Warning
	ColOverlayBg = t.OverlayBg
	ColOverlayBorder = t.OverlayBorder
}
