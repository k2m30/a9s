// qa_sweep_once_per_session_test.go — pins the "availability sweep runs once
// per session per profile--region pair" contract (fixed defect, landed
// 2026-07-14: `:profile` switching used to re-run the FULL availability
// sweep, all types plus Wave-2 enrichment, on EVERY switch instead of once
// per pair).
//
// Pinned production contract under test:
//
//	Session gains SweptPairs map[string]bool keyed profile+"--"+region, plus
//	MarkPairSwept()/PairSwept() bool operating on the CURRENT pair under the
//	existing pairMu. Session-lifetime: Rotate() does NOT clear it; New()
//	initializes it.
//
//	handleAvailabilityCacheLoaded (core/runtime/handlers_availability.go):
//	AMENDED by cachegen row 10 — a pair re-entry sweeps again (C1: everything
//	cached is re-verified on sight), and the sweep start clears the pair's
//	memo. The memo now only stops a duplicate probe result from re-running
//	one sweep's completion. Disk-cache seeding of rows/counts/issue badges is
//	unaffected either way.
//
//	handleAvailabilityChecked: on sweep completion (AvailChecked reaches
//	AvailTotal for a non-empty sweep) it must call MarkPairSwept().
//
//	internal/tui's Model.handleRefresh (Ctrl+R on the main menu,
//	runtime_adapter_navigate.go:591) is the one existing manual full-menu
//	refresh gesture (the headless Controller.handleActionRefresh in
//	core/app/actions_list.go has no menu-level branch at all — it is a
//	no-op on the menu screen). It must clear the current pair's memo before
//	re-triggering the availability-cache reload.
package unit_test

import (
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// countProbeTasks returns how many TaskKindProbeAvailability entries appear
// in tasks — the observable signal that a batch of Wave-1 availability
// probes was actually dispatched.
func countProbeTasks(tasks []runtime.TaskRequest) int {
	n := 0
	for _, tr := range tasks {
		if tr.Key.Kind == runtime.TaskKindProbeAvailability {
			n++
		}
	}
	return n
}

// fireAvailabilitySweep drives the real two-message sequence a live
// cache-seeded sweep needs to actually dispatch probes rather than merely
// latch AvailSweepPending. Since commit 89f0f69d ("availability sweep waits
// for client readiness"), handleAvailabilityCacheLoaded dispatches
// TaskKindProbeAvailability tasks ONLY when c.session.Clients != nil; a
// controller built without pre-supplied clients (as newRowStoreControllerPin
// builds — pinned GREEN by TestProbeResourceAvailability_NilClients, which
// this helper must not relax) instead latches AvailSweepPending and dispatches
// nothing until a follow-up messages.ClientsReady drains it
// (fireNextAvailabilityProbes(4) inside handleClientsReadySuccess runs
// unconditionally off the latch, independent of whether real *ServiceClients
// ever land — see TestWebBoot_Refreshing_TrueDuringCacheSeededSweep_
// FalseOnComplete, tests/unit/app_web_live_cold_boot_test.go:206, for the
// same two-call sequence against the same nil-clients construction). Gen must
// match the CURRENT ConnectGen (bumped by every Rotate()) or
// Core.HandleClientsReady's own equality guard drops the event silently.
func fireAvailabilitySweep(c *app.Controller, s *session.Session, entries map[string]int, truncated map[string]bool) []runtime.TaskRequest {
	_, tasks := c.Handle(messages.AvailabilityCacheLoaded{
		Entries:   entries,
		Truncated: truncated,
	})
	_, readyTasks := c.Handle(messages.ClientsReady{Gen: s.ConnectGen})
	return append(tasks, readyTasks...)
}

// drainAvailabilitySweep drives every TaskKindProbeAvailability task in
// initial (and every follow-on task handleAvailabilityChecked fires as the
// queue empties) through Controller.Handle via synthesized
// messages.AvailabilityChecked results, until the sweep is fully drained.
// Mirrors what the real executor does one probe result at a time, just
// serially instead of concurrently.
func drainAvailabilitySweep(c *app.Controller, s *session.Session, initial []runtime.TaskRequest) {
	pending := make([]runtime.TaskRequest, 0, len(initial))
	for _, tr := range initial {
		if tr.Key.Kind == runtime.TaskKindProbeAvailability {
			pending = append(pending, tr)
		}
	}
	for len(pending) > 0 {
		tr := pending[0]
		pending = pending[1:]
		_, tasks := c.Handle(messages.AvailabilityChecked{
			ResourceType: tr.Key.Scope,
			Gen:          s.AvailabilityGen,
		})
		for _, nt := range tasks {
			if nt.Key.Kind == runtime.TaskKindProbeAvailability {
				pending = append(pending, nt)
			}
		}
	}
}

// switchPair simulates the house profile/region-switch flow (mirrors
// Core.HandleProfileSelected in core/runtime/handlers.go): rotate the
// session, set the new pair, then clear the root menu's availability state
// the way the real switch handler's MenuClearAvailabilityIntent does — so a
// revisit test can't accidentally read stale Controller-level menu state
// left over from a prior pair instead of a fresh PatchMenuAvailability the
// revisit itself must emit.
func switchPair(c *app.Controller, s *session.Session, profile, region string) {
	s.Rotate()
	s.SetProfileRegion(profile, region)
	c.ApplyIntents([]runtime.UIIntent{runtime.MenuClearAvailabilityIntent{}})
}

// -----------------------------------------------------------------------
// 1 — First visit: fresh session pair fires probe tasks (existing
// first-visit behavior must be preserved by the fix).
// -----------------------------------------------------------------------

func TestSweepOnce_FreshPair_FirstVisit_FiresProbes(t *testing.T) {
	s, _, c := newRowStoreControllerPin(t)

	tasks := fireAvailabilitySweep(c, s, map[string]int{"ec2": 3}, map[string]bool{"ec2": false})

	if n := countProbeTasks(tasks); n == 0 {
		t.Fatal("first visit: handleAvailabilityCacheLoaded fired zero TaskKindProbeAvailability tasks, want > 0 (first-visit sweep must still fire)")
	}
	if s.AvailTotal == 0 {
		t.Errorf("AvailTotal = 0 after first-visit cache-loaded, want len(resource.AllShortNames())")
	}
	if s.PairSwept() {
		t.Error("PairSwept() = true immediately after the first probe batch fired, want false — the sweep has not completed yet")
	}
}

// -----------------------------------------------------------------------
// 1b — Mid-sweep double delivery: a second AvailabilityCacheLoaded landing
// before the first sweep completes must not rebuild the queue or re-fire
// probes (handlers_availability.go's handleAvailabilityCacheLoaded: both the
// pre-connect disk-cache seed and handleClientsReadySuccess's own
// TaskKindLoadAvailCache dispatch land for the same pair before PairSwept
// goes true, since messages.AvailabilityCacheLoaded carries no Gen).
// -----------------------------------------------------------------------

func TestSweepOnce_MidSweepDuplicateCacheLoaded_DoesNotRebuildQueueOrReprobe(t *testing.T) {
	s, _, c := newRowStoreControllerPin(t)

	tasks := fireAvailabilitySweep(c, s, map[string]int{"ec2": 1}, map[string]bool{"ec2": false})
	if s.PairSwept() {
		t.Fatal("precondition: PairSwept() must be false before the sweep completes")
	}
	if s.AvailTotal == 0 {
		t.Fatal("precondition: AvailTotal must be non-zero after the first-visit cache load")
	}
	if n := countProbeTasks(tasks); n == 0 {
		t.Fatal("precondition: the first cache load must have fired at least one probe task")
	}
	queueLenBefore := len(s.AvailQueue)
	checkedBefore := s.AvailChecked
	totalBefore := s.AvailTotal

	// A second AvailabilityCacheLoaded lands mid-sweep, for the SAME pair,
	// before AvailChecked ever reaches AvailTotal.
	_, secondTasks := c.Handle(messages.AvailabilityCacheLoaded{
		Entries:   map[string]int{"ec2": 1},
		Truncated: map[string]bool{"ec2": false},
	})

	if n := countProbeTasks(secondTasks); n != 0 {
		t.Errorf("second, mid-sweep AvailabilityCacheLoaded fired %d probe tasks, want 0 — a sweep already under way must not re-dispatch probes", n)
	}
	if len(s.AvailQueue) != queueLenBefore {
		t.Errorf("AvailQueue len = %d after the mid-sweep second cache load, want unchanged %d — the queue must not be rebuilt", len(s.AvailQueue), queueLenBefore)
	}
	if s.AvailChecked != checkedBefore {
		t.Errorf("AvailChecked = %d after the mid-sweep second cache load, want unchanged %d", s.AvailChecked, checkedBefore)
	}
	if s.AvailTotal != totalBefore {
		t.Errorf("AvailTotal = %d after the mid-sweep second cache load, want unchanged %d — a rebuild would reset it to len(AllShortNames())", s.AvailTotal, totalBefore)
	}
}

// -----------------------------------------------------------------------
// 1c — Duplicate post-completion delivery: once a sweep has completed
// (PairSwept() true), a further AvailabilityChecked for an already-checked
// type must not re-run completion — no second TaskKindSaveCache dispatch, no
// second startEnrichment (would rebuild EnrichQueue and bump EnrichmentGen,
// discarding whatever Wave-2 probes the first completion already
// dispatched).
// -----------------------------------------------------------------------

func TestSweepOnce_DuplicateAvailabilityCheckedAfterCompletion_DoesNotRerunCompletion(t *testing.T) {
	s, _, c := newRowStoreControllerPin(t)

	tasks := fireAvailabilitySweep(c, s, map[string]int{"ec2": 1}, map[string]bool{"ec2": false})
	drainAvailabilitySweep(c, s, tasks)

	if !s.PairSwept() {
		t.Fatal("precondition: sweep must be fully drained and PairSwept() true before testing the duplicate-delivery guard")
	}
	enrichGenBefore := s.EnrichmentGen

	// A duplicate/redelivered AvailabilityChecked reaches
	// handleAvailabilityChecked again after PairSwept() is already true.
	_, dupTasks := c.Handle(messages.AvailabilityChecked{
		ResourceType: "ec2",
		Gen:          s.AvailabilityGen,
		HasResources: true,
		Count:        1,
	})

	for _, tr := range dupTasks {
		if tr.Key.Kind == runtime.TaskKindSaveCache {
			t.Error("duplicate AvailabilityChecked after PairSwept() re-dispatched TaskKindSaveCache — completion re-ran")
		}
		if tr.Key.Kind == runtime.TaskKindProbeEnrich {
			t.Error("duplicate AvailabilityChecked after PairSwept() re-dispatched TaskKindProbeEnrich — startEnrichment re-ran")
		}
	}
	if s.EnrichmentGen != enrichGenBefore {
		t.Errorf("EnrichmentGen = %d after a duplicate AvailabilityChecked post-completion, want unchanged %d — startEnrichment must not re-run", s.EnrichmentGen, enrichGenBefore)
	}
	if !s.PairSwept() {
		t.Error("PairSwept() = false after a duplicate AvailabilityChecked post-completion, want still true")
	}
}

// -----------------------------------------------------------------------
// 2 — Completion memo: driving AvailChecked to AvailTotal marks the pair
// swept.
// -----------------------------------------------------------------------

func TestSweepOnce_CompletionMarksPairSwept(t *testing.T) {
	s, _, c := newRowStoreControllerPin(t)

	tasks := fireAvailabilitySweep(c, s, map[string]int{"ec2": 1}, map[string]bool{"ec2": false})
	if s.PairSwept() {
		t.Fatal("precondition: PairSwept() must be false before the sweep completes")
	}

	drainAvailabilitySweep(c, s, tasks)

	if s.AvailChecked != s.AvailTotal {
		t.Fatalf("precondition: sweep did not fully drain — AvailChecked=%d AvailTotal=%d", s.AvailChecked, s.AvailTotal)
	}
	if !s.PairSwept() {
		t.Error("PairSwept() = false after AvailChecked reached AvailTotal, want true — sweep completion must mark the pair swept")
	}
}

// -----------------------------------------------------------------------
// 3 — INVERTED (cachegen row 10, C1 verify-on-sight): A (fully swept) -> B
// -> A. The revisit must sweep AGAIN. The old expectation — zero probes and
// progress reported as done — was the defect: the account moves on while the
// operator is looking at the other pair, so disk values looked verified when
// nothing had verified them this visit. The swept memo survives as a
// completion latch only (it stops a duplicate probe result from re-running
// one sweep's completion); it is no longer permission to skip. Do not
// restore the skip.
// -----------------------------------------------------------------------

func TestSweepOnce_RevisitSweptPair_SweepsAgain_UnsweptPairAlsoProbes(t *testing.T) {
	s, _, c := newRowStoreControllerPin(t)

	tasksA := fireAvailabilitySweep(c, s, map[string]int{"ec2": 1}, map[string]bool{"ec2": false})
	drainAvailabilitySweep(c, s, tasksA)
	if !s.PairSwept() {
		t.Fatal("precondition: pair A must be fully swept before testing the revisit skip")
	}

	// A -> B (never visited before).
	switchPair(c, s, "sweeponce-pair-b", "eu-west-1")
	if s.PairSwept() {
		t.Fatal("precondition: pair B must report unswept on its first visit")
	}
	tasksB := fireAvailabilitySweep(c, s, map[string]int{"ec2": 1}, map[string]bool{"ec2": false})
	if n := countProbeTasks(tasksB); n == 0 {
		t.Error("pair B (never swept): handleAvailabilityCacheLoaded fired zero probe tasks, want > 0")
	}

	// B -> A (revisit an already fully-swept pair).
	switchPair(c, s, "demo", "us-east-1")
	if !s.PairSwept() {
		t.Fatal("pair A's completed-sweep memo did not survive the A->B->A round trip")
	}
	// The memo is a completion latch, not a skip: the revisit's own sweep
	// start clears it and takes ownership of its own completion.
	tasksARevisit := fireAvailabilitySweep(c, s, map[string]int{"ec2": 1}, map[string]bool{"ec2": false})
	if n := countProbeTasks(tasksARevisit); n == 0 {
		t.Error("pair A revisit: handleAvailabilityCacheLoaded fired zero probe tasks, want > 0 — C1 re-verifies on every pair entry")
	}
	if s.AvailChecked != 0 {
		t.Errorf("pair A revisit: AvailChecked=%d, want 0 — the revisit starts a fresh sweep, it does not report the old one as done", s.AvailChecked)
	}
	if got := c.GetMenuAvailability()["ec2"]; got != 1 {
		t.Errorf("pair A revisit: GetMenuAvailability()[ec2] = %d, want 1 — disk-cache seeding of counts must still run on the skip path", got)
	}
}

// -----------------------------------------------------------------------
// 4 — Interrupted sweep: partial progress, then switch away and back must
// NOT skip — the memo is only set on genuine completion.
// -----------------------------------------------------------------------

func TestSweepOnce_InterruptedSweep_DoesNotMarkSwept_ProbesAgainOnRevisit(t *testing.T) {
	s, _, c := newRowStoreControllerPin(t)

	tasks := fireAvailabilitySweep(c, s, map[string]int{"ec2": 1}, map[string]bool{"ec2": false})
	firstProbeIdx := -1
	for i, tr := range tasks {
		if tr.Key.Kind == runtime.TaskKindProbeAvailability {
			firstProbeIdx = i
			break
		}
	}
	if firstProbeIdx < 0 {
		t.Fatal("precondition: no TaskKindProbeAvailability task in the first-visit batch")
	}

	// Complete exactly one probe result — genuinely interrupted, not drained.
	c.Handle(messages.AvailabilityChecked{
		ResourceType: tasks[firstProbeIdx].Key.Scope,
		Gen:          s.AvailabilityGen,
	})
	if s.AvailChecked >= s.AvailTotal {
		t.Fatalf("precondition: sweep must still be in flight — AvailChecked=%d AvailTotal=%d", s.AvailChecked, s.AvailTotal)
	}
	if s.PairSwept() {
		t.Fatal("precondition: PairSwept() must be false while the sweep is still in flight")
	}

	// Switch away and back before the sweep ever finishes.
	switchPair(c, s, "sweeponce-pair-interrupted", "eu-west-1")
	switchPair(c, s, "demo", "us-east-1")

	if s.PairSwept() {
		t.Fatal("an interrupted (never-completed) sweep must not leave the pair marked swept after a round trip")
	}
	revisitTasks := fireAvailabilitySweep(c, s, map[string]int{"ec2": 1}, map[string]bool{"ec2": false})
	if n := countProbeTasks(revisitTasks); n == 0 {
		t.Error("revisiting a pair whose sweep was interrupted (never completed) fired zero probe tasks, want > 0")
	}
}

// -----------------------------------------------------------------------
// 5 — Rotate survival: Session.Rotate() must not clear SweptPairs (it is a
// session-lifetime memo, unlike every other Rotate-cleared queue/counter).
// -----------------------------------------------------------------------

func TestSweepOnce_RotatePreservesSweptPairs(t *testing.T) {
	s := session.New()
	s.SetProfileRegion("demo", "us-east-1")

	if s.PairSwept() {
		t.Fatal("precondition: a fresh session must report PairSwept() == false before MarkPairSwept is ever called")
	}
	s.MarkPairSwept()
	if !s.PairSwept() {
		t.Fatal("precondition: MarkPairSwept() did not take effect")
	}
	if !s.SweptPairs["demo--us-east-1"] {
		t.Errorf(`SweptPairs["demo--us-east-1"] = false after MarkPairSwept(), want true — the pinned key format is profile+"--"+region`)
	}

	s.Rotate()

	if !s.PairSwept() {
		t.Error("PairSwept() = false for the SAME pair after Rotate(), want true — Rotate() must not clear SweptPairs")
	}

	// A different, never-swept pair must still report false post-Rotate —
	// the memo is per-pair, not a single session-wide flag.
	s.SetProfileRegion("never-swept-profile", "ap-southeast-1")
	if s.PairSwept() {
		t.Error("PairSwept() = true for a pair that was never marked swept, want false — the memo must be keyed per profile--region pair")
	}
}

// -----------------------------------------------------------------------
// 6 — Manual full-menu refresh (Ctrl+R on the main menu, the ONE existing
// gesture found at internal/tui/runtime_adapter_navigate.go:591
// Model.handleRefresh's rsKindMenu branch) must clear the current pair's
// memo before re-sweeping. core/app's headless
// Controller.handleActionRefresh (actions_list.go) has no menu-level branch
// at all, so this contract is TUI-only.
// -----------------------------------------------------------------------

// newRootSizedModel builds an 80x40 tui.Model the same disciplined way
// tests/unit/tui_root_test.go's helper of the same name does (t.TempDir()
// config isolation + t.Cleanup(m.CloseController) so the availability-save
// writer goroutine this test's Ctrl+R press queues can never outlive the
// TempDir teardown — see TestControllerConstructionDisciplineGate,
// qa_controller_construction_discipline_test.go). Package unit_test cannot
// call the package-unit original directly (test packages in the same
// directory do not share unexported — or even same-named — declarations
// across the internal/external split), so this is a same-name, same-shape
// re-implementation, not a duplicate declaration.
func newRootSizedModel(t *testing.T) tui.Model {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	m := tui.New("sweeponce-manual-refresh", "us-east-1")
	t.Cleanup(m.CloseController)
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 40})
	return newM.(tui.Model)
}

func TestSweepOnce_ManualRefresh_ClearsMemo_AndReprobes(t *testing.T) {
	m := newRootSizedModel(t)

	s := m.Core().Session()
	s.MarkPairSwept()
	if !s.PairSwept() {
		t.Fatal("precondition: MarkPairSwept() did not take effect")
	}

	newM, cmd := m.Update(tea.KeyPressMsg{Code: 'r', Mod: tea.ModCtrl})
	m = newM.(tui.Model)

	if m.Core().Session().PairSwept() {
		t.Error("Ctrl+R manual refresh on the main menu did not clear the completed-sweep memo — a full refresh must force a fresh sweep even for an already-swept pair")
	}
	if cmd == nil {
		t.Fatal("Ctrl+R on the main menu produced no cmd — expected the availability-cache reload to fire")
	}

	msg := cmd()
	if msg == nil {
		t.Fatal("Ctrl+R's cmd produced no message")
	}
	newM, _ = m.Update(msg)
	m = newM.(tui.Model)

	if len(m.Core().Session().AvailQueue) == 0 {
		t.Error("after Ctrl+R's availability-cache reload landed, AvailQueue is empty — clearing the memo must cause a fresh sweep to be queued, not the already-swept skip path")
	}
}
