// qa_color_findings_conformance_test.go — the gate for the
// "color derives from findings" architectural invariant: for every registered
// type × demo fixture row, td.ResolveColor(merged) must equal the color
// implied by the resource's own Findings (Wave-1 seeded, Wave-2 merged) via
// the shared severity-to-color mapping in core/resource/severity_color.go
// (ColorFromSeverity / ColorFromWave1) — NOT some other structural field the
// classifier reads directly.
//
// findingsDerivedColor computes the "max severity wins" color over
// res.Findings: any SevBroken finding -> ColorBroken; else any SevWarn ->
// ColorWarning; else any SevDim -> ColorDim; no findings at all -> ColorHealthy.
// This reuses resource.ColorFromSeverity's exact severity->color table (see
// core/resource/severity_color.go) rather than reinventing the mapping;
// the only addition here is the "pick the worst finding" reduction, which
// ColorFromWave1 does NOT do (it only inspects Source=="wave1" findings, first
// match wins) — this gate deliberately looks at ALL findings regardless of
// Source, because the invariant is "color derives from findings",
// full stop, not "color derives from wave1 findings only".
//
// A mismatch means one of:
//   - the type's Color classifier reads a raw structural field (status,
//     record count, cert expiry, etc.) directly instead of emitting/reading a
//     Finding for that same signal, OR
//   - a Finding is present but its Severity disagrees with the branch color
//     the classifier actually returned.
//
// Neither case is a visibility bug (qa_issue_visibility_gate_test.go already
// polices "is the problem shown somewhere") — this is the DIFFERENT, stricter
// rule that the single source of truth for color must be Findings, so a
// future refactor that deletes the raw-field branches and reads only
// Findings would need this divergence list to shrink to reflect real
// conversions, not silently drift.
//
// No exemption is carved out for "lifecycle dim without findings": the one
// exported function in core/resource/severity_color.go (ColorFromSeverity)
// defines no such carve-out, and colorFromAnyFinding's ok=false path (no
// Finding present) returns (ColorHealthy, false), not Dim. A classifier that
// reads a raw status word, such as colorFallback in
// core/catalog/color_helpers.go, is exactly what this gate is designed to
// catch — it is not one of "the shared severity functions" the rule's
// exemption clause refers to.
//
// RATCHET semantics (identical contract to knownVisibilityGaps /
// knownStateCoverageGaps):
//   - A divergence NOT in knownColorDivergence is a NEW regression — always
//     fails, unconditionally.
//   - An allowlisted divergence that NOW conforms (colors match) fails with a
//     "remove from allowlist" message.
//   - An allowlisted divergence still mismatched is skipped (logged),
//     pre-existing debt — this list IS the deliverable driving the
//     findings-conversion work.
package unit_test

import (
	"fmt"
	"sort"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// knownColorDivergence is the inventory of (type, resource-key) rows where
// td.ResolveColor(merged) may disagree with findingsDerivedColor(merged). Key
// shape: "<shortName>:<resourceID>", matching knownVisibilityGaps' convention
// so per-type resource IDs never collide across types.
//
// It is empty: "color derives from findings" is a hard, unconditional
// invariant, and any divergence anywhere is a regression. An entry present
// here but matching fails until removed, so the map cannot quietly grow.
var knownColorDivergence = map[string]bool{}

// findingsDerivedColor computes the "worst finding wins" color implied by
// res.Findings using resource.ColorFromSeverity's severity->color table:
// SevBroken > SevWarn > SevDim > (no findings, or only SevOK/other) ->
// ColorHealthy. Unlike resource.ColorFromWave1, this does NOT filter by
// Source=="wave1" — it considers every Finding on the resource (Wave-1
// seeded or Wave-2 merged), because the invariant is "color derives
// from findings" in general, not "from wave1 findings only".
func findingsDerivedColor(findings []domain.Finding) resource.Color {
	worst := domain.SevOK
	seen := false
	for _, f := range findings {
		if !seen || severityRank(f.Severity) > severityRank(worst) {
			worst = f.Severity
			seen = true
		}
	}
	if !seen {
		return resource.ColorHealthy
	}
	return resource.ColorFromSeverity(worst)
}

// severityRank orders domain.Severity by "worseness" for the max-severity
// reduction: Broken worst, then Warn, then Dim, then OK/anything else least.
// domain.Severity's own iota order (Dim, OK, Warn, Broken) is declaration
// order, not severity order, so this cannot reuse int comparison on the enum
// directly.
func severityRank(s domain.Severity) int {
	switch s {
	case domain.SevBroken:
		return 3
	case domain.SevWarn:
		return 2
	case domain.SevDim:
		return 1
	default:
		return 0
	}
}

// TestColorFindingsConformanceGate_ColorAlwaysDerivesFromFindings is the
// gate: for every registered type with a Wave-1 Fetcher,
// every fixture resource's td.ResolveColor(merged) must equal
// findingsDerivedColor(merged.Findings) — UNLESS the (type, resourceID) pair
// is pinned in knownColorDivergence as pre-existing debt, in which case it is
// skipped (logged) instead of failed.
func TestColorFindingsConformanceGate_ColorAlwaysDerivesFromFindings(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	var stillGapped []string
	var newlyRegressed []string
	var readyForBurnDown []string

	for _, td := range types {
		fixtures := byType[td.ShortName]
		if len(fixtures) == 0 {
			continue
		}

		merged := mergeWave2Findings(t, td, fixtures, cache, clients)

		for _, res := range merged {
			actual := td.ResolveColor(res)
			expected := findingsDerivedColor(res.Findings)
			conforms := actual == expected

			key := td.ShortName + ":" + res.ID
			testName := fmt.Sprintf("%s/%s", td.ShortName, res.ID)

			t.Run(testName, func(t *testing.T) {
				allowlisted := knownColorDivergence[key]

				switch {
				case conforms && allowlisted:
					readyForBurnDown = append(readyForBurnDown, key)
					t.Errorf(
						"BURN-DOWN: %s now conforms (ResolveColor=%s == findings-derived=%s, findings=%d) "+
							"but is still pinned in knownColorDivergence — remove %q from the allowlist in this PR",
						key, bucketName(actual), bucketName(expected), len(res.Findings), key,
					)
				case conforms:
					// ResolveColor matches the findings-derived color and is not allowlisted — expected steady state.
				case allowlisted:
					stillGapped = append(stillGapped, key)
					t.Skipf(
						"KNOWN DIVERGENCE (allowlisted): %s: ResolveColor=%s != findings-derived=%s "+
							"(findings=%d) — pre-existing debt, see knownColorDivergence",
						key, bucketName(actual), bucketName(expected), len(res.Findings),
					)
				default:
					newlyRegressed = append(newlyRegressed, key)
					t.Errorf(
						"NEW DIVERGENCE (not allowlisted): %s: resource %q (type=%s) resolves to color=%s "+
							"but its Findings (%d present) imply color=%s. Either make the Color classifier "+
							"read/emit a Finding for this signal, or if this is pre-existing debt, add %q to "+
							"knownColorDivergence",
						key, res.ID, td.ShortName, bucketName(actual), len(res.Findings), bucketName(expected), key,
					)
				}
			})
		}
	}

	if len(newlyRegressed) > 0 {
		t.Logf("NEW DIVERGENCE INVENTORY (%d): %v", len(newlyRegressed), newlyRegressed)
	}
	if len(readyForBurnDown) > 0 {
		t.Logf("READY-FOR-BURN-DOWN INVENTORY (%d): %v", len(readyForBurnDown), readyForBurnDown)
	}
	if len(stillGapped) > 0 {
		t.Logf("STILL-GAPPED (allowlisted, skipped) INVENTORY (%d): %v", len(stillGapped), stillGapped)
	}
}
