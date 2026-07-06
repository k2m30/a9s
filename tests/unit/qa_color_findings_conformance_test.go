// qa_color_findings_conformance_test.go — the standing OWNER RULE gate for the
// "color derives from findings" architectural invariant: for every registered
// type × demo fixture row, td.ResolveColor(merged) must equal the color
// implied by the resource's own Findings (Wave-1 seeded, Wave-2 merged) via
// the shared severity-to-color mapping in internal/resource/severity_color.go
// (ColorFromSeverity / ColorFromWave1) — NOT some other structural field the
// classifier reads directly.
//
// findingsDerivedColor computes the "max severity wins" color over
// res.Findings: any SevBroken finding -> ColorBroken; else any SevWarn ->
// ColorWarning; else any SevDim -> ColorDim; no findings at all -> ColorHealthy.
// This reuses resource.ColorFromSeverity's exact severity->color table (see
// internal/resource/severity_color.go) rather than reinventing the mapping;
// the only addition here is the "pick the worst finding" reduction, which
// ColorFromWave1 does NOT do (it only inspects Source=="wave1" findings, first
// match wins) — this gate deliberately looks at ALL findings regardless of
// Source, because the owner's invariant is "color derives from findings",
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
// No exemption is carved out for "lifecycle dim without findings": the only
// two exported functions in internal/resource/severity_color.go
// (ColorFromSeverity, ColorFromWave1) define no such carve-out —
// ColorFromWave1's ok=false path (no wave1 Finding present) returns
// (ColorHealthy, false), not Dim. Per-type helpers like colorFallback /
// colorWave1OrHealthy / cfnStackColor / acmColor / r53Color in
// internal/aws/catalog_color_helpers.go are exactly the raw-field classifiers
// this gate is designed to catch — they are not "the shared severity
// functions" the owner's exemption clause refers to.
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

	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// knownColorDivergence pins the exact inventory of (type, resource-key) rows
// where td.ResolveColor(merged) disagrees with findingsDerivedColor(merged) —
// today's full census of classifier branches still coloring from raw fields
// instead of Findings. Key shape: "<shortName>:<resourceID>", matching
// knownVisibilityGaps' convention so per-type resource IDs never collide
// across types. A handful of KMS resource IDs repeat across two demo
// fixtures (grant-target aliasing) — both occurrences collapse onto the same
// map key here, same dedupe-by-key behavior as knownVisibilityGaps.
//
// Burn-down semantics: an entry present here AND still mismatched today is
// pre-existing debt (skipped). An entry present here but NOW matching is a
// completed conversion — fails until removed from this map. An entry not
// present here that mismatches is a brand-new regression — always fails.
//
// CENSUS (2026-07-07, seeded when this gate was introduced), 159 unique keys
// across 171 divergent (type, resource) subtests, by ShortName (count:
// dominant actual-color -> findings-derived-color pattern, i.e. the
// structural-field branch this gate expects a conversion to replace):
//
//	role (44): healthy->warning (43, trust-policy/wildcard findings not yet
//	    read for color), broken->warning (1)
//	kms (25): healthy->warning (25, key-rotation/grant findings not read)
//	eb-rule (11): healthy->warning (7), healthy->broken (2), dim->healthy (1),
//	    dim->warning (1)
//	acm (6): healthy->warning (2), warning->healthy (1), broken->healthy (2),
//	    dim->healthy (1) — certificate-expiry classifier reads raw fields
//	    both directions
//	vpc (6): healthy->warning (6, orphan/no-flow-logs findings not read)
//	dbc-snap (5): healthy->broken (5, orphan/retention findings not read)
//	backup (5): healthy->broken (4), healthy->warning (1)
//	ecs-task (4), eb (4), cf (4), efs (4), s3 (4), sqs (4), opensearch (4):
//	    mixed healthy/warning/dim vs broken/warning/healthy — see
//	    divergence_rows detail in test output; each is a structural classifier
//	    (task health, EB env health, CF distribution status, EFS mount
//	    target/lifecycle, S3 PAB gaps, SQS DLQ/redrive, OpenSearch cluster
//	    health) not yet reading its own Wave-1/Wave-2 Finding for color
//	dbi-snap (3), codeartifact (3), apigw (3): healthy/warning->broken/warning
//	iam-group (2), ecs-svc (2), ec2 (2), dbc (2), athena (2): 1-2 rows each
//	waf, vpce, tgw, tg, sns, sfn, secrets, r53, policy, pipeline, nat, igw,
//	    iam-user, glue, eks, ecs, ecr, ddb, dbi, cfn, cb, ami (1 each):
//	    single-fixture divergences, one per type
//
// This list IS the deliverable for the findings-conversion work: each entry
// names a classifier branch (by ShortName) that still needs its raw-field
// check replaced with a Finding the color can be derived from.
var knownColorDivergence = map[string]bool{
	"acm:*.acme-corp.com":                                                true,
	"acm:acme-logs.internal.com":                                         true,
	"acm:inactive.acme-corp.com":                                         true,
	"acm:staging.acme-corp.com":                                          true,
	"acm:timeout.acme-corp.com":                                          true,
	"acm:validation-failed.acme-corp.com":                                true,
	"ami:ami-0a1b2c3d4e5f60004":                                          true,
	"apigw:abc123def4":                                                   true,
	"apigw:efg567hij8":                                                   true,
	"apigw:klm901nop2":                                                   true,
	"athena:a9s-demo-s3-queries":                                         true,
	"athena:acme-data-science":                                           true,
	"backup:33333333-3333-3333-3333-333333333333":                        true,
	"backup:44444444-4444-4444-4444-444444444444":                        true,
	"backup:55555555-5555-5555-5555-555555555555":                        true,
	"backup:66666666-6666-6666-6666-666666666666":                        true,
	"backup:77777777-7777-7777-7777-777777777777":                        true,
	"cb:acme-integration-tests":                                          true,
	"cf:E3C4D5E6F7G8H9":                                                  true,
	"cf:E4D5E6F7G8H9I0":                                                  true,
	"cf:E5E6F7G8H9I0J1":                                                  true,
	"cf:E8H9I0J1K2L3M4":                                                  true,
	"cfn:acme-decommissioned-poc":                                        true,
	"codeartifact:acme-maven":                                            true,
	"codeartifact:acme-npm":                                              true,
	"codeartifact:acme-pypi":                                             true,
	"dbc-snap:orphan-deleted-cluster-snap":                               true,
	"dbc-snap:rds:acme-docdb-prod-2026-03-20":                            true,
	"dbc-snap:rds:analytics-docdb-2026-03-20":                            true,
	"dbc-snap:rds:dbc-retention-test":                                    true,
	"dbc-snap:rds:prod-aurora-cluster-2026-04-15":                        true,
	"dbc:healthy-dbc-maint-overdue":                                      true,
	"dbc:warn-dbc-no-bkp-plus-maint":                                     true,
	"dbi-snap:multi-orphan-unenc-snap":                                   true,
	"dbi-snap:orphan-deleted-db-snap":                                    true,
	"dbi-snap:rds:retention-test-2026-03-25":                             true,
	"dbi:maint-dbi-scheduled":                                            true,
	"ddb:audit-pitr-off":                                                 true,
	"eb-rule:a9s-demo-s3-events-rule":                                    true,
	"eb-rule:acme-api-deploy-trigger":                                    true,
	"eb-rule:cost-anomaly-detector":                                      true,
	"eb-rule:eb-rule-disabled-with-targets":                              true,
	"eb-rule:eb-rule-no-targets":                                         true,
	"eb-rule:ec2-state-change-handler":                                   true,
	"eb-rule:ecr-api-service-scan-complete":                              true,
	"eb-rule:ecs-acme-services-task-state-change":                        true,
	"eb-rule:nightly-db-backup":                                          true,
	"eb-rule:nightly-order-fulfillment-trigger":                          true,
	"eb-rule:staging-cleanup-rule":                                       true,
	"eb:e-acmeebred":                                                     true,
	"eb:e-acmelegacy":                                                    true,
	"eb:e-acmestagapi":                                                   true,
	"eb:e-acmetermed":                                                    true,
	"ec2:i-0a1b2c3d4e5f60009":                                            true,
	"ec2:i-0a1b2c3d4e5f60010":                                            true,
	"ecr:acme/api-service":                                               true,
	"ecs-svc:acme-svc-degraded":                                          true,
	"ecs-svc:order-worker":                                               true,
	"ecs-task:a1b2c3d4e5f6a1b2c3d4e5f6":                                  true,
	"ecs-task:a7b8c9d0e1f2a7b8c9d0e1f2":                                  true,
	"ecs-task:e5f6a1b2c3d4e5f601020304":                                  true,
	"ecs-task:f6a1b2c3d4e5f60102030405":                                  true,
	"ecs:acme-services":                                                  true,
	"efs:fs-0healthymtdown001":                                           true,
	"efs:fs-0warncreating0001":                                           true,
	"efs:fs-0warndeleting0001":                                           true,
	"efs:fs-0warnupdmtdown001":                                           true,
	"eks:acme-degraded-prod":                                             true,
	"glue:glue-error-run":                                                true,
	"iam-group:empty-group":                                              true,
	"iam-group:readonly":                                                 true,
	"iam-user:bob.smith":                                                 true,
	"igw:igw-0unattached111111c":                                         true,
	"kms:11111111-1111-1111-1111-111111111111":                           true,
	"kms:2f7e9a5b-8c1d-4e3f-9a0b-1c2d3e4f5a6b":                           true,
	"kms:a1b2c3d4-5678-90ab-cdef-111111111111":                           true,
	"kms:a9s-demo-s3-key":                                                true,
	"kms:acme-opensearch-key":                                            true,
	"kms:acme-prod-master-key":                                           true,
	"kms:arn:aws:kms:us-east-1:123456789012:key/ami-ebs-boot-volume-key": true,
	"kms:b2c3d4e5-6789-01ab-cdef-222222222222":                           true,
	"kms:efs-prod-app-data-key":                                          true,
	"kms:f6a7b8c9-def0-3456-789a-ccddeeff0011":                           true,
	"kms:kms-redshift-1":                                                 true,
	"kms:kms-redshift-2":                                                 true,
	"kms:orders-prod-cmk-0001":                                           true,
	"nat:nat-0deleted11111111f":                                          true,
	"opensearch:acme-metrics":                                            true,
	"opensearch:acme-product-search":                                     true,
	"opensearch:acme-search-alpha":                                       true,
	"opensearch:legacy-analytics":                                        true,
	"pipeline:acme-frontend-deploy":                                      true,
	"policy:wildcard-allow-policy":                                       true,
	"r53:/hostedzone/Z1234567890ABCDEFGHIJ":                              true,
	"role:AWSBackupDefaultServiceRole":                                   true,
	"role:AWSReservedSSO_AdminAccess_3c4d5e6f7a8b9c0d":                   true,
	"role:AcmeBackupRoleProd":                                            true,
	"role:CiBuildRole":                                                   true,
	"role:DataPipelineRole":                                              true,
	"role:KarpenterNodeRole":                                             true,
	"role:a9s-demo-s3-access-role":                                       true,
	"role:acme-backup-service":                                           true,
	"role:acme-ci-deploy-role":                                           true,
	"role:acme-cloudtrail-cwlogs-role":                                   true,
	"role:acme-config-rule":                                              true,
	"role:acme-dms-service":                                              true,
	"role:acme-ec2-instance-profile":                                     true,
	"role:acme-ec2-instance-role":                                        true,
	"role:acme-ecs-task-exec":                                            true,
	"role:acme-eks-cluster-role":                                         true,
	"role:acme-eks-node-role":                                            true,
	"role:acme-eventbridge-invoke":                                       true,
	"role:acme-firehose-delivery":                                        true,
	"role:acme-glue-role":                                                true,
	"role:acme-guardduty-service":                                        true,
	"role:acme-kms-admin":                                                true,
	"role:acme-lambda-execution":                                         true,
	"role:acme-rds-monitoring":                                           true,
	"role:acme-s3-replication":                                           true,
	"role:acme-sagemaker-exec":                                           true,
	"role:acme-shield-response":                                          true,
	"role:acme-ssm-managed":                                              true,
	"role:acme-step-functions":                                           true,
	"role:acme-xray-daemon":                                              true,
	"role:anyone-can-assume-role":                                        true,
	"role:arn:aws:iam::123456789012:role/eks-gpu-node-role":              true,
	"role:arn:aws:iam::123456789012:role/eks-node-role":                  true,
	"role:arn:aws:iam::123456789012:role/service-role/acme-lambda-execution": true,
	"role:ci-runner":                                               true,
	"role:deploy-bot":                                              true,
	"role:eks-checkout-svc-sa":                                     true,
	"role:prod-ci-deploy-role":                                     true,
	"role:rds-enhanced-monitoring":                                 true,
	"role:rds-monitoring-role":                                     true,
	"role:redshift-copy-role":                                      true,
	"role:redshift-reporting-copy-role":                            true,
	"role:redshift-unload-role":                                    true,
	"role:wildcard-trust-role":                                     true,
	"s3:a9s-demo-multifail-pab":                                    true,
	"s3:a9s-demo-nilcfg":                                           true,
	"s3:a9s-demo-nopab":                                            true,
	"s3:a9s-demo-partial-pab":                                      true,
	"secrets:dev/deprecated/old-webhook-key":                       true,
	"sfn:payment-validation":                                       true,
	"sns:arn:aws:sns:us-east-1:123456789012:staging-deploy-alerts": true,
	"sqs:a9s-demo-s3-dlq":                                          true,
	"sqs:data-pipeline-dlq":                                        true,
	"sqs:email-notification-queue":                                 true,
	"sqs:webhook-ingest-queue.fifo":                                true,
	"tg:acme-web-tg":                                               true,
	"tgw:tgw-0deleted11111111e":                                    true,
	"vpc:vpc-0abc123def456789a":                                    true,
	"vpc:vpc-0def456789abc123d":                                    true,
	"vpc:vpc-0default00000000":                                     true,
	"vpc:vpc-0efs0prod0000001":                                     true,
	"vpc:vpc-demo-a":                                               true,
	"vpc:vpc-prod-main":                                            true,
	"vpce:vpce-0deleted111111111f":                                 true,
	"waf:a1b2c3d4-5678-90ab-cdef-333333333333":                     true,
}

// findingsDerivedColor computes the "worst finding wins" color implied by
// res.Findings using resource.ColorFromSeverity's severity->color table:
// SevBroken > SevWarn > SevDim > (no findings, or only SevOK/other) ->
// ColorHealthy. Unlike resource.ColorFromWave1, this does NOT filter by
// Source=="wave1" — it considers every Finding on the resource (Wave-1
// seeded or Wave-2 merged), because the owner's invariant is "color derives
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
// standing OWNER RULE gate: for every registered type with a Wave-1 Fetcher,
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
