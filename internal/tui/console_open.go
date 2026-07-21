// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/consolelink"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// handleOpenConsole resolves the active (typeDef, resource) pair and either
// opens its AWS console URL in a browser (copyOnly=false, the 'o' key) or
// copies it to the clipboard (copyOnly=true, the 'O' key). Screens with no
// resolvable target (menu, costs, help, selector, identity, YAML/JSON,
// reveal) are a silent no-op, matching handleCopy's precedent for the same
// screen kinds.
func (m Model) handleOpenConsole(copyOnly bool) (tea.Model, tea.Cmd) {
	td, res, ok := m.ctrl.ConsoleTarget()
	if !ok {
		return m, nil
	}

	region := m.core.Region()
	accountID := ""
	if id := m.core.Identity(); id != nil {
		accountID = id.AccountID
	}

	consoleURL, resolved := consolelink.Resolve(*td, res, region, accountID)
	if !resolved {
		return m.handleFlash(messages.Flash{Text: fmt.Sprintf("no console link for %s", td.ShortName), IsError: true})
	}

	if copyOnly {
		return m, copyToClipboard(consoleURL, "Copied console URL to clipboard")
	}

	if m.isDemo {
		return m.handleFlash(messages.Flash{Text: "demo mode — console link disabled (O still copies)"})
	}

	return m, openBrowserCmd(consoleURL)
}

// openBrowserCmd returns a tea.Cmd that opens consoleURL in the user's
// browser. $BROWSER wins when set; otherwise the platform's default opener
// is used. The command is exec'd directly (argv form) — never through a
// shell.
func openBrowserCmd(consoleURL string) tea.Cmd {
	return func() tea.Msg {
		if !consolelink.Valid(consoleURL) {
			return messages.Flash{Text: "invalid console URL", IsError: true}
		}
		cmd := browserOpenCommand(consoleURL)
		if err := cmd.Start(); err != nil {
			return messages.Flash{Text: fmt.Sprintf("open failed: %v", err), IsError: true}
		}
		go func() { _ = cmd.Wait() }()
		return messages.Flash{Text: "opened in AWS console", IsError: false}
	}
}

// browserOpenCommand builds the argv-form command that opens consoleURL,
// preferring $BROWSER when set, else the per-GOOS default opener.
func browserOpenCommand(consoleURL string) *exec.Cmd {
	if browser := os.Getenv("BROWSER"); browser != "" {
		if argv := strings.Fields(browser); len(argv) > 0 {
			argv = append(argv, consoleURL)
			return exec.Command(argv[0], argv[1:]...) //nolint:gosec,noctx // G204: argv exec, no shell, URL Valid()-checked upstream (https + console-domain allow-list); $BROWSER is the user's own local opener choice, whitespace-split (no shell quoting); noctx: fire-and-forget browser launch, no cancellation surface
		}
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", consoleURL) //nolint:gosec,noctx // G204: argv exec, no shell, URL Valid()-checked upstream (https + console-domain allow-list); noctx: fire-and-forget browser launch, no cancellation surface
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", consoleURL) //nolint:gosec,noctx // G204: argv exec, no shell, URL Valid()-checked upstream (https + console-domain allow-list); noctx: fire-and-forget browser launch, no cancellation surface
	default:
		return exec.Command("xdg-open", consoleURL) //nolint:gosec,noctx // G204: argv exec, no shell, URL Valid()-checked upstream (https + console-domain allow-list); noctx: fire-and-forget browser launch, no cancellation surface
	}
}
