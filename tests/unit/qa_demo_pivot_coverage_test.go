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

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
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
	// alarm:ct-events is structurally unwitnessable: checkAlarmCTEvents
	// (internal/aws/alarm_related_extra.go) reads evRes.Fields["event_source"],
	// but FetchCTEvents (internal/aws/ct_events.go) never writes that key — it
	// writes Fields["source"] for the AWS EventSource value instead. The
	// Contains() guard is permanently comparing against "", so no fixture can
	// produce a witness without a checker-code fix (out of scope for this
	// fixture-only wave).
	"alarm:ct-events": true,
	// apigw:elb, apigw:r53, apigw:role, apigw:vpce, apigw:waf: each checker
	// (internal/aws/apigw_related.go checkApigwELB/R53/Role/VPCE/WAF) is
	// hardcoded to return Count:-1 whenever res.ID != "" (i.e. always, for any
	// real fixture) — the AWS API path needed to resolve a concrete ID is
	// documented in each function's comment as unavailable from GetApis
	// (e.g. VpcLink->NLB needs GetVpcLinks, role/authorizer needs
	// per-route GetAuthorizer, private-API endpoint id is v1-only). Since
	// isWitnessResult requires Count>0 for non-FetchFilter results, no
	// fixture can ever produce a witness here.
	"apigw:elb": true, "apigw:r53": true,
	"apigw:role": true, "apigw:vpce": true, "apigw:waf": true,
	// apigw:sfn, apigw:sns: checkApigwSFN/SNS (same file) can detect that an
	// SFN/SNS integration exists but the target ARN lives in the per-route
	// request template, not the integration URI — both functions explicitly
	// return Count:-1 (found-but-unidentified) or Count:0 (not found), never
	// Count>0. Structurally unwitnessable without request-template parsing.
	"apigw:sfn": true, "apigw:sns": true,
	// asg:role: checkASGRole (internal/aws/asg_related.go) returns role IDs as
	// full ARNs (ServiceLinkedRoleARN verbatim, and asgInstanceProfileToRoles'
	// r.Arn), unlike every sibling role-pivot checker (checkEC2Role,
	// checkEKSRole, checkNGRole) which all extract the bare role name via the
	// last "/" segment before emitting it. FetchRolesByIDs
	// (internal/aws/iam_roles.go) calls iam:GetRole(RoleName: id), which AWS
	// requires to be a bare name, not an ARN — a fixture wired to produce a
	// non-zero checkASGRole count resolves the pivot-coverage witness but then
	// fails the harder TestDemoRelatedIDsResolve_EveryWitnessedIDIsFetchable
	// gate (drill click 404s: "NoSuchEntity ... cannot be found" on the ARN
	// string). Fixing this requires editing checkASGRole/asgInstanceProfileToRoles
	// to strip the ARN to a name like its siblings do — internal/aws/ is
	// frozen for this fixture-only wave.
	"asg:role": true,
	// athena:glue: checkAthenaGlue (internal/aws/athena_related.go) is
	// documented as structurally incapable of a non-zero result — its own
	// comment states "No structured glue job/catalog field exists on the WG
	// config... resolving which specific Glue jobs share this catalog
	// requires a catalog crawl" — the function always returns Count:0 or
	// Count:-1, never Count>0.
	"athena:glue": true,
	// ecr:cfn: ecrCFNStackName (internal/aws/ecr_related.go) reads
	// res.Fields["cfn_stack_name"], but FetchECRRepositories
	// (internal/aws/ecr.go) never writes that key — no ListTagsForResource
	// enrichment call is wired for ECR anywhere in the fetcher. The field is
	// permanently "", so no fixture can produce a witness without a
	// fetcher-code change (out of scope for this fixture-only wave).
	// ecr:ecs-task: checkECRECSTask (internal/aws/ecr_related_extra.go) scans
	// every ecs-task Fields value for a substring match on ".dkr.ecr." — but
	// FetchECSTasks (internal/aws/ecs_task.go) never stores the container
	// image URI in any Fields entry (only task_id/cluster/status/roles/etc are
	// populated). No fixture can produce a matching field without a
	// fetcher-code change.
	// ecr:role: checkECRRole → ecrPolicyRoleARNs (internal/aws/ecr_related_extra.go)
	// returns role IDs as full ARNs verbatim from the repository policy's
	// Principal.AWS field, same class of bug as asg:role — FetchRolesByIDs
	// (internal/aws/iam_roles.go) requires a bare role name for
	// iam:GetRole(RoleName:...). A fixture policy would produce a
	// pivot-coverage witness that then fails
	// TestDemoRelatedIDsResolve_EveryWitnessedIDIsFetchable. See
	// internal/demo/fakes/ecr.go GetRepositoryPolicy for the documented
	// rationale.
	"ecr:cfn": true, "ecr:ecs-task": true, "ecr:role": true,
	// eip:ecs, eip:ecs-svc, eip:ecs-task, eip:logs: each checker
	// (internal/aws/eip_related.go checkEIPECS/ECSSvc/ECSTask/Logs) is
	// hardcoded to return Count:-1 whenever res.ID != "" — every function's
	// own comment documents the missing AWS API path (ECS task-to-EIP
	// association requires per-cluster DescribeTasks; EIP flow logs require
	// per-ENI DescribeFlowLogs), both explicitly "outside the 1-call budget".
	// Structurally unwitnessable; no fixture changes this.
	"eip:ecs": true, "eip:ecs-svc": true, "eip:ecs-task": true, "eip:logs": true,
	// elb:r53: checkELBR53 (internal/aws/elb_related.go) is hardcoded to
	// return Count:-1 whenever Fields["dns_name"] != "" (true for every real
	// ELB) — reverse-resolving which R53 records alias to this LB's DNS name
	// requires enumerating every hosted zone's record sets, outside the
	// checker's call budget. Structurally unwitnessable.
	"elb:r53": true,
	// glue:cfn: checkGlueCFN (internal/aws/glue_related.go) constructs the
	// job ARN from regionFromEnv() (reads AWS_REGION / AWS_DEFAULT_REGION)
	// and accountIDFromClients() (STS GetCallerIdentity), returning Count:-1
	// whenever either is empty. This test process runs with neither
	// AWS_REGION nor AWS_DEFAULT_REGION set, so region is always "" here —
	// no fixture data can influence an os.Getenv read. Fixed fixture data
	// (SecurityConfigurations/TagsByResourceARN) is in place and the pivot
	// resolves correctly whenever a region env var is present (e.g. real
	// `./a9s --demo` launches, which typically inherit AWS_REGION from the
	// operator's shell/profile).
	"glue:cfn": true,
	// kinesis:lambda: checkKinesisLambda (internal/aws/kinesis_related.go)
	// matches lambda cache entries whose Fields["event_source_arn"] equals
	// this stream's ARN — but the registered Wave-1 Lambda Fetcher
	// (internal/aws/catalog_compute.go) calls FetchLambdaFunctionsPage, which
	// always passes eventSourceAPI=nil into
	// FetchLambdaFunctionsPageWithEventSources. That nil guard means
	// Fields["event_source_arn"] is permanently "" for every lambda resource
	// in both production and demo, regardless of fixture data (the demo
	// fixtures already model a working ESM — data-pipeline-transform →
	// clickstream-ingest — the field is just never populated). Fixing this
	// requires passing a real eventSourceAPI into the registered fetcher,
	// which is an internal/aws/ change out of scope for this fixture-only wave.
	"kinesis:lambda": true,
	// kms:s3: checkKMSS3 (internal/aws/kms_related.go) is hardcoded to return
	// Count:-1 whenever res.ID != "" — its own comment states "S3 resources
	// do not expose KMS key IDs in Fields or RawStruct, so the relationship
	// cannot be determined from cache alone." Structurally unwitnessable.
	"kms:s3": true,
	// logs:ecs-task: checkLogsECSTask (internal/aws/logs_related.go) extracts
	// a "family" substring from the log group name (text after "/ecs/" up to
	// the next "/") and checks whether any cached ecs-task's ID or Name
	// contains that family. But FetchECSTasks (internal/aws/ecs_task.go)
	// always sets task ID/Name to the bare task UUID parsed from the task
	// ARN's last "/" segment — never the family/service name. No real AWS
	// task UUID would contain a human-readable family substring; the only way
	// to produce a witness would be to fabricate a UUID that happens to embed
	// the literal family string, which is an artificial ID collision, not a
	// realistic fixture. Structurally unwitnessable without a fetcher/checker
	// change (out of scope for this fixture-only wave).
	"logs:ecs-task": true,
	// msk:lambda: checkMSKLambda (internal/aws/msk_related.go) matches lambda
	// cache entries whose Fields["event_source_arn"] equals the cluster ARN —
	// same eventSourceAPI=nil registration bug as kinesis:lambda above (the
	// registered Wave-1 Lambda Fetcher never populates this field). The demo
	// fixture already models a working ESM (data-pipeline-transform →
	// acme-events-prod), the field is just never populated. Out of scope for
	// this fixture-only wave.
	"msk:lambda": true,
	// pipeline:eb-rule: checkPipelineEbRule (internal/aws/pipeline_related.go)
	// reads res.Fields["arn"], but FetchPipelines (internal/aws/pipeline.go)
	// never populates that key in its Fields map (only name/pipeline_type/
	// created/updated/version are set) — the read is permanently "" and the
	// checker short-circuits to Count:0 before ever calling
	// eventbridge:ListRuleNamesByTarget (which the fake already implements
	// correctly). Fixing this requires adding Fields["arn"] to the pipeline
	// fetcher, an internal/aws/ change out of scope for this fixture-only wave.
	"pipeline:eb-rule": true,
	// r53:logs: checkR53Logs (internal/aws/r53_related.go) is hardcoded to
	// return Count:-1 whenever res.ID != "" — its own comment states the
	// query-log configuration API (route53:ListQueryLoggingConfigs) "is not
	// in Route53API yet" and there is "no second-call workaround... at 1-call
	// budget." Structurally unwitnessable without adding that API to the
	// Route53 interface — out of scope for this fixture-only wave.
	"r53:logs": true,
	// sqs:kms: checkSQSKMS (internal/aws/sqs_related.go) reads
	// res.Fields["kms_key_id"], but FetchSQSQueuesPage (internal/aws/sqs.go)
	// never writes that key — its Fields map only sets queue_name/queue_url/
	// arn/approx_messages/approx_not_visible/delay_seconds (kms_key_id is only
	// ever populated by the unrelated cwlogs.go fetcher for log groups). The
	// KmsMasterKeyId value lives in the raw GetQueueAttributes Attributes map
	// but the checker's own comment documents it deliberately does not read
	// RawStruct there. No fixture can produce a witness without a
	// fetcher-code change — out of scope for this fixture-only wave.
	"sqs:kms": true,
	// subnet:asg: checkSubnetASG (internal/aws/subnet_related.go) reads
	// asgRes.Fields["vpc_zone_identifier"] (falling back to Fields["subnets"]),
	// but FetchAutoScalingGroupsPage (internal/aws/asg.go) never writes either
	// key into its Fields map (only asg_name/min_size/max_size/desired/
	// instances/status/instances_unhealthy_count/in_service_count/
	// suspended_processes are set) — the sibling forward checker checkASGSubnets
	// reads RawStruct.VPCZoneIdentifier directly and works fine, but the
	// reverse checker here only looks at Fields. No fixture can produce a
	// witness without a fetcher-code change — out of scope for this
	// fixture-only wave.
	"subnet:asg": true,
	// subnet:efs: checkSubnetEFS (internal/aws/subnet_related.go) is hardcoded
	// to return Count:-1 whenever res.ID != "" — its own comment states mount
	// targets are listed per-file-system via DescribeMountTargets, and the EFS
	// list cache only carries FileSystemDescription (no mount targets),
	// "outside the 1-call budget." Structurally unwitnessable.
	"subnet:efs": true,
	// subnet:eks: checkSubnetEKS (internal/aws/subnet_related.go) reads
	// eksRes.Fields["subnets"] (falling back to Fields["subnet_ids"]), but
	// buildEKSResource (internal/aws/eks.go) never writes either key into its
	// Fields map (only cluster_name/version/status/endpoint/platform_version/
	// arn/health_issues_count/health_issues are set) — the subnet IDs live in
	// RawStruct.ResourcesVpcConfig.SubnetIds, which this checker never reads.
	// Same class of bug as subnet:asg above — a fetcher-code change out of
	// scope for this fixture-only wave.
	"subnet:eks": true,
	// tg:backup, tg:dbc, tg:dbi, tg:dbi-snap, tg:logs, tg:sg, tg:subnet: each
	// checker (internal/aws/tg_related.go checkTGBackup/DBC/DBI/DBISnap/Logs/
	// SG/Subnet) is hardcoded to return Count:-1 whenever the TG has an ARN
	// (or, for sg/subnet, whenever Fields["vpc_id"] != "") — every function's
	// own comment documents the missing AWS API path: target identity
	// (backup/dbc/dbi/dbi-snap) requires DescribeTargetHealth + matching IP
	// addresses against instance/DB ENIs; logs requires resolving to the
	// parent ELB's access logs; sg/subnet require DescribeTargetHealth + ENI
	// lookup — all explicitly "outside the 1-call budget." Structurally
	// unwitnessable; no fixture changes this.
	"tg:backup": true, "tg:dbc": true, "tg:dbi": true,
	"tg:dbi-snap": true, "tg:logs": true, "tg:sg": true, "tg:subnet": true,
	// vpce:acm, vpce:cf, vpce:r53, vpce:s3, vpce:tg, vpce:waf: each checker
	// (internal/aws/vpce_related.go checkVPCEACM/CF/R53/S3/TG/WAF) is
	// hardcoded to return Count:-1 whenever res.ID != "" — every function's
	// own comment documents the missing AWS API path (CloudFront->VPCE needs
	// VPC Origins, not on DistributionSummary; R53 needs
	// route53:ListHostedZonesByVPC, not in the hosted-zone cache; S3 gateway
	// access needs policy-document JSON interpretation; TG/WAF associations
	// require DescribeTargetHealth / wafv2:ListResourcesForWebACL from the
	// other side). Structurally unwitnessable; no fixture changes this.
	"vpce:acm": true, "vpce:cf": true, "vpce:r53": true,
	"vpce:s3": true, "vpce:tg": true, "vpce:waf": true,
	// waf:cf: checkWAFCF (internal/aws/waf_related.go) guards on
	// res.Fields["scope"] == wafv2types.ScopeCloudfront before ever calling
	// cloudfront:ListDistributionsByWebACLId. But FetchWAFWebACLsPage
	// (internal/aws/waf.go) hardcodes both the ListWebACLs request
	// (Scope: wafv2types.ScopeRegional) and every returned resource's
	// Fields["scope"] to ScopeRegional — it never issues the separate
	// Scope=CLOUDFRONT ListWebACLs call CloudFront-associated WebACLs require
	// (which AWS also mandates be made against us-east-1). No WAF resource,
	// in demo or production, can ever carry scope=CLOUDFRONT, so the guard
	// permanently short-circuits to Count:0. Fixing this requires a second
	// fetch path in the WAF fetcher — an internal/aws/ change out of scope
	// for this fixture-only wave.
	"waf:cf": true,
}

// knownIssueCoverageGaps pins the exact issue-capable types that had zero
// flagged demo fixtures at ratchet-conversion time. Same burn-down semantics
// as knownDisconnectedPivots: still-gapped -> skip (logged); now-flagged ->
// FAIL ("remove from allowlist"); a gap NOT in this list -> FAIL unconditionally.
// trail: registered with Wave2: IssueEnricher{Fn: InFetcherWave2Sentinel} —
// a documentation-only marker meaning "this type's Wave-2 signal is computed
// inside the fetcher, not via a separate enricher call." But
// FetchCloudTrailTrails (internal/aws/trail.go) only ever writes
// is_logging/latest_delivery_error/log_file_validation_enabled into
// Fields — colorTrail (internal/aws/catalog_monitoring.go) reads those Fields
// directly for row coloring, but no code path ever appends to r.Findings.
// InFetcherWave2Sentinel itself unconditionally returns empty
// Findings/IssueCount. So isIssueCapable reports trail as issue-capable (a
// Wave2 enricher is registered) but neither Wave-1 Findings nor the Wave-2
// enricher result can ever be non-empty, in demo or production, regardless
// of fixture data. Fixing this requires the fetcher to append r.Findings for
// the same conditions colorTrail already checks — an internal/aws/ change
// out of scope for this fixture-only wave.
var knownIssueCoverageGaps = map[string]bool{
	"trail": true,
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
