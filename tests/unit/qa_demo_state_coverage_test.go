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
//     pivot logic. "Reachable" is discovered empirically: a bucket is
//     reachable for a type iff at least one demo fixture resource of that
//     type actually classifies into it via td.ResolveColor. A bucket no demo
//     fixture ever reaches is either (a) a fixture gap — the type has real
//     resources that could be in that state but none exist in demo data, pin
//     it in knownStateCoverageGaps — or (b) structurally unreachable for that
//     type (e.g. a type with no lifecycle signal at all only ever resolves
//     ColorHealthy) in which case it is never observed as a gap in the first
//     place: this test only ever asks about buckets it saw at least one
//     fixture claim as a candidate improvement target, so "unreachable"
//     buckets are silently absent from both the failure set and the
//     allowlist, never scored as N/A explicitly.
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
// "public access block incomplete" finding: internal/demo/fixtures/s3.go
// already carries a bucket with an incomplete GetPublicAccessBlockOutput, so
// EnrichS3PublicAccessBlock (or the Wave-1 classification, whichever the
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

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// knownStateCoverageGaps pins the exact inventory of missing demo-fixture
// states captured at ratchet-conversion time, in two key shapes:
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
var knownStateCoverageGaps = map[string]bool{
	// --- bucket gaps: type never resolves to this domain.Color via td.ResolveColor ---
	"alarm:dim":    true,
	"apigw:broken": true, "apigw:dim": true, "apigw:warning": true,
	"asg:dim":       true,
	"athena:broken": true, "athena:dim": true,
	"backup:broken": true, "backup:dim": true, "backup:warning": true,
	"cb:broken": true, "cb:dim": true, "cb:warning": true,
	"cf:broken":           true,
	"codeartifact:broken": true, "codeartifact:dim": true, "codeartifact:warning": true,
	"ct-events:healthy": true,
	"dbc:dim":           true,
	"dbc-snap:dim":      true,
	"dbi:dim":           true, "dbi-snap:dim": true,
	"ddb:dim":        true,
	"eb-rule:broken": true, "eb-rule:warning": true,
	"ebs:dim": true, "ebs-snap:dim": true,
	"ecr:broken": true, "ecr:dim": true, "ecr:warning": true,
	"ecs:dim": true, "ecs-svc:dim": true,
	"efs:dim":    true,
	"eip:broken": true, "eip:dim": true,
	"eks:dim":    true,
	"elb:dim":    true,
	"eni:broken": true, "eni:dim": true,
	"glue:broken": true, "glue:dim": true, "glue:warning": true,
	"iam-group:broken": true, "iam-group:dim": true, "iam-group:warning": true,
	"iam-user:broken": true, "iam-user:dim": true,
	// iam-user:warning: structurally unreachable through THIS harness — every
	// raw fixture is fed straight through td.ResolveColor (no Wave-2
	// FieldUpdates fold), and FetchIAMUsersPage (internal/aws/iam_users.go)
	// unconditionally hardcodes Fields["has_console_password"]="false" at
	// fetch time. colorIAMUser's only non-Healthy branch requires
	// has_console_password=="true", so no raw fixture — regardless of its
	// PasswordLastUsed value — can ever resolve to Warning here. The
	// corrected classification (docs/resources/iam-user.md §3.2 console-
	// login-without-MFA -> Broken) requires the Wave-2 field-update merge
	// this harness intentionally omits; see TestColorIAMUser_ConsoleUserWithoutMFAClassifiesBroken
	// in aws_classifier_fivepack_test.go for the pinned production gap.
	"iam-user:warning": true,
	"igw:broken":       true, "igw:dim": true,
	"kinesis:broken": true, "kinesis:dim": true,
	// kms:dim: colorKMS (internal/aws/catalog_secrets.go) has exactly three
	// branches — Enabled->Healthy, Disabled->Warning, PendingDeletion/
	// PendingImport/PendingReplicaDeletion/Unavailable->Broken — and no Dim
	// return. docs/resources/kms.md §3.1/§3.2 document no Dim-producing
	// signal for this type; structurally unreachable, not a fixture gap.
	"kms:dim":     true,
	"logs:broken": true, "logs:dim": true,
	"msk:dim":         true,
	"ng:dim":          true,
	"pipeline:broken": true, "pipeline:dim": true, "pipeline:warning": true,
	"policy:broken": true, "policy:dim": true,
	"r53:broken": true, "r53:dim": true,
	// redis:dim: colorRedis (internal/aws/catalog_databases.go) has no Dim
	// branch — deleted as dead code per AWS API behavior (a torn-down
	// ElastiCache ReplicationGroup simply stops appearing in
	// DescribeReplicationGroups rather than reporting a "deleted" status;
	// see docs/resources/redis.md §3.1/§3.2/§5 and the Bug 4 pin in
	// aws_classifier_fivepack_test.go). Structurally unreachable, not a
	// fixture gap.
	"redis:dim":    true,
	"redshift:dim": true,
	"role:dim":     true, "role:warning": true,
	"rtb:dim":   true,
	"s3:broken": true, "s3:dim": true, "s3:warning": true,
	"secrets:dim": true,
	"ses:dim":     true,
	"sfn:broken":  true, "sfn:dim": true, "sfn:warning": true,
	"sg:dim": true, "sg:warning": true,
	"sns:broken": true, "sns:dim": true, "sns:warning": true,
	"sns-sub:broken": true,
	"sqs:broken":     true, "sqs:dim": true, "sqs:warning": true,
	"ssm:dim":    true,
	"subnet:dim": true,
	"tg:broken":  true, "tg:dim": true, "tg:warning": true,
	"trail:dim":  true,
	"vpc:broken": true, "vpc:dim": true,
	"waf:broken": true, "waf:dim": true, "waf:warning": true,

	// --- finding gaps: seeded 2026-07-06 from the machine findings registry
	// (249 FindingDefs across all types). The 52 original entries from that
	// census (apigw, asg, cfn, dbi, ebs, ecs, ecs-task, eks, eni, igw,
	// kinesis, logs, msk, ng, redshift, secrets, sns, subnet, tgw, vpce)
	// have since been burned down — each now has a demo fixture producing
	// the finding. s3 is deliberately absent and MUST stay absent: its
	// per-resource Wave-1/Wave-2 "public access block incomplete" signal
	// (EnrichS3PublicAccessBlock / GetPublicAccessBlockOutput fixtures)
	// already flows into TestDemoIssueCoverage_EveryIssueCapableTypeHasAFlaggedFixture
	// in qa_demo_pivot_coverage_test.go, which is unaffected by this ratchet.
	//
	// Only the three ses codes remain: SES exposes exactly one
	// GetAccount-shaped Wave-2 signal per account (no per-resource dimension
	// to vary), and the canonical demo account is intentionally modeled
	// healthy so the rest of the demo fleet has a non-degraded sending
	// identity to reference. The distress shapes for account-shutdown /
	// account-probation / quota-high are constructed inline in QA tests
	// instead (see internal/demo/fixtures/ses.go's own doc comment).
	"ses:ses.account-shutdown":  true,
	"ses:ses.account-probation": true,
	"ses:ses.quota-high":        true,
}

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
// every fixture resource.
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

	for _, res := range fixtures {
		buckets[td.ResolveColor(res)] = true
		for _, f := range res.Findings {
			codes[f.Code] = true
		}
	}

	if enricher, ok := awsclient.Wave2EnricherFor(td.ShortName); ok && enricher.Fn != nil {
		result, err := enricher.Fn(context.Background(), clients, fixtures, cache)
		if err != nil {
			t.Fatalf("%s: Wave-2 enricher returned error: %v", td.ShortName, err)
		}
		for _, f := range result.Findings {
			codes[f.Code] = true
		}
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
