// runtime_alias_enrich_scope_test.go pins the list-open Wave-2 dispatch's
// scope canonicalization: an alias-opened list must still dispatch its
// ProbeEnrich task under the CANONICAL short name, not the raw alias the
// event carried.
//
// "buckets" is a registered alias for s3, and s3 has a Wave-2 issue enricher,
// so c.HasIssueEnricher("buckets") resolves true through the alias lookup.
// The list-open branch is also gated on ev.Provenance.CanonicalList(), so
// every event literal driving it sets FetchProvenanceCanonicalList.
package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
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

// TestHandleResourcesLoaded_AliasOpenedList_ProbeEnrichScopedToCanonical: a
// list-open event (TypeGen=0, Append=false, Err=nil) carrying the alias
// returns a ProbeEnrich task scoped to "s3", not "buckets".
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
		Provenance:   messages.FetchProvenanceCanonicalList,
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

// TestHandleResourcesLoaded_AliasRerun_ProbeEnrichScopedToCanonical: the
// enrichment-rerun branch (TypeGen matching session.EnrichmentTypeGen) scopes
// its ProbeEnrich task to the canonical short name when the rerun event
// carries the alias. session.EnrichmentTypeGen is keyed by the canonical
// ShortName, so the gen-guard map is seeded under "s3" while the event
// carries "buckets".
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

// TestControllerHandle_AliasOpenedList_ProbeEnrichScopedToCanonical: a
// headless Controller in web UI mode receiving a messages.ResourcesLoaded
// with an alias ResourceType through Controller.Handle returns a
// TaskKindProbeEnrich task scoped to the canonical short name.
func TestControllerHandle_AliasOpenedList_ProbeEnrichScopedToCanonical(t *testing.T) {
	sess := session.New()
	core := runtime.New(sess, resource.AllResourceTypes())
	aliasEnrichScopeSanityCheck(t, core)
	ctrl := newBlessedController(t, core)
	ctrl.SetUIMode("web")

	_, tasks := handlePage(ctrl, messages.ResourcesLoaded{Provenance: messages.FetchProvenanceCanonicalList,
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
