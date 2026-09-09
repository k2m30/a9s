package unit

// d1_color_table_test.go — the shared oracle for the per-type status→colour
// tables.
//
// Colour derives from findings, so the mapping those tables pin runs status →
// the type's findings predicate → severity → colour. The input is an SDK
// struct fed through the type's fetcher, and the expected colour is computed
// here from the severity rather than asked of the classifier under test.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// d1ColorOf is the expected-side oracle: the colour implied by the worst
// severity among a row's findings, computed in the test. A row with no finding
// is healthy — that is the contract, not a fallback.
func d1ColorOf(findings []domain.Finding) resource.Color {
	if len(findings) == 0 {
		return resource.ColorHealthy
	}
	worst := findings[0].Severity
	for _, f := range findings[1:] {
		if f.Severity > worst {
			worst = f.Severity
		}
	}
	return resource.ColorFromSeverity(worst)
}

// d1AssertColor compares the colour a fetched row's findings imply against the
// colour the table expects, naming the findings when they disagree.
func d1AssertColor(t *testing.T, r resource.Resource, want resource.Color) {
	t.Helper()
	if got := d1ColorOf(r.Findings); got != want {
		t.Errorf("colour from findings = %v, want %v; findings = %+v", got, want, r.Findings)
	}
}
