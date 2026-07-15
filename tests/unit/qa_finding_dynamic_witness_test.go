// qa_finding_dynamic_witness_test.go — the DYNAMIC witness gate.
//
// OWNER GAP this test closes: the STATIC state-coverage gate
// (qa_demo_state_coverage_test.go, TestDemoStateCoverage_EveryDocumentedFindingHasAFixture)
// only proves a (type, code) pair is either witnessed or explicitly
// allowlisted as a known gap — it never separately reports, in one place,
// the exact live census of every FindingDef that fires today. The user's
// live AWS accounts don't contain every resource type or state; the demo
// fixtures are the only bench he can use to see a finding actually fire.
// Two proven live failures motivate this file:
//   - elb.misconfigured passed the static gate's bookkeeping while never
//     actually firing in demo mode: no DescribeLoadBalancerAttributes fake
//     was wired, so the Wave-2 enricher had nothing to classify against.
//   - dbc-snap rows rendered colored with NO finding behind them at all —
//     a color/finding divergence the static gate does not check per-code.
//
// This gate re-derives, from scratch, per (type, code): does at least one
// demo fixture resource — after the exact same Wave-1-then-Wave-2 fold used
// by qa_demo_state_coverage_test.go's findingCodesFor / buildDemoStateTypeCache
// (reused verbatim, not reimplemented) — actually carry a domain.Finding
// with that Code? That is a DYNAMIC witness: the finding fired, in-process,
// against real demo fixtures and the real registered Wave-2 enricher. It is
// strictly narrower than "the code exists in td.Findings" (static) and
// strictly narrower than "some row somewhere is colored" (visibility gate,
// qa_issue_visibility_gate_test.go) — it demands the SPECIFIC code.
//
// RATCHET semantics (identical contract to knownStateCoverageGaps /
// knownVisibilityGaps):
//   - A code NOT in knownUnwitnessedFindings is a NEW regression — always
//     fails, unconditionally.
//   - An allowlisted code that NOW fires dynamically fails with a "prune
//     from allowlist" message — forces the fixture/fake fix and this list
//     to land in the same PR.
//   - An allowlisted code still unwitnessed is skipped (logged), pre-existing
//     debt — this IS the burn-down deliverable: it names, per type, exactly
//     which demo fake or fixture is missing.
//
// knownUnwitnessedFindings was seeded from a full census run at the END of
// this file's authoring session. A parallel coder was, at that time, actively
// wiring the elb DescribeLoadBalancerAttributes demo fake and the
// dbc-snap/dbi-snap witnesses; two consecutive census runs against the live
// working tree still showed all five of those codes failing dynamically, so
// they are pinned below as honest, current debt rather than omitted — the
// moment that fix lands, this gate flips those five to "PRUNE" failures,
// forcing the allowlist entries out in the same PR as the fix.
package unit_test

import (
	"fmt"
	"sort"
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

// knownUnwitnessedFindings pins the exact census, at seeding time, of
// registered catalog.FindingDef codes that do NOT yet fire dynamically
// against any demo fixture — neither via Wave-1 res.Findings nor the type's
// registered Wave-2 IssueEnricher, after the shared buildDemoStateTypeCache
// fold. Key shape: "<shortName>:<code>", matching knownStateCoverageGaps'
// finding-gap key convention exactly so the two allowlists stay comparable.
//
// Same burn-down semantics as knownStateCoverageGaps:
//   - present + still unwitnessed today -> skip (logged), expected debt.
//   - present + now witnessed           -> FAIL ("prune from allowlist").
//   - a code NOT present here           -> FAIL unconditionally, a new
//     regression the allowlist was never told about.
//
// All 52 original entries from the seeding census (apigw, asg, cfn, dbi,
// ebs, ecs, ecs-task, eks, eni, igw, kinesis, logs, msk, ng, redshift,
// secrets, sns, subnet, tgw, vpce) have since been pruned — each now fires
// dynamically against its demo fixtures. Only the three ses codes remain:
// SES exposes exactly one GetAccount-shaped Wave-2 signal per account (no
// per-resource dimension to vary), and the canonical demo account is
// intentionally modeled healthy so the rest of the demo fleet has a
// non-degraded sending identity to reference. The distress shapes for
// account-shutdown / account-probation / quota-high are constructed inline
// in QA tests instead (see internal/demo/fixtures/ses.go's own doc comment).
var knownUnwitnessedFindings = map[string]bool{
	"ses:ses.account-shutdown":  true,
	"ses:ses.account-probation": true,
	"ses:ses.quota-high":        true,
	// Fire in production (shared DegradedDetails degraded-row path, which
	// classifies the error into details_denied for an authorization failure
	// vs details_unavailable for any other failure or a nil/empty body;
	// unit-tested with inline stubs) but a demo witness would leak into
	// every cluster-enumerating / DescribeNodegroup-fanning related checker
	// and flash an error on each detail open — see knownStateCoverageGaps
	// for the full reason.
	"ng:ng.warn.details_denied":        true,
	"eks:eks.warn.details_denied":      true,
	"ng:ng.warn.details_unavailable":   true,
	"eks:eks.warn.details_unavailable": true,
	// Non-auth degraded path is production-only for the auth-witnessed types
	// (mwaa/transfer/lt/ddb demo their AccessDenied → details_denied case);
	// opensearch is the inverse (demo witnesses the absent-from-response →
	// details_unavailable case, so its details_denied is production-only).
	"mwaa:mwaa.warn.details_unavailable":         true,
	"transfer:transfer.warn.details_unavailable": true,
	"lt:lt.warn.details_unavailable":             true,
	"ddb:ddb.warn.details_unavailable":           true,
	"opensearch:opensearch.warn.details_denied":  true,
}

// TestFindingDynamicWitness_EveryRegisteredCodeFiresOnDemoFixtures is the
// DYNAMIC witness gate: for every registered type and every catalog.FindingDef
// in td.Findings, at least one demo fixture resource must actually PRODUCE a
// domain.Finding carrying that exact Code after the shared Wave-1-then-Wave-2
// fold (buildDemoStateTypeCache + findingCodesFor, reused verbatim from
// qa_demo_state_coverage_test.go) — UNLESS the (type, code) pair is pinned in
// knownUnwitnessedFindings as pre-existing debt, in which case it is skipped
// (logged) instead of failed. One subtest per (type, code) so the full
// census is enumerable from `go test -v` output.
func TestFindingDynamicWitness_EveryRegisteredCodeFiresOnDemoFixtures(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildDemoStateTypeCache(t)

	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	var stillUnwitnessed []string
	var newlyRegressed []string
	var readyForPrune []string

	totalCodes := 0

	for _, td := range types {
		if len(td.Findings) == 0 {
			continue
		}
		fixtures := byType[td.ShortName]

		codes, _ := findingCodesFor(t, td, fixtures, cache, clients)

		for _, fd := range td.Findings {
			totalCodes++
			key := td.ShortName + ":" + string(fd.Code)
			testName := fmt.Sprintf("%s/%s", td.ShortName, fd.Code)
			witnessed := codes[fd.Code]
			allowlisted := knownUnwitnessedFindings[key]

			t.Run(testName, func(t *testing.T) {
				if len(fixtures) == 0 {
					if allowlisted {
						stillUnwitnessed = append(stillUnwitnessed, key)
						t.Skipf(
							"KNOWN GAP (allowlisted): %s: type has zero demo fixtures — cannot dynamically "+
								"witness finding %q", td.ShortName, fd.Code,
						)
						return
					}
					newlyRegressed = append(newlyRegressed, key)
					t.Errorf(
						"finding %q is registered but never fires on any demo fixture — either the fixture "+
							"witness or the demo fake wiring is missing (type %s has zero demo fixtures at "+
							"all). Either fix demo fixtures or add %q to knownUnwitnessedFindings",
						fd.Code, td.ShortName, key,
					)
					return
				}

				switch {
				case witnessed && allowlisted:
					readyForPrune = append(readyForPrune, key)
					t.Errorf(
						"PRUNE: %s now dynamically fires finding %q but is still pinned in "+
							"knownUnwitnessedFindings — remove %q from the allowlist in this PR",
						td.ShortName, fd.Code, key,
					)
				case witnessed:
					// Dynamically witnessed and not allowlisted — expected steady state.
				case allowlisted:
					stillUnwitnessed = append(stillUnwitnessed, key)
					t.Skipf(
						"KNOWN GAP (allowlisted): finding %q is registered but never fires on any demo "+
							"fixture for %s (phrase %q, severity %v, source %q) — either the fixture witness "+
							"or the demo fake wiring is missing, see knownUnwitnessedFindings",
						fd.Code, td.ShortName, fd.Phrase, fd.Severity, fd.Source,
					)
				default:
					newlyRegressed = append(newlyRegressed, key)
					t.Errorf(
						"finding %q is registered but never fires on any demo fixture — either the fixture "+
							"witness or the demo fake wiring is missing (type %s, phrase %q, severity %v, "+
							"source %q, %d fixtures examined). Either fix the fixtures/enricher wiring or, "+
							"if this is pre-existing debt, add %q to knownUnwitnessedFindings",
						fd.Code, td.ShortName, fd.Phrase, fd.Severity, fd.Source, len(fixtures), key,
					)
				}
			})
		}
	}

	t.Logf("TOTAL REGISTERED FINDING CODES: %d", totalCodes)
	if len(newlyRegressed) > 0 {
		t.Logf("NEW REGRESSION INVENTORY (%d): %v", len(newlyRegressed), newlyRegressed)
	}
	if len(readyForPrune) > 0 {
		t.Logf("READY-FOR-PRUNE INVENTORY (%d): %v", len(readyForPrune), readyForPrune)
	}
	if len(stillUnwitnessed) > 0 {
		sort.Strings(stillUnwitnessed)
		t.Logf("CENSUS — STILL UNWITNESSED (allowlisted, skipped) (%d): %v", len(stillUnwitnessed), stillUnwitnessed)
	}
}
