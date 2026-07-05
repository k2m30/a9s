// qa_demo_pivot_coverage_test.go — the standing demo-verification gate.
//
// Demo mode (./a9s --demo) is the bench humans use to verify every
// related-resource pivot and issue glyph without live AWS credentials. This
// test is registry-driven and demo-clients-driven: for every registered type
// with RelatedDefs (resource.GetRelated), it drains that type's real Wave-1
// fetcher against demo.NewServiceClients() (the typed fakes) to get the
// actual demo fixture resources, builds one shared resource.ResourceCache
// from every fetchable type (so reverse-scan / cross-type checkers see
// sibling data exactly as production does), then re-runs each registered
// RelatedDef.Checker — the real production checker, not a demo override —
// against every fixture resource of the owning type.
//
// Contract asserted per (type, pivot):
//
//	At least one fixture resource of that type must produce a witness result
//	for that pivot (isWitnessResult — Count > 0 for ordinary pivots; for the
//	FetchFilter-bearing "search CloudTrail" pivot family, Count != 0 is
//	sufficient, mirroring resource.IsRelatedActionable's real actionability
//	rule where a server-side-filtered re-fetch is drillable even at Count=-1).
//	Not every resource x every pivot — one witness suffices. A pivot with
//	zero witnesses across every fixture resource means the demo fixture graph
//	is disconnected for that pivot: a user who opens that type in demo mode
//	and looks at every row will see a dead "(0)" row forever, so the panel is
//	dead weight in the one environment meant to demonstrate it works.
//
// Each type+pivot combination is its own t.Run subtest so the full backlog
// of disconnected pivots is enumerable from `go test -v` output in one pass,
// per type independently — a fix to one type's fixtures does not hide
// failures in another's.
//
// Plus: for every issue-capable type (a Wave-2 IssueEnricher is registered
// via aws.Wave2EnricherFor, and the type is not excluded from the issue
// badge via ExcludeFromIssueBadge), at least one fixture resource of that
// type must carry an issue in demo mode — either a Wave-1 Finding already
// present on the fixture Resource, or the type's registered Wave-2 enricher
// returning IssueCount > 0 / a non-empty Findings map when run against the
// type's own fixture resources. A type with an issue-capable enricher but
// zero fixtures ever flagged means ctrl+z / the issue badge has nothing to
// demonstrate for that type in demo mode.
//
// RATCHET: at the time this test was written, s3's fixture graph was
// disconnected for 9 of its registered pivots (a coder was rebuilding
// internal/demo/fixtures/s3.go in parallel). That fix landed and s3 is now
// fully connected. Every OTHER disconnected pivot and issue-coverage gap
// found at that time is pinned below in knownDisconnectedPivots /
// knownIssueCoverageGaps — the documented burn-down backlog. The gate is a
// ratchet, not a static allowlist:
//
//   - An entry NOT in the allowlist with no witness is a NEW regression —
//     always fails, unconditionally.
//   - An allowlisted entry that NOW HAS a witness fails with a "remove from
//     allowlist" message — this forces the fix to be reflected here in the
//     same PR that lands it, so the backlog only ever shrinks.
//   - An allowlisted entry that is still disconnected is skipped (logged,
//     not failed) — expected, pre-existing debt.
//
// s3 must NOT appear in either allowlist below: it is already connected, and
// re-adding it would silently mask a regression in the very type this gate
// was built to protect.
package unit_test

import (
	"context"
	"fmt"
	"sort"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// knownDisconnectedPivots pins the exact (type, pivot) inventory captured at
// ratchet-conversion time, in "type:pivot" form (matching def.TargetType, not
// DisplayName). This is the burn-down backlog, not a permanent exemption:
//
//   - present here + still disconnected today  -> skip (logged), expected debt.
//   - present here + now has a witness         -> FAIL ("remove from allowlist"),
//     forcing the fixture fix and this list to land in the same PR.
//   - a disconnected pivot NOT in this list     -> FAIL unconditionally, a new
//     regression the allowlist was never told about.
//
// s3 is deliberately absent: its 9 originally-disconnected pivots were fixed
// by a parallel fixture rebuild before this ratchet was written, and must
// never be re-added here.
var knownDisconnectedPivots = map[string]bool{
	"acm:apigw": true, "acm:cf": true, "acm:elb": true, "acm:r53": true,
	"alarm:apigw": true, "alarm:asg": true, "alarm:cb": true, "alarm:ct-events": true,
	"alarm:ecs": true, "alarm:eks": true, "alarm:kms": true, "alarm:lambda": true,
	"alarm:logs": true, "alarm:s3": true, "alarm:sfn": true, "alarm:waf": true,
	"ami:asg": true, "ami:cfn": true, "ami:ebs-snap": true, "ami:kms": true, "ami:ng": true,
	"apigw:acm": true, "apigw:alarm": true, "apigw:cf": true, "apigw:elb": true,
	"apigw:kms": true, "apigw:lambda": true, "apigw:logs": true, "apigw:r53": true,
	"apigw:role": true, "apigw:sfn": true, "apigw:sns": true, "apigw:vpce": true, "apigw:waf": true,
	"asg:alarm": true, "asg:elb": true, "asg:role": true, "asg:sns": true, "asg:tg": true,
	"athena:glue": true, "athena:kms": true, "athena:logs": true, "athena:role": true,
	"cb:alarm": true, "cb:ecr": true, "cb:kms": true, "cb:logs": true, "cb:pipeline": true,
	"cb:role": true, "cb:s3": true, "cb:secrets": true, "cb:sg": true, "cb:ssm": true,
	"cb:subnet": true, "cb:vpc": true,
	"cf:acm": true, "cf:alarm": true, "cf:elb": true, "cf:lambda": true, "cf:logs": true, "cf:waf": true,
	"cfn:cfn": true, "cfn:eb-rule": true, "cfn:role": true, "cfn:s3": true, "cfn:sns": true,
	"codeartifact:kms": true,
	"ct-events:cfn": true, "ct-events:lambda": true, "ct-events:secrets": true,
	"ct-events:sg": true, "ct-events:trail": true, "ct-events:vpce": true,
	"dbc-snap:ct-events": true, "dbc:ct-events": true, "dbi-snap:ct-events": true,
	"eb-rule:kinesis": true, "eb-rule:logs": true, "eb-rule:sfn": true,
	"eb:alarm": true, "eb:cfn": true, "eb:ec2": true, "eb:elb": true, "eb:logs": true,
	"eb:role": true, "eb:s3": true, "eb:sg": true, "eb:tg": true,
	"ebs-snap:ami": true, "ebs-snap:backup": true, "ebs-snap:ec2": true,
	"ebs:alarm": true, "ebs:backup": true, "ebs:cfn": true,
	"ec2:asg": true, "ec2:backup": true, "ec2:cfn": true, "ec2:logs": true, "ec2:ng": true, "ec2:ssm": true,
	"ecr:cb": true, "ecr:cfn": true, "ecr:ct-events": true, "ecr:eb-rule": true,
	"ecr:ecs-task": true, "ecr:pipeline": true, "ecr:role": true,
	"ecs-svc:alarm": true, "ecs-svc:cfn": true, "ecs-svc:ct-events": true, "ecs-svc:eb-rule": true,
	"ecs-svc:elb": true, "ecs-svc:role": true, "ecs-svc:sfn": true, "ecs-svc:sg": true,
	"ecs-svc:subnet": true, "ecs-svc:tg": true, "ecs-svc:vpc": true,
	"ecs-task:alarm": true, "ecs-task:ct-events": true, "ecs-task:ec2": true, "ecs-task:ecr": true,
	"ecs-task:eni": true, "ecs-task:role": true, "ecs-task:secrets": true, "ecs-task:sg": true,
	"ecs-task:ssm": true, "ecs-task:subnet": true,
	"ecs:alarm": true, "ecs:asg": true, "ecs:cfn": true, "ecs:ec2": true,
	"eip:asg": true, "eip:cfn": true, "eip:ecs": true, "eip:ecs-svc": true, "eip:ecs-task": true, "eip:logs": true,
	"eks:alarm": true, "eks:cfn": true, "eks:ec2": true,
	"elb:acm": true, "elb:alarm": true, "elb:cf": true, "elb:cfn": true, "elb:eni": true,
	"elb:r53": true, "elb:s3": true, "elb:waf": true,
	"eni:elb": true, "eni:lambda": true, "eni:nat": true, "eni:vpce": true,
	"glue:alarm": true, "glue:athena": true, "glue:cfn": true, "glue:kms": true, "glue:logs": true, "glue:secrets": true,
	"kinesis:alarm": true, "kinesis:cfn": true, "kinesis:kms": true, "kinesis:lambda": true,
	"kms:role": true, "kms:s3": true,
	"lambda:alarm": true, "lambda:apigw": true, "lambda:cf": true, "lambda:cfn": true, "lambda:ecr": true,
	"lambda:eni": true, "lambda:kinesis": true, "lambda:kms": true, "lambda:msk": true,
	"lambda:secrets": true, "lambda:ssm": true, "lambda:tg": true,
	"logs:alarm": true, "logs:apigw": true, "logs:ecs-task": true, "logs:kinesis": true, "logs:s3": true,
	"msk:alarm": true, "msk:cfn": true, "msk:kms": true, "msk:lambda": true, "msk:logs": true,
	"msk:s3": true, "msk:secrets": true, "msk:sg": true, "msk:vpc": true,
	"nat:alarm": true, "nat:eni": true,
	"ng:ebs": true, "ng:ec2": true, "ng:sg": true,
	"pipeline:cb": true, "pipeline:cfn": true, "pipeline:codeartifact": true, "pipeline:eb-rule": true,
	"pipeline:ecr": true, "pipeline:ecs-svc": true, "pipeline:kms": true, "pipeline:lambda": true,
	"pipeline:role": true, "pipeline:s3": true, "pipeline:sns": true,
	"r53:acm": true, "r53:apigw": true, "r53:logs": true, "r53:s3": true, "r53:vpc": true,
	"role:ec2": true, "role:eks": true, "role:iam-group": true, "role:iam-user": true,
	"rtb:cfn": true, "rtb:eni": true, "rtb:tgw": true,
	"secrets:cb": true, "secrets:cfn": true, "secrets:codeartifact": true, "secrets:eb": true,
	"secrets:role": true, "secrets:sns": true,
	"sfn:alarm": true, "sfn:eb-rule": true, "sfn:kms": true, "sfn:lambda": true, "sfn:logs": true, "sfn:role": true,
	"sg:cfn": true,
	"sns:kms": true, "sns:role": true,
	"sqs:alarm": true, "sqs:eb-rule": true, "sqs:kms": true, "sqs:sqs": true,
	"subnet:asg": true, "subnet:cfn": true, "subnet:efs": true, "subnet:eks": true,
	"tg:alarm": true, "tg:asg": true, "tg:backup": true, "tg:cfn": true, "tg:dbc": true, "tg:dbi": true,
	"tg:dbi-snap": true, "tg:ecs-svc": true, "tg:lambda": true, "tg:logs": true, "tg:sg": true, "tg:subnet": true,
	"tgw:role": true, "tgw:rtb": true, "tgw:subnet": true, "tgw:vpc": true,
	"trail:logs": true, "trail:role": true, "trail:sns": true,
	"vpc:cfn": true,
	"vpce:acm": true, "vpce:alarm": true, "vpce:cf": true, "vpce:logs": true, "vpce:r53": true,
	"vpce:s3": true, "vpce:tg": true, "vpce:waf": true,
	"waf:alarm": true, "waf:cf": true,
}

// knownIssueCoverageGaps pins the exact issue-capable types that had zero
// flagged demo fixtures at ratchet-conversion time. Same burn-down semantics
// as knownDisconnectedPivots: still-gapped -> skip (logged); now-flagged ->
// FAIL ("remove from allowlist"); a gap NOT in this list -> FAIL unconditionally.
var knownIssueCoverageGaps = map[string]bool{
	"acm": true, "cf": true, "codeartifact": true, "eb": true, "ecr": true,
	"iam-user": true, "pipeline": true, "sfn": true, "sns": true, "trail": true,
}

// demoPivotMaxFetchPages bounds the pagination drain per type as a safety
// valve against a runaway fake that never sets IsTruncated=false. Every real
// demo fixture set fits comfortably within a handful of pages.
const demoPivotMaxFetchPages = 50

// drainDemoFixtures runs td.Fetcher to exhaustion against the demo clients,
// exactly as the production fetch loop does (see aws.FetchS3Buckets), and
// returns every resource.Resource the type's demo fixtures produce. Returns
// (nil, false) when the type has no Wave-1 Fetcher registered — such types
// cannot be driven generically and are skipped by the caller.
func drainDemoFixtures(t *testing.T, td resource.ResourceTypeDef, clients *awsclient.ServiceClients) ([]resource.Resource, bool) {
	t.Helper()
	if td.Fetcher == nil {
		return nil, false
	}
	ctx := context.Background()
	var all []resource.Resource
	token := ""
	for page := range demoPivotMaxFetchPages {
		result, err := td.Fetcher(ctx, clients, token)
		if err != nil {
			t.Fatalf("%s: Fetcher page %d returned error: %v", td.ShortName, page, err)
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			return all, true
		}
		token = result.Pagination.NextToken
	}
	t.Fatalf("%s: Fetcher did not terminate within %d pages — runaway pagination in demo fixtures", td.ShortName, demoPivotMaxFetchPages)
	return all, true
}

// buildDemoTypeCache drains every registered type's demo fixtures via its
// real Wave-1 Fetcher and returns both the per-type resource lists and one
// shared resource.ResourceCache built from all of them, so that reverse-scan
// / cross-type RelatedCheckers see sibling data exactly as production does
// (the same shape TestCtEventsDemoRightColumnCheckers's buildFakeResourceCache
// establishes for ct-events, generalized to every registered type).
func buildDemoTypeCache(t *testing.T) (map[string][]resource.Resource, resource.ResourceCache) {
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

// isWitnessResult reports whether a RelatedCheckResult demonstrates a
// graph-connected pivot in the sense the UI actually cares about: the row
// would be drillable. Mirrors resource.IsRelatedActionable's real ordering —
// a FetchFilter-bearing result (the ct-events "search CloudTrail" affordance
// family, per BuildCTEventsPivotChecker) is actionable whenever Count != 0,
// including the Count=-1 "unknown, re-fetch server-side" case — it is NOT
// required to resolve a concrete positive count the way a plain cache-scan
// pivot is. A witness for a non-FetchFilter pivot still requires Count > 0
// (matches IsRelatedActionable's approximate/count>0 branches; Count==-1
// with no filter is genuinely a dead, unknown row).
func isWitnessResult(result resource.RelatedCheckResult) bool {
	if len(result.FetchFilter) > 0 {
		return result.Count != 0
	}
	return result.Count > 0
}

// TestDemoPivotCoverage_EveryRegisteredPivotHasAWitness is the standing
// ratchet: for every registered type with RelatedDefs, every pivot must have
// at least one fixture resource for which the real production checker
// produces a witness (isWitnessResult) — UNLESS the (type, pivot) pair is
// pinned in knownDisconnectedPivots as pre-existing debt, in which case it is
// skipped (logged) instead of failed. An allowlisted pair that now has a
// witness fails with a "remove from allowlist" message so the burn-down
// bookkeeping cannot silently drift from reality. One subtest per (type,
// pivot) so the full backlog is enumerable in -v output.
func TestDemoPivotCoverage_EveryRegisteredPivotHasAWitness(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildDemoTypeCache(t)
	ctx := context.Background()

	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	var stillDisconnected []string
	var newlyRegressed []string
	var readyForBurnDown []string

	for _, td := range types {
		defs := resource.GetRelated(td.ShortName)
		if len(defs) == 0 {
			continue
		}
		fixtures := byType[td.ShortName]

		for _, def := range defs {
			key := td.ShortName + ":" + def.TargetType
			testName := fmt.Sprintf("%s/%s", td.ShortName, def.TargetType)
			t.Run(testName, func(t *testing.T) {
				if def.Checker == nil {
					t.Fatalf("%s -> %s: RelatedDef has a nil Checker (structural bug)", td.ShortName, def.TargetType)
				}
				if len(fixtures) == 0 {
					t.Fatalf("%s -> %s: type %q has zero demo fixtures — cannot have a pivot witness", td.ShortName, def.TargetType, td.ShortName)
				}

				maxCount := -2
				hasWitness := false
				for _, res := range fixtures {
					result := def.Checker(ctx, clients, res, cache)
					if result.Count > maxCount {
						maxCount = result.Count
					}
					if isWitnessResult(result) {
						hasWitness = true
						break
					}
				}

				allowlisted := knownDisconnectedPivots[key]

				switch {
				case hasWitness && allowlisted:
					readyForBurnDown = append(readyForBurnDown, key)
					t.Errorf(
						"BURN-DOWN: %s -> %s now has a witness (best Count seen: %d) but is still pinned in "+
							"knownDisconnectedPivots — remove %q from the allowlist in this PR",
						td.ShortName, def.TargetType, maxCount, key,
					)
				case hasWitness:
					// Connected and not allowlisted — the expected steady state.
				case allowlisted:
					stillDisconnected = append(stillDisconnected, key)
					t.Skipf(
						"KNOWN GAP (allowlisted): %s -> %s has no witness among %d %q fixtures (best Count seen: %d) — "+
							"pre-existing debt, see knownDisconnectedPivots",
						td.ShortName, def.TargetType, len(fixtures), td.ShortName, maxCount,
					)
				default:
					newlyRegressed = append(newlyRegressed, key)
					t.Errorf(
						"NEW REGRESSION (not allowlisted): %s -> %s: no fixture resource among %d %q fixtures "+
							"produced an actionable result for this pivot (best Count seen: %d) — demo mode shows a "+
							"dead \"(0)\" row for this pivot forever. Either fix the fixture graph or, if this is "+
							"pre-existing debt, add %q to knownDisconnectedPivots",
						td.ShortName, def.TargetType, len(fixtures), td.ShortName, maxCount, key,
					)
				}
			})
		}
	}

	if len(newlyRegressed) > 0 {
		t.Logf("NEW REGRESSION INVENTORY (%d): %v", len(newlyRegressed), newlyRegressed)
	}
	if len(readyForBurnDown) > 0 {
		t.Logf("READY-FOR-BURN-DOWN INVENTORY (%d): %v", len(readyForBurnDown), readyForBurnDown)
	}
	if len(stillDisconnected) > 0 {
		t.Logf("STILL-DISCONNECTED (allowlisted, skipped) INVENTORY (%d): %v", len(stillDisconnected), stillDisconnected)
	}
}

// isIssueCapable reports whether a resource type participates in the issue
// badge / ctrl+z surface: it has a registered Wave-2 IssueEnricher AND is not
// explicitly excluded from the issue badge. Mirrors the production check at
// (*runtime.Core).HasIssueEnricher (awsclient.Wave2EnricherFor) plus the
// ExcludeFromIssueBadge gate that controls menu/list issue-glyph visibility.
func isIssueCapable(td resource.ResourceTypeDef) bool {
	if td.ExcludeFromIssueBadge {
		return false
	}
	_, ok := awsclient.Wave2EnricherFor(td.ShortName)
	return ok
}

// TestDemoIssueCoverage_EveryIssueCapableTypeHasAFlaggedFixture is the
// issue-glyph half of the standing ratchet: for every issue-capable type
// (Wave-2 enricher registered, not excluded from the issue badge), at least
// one demo fixture resource of that type must carry an issue — either a
// Wave-1 Finding already on the fixture, or the type's own Wave-2 enricher
// returning IssueCount > 0 / a non-empty Findings map when run against the
// type's fixtures — UNLESS the type is pinned in knownIssueCoverageGaps as
// pre-existing debt, in which case it is skipped (logged) instead of failed.
// An allowlisted type that now has a flagged fixture fails with a "remove
// from allowlist" message. One subtest per type so gaps are independently
// enumerable.
func TestDemoIssueCoverage_EveryIssueCapableTypeHasAFlaggedFixture(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildDemoTypeCache(t)
	ctx := context.Background()

	types := resource.AllResourceTypes()
	sort.Slice(types, func(i, j int) bool { return types[i].ShortName < types[j].ShortName })

	var stillGapped []string
	var newlyRegressed []string
	var readyForBurnDown []string

	for _, td := range types {
		if !isIssueCapable(td) {
			continue
		}
		t.Run(td.ShortName, func(t *testing.T) {
			fixtures := byType[td.ShortName]
			if len(fixtures) == 0 {
				t.Fatalf("%s: issue-capable type has zero demo fixtures — cannot demonstrate an issue", td.ShortName)
			}

			flagged := false
			for _, res := range fixtures {
				if len(res.Findings) > 0 {
					flagged = true
					break
				}
			}

			var issueCount int
			var findingsLen int
			if !flagged {
				enricher, ok := awsclient.Wave2EnricherFor(td.ShortName)
				if !ok || enricher.Fn == nil {
					t.Fatalf("%s: isIssueCapable reported true but Wave2EnricherFor now returns ok=%v", td.ShortName, ok)
				}

				result, err := enricher.Fn(ctx, clients, fixtures, cache)
				if err != nil {
					t.Fatalf("%s: Wave-2 enricher returned error: %v", td.ShortName, err)
				}
				issueCount = result.IssueCount
				findingsLen = len(result.Findings)
				flagged = result.IssueCount > 0 || len(result.Findings) > 0
			}

			allowlisted := knownIssueCoverageGaps[td.ShortName]

			switch {
			case flagged && allowlisted:
				readyForBurnDown = append(readyForBurnDown, td.ShortName)
				t.Errorf(
					"BURN-DOWN: %s now has a flagged demo fixture but is still pinned in knownIssueCoverageGaps — "+
						"remove %q from the allowlist in this PR",
					td.ShortName, td.ShortName,
				)
			case flagged:
				// Issue demonstrated and not allowlisted — the expected steady state.
			case allowlisted:
				stillGapped = append(stillGapped, td.ShortName)
				t.Skipf(
					"KNOWN GAP (allowlisted): %s: none of %d demo fixtures carry a Wave-1 Finding, and the Wave-2 "+
						"enricher flagged zero issues (IssueCount=%d, len(Findings)=%d) — pre-existing debt, see "+
						"knownIssueCoverageGaps",
					td.ShortName, len(fixtures), issueCount, findingsLen,
				)
			default:
				newlyRegressed = append(newlyRegressed, td.ShortName)
				t.Errorf(
					"NEW REGRESSION (not allowlisted): %s: none of %d demo fixtures carry a Wave-1 Finding, and the "+
						"Wave-2 enricher flagged zero issues (IssueCount=%d, len(Findings)=%d) — ctrl+z / the issue "+
						"badge has nothing to demonstrate for this type in demo mode. Either fix the fixtures or, if "+
						"this is pre-existing debt, add %q to knownIssueCoverageGaps",
					td.ShortName, len(fixtures), issueCount, findingsLen, td.ShortName,
				)
			}
		})
	}

	if len(newlyRegressed) > 0 {
		t.Logf("NEW REGRESSION INVENTORY (%d): %v", len(newlyRegressed), newlyRegressed)
	}
	if len(readyForBurnDown) > 0 {
		t.Logf("READY-FOR-BURN-DOWN INVENTORY (%d): %v", len(readyForBurnDown), readyForBurnDown)
	}
	if len(stillGapped) > 0 {
		t.Logf("STILL-GAPPED (allowlisted, skipped) INVENTORY (%d): %v", len(stillGapped), stillGapped)
	}
}
