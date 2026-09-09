// SPDX-License-Identifier: GPL-3.0-or-later

// error_history_one_boundary_test.go — whether a failure reaches the "!" log
// must not depend on which host is rendering. The terminal adapter re-emits
// every error flash through Core.HandleFlash, which used to be where the
// history entry was made; the headless/web host applies the same FlashIntent
// straight to the controller and got no entry at all. Same failure, same
// event, two different logs.
//
// One rule now: the controller records the entry for every error flash it
// applies. These pins drive the same probe failure through both hosts and
// read the surface each one shows the operator.
package unit

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// boundaryProbeFailure is the hard probe failure both hosts are driven with:
// no rows came back, so softFailure is false and the runtime raises a banner
// rather than a log-only entry. Gen 1 is the seeded AvailabilityGen (a zero
// stamp is always stale for this event).
func boundaryProbeFailure(shortName string) messages.AvailabilityChecked {
	return messages.AvailabilityChecked{
		ResourceType: shortName,
		Err:          context.DeadlineExceeded,
		Gen:          1,
	}
}

// boundaryWantLine is the sentence failureLine gives that failure. Both hosts
// must show this exact line, exactly once.
func boundaryWantLine(shortName string) string {
	return "availability " + shortName + ": timeout"
}

// TestErrorFlash_HeadlessHostRecordsTheEntry drives the probe failure through
// the headless/web host (Controller.Handle, the entry point the web server
// uses) and reads the error log the "!" action opens.
func TestErrorFlash_HeadlessHostRecordsTheEntry(t *testing.T) {
	target := resource.AllShortNames()[0]
	ctrl := newTestController(t)
	ctrl.SetUIMode("web")

	ctrl.Handle(boundaryProbeFailure(target))

	want := boundaryWantLine(target)
	lines := ctrl.ErrorHistoryLines()
	if n := strings.Count(strings.Join(lines, "\n"), want); n != 1 {
		t.Errorf("headless host: want exactly 1 error-history entry containing %q, got %d in %v", want, n, lines)
	}

	// The surface the operator actually reads on the web host.
	vs, _ := ctrl.Apply(app.Action{Kind: app.ActionOpenErrorLog})
	if vs.Body.Text == nil {
		t.Fatalf("headless host: the error-log action opened no text body — the log is empty, so it flashed %q instead", "No errors this session")
	}
	if n := strings.Count(strings.Join(vs.Body.Text.Lines, "\n"), want); n != 1 {
		t.Errorf("web error-log surface: want exactly 1 line containing %q, got %d in %v", want, n, vs.Body.Text.Lines)
	}
}

// TestErrorFlash_TerminalHostRecordsTheSameOneEntry drives the identical
// failure through the terminal host and reads the same log. The count is the
// point: the terminal path applies the flash to the controller AND re-emits it
// through Core.HandleFlash, so a rule that fires on both would log it twice.
func TestErrorFlash_TerminalHostRecordsTheSameOneEntry(t *testing.T) {
	target := resource.AllShortNames()[0]
	m := newBlessedModel(t, "profile-a", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 100, Height: 40})

	m, cmd := rootApplyMsg(m, boundaryProbeFailure(target))
	m, _ = deliverProbeWindow(t, m, cmd)

	m = openErrorLog(m)
	plain := stripANSI(rootViewContent(m))

	want := boundaryWantLine(target)
	if n := strings.Count(plain, want); n != 1 {
		t.Errorf("terminal host: want exactly 1 occurrence of %q in the error log, got %d in:\n%s", want, n, plain)
	}
}
