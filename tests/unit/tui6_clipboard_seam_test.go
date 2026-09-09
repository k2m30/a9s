// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tui6_clipboard_seam_test.go — the reveal screen's two answers, read off the
// seam rather than off the operator's clipboard.
//
// A secret is the one value on any screen the operator takes away to use as it
// is, so the clipboard hands back the bytes AWS stored while the screen shows
// the inert form. That is a deliberate divergence, and a test that cannot see
// what was written cannot tell it from a bug. The write goes through one
// injectable writer, so these pins read the bytes without touching the OS
// clipboard, which is one resource shared by every worktree on the machine.
package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// tui6RevealAndCopy reveals secret through the real root model, presses the
// copy key, and returns the painted screen and whatever reached the writer.
func tui6RevealAndCopy(t *testing.T, secret string) (screen string, written []string) {
	t.Helper()
	t.Cleanup(tui.SetClipboardWriteForTest(func(s string) error {
		written = append(written, s)
		return nil
	}))

	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})
	m, _ = rootApplyMsg(m, messages.ValueRevealed{
		ResourceType: "secret", ResourceID: "prod/db-password", Value: secret,
	})
	screen = ansi.Strip(rootViewContent(m))

	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, rootKeyPress("c"))
	if cmd != nil {
		_, _ = drainCmds(t, m, cmd, 5)
	}
	return screen, written
}

// TestRevealCopy_ScreenIsInertAndTheClipboardIsVerbatim pins both halves of
// the one place the screen and the clipboard legitimately differ.
func TestRevealCopy_ScreenIsInertAndTheClipboardIsVerbatim(t *testing.T) {
	const secret = "pa55\x1b[31mword\x07!"
	screen, written := tui6RevealAndCopy(t, secret)

	if !strings.Contains(screen, "pa55") {
		t.Fatalf("the secret is not on the screen, so this pin proves nothing:\n%s", screen)
	}
	// Line by line: the newlines between them are the screen's own.
	for i, line := range strings.Split(screen, "\n") {
		if ctrls := tui6Controls(line); len(ctrls) > 0 {
			t.Errorf("painted line %d carries control runes %q: %q", i, ctrls, line)
		}
	}
	if strings.Contains(screen, "[31m") {
		t.Errorf("the revealed secret kept its CSI payload as visible text:\n%s", screen)
	}
	if !strings.Contains(screen, "as stored") {
		t.Errorf("the screen shows a value that differs from the clipboard and says nothing about it:\n%s", screen)
	}

	if len(written) != 1 {
		t.Fatalf("the copy key wrote the clipboard %d times, want once: %q", len(written), written)
	}
	if written[0] != secret {
		t.Errorf("the clipboard carries %q; a secret has to be the bytes AWS stored, %q", written[0], secret)
	}
}

// TestRevealCopy_CleanSecretIsUnannotated is the counterpart: a secret with
// nothing to strip looks exactly as it did and earns no note, so the note
// means what it says when it appears.
func TestRevealCopy_CleanSecretIsUnannotated(t *testing.T) {
	const secret = "correct-horse-battery-staple"
	screen, written := tui6RevealAndCopy(t, secret)

	if !strings.Contains(screen, secret) {
		t.Errorf("a clean secret is not painted as it stands:\n%s", screen)
	}
	if strings.Contains(screen, "as stored") {
		t.Errorf("a clean secret earns a note about a difference there is none of:\n%s", screen)
	}
	if len(written) != 1 || written[0] != secret {
		t.Errorf("the clipboard carries %q, want exactly %q once", written, secret)
	}
}
