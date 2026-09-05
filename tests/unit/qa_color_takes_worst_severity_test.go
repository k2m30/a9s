package unit_test

// qa_color_takes_worst_severity_test.go — ruling M, widened past the five
// networking types.
//
// One selector now decides the row colour for every type that classifies from
// findings. The networking sweep could not catch a type that switched selector
// without a fixture mixing severities on one row, and most types have no such
// fixture, so the row is constructed instead: warn first, broken second. A
// classifier that takes the head of the slice renders yellow while the row
// carries a broken finding.

import (
	"fmt"
	"sort"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// rawFieldClassifiers never consult r.Findings at all: they classify from
// Fields alone. They predate this batch and did not change in it, so they are
// excluded here rather than silently failing a rule they were never held to.
// Each is a standing violation of the catalog contract's "colour derives from
// findings" rule, tracked outside this batch.
var rawFieldClassifiers = map[string]bool{
	"rtb": true, "alarm": true, "trail": true, "ct-events": true,
	"sns-sub": true, "ses": true, "ssm": true,
}

// TestColorTakesWorstSeverity_EveryType constructs a warn-then-broken row for
// every finding-driven type and asserts the colour comes from the broken
// finding, wherever it sits in the slice.
func TestColorTakesWorstSeverity_EveryType(t *testing.T) {
	var bad []string
	for _, td := range resource.AllResourceTypes() {
		if rawFieldClassifiers[td.ShortName] {
			continue
		}
		row := domain.Resource{
			ID:   "probe-" + td.ShortName,
			Name: "probe-" + td.ShortName,
			Findings: []domain.Finding{
				{Code: domain.FindingCode(td.ShortName + ".probe.warn"), Phrase: "warn probe", Severity: domain.SevWarn, Source: "wave1"},
				{Code: domain.FindingCode(td.ShortName + ".probe.broken"), Phrase: "broken probe", Severity: domain.SevBroken, Source: "wave1"},
			},
		}
		if got := td.ResolveColor(row); got != domain.ColorBroken {
			bad = append(bad, fmt.Sprintf("%s = %v", td.ShortName, got))
		}
	}
	if len(bad) == 0 {
		return
	}
	sort.Strings(bad)
	t.Errorf("%d type(s) colour from the first finding rather than the worst: %v", len(bad), bad)
}

// TestColorTakesWorstSeverity_DemoBench is the same rule over the rows that
// actually ship, so a fixture that starts mixing severities is covered too.
func TestColorTakesWorstSeverity_DemoBench(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()

	var bad []string
	for _, td := range resource.AllResourceTypes() {
		if rawFieldClassifiers[td.ShortName] {
			continue
		}
		fixtures := byType[td.ShortName]
		if len(fixtures) == 0 {
			continue
		}
		for _, res := range mergeWave2Findings(t, td, fixtures, cache, clients) {
			worst, ok := netWorstSeverity(res)
			if !ok {
				continue
			}
			if got, want := td.ResolveColor(res), netColorForSeverity(worst); got != want {
				bad = append(bad, fmt.Sprintf("%s/%s: %v, want %v", td.ShortName, res.ID, got, want))
			}
		}
	}
	if len(bad) == 0 {
		return
	}
	sort.Strings(bad)
	t.Errorf("%d demo row(s) colour from something other than the worst finding: %v", len(bad), bad)
}
