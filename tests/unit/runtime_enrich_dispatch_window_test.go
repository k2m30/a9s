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
