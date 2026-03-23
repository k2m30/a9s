package unit

import (
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/internal/tui/keys"
)

// keyMsgFromText creates a tea.KeyPressMsg for a printable character.
func keyMsgFromText(char string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Code: -1, Text: char}
}

// ===========================================================================
// Step 2: Child-view trigger key bindings
// ===========================================================================

func TestKeys_Events(t *testing.T) {
	k := keys.Default()
	if !key.Matches(keyMsgFromText("e"), k.Events) {
		t.Error("'e' should match Events binding")
	}
}

func TestKeys_Logs(t *testing.T) {
	k := keys.Default()
	if !key.Matches(keyMsgFromText("L"), k.Logs) {
		t.Error("'L' should match Logs binding")
	}
}

func TestKeys_Resources(t *testing.T) {
	k := keys.Default()
	if !key.Matches(keyMsgFromText("r"), k.Resources) {
		t.Error("'r' should match Resources binding")
	}
}

func TestKeys_Source(t *testing.T) {
	k := keys.Default()
	if !key.Matches(keyMsgFromText("s"), k.Source) {
		t.Error("'s' should match Source binding")
	}
}

