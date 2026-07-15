// runtime_list_open_enrich_dispatch_test.go — RED pins for the live-web
// defect: a normal list-open (TypeGen==0, the common case) never dispatches
// TaskKindProbeEnrich.
//
// Root cause (verified by reading the real call chains, not the dispatch's
// claim alone):
//
//   - core/runtime/handlers_resources.go's HandleResourcesLoaded only
//     appends a TaskRequest{Kind: TaskKindProbeEnrich} inside the
//     `if ev.TypeGen != 0 && ev.TypeGen == c.session.EnrichmentTypeGen[...]`
//     branch — the Ctrl+R-for-rerun path. A normal list-open message carries
//     TypeGen == 0, so that branch never fires and no Wave-2 task is ever
//     requested for the common case.
//   - The TUI adapter (internal/tui/runtime_adapter_resources.go's
//     handleResourcesLoaded) calls Core.HandleResourcesLoaded directly and
//     forwards intents/tasks via dispatchCoreScreenResult — it has NO
//     additional list-open Wave-2 dispatch of its own; probeEnrichment
//     (internal/tui/probe_adapter.go) is never invoked from production code
//     at all (grep-confirmed: only test files call m.probeEnrichment). So the
//     TUI's real production behavior for a plain list-open ALSO never fires
//     Wave-2 today, and pinning "the TUI already does this correctly" would
//     be pinning a fiction — pins below match the actual (broken) shared
//     behavior instead of an imagined TUI-only special case.
//   - The headless/web lane is even more clearly broken: HandleEvent's
//     messages.ResourcesLoaded case (core/runtime/orchestrator.go) is
//     explicitly "Row-store dual-write ONLY" — it calls
//     observeResourcesLoadedRows and returns nil, nil, NEVER reaching
//     Core.HandleResourcesLoaded. Controller.Handle (core/app/handle.go)
//     only adds its own append-only branches (refreshTasksForIntents,
//     autoOpenSingleDetail) on top of HandleEvent's tasks — none of them
//     request TaskKindProbeEnrich. So a web/headless session that opens a
//     list gets no row flags and no menu badge, exactly as the live defect
//     report describes.
//
// Harness: mirrors the package-runtime style in handlers_resources_test.go
// (Core built via runtime.New(session.New(), catalog.All()), findIntent/
// hasTask-shaped assertions) for pins 1-3, and the tests/unit
// app.New(core)+ctrl.Handle(messages.ResourcesLoaded{...}) seam used by
// qa_late_replace_and_false_exact_test.go / qa_cache_lifecycle_test.go for
// pin 4 — the exact web-lane surface named in the dispatch.
package unit

import (
	"errors"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// listOpenEnrichHasTask reports whether xs contains a TaskRequest of Kind k
// scoped to scope. Duplicated from handlers_resources_test.go's hasTask
// (package runtime, not importable here) rather than exported for a single
// test file's use.
func listOpenEnrichHasTask(xs []runtime.TaskRequest, k runtime.TaskKind, scope string) bool {
	for _, t := range xs {
		if t.Key.Kind == k && (scope == "" || t.Key.Scope == scope) {
			return true
		}
	}
	return false
}

// findWave2TypeShortName returns the ShortName of the first catalog entry
// whose Wave2 field is non-nil AND awsclient.Wave2EnricherFor confirms it via
// HasIssueEnricher, per the dispatch's "verify via HasIssueEnricher"
// instruction. Fails the test immediately if none exists (would mean the
// whole pin is meaningless).
func findWave2TypeShortName(t *testing.T, core *runtime.Core) string {
	t.Helper()
	for _, td := range catalog.All() {
		if td.Wave2 != nil && core.HasIssueEnricher(td.ShortName) {
			return td.ShortName
		}
	}
	t.Fatal("no catalog entry with a registered Wave2 issue enricher found — cannot exercise the ProbeEnrich dispatch pin")
	return ""
}

// findNoWave2TypeShortName returns the ShortName of the first catalog entry
// with NO issue enricher (HasIssueEnricher false), for the negative pin.
func findNoWave2TypeShortName(t *testing.T, core *runtime.Core) string {
	t.Helper()
	for _, td := range catalog.All() {
		if !core.HasIssueEnricher(td.ShortName) {
			return td.ShortName
		}
	}
	t.Fatal("every catalog entry has a registered issue enricher — cannot exercise the negative ProbeEnrich pin")
	return ""
}

// TestHandleResourcesLoaded_ListOpen_IssueCapableType_DispatchesProbeEnrich
// is pin 1: a normal (TypeGen==0, Append==false, Err==nil) list-open result
// for an issue-enricher-capable type must request TaskKindProbeEnrich so Wave
// 2 row flags and the menu badge populate on a live/headless/web session that
// never runs the Ctrl+R rerun path.
//
// RED today: HandleResourcesLoaded only appends this task inside the
// TypeGen!=0 rerun-token-match branch; TypeGen==0 short-circuits it entirely.
func TestHandleResourcesLoaded_ListOpen_IssueCapableType_DispatchesProbeEnrich(t *testing.T) {
	sess := session.New()
	core := runtime.New(sess, catalog.All())
	shortName := findWave2TypeShortName(t, core)

	rows := []resource.Resource{{ID: "r-1"}, {ID: "r-2"}, {ID: "r-3"}}
	_, tasks := core.HandleResourcesLoaded(runtime.ResourcesLoadedEvent{
		ResourceType: shortName,
		Resources:    rows,
		TypeGen:      0,
		Append:       false,
		Err:          nil,
	})

	if !listOpenEnrichHasTask(tasks, runtime.TaskKindProbeEnrich, shortName) {
		t.Fatalf("HandleResourcesLoaded(list-open, TypeGen=0) for issue-capable type %q returned %d tasks, want a TaskKindProbeEnrich for %q — Wave-2 never dispatches on normal list-open (live defect: no row flags, no menu badge)", shortName, len(tasks), shortName)
	}
}

// TestHandleResourcesLoaded_ListOpen_NoIssueEnricherType_NoProbeEnrich is pin
// 2 (negative control): a type with NO registered issue enricher must never
// request TaskKindProbeEnrich — there would be nothing for the task to run.
func TestHandleResourcesLoaded_ListOpen_NoIssueEnricherType_NoProbeEnrich(t *testing.T) {
	sess := session.New()
	core := runtime.New(sess, catalog.All())
	shortName := findNoWave2TypeShortName(t, core)

	_, tasks := core.HandleResourcesLoaded(runtime.ResourcesLoadedEvent{
		ResourceType: shortName,
		Resources:    []resource.Resource{{ID: "r-1"}},
		TypeGen:      0,
		Append:       false,
	})

	if listOpenEnrichHasTask(tasks, runtime.TaskKindProbeEnrich, shortName) {
		t.Errorf("HandleResourcesLoaded for non-issue-capable type %q returned a TaskKindProbeEnrich task — HasIssueEnricher(%q) is false, there is no enricher to run", shortName, shortName)
	}
}

// TestHandleResourcesLoaded_AppendPage_NoProbeEnrich is pin 3's Append case.
// A load-more (Append==true) page must NOT re-trigger TaskKindProbeEnrich:
// the type was already enriched (or queued for enrichment) when its first
// page loaded, so re-probing on every subsequent page would be redundant
// work fanned out per page. This mirrors the pre-existing !ev.Append guard
// already governing the cross-view PatchResourceCache branch above in the
// same handler — the list-open Wave-2 dispatch this file pins must respect
// the same page-1-only semantics, not fire on every LoadMore.
func TestHandleResourcesLoaded_AppendPage_NoProbeEnrich(t *testing.T) {
	sess := session.New()
	core := runtime.New(sess, catalog.All())
	shortName := findWave2TypeShortName(t, core)

	_, tasks := core.HandleResourcesLoaded(runtime.ResourcesLoadedEvent{
		ResourceType: shortName,
		Resources:    []resource.Resource{{ID: "r-4"}},
		TypeGen:      0,
		Append:       true,
	})

	if listOpenEnrichHasTask(tasks, runtime.TaskKindProbeEnrich, shortName) {
		t.Errorf("HandleResourcesLoaded(Append=true) for issue-capable type %q returned a TaskKindProbeEnrich task, want none — a load-more page must not re-trigger the probe (already enriched/queued on page 1)", shortName)
	}
}

// TestHandleResourcesLoaded_ErrorLoad_NoProbeEnrich is pin 3's error case.
// The TUI adapter's only Err-specific behavior is Core's own FlashIntent
// (handlers_resources.go's `if ev.Err != nil` branch) — there is no
// Err-conditional skip on the probe dispatch anywhere in the adapter or
// Core. A failed fetch (Err non-nil, Resources present or not) has nothing
// new and reliable to enrich, so the fixed contract must NOT request
// TaskKindProbeEnrich on an errored load.
func TestHandleResourcesLoaded_ErrorLoad_NoProbeEnrich(t *testing.T) {
	sess := session.New()
	core := runtime.New(sess, catalog.All())
	shortName := findWave2TypeShortName(t, core)

	_, tasks := core.HandleResourcesLoaded(runtime.ResourcesLoadedEvent{
		ResourceType: shortName,
		Resources:    []resource.Resource{{ID: "r-5"}},
		TypeGen:      0,
		Append:       false,
		Err:          errors.New("fetch s3: AccessDenied"),
	})

	if listOpenEnrichHasTask(tasks, runtime.TaskKindProbeEnrich, shortName) {
		t.Errorf("HandleResourcesLoaded(Err=non-nil) for %q returned a TaskKindProbeEnrich task, want none — an errored load has nothing reliable to enrich", shortName)
	}
}

// TestControllerHandle_ListOpen_ResourcesLoaded_DispatchesProbeEnrich is pin
// 4: the exact web-lane surface. A headless Controller in web UI mode
// receiving a real messages.ResourcesLoaded through Controller.Handle (the
// same entry point DrainSync and the web renderer use) must return a
// TaskKindProbeEnrich task among the TaskRequests, for an issue-capable type.
//
// RED today: HandleEvent's messages.ResourcesLoaded case
// (core/runtime/orchestrator.go) is documented "Row-store dual-write
// ONLY" — it calls observeResourcesLoadedRows and returns nil, nil,
// NEVER invoking Core.HandleResourcesLoaded. Controller.Handle's only
// additional task sources for this message (refreshTasksForIntents,
// autoOpenSingleDetail) do not request TaskKindProbeEnrich either. So a
// headless/web session opening a list never gets the Wave-2 task at all —
// worse than the TUI's shared TypeGen==0 gap, this lane doesn't even reach
// the gate.
func TestControllerHandle_ListOpen_ResourcesLoaded_DispatchesProbeEnrich(t *testing.T) {
	sess := session.New()
	core := runtime.New(sess, resource.AllResourceTypes())
	shortName := findWave2TypeShortName(t, core)
	ctrl := app.New(core)
	ctrl.SetUIMode("web")

	_, tasks := ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: shortName,
		Resources:    []resource.Resource{{ID: "r-1"}, {ID: "r-2"}},
		Gen:          0,
	})

	if !listOpenEnrichHasTask(tasks, runtime.TaskKindProbeEnrich, shortName) {
		t.Fatalf("ctrl.Handle(messages.ResourcesLoaded) for issue-capable type %q returned %d tasks, want a TaskKindProbeEnrich for %q — a headless/web session opening a list never dispatches Wave-2 (live defect: no row flags, no menu badge)", shortName, len(tasks), shortName)
	}
}
