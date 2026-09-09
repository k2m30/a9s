// SPDX-License-Identifier: GPL-3.0-or-later

// scan_health_errorlog_test.go — pins CONTRACT 2 of the observability
// slice: an availability probe that fails or times out must add one entry
// to the TUI's existing "!" error log (core/runtime/scan_status.go:29 has
// promised per-type scan health to hosts since #462; nothing has ever
// surfaced it there).
//
// INVERTED by the acceptance ruling on pass 1 of the "errors" task (spec row
// 3): the entry was pinned in the "probe <type>: <outcome>: <detail>" shape,
// whose <outcome> and <detail> are the internal ProbeOutcome and class names
// from core/runtime/scan_status.go. That put "probe ec2: failed: transport"
// on the banner beside "availability ec2: <cause>" in the log for the same
// event. The shape is now the one sentence every failed call gets. Do not
// restore the internal names — what the contract is about is that exactly one
// entry appears per type per sweep, which the assertions below still pin.
package unit

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// failingAvailFetcher deterministically fails with context.DeadlineExceeded.
// classifyProbeErr (core/runtime/scan_status.go) checks errors.Is against
// this exact sentinel before falling through to ClassifyAWSError, so this is
// the one error value guaranteed to classify as "timeout" without actually
// waiting out ProbeResourceAvailability's real 10s budget.
func failingAvailFetcher(_ context.Context, _ any, _ string) (resource.FetchResult, error) {
	return resource.FetchResult{}, context.DeadlineExceeded
}

// registerFullAvailabilitySweepFakes overrides EVERY resource type's
// availability fetcher, then re-overrides target with targetFetcher. A
// bare 4-wide registerProbeWindow is not enough here: delivering a
// completed probe's result back into the model (which this file's
// deliverProbeWindow does, unlike app_rotation_task_ctx_test.go's
// runBatchConcurrently) makes handleAvailabilityChecked re-emit its
// FlashIntent as a messages.Flash follow-up cmd bundled in the SAME
// tea.Batch as the "fire the next queued probe" cmd — the two are
// indistinguishable from the outside without executing them, and the flash
// re-emit is the ONLY path that reaches the error-log surface this file
// reads (see runtime_adapter.go's applyIntents: FlashIntent is re-emitted
// as messages.Flash specifically so it "routes through HandleFlash and
// picks up ... [the] history entry"). Executing every leaf therefore means
// executing whatever the next-probe cmd is too, which — with only the
// 4-wide window registered — would eventually reach a type with no
// override and invoke a real production fetcher against the zero-value
// *awsclient.ServiceClients this harness supplies. Stubbing every type
// removes that risk; empirically verified (see this file's RED/GREEN
// report) not to cascade into any Wave-2 enrichment call within the single
// target-completion drive this file performs.
func registerFullAvailabilitySweepFakes(t *testing.T, target string, targetFetcher resource.AvailabilityFetcher) {
	t.Helper()
	for _, name := range resource.AllShortNames() {
		resource.SetAvailabilityFetcherForTest(name, stubAvailFetcher)
		t.Cleanup(func(n string) func() { return func() { resource.CleanupAvailabilityFetcherForTest(n) } }(name))
	}
	resource.SetAvailabilityFetcherForTest(target, targetFetcher)
}

// deliverProbeWindow executes every leaf tea.Cmd in cmd's tea.BatchMsg tree,
// delivers each leaf's message back into m via rootApplyMsg, and recurses
// into whatever follow-up cmd that delivery itself returns — fully draining
// the sweep — unlike app_rotation_task_ctx_test.go's runBatchConcurrently,
// which deliberately discards leaf results (it only needs a probe to have
// STARTED, not completed-and-applied). Safe only because every resource
// type has a registered fetcher (registerFullAvailabilitySweepFakes), so no
// leaf ever reaches a real production fetcher.
func deliverProbeWindow(t *testing.T, m tui.Model, cmd tea.Cmd) (tui.Model, []tea.Msg) {
	t.Helper()
	var leaves []tea.Msg
	var walk func(c tea.Cmd)
	walk = func(c tea.Cmd) {
		if c == nil {
			return
		}
		msg := c()
		if msg == nil {
			return
		}
		if batch, ok := msg.(tea.BatchMsg); ok {
			for _, sub := range batch {
				walk(sub)
			}
			return
		}
		leaves = append(leaves, msg)
		var next tea.Cmd
		m, next = rootApplyMsg(m, msg)
		walk(next)
	}
	walk(cmd)
	return m, leaves
}

// openErrorLog presses "!" and executes any resulting deferred viewer-push
// cmd — the exact two-step sequence
// TestErrorHistoryAccumulation_ErrorFlashesAddToHistory
// (qa_error_log_test.go) uses to observe the error-log surface.
func openErrorLog(m tui.Model) tui.Model {
	m, cmd := rootApplyMsg(m, tea.KeyPressMsg{Code: '!'})
	if cmd != nil {
		if msg := cmd(); msg != nil {
			m, _ = rootApplyMsg(m, msg)
		}
	}
	return m
}

func newProbeWindowModel(t *testing.T) tui.Model {
	t.Helper()
	m := newBlessedModel(t, "profile-a", "us-east-1", tui.WithNoCache(true))
	m, _ = rootApplyMsg(m, tea.WindowSizeMsg{Width: 80, Height: 40})
	return m
}

// -----------------------------------------------------------------------
// (a) A failed/timed-out probe adds an "availability <type>: <cause>" entry
// to the error log.
// -----------------------------------------------------------------------

func TestScanHealthErrorLog_FailedProbe_AddsEntry(t *testing.T) {
	target := resource.AllShortNames()[0]
	registerFullAvailabilitySweepFakes(t, target, failingAvailFetcher)

	m := newProbeWindowModel(t)
	// Gen:1 — ConnectGen seeds at 1 (session.New()); this model is never rotated.
	m, batchCmd := dispatchProbeWindow(t, m, "us-east-1", 1)
	m, _ = deliverProbeWindow(t, m, batchCmd)

	m = openErrorLog(m)
	plain := stripANSI(rootViewContent(m))

	want := "availability " + target + ": timeout"
	if !strings.Contains(plain, want) {
		t.Errorf("error log missing scan-health entry: want substring %q, got:\n%s", want, plain)
	}
}

// -----------------------------------------------------------------------
// (b) A successful probe adds no scan-health entry.
// -----------------------------------------------------------------------

func TestScanHealthErrorLog_SuccessfulProbe_AddsNoEntry(t *testing.T) {
	target := resource.AllShortNames()[0]
	registerFullAvailabilitySweepFakes(t, target, stubAvailFetcher)

	m := newProbeWindowModel(t)
	// Gen:1 — ConnectGen seeds at 1 (session.New()); this model is never rotated.
	m, batchCmd := dispatchProbeWindow(t, m, "us-east-1", 1)
	m, _ = deliverProbeWindow(t, m, batchCmd)

	m = openErrorLog(m)
	plain := stripANSI(rootViewContent(m))

	if strings.Contains(plain, "availability "+target+":") {
		t.Errorf("successful probe must add no scan-health entry, but found one in:\n%s", plain)
	}
}

// -----------------------------------------------------------------------
// (c) The SAME type failing twice in the SAME sweep adds only one entry —
// not spam.
// -----------------------------------------------------------------------

func TestScanHealthErrorLog_DuplicateDeliveryInSameSweep_DoesNotDuplicateEntry(t *testing.T) {
	target := resource.AllShortNames()[0]
	registerFullAvailabilitySweepFakes(t, target, failingAvailFetcher)

	m := newProbeWindowModel(t)
	// Gen:1 — ConnectGen seeds at 1 (session.New()); this model is never rotated.
	m, batchCmd := dispatchProbeWindow(t, m, "us-east-1", 1)
	m, leaves := deliverProbeWindow(t, m, batchCmd)

	var targetMsg messages.AvailabilityChecked
	found := false
	for _, l := range leaves {
		if ac, ok := l.(messages.AvailabilityChecked); ok && ac.ResourceType == target {
			targetMsg = ac
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("target type %s never produced an AvailabilityChecked message during the drive", target)
	}

	// Re-deliver the identical failure a second time within the same sweep,
	// chasing its own follow-up cmd exactly like the first delivery did —
	// the flash re-emit that actually reaches the error-log surface
	// (runtime_adapter.go's applyIntents) is itself a follow-up cmd, not a
	// direct effect of rootApplyMsg's first return value alone.
	m, redeliverCmd := rootApplyMsg(m, targetMsg)
	m, _ = deliverProbeWindow(t, m, redeliverCmd)

	m = openErrorLog(m)
	plain := stripANSI(rootViewContent(m))

	want := "availability " + target + ": timeout"
	if n := strings.Count(plain, want); n != 1 {
		t.Errorf("want exactly 1 occurrence of %q after a same-sweep duplicate delivery, got %d in:\n%s", want, n, plain)
	}
}
