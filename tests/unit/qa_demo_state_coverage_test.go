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
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
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
	// Reason class: "demo witness would corrupt the showroom" — these
	// findings fire in production (degraded row on a failed per-item
	// describe, shared DegradedDetails contract, which classifies the error:
	// an authorization failure → details_denied, any other failure or a
	// nil/empty body → details_unavailable) and are unit-tested with inline
	// stubs (TestFetchEKSClusters_DescribeFailureSurfacesError,
	// TestRegisteredNGFetcher_NilNodegroup_KeepsDegradedRow), but a
	// listed-but-degraded cluster/ng in the DEMO leaks into every related
	// checker that enumerates clusters or fans out DescribeNodegroup
	// (ec2/asg/ami/eks/subnet/role pivots), flashing an error on each
	// detail open.
	"ng:ng.warn.details_denied":        true,
	"eks:eks.warn.details_denied":      true,
	"ng:ng.warn.details_unavailable":   true,
	"eks:eks.warn.details_unavailable": true,
	// The non-auth degraded path (nil/empty body, transient error →
	// details_unavailable) is production-only for the types whose demo
	// witness models the AUTH path (an AccessDenied fake → details_denied):
	// mwaa/transfer/lt/ddb. Their details_unavailable is unit-tested with
	// inline nil-body/non-auth stubs, not the showroom.
	"mwaa:mwaa.warn.details_unavailable":         true,
	"transfer:transfer.warn.details_unavailable": true,
	"lt:lt.warn.details_unavailable":             true,
	"ddb:ddb.warn.details_unavailable":           true,
	// opensearch is the inverse: its demo witness is the absent-from-response
	// case (describeErr nil → details_unavailable), so the AUTH details_denied
	// path is the production-only one (unit-tested by the batch-denial stub).
	"opensearch:opensearch.warn.details_denied": true,

	// --- bucket gaps: type never resolves to this domain.Color via td.ResolveColor ---
	//

	// Reason class: "dim unreachable: AWS stops listing the resource once
	// deleted" — the Dim bucket models a genuine AWS terminal "deleted"
	// lifecycle state, but the type's own List/Describe API drops the
	// resource from its output once torn down, so no fixture — demo or
	// real — can ever witness it without misrepresenting live AWS
	// behavior.
	"redis:dim": true, // colorRedis (core/aws/catalog_databases.go) has no Dim branch — deleted as dead code per AWS API behavior (a torn-down ElastiCache ReplicationGroup simply stops appearing in DescribeReplicationGroups rather than reporting a "deleted" status; see docs/resources/redis.md §3.1/§3.2/§5 and the Bug 4 pin in aws_classifier_fivepack_test.go).

	// Reason class: "color never modeled by the classifier or any
	// FindingDef" — verified per entry against BOTH the type's Color func
	// (core/aws/catalog_*.go — no switch case or FieldUpdates-driven
	// branch returns this domain.Color) AND its registered
	// catalog.FindingDef table (core/aws/catalog_*.go Findings: [] —
	// no entry carries the matching Severity). With neither a structural
	// path nor a registered Finding of that severity, no fixture of any
	// shape could ever witness this bucket — it is not a fixture gap.
	"alarm:dim":    true,
	"apigw:broken": true, "apigw:dim": true,
	"asg:dim":       true,
	"athena:broken": true, "athena:dim": true,
	"backup:dim": true,
	"cb:dim":     true, "cb:warning": true,
	"cf:broken":        true,
	"codeartifact:dim": true,
	"dbc:dim":          true,
	"dbc-snap:dim":     true,
	"dbi:dim":          true, "dbi-snap:dim": true,
	"ddb:dim": true,
	"ebs:dim": true, "ebs-snap:dim": true,
	"ecr:dim": true, "ecr:warning": true,
	"ecs:dim": true, "ecs-svc:dim": true,
	"efs:dim":    true,
	"eip:broken": true, "eip:dim": true,
	"eks:dim":    true,
	"elb:dim":    true,
	"eni:broken": true, "eni:dim": true,
	"glue:dim": true, "glue:warning": true,
	"iam-group:broken": true, "iam-group:dim": true,
	"iam-user:dim": true,
	"igw:broken":   true, "igw:dim": true,
	"kinesis:broken": true, "kinesis:dim": true,
	"kms:dim":     true, // colorKMS (core/aws/catalog_secrets.go) has exactly three branches — Enabled->Healthy, Disabled->Warning, PendingDeletion/PendingImport/PendingReplicaDeletion/Unavailable->Broken — and no Dim return; docs/resources/kms.md §3.1/§3.2 document no Dim-producing signal for this type.
	"logs:broken": true, "logs:dim": true,
	"lt:dim":       true, // colorLT (core/aws/catalog_compute.go) is colorFromAnyFinding-only and no registered lt.* FindingDef is SevDim, so no fixture can resolve to the dim bucket; docs/resources/lt.md §4 documents no Dim-producing signal for this type.
	"msk:dim":      true,
	"ng:dim":       true,
	"pipeline:dim": true, "pipeline:warning": true,
	"policy:dim": true,
	"r53:broken": true, "r53:dim": true,
	"redshift:dim": true,
	"role:dim":     true,
	"rtb:dim":      true,
	"s3:dim":       true,
	"secrets:dim":  true,
	"ses:dim":      true,
	"sfn:dim":      true, "sfn:warning": true,
	"sg:dim":     true,
	"sns:broken": true, "sns:dim": true,
	"sns-sub:broken": true,
	"sqs:broken":     true, "sqs:dim": true,
	"ssm:dim":      true,
	"subnet:dim":   true,
	"tg:dim":       true,
	"trail:dim":    true,
	"transfer:dim": true, // colorTransfer (core/aws/catalog_networking.go) is colorFromAnyFinding-only, and no registered FindingDef carries SevDim — structurally, AWS Transfer Family's DescribeServer State enum (OFFLINE|ONLINE|STARTING|STOPPING|START_FAILED|STOP_FAILED per docs.aws.amazon.com/transfer/latest/APIReference/API_DescribeServer.html) has no deleted/terminal value at all, so no fixture of any shape could ever witness a Dim row for this type.
	"vpc:broken":   true, "vpc:dim": true,
	"waf:broken": true, "waf:dim": true,

	// ct-events:healthy — colorCTEvents (core/aws/catalog_monitoring.go)
	// has exactly two colored branches (ct-danger -> Broken, ct-attention ->
	// Warning) and defaults every other status straight to Dim; Healthy is
	// not a reachable return value from this classifier by design, not a
	// fixture gap.
	"ct-events:healthy": true,

	// --- finding gaps: seeded 2026-07-06 from the machine findings registry
	// (249 FindingDefs across all types). The 52 original entries from that
	// census (apigw, asg, cfn, dbi, ebs, ecs, ecs-task, eks, eni, igw,
	// kinesis, logs, msk, ng, redshift, secrets, sns, subnet, tgw, vpce)
	// have since been burned down — each now has a demo fixture producing
	// the finding. s3 is deliberately absent and MUST stay absent: its
	// per-resource Wave-1/Wave-2 "public access block incomplete" signal
	// (EnrichS3Posture / GetPublicAccessBlockOutput fixtures)
	// already flows into TestDemoIssueCoverage_EveryIssueCapableTypeHasAFlaggedFixture
	// in qa_demo_pivot_coverage_test.go, which is unaffected by this ratchet.
	//
	// Only the three ses codes remain: SES exposes exactly one
	// GetAccount-shaped Wave-2 signal per account (no per-resource dimension
	// to vary), and the canonical demo account is intentionally modeled
	// healthy so the rest of the demo fleet has a non-degraded sending
	// identity to reference. The distress shapes for account-shutdown /
	// account-probation / quota-high are constructed inline in QA tests
	// instead (see core/demo/fixtures/ses.go's own doc comment).
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
