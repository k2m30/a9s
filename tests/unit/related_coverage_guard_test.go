package unit_test

import (
	"context"
	"fmt"
	"sort"
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// Every registered pivot, run over every demo row, states its coverage, and
// the only result that may be read as a proven zero is a complete one. The
// bench must also show each kind of answer the panel distinguishes, or the
// rule could hold only because nothing tested it.
func TestRelatedCoverage_ClassGuardOnDemoBench(t *testing.T) {
	b := newRefBench(t)
	clients := refClients()
	known := map[resource.RelatedCoverage]bool{
		resource.CoverageComplete:  true,
		resource.CoveragePartial:   true,
		resource.CoverageNoPath:    true,
		resource.CoverageHeuristic: true,
	}

	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	var completeZero, completeMatch, partial, heuristic int
	for _, td := range types {
		for _, def := range resource.GetRelated(td.ShortName) {
			var bad []string
			for _, row := range b.byType[td.ShortName] {
				r := def.Checker(context.Background(), clients, row, b.cache)
				cov := r.Coverage()
				resolvedZero := r.State() == domain.RelatedResolved && r.Count() == 0 && !r.Truncated()
				switch {
				case !known[cov]:
					bad = append(bad, fmt.Sprintf("%s: coverage %v is none of complete, partial, no-path, heuristic", row.ID, cov))
				case resolvedZero && cov == resource.CoveragePartial:
					bad = append(bad, row.ID+": an exact zero labelled partial")
				case cov == resource.CoverageComplete && r.Count() == 0 && !resolvedZero:
					bad = append(bad, fmt.Sprintf("%s: complete coverage with no match but state %v, truncated %v", row.ID, r.State(), r.Truncated()))
				case cov == resource.CoverageComplete && resolvedZero:
					completeZero++
				case cov == resource.CoverageComplete && r.Count() > 0:
					completeMatch++
				case cov == resource.CoveragePartial:
					partial++
				case cov == resource.CoverageHeuristic:
					heuristic++
				}
			}
			if len(bad) > 0 {
				t.Errorf("%s → %s: %d rows violate coverage, first %s", td.ShortName, def.TargetType, len(bad), bad[0])
			}
		}
	}
	if completeZero == 0 || completeMatch == 0 || partial == 0 || heuristic == 0 {
		t.Errorf("demo bench shows complete zeros %d, complete matches %d, partial %d, heuristic %d; each must be at least 1",
			completeZero, completeMatch, partial, heuristic)
	}
}
