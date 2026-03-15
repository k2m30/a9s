package ui

import (
	"fmt"

	"github.com/k2m30/a9s/internal/styles"
)

// StatusMode represents the current mode of the status bar.
type StatusMode int

const (
	// NormalMode shows default key hints.
	NormalMode StatusMode = iota
	// CommandMode shows the command input prefixed with ":".
	CommandMode
	// FilterMode shows the filter input prefixed with "/".
	FilterMode
	// ErrorMode shows an error message.
	ErrorMode
	// LoadingMode shows a loading indicator.
	LoadingMode
)

// RenderStatusBar renders the bottom status bar with mode-dependent content.
//   - NormalMode: key hint summary
//   - CommandMode: ":" followed by text
//   - FilterMode: "/" followed by text and match count
//   - ErrorMode: error text in red
//   - LoadingMode: loading indicator with spinner
func RenderStatusBar(mode StatusMode, text string, matchCount int, isError bool, width int) string {
	var content string

	switch mode {
	case NormalMode:
		content = "? help  : command  / filter  q quit"
	case CommandMode:
		content = ":" + text
	case FilterMode:
		content = fmt.Sprintf("/%s (%d matches)", text, matchCount)
	case ErrorMode:
		if width > 0 {
			return styles.ErrorStyle.Width(width).Render(text)
		}
		return styles.ErrorStyle.Render(text)
	case LoadingMode:
		content = "Loading... \u28BE"
	default:
		content = ""
	}

	if width > 0 {
		return styles.StatusBarStyle.Width(width).Render(content)
	}
	return styles.StatusBarStyle.Render(content)
}
