// runtime_enrich_dispatch_window_test.go — RED pins for the upcoming
// bounded-window Wave-2 enrichment dispatch.
//
// Current behavior (RED today): startEnrichment (core/runtime/handlers_availability.go
// ~:429-465) drains the ENTIRE session.EnrichQueue into one task batch — with
// ~49 registered Wave2 enrichers (core/aws/catalog_*.go) that is ~49
// concurrent AWS probe chains when the adapter executes the batch. The
// pre-existing queue-refill branch in handleEnrichmentChecked
// (core/runtime/handlers_availability.go ~:653-657, "if queue has more items,
// fire next probe") is DEAD because the queue is always empty by the time any
// EnrichmentChecked result arrives.
//
// Upcoming behavior pinned here: startEnrichment dispatches an initial window
// of AT MOST 4 TaskKindProbeEnrich tasks (mirroring Wave-1's
// runtime.MaxConcurrentProbes=4, core/runtime/related.go:12-13), leaving the
// remainder queued in session.EnrichQueue; each EnrichmentChecked completion
// pops exactly one next queued type (the existing refill branch goes live);
// the full queue still drains to completion over successive completions with
// no type lost and no type dispatched twice.
//
// The window size (4) is pinned as a literal here rather than referenced via
// runtime.MaxConcurrentProbes: that constant is documented as scoped to the
// related-checker fan-out (core/runtime/related.go), and the dispatch's
// explicit contract for this Wave-2 window is the number 4, not "whatever
// MaxConcurrentProbes happens to equal" — the two are allowed to diverge
// without silently breaking these pins.
//
// Harness: Core built via runtime.New(session.New(), catalog.All()) (the
// package-level style already used by runtime_list_open_enrich_dispatch_test.go
// and enrich_queue_test.go), driven through the exported
// Core.HandleEvent(messages.AvailabilityPrefetched{...}) /
// Core.HandleEvent(messages.EnrichmentChecked{...}) entry points so every
// assertion reads the []TaskRequest returned synchronously — no tea.Cmd tree
// is executed, so these pins observe the dispatch-time task count directly
// rather than a drained cascade.
package unit

import (
	"slices"
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// enrichDispatchWindow is the pinned contract value for this file: the
// maximum number of TaskKindProbeEnrich tasks startEnrichment may dispatch in
// its initial batch, regardless of queue size.
const enrichDispatchWindow = 4

// allWave2ShortNames returns every catalog resource type ShortName with a
// registered Wave2 issue enricher (HasIssueEnricher true), in catalog.All()
// order. Fails the test immediately if empty — every pin in this file
// requires a real, non-trivial dispatch queue to be meaningful.
func allWave2ShortNames(t *testing.T, core *runtime.Core) []string {
	t.Helper()
	var out []string
	for _, td := range catalog.All() {
		if td.Wave2 != nil && core.HasIssueEnricher(td.ShortName) {
			out = append(out, td.ShortName)
		}
	}
	if len(out) == 0 {
		t.Fatal("no catalog entry with a registered Wave2 issue enricher found — cannot exercise the enrichment dispatch window pins")
	}
	return out
}

// seedProbeEnrichResources builds an AvailabilityPrefetched.Resources map
// with one retained resource per name, so BuildEnrichQueue's RowStore
// membership check (tr.Gen != 0) accepts every one of them.
func seedProbeEnrichResources(names []string) map[string][]resource.Resource {
	out := make(map[string][]resource.Resource, len(names))
	for _, n := range names {
		out[n] = []resource.Resource{{ID: n + "-r1"}}
	}
	return out
}

// countProbeEnrichTasks returns the number of TaskKindProbeEnrich entries in
// tasks.
func countProbeEnrichTasks(tasks []runtime.TaskRequest) int {
	n := 0
	for _, tr := range tasks {
		if tr.Key.Kind == runtime.TaskKindProbeEnrich {
			n++
		}
	}
	return n
}

// probeEnrichScopes returns the Scope (resource ShortName) of every
// TaskKindProbeEnrich entry in tasks, in order.
func probeEnrichScopes(tasks []runtime.TaskRequest) []string {
	var out []string
	for _, tr := range tasks {
		if tr.Key.Kind == runtime.TaskKindProbeEnrich {
			out = append(out, tr.Key.Scope)
		}
	}
	return out
}

// TestStartEnrichment_SweepCompletion_DispatchesBoundedWindow is pin (a):
// a sweep completion with N>4 queued Wave2-capable types must dispatch
// exactly 4 TaskKindProbeEnrich tasks, leaving N-4 entries queued.
//
// RED today: startEnrichment drains the entire queue into the returned task
// slice — this asserts exactly 4 tasks and N-4 remaining queued, which fails
// against the current "dispatch everything" behavior for any real catalog
// (49 registered Wave2 enrichers as of this writing).
func TestStartEnrichment_SweepCompletion_DispatchesBoundedWindow(t *testing.T) {
	sess := session.New()
	core := runtime.New(sess, catalog.All())
	names := allWave2ShortNames(t, core)
	if len(names) <= enrichDispatchWindow {
		t.Fatalf("test requires more than %d registered Wave2 enrichers to exercise the bounded window, found %d", enrichDispatchWindow, len(names))
	}

	_, tasks := core.HandleEvent(messages.AvailabilityPrefetched{
		Entries:   map[string]int{},
		Resources: seedProbeEnrichResources(names),
		Gen:       sess.AvailabilityGen,
	})

	if got := countProbeEnrichTasks(tasks); got != enrichDispatchWindow {
		t.Fatalf("startEnrichment dispatched %d TaskKindProbeEnrich tasks for %d queued types, want exactly %d (bounded initial window); dispatched scopes: %v",
			got, len(names), enrichDispatchWindow, probeEnrichScopes(tasks))
	}

	wantRemaining := len(names) - enrichDispatchWindow
	if remaining := len(core.Session().EnrichQueue); remaining != wantRemaining {
		t.Fatalf("after the initial window dispatch, session.EnrichQueue has %d entries, want %d (N=%d minus window=%d): %v",
			remaining, wantRemaining, len(names), enrichDispatchWindow, core.Session().EnrichQueue)
	}
}

// TestHandleEnrichmentChecked_RefillsWindowByOne is pin (b): one
// EnrichmentChecked completion for a type dispatched in the initial window
// must dispatch exactly one additional TaskKindProbeEnrich task (the
// pre-existing refill branch going live) and shrink session.EnrichQueue by
// exactly one.
//
// RED today: because startEnrichment already drained the entire queue up
// front, session.EnrichQueue is empty by the time this EnrichmentChecked
// arrives, so handleEnrichmentChecked's "if queue has more items" branch
// never fires — zero additional tasks, not one.
func TestHandleEnrichmentChecked_RefillsWindowByOne(t *testing.T) {
	sess := session.New()
	core := runtime.New(sess, catalog.All())
	names := allWave2ShortNames(t, core)
	if len(names) <= enrichDispatchWindow {
		t.Fatalf("test requires more than %d registered Wave2 enrichers to exercise window refill, found %d", enrichDispatchWindow, len(names))
	}

	_, initialTasks := core.HandleEvent(messages.AvailabilityPrefetched{
		Entries:   map[string]int{},
		Resources: seedProbeEnrichResources(names),
		Gen:       sess.AvailabilityGen,
	})
	dispatched := probeEnrichScopes(initialTasks)
	if len(dispatched) != enrichDispatchWindow {
		t.Fatalf("setup: initial window dispatched %d TaskKindProbeEnrich tasks, want %d — cannot exercise refill without a full initial window", len(dispatched), enrichDispatchWindow)
	}
	queuedBefore := len(core.Session().EnrichQueue)

	_, followupTasks := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: dispatched[0],
		TypeGen:      0,
		Gen:          0,
	})

	if got := countProbeEnrichTasks(followupTasks); got != 1 {
		t.Fatalf("handleEnrichmentChecked completion for %q dispatched %d additional TaskKindProbeEnrich tasks, want exactly 1 (window refill); returned scopes: %v",
			dispatched[0], got, probeEnrichScopes(followupTasks))
	}
	if remaining := len(core.Session().EnrichQueue); remaining != queuedBefore-1 {
		t.Fatalf("after one completion, session.EnrichQueue has %d entries, want %d (was %d before completion)", remaining, queuedBefore-1, queuedBefore)
	}
}

// TestStartEnrichment_FullDrain_EveryQueuedTypeEnrichedExactlyOnce is pin
// (c): driving every dispatched EnrichmentChecked completion to exhaustion
// must enrich every Wave2-capable queued type exactly once — no type lost,
// no type dispatched twice, regardless of window size.
//
// This is a forward-looking regression guard: it may already be GREEN today
// (an unbounded initial dispatch also enriches every type exactly once,
// since the dead refill branch never re-adds anything), but it is the pin
// that would catch a windowed implementation introducing loss or duplication
// across refill cycles — a real risk the naive "pop one on every completion"
// refill branch does not by itself rule out (e.g. an off-by-one on the
// window boundary, or a refill firing for an already-dispatched type).
func TestStartEnrichment_FullDrain_EveryQueuedTypeEnrichedExactlyOnce(t *testing.T) {
	sess := session.New()
	core := runtime.New(sess, catalog.All())
	names := allWave2ShortNames(t, core)

	_, tasks := core.HandleEvent(messages.AvailabilityPrefetched{
		Entries:   map[string]int{},
		Resources: seedProbeEnrichResources(names),
		Gen:       sess.AvailabilityGen,
	})

	dispatchCount := map[string]int{}
	var pending []string
	for _, name := range probeEnrichScopes(tasks) {
		dispatchCount[name]++
		pending = append(pending, name)
	}

	// Bound the drain loop defensively: it must terminate within
	// len(names) completions if the contract holds (each completion
	// dispatches at most one follow-up). A stray duplicate/loss bug could
	// otherwise loop far longer than that, so cap iterations rather than
	// looping on len(pending) > 0 unconditionally.
	maxIterations := len(names) * 2
	iterations := 0
	for len(pending) > 0 && iterations < maxIterations {
		iterations++
		name := pending[0]
		pending = pending[1:]

		_, followup := core.HandleEvent(messages.EnrichmentChecked{
			ResourceType: name,
			TypeGen:      0,
			Gen:          0,
		})
		for _, next := range probeEnrichScopes(followup) {
			dispatchCount[next]++
			pending = append(pending, next)
		}
	}
	if iterations >= maxIterations {
		t.Fatalf("full drain did not terminate within %d completions for %d queued types — suspect a refill loop (dispatchCount so far: %v)", maxIterations, len(names), dispatchCount)
	}

	if len(dispatchCount) != len(names) {
		t.Errorf("full drain enriched %d distinct types, want %d (some queued type was lost)", len(dispatchCount), len(names))
	}
	for _, n := range names {
		if dispatchCount[n] != 1 {
			t.Errorf("type %q was enriched %d times during full drain, want exactly 1 (no loss, no dup)", n, dispatchCount[n])
		}
	}
}

// TestStartEnrichment_SmallQueue_AllDispatchedImmediately is pin (d): a
// queue with N<=4 (<= the window) must dispatch all N tasks immediately and
// leave nothing queued — the windowed implementation must not regress the
// small-queue case (e.g. no off-by-one under-dispatch).
//
// GREEN today (unbounded dispatch already sends all N immediately for a
// small queue) and must stay GREEN once the window is introduced.
func TestStartEnrichment_SmallQueue_AllDispatchedImmediately(t *testing.T) {
	sess := session.New()
	core := runtime.New(sess, catalog.All())
	all := allWave2ShortNames(t, core)
	if len(all) < 3 {
		t.Fatalf("test requires at least 3 registered Wave2 types, found %d", len(all))
	}
	names := all[:3]

	_, tasks := core.HandleEvent(messages.AvailabilityPrefetched{
		Entries:   map[string]int{},
		Resources: seedProbeEnrichResources(names),
		Gen:       sess.AvailabilityGen,
	})

	if got := countProbeEnrichTasks(tasks); got != len(names) {
		t.Fatalf("startEnrichment for %d queued types (<= window of %d) dispatched %d tasks, want all %d dispatched immediately; dispatched scopes: %v",
			len(names), enrichDispatchWindow, got, len(names), probeEnrichScopes(tasks))
	}
	if remaining := len(core.Session().EnrichQueue); remaining != 0 {
		t.Fatalf("startEnrichment for %d queued types (<= window) left %d entries queued, want 0 (all dispatched immediately): %v", len(names), remaining, core.Session().EnrichQueue)
	}
}

// TestHandleEnrichmentChecked_StaleTypeGenCompletion_StillRefillsQueueByOne
// pins defect #1 (external review, 2026-07-19): a completion for a windowed
// (in-flight) type whose per-type generation was bumped while its sweep
// probe was still outstanding — the rerun/refresh path, e.g. Ctrl+R on the
// open list or a second sweep re-queuing the same type — must still refill
// the queue by one. The stale result itself is correctly discarded, but the
// sweep's forward progress must not stall because of it.
//
// RED today: handleEnrichmentChecked's TypeGen staleness guard
// (core/runtime/handlers_availability.go ~:484) returns (nil, nil)
// immediately on a TypeGen mismatch, before ever reaching the refill branch
// (~:658-665). With more than enrichDispatchWindow queued types, a stale
// completion for any one of the four in-flight types drops its refill on
// the floor — zero additional tasks dispatched, not one — and every type
// still queued behind it never gets a turn.
func TestHandleEnrichmentChecked_StaleTypeGenCompletion_StillRefillsQueueByOne(t *testing.T) {
	sess := session.New()
	core := runtime.New(sess, catalog.All())
	names := allWave2ShortNames(t, core)
	if len(names) <= enrichDispatchWindow {
		t.Fatalf("test requires more than %d registered Wave2 enrichers to exercise a windowed stale completion, found %d", enrichDispatchWindow, len(names))
	}

	_, initialTasks := core.HandleEvent(messages.AvailabilityPrefetched{
		Entries:   map[string]int{},
		Resources: seedProbeEnrichResources(names),
		Gen:       sess.AvailabilityGen,
	})
	dispatched := probeEnrichScopes(initialTasks)
	if len(dispatched) != enrichDispatchWindow {
		t.Fatalf("setup: initial window dispatched %d TaskKindProbeEnrich tasks, want %d — cannot exercise the stale-completion pin without a full initial window", len(dispatched), enrichDispatchWindow)
	}
	staleType := dispatched[0]
	dispatchTypeGen := core.Session().EnrichmentTypeGenGet(staleType)
	queuedBefore := len(core.Session().EnrichQueue)

	// Simulate a rerun/refresh bumping staleType's generation while its
	// sweep probe is still in flight.
	core.Session().EnrichmentTypeGenBump(staleType)

	_, followupTasks := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: staleType,
		TypeGen:      dispatchTypeGen,
		Gen:          0,
	})

	if got := countProbeEnrichTasks(followupTasks); got != 1 {
		t.Fatalf("stale completion for %q (dispatched at TypeGen=%d, now stale) dispatched %d additional TaskKindProbeEnrich tasks, want exactly 1 (the refill must still fire even though the stale result itself is discarded); returned scopes: %v",
			staleType, dispatchTypeGen, got, probeEnrichScopes(followupTasks))
	}
	if remaining := len(core.Session().EnrichQueue); remaining != queuedBefore-1 {
		t.Fatalf("after the stale completion for %q, session.EnrichQueue has %d entries, want %d (was %d before) — the queue must still drain by one even when the triggering completion is itself stale", staleType, remaining, queuedBefore-1, queuedBefore)
	}
}

// TestHandleEnrichmentChecked_NonSweepCompletion_DoesNotStealQueueRefill pins
// defect #2a (external review, 2026-07-19): the refill branch
// (core/runtime/handlers_availability.go ~:658-665) fires for ANY
// current-gen TaskKindProbeEnrich completion, not just a completion for a
// type that is actually part of the active sweep. A list-open enrichment probe
// for a type that is not in the active sweep at all
// (HandleResourcesLoaded's list-open dispatch, handlers_resources.go
// ~:130-147) completing while a sweep still has queued types must not pop one
// off the queue — those queued types' eventual sweep dispatch is unrelated to
// this list-open probe's lifecycle.
//
// Regression shape: session.EnrichQueue must not lose its head entry or
// dispatch a follow-up TaskKindProbeEnrich when listOpenType was never part of
// the sweep.
func TestHandleEnrichmentChecked_NonSweepCompletion_DoesNotStealQueueRefill(t *testing.T) {
	sess := session.New()
	core := runtime.New(sess, catalog.All())
	names := allWave2ShortNames(t, core)
	if len(names) <= enrichDispatchWindow+1 {
		t.Fatalf("test requires more than %d registered Wave2 enrichers to leave a queue behind the window and one non-sweep list-open type, found %d", enrichDispatchWindow+1, len(names))
	}
	sweepNames := names[:len(names)-1]
	listOpenType := names[len(names)-1]

	_, initialTasks := core.HandleEvent(messages.AvailabilityPrefetched{
		Entries:   map[string]int{},
		Resources: seedProbeEnrichResources(sweepNames),
		Gen:       sess.AvailabilityGen,
	})
	if got := countProbeEnrichTasks(initialTasks); got != enrichDispatchWindow {
		t.Fatalf("setup: initial window dispatched %d TaskKindProbeEnrich tasks, want %d", got, enrichDispatchWindow)
	}

	queueBefore := append([]string(nil), core.Session().EnrichQueue...)
	if len(queueBefore) < 2 {
		t.Fatalf("test requires at least 2 queued types (found %d) so the list-open victim type can be distinct from the queue head", len(queueBefore))
	}

	_, dispatchTasks := core.HandleResourcesLoaded(runtime.ResourcesLoadedEvent{
		ResourceType: listOpenType,
		Resources:    []resource.Resource{{ID: listOpenType + "-list-open-r1"}},
	})
	if got := countProbeEnrichTasks(dispatchTasks); got != 1 {
		t.Fatalf("setup: list-open dispatch for %q produced %d TaskKindProbeEnrich tasks, want 1", listOpenType, got)
	}

	_, followupTasks := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: listOpenType,
		TypeGen:      0,
		Gen:          0,
	})

	if got := countProbeEnrichTasks(followupTasks); got != 0 {
		t.Fatalf("list-open completion for %q (not a member of the current sweep window) dispatched %d TaskKindProbeEnrich follow-up tasks, want 0 — it must not steal a sweep refill; returned scopes: %v",
			listOpenType, got, probeEnrichScopes(followupTasks))
	}
	if got := core.Session().EnrichQueue; !slices.Equal(got, queueBefore) {
		t.Fatalf("list-open completion for %q mutated session.EnrichQueue: before=%v after=%v — a non-sweep completion must leave the sweep's queue untouched", listOpenType, queueBefore, got)
	}
}

// TestHandleEnrichmentChecked_ListOpenDuringSweep_NoDoubleDispatchNoCounterOvercounts
// pins defect #2b (external review, 2026-07-19): the full interleaving
// defect #2a's setup enables. An unrelated list-open completion stealing a
// sweep refill can cause the stolen type to be dispatched twice — once via
// its own list-open probe, once via the steal-refill treating it as "next in
// the sweep queue" — and inflates session.EnrichChecked past
// session.EnrichTotal, the counters that back the menu's Enriching X/Y
// progress badge.
//
// RED today: driving the full interleaving to exhaustion dispatches the
// type sitting at the queue head (when the list-open probe completes) a
// second time, and session.EnrichChecked overshoots EnrichTotal by the
// number of such double dispatches.
func TestHandleEnrichmentChecked_ListOpenDuringSweep_NoDoubleDispatchNoCounterOvercounts(t *testing.T) {
	sess := session.New()
	core := runtime.New(sess, catalog.All())
	names := allWave2ShortNames(t, core)
	if len(names) <= enrichDispatchWindow {
		t.Fatalf("test requires more than %d registered Wave2 enrichers to leave a queue behind the window, found %d", enrichDispatchWindow, len(names))
	}

	_, initialTasks := core.HandleEvent(messages.AvailabilityPrefetched{
		Entries:   map[string]int{},
		Resources: seedProbeEnrichResources(names),
		Gen:       sess.AvailabilityGen,
	})

	dispatchCount := map[string]int{}
	var pending []string
	for _, name := range probeEnrichScopes(initialTasks) {
		dispatchCount[name]++
		pending = append(pending, name)
	}

	if len(core.Session().EnrichQueue) == 0 {
		t.Fatalf("setup: session.EnrichQueue is empty after the initial window dispatch — cannot exercise a list-open probe against a still-queued sweep")
	}
	listOpenType := core.Session().EnrichQueue[0]

	_, dispatchTasks := core.HandleResourcesLoaded(runtime.ResourcesLoadedEvent{
		ResourceType: listOpenType,
		Resources:    []resource.Resource{{ID: listOpenType + "-list-open-r1"}},
	})
	for _, name := range probeEnrichScopes(dispatchTasks) {
		dispatchCount[name]++
		pending = append(pending, name)
	}

	// Bound the drain loop defensively, mirroring
	// TestStartEnrichment_FullDrain_EveryQueuedTypeEnrichedExactlyOnce — a
	// duplicate-dispatch bug could otherwise loop far longer than the
	// well-behaved case.
	maxIterations := len(names) * 2
	iterations := 0
	for len(pending) > 0 && iterations < maxIterations {
		iterations++
		name := pending[0]
		pending = pending[1:]

		_, followup := core.HandleEvent(messages.EnrichmentChecked{
			ResourceType: name,
			TypeGen:      0,
			Gen:          0,
		})
		for _, next := range probeEnrichScopes(followup) {
			dispatchCount[next]++
			pending = append(pending, next)
		}
	}
	if iterations >= maxIterations {
		t.Fatalf("full drain with one interleaved list-open completion (for %q) did not terminate within %d completions — suspect an unbounded refill loop (dispatchCount so far: %v)", listOpenType, maxIterations, dispatchCount)
	}

	for _, name := range names {
		if n := dispatchCount[name]; n > 1 {
			t.Errorf("type %q was enrichment-dispatched %d times within this sweep generation, want at most 1 — a non-sweep completion (list-open probe of %q) must never cause a type to be dispatched twice", name, n, listOpenType)
		}
	}

	if got, want := core.Session().EnrichChecked, core.Session().EnrichTotal; got != want {
		t.Errorf("session.EnrichChecked ended at %d after the full drain, want exactly %d (session.EnrichTotal, the sweep's queue size at kickoff) — a stolen refill must not inflate the menu's Enriching X/Y progress counter", got, want)
	}
}

// TestHandleEnrichmentChecked_ListOpenQueuedCompletionBeforeRefill_CoversQueueEntry
// pins the opposite ordering from
// TestHandleEnrichmentChecked_ListOpenDuringSweep_NoDoubleDispatchNoCounterOvercounts:
// the list-open probe for a type still in EnrichQueue completes BEFORE any
// sweep-member completion reaches refillEnrichSweep. That completion has
// already covered the queued type, so the later refill must not dispatch the
// same type again.
func TestHandleEnrichmentChecked_ListOpenQueuedCompletionBeforeRefill_CoversQueueEntry(t *testing.T) {
	sess := session.New()
	core := runtime.New(sess, catalog.All())
	names := allWave2ShortNames(t, core)
	if len(names) <= enrichDispatchWindow {
		t.Fatalf("test requires more than %d registered Wave2 enrichers to leave a queue behind the window, found %d", enrichDispatchWindow, len(names))
	}

	_, initialTasks := core.HandleEvent(messages.AvailabilityPrefetched{
		Entries:   map[string]int{},
		Resources: seedProbeEnrichResources(names),
		Gen:       sess.AvailabilityGen,
	})
	dispatched := probeEnrichScopes(initialTasks)
	if len(dispatched) != enrichDispatchWindow {
		t.Fatalf("setup: initial window dispatched %d TaskKindProbeEnrich tasks, want %d", len(dispatched), enrichDispatchWindow)
	}
	if len(core.Session().EnrichQueue) == 0 {
		t.Fatalf("setup: session.EnrichQueue is empty after the initial window dispatch")
	}
	listOpenType := core.Session().EnrichQueue[0]

	_, dispatchTasks := core.HandleResourcesLoaded(runtime.ResourcesLoadedEvent{
		ResourceType: listOpenType,
		Resources:    []resource.Resource{{ID: listOpenType + "-list-open-r1"}},
	})
	if got := countProbeEnrichTasks(dispatchTasks); got != 1 {
		t.Fatalf("setup: list-open dispatch for %q produced %d TaskKindProbeEnrich tasks, want 1", listOpenType, got)
	}

	checkedBefore := core.Session().EnrichChecked
	_, completionTasks := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: listOpenType,
		TypeGen:      0,
		Gen:          0,
	})
	if got := countProbeEnrichTasks(completionTasks); got != 0 {
		t.Fatalf("list-open completion for queued %q dispatched %d follow-up enrich tasks, want 0 because no sweep slot was freed; returned scopes: %v",
			listOpenType, got, probeEnrichScopes(completionTasks))
	}
	if got, want := core.Session().EnrichChecked, checkedBefore+1; got != want {
		t.Fatalf("list-open completion for queued %q left EnrichChecked=%d, want %d — the queued sweep entry was covered by this completion",
			listOpenType, got, want)
	}
	if slices.Contains(core.Session().EnrichQueue, listOpenType) {
		t.Fatalf("list-open completion for queued %q left it in EnrichQueue: %v — the next sweep refill would dispatch a duplicate",
			listOpenType, core.Session().EnrichQueue)
	}

	_, followup := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: dispatched[0],
		TypeGen:      0,
		Gen:          0,
	})
	if scopes := probeEnrichScopes(followup); slices.Contains(scopes, listOpenType) {
		t.Fatalf("next sweep refill dispatched %q again after its list-open completion had already covered it; returned scopes: %v",
			listOpenType, scopes)
	}
}

func enrichProgressIntent(intents []runtime.UIIntent, checked, total int) bool {
	for _, intent := range intents {
		progress, ok := intent.(runtime.PatchMenuEnrichProgress)
		if ok && progress.Checked == checked && progress.Total == total {
			return true
		}
	}
	return false
}

// TestHandleEnrichmentChecked_StaleSweepMemberCompletion_AdvancesProgressToDone
// pins the progress half of stale sweep completions: even when TypeGen makes
// the payload obsolete, the task that occupied a sweep slot did complete, so
// EnrichChecked must advance and the done signal must be emitted when it was
// the last outstanding member.
func TestHandleEnrichmentChecked_StaleSweepMemberCompletion_AdvancesProgressToDone(t *testing.T) {
	sess := session.New()
	core := runtime.New(sess, catalog.All())
	names := allWave2ShortNames(t, core)
	staleType := names[0]

	dispatchTypeGen := sess.EnrichmentTypeGenBump(staleType)
	sess.EnrichSweepMembers = map[string]bool{staleType: true}
	sess.EnrichChecked = 0
	sess.EnrichTotal = 1
	sess.EnrichQueue = nil
	sess.EnrichmentTypeGenBump(staleType)

	intents, tasks := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: staleType,
		TypeGen:      dispatchTypeGen,
		Gen:          0,
	})

	if got := sess.EnrichChecked; got != 1 {
		t.Fatalf("stale sweep-member completion left EnrichChecked=%d, want 1 — stale payload must not strand sweep progress", got)
	}
	if got := countProbeEnrichTasks(tasks); got != 0 {
		t.Fatalf("stale final completion dispatched %d TaskKindProbeEnrich tasks with an empty queue, want 0; returned scopes: %v",
			got, probeEnrichScopes(tasks))
	}
	if !enrichProgressIntent(intents, 0, 0) {
		t.Fatalf("stale final completion did not emit PatchMenuEnrichProgress{0,0}; intents=%v", intents)
	}
}

// TestHandleEnrichmentChecked_RotateMidSweep_OldCompletionNoRefillNoCounterMovement
// pins the rotation boundary a fix for defects #1/#2 must not break: a
// completion for a type dispatched by a sweep that a profile/region switch
// (session.Rotate) has since abandoned must be discarded outright, with no
// refill and no progress-counter movement, however sweep membership ends up
// being tracked.
//
// Documented status: GREEN today. Core.HandleEvent's central guard
// (core/runtime/orchestrator.go ~:103, messages.IsStale) already discards
// any GenStamped event whose Gen no longer matches session.EnrichmentGen
// before handleEnrichmentChecked ever runs, and Rotate bumps EnrichmentGen
// unconditionally — a mechanism independent of the per-type TypeGen guard
// defect #1 exploits. Kept here as a regression pin so a future
// sweep-membership fix (e.g. a session-held in-flight-type set) cannot
// reintroduce cross-rotation leakage by keying off something Rotate doesn't
// clear.
func TestHandleEnrichmentChecked_RotateMidSweep_OldCompletionNoRefillNoCounterMovement(t *testing.T) {
	sess := session.New()
	core := runtime.New(sess, catalog.All())
	names := allWave2ShortNames(t, core)
	if len(names) <= enrichDispatchWindow {
		t.Fatalf("test requires more than %d registered Wave2 enrichers to leave a queue behind the window, found %d", enrichDispatchWindow, len(names))
	}

	_, initialTasks := core.HandleEvent(messages.AvailabilityPrefetched{
		Entries:   map[string]int{},
		Resources: seedProbeEnrichResources(names),
		Gen:       sess.AvailabilityGen,
	})
	dispatched := probeEnrichScopes(initialTasks)
	if len(dispatched) != enrichDispatchWindow {
		t.Fatalf("setup: initial window dispatched %d TaskKindProbeEnrich tasks, want %d", len(dispatched), enrichDispatchWindow)
	}
	oldSweepType := dispatched[0]
	genAtDispatch := core.Session().EnrichmentGen

	core.Session().Rotate()

	if got := len(core.Session().EnrichQueue); got != 0 {
		t.Fatalf("setup: session.EnrichQueue has %d entries immediately after Rotate, want 0", got)
	}

	_, followupTasks := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: oldSweepType,
		TypeGen:      0,
		Gen:          genAtDispatch,
	})

	if got := countProbeEnrichTasks(followupTasks); got != 0 {
		t.Errorf("post-rotation completion for %q (old sweep, Gen=%d) dispatched %d TaskKindProbeEnrich tasks, want 0 — the old sweep must not refill after rotation; returned scopes: %v",
			oldSweepType, genAtDispatch, got, probeEnrichScopes(followupTasks))
	}
	if got := len(core.Session().EnrichQueue); got != 0 {
		t.Errorf("post-rotation completion for %q left session.EnrichQueue with %d entries, want 0 (the new pair's queue must stay empty)", oldSweepType, got)
	}
	if got := core.Session().EnrichChecked; got != 0 {
		t.Errorf("post-rotation completion for %q moved session.EnrichChecked to %d, want 0 (must not advance the new pair's progress counter)", oldSweepType, got)
	}
}

// TestHandleEnrichmentChecked_ListOpenDeepInQueue_EarlyCompletionCausesRedundantDispatch
// pins a race the existing
// TestHandleEnrichmentChecked_ListOpenDuringSweep_NoDoubleDispatchNoCounterOvercounts
// pin cannot reach: that pin's list-open type sits at the queue HEAD, so the
// very next refill absorbs it (session.EnrichListOpenPending is still true
// when refillEnrichSweep pops it) before its own completion is ever
// processed. Here the list-open type sits at the queue TAIL instead — deep
// enough that its own list-open completion lands well before any refill
// walks the queue that far.
//
// The queued entry must be removed and counted as covered when that current
// list-open completion arrives. Otherwise, by the time refillEnrichSweep
// finally pops the type's queue entry, nothing marks it as already covered and
// it dispatches a second, redundant TaskKindProbeEnrich for a type whose
// enrichment already landed this sweep.
func TestHandleEnrichmentChecked_ListOpenDeepInQueue_EarlyCompletionCausesRedundantDispatch(t *testing.T) {
	sess := session.New()
	core := runtime.New(sess, catalog.All())
	names := allWave2ShortNames(t, core)
	if len(names) <= enrichDispatchWindow+1 {
		t.Fatalf("test requires more than %d registered Wave2 enrichers so a list-open type can sit deep in the queue, unreachable by the next refill, found %d", enrichDispatchWindow+1, len(names))
	}

	_, initialTasks := core.HandleEvent(messages.AvailabilityPrefetched{
		Entries:   map[string]int{},
		Resources: seedProbeEnrichResources(names),
		Gen:       sess.AvailabilityGen,
	})
	dispatchCount := map[string]int{}
	var pending []string
	for _, name := range probeEnrichScopes(initialTasks) {
		dispatchCount[name]++
		pending = append(pending, name)
	}
	if len(dispatchCount) != enrichDispatchWindow {
		t.Fatalf("setup: initial window dispatched %d TaskKindProbeEnrich tasks, want %d", len(dispatchCount), enrichDispatchWindow)
	}

	queueBefore := append([]string(nil), core.Session().EnrichQueue...)
	if len(queueBefore) < 2 {
		t.Fatalf("test requires at least 2 queued types behind the window (found %d) so the list-open type can sit deep, unreachable by the immediate next refill", len(queueBefore))
	}
	listOpenType := queueBefore[len(queueBefore)-1]

	_, listOpenDispatch := core.HandleResourcesLoaded(runtime.ResourcesLoadedEvent{
		ResourceType: listOpenType,
		Resources:    []resource.Resource{{ID: listOpenType + "-list-open-r1"}},
	})
	if got := countProbeEnrichTasks(listOpenDispatch); got != 1 {
		t.Fatalf("setup: list-open dispatch for %q produced %d TaskKindProbeEnrich tasks, want 1", listOpenType, got)
	}
	dispatchCount[listOpenType]++

	// The list-open probe's own completion lands FIRST — current gen, well
	// before any sweep refill has walked the queue anywhere near
	// listOpenType's slot at the tail. This is the ordering the queue-head
	// existing pin cannot exercise.
	_, earlyFollowup := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: listOpenType,
		TypeGen:      0,
		Gen:          0,
	})
	if got := countProbeEnrichTasks(earlyFollowup); got != 0 {
		t.Fatalf("setup: the list-open completion for %q itself dispatched %d TaskKindProbeEnrich tasks (not a sweep member yet), want 0; returned scopes: %v", listOpenType, got, probeEnrichScopes(earlyFollowup))
	}

	// Drive the sweep's own completions (and any further refills) to
	// exhaustion so every refill gets a chance to walk the queue as far as
	// listOpenType's slot at the tail.
	maxIterations := len(names) * 2
	iterations := 0
	for len(pending) > 0 && iterations < maxIterations {
		iterations++
		name := pending[0]
		pending = pending[1:]

		_, followup := core.HandleEvent(messages.EnrichmentChecked{
			ResourceType: name,
			TypeGen:      0,
			Gen:          0,
		})
		for _, next := range probeEnrichScopes(followup) {
			dispatchCount[next]++
			pending = append(pending, next)
		}
	}
	if iterations >= maxIterations {
		t.Fatalf("full drain did not terminate within %d completions for %d queued types plus one early list-open completion — suspect an unbounded refill loop (dispatchCount so far: %v)", maxIterations, len(names), dispatchCount)
	}

	if got := dispatchCount[listOpenType]; got != 1 {
		t.Errorf("type %q (list-open probe deep in queue, early completion) was dispatched %d TaskKindProbeEnrich tasks across the sweep, want exactly 1 — its early list-open completion must be recognized by the time refillEnrichSweep later pops its queue slot, not redispatched", listOpenType, got)
	}

	if remaining := len(core.Session().EnrichQueue); remaining != 0 {
		t.Errorf("after the full drain, session.EnrichQueue has %d entries left, want 0", remaining)
	}

	if got, want := core.Session().EnrichChecked, core.Session().EnrichTotal; got != want {
		t.Errorf("session.EnrichChecked ended at %d after the full drain, want exactly %d (session.EnrichTotal) — this must stay exact through any fix for the redundant dispatch above", got, want)
	}
}

// TestHandleEnrichmentChecked_ListOpenDeepInQueue_StaleCompletionStillDispatchesFreshProbe
// is the companion GREEN case to
// TestHandleEnrichmentChecked_ListOpenDeepInQueue_EarlyCompletionCausesRedundantDispatch:
// the list-open probe's early completion carries a STALE TypeGen (something
// else — a second rerun/refresh racing the same list-open probe — bumps the
// type's generation again between dispatch and completion), so its payload
// never lands (msg.TypeGen != session.EnrichmentTypeGenGet(...) discards it,
// handlers_availability.go ~:547-549). The type's queue entry must NOT be
// treated as already covered by that discarded completion — the sweep still
// needs a real probe for it later.
//
// Regression shape: a fix for the sibling redundant-dispatch defect must not
// absorb on ANY non-member completion regardless of staleness. A stale payload
// did not cover the queued type, so refillEnrichSweep must later dispatch a
// genuine probe for it.
func TestHandleEnrichmentChecked_ListOpenDeepInQueue_StaleCompletionStillDispatchesFreshProbe(t *testing.T) {
	sess := session.New()
	core := runtime.New(sess, catalog.All())
	names := allWave2ShortNames(t, core)
	if len(names) <= enrichDispatchWindow+1 {
		t.Fatalf("test requires more than %d registered Wave2 enrichers so a list-open type can sit deep in the queue, unreachable by the next refill, found %d", enrichDispatchWindow+1, len(names))
	}

	_, initialTasks := core.HandleEvent(messages.AvailabilityPrefetched{
		Entries:   map[string]int{},
		Resources: seedProbeEnrichResources(names),
		Gen:       sess.AvailabilityGen,
	})
	dispatchCount := map[string]int{}
	var pending []string
	for _, name := range probeEnrichScopes(initialTasks) {
		dispatchCount[name]++
		pending = append(pending, name)
	}
	if len(dispatchCount) != enrichDispatchWindow {
		t.Fatalf("setup: initial window dispatched %d TaskKindProbeEnrich tasks, want %d", len(dispatchCount), enrichDispatchWindow)
	}

	queueBefore := append([]string(nil), core.Session().EnrichQueue...)
	if len(queueBefore) < 2 {
		t.Fatalf("test requires at least 2 queued types behind the window (found %d) so the list-open type can sit deep, unreachable by the immediate next refill", len(queueBefore))
	}
	listOpenType := queueBefore[len(queueBefore)-1]
	core.Session().EnrichmentTypeGenBump(listOpenType)

	_, listOpenDispatch := core.HandleResourcesLoaded(runtime.ResourcesLoadedEvent{
		ResourceType: listOpenType,
		Resources:    []resource.Resource{{ID: listOpenType + "-list-open-r1"}},
	})
	if got := countProbeEnrichTasks(listOpenDispatch); got != 1 {
		t.Fatalf("setup: list-open dispatch for %q produced %d TaskKindProbeEnrich tasks, want 1", listOpenType, got)
	}
	dispatchCount[listOpenType]++
	dispatchTypeGen := core.Session().EnrichmentTypeGenGet(listOpenType)

	// Something else (a second rerun/refresh racing the same list-open
	// probe) bumps listOpenType's generation before its completion lands —
	// the completion below is now stale, and its payload must be discarded.
	core.Session().EnrichmentTypeGenBump(listOpenType)

	_, staleFollowup := core.HandleEvent(messages.EnrichmentChecked{
		ResourceType: listOpenType,
		TypeGen:      dispatchTypeGen,
		Gen:          0,
	})
	if got := countProbeEnrichTasks(staleFollowup); got != 0 {
		t.Fatalf("setup: the stale list-open completion for %q itself dispatched %d TaskKindProbeEnrich tasks, want 0", listOpenType, got)
	}

	// Drive the sweep's own completions to exhaustion so a refill walks the
	// queue as far as listOpenType's slot at the tail.
	maxIterations := len(names) * 2
	iterations := 0
	for len(pending) > 0 && iterations < maxIterations {
		iterations++
		name := pending[0]
		pending = pending[1:]

		_, followup := core.HandleEvent(messages.EnrichmentChecked{
			ResourceType: name,
			TypeGen:      0,
			Gen:          0,
		})
		for _, next := range probeEnrichScopes(followup) {
			dispatchCount[next]++
			pending = append(pending, next)
		}
	}
	if iterations >= maxIterations {
		t.Fatalf("full drain did not terminate within %d completions for %d queued types plus one stale list-open completion — suspect an unbounded refill loop (dispatchCount so far: %v)", maxIterations, len(names), dispatchCount)
	}

	if got := dispatchCount[listOpenType]; got != 2 {
		t.Errorf("type %q (stale list-open completion, payload discarded) was dispatched %d TaskKindProbeEnrich tasks across the sweep, want exactly 2 (the original stale/discarded probe plus one genuine, required fresh probe) — a stale completion must not cause the queue entry to be silently absorbed as 'already covered'", listOpenType, got)
	}

	if remaining := len(core.Session().EnrichQueue); remaining != 0 {
		t.Errorf("after the full drain, session.EnrichQueue has %d entries left, want 0", remaining)
	}

	if got, want := core.Session().EnrichChecked, core.Session().EnrichTotal; got != want {
		t.Errorf("session.EnrichChecked ended at %d after the full drain, want exactly %d (session.EnrichTotal)", got, want)
	}
}
