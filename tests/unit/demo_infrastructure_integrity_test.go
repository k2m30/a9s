package unit_test

// demo_infrastructure_integrity_test.go — single source of truth for demo fixture contracts.
//
// This file supersedes the scattered tests in:
//   - qa_demo_completeness_test.go (TestDemoFieldCompleteness, TestDemoCrossReference, TestDemoReverseRelationship)
//   - demo_fixture_integrity_issue189_test.go (TestIssue189_*)
//
// The five parts below define the complete contract that demo fixtures must satisfy:
//
//   Part 1: Every registered resource type has non-empty fixtures with non-nil RawStructs.
//   Part 2: fixture[0] of every type has every detail-view field path resolve non-empty.
//   Part 3: Every navigable field on every fixture resolves to an ID that exists in the target type.
//   Part 4: Every registered related-demo checker returns Count > 0 on fixture[0].
//   Part 5: Named cross-reference constants are self-consistent across fixture sets.
//
// If a subtest fails, FIX THE FIXTURES — not this file.

import (
	"strings"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws" // registers all resource types, related defs, navigable fields
	"github.com/k2m30/a9s/v3/internal/config"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/fieldpath"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// allFixtures returns all demo fixtures keyed by short name.
// It calls t.Fatal if any registered type has no fixtures.
func allFixtures(t *testing.T) map[string][]resource.Resource {
	t.Helper()
	out := make(map[string][]resource.Resource)
	for _, rt := range resource.AllResourceTypes() {
		list, ok := demo.GetResources(rt.ShortName)
		if !ok || len(list) == 0 {
			t.Fatalf("Part 1 violation: demo fixtures missing for resource type %q", rt.ShortName)
		}
		out[rt.ShortName] = list
	}
	return out
}

// fixtureIDs returns a set of IDs for the given fixture slice.
func fixtureIDs(list []resource.Resource) map[string]bool {
	m := make(map[string]bool, len(list))
	for _, r := range list {
		m[r.ID] = true
	}
	return m
}

// TestDemoInfrastructureIntegrity is the single top-level test for all five parts.
func TestDemoInfrastructureIntegrity(t *testing.T) {
	// Load once; subtests share this map (read-only after construction).
	fixtures := allFixtures(t)

	// -------------------------------------------------------------------------
	// Part 1: All fixtures present and non-nil RawStructs
	// -------------------------------------------------------------------------
	t.Run("Part1_AllFixturesPresentAndNonNilRawStructs", func(t *testing.T) {
		for _, rt := range resource.AllResourceTypes() {
			t.Run(rt.ShortName, func(t *testing.T) {
				list := fixtures[rt.ShortName] // already validated non-empty by allFixtures
				for i, r := range list {
					if r.RawStruct == nil {
						t.Errorf("%s fixture[%d] ID=%s has nil RawStruct", rt.ShortName, i, r.ID)
					}
				}
			})
		}
	})

	// -------------------------------------------------------------------------
	// Part 2: All detail view fields populated on fixture[0]
	// -------------------------------------------------------------------------
	//
	// Fields that are legitimately empty in specific circumstances, with reasons:
	//
	//   ec2/InstanceLifecycle     — empty for on-demand instances; only set for spot/scheduled
	//   ct-events/AccessKeyId     — empty for AssumedRole events; AWS SDK Event.AccessKeyId is nil
	//                               when the call uses a session token rather than a direct key
	//
	conditionallyEmptyFields := map[string]map[string]string{
		"ec2": {
			"InstanceLifecycle": "empty for on-demand instances; only set for spot or scheduled",
		},
		"ct-events": {
			"AccessKeyId": "empty for AssumedRole events; AWS SDK Event.AccessKeyId is nil for session-based calls",
		},
	}

	t.Run("Part2_AllDetailFieldsPopulatedOnFixture0", func(t *testing.T) {
		cfg := config.DefaultConfig()
		for _, rt := range resource.AllResourceTypes() {
			t.Run(rt.ShortName, func(t *testing.T) {
				vd := config.GetViewDef(cfg, rt.ShortName)
				if len(vd.Detail) == 0 {
					t.Skipf("no detail view config for %q — skip", rt.ShortName)
				}
				fix0 := fixtures[rt.ShortName][0]
				if fix0.RawStruct == nil {
					t.Fatalf("%s fixture[0] has nil RawStruct — cannot verify field paths", rt.ShortName)
				}
				exceptions := conditionallyEmptyFields[rt.ShortName]
				for _, path := range vd.Detail {
					if reason, skip := exceptions[path]; skip {
						t.Logf("SKIP %s/%s: %s", rt.ShortName, path, reason)
						continue
					}
					t.Run(path, func(t *testing.T) {
						val := fieldpath.ExtractSubtree(fix0.RawStruct, path)
						if strings.TrimSpace(val) == "" {
							t.Errorf(
								"%s fixture[0] (ID=%q): detail field path %q resolved to empty via ExtractSubtree — populate in RawStruct",
								rt.ShortName, fix0.ID, path,
							)
						}
					})
				}
			})
		}
	})

	// -------------------------------------------------------------------------
	// Part 3: All navigable fields resolve to real fixture IDs
	//
	// Note: fieldpath.ExtractSubtree may return newline-separated values when
	// the path resolves to a slice field (e.g. SecurityGroups.GroupId).
	// Each line is treated as a separate ID to look up.
	// -------------------------------------------------------------------------
	t.Run("Part3_NavigableFieldsResolveToRealFixtureIDs", func(t *testing.T) {
		for _, rt := range resource.AllResourceTypes() {
			navFields := resource.GetNavigableFields(rt.ShortName)
			if len(navFields) == 0 {
				continue
			}
			t.Run(rt.ShortName, func(t *testing.T) {
				for _, nf := range navFields {
					t.Run(nf.FieldPath+"->"+nf.TargetType, func(t *testing.T) {
						targetList, ok := fixtures[nf.TargetType]
						if !ok || len(targetList) == 0 {
							t.Fatalf("navigable field %s.%s targets %q but that type has no fixtures",
								rt.ShortName, nf.FieldPath, nf.TargetType)
						}
						targetIDs := fixtureIDs(targetList)
						list := fixtures[rt.ShortName]
						for i, r := range list {
							if r.RawStruct == nil {
								t.Errorf("%s fixture[%d] ID=%s has nil RawStruct", rt.ShortName, i, r.ID)
								continue
							}
							raw := fieldpath.ExtractSubtree(r.RawStruct, nf.FieldPath)
							// ExtractSubtree returns newline-separated values for slice paths.
							resolvedIDs := strings.Split(raw, "\n")
							hasValue := false
							for _, val := range resolvedIDs {
								val = strings.TrimSpace(val)
								if val == "" || val == "-" || strings.EqualFold(val, "<nil>") {
									continue
								}
								hasValue = true
								if !targetIDs[val] {
									t.Errorf("%s fixture[%d] ID=%s: navigable field %q = %q but no %s fixture has that ID",
										rt.ShortName, i, r.ID, nf.FieldPath, val, nf.TargetType)
								}
							}
							if !hasValue {
								t.Errorf("%s fixture[%d] ID=%s: navigable field path %q resolved to empty",
									rt.ShortName, i, r.ID, nf.FieldPath)
							}
						}
					})
				}
			})
		}
	})

	// -------------------------------------------------------------------------
	// Part 4: All related-demo checkers return Count > 0 on fixture[0]
	//
	// Known exceptions where all-zero results are by design (not a fixture gap):
	//
	//   acm — ACM related defs have Checker: nil (pure stubs); attachment info
	//          is only available via NeedsTargetCache pattern which is not
	//          implemented for demo mode. Count=0 is the correct demo behavior.
	//
	// If you add a new related demo checker that legitimately returns all zeros,
	// document it here with a reason before adding to the exceptions map.
	// -------------------------------------------------------------------------
	knownAllZeroRelatedDemos := map[string]string{
		"acm": "related defs have Checker: nil (stubs); ACM attachment data not available in demo mode",
	}

	t.Run("Part4_RelatedDemoCheckersReturnHitsOnFixture0", func(t *testing.T) {
		for _, rt := range resource.AllResourceTypes() {
			checker := resource.GetRelatedDemo(rt.ShortName)
			if checker == nil {
				continue
			}
			if reason, exempt := knownAllZeroRelatedDemos[rt.ShortName]; exempt {
				t.Logf("SKIP %s Part4: %s", rt.ShortName, reason)
				continue
			}
			t.Run(rt.ShortName, func(t *testing.T) {
				fix0 := fixtures[rt.ShortName][0]
				results := checker(fix0)
				if len(results) == 0 {
					t.Errorf("%s: related demo checker returned zero RelatedCheckResults on fixture[0] (ID=%s)",
						rt.ShortName, fix0.ID)
					return
				}
				allZero := true
				for _, res := range results {
					if res.Count > 0 {
						allZero = false
					}
				}
				if allZero {
					t.Errorf("%s: related demo checker returned all Count=0 results on fixture[0] (ID=%s) — cross-link the infrastructure",
						rt.ShortName, fix0.ID)
				}
				// Every returned ResourceID must exist in the target type's fixtures.
				for _, res := range results {
					if len(res.ResourceIDs) == 0 {
						continue
					}
					targetList := fixtures[res.TargetType]
					if len(targetList) == 0 {
						t.Errorf("%s: related demo checker references target type %q which has no fixtures",
							rt.ShortName, res.TargetType)
						continue
					}
					targetIDs := fixtureIDs(targetList)
					for _, id := range res.ResourceIDs {
						if !targetIDs[id] {
							t.Errorf("%s -> %s: related demo checker returned ID=%q but no %s fixture has that ID",
								rt.ShortName, res.TargetType, id, res.TargetType)
						}
					}
				}
			})
		}
	})

	// -------------------------------------------------------------------------
	// Part 5: Cross-reference constants consistency
	// -------------------------------------------------------------------------
	//
	// These are named constants that the demo fixtures use internally.  If any
	// constant changes, the cross-links break silently. Asserting them here
	// means the test suite detects the drift immediately.

	t.Run("Part5_CrossReferenceConstantsConsistent", func(t *testing.T) {

		// Helper: look up a single fixture by ID.
		requireFixture := func(t *testing.T, shortName, id string) resource.Resource {
			t.Helper()
			for _, r := range fixtures[shortName] {
				if r.ID == id {
					return r
				}
			}
			t.Fatalf("%s: no fixture found with ID=%q", shortName, id)
			return resource.Resource{}
		}

		// Helper: assert that a set of IDs all exist in a fixture set.
		requireIDs := func(t *testing.T, shortName string, ids []string) {
			t.Helper()
			known := fixtureIDs(fixtures[shortName])
			for _, id := range ids {
				if !known[id] {
					t.Errorf("%s: expected fixture ID=%q not found", shortName, id)
				}
			}
		}

		// Helper: check whether any fixture of a type contains needle anywhere.
		anyContains := func(shortName, needle string) bool {
			for _, r := range fixtures[shortName] {
				if strings.Contains(r.ID, needle) || strings.Contains(r.Name, needle) {
					return true
				}
				for _, v := range r.Fields {
					if strings.Contains(v, needle) {
						return true
					}
				}
			}
			return false
		}

		// -- ECS task definition ARN is populated and well-formed on fixture[0] --
		//
		// fixture[0] is the "api-gateway" task (task-definition/api-gateway:12).
		// We assert non-empty and that it looks like an ECS task-definition ARN.
		t.Run("ecs-task-definition-arn", func(t *testing.T) {
			ecsTaskFixtures := fixtures["ecs-task"]
			if len(ecsTaskFixtures) == 0 {
				t.Fatal("no ecs-task fixtures")
			}
			fix0 := ecsTaskFixtures[0]
			taskDefARN := fix0.Fields["task_definition_arn"]
			if taskDefARN == "" {
				// Also try RawStruct extraction.
				if fix0.RawStruct != nil {
					taskDefARN = fieldpath.ExtractSubtree(fix0.RawStruct, "TaskDefinitionArn")
				}
			}
			if taskDefARN == "" {
				t.Errorf("ecs-task fixture[0] (ID=%s): TaskDefinitionArn is empty", fix0.ID)
				return
			}
			if !strings.Contains(taskDefARN, "task-definition/") {
				t.Errorf("ecs-task fixture[0] (ID=%s): TaskDefinitionArn=%q does not look like an ECS task definition ARN (missing 'task-definition/')",
					fix0.ID, taskDefARN)
			}
		})

		// -- ECS service → ELB: service fixture[0] load balancer name resolves -----
		t.Run("ecs-svc-elb-name", func(t *testing.T) {
			const prodELBName = "acme-prod-web"
			requireIDs(t, "elb", []string{prodELBName})
		})

		// -- TG → ELB: every TG fixture's LoadBalancerArns resolves to ELB fixture ---
		t.Run("tg-lb-arns-resolve-to-elb", func(t *testing.T) {
			elbByName := make(map[string]bool)
			for _, r := range fixtures["elb"] {
				elbByName[r.ID] = true
			}
			for _, tg := range fixtures["tg"] {
				// The ELB reference appears in tg.Fields["load_balancer_arns"] or similar.
				// lbNameFromARN matches the helper in the existing tests.
				lbField := tg.Fields["load_balancer_arns"]
				if lbField == "" {
					continue
				}
				// lbField may be a comma-separated list of ARNs or a single ARN.
				for raw := range strings.SplitSeq(lbField, ",") {
					raw = strings.TrimSpace(raw)
					if raw == "" {
						continue
					}
					lbName := lbNameFromARNIntegrity(raw)
					if !elbByName[lbName] {
						t.Errorf("tg %s: LoadBalancerArns entry %q resolves to name %q but no elb fixture has that ID",
							tg.ID, raw, lbName)
					}
				}
			}
		})

		// -- EC2 → VPC: every EC2 fixture's vpc_id resolves to a VPC fixture ------
		t.Run("ec2-vpc-ids-resolve", func(t *testing.T) {
			vpcIDs := fixtureIDs(fixtures["vpc"])
			for _, r := range fixtures["ec2"] {
				vpcID := r.Fields["vpc_id"]
				if vpcID == "" {
					continue // terminated instances may have no VPC
				}
				if !vpcIDs[vpcID] {
					t.Errorf("ec2 %s: vpc_id=%q not found in vpc fixtures", r.ID, vpcID)
				}
			}
		})

		// -- EC2 → SG: EC2 fixtures reference known SG IDs that exist -------------
		t.Run("ec2-sg-ids-exist", func(t *testing.T) {
			requireIDs(t, "sg", []string{
				"sg-0aaa111111111111a", // prodWebALBSGID
				"sg-0bbb222222222222b", // prodAPIInternalSGID
				"sg-0ccc333333333333c", // prodRDSSGID
				"sg-0ddd444444444444d", // prodDBProxySGID
			})
		})

		// -- EKS node groups → EKS cluster: ClusterName field resolves ------------
		t.Run("ng-cluster-name-resolves-to-eks", func(t *testing.T) {
			eksNames := make(map[string]bool)
			for _, r := range fixtures["eks"] {
				eksNames[r.ID] = true
				if r.Name != "" {
					eksNames[r.Name] = true
				}
			}
			for _, ng := range fixtures["ng"] {
				clusterName := ng.Fields["cluster_name"]
				if clusterName == "" {
					t.Errorf("ng %s: cluster_name field is empty", ng.ID)
					continue
				}
				if !eksNames[clusterName] {
					t.Errorf("ng %s: cluster_name=%q not found in eks fixture IDs or names", ng.ID, clusterName)
				}
			}
		})

		// -- RDS → subnet group: dbi fixture[0] has DBSubnetGroup populated -------
		t.Run("rds-subnet-group-populated", func(t *testing.T) {
			dbiFixtures := fixtures["dbi"]
			if len(dbiFixtures) == 0 {
				t.Fatal("no dbi fixtures")
			}
			fix0 := dbiFixtures[0]
			if fix0.RawStruct == nil {
				t.Fatal("dbi fixture[0] has nil RawStruct")
			}
			subnetGroup := fieldpath.ExtractSubtree(fix0.RawStruct, "DBSubnetGroup.DBSubnetGroupName")
			if strings.TrimSpace(subnetGroup) == "" {
				t.Errorf("dbi fixture[0] (ID=%s): DBSubnetGroup.DBSubnetGroupName resolved to empty — populate DBSubnetGroup in RawStruct",
					fix0.ID)
			}
		})

		// -- Lambda → log group: lambda fixture[0] has LogGroup populated ---------
		t.Run("lambda-log-group-populated", func(t *testing.T) {
			lambdaFixtures := fixtures["lambda"]
			if len(lambdaFixtures) == 0 {
				t.Fatal("no lambda fixtures")
			}
			fix0 := lambdaFixtures[0]
			if fix0.RawStruct == nil {
				t.Fatal("lambda fixture[0] has nil RawStruct")
			}
			// LogGroup lives in LoggingConfig.LogGroup on the AWS SDK struct.
			logGroup := fieldpath.ExtractSubtree(fix0.RawStruct, "LoggingConfig.LogGroup")
			if strings.TrimSpace(logGroup) == "" {
				// Fallback: check Fields map for log_group key.
				logGroup = strings.TrimSpace(fix0.Fields["log_group"])
			}
			if logGroup == "" {
				t.Errorf("lambda fixture[0] (ID=%s): no LogGroup found in RawStruct.LoggingConfig.LogGroup or Fields[\"log_group\"]",
					fix0.ID)
				return
			}
			logIDs := fixtureIDs(fixtures["logs"])
			if !logIDs[logGroup] {
				t.Errorf("lambda fixture[0] (ID=%s): LogGroup=%q not found in logs fixtures", fix0.ID, logGroup)
			}
		})

		// -- VPC IDs: prodVPCID and stagingVPCID must exist -----------------------
		t.Run("vpc-ids-exist", func(t *testing.T) {
			requireIDs(t, "vpc", []string{
				"vpc-0abc123def456789a", // prodVPCID
				"vpc-0def456789abc123d", // stagingVPCID
			})
		})

		// -- Subnet IDs: all subnets referenced by EC2/ASG/EKS must exist --------
		t.Run("subnet-ids-exist", func(t *testing.T) {
			requireIDs(t, "subnet", []string{
				"subnet-0aaa111111111111a",
				"subnet-0bbb222222222222b",
				"subnet-0ccc333333333333c",
				"subnet-0ddd444444444444d",
				"subnet-0eee555555555555e",
				"subnet-0fff666666666666f",
			})
		})

		// -- AMI IDs referenced by EC2 fixtures must exist -----------------------
		t.Run("ami-ids-exist", func(t *testing.T) {
			requireIDs(t, "ami", []string{
				"ami-0a1b2c3d4e5f60001",
				"ami-0a1b2c3d4e5f60002",
				"ami-0a1b2c3d4e5f60003",
			})
		})

		// -- ACM cert ARNs referenced by ELB must exist --------------------------
		t.Run("acm-cert-arns-exist", func(t *testing.T) {
			requireIDs(t, "acm", []string{
				"arn:aws:acm:us-east-1:123456789012:certificate/a1b2c3d4-5678-90ab-cdef-111111111111",
				"arn:aws:acm:us-east-1:123456789012:certificate/b2c3d4e5-6789-01ab-cdef-222222222222",
			})
		})

		// -- Alarm → SNS: alarm-notifications SNS topic must exist ---------------
		t.Run("alarm-sns-topic-exists", func(t *testing.T) {
			const alarmSNSARN = "arn:aws:sns:us-east-1:123456789012:alarm-notifications"
			requireIDs(t, "sns", []string{alarmSNSARN})
		})

		// -- EBS volumes referenced by EC2 related-demo must exist ---------------
		t.Run("ebs-volume-ids-exist", func(t *testing.T) {
			requireIDs(t, "ebs", []string{
				"vol-0a1b2c3d4e5f60001",
				"vol-0a1b2c3d4e5f60002",
			})
		})

		// -- EBS snapshot IDs referenced by EC2 related-demo must exist ----------
		t.Run("ebs-snap-ids-exist", func(t *testing.T) {
			requireIDs(t, "ebs-snap", []string{
				"snap-0a1b2c3d4e5f60001",
				"snap-0a1b2c3d4e5f60002",
			})
		})

		// -- EC2 target group (acme-web-tg) must exist ---------------------------
		t.Run("ec2-tg-exists", func(t *testing.T) {
			requireIDs(t, "tg", []string{"acme-web-tg"})
		})

		// -- EC2 ASG (acme-web-prod-asg) must exist ------------------------------
		t.Run("ec2-asg-exists", func(t *testing.T) {
			requireIDs(t, "asg", []string{"acme-web-prod-asg"})
		})

		// -- EC2 EIP (eipalloc-0aaa111111111111a) must exist ---------------------
		t.Run("ec2-eip-exists", func(t *testing.T) {
			requireIDs(t, "eip", []string{"eipalloc-0aaa111111111111a"})
		})

		// -- EC2 alarm IDs referenced by related-demo must exist -----------------
		t.Run("ec2-alarm-ids-exist", func(t *testing.T) {
			requireIDs(t, "alarm", []string{
				"api-high-error-rate",
				"rds-cpu-utilization",
			})
		})

		// -- AMI related-demo references a specific EC2 and snapshot -------------
		t.Run("ami-ec2-fixture-exists", func(t *testing.T) {
			requireFixture(t, "ec2", "i-0a1b2c3d4e5f60001")
		})
		t.Run("ami-snap-fixture-exists", func(t *testing.T) {
			requireFixture(t, "ebs-snap", "snap-0a1b2c3d4e5f60001")
		})

		// -- EC2 nodegroup (general-pool) must exist in ng fixtures --------------
		t.Run("ec2-nodegroup-fixture-exists", func(t *testing.T) {
			requireIDs(t, "ng", []string{"general-pool"})
		})

		// -- Lambda SQS event source queue must exist ----------------------------
		t.Run("lambda-sqs-event-source-exists", func(t *testing.T) {
			const orderQueueName = "order-processing-queue"
			if !anyContains("sqs", orderQueueName) {
				t.Errorf("expected sqs fixture containing %q (lambda process-orders event source) but none found", orderQueueName)
			}
		})

		// -- ELB DNS name (prodELBDNS) must appear in ELB fixtures ---------------
		t.Run("r53-elb-dns-exists-in-elb", func(t *testing.T) {
			const prodELBDNS = "acme-prod-web-1234567890.us-east-1.elb.amazonaws.com"
			if !anyContains("elb", prodELBDNS) {
				t.Errorf("elb fixtures do not contain DNS=%q (referenced by r53 alias records)", prodELBDNS)
			}
		})

		// -- CloudFront origin S3 bucket must exist ------------------------------
		t.Run("cf-origin-s3-bucket-exists", func(t *testing.T) {
			requireIDs(t, "s3", []string{"webapp-assets-prod"})
		})

		// -- prodVPCID appears in EC2 and subnet fixtures -------------------------
		t.Run("prodVPCID-appears-in-ec2-and-subnet", func(t *testing.T) {
			const prodVPCID = "vpc-0abc123def456789a"
			if !anyContains("ec2", prodVPCID) {
				t.Errorf("prodVPCID %q not found in any ec2 fixture field", prodVPCID)
			}
			if !anyContains("subnet", prodVPCID) {
				t.Errorf("prodVPCID %q not found in any subnet fixture field", prodVPCID)
			}
		})
	})
}

// lbNameFromARNIntegrity extracts the load balancer name from an ALB/NLB ARN.
// Example: arn:aws:elasticloadbalancing:...:loadbalancer/app/acme-prod-web/abc → "acme-prod-web"
func lbNameFromARNIntegrity(arn string) string {
	parts := strings.Split(arn, "/")
	if len(parts) >= 3 {
		return parts[len(parts)-2]
	}
	idx := strings.LastIndex(arn, "/")
	if idx >= 0 && idx < len(arn)-1 {
		return arn[idx+1:]
	}
	return arn
}
