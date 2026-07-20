// SPDX-License-Identifier: GPL-3.0-or-later

package tui

import (
	"runtime"
	"strings"
	"testing"
)

const browserOpenTestURL = "https://console.aws.amazon.com/ec2/home?region=us-east-1#InstanceDetails:instanceId=i-0abc123"

// wantDefaultOpenerArgv returns the argv this test's own runtime.GOOS should
// produce when $BROWSER is empty/whitespace-only — mirroring the exact
// per-GOOS switch in browserOpenCommand (darwin -> "open", windows ->
// "rundll32" + fixed arg, else -> "xdg-open"), always ending in the URL.
func wantDefaultOpenerArgv(url string) []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"open", url}
	case "windows":
		return []string{"rundll32", "url.dll,FileProtocolHandler", url}
	default:
		return []string{"xdg-open", url}
	}
}

func assertCmdArgs(t *testing.T, gotArgs, wantArgs []string) {
	t.Helper()
	if len(gotArgs) != len(wantArgs) {
		t.Fatalf("Args = %v, want %v", gotArgs, wantArgs)
	}
	for i := range wantArgs {
		if gotArgs[i] != wantArgs[i] {
			t.Errorf("Args[%d] = %q, want %q (full: got=%v want=%v)", i, gotArgs[i], wantArgs[i], gotArgs, wantArgs)
		}
	}
}

// assertCmdPathBase checks the resolved exec.Cmd.Path's base name, rather
// than an exact string match: exec.Command runs argv[0] through LookPath
// when it has no path separators, so on a system where that binary exists
// (e.g. "open" on darwin) Path becomes an absolute resolved path instead of
// the literal name — Args is unaffected either way and always exact.
func assertCmdPathBase(t *testing.T, gotPath, wantName string) {
	t.Helper()
	if gotPath != wantName && !strings.HasSuffix(gotPath, "/"+wantName) {
		t.Errorf("Path = %q, want %q or a LookPath-resolved path ending in /%q", gotPath, wantName, wantName)
	}
}

func TestBrowserOpenCommand_BrowserWithOneFlag_SplitsArgv(t *testing.T) {
	t.Setenv("BROWSER", "firefox --new-tab")

	cmd := browserOpenCommand(browserOpenTestURL)

	assertCmdPathBase(t, cmd.Path, "firefox")
	assertCmdArgs(t, cmd.Args, []string{"firefox", "--new-tab", browserOpenTestURL})
}

func TestBrowserOpenCommand_BrowserWithMultipleFlags_SplitsArgv(t *testing.T) {
	t.Setenv("BROWSER", "open -a Safari")

	cmd := browserOpenCommand(browserOpenTestURL)

	assertCmdPathBase(t, cmd.Path, "open")
	assertCmdArgs(t, cmd.Args, []string{"open", "-a", "Safari", browserOpenTestURL})
}

func TestBrowserOpenCommand_BrowserPlainName_AppendsURLAsSoleArg(t *testing.T) {
	t.Setenv("BROWSER", "mybrowser")

	cmd := browserOpenCommand(browserOpenTestURL)

	assertCmdPathBase(t, cmd.Path, "mybrowser")
	assertCmdArgs(t, cmd.Args, []string{"mybrowser", browserOpenTestURL})
}

func TestBrowserOpenCommand_BrowserWhitespaceOnly_FallsBackToDefaultOpener(t *testing.T) {
	t.Setenv("BROWSER", "   ")

	cmd := browserOpenCommand(browserOpenTestURL)

	want := wantDefaultOpenerArgv(browserOpenTestURL)
	assertCmdPathBase(t, cmd.Path, want[0])
	assertCmdArgs(t, cmd.Args, want)
}

func TestBrowserOpenCommand_BrowserUnset_FallsBackToDefaultOpener(t *testing.T) {
	// os.Getenv cannot distinguish "unset" from "set to empty string" —
	// browserOpenCommand only ever calls os.Getenv, so this is the faithful
	// stand-in for an unset $BROWSER.
	t.Setenv("BROWSER", "")

	cmd := browserOpenCommand(browserOpenTestURL)

	want := wantDefaultOpenerArgv(browserOpenTestURL)
	assertCmdPathBase(t, cmd.Path, want[0])
	assertCmdArgs(t, cmd.Args, want)
}
