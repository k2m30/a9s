// Every registered FindingDef code fires
// on at least one demo fixture.
//
// Live AWS accounts don't contain every resource type or state; the demo
// fixtures are the only bench an operator can use to see a finding fire. The
// static state-coverage gate (qa_demo_state_coverage_test.go) checks
// bookkeeping and cannot see a code that never fires in demo mode (e.g. no
// DescribeLoadBalancerAttributes fake wired, so the Wave-2 enricher has
// nothing to classify against).
//
// Per (type, code), this gate asks whether at least one demo fixture
// resource, after qa_demo_state_coverage_test.go's findingCodesFor /
// buildDemoStateTypeCache Wave-1-then-Wave-2 fold, carries a domain.Finding
// with that Code.
//
// Ratchet semantics (same as knownStateCoverageGaps / knownVisibilityGaps):
//   - A code not in knownUnwitnessedFindings that does not fire fails.
//   - An allowlisted code that fires fails with a "prune from allowlist"
//     message.
//   - An allowlisted code that does not fire is skipped (logged).
package unit_test

import (
	"fmt"
	"sort"
	"testing"

	_ "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
)

// knownUnwitnessedFindings lists registered catalog.FindingDef codes that fire
// on no demo fixture. Key shape: "<shortName>:<code>", matching
// knownStateCoverageGaps.
//
// SES exposes one GetAccount-shaped Wave-2 signal per account, and the
// canonical demo account is modeled healthy so the rest of the demo fleet has
// a non-degraded sending identity to reference. The account-shutdown /
// account-probation / quota-high shapes are constructed inline in tests
// instead (see core/demo/fixtures/ses.go).
var knownUnwitnessedFindings = map[string]bool{
	"ses:ses.account-shutdown":  true,
	"ses:ses.account-probation": true,
	"ses:ses.quota-high":        true,
	// DescribeDomains is one batched call, so a denial degrades every listed
	// domain at once and no demo fixture can sit beside healthy domains —
	// see knownStateCoverageGaps.
	"opensearch:opensearch.warn.details_denied": true,
}

// w6aCodesUnderTheNameKeyedGate are codes checked by
// TestW6AEveryFindingFiresOnItsNamedWitnessOnly, which asserts the stronger
// property — exactly one demo row carries the code and it is the row the
// name constant names.
var w6aCodesUnderTheNameKeyedGate = map[string]bool{ //nolint:gochecknoglobals // test-only lookup
	"trail.no-cloudwatch-logs": true, "trail.no-kms": true,
	"trail.log-bucket-public": true, "trail.log-bucket-no-access-logging": true,
	"logs.no-kms": true, "alarm.actions-disabled": true,
	"r53.query-logging-off": true, "r53.dangling-record": true,
	"cf.origin-bucket-missing": true, "cf.deprecated-tls": true,
	"cf.logging-off": true, "cf.no-default-root-object": true,
	"cf.s3-origin-no-oac": true, "cf.default-certificate": true,
	"cf.no-geo-restriction": true, "acm.weak-key": true,
	"apigw.no-authorizer-public": true, "apigw.no-authorizer": true,
	"apigw.no-access-logs": true, "apigw.tracing-off": true,
	"apigw.stage-variable-secret": true,
}

// TestFindingDynamicWitness_EveryRegisteredCodeFiresOnDemoFixtures: for every
// registered type and every catalog.FindingDef in td.Findings, at least one
// demo fixture resource must produce a domain.Finding carrying that Code after
// the shared Wave-1-then-Wave-2 fold, unless the (type, code) pair is pinned
// in knownUnwitnessedFindings. One subtest per (type, code) so the full census
// is enumerable from `go test -v` output.
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
				if w6aCodesUnderTheNameKeyedGate[string(fd.Code)] {
					// That gate asserts a code fires on the one row its name
					// constant names; firing somewhere is a weaker claim.
					t.Skip("covered by the name-keyed bench gate for batch w6a")
				}
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
