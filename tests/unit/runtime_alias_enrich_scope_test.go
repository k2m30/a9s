// runtime_alias_enrich_scope_test.go pins the list-open Wave-2 dispatch's
// scope canonicalization: an alias-opened list must still dispatch its
// ProbeEnrich task under the CANONICAL short name, not the raw alias the
// event carried.
//
// "buckets" is a real registered alias for s3 (internal/aws/catalog_databases.go
// Aliases: []string{"s3", "buckets"}), and s3 has a registered Wave2 issue
// enricher (Wave2: IssueEnricher{Fn: EnrichS3PublicAccessBlock, ...}) — so
// c.HasIssueEnricher("buckets") resolves true via the same alias lookup and
// the list-open dispatch condition in HandleResourcesLoaded fires.
//
// RED today (HEAD): HandleResourcesLoaded's list-open branch builds
// TaskKey{Kind: TaskKindProbeEnrich, Scope: ev.ResourceType} directly from
// the event's raw ResourceType with no canonicalization through
// resource.FindResourceType — an alias-opened list ("buckets") dispatches a
// task scoped to "buckets", a scope no downstream enrichment consumer reads
// (RowStore/EnrichmentTypeGen/HasIssueEnricher are all keyed by the
// canonical ShortName "s3" elsewhere in the same file, e.g.
// canonShortName). The task is scoped to the dead alias, not absent.
//
// Harness mirrors tests/unit/runtime_list_open_enrich_dispatch_test.go: pins
// 1-2 use the package-runtime style (runtime.New(session.New(), catalog.All())),
// pin 3 uses the tests/unit app.New(core)+ctrl.Handle(messages.ResourcesLoaded{...})
// web-lane seam.
package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/app"
	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/session"
)

const (
	aliasEnrichScopeAlias     = "buckets"
	aliasEnrichScopeCanonical = "s3"
)

// aliasEnrichScopeSanityCheck asserts the fixture pair used by every test in
// this file is real: "buckets" must resolve via resource.FindResourceType to
// the canonical "s3" ShortName, and "s3" must be issue-capable — otherwise
// every pin below would be exercising a fiction.
func aliasEnrichScopeSanityCheck(t *testing.T, core *runtime.Core) {
	t.Helper()
	td := resource.FindResourceType(aliasEnrichScopeAlias)
	if td == nil {
		t.Fatalf("resource.FindResourceType(%q) = nil — alias fixture is not registered", aliasEnrichScopeAlias)
	}
	if td.ShortName != aliasEnrichScopeCanonical {
		t.Fatalf("resource.FindResourceType(%q).ShortName = %q, want %q", aliasEnrichScopeAlias, td.ShortName, aliasEnrichScopeCanonical)
	}
	if !core.HasIssueEnricher(aliasEnrichScopeCanonical) {
		t.Fatalf("core.HasIssueEnricher(%q) = false — canonical type has no Wave2 issue enricher, pin is meaningless", aliasEnrichScopeCanonical)
	}
}

// TestHandleResourcesLoaded_AliasOpenedList_ProbeEnrichScopedToCanonical is
// pin 1: Core.HandleResourcesLoaded, alias event ResourceType, list-open
// (TypeGen=0, Append=false, Err=nil). The returned ProbeEnrich task's Scope
// must equal the canonical short name "s3", not the alias "buckets".
func TestHandleResourcesLoaded_AliasOpenedList_ProbeEnrichScopedToCanonical(t *testing.T) {
	sess := session.New()
	core := runtime.New(sess, catalog.All())
	aliasEnrichScopeSanityCheck(t, core)

	rows := []resource.Resource{{ID: "bucket-1"}, {ID: "bucket-2"}}
	_, tasks := core.HandleResourcesLoaded(runtime.ResourcesLoadedEvent{
		ResourceType: aliasEnrichScopeAlias,
		Resources:    rows,
		TypeGen:      0,
		Append:       false,
		Err:          nil,
	})

	var found *runtime.TaskRequest
	for i := range tasks {
		if tasks[i].Key.Kind == runtime.TaskKindProbeEnrich {
			found = &tasks[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("HandleResourcesLoaded(alias %q, list-open) returned %d tasks, want a TaskKindProbeEnrich task (none found at all)", aliasEnrichScopeAlias, len(tasks))
	}
	if found.Key.Scope != aliasEnrichScopeCanonical {
		t.Errorf("TaskKindProbeEnrich Scope = %q, want canonical %q — alias-opened list dispatched a dead alias scope no enrichment consumer reads", found.Key.Scope, aliasEnrichScopeCanonical)
	}
}

// TestHandleResourcesLoaded_AliasRerun_ProbeEnrichScopedToCanonical is pin 2:
// the enrichment-rerun branch (TypeGen non-zero, matching
// session.EnrichmentTypeGen) must ALSO scope its ProbeEnrich task to the
// canonical short name when the rerun event carries the alias — the rerun
// branch builds its TaskKey the same unguarded way as the list-open branch.
// session.EnrichmentTypeGen is written and read elsewhere exclusively under
// the canonical ShortName (e.g. internal/runtime/executor.go's
// snap.EnrichmentTypeGen[shortName]), so the gen-guard map is seeded here
// under the canonical key "s3" — matching that convention — while the event
// itself still carries the alias "buckets", exercising the handler's own
// canonicalization of ev.ResourceType before the gen-guard lookup.
func TestHandleResourcesLoaded_AliasRerun_ProbeEnrichScopedToCanonical(t *testing.T) {
	sess := session.New()
	sess.EnrichmentTypeGen[aliasEnrichScopeCanonical] = 7
	core := runtime.New(sess, catalog.All())
	aliasEnrichScopeSanityCheck(t, core)

	rows := []resource.Resource{{ID: "bucket-1"}}
	_, tasks := core.HandleResourcesLoaded(runtime.ResourcesLoadedEvent{
		ResourceType: aliasEnrichScopeAlias,
		Resources:    rows,
		TypeGen:      7,
		Append:       false,
		Err:          nil,
	})

	var found *runtime.TaskRequest
	for i := range tasks {
		if tasks[i].Key.Kind == runtime.TaskKindProbeEnrich {
			found = &tasks[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("HandleResourcesLoaded(alias %q, rerun TypeGen=7) returned %d tasks, want a TaskKindProbeEnrich task", aliasEnrichScopeAlias, len(tasks))
	}
	if found.Key.Scope != aliasEnrichScopeCanonical {
		t.Errorf("TaskKindProbeEnrich Scope = %q, want canonical %q — alias rerun dispatched a dead alias scope", found.Key.Scope, aliasEnrichScopeCanonical)
	}
}

// TestControllerHandle_AliasOpenedList_ProbeEnrichScopedToCanonical is pin 3:
// the exact web-lane surface named in the dispatch — a headless Controller
// in web UI mode receiving a real messages.ResourcesLoaded with an alias
// ResourceType through Controller.Handle must return a TaskKindProbeEnrich
// task scoped to the canonical short name.
func TestControllerHandle_AliasOpenedList_ProbeEnrichScopedToCanonical(t *testing.T) {
	sess := session.New()
	core := runtime.New(sess, resource.AllResourceTypes())
	aliasEnrichScopeSanityCheck(t, core)
	ctrl := app.New(core)
	ctrl.SetUIMode("web")

	_, tasks := ctrl.Handle(messages.ResourcesLoaded{
		ResourceType: aliasEnrichScopeAlias,
		Resources:    []resource.Resource{{ID: "bucket-1"}, {ID: "bucket-2"}},
		Gen:          0,
	})

	var found *runtime.TaskRequest
	for i := range tasks {
		if tasks[i].Key.Kind == runtime.TaskKindProbeEnrich {
			found = &tasks[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("ctrl.Handle(messages.ResourcesLoaded, alias %q) returned %d tasks, want a TaskKindProbeEnrich task", aliasEnrichScopeAlias, len(tasks))
	}
	if found.Key.Scope != aliasEnrichScopeCanonical {
		t.Errorf("TaskKindProbeEnrich Scope = %q, want canonical %q — headless/web alias-opened list dispatched a dead alias scope", found.Key.Scope, aliasEnrichScopeCanonical)
	}
}
