// aws_efs_related_test.go — graph-root related-pivot tests for EFS.
//
// Tests the CONTRACT from docs/resources/efs.md §2 and docs/resources/efs-impl-plan.md §2.
// One graph-root test per §2 pivot that has count shown: yes.
// Expected counts (per impl-plan): alarm=2, backup=2, cfn=1, ecs-task=2,
// eni=3, kms=1, lambda=2, sg=2, subnet=3, vpc=1, ec2=0 (intentional per spec §5).
//
// efsCheckerByTarget is the shared helper consumed by:
//   - aws_efs_related_extra_test.go (checkEFSAlarm, checkEFSEC2, checkEFSENI, checkEFSVPC)
//   - aws_efs_related_wave2_test.go (checkEFSSG, checkEFSSubnet)
//   - aws_wave5_related_test.go    (checkEFSLambda)
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fakes"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Shared helper: efsCheckerByTarget
//
// Used by this file AND sibling files aws_efs_related_extra_test.go,
// aws_efs_related_wave2_test.go, and aws_wave5_related_test.go.
// MUST NOT be removed or renamed.
// ---------------------------------------------------------------------------

func efsCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("efs") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("efs related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("efs related checker for %s not found", target)
	return nil
}

// ---------------------------------------------------------------------------
// Graph-root source resource
// ---------------------------------------------------------------------------

func efsGraphRootSource() resource.Resource {
	fix := fixtures.NewEFSFixtures()
	var fs efstypes.FileSystemDescription
	for _, f := range fix.FileSystems {
		if aws.ToString(f.FileSystemId) == fixtures.ProdEFSID {
			fs = f
			break
		}
	}
	return resource.Resource{
		ID:        fixtures.ProdEFSID,
		Name:      "prod-app-data",
		RawStruct: fs,
	}
}

// ---------------------------------------------------------------------------
// Cache builders from fixture data
// ---------------------------------------------------------------------------

func efsAlarmCache() resource.ResourceCache {
	fix := fixtures.NewCloudWatchFixtures()
	var rs []resource.Resource
	for _, a := range fix.Alarms {
		name := aws.ToString(a.AlarmName)
		rs = append(rs, resource.Resource{ID: name, Name: name, RawStruct: a})
	}
	return resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: rs},
	}
}

func efsCFNCache() resource.ResourceCache {
	fix := fixtures.NewCFNFixtures()
	var rs []resource.Resource
	for _, s := range fix.Stacks {
		name := aws.ToString(s.StackName)
		rs = append(rs, resource.Resource{ID: name, Name: name, RawStruct: s})
	}
	return resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: rs},
	}
}

func efsKMSCache() resource.ResourceCache {
	fix := fixtures.NewKMSFixtures()
	var rs []resource.Resource
	for id, k := range fix.Keys {
		rs = append(rs, resource.Resource{ID: id, Name: id, RawStruct: *k})
	}
	return resource.ResourceCache{
		"kms": resource.ResourceCacheEntry{Resources: rs},
	}
}

func efsENICache() resource.ResourceCache {
	fix := fixtures.NewEC2Fixtures()
	var rs []resource.Resource
	for _, ni := range fix.NetworkInterfaces {
		id := aws.ToString(ni.NetworkInterfaceId)
		rs = append(rs, resource.Resource{ID: id, Name: id, RawStruct: ni})
	}
	return resource.ResourceCache{
		"eni": resource.ResourceCacheEntry{Resources: rs},
	}
}

func efsLambdaCache() resource.ResourceCache {
	fix := fixtures.NewLambdaFixtures()
	var rs []resource.Resource
	for _, fn := range fix.Functions {
		name := aws.ToString(fn.FunctionName)
		rs = append(rs, resource.Resource{ID: name, Name: name, RawStruct: fn})
	}
	return resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: rs},
	}
}

// efsECSTaskCacheWithEFSIDs builds an ecs-task cache where two tasks carry
// Fields["efs_file_system_ids"] = ProdEFSID (the new Phase-7 contract).
// The task IDs are the ECS task ARNs from the fixture tasks that use
// api-gateway and web-frontend task definitions (which mount ProdEFSID).
func efsECSTaskCacheWithEFSIDs() resource.ResourceCache {
	// Two tasks that reference ProdEFSID via their task definitions.
	taskA := resource.Resource{
		ID:   "arn:aws:ecs:us-east-1:123456789012:task/acme-services/a1b2c3d4e5f6a1b2c3d4e5f6",
		Name: "a1b2c3d4e5f6a1b2c3d4e5f6",
		Fields: map[string]string{
			"efs_file_system_ids": fixtures.ProdEFSID,
		},
	}
	taskB := resource.Resource{
		ID:   "arn:aws:ecs:us-east-1:123456789012:task/acme-services/b2c3d4e5f6a1b2c3d4e5f601",
		Name: "b2c3d4e5f6a1b2c3d4e5f601",
		Fields: map[string]string{
			"efs_file_system_ids": fixtures.ProdEFSID,
		},
	}
	// Third task that does NOT reference ProdEFSID.
	taskC := resource.Resource{
		ID:   "arn:aws:ecs:us-east-1:123456789012:task/acme-batch/d4e5f6a1b2c3d4e5f6010203",
		Name: "d4e5f6a1b2c3d4e5f6010203",
		Fields: map[string]string{
			"efs_file_system_ids": "fs-0000000000000000",
		},
	}
	return resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{
			Resources: []resource.Resource{taskA, taskB, taskC},
		},
	}
}

// ---------------------------------------------------------------------------
// TEST: TestRelated_EFS_KMS_GraphRoot
//
// GIVEN: graph-root EFS (ProdEFSID) with KmsKeyId = ProdEFSKmsKeyARN.
// THEN:  checkEFSKMS returns Count=1, ResourceIDs contains ProdEFSKmsKeyID
//        (the bare key ID stripped from the ARN). The checker emits blindly;
//        the related-check orchestrator's lazy-add path populates the kms
//        cache with the referenced key (customer-managed or AWS-managed) at
//        dispatch time, so passing a nil cache here is the correct test
//        shape.
// ---------------------------------------------------------------------------

func TestRelated_EFS_KMS_GraphRoot(t *testing.T) {
	source := efsGraphRootSource()
	checker := efsCheckerByTarget(t, "kms")

	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (ProdEFSKmsKeyID); RawStruct = %+v", result.Count, source.RawStruct)
	}
	if len(result.ResourceIDs) < 1 {
		t.Fatalf("ResourceIDs is empty, want [%s]", fixtures.ProdEFSKmsKeyID)
	}
	if result.ResourceIDs[0] != fixtures.ProdEFSKmsKeyID {
		t.Errorf("ResourceIDs[0] = %q, want %q", result.ResourceIDs[0], fixtures.ProdEFSKmsKeyID)
	}
}

// ---------------------------------------------------------------------------
// TEST: TestRelated_EFS_CFN_GraphRoot
//
// GIVEN: graph-root EFS with aws:cloudformation:stack-name = ProdEFSCFNStackName.
// THEN:  checkEFSCFN returns Count=1, ResourceIDs contains ProdEFSCFNStackName.
// ---------------------------------------------------------------------------

func TestRelated_EFS_CFN_GraphRoot(t *testing.T) {
	source := efsGraphRootSource()
	checker := efsCheckerByTarget(t, "cfn")
	cache := efsCFNCache()

	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (ProdEFSCFNStackName %q)", result.Count, fixtures.ProdEFSCFNStackName)
	}
	found := false
	for _, id := range result.ResourceIDs {
		if id == fixtures.ProdEFSCFNStackName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ResourceIDs = %v, want it to contain %q", result.ResourceIDs, fixtures.ProdEFSCFNStackName)
	}
}

// ---------------------------------------------------------------------------
// TEST: TestRelated_EFS_Alarm_GraphRoot
//
// GIVEN: graph-root EFS; alarm cache contains ProdEFSAlarmAID and ProdEFSAlarmBID
//        (Namespace=AWS/EFS, Dimension FileSystemId=ProdEFSID).
// THEN:  checkEFSAlarm returns Count=2 and both alarm IDs in ResourceIDs.
// ---------------------------------------------------------------------------

func TestRelated_EFS_Alarm_GraphRoot(t *testing.T) {
	source := efsGraphRootSource()
	checker := efsCheckerByTarget(t, "alarm")
	cache := efsAlarmCache()

	result := checker(context.Background(), nil, source, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2 (both EFS alarms); ResourceIDs = %v", result.Count, result.ResourceIDs)
	}
	wantAlarms := []string{fixtures.ProdEFSAlarmAID, fixtures.ProdEFSAlarmBID}
	idSet := make(map[string]bool, len(result.ResourceIDs))
	for _, id := range result.ResourceIDs {
		idSet[id] = true
	}
	for _, want := range wantAlarms {
		if !idSet[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs)
		}
	}
}

// ---------------------------------------------------------------------------
// TEST: TestRelated_EFS_Lambda_GraphRoot
//
// GIVEN: graph-root EFS (ProdEFSID) with access points AP-A and AP-B.
//        Lambda cache contains ProdEFSLambdaAName and ProdEFSLambdaBName,
//        each with FileSystemConfigs referencing the respective AP ARNs.
//        Live EFS client (EFSFake) returns access points for ProdEFSID.
// THEN:  checkEFSLambda returns Count=2, both lambda names in ResourceIDs.
// ---------------------------------------------------------------------------

func TestRelated_EFS_Lambda_GraphRoot(t *testing.T) {
	source := efsGraphRootSource()
	checker := efsCheckerByTarget(t, "lambda")

	efsFake := fakes.NewEFS()
	clients := &awsclient.ServiceClients{EFS: efsFake}
	cache := efsLambdaCache()

	result := checker(context.Background(), clients, source, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2 (ProdEFSLambdaAName + ProdEFSLambdaBName); ResourceIDs = %v", result.Count, result.ResourceIDs)
	}
	wantLambdas := []string{fixtures.ProdEFSLambdaAName, fixtures.ProdEFSLambdaBName}
	idSet := make(map[string]bool, len(result.ResourceIDs))
	for _, id := range result.ResourceIDs {
		idSet[id] = true
	}
	for _, want := range wantLambdas {
		if !idSet[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs)
		}
	}
}

// ---------------------------------------------------------------------------
// TEST: TestRelated_EFS_Backup_GraphRoot
//
// GIVEN: graph-root EFS (ProdEFSARN); backup cache contains plans whose
//        Fields["resources"] CSV lists ProdEFSARN.
// THEN:  checkEFSBackup returns Count=2 and both plan IDs in ResourceIDs.
//
// The checker now reverse-scans the backup cache (ID-format matches the
// backup fetcher's Resource.ID=BackupPlanId). Recovery-point ARNs are not
// a valid target ID for the `backup` resource type — drill-through would
// land empty if the checker returned them. See 2026-04-24 bug.
// ---------------------------------------------------------------------------

func TestRelated_EFS_Backup_GraphRoot(t *testing.T) {
	source := efsGraphRootSource()
	checker := efsCheckerByTarget(t, "backup")

	// Build a backup cache with two plans that reference ProdEFSARN in their
	// resources CSV, and one plan that does NOT.
	cache := resource.ResourceCache{
		"backup": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{
				ID:   fixtures.HealthyDailyPlanID,
				Name: "plan-healthy-daily",
				Fields: map[string]string{
					"resources": fixtures.HealthyBucketARN + "," + fixtures.ProdEFSARN + "," + fixtures.OrdersProdARN,
				},
			},
			{
				ID:   fixtures.AppDataPlanID,
				Name: "plan-warning-partial",
				Fields: map[string]string{
					"resources": "arn:aws:dynamodb:us-east-1:123456789012:table/acme-app-sessions," + fixtures.ProdEFSARN,
				},
			},
			{
				ID:   fixtures.ProdCriticalPlanID,
				Name: "plan-broken-1failed",
				Fields: map[string]string{
					"resources": "arn:aws:rds:us-east-1:123456789012:db:acme-prod-secondary",
				},
			},
		}},
	}

	result := checker(context.Background(), nil, source, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2 (two plans protect the graph-root EFS); ResourceIDs = %v", result.Count, result.ResourceIDs)
	}
	wantPlanIDs := []string{fixtures.HealthyDailyPlanID, fixtures.AppDataPlanID}
	idSet := make(map[string]bool, len(result.ResourceIDs))
	for _, id := range result.ResourceIDs {
		idSet[id] = true
	}
	for _, want := range wantPlanIDs {
		if !idSet[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs)
		}
	}
}

// ---------------------------------------------------------------------------
// TEST: TestRelated_EFS_ENI_GraphRoot
//
// GIVEN: graph-root EFS; ENI cache contains 3 mount-target ENIs for ProdEFSID
//        (description "EFS mount target for <ProdEFSID>").
// THEN:  checkEFSENI returns Count=3 and all three ENI IDs in ResourceIDs.
// ---------------------------------------------------------------------------

func TestRelated_EFS_ENI_GraphRoot(t *testing.T) {
	source := efsGraphRootSource()
	checker := efsCheckerByTarget(t, "eni")
	cache := efsENICache()

	result := checker(context.Background(), nil, source, cache)

	if result.Count != 3 {
		t.Errorf("Count = %d, want 3 (ProdEFSEniA/B/C); ResourceIDs = %v", result.Count, result.ResourceIDs)
	}
	wantENIs := []string{fixtures.ProdEFSEniAID, fixtures.ProdEFSEniBID, fixtures.ProdEFSEniCID}
	idSet := make(map[string]bool, len(result.ResourceIDs))
	for _, id := range result.ResourceIDs {
		idSet[id] = true
	}
	for _, want := range wantENIs {
		if !idSet[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs)
		}
	}
}

// ---------------------------------------------------------------------------
// TEST: TestRelated_EFS_SG_GraphRoot
//
// GIVEN: graph-root EFS; ENI cache has 3 mount-target ENIs, each with SGs
//        ProdEFSSecurityGroupAID and ProdEFSSecurityGroupBID.
// THEN:  checkEFSSG returns Count=2 (deduplicated), both SG IDs in ResourceIDs.
// ---------------------------------------------------------------------------

func TestRelated_EFS_SG_GraphRoot(t *testing.T) {
	source := efsGraphRootSource()
	checker := efsCheckerByTarget(t, "sg")
	cache := efsENICache()

	result := checker(context.Background(), nil, source, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2 (ProdEFSSecurityGroupA/B); ResourceIDs = %v", result.Count, result.ResourceIDs)
	}
	wantSGs := []string{fixtures.ProdEFSSecurityGroupAID, fixtures.ProdEFSSecurityGroupBID}
	idSet := make(map[string]bool, len(result.ResourceIDs))
	for _, id := range result.ResourceIDs {
		idSet[id] = true
	}
	for _, want := range wantSGs {
		if !idSet[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs)
		}
	}
}

// ---------------------------------------------------------------------------
// TEST: TestRelated_EFS_Subnet_GraphRoot
//
// GIVEN: graph-root EFS; ENI cache has 3 mount-target ENIs in 3 subnets
//        (ProdEFSSubnetAID, ProdEFSSubnetBID, ProdEFSSubnetCID).
// THEN:  checkEFSSubnet returns Count=3, all three subnet IDs in ResourceIDs.
// ---------------------------------------------------------------------------

func TestRelated_EFS_Subnet_GraphRoot(t *testing.T) {
	source := efsGraphRootSource()
	checker := efsCheckerByTarget(t, "subnet")
	cache := efsENICache()

	result := checker(context.Background(), nil, source, cache)

	if result.Count != 3 {
		t.Errorf("Count = %d, want 3 (ProdEFSSubnetA/B/C); ResourceIDs = %v", result.Count, result.ResourceIDs)
	}
	wantSubnets := []string{fixtures.ProdEFSSubnetAID, fixtures.ProdEFSSubnetBID, fixtures.ProdEFSSubnetCID}
	idSet := make(map[string]bool, len(result.ResourceIDs))
	for _, id := range result.ResourceIDs {
		idSet[id] = true
	}
	for _, want := range wantSubnets {
		if !idSet[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs)
		}
	}
}

// ---------------------------------------------------------------------------
// TEST: TestRelated_EFS_VPC_GraphRoot
//
// GIVEN: graph-root EFS; ENI cache has 3 mount-target ENIs, all in ProdEFSVpcID.
// THEN:  checkEFSVPC returns Count=1 (deduplicated), ResourceIDs=[ProdEFSVpcID].
// ---------------------------------------------------------------------------

func TestRelated_EFS_VPC_GraphRoot(t *testing.T) {
	source := efsGraphRootSource()
	checker := efsCheckerByTarget(t, "vpc")
	cache := efsENICache()

	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (ProdEFSVpcID); ResourceIDs = %v", result.Count, result.ResourceIDs)
	}
	if len(result.ResourceIDs) < 1 {
		t.Fatalf("ResourceIDs is empty, want [%s]", fixtures.ProdEFSVpcID)
	}
	if result.ResourceIDs[0] != fixtures.ProdEFSVpcID {
		t.Errorf("ResourceIDs[0] = %q, want %q", result.ResourceIDs[0], fixtures.ProdEFSVpcID)
	}
}

// ---------------------------------------------------------------------------
// TEST: TestRelated_EFS_ECSTask_GraphRoot
//
// GIVEN: graph-root EFS (ProdEFSID); ecs-task cache contains 3 Resources:
//        taskA and taskB have Fields["efs_file_system_ids"]=ProdEFSID,
//        taskC has Fields["efs_file_system_ids"]="fs-0000000000000000".
// THEN:  checkEFSECSTask returns Count=2, ResourceIDs=[taskA.ID, taskB.ID].
//        (New Phase-7 contract: reads Fields["efs_file_system_ids"], NOT RawStruct.)
// ---------------------------------------------------------------------------

func TestRelated_EFS_ECSTask_GraphRoot(t *testing.T) {
	source := efsGraphRootSource()
	checker := efsCheckerByTarget(t, "ecs-task")
	cache := efsECSTaskCacheWithEFSIDs()

	result := checker(context.Background(), nil, source, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2 (taskA + taskB carry ProdEFSID); ResourceIDs = %v", result.Count, result.ResourceIDs)
	}
	wantTasks := []string{
		"arn:aws:ecs:us-east-1:123456789012:task/acme-services/a1b2c3d4e5f6a1b2c3d4e5f6",
		"arn:aws:ecs:us-east-1:123456789012:task/acme-services/b2c3d4e5f6a1b2c3d4e5f601",
	}
	idSet := make(map[string]bool, len(result.ResourceIDs))
	for _, id := range result.ResourceIDs {
		idSet[id] = true
	}
	for _, want := range wantTasks {
		if !idSet[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs)
		}
	}
}

// ---------------------------------------------------------------------------
// TEST: TestRelated_EFS_EC2_GraphRoot
// EFS→EC2 pivot was removed (2026-04-24): the previous test asserted
// Count=0 as "intentional per spec §5", but a registered pivot that always
// returns 0 is a U9 violation regardless of the excuse. See
// internal/aws/efs_related.go for the removal rationale.
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// Compile-time smoke: ensure all expected fixture IDs are referenced by the
// test file (prevents a rename in fixtures from silently breaking these tests).
// ---------------------------------------------------------------------------

var (
	_ = fixtures.ProdEFSID
	_ = fixtures.ProdEFSKmsKeyID
	_ = fixtures.ProdEFSKmsKeyARN
	_ = fixtures.ProdEFSVpcID
	_ = fixtures.ProdEFSSubnetAID
	_ = fixtures.ProdEFSSubnetBID
	_ = fixtures.ProdEFSSubnetCID
	_ = fixtures.ProdEFSSecurityGroupAID
	_ = fixtures.ProdEFSSecurityGroupBID
	_ = fixtures.ProdEFSEniAID
	_ = fixtures.ProdEFSEniBID
	_ = fixtures.ProdEFSEniCID
	_ = fixtures.ProdEFSCFNStackName
	_ = fixtures.ProdEFSAlarmAID
	_ = fixtures.ProdEFSAlarmBID
	_ = fixtures.ProdEFSLambdaAName
	_ = fixtures.ProdEFSLambdaBName
	_ = fixtures.ProdEFSBackupARecoveryARN
	_ = fixtures.ProdEFSBackupBRecoveryARN
)

// Verify that NewKMSFixtures, NewCFNFixtures, NewEC2Fixtures, NewLambdaFixtures
// are not nil (compile-time check that these constructors are importable).
var (
	_ *kmstypes.KeyMetadata     = nil
	_ *cfntypes.Stack           = nil
	_ *ec2types.NetworkInterface = nil
	_ *cwtypes.MetricAlarm      = nil
	_ *lambdatypes.FunctionConfiguration = nil
	_ *efstypes.FileSystemDescription    = nil
)
