// One failed read of the local AWS config file, two lanes that flash it.
package unit

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// wipfixUnreadableConfigs are the two ways the local AWS config file can
// refuse to be read: something that is not a file in its place, and a file
// nobody may open.
func wipfixUnreadableConfigs() []struct {
	name  string
	path  func(t *testing.T) string
	cause string
} {
	return []struct {
		name  string
		path  func(t *testing.T) string
		cause string
	}{
		{"a directory in the config file's place", func(t *testing.T) string {
			return t.TempDir()
		}, "is a directory"},
		{"a config file nobody may open", func(t *testing.T) string {
			p := filepath.Join(t.TempDir(), "config")
			if err := os.WriteFile(p, []byte("[profile example-readonly]\nregion = us-east-1\n"), 0o000); err != nil {
				t.Fatalf("seeding config: %v", err)
			}
			if _, err := os.ReadFile(p); err == nil {
				t.Skip("this user can read a mode-000 file, so there is no failure to phrase")
			}
			return p
		}, "permission denied"},
	}
}

// TestFailedProfileRead_ReadsTheSameOnBothLanes pins row 57. A failed read of
// the local AWS config is one failure with one sentence. The controller's
// profile selector and the terminal host's profile fetch are two lanes over
// that one failure, and an operator who meets it on one lane and then the
// other must not be told two different things about the same file. The
// sentence is the one the landed contract already carries — the local AWS
// config named, then the file's own words — and a caller that adds words to
// what the formatter returned is a second owner of it.
func TestFailedProfileRead_ReadsTheSameOnBothLanes(t *testing.T) {
	for _, tc := range wipfixUnreadableConfigs() {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("AWS_CONFIG_FILE", tc.path(t))

			c := newTestController(t)
			vs, _ := c.OpenProfileSelector()
			fromController := vs.Header.Flash.Text

			m := tui.New("", "")
			_, cmd := m.Update(messages.Navigate{Target: messages.TargetProfile})
			flash, ok := walkForFlash(cmd)
			if !ok {
				t.Fatalf("the terminal host's profile fetch raised no flash for a config it "+
					"could not read (%s)", tc.name)
			}
			fromTUI := flash.Text

			if fromController != fromTUI {
				t.Errorf("one failed read of the local AWS config reads two ways:\n"+
					"  profile selector: %q\n"+
					"  terminal host:    %q\n"+
					"the operator meets the same file through both, and only one of them can "+
					"be the sentence", fromController, fromTUI)
			}
			for _, lane := range []struct {
				what    string
				text    string
				isError bool
			}{
				{"profile selector", fromController, vs.Header.Flash.IsError},
				{"terminal host", fromTUI, flash.IsError},
			} {
				text := lane.text
				if !strings.HasPrefix(text, "local AWS config: ") {
					t.Errorf("the %s flashes %q — the sentence names the file the operator has "+
						"to go and fix", lane.what, text)
				}
				if !strings.Contains(text, tc.cause) {
					t.Errorf("the %s flashes %q, without the file's own words (%q)",
						lane.what, text, tc.cause)
				}
				if !lane.isError {
					t.Errorf("the %s flashes %q without marking it an error", lane.what, text)
				}
			}
		})
	}
}

// TestFailedProfileRead_NeitherLaneAddsWordsToTheFormatter is the shape half of
// row 57. Equal strings today can drift apart again the moment either flash
// site is edited, because each builds its own sentence out of a shared
// fragment. The sentence has one owner; a flash site hands the error over and
// flashes what comes back, with no literal of its own.
func TestFailedProfileRead_NeitherLaneAddsWordsToTheFormatter(t *testing.T) {
	for _, site := range []struct {
		rel  string
		what string
	}{
		{"core/app/profile_selector.go", "the profile selector"},
		{"internal/tui/fetch_adapter.go", "the terminal host's profile fetch"},
	} {
		src := wipfixReadRepoFile(t, site.rel)
		for _, line := range strings.Split(src, "\n") {
			if !strings.Contains(line, "MessageOf(err)") {
				continue
			}
			if strings.Contains(line, `"`) {
				t.Errorf("%s builds the sentence at its flash site:\n  %s\n"+
					"a literal here is a second owner of the phrasing, which is how the two "+
					"lanes came to say different things about one file — the formatter returns "+
					"the whole sentence and the site flashes it unchanged",
					site.what, strings.TrimSpace(line))
			}
		}
	}
}
