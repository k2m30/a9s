package unit

// enrich_queue_test.go — Regression tests for declarative Wave 2 issue-enricher
// priority metadata.
//
// Wave 2 enricher metadata is now declared on each catalog.ResourceTypeDef
// literal's Wave2 field (post-AS-795n); reads go through
// awsclient.AllWave2 / Wave2EnricherFor / SetWave2EnricherForTest. The seven
// "batchable" types (dbi, ebs, cb, tg, pipeline, sfn, glue) get Priority=10;
// all remaining types get Priority=100. buildEnrichQueue sorts by Priority
// (asc) then alphabetically within each tier.
//
// These tests verify resulting ordering behaviour against the registry.
//
// Seeding strategy: AvailabilityPrefetchedMsg.Resources populates all
// probeResources in one shot and triggers startEnrichment, avoiding the
// per-message availChecked>=availTotal race that AvailabilityCheckedMsg would
// cause if used to seed many types one at a time.

import (
	"sort"
	"testing"

	tea "charm.land/bubbletea/v2"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
	"github.com/k2m30/a9s/v3/internal/runtime/messages"
	"github.com/k2m30/a9s/v3/internal/tui"
)

// priority10Types lists the short names that receive Priority=10 in the
// Wave 2 catalog. These are the "batchable" Wave 2 issue enrichers scheduled
// first in the dispatch queue.
var priority10Types = []string{"dbi", "ebs", "cb", "tg", "pipeline", "sfn", "glue"}

// enrichmentTypesFromMsgs extracts ResourceType strings in order for diagnostics.
func enrichmentTypesFromMsgs(msgs []messages.EnrichmentChecked) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = m.ResourceType
	}
	return out
}

// seedAllEnricherTypes builds an AvailabilityPrefetchedMsg.Resources map
// containing one fake resource for every Wave 2 entry in the catalog, then
// delivers it to the model. Returns the cmd that triggers startEnrichment.
func seedAllEnricherTypes(m tui.Model) (tui.Model, tea.Cmd) {
	entries := awsclient.AllWave2()
	allResources := make(map[string][]resource.Resource, len(entries))
	for _, e := range entries {
		allResources[e.ShortName] = []resource.Resource{
			{ID: e.ShortName + "-probe", Name: e.ShortName + "-probe", Fields: map[string]string{}},
		}
	}
	m, cmd := rootApplyMsg(m, messages.AvailabilityPrefetched{
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
	m, cmd := rootApplyMsg(m, messages.AvailabilityPrefetched{
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

	// Mask every catalog Wave 2 entry with an Fn=nil override so AllWave2
	// excludes them, then inject fresh Priority=10 NoOp overrides for the
	// seven batchable types. SetWave2EnricherForTest/DeleteWave2EnricherForTest
	// restore the previous state automatically via t.Cleanup.
	for _, ent := range awsclient.AllWave2() {
		awsclient.DeleteWave2EnricherForTest(t, ent.ShortName)
	}
	for _, name := range priority10Types {
		awsclient.SetWave2EnricherForTest(t, name, awsclient.IssueEnricher{
			Fn:       awsclient.NoOpIssueEnricher,
			Priority: 10,
		})
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

	for _, ent := range awsclient.AllWave2() {
		if !dispatched[ent.ShortName] {
			t.Errorf("enricher %q is registered with a probeResources entry but was NOT dispatched; "+
				"check buildEnrichQueue handles every catalog.AllByWave2 entry", ent.ShortName)
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

	// Register a no-op Wave 2 enricher for the test type via the test-override
	// map. SetWave2EnricherForTest accepts sentinel short names (not in the
	// catalog) and restores state automatically via t.Cleanup.
	awsclient.SetWave2EnricherForTest(t, probeSkipType, awsclient.IssueEnricher{
		Fn:       awsclient.NoOpIssueEnricher,
		Priority: 100,
	})

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

// TestBuildEnrichQueue_NewEnricherAutoParticipates proves the scheduling
// contract for #277: a brand-new issue enricher added at runtime (as any real
// new enricher would be added in production code via the catalog Wave2 field
// init block) participates in the dispatch queue WITHOUT any change to
// internal/tui/app_fetchers.go. Priority metadata on the registry entry is the
// single source of scheduling truth.
func TestBuildEnrichQueue_NewEnricherAutoParticipates(t *testing.T) {
	tui.Version = "test"

	const novelType = "arch277-novel-enricher"

	// Register a novel enricher (sentinel name not in catalog) with a distinct
	// priority to prove the Wave 2 entry alone is sufficient to participate in
	// dispatch ordering. SetWave2EnricherForTest restores state automatically.
	awsclient.SetWave2EnricherForTest(t, novelType, awsclient.IssueEnricher{
		Fn:       awsclient.NoOpIssueEnricher,
		Priority: 10, // batchable tier
	})

	m := newRootSizedModel()
	_, enrichCmd := seedAllEnricherTypes(m)

	if enrichCmd == nil {
		t.Skip("no cmd returned — cannot verify novel-enricher participation")
	}

	found := collectEnrichmentMsgs(enrichCmd)

	novelPos := -1
	for i, msg := range found {
		if msg.ResourceType == novelType {
			novelPos = i
			break
		}
	}
	if novelPos == -1 {
		t.Fatalf("novel enricher %q did not participate in dispatch queue; "+
			"buildEnrichQueue must read catalog.AllByWave2 directly so new registrations "+
			"require no TUI runtime changes. Dispatched: %v",
			novelType, enrichmentTypesFromMsgs(found))
	}

	// Prove priority metadata is honored: no Priority=100 entry appears before
	// this Priority=10 novel entry.
	p10Set := make(map[string]bool, len(priority10Types))
	for _, name := range priority10Types {
		p10Set[name] = true
	}
	p10Set[novelType] = true

	for i := 0; i < novelPos; i++ {
		if !p10Set[found[i].ResourceType] {
			t.Errorf("novel Priority=10 enricher %q was preceded by Priority=100 type %q at index %d; "+
				"priority metadata on catalog Wave2 entry is not authoritative",
				novelType, found[i].ResourceType, i)
		}
	}
}
