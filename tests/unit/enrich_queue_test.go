package unit

// enrich_queue_test.go — Regression tests for ARCH-04: declarative enricher
// priority metadata.
//
// After ARCH-04, EnricherRegistry changes from map[string]EnricherFunc to
// map[string]Enricher{Fn EnricherFunc; Priority int}. The seven "batchable"
// types (dbi, ebs, cb, tg, pipeline, sfn, glue) get Priority=10; all remaining
// types get Priority=100. buildEnrichQueue sorts by Priority (asc) then
// alphabetically within each tier.
//
// TDD contract: tests are RED until ARCH-04 coder task finishes.  The coder
// changes the EnricherRegistry map-value type; tests below verify the resulting
// ordering behaviour.
//
// Seeding strategy: AvailabilityPrefetchedMsg.Resources populates all
// probeResources in one shot and triggers startEnrichment, avoiding the
// per-message availChecked>=availTotal race that AvailabilityCheckedMsg would
// cause if used to seed many types one at a time.

import (
	"maps"
	"sort"
	"testing"

	tea "charm.land/bubbletea/v2"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/tui"
	"github.com/k2m30/a9s/v3/internal/tui/messages"
)

// priority10Types lists the short names that receive Priority=10 after ARCH-04.
// These are the "batchable" enrichers scheduled first in the dispatch queue.
var priority10Types = []string{"dbi", "ebs", "cb", "tg", "pipeline", "sfn", "glue"}

// enrichmentTypesFromMsgs extracts ResourceType strings in order for diagnostics.
func enrichmentTypesFromMsgs(msgs []messages.EnrichmentCheckedMsg) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = m.ResourceType
	}
	return out
}

// seedAllEnricherTypes builds an AvailabilityPrefetchedMsg.Resources map containing
// one fake resource for every key in EnricherRegistry, then delivers it to the
// model.  Returns the cmd that triggers startEnrichment.
func seedAllEnricherTypes(m tui.Model) (tui.Model, tea.Cmd) {
	allResources := make(map[string][]resource.Resource, len(awsclient.EnricherRegistry))
	for shortName := range awsclient.EnricherRegistry {
		allResources[shortName] = []resource.Resource{
			{ID: shortName + "-probe", Name: shortName + "-probe", Fields: map[string]string{}},
		}
	}
	m, cmd := rootApplyMsg(m, messages.AvailabilityPrefetchedMsg{
		Entries:   make(map[string]int),
		Resources: allResources,
	})
	return m, cmd
}

// seedEnricherSubset delivers an AvailabilityPrefetchedMsg seeding only the
// provided shortNames and returns the resulting model and cmd.
func seedEnricherSubset(m tui.Model, names []string) (tui.Model, tea.Cmd) {
	subset := make(map[string][]resource.Resource, len(names))
	for _, name := range names {
		subset[name] = []resource.Resource{
			{ID: name + "-probe", Name: name + "-probe", Fields: map[string]string{}},
		}
	}
	m, cmd := rootApplyMsg(m, messages.AvailabilityPrefetchedMsg{
		Entries:   make(map[string]int),
		Resources: subset,
	})
	return m, cmd
}

// ─────────────────────────────────────────────────────────────────────────────
// TestBuildEnrichQueue_OrdersByMetadataPriority
// ─────────────────────────────────────────────────────────────────────────────

// TestBuildEnrichQueue_OrdersByMetadataPriority verifies that all Priority=10
// types come before all Priority=100 types in the buildEnrichQueue output.
//
// We seed probeResources for all registered enricher types via a single
// AvailabilityPrefetchedMsg (so all types are available when startEnrichment
// fires), execute the resulting cmd tree, and assert no priority-10 type
// appears after any priority-100 type in the collected EnrichmentCheckedMsg
// sequence.
//
// GREEN: buildEnrichQueue already reads EnricherPriority correctly.
// Any regression in the priority sort order will flip this to RED.
func TestBuildEnrichQueue_OrdersByMetadataPriority(t *testing.T) {
	tui.Version = "test"

	m := newRootSizedModel()
	_, enrichCmd := seedAllEnricherTypes(m)

	if enrichCmd == nil {
		t.Skip("no cmd returned — cannot verify queue order without enrichment dispatch")
	}

	found := collectEnrichmentMsgs(enrichCmd)
	if len(found) == 0 {
		t.Skip("no EnrichmentCheckedMsg in cmd tree — skipping order check")
	}

	p10Set := make(map[string]bool, len(priority10Types))
	for _, name := range priority10Types {
		p10Set[name] = true
	}

	// Walk found messages in order.  Once we see a Priority-100 type,
	// no Priority-10 type must follow it.
	seenLowPriority := false
	for _, msg := range found {
		isPriority10 := p10Set[msg.ResourceType]
		if !isPriority10 {
			seenLowPriority = true
		}
		if seenLowPriority && isPriority10 {
			t.Errorf("priority ordering violation: %q (priority-10) appeared after a priority-100 type; "+
				"full dispatch order: %v", msg.ResourceType, enrichmentTypesFromMsgs(found))
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestBuildEnrichQueue_StableAlphabeticalWithinPriority
// ─────────────────────────────────────────────────────────────────────────────

// TestBuildEnrichQueue_StableAlphabeticalWithinPriority verifies alphabetical
// ordering within the priority-10 tier by seeding exactly those seven types.
//
// Expected dispatch order (sorted): cb, dbi, ebs, glue, pipeline, sfn, tg.
//
// GREEN: sort.Slice in buildEnrichQueue already sorts by Priority then Name.
// Any change that breaks the alphabetical-within-tier guarantee fails this.
func TestBuildEnrichQueue_StableAlphabeticalWithinPriority(t *testing.T) {
	tui.Version = "test"

	// Snapshot and restore the full registry around this test.
	snapshot := make(map[string]awsclient.Enricher, len(awsclient.EnricherRegistry))
	maps.Copy(snapshot, awsclient.EnricherRegistry)
	t.Cleanup(func() {
		for k := range awsclient.EnricherRegistry {
			delete(awsclient.EnricherRegistry, k)
		}
		maps.Copy(awsclient.EnricherRegistry, snapshot)
	})

	// Replace registry with only the seven priority-10 types.
	for k := range awsclient.EnricherRegistry {
		delete(awsclient.EnricherRegistry, k)
	}
	for _, name := range priority10Types {
		awsclient.EnricherRegistry[name] = awsclient.Enricher{
			Fn:       awsclient.NoOpEnricher,
			Priority: 10,
		}
	}

	m := newRootSizedModel()
	_, enrichCmd := seedEnricherSubset(m, priority10Types)

	if enrichCmd == nil {
		t.Skip("no cmd returned — cannot verify alphabetical order")
	}

	found := collectEnrichmentMsgs(enrichCmd)
	if len(found) == 0 {
		t.Skip("no EnrichmentCheckedMsg in cmd tree — skipping alphabetical check")
	}

	got := make([]string, 0, len(found))
	for _, msg := range found {
		got = append(got, msg.ResourceType)
	}

	// Assert got is non-decreasing (alphabetical).
	for i := 1; i < len(got); i++ {
		if got[i] < got[i-1] {
			t.Errorf("alphabetical order violated at index %d: %q appears before %q; full order: %v",
				i, got[i-1], got[i], got)
		}
	}

	// Also verify all priority-10 types were dispatched.
	expected := make([]string, len(priority10Types))
	copy(expected, priority10Types)
	sort.Strings(expected)

	dispatched := make(map[string]bool, len(got))
	for _, name := range got {
		dispatched[name] = true
	}
	for _, name := range expected {
		if !dispatched[name] {
			t.Errorf("priority-10 type %q was registered and seeded but not dispatched; got: %v", name, got)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestBuildEnrichQueue_IncludesAllRegisteredEnrichers
// ─────────────────────────────────────────────────────────────────────────────

// TestBuildEnrichQueue_IncludesAllRegisteredEnrichers verifies that every key in
// EnricherRegistry that has a probeResources entry appears in the dispatch queue.
//
// After ARCH-04, the registry values carry an Enricher struct with Fn and Priority.
// probeEnrichment reads Fn to invoke the enricher; a zero-value Enricher (Fn=nil)
// silently skips. This test ensures every registered key is dispatched when seeded,
// catching any partial migration that leaves a zero-value entry.
//
// GREEN: current buildEnrichQueue iterates EnricherRegistry correctly.
// RED if any registry entry is silently dropped (e.g., after struct migration).
func TestBuildEnrichQueue_IncludesAllRegisteredEnrichers(t *testing.T) {
	tui.Version = "test"

	m := newRootSizedModel()
	_, enrichCmd := seedAllEnricherTypes(m)

	if enrichCmd == nil {
		t.Skip("no cmd returned — cannot verify queue completeness")
	}

	found := collectEnrichmentMsgs(enrichCmd)
	dispatched := make(map[string]bool, len(found))
	for _, msg := range found {
		dispatched[msg.ResourceType] = true
	}

	for shortName := range awsclient.EnricherRegistry {
		if !dispatched[shortName] {
			t.Errorf("enricher %q is registered with a probeResources entry but was NOT dispatched; "+
				"check buildEnrichQueue handles all EnricherRegistry keys", shortName)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestBuildEnrichQueue_SkipsTypesWithoutProbe
// ─────────────────────────────────────────────────────────────────────────────

// TestBuildEnrichQueue_SkipsTypesWithoutProbe verifies that a type registered in
// EnricherRegistry without a probeResources entry is excluded from the queue.
//
// This test registers a test-only enricher, seeds probeResources for "dbi" only
// (via AvailabilityPrefetchedMsg), and asserts no EnrichmentCheckedMsg is produced
// for the test-only type.
func TestBuildEnrichQueue_SkipsTypesWithoutProbe(t *testing.T) {
	tui.Version = "test"

	const probeSkipType = "test-no-probe-skip"

	// Register a no-op Enricher for the test type.
	// This assignment uses the new Enricher struct — TDD-red until ARCH-04 completes
	// the registry type migration from map[string]EnricherFunc to map[string]Enricher.
	awsclient.EnricherRegistry[probeSkipType] = awsclient.Enricher{
		Fn:       awsclient.NoOpEnricher,
		Priority: 100,
	}
	t.Cleanup(func() { delete(awsclient.EnricherRegistry, probeSkipType) })

	m := newRootSizedModel()

	// Seed only "dbi" — deliberately omitting probeSkipType.
	_, enrichCmd := seedEnricherSubset(m, []string{"dbi"})

	if enrichCmd == nil {
		t.Skip("no cmd returned — cannot verify skip behavior")
	}

	found := collectEnrichmentMsgs(enrichCmd)
	for _, msg := range found {
		if msg.ResourceType == probeSkipType {
			t.Errorf("type %q has no probeResources entry but buildEnrichQueue dispatched it", probeSkipType)
		}
	}
}
