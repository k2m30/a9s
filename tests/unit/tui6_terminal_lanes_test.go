// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// tui6_terminal_lanes_test.go — the terminal's own copies of facts the
// controller already decided.
//
// The adapter keeps a second copy of the flash text and takes it from the
// intent rather than from the snapshot the controller sanitised, and a resize
// reaches the renderer without ever reaching the controller, so the body that
// decided the layout is still the one built for the old width. Both are the
// same shape: the terminal answering a question the controller has already
// answered, and answering it differently.
//
// The sanitiser cases beside them are the one function's own edge cases, where
// the parser has to agree with what a terminal actually does with the bytes.
package unit

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf8"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
)

// TestAdapterFlash_PaintsTheSanitisedText pins the flash lane through the real
// terminal adapter. The controller cleans the text it records; the banner the
// operator reads has to be the same text, not the intent's own copy.
//
// a9s paints its own colour in truecolor form, so a literal \x1b[31m on the
// screen can only have come from the AWS message.
func TestAdapterFlash_PaintsTheSanitisedText(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "s3"})
	m, _ = rootApplyMsg(m, messages.APIError{
		ResourceType: "s3",
		Err:          errors.New("InvalidBucketName: the bucket web\x1b[31m-prod\x07 was rejected"),
	})

	screen := rootViewContent(m)
	if !strings.Contains(screen, "InvalidBucketName") {
		t.Fatalf("the failure raised no banner, so this pin proves nothing:\n%s", screen)
	}
	if strings.Contains(screen, "\x1b[31m") {
		t.Error("the AWS message's own escape sequence reached the painted banner, where it colours everything after it")
	}
	if strings.Contains(screen, "\x07") {
		t.Error("the AWS message's BEL reached the painted banner")
	}
}

// TestAdapterFlash_DoesNotReadTheIntentsText is the gate beside it: the
// adapter must take the flash from the controller's snapshot, so there is no
// second copy to sanitise and no second copy to forget.
func TestAdapterFlash_DoesNotReadTheIntentsText(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate this test file")
	}
	path := filepath.Join(filepath.Dir(thisFile), "..", "..", "internal", "tui", "runtime_adapter.go")
	src, err := os.ReadFile(path) //nolint:gosec // a fixed path inside the repo
	if err != nil {
		t.Fatalf("reading the adapter: %v", err)
	}
	if !strings.Contains(string(src), "FlashIntent") {
		t.Fatal("the adapter no longer handles FlashIntent — this gate is reading the wrong file")
	}
	if strings.Contains(string(src), "= v.Text") {
		t.Error("the adapter assigns the intent's own flash text; it must read the text the controller sanitised, off the snapshot")
	}
}

// TestDetailResize_RewrapsForTheNewWidth pins row 3's class finished. The
// attention sentence is wrapped where the layout is decided, against the
// viewport the controller was told about. A resize that reaches only the
// renderer leaves the sentence wrapped for the old width, and the narrower
// pane then cuts it off mid-word instead of re-wrapping it.
func TestDetailResize_RewrapsForTheNewWidth(t *testing.T) {
	m := newRootSizedModel()
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 36})
	page, err := awsclient.FetchEC2InstancesPage(context.Background(), demo.NewServiceClients().EC2, "")
	if err != nil {
		t.Fatalf("demo ec2 fetch: %v", err)
	}
	m, _ = rootApplyMsg(m, messages.Navigate{Target: messages.TargetResourceList, ResourceType: "ec2"})
	m, _ = rootApplyMsg(m, messages.ResourcesLoaded{
		Provenance: messages.FetchProvenanceCanonicalList, ResourceType: "ec2", Resources: page.Resources,
	})
	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, rootSpecialKey(tea.KeyEnter))
	m, _ = drainCmds(t, m, cmd, 5)

	const lastWords = "addressed directly."
	if wide := ansi.Strip(rootViewContent(m)); !strings.Contains(wide, lastWords) {
		t.Fatalf("the attention sentence does not end with %q at 120 columns, so this pin proves nothing:\n%s", lastWords, wide)
	}

	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 60, Height: 36})
	narrow := ansi.Strip(rootViewContent(m))
	if !strings.Contains(narrow, lastWords) {
		t.Errorf("after shrinking to 60 columns the attention sentence is cut off: it was wrapped for the old width and the new pane clips it instead of re-wrapping.\n%s", narrow)
	}
}

// TestSanitize_ParsesWhatATerminalParses pins the four cases where consuming
// exactly one byte after an introducer disagrees with what a terminal does
// with the same bytes. A clean value is one space where the sequence was.
func TestSanitize_ParsesWhatATerminalParses(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want string
	}{
		{
			// ESC ( B selects the ASCII character set: an intermediate byte
			// and then a final. Stopping after the "(" leaves the "B" on
			// screen as a capital letter nobody wrote.
			name: "an escape with an intermediate and a final",
			in:   "a\x1b(Bb",
			want: "a b",
		},
		{
			// The byte after ESC is the first byte of a two-byte rune.
			// Consuming one byte splits it and leaves an invalid tail, which
			// is a worse output than the input.
			name: "a multi-byte rune after a bare escape",
			in:   "a\x1b\u00e9b",
			want: "a \u00e9b",
		},
		{
			// The second byte of "\u00dc" is 0x9c, which is ST as a control.
			// Scanning the payload byte by byte ends the string control inside
			// a rune and spills the rest of the title onto the screen.
			name: "a continuation byte that reads as ST",
			in:   "a\x1b]0;\u00dctitle\x07b",
			want: "a b",
		},
		{
			// BEL ends an OSC. Inside a CSI it is just a control character,
			// and the sequence runs on to its own final byte.
			name: "BEL ends a string control, not a CSI",
			in:   "a\x1b[31\x07mb",
			want: "a b",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := domain.Sanitize(tc.in)
			if got != tc.want {
				t.Errorf("Sanitize(%q) = %q, want %q", tc.in, got, tc.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("Sanitize(%q) = %q, which is not valid UTF-8", tc.in, got)
			}
			if ctrls := tui6Controls(got); len(ctrls) > 0 {
				t.Errorf("Sanitize(%q) = %q, which still carries control runes %q", tc.in, got, ctrls)
			}
			if again := domain.Sanitize(got); again != got {
				t.Errorf("Sanitize is not idempotent on %q: %q then %q", tc.in, got, again)
			}
		})
	}
}
