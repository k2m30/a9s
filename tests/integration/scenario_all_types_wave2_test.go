//go:build integration

package integration

// scenario_all_types_wave2_test.go — Blanket scenario guard: for every resource
// type with a registered issue enricher, open the list in demo mode, drain the
// availability / enrichment message chain, then assert no EnrichmentCheckedMsg
// carried a non-nil error.
//
// Rationale: this is the harness-level catch-all for the "fetcher emits ID =
// bare name, enricher passes r.ID as an ARN param" bug class — and any other
// wave-2 wiring bug that real AWS would reject with ValidationError /
// InvalidArn / similar. Combined with strict demo fakes (every *Arn param
// rejects non-ARN input), every broken enricher fails this test immediately
// during `make test`, instead of shipping to production where the operator
// sees it via the `!` key.
//
// Five bugs of this exact shape shipped in 2026-04 (tg, sfn, elb, acm, msk)
// because scenarios didn't drain wave-2 errors and fakes were permissive. This
// test closes that gap without requiring one hand-written scenario per type.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestScenario_AllTypes_NoEnrichmentErrors runs the demo startup once and
// asserts that across ALL registered resource types (availability prefetch
// runs wave-2 enrichment for every type), NO EnrichmentCheckedMsg carried a
// non-nil error.
//
// One scenario, total coverage: the demo startup drives the same code path as
// `./a9s --demo` and fires enrichment for every registered type. If any
// enricher rejects its input (e.g. AWS SDK returns ValidationError because
// the fetcher emitted ID = bare name and the enricher passed r.ID as an ARN
// param), this test fails with the resource type and error message.
//
// Combined with strict demo fakes (internal/demo/fakes/*.go reject non-ARN
// input on *Arn params, mirroring real AWS validation), this test is the
// end-to-end guard that would have caught tg / sfn / elb / acm / msk before
// they shipped.
func TestScenario_AllTypes_NoEnrichmentErrors(t *testing.T) {
	_ = resource.AllShortNames // referenced for clarity; startup drives all types

	scenario := fullIntegrationNewDemoScenario(t)
	runDemoStartup(t, scenario)

	if len(scenario.enrichmentErrors) > 0 {
		keys := make([]string, 0, len(scenario.enrichmentErrors))
		for k := range scenario.enrichmentErrors {
			keys = append(keys, k)
		}
		t.Logf("enrichment errors observed: %s", strings.Join(keys, ", "))
	}
	scenario.AssertNoEnrichmentErrors()
}
