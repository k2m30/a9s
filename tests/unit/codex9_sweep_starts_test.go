// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// codex9_sweep_starts_test.go — the start-up availability sweep must run to
// completion whichever of the two start-up legs lands first.
//
// A live start fires the disk-cache seed and the AWS connect concurrently
// (tui.Model.Init's tea.Batch), so their arrival order is a race decided by
// local disk I/O against a network handshake. Both orders are production
// orders, so both must leave the operator with the same menu: every
// registered type probed, every row counted, and the "[verifying k/N]"
// counter gone from the frame title once the last probe answers. A sweep
// that stalls at "[verifying 0/N]" for the whole session is the defect these
// pins exist to catch.
//
// Both tests drive the real tui.Model.Update seam, then run every tea.Cmd the
// model returns and feed the resulting messages back, exactly as the Bubble
// Tea event loop would — the probe dispatch under test lives entirely inside
// those returned commands, so a pin that only inspected state after Update
// would not see it at all.
package unit

import (
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

const (
	c9Profile = "codex9-sweep-prof"
	c9Region  = "us-east-1"
)

// newC9Model builds a tui.Model with on-disk caching enabled (the live path —
// WithNoCache would route the connect to the synchronous demo prefetch and
// skip the probe pipeline entirely) over a warm disk cache, and returns it
// alongside the connect message its own Init produced. Taking the connect
// message from Init rather than hand-building one is what makes it carry the
// live connect generation: the handler drops a ClientsReady whose generation
// does not match, so a guessed one would be silently ignored and both
// orderings would look equally broken.
func newC9Model(t *testing.T) (tui.Model, tea.Msg) {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	seedTypeFile(t, c9Profile, c9Region, "s3", 7)

	m := newBlessedModel(t, c9Profile, c9Region,
		tui.WithClients(demo.NewServiceClients()),
		tui.WithIsDemo(true),
		tui.WithProfileForTest(c9Profile),
		tui.WithRegionForTest(c9Region))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 120, Height: 40})

	initCmd := m.Init()
	if initCmd == nil {
		t.Fatal("Init() returned nil — expected the pre-supplied-clients connect leg")
	}
	connect := initCmd()
	if _, ok := connect.(messages.ClientsReady); !ok {
		t.Fatalf("Init() produced %T, want messages.ClientsReady", connect)
	}
	return m, connect
}

// c9AssertSweepFinished asserts the two things the operator reads off the
// frame title once the start-up sweep has run: the counter moved off its
// "0/N" start (probe results landed at all) and no counter is left (every
// registered type answered).
func c9AssertSweepFinished(t *testing.T, m tui.Model, ordering string) {
	t.Helper()
	content := stripANSI(rootViewContent(m))
	total := len(resource.AllShortNames())
	stalled := "verifying 0/" + strconv.Itoa(total)
	if strings.Contains(content, stalled) {
		t.Errorf("%s: frame title still shows %q after the whole start-up command tree was drained — the availability sweep never dispatched a probe:\n%s", ordering, stalled, content)
	}
	if strings.Contains(content, "verifying") {
		t.Errorf("%s: frame title still carries a verifying counter after the whole start-up command tree was drained — all %d registered types were expected to have answered:\n%s", ordering, total, content)
	}
}

// TestSweepStarts_SeedBeforeConnect pins the losing race: the disk-cache seed
// lands while the session still has no AWS transport, so the sweep is latched
// rather than dispatched, and the connect that follows is the leg responsible
// for firing the held-back first probe batch. Those probe tasks must reach a
// tea.Cmd and run; if the connect leg's dispatch drops them, no probe ever
// fires, the checked counter never moves, and the title sits at
// "[verifying 0/N]" for the rest of the session.
func TestSweepStarts_SeedBeforeConnect(t *testing.T) {
	m, connect := newC9Model(t)

	m, _ = rootApplyMsg(m, messages.AvailabilityCacheLoaded{Entries: map[string]int{"s3": 7}})

	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, connect)
	m = runCmdTree(t, m, cmd)

	c9AssertSweepFinished(t, m, "seed before connect")
}

// TestSweepStarts_ConnectBeforeSeed pins the winning race as the control: the
// connect settles first, so the seed handler itself finds a live transport and
// fires the first probe batch on its own path. This ordering works today and
// must keep working — it is what makes the other ordering's failure a
// difference between the two legs rather than a broken harness.
func TestSweepStarts_ConnectBeforeSeed(t *testing.T) {
	m, connect := newC9Model(t)

	var cmd tea.Cmd
	m, cmd = rootApplyMsg(m, connect)
	m = runCmdTree(t, m, cmd)

	m, cmd = rootApplyMsg(m, messages.AvailabilityCacheLoaded{Entries: map[string]int{"s3": 7}})
	m = runCmdTree(t, m, cmd)

	c9AssertSweepFinished(t, m, "connect before seed")
}
