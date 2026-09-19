// The terminal main menu's dim "refreshing…" line is a statement about the
// Wave-1 availability sweep in flight, so it must disappear when that sweep
// ends; an operator who sees it for the rest of the session cannot tell a
// finished start-up from a hung one.
//
// A fresh session's AvailabilityGen is seeded at 1, so Gen: 1 is the live
// stamp; messages.AvailabilityChecked.AcceptZeroGen() is false.
package unit

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// menuRefreshingLine is the exact dim line internal/tui/views/mainmenu.go
// appends under the rows while MenuBody.Refreshing is true.
const menuRefreshingLine = "refreshing…"

// probeResultFor builds the AvailabilityChecked a live Wave-1 probe emits for
// a type with no resources: Err nil (the probe succeeded), Count 0,
// HasResources false, Resources nil. That is the shape production sends for
// the great majority of types in any single account, and it is enough to
// count as one landed probe result.
func probeResultFor(shortName string) messages.AvailabilityChecked {
	return messages.AvailabilityChecked{
		ResourceType: shortName,
		HasResources: false,
		Count:        0,
		Gen:          1,
	}
}

func menuRefreshingVisible(m tui.Model) bool {
	return strings.Contains(stripANSI(rootViewContent(m)), menuRefreshingLine)
}

// A cache-seeded start is the state the line exists for: counts on screen that
// no live probe has confirmed yet. Once every queued type has reported there
// is nothing left to confirm.
func TestTerminalMenu_RefreshingClearsWhenSweepCompletes(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const profile, region = "refreshing-tui-profile", "us-east-1"
	seedDiskStoreWithS3Rows(t, profile, region)

	m := newPostSweepApp(t, profile, region)
	// The refreshing line is appended below the last menu row, and the menu
	// lists ~70 types: at the default 40-line test window it falls outside
	// the rendered viewport whatever its state. Give the model a terminal
	// tall enough to show the whole list plus that line.
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 140, Height: 120})

	m, _ = rootApplyMsg(m, messages.AvailabilityCacheLoaded{
		Entries: map[string]int{"s3": 2},
	})

	if !menuRefreshingVisible(m) {
		t.Fatalf("cache-seeded start renders no %q line — the sweep is in flight and the seeded counts are unconfirmed, so the operator must be told:\n%s", menuRefreshingLine, stripANSI(rootViewContent(m)))
	}

	names := resource.AllShortNames()
	if len(names) < 2 {
		t.Fatalf("fixture assumption broken: resource.AllShortNames() = %d types, need at least 2 to leave one outstanding", len(names))
	}

	for _, shortName := range names[:len(names)-1] {
		m, _ = rootApplyMsg(m, probeResultFor(shortName))
	}

	if !menuRefreshingVisible(m) {
		t.Errorf("the %q line vanished with one probe result (%q) still outstanding — the sweep has not finished yet:\n%s", menuRefreshingLine, names[len(names)-1], stripANSI(rootViewContent(m)))
	}

	m, _ = rootApplyMsg(m, probeResultFor(names[len(names)-1]))

	if menuRefreshingVisible(m) {
		t.Errorf("every type's probe result has landed, yet the menu still renders the %q line — it would keep saying so for the rest of the session, and the operator can no longer tell a finished start-up from a stuck one:\n%s", menuRefreshingLine, stripANSI(rootViewContent(m)))
	}
}
