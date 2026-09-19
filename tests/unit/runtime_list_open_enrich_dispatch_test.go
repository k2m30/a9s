// A normal list-open
// (TypeGen==0, the common case) dispatches TaskKindProbeEnrich for an
// issue-capable type so row flags and the menu badge populate without waiting
// for a Ctrl+R rerun.
//
// HandleResourcesLoaded (core/runtime/handlers_resources.go) dispatches on
// `ev.TypeGen == 0 && ev.Err == nil && !ev.Append &&
// ev.Provenance.CanonicalList() && ... c.HasIssueEnricher(resType)`. An event
// literal meant to reach that branch MUST set Provenance:
// messages.FetchProvenanceCanonicalList — the zero value
// (FetchProvenanceUnknown) fails CanonicalList() and produces 0 tasks with no
// other signal of why. The negative controls (no issue enricher, Append, Err)
// do not need it: those guards block dispatch regardless of provenance.
//
// The Core-level tests mirror handlers_resources_test.go
// (runtime.New(session.New(), catalog.All()) with hasTask-shaped assertions);
// the web-lane test drives app.New(core)+ctrl.Handle(messages.ResourcesLoaded{...}).
package unit

import (
	"errors"
	"testing"

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
// HasIssueEnricher. Fails the test immediately if none exists (would mean the
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

// TestHandleResourcesLoaded_ListOpen_IssueCapableType_DispatchesProbeEnrich:
// a normal (TypeGen==0, Append==false, Err==nil, Provenance
// CanonicalList) list-open result for an issue-enricher-capable type must
// request TaskKindProbeEnrich so Wave 2 row flags and the menu badge
// populate on a live/headless/web session that never runs the Ctrl+R rerun
// path.
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
		Provenance:   messages.FetchProvenanceCanonicalList,
	})

	if !listOpenEnrichHasTask(tasks, runtime.TaskKindProbeEnrich, shortName) {
		t.Fatalf("HandleResourcesLoaded(list-open, TypeGen=0) for issue-capable type %q returned %d tasks, want a TaskKindProbeEnrich for %q — Wave-2 never dispatches on normal list-open (live defect: no row flags, no menu badge)", shortName, len(tasks), shortName)
	}
}

// TestHandleResourcesLoaded_ListOpen_NoIssueEnricherType_NoProbeEnrich: a
// type with NO registered issue enricher must never
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

// TestHandleResourcesLoaded_AppendPage_NoProbeEnrich: a load-more
// (Append==true) page must NOT re-trigger TaskKindProbeEnrich: the type was
// already enriched (or queued for enrichment) when its first page loaded, so
// re-probing on every page would be redundant work, matching the !ev.Append
// guard on the cross-view PatchResourceCache branch.
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

// TestHandleResourcesLoaded_ErrorLoad_NoProbeEnrich: a failed fetch (Err
// non-nil, Resources present or not) has nothing new and reliable to enrich,
// so it must NOT request TaskKindProbeEnrich.
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

// TestControllerHandle_ListOpen_ResourcesLoaded_DispatchesProbeEnrich is the
// web-lane surface. A headless Controller in web UI mode
// receiving a real messages.ResourcesLoaded (Provenance: CanonicalList)
// through Controller.Handle (the same entry point DrainSync and the web
// renderer use) must return a TaskKindProbeEnrich task among the
// TaskRequests, for an issue-capable type.
func TestControllerHandle_ListOpen_ResourcesLoaded_DispatchesProbeEnrich(t *testing.T) {
	sess := session.New()
	core := runtime.New(sess, resource.AllResourceTypes())
	shortName := findWave2TypeShortName(t, core)
	ctrl := newBlessedController(t, core)
	ctrl.SetUIMode("web")

	_, tasks := handlePage(ctrl, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
		ResourceType: shortName,
		Resources:    []resource.Resource{{ID: "r-1"}, {ID: "r-2"}},
		Gen:          0,
	})

	if !listOpenEnrichHasTask(tasks, runtime.TaskKindProbeEnrich, shortName) {
		t.Fatalf("ctrl.Handle(messages.ResourcesLoaded) for issue-capable type %q returned %d tasks, want a TaskKindProbeEnrich for %q — a headless/web session opening a list never dispatches Wave-2 (live defect: no row flags, no menu badge)", shortName, len(tasks), shortName)
	}
}
