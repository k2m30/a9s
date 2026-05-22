// wave2.go — Wave 2 enricher accessors over the catalog struct literals.
//
// AS-795n replaced the legacy package-init IssueEnricherRegistry map with the
// catalog.ResourceTypeDef.Wave2 field. This file provides the read-side API
// (Wave2EnricherFor, AllWave2) plus a test override map so test packages can
// inject sentinel enrichers without mutating the immutable catalog.
package aws

import (
	"sort"
	"sync"

	"github.com/k2m30/a9s/v3/internal/catalog"
)

// Wave2Entry pairs a resource type ShortName with its registered Wave 2
// enricher. Returned by AllWave2 in priority/alpha order for dispatch.
type Wave2Entry struct {
	ShortName string
	Enricher  IssueEnricher
}

// testWave2Mu guards testWave2Overrides. Only touched by tests via
// SetWave2EnricherForTest; production reads are short critical sections.
var testWave2Mu sync.RWMutex //nolint:gochecknoglobals // test injection mutex

// testWave2Overrides is the test-only injection map. Populated by
// SetWave2EnricherForTest; consulted before the catalog by Wave2EnricherFor
// and AllWave2. Empty in production.
var testWave2Overrides = map[string]IssueEnricher{} //nolint:gochecknoglobals // test injection map

// Wave2EnricherFor returns the Wave 2 IssueEnricher for shortName.
//
// Lookup order:
//  1. testWave2Overrides (only populated by SetWave2EnricherForTest)
//  2. catalog.Find(shortName).Wave2 cast to IssueEnricher
//
// ok is false when neither source has a non-nil Fn for the name.
//
// Post-AS-795n: replaces direct IssueEnricherRegistry[shortName] reads.
func Wave2EnricherFor(shortName string) (IssueEnricher, bool) {
	testWave2Mu.RLock()
	override, hasOverride := testWave2Overrides[shortName]
	testWave2Mu.RUnlock()
	if hasOverride {
		if override.Fn == nil {
			return IssueEnricher{}, false
		}
		return override, true
	}
	ct := catalog.Find(shortName)
	if ct == nil || ct.Wave2 == nil {
		return IssueEnricher{}, false
	}
	e, ok := ct.Wave2.(IssueEnricher)
	if !ok || e.Fn == nil {
		return IssueEnricher{}, false
	}
	return e, true
}

// AllWave2 returns every Wave 2 enricher (from catalog + test overrides) in
// dispatch order: ascending Priority, then alphabetical ShortName within a
// priority tier. Replaces iteration over IssueEnricherRegistry after AS-795n.
//
// Test overrides win on name collision; a test override with a nil Fn deletes
// the catalog entry from the returned slice for the duration of the test.
func AllWave2() []Wave2Entry {
	seen := make(map[string]Wave2Entry)
	for _, ct := range catalog.All() {
		if ct.Wave2 == nil {
			continue
		}
		e, ok := ct.Wave2.(IssueEnricher)
		if !ok || e.Fn == nil {
			continue
		}
		seen[ct.ShortName] = Wave2Entry{ShortName: ct.ShortName, Enricher: e}
	}
	testWave2Mu.RLock()
	for name, e := range testWave2Overrides {
		if e.Fn == nil {
			delete(seen, name)
			continue
		}
		seen[name] = Wave2Entry{ShortName: name, Enricher: e}
	}
	testWave2Mu.RUnlock()

	out := make([]Wave2Entry, 0, len(seen))
	for _, w := range seen {
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Enricher.Priority != out[j].Enricher.Priority {
			return out[i].Enricher.Priority < out[j].Enricher.Priority
		}
		return out[i].ShortName < out[j].ShortName
	})
	return out
}

// wave2TestHelper is the minimum testing.TB subset required by
// SetWave2EnricherForTest. Defined as an interface so test packages need
// only pass *testing.T directly without importing the testing package here.
type wave2TestHelper interface {
	Helper()
	Cleanup(func())
}

// SetWave2EnricherForTest registers a Wave 2 enricher under shortName for the
// duration of the test. The previous entry (if any) is restored via t.Cleanup
// so callers do not need to manage save/restore manually.
//
// Test-only — production code does not call this. Callers may pass shortName
// values that are NOT in the catalog (sentinel names) to exercise dispatch
// without polluting real resource type wiring.
func SetWave2EnricherForTest(t wave2TestHelper, shortName string, e IssueEnricher) {
	t.Helper()
	testWave2Mu.Lock()
	prev, hadPrev := testWave2Overrides[shortName]
	testWave2Overrides[shortName] = e
	testWave2Mu.Unlock()
	t.Cleanup(func() {
		testWave2Mu.Lock()
		if hadPrev {
			testWave2Overrides[shortName] = prev
		} else {
			delete(testWave2Overrides, shortName)
		}
		testWave2Mu.Unlock()
	})
}

// DeleteWave2EnricherForTest removes any test override and shadows the
// catalog entry by injecting an empty (Fn=nil) override. Used by tests that
// want to assert "no enricher registered" semantics for a name that does
// have a real catalog Wave 2 entry. Restores on t.Cleanup.
func DeleteWave2EnricherForTest(t wave2TestHelper, shortName string) {
	t.Helper()
	SetWave2EnricherForTest(t, shortName, IssueEnricher{})
}
