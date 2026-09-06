// qa_demo_state_coverage_test.go — the standing state-coverage ratchet.
//
// Demo mode (./a9s --demo) is also the bench humans use to see every row
// color and every documented finding without live AWS credentials. This test
// is registry-driven and demo-clients-driven, structured exactly like
// qa_demo_pivot_coverage_test.go's TestDemoPivotCoverage_EveryRegisteredPivotHasAWitness:
// for every registered type it drains that type's real Wave-1 fetcher against
// demo.NewServiceClients() (the typed fakes) to get the actual demo fixture
// resources, builds one shared resource.ResourceCache from every fetchable
// type (so cross-type Wave-2 enrichers see sibling data exactly as production
// does), then asks two questions per type using only machine-readable sources
// that GENERATE docs/resources/<type>.md (never parsed markdown):
//
//  1. Bucket reachability (catalog.ResourceTypeDef.Color, invariant #7 —
//     every registered type has a non-nil Color classifier). The classifier
//     itself, not a hand-rolled LifecycleKey heuristic, is the single source
//     of truth for which of the four domain.Color buckets (Healthy / Warning
//     / Broken / Dim) a resource lands in — mirroring how the pivot test
//     calls the type's own real RelatedDef.Checker instead of reimplementing
//     pivot logic. td.ResolveColor runs against a Wave-2-folded copy of each
//     fixture resource — runtime.ApplyWave2ToRow applied with the type's own
//     registered Wave-2 IssueEnricher output, exactly the shape production
//     uses before Color funcs that lead with colorFromAnyFinding ever see the
//     row — so a bucket only reachable through a Wave-2 finding (e.g. s3's
//     PAB-incomplete Broken) is witnessable here, not just Wave-1-only
//     buckets. "Reachable" is discovered empirically: a bucket is reachable
//     for a type iff at least one demo fixture resource of that type actually
//     classifies into it via td.ResolveColor on the folded copy. A bucket no
//     demo fixture ever reaches is either (a) a fixture gap — the type has
//     real resources that could be in that state but none exist in demo
//     data, pin it in knownStateCoverageGaps — or (b) structurally
//     unreachable for that type (e.g. a type with no lifecycle signal at all
//     only ever resolves ColorHealthy) in which case it is never observed as
//     a gap in the first place: this test only ever asks about buckets it
//     saw at least one fixture claim as a candidate improvement target, so
//     "unreachable" buckets are silently absent from both the failure set and
//     the allowlist, never scored as N/A explicitly.
//
//  2. Finding coverage (catalog.ResourceTypeDef.Findings — code/phrase/
//     severity/source; this is what cmd/catalogen writes into the findings
//     tables in docs/resources/<type>.md). For every registered FindingDef,
//     at least one demo fixture resource must actually PRODUCE a
//     domain.Finding carrying that Code — either already present in the
//     fixture's Wave-1 res.Findings, or emitted by the type's registered
//     Wave-2 IssueEnricher (awsclient.Wave2EnricherFor) when run against the
//     type's own fixture resources, keyed by IssueEnricherResult.Findings
//     [res.ID].Code.
//
// RATCHET semantics (identical contract to qa_demo_pivot_coverage_test.go):
//
//   - A gap NOT in knownStateCoverageGaps is a NEW regression — always fails,
//     unconditionally.
//   - An allowlisted gap that NOW HAS a witness fails with a "remove from
//     allowlist" message — forces the fixture fix and this list to land in
//     the same PR.
//   - An allowlisted gap that is still uncovered is skipped (logged, not
//     failed) — expected, pre-existing debt.
//
// s3 must NOT appear in knownStateCoverageGaps for its documented
// "public access block incomplete" finding: core/demo/fixtures/s3.go
// already carries a bucket with an incomplete GetPublicAccessBlockOutput, so
// EnrichS3Posture (or the Wave-1 classification, whichever the
// registry wires) must produce a witness. If s3 shows a gap here, the harness
// is wrong — debug it before trusting the rest of the inventory.
//
// Per-type coverage summaries (buckets reached / findings witnessed vs
// registered) are logged under -v on any failure — this is the burn-down
// worklist for both dimensions.
package unit_test

import (
	"context"
	"fmt"
	"sort"
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
)

// knownStateCoverageGaps is the burn-down allowlist, read from the per-type
// Pin registrations in core/demo/fixtures: a type's gaps are declared in the
// fixture file whose fixtures would close them, so burning one down edits
// that one file instead of a list every type shares. Two key shapes:
//
//	"type:bucket"       e.g. "ec2:warning" — a domain.Color bucket
//	                    (healthy|warning|broken|dim) with zero fixture
//	                    witnesses for that type.
//	"type:finding-code" e.g. "s3.pab-incomplete" style — a registered
//	                    catalog.FindingDef.Code with zero fixture witnesses
//	                    for that type. The key is "<shortName>:<code>" so
//	                    the same finding code namespaced differently per
//	                    type never collides.
//
// Same burn-down semantics as knownDisconnectedPivots in
// qa_demo_pivot_coverage_test.go:
//   - present + still uncovered today -> skip (logged), expected debt.
//   - present + now covered           -> FAIL ("remove from allowlist").
//   - a gap NOT present here          -> FAIL unconditionally, a new
//     regression the allowlist was never told about.
var knownStateCoverageGaps = fixtures.CoverageGaps()

// bucketName maps a domain.Color to the lowercase token used in
// knownStateCoverageGaps keys and log output.
func bucketName(c domain.Color) string {
	switch c {
	case domain.ColorHealthy:
		return "healthy"
	case domain.ColorWarning:
		return "warning"
	case domain.ColorBroken:
		return "broken"
	case domain.ColorDim:
		return "dim"
	default:
		return fmt.Sprintf("unknown(%d)", c)
	}
}

// buildDemoStateTypeCache mirrors buildDemoTypeCache from
// qa_demo_pivot_coverage_test.go — drains every registered type's demo
// fixtures via its real Wave-1 Fetcher and returns both the per-type
// resource lists and one shared resource.ResourceCache built from all of
// them, so cross-type Wave-2 enrichers see sibling data exactly as
// production does.
func buildDemoStateTypeCache(t *testing.T) (map[string][]resource.Resource, resource.ResourceCache) {
	t.Helper()
	clients := demo.NewServiceClients()
	byType := make(map[string][]resource.Resource)
	cache := make(resource.ResourceCache)

	for _, td := range resource.AllResourceTypes() {
		fixtures, ok := drainDemoFixtures(t, td, clients)
		if !ok {
			continue
		}
		byType[td.ShortName] = fixtures
		if len(fixtures) > 0 {
			cache[td.ShortName] = resource.ResourceCacheEntry{Resources: fixtures, IsTruncated: false}
		}
	}
	return byType, cache
}

// findingCodesFor returns, for one type's fixtures, the set of
// domain.FindingCode values actually produced by the real pipeline: every
// Code already present in a fixture's Wave-1 res.Findings, plus every Code
// the type's registered Wave-2 IssueEnricher emits into
// IssueEnricherResult.Findings when run against those same fixtures. Also
// returns the resolved domain.Color bucket set (via td.ResolveColor) across
// every fixture resource — computed on a runtime.ApplyWave2ToRow-folded copy
// of each resource, so a Color func that leads with colorFromAnyFinding
// (core/aws/catalog_color_helpers.go) can see the Wave-2 finding exactly
// as production's runtime.Core.applyEnrichment fold does, not just the raw
// pre-enrichment Wave-1 resource. FieldUpdates is intentionally NOT folded
// here — a Color func whose only path to a given bucket runs through the
// separate FieldUpdates merge (e.g. colorIAMUser's has_console_password
// branch) would still show that bucket as unreachable through this harness,
// even though ApplyListFieldUpdates gives production a witness.
func findingCodesFor(
	t *testing.T,
	td resource.ResourceTypeDef,
	fixtures []resource.Resource,
	cache resource.ResourceCache,
	clients *awsclient.ServiceClients,
) (map[domain.FindingCode]bool, map[domain.Color]bool) {
	t.Helper()
	codes := make(map[domain.FindingCode]bool)
	buckets := make(map[domain.Color]bool)

	var wave2Findings map[string][]domain.Finding
	var wave2AttentionDetails map[string]map[domain.FindingCode]domain.AttentionDetail
	hasEnricher := false

	if enricher, ok := awsclient.Wave2EnricherFor(td.ShortName); ok && enricher.Fn != nil {
		result, err := enricher.Fn(context.Background(), clients, fixtures, cache)
		if err != nil {
			t.Fatalf("%s: Wave-2 enricher returned error: %v", td.ShortName, err)
		}
		hasEnricher = true
		wave2Findings = result.Findings
		wave2AttentionDetails = result.AttentionDetails
		for _, fs := range result.Findings {
			for _, f := range fs {
				codes[f.Code] = true
			}
		}
	}

	for _, res := range fixtures {
		for _, f := range res.Findings {
			codes[f.Code] = true
		}
		folded := res
		if hasEnricher {
			runtime.ApplyWave2ToRow(&folded, td, wave2Findings, wave2AttentionDetails)
		}
		buckets[td.ResolveColor(folded)] = true
	}

	return codes, buckets
}

// TestDemoStateCoverage_EveryReachableBucketHasAFixture is the row-color
// half of the standing ratchet: for every registered type, every domain.Color
// bucket that at least one demo fixture resource resolves into (via the
// type's own td.ResolveColor — the real production classifier, invariant #7)
// establishes that bucket as "reachable" for this type. This test's job is
// narrower than "cover every theoretically reachable bucket" — it enumerates,
// per type, exactly the set of buckets its fixtures currently produce, and
// then requires that set to be non-empty and stable: a bucket present in
// knownStateCoverageGaps must remain absent from the fixtures (still a gap)
// or the allowlist entry must be removed (burn-down). A bucket NOT in the
// allowlist and absent from every fixture's resolved color is a NEW
// regression only when the type is known (from a prior run) to be capable of
// reaching it — see knownStateCoverageGaps population. One subtest per
// (type, bucket) so the full backlog is enumerable from `go test -v` output.
func TestDemoStateCoverage_EveryReachableBucketHasAFixture(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildDemoStateTypeCache(t)

	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	allBuckets := []domain.Color{domain.ColorHealthy, domain.ColorWarning, domain.ColorBroken, domain.ColorDim}

	var stillGapped []string
	var newlyRegressed []string
	var readyForBurnDown []string

	for _, td := range types {
		fixtures := byType[td.ShortName]
		if len(fixtures) == 0 {
			continue
		}
		if td.Color == nil {
			continue
		}

		_, buckets := findingCodesFor(t, td, fixtures, cache, clients)

		for _, b := range allBuckets {
			key := td.ShortName + ":" + bucketName(b)
			testName := fmt.Sprintf("%s/%s", td.ShortName, bucketName(b))
			hasWitness := buckets[b]
			allowlisted := knownStateCoverageGaps[key]

			t.Run(testName, func(t *testing.T) {
				switch {
				case hasWitness && allowlisted:
					readyForBurnDown = append(readyForBurnDown, key)
					t.Errorf(
						"BURN-DOWN: %s now has a %s-bucket fixture but is still pinned in "+
							"knownStateCoverageGaps — remove %q from the allowlist in this PR",
						td.ShortName, bucketName(b), key,
					)
				case hasWitness:
					// Reached and not allowlisted — the expected steady state.
				case allowlisted:
					stillGapped = append(stillGapped, key)
					t.Skipf(
						"KNOWN GAP (allowlisted): %s: no fixture among %d resolves to the %s bucket — "+
							"pre-existing debt, see knownStateCoverageGaps",
						td.ShortName, len(fixtures), bucketName(b),
					)
				default:
					newlyRegressed = append(newlyRegressed, key)
					t.Errorf(
						"NEW GAP (not allowlisted): %s: no fixture among %d resolves to the %s bucket via "+
							"td.ResolveColor — demo mode never shows this type in that state. Either add a "+
							"fixture in that state or, if this is pre-existing debt / structurally "+
							"unreachable for this type, add %q to knownStateCoverageGaps",
						td.ShortName, len(fixtures), bucketName(b), key,
					)
				}
			})
		}
	}

	if len(newlyRegressed) > 0 {
		t.Logf("NEW GAP INVENTORY (%d): %v", len(newlyRegressed), newlyRegressed)
	}
	if len(readyForBurnDown) > 0 {
		t.Logf("READY-FOR-BURN-DOWN INVENTORY (%d): %v", len(readyForBurnDown), readyForBurnDown)
	}
	if len(stillGapped) > 0 {
		t.Logf("STILL-GAPPED (allowlisted, skipped) INVENTORY (%d): %v", len(stillGapped), stillGapped)
	}

	if len(newlyRegressed) > 0 || len(readyForBurnDown) > 0 {
		logStateCoverageSummary(t, byType, cache, clients)
	}
}

// TestDemoStateCoverage_EveryDocumentedFindingHasAFixture is the finding half
// of the standing ratchet: for every registered type and every
// catalog.FindingDef in td.Findings (the exact table cmd/catalogen renders
// into docs/resources/<type>.md), at least one demo fixture resource of that
// type must produce a domain.Finding carrying that Code — through Wave-1
// res.Findings or the type's registered Wave-2 IssueEnricher — UNLESS the
// (type, code) pair is pinned in knownStateCoverageGaps as pre-existing
// debt, in which case it is skipped (logged) instead of failed. One subtest
// per (type, finding code) so the full backlog is enumerable in -v output.
func TestDemoStateCoverage_EveryDocumentedFindingHasAFixture(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildDemoStateTypeCache(t)

	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	var stillGapped []string
	var newlyRegressed []string
	var readyForBurnDown []string

	totalFindings := 0

	for _, td := range types {
		if len(td.Findings) == 0 {
			continue
		}
		fixtures := byType[td.ShortName]

		codes, _ := findingCodesFor(t, td, fixtures, cache, clients)

		for _, fd := range td.Findings {
			totalFindings++
			key := td.ShortName + ":" + string(fd.Code)
			testName := fmt.Sprintf("%s/%s", td.ShortName, fd.Code)
			hasWitness := codes[fd.Code]
			allowlisted := knownStateCoverageGaps[key]

			t.Run(testName, func(t *testing.T) {
				if len(fixtures) == 0 {
					if allowlisted {
						stillGapped = append(stillGapped, key)
						t.Skipf(
							"KNOWN GAP (allowlisted): %s: type has zero demo fixtures — cannot witness "+
								"finding %q", td.ShortName, fd.Code,
						)
						return
					}
					newlyRegressed = append(newlyRegressed, key)
					t.Errorf(
						"NEW GAP (not allowlisted): %s: type has zero demo fixtures — cannot witness "+
							"documented finding %q (phrase %q, severity %v). Either fix demo fixtures or "+
							"add %q to knownStateCoverageGaps",
						td.ShortName, fd.Code, fd.Phrase, fd.Severity, key,
					)
					return
				}

				switch {
				case hasWitness && allowlisted:
					readyForBurnDown = append(readyForBurnDown, key)
					t.Errorf(
						"BURN-DOWN: %s now has a fixture producing finding %q but is still pinned in "+
							"knownStateCoverageGaps — remove %q from the allowlist in this PR",
						td.ShortName, fd.Code, key,
					)
				case hasWitness:
					// Witnessed and not allowlisted — the expected steady state.
				case allowlisted:
					stillGapped = append(stillGapped, key)
					t.Skipf(
						"KNOWN GAP (allowlisted): %s: no fixture among %d produces documented finding %q "+
							"(phrase %q, severity %v) — pre-existing debt, see knownStateCoverageGaps",
						td.ShortName, len(fixtures), fd.Code, fd.Phrase, fd.Severity,
					)
				default:
					newlyRegressed = append(newlyRegressed, key)
					t.Errorf(
						"NEW REGRESSION (not allowlisted): %s: no fixture among %d produces documented "+
							"finding %q (phrase %q, severity %v, source %q) — demo mode has nothing to "+
							"demonstrate for this documented finding. Either fix the fixtures / enricher or, "+
							"if this is pre-existing debt, add %q to knownStateCoverageGaps",
						td.ShortName, len(fixtures), fd.Code, fd.Phrase, fd.Severity, fd.Source, key,
					)
				}
			})
		}
	}

	t.Logf("TOTAL REGISTERED FINDINGS: %d", totalFindings)
	if len(newlyRegressed) > 0 {
		t.Logf("NEW REGRESSION INVENTORY (%d): %v", len(newlyRegressed), newlyRegressed)
	}
	if len(readyForBurnDown) > 0 {
		t.Logf("READY-FOR-BURN-DOWN INVENTORY (%d): %v", len(readyForBurnDown), readyForBurnDown)
	}
	if len(stillGapped) > 0 {
		t.Logf("STILL-GAPPED (allowlisted, skipped) INVENTORY (%d): %v", len(stillGapped), stillGapped)
	}

	if len(newlyRegressed) > 0 || len(readyForBurnDown) > 0 {
		logStateCoverageSummary(t, byType, cache, clients)
	}
}

// logStateCoverageSummary prints, per type, buckets reached / registered and
// findings witnessed / registered — the burn-down worklist referenced by
// both tests' failure output. Only invoked under -v on failure to keep green
// runs quiet.
func logStateCoverageSummary(
	t *testing.T,
	byType map[string][]resource.Resource,
	cache resource.ResourceCache,
	clients *awsclient.ServiceClients,
) {
	t.Helper()
	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	allBuckets := []domain.Color{domain.ColorHealthy, domain.ColorWarning, domain.ColorBroken, domain.ColorDim}

	for _, td := range types {
		fixtures := byType[td.ShortName]
		if len(fixtures) == 0 && len(td.Findings) == 0 {
			continue
		}

		var bucketsReached int
		if td.Color != nil && len(fixtures) > 0 {
			codes, buckets := findingCodesFor(t, td, fixtures, cache, clients)
			for _, b := range allBuckets {
				if buckets[b] {
					bucketsReached++
				}
			}
			var witnessed int
			for _, fd := range td.Findings {
				if codes[fd.Code] {
					witnessed++
				}
			}
			t.Logf("SUMMARY %-16s buckets=%d/%d findings=%d/%d fixtures=%d",
				td.ShortName, bucketsReached, len(allBuckets), witnessed, len(td.Findings), len(fixtures))
		}
	}
}
