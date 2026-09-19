package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	backuptypes "github.com/aws/aws-sdk-go-v2/service/backup/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
	unit "github.com/k2m30/a9s/v3/tests/unit"
)

// efsCheckerByTarget is shared with aws_efs_related_extra_test.go,
// aws_efs_related_wave2_test.go and aws_related_checkers_branch_coverage_test.go.

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

// Task IDs are the ECS task ARNs of the fixture tasks whose api-gateway and
// web-frontend task definitions mount ProdEFSID.
func efsECSTaskCacheWithEFSIDs() resource.ResourceCache {
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

// checkEFSKMS emits the key ID without a cache lookup; the related-check
// orchestrator's lazy-add path populates the kms cache at dispatch time, so a
// nil cache is the correct shape here.

func TestRelated_EFS_KMS_GraphRoot(t *testing.T) {
	source := efsGraphRootSource()
	checker := efsCheckerByTarget(t, "kms")

	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (ProdEFSKmsKeyID); RawStruct = %+v", result.Count(), source.RawStruct)
	}
	if len(result.ResourceIDs()) < 1 {
		t.Fatalf("ResourceIDs is empty, want [%s]", fixtures.ProdEFSKmsKeyID)
	}
	if result.ResourceIDs()[0] != fixtures.ProdEFSKmsKeyID {
		t.Errorf("ResourceIDs[0] = %q, want %q", result.ResourceIDs()[0], fixtures.ProdEFSKmsKeyID)
	}
}

func TestRelated_EFS_CFN_GraphRoot(t *testing.T) {
	source := efsGraphRootSource()
	checker := efsCheckerByTarget(t, "cfn")
	cache := efsCFNCache()

	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (ProdEFSCFNStackName %q)", result.Count(), fixtures.ProdEFSCFNStackName)
	}
	found := false
	for _, id := range result.ResourceIDs() {
		if id == fixtures.ProdEFSCFNStackName {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("ResourceIDs = %v, want it to contain %q", result.ResourceIDs(), fixtures.ProdEFSCFNStackName)
	}
}

func TestRelated_EFS_Alarm_GraphRoot(t *testing.T) {
	source := efsGraphRootSource()
	checker := efsCheckerByTarget(t, "alarm")
	cache := efsAlarmCache()

	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2 (both EFS alarms); ResourceIDs = %v", result.Count(), result.ResourceIDs())
	}
	wantAlarms := []string{fixtures.ProdEFSAlarmAID, fixtures.ProdEFSAlarmBID}
	idSet := make(map[string]bool, len(result.ResourceIDs()))
	for _, id := range result.ResourceIDs() {
		idSet[id] = true
	}
	for _, want := range wantAlarms {
		if !idSet[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs())
		}
	}
}

func TestRelated_EFS_Lambda_GraphRoot(t *testing.T) {
	source := efsGraphRootSource()
	checker := efsCheckerByTarget(t, "lambda")

	efsFake := fakes.NewEFS()
	clients := &awsclient.ServiceClients{EFS: efsFake}
	cache := efsLambdaCache()

	result := checker(context.Background(), clients, source, cache)

	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2 (ProdEFSLambdaAName + ProdEFSLambdaBName); ResourceIDs = %v", result.Count(), result.ResourceIDs())
	}
	wantLambdas := []string{fixtures.ProdEFSLambdaAName, fixtures.ProdEFSLambdaBName}
	idSet := make(map[string]bool, len(result.ResourceIDs()))
	for _, id := range result.ResourceIDs() {
		idSet[id] = true
	}
	for _, want := range wantLambdas {
		if !idSet[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs())
		}
	}
}

// Recovery-point ARNs are not a valid ID for the backup type, so the checker
// reverse-scans the backup cache and returns BackupPlanIds.

func TestRelated_EFS_Backup_GraphRoot(t *testing.T) {
	source := efsGraphRootSource()
	checker := efsCheckerByTarget(t, "backup")

	cache := resource.ResourceCache{
		"backup": resource.ResourceCacheEntry{Resources: []resource.Resource{
			unit.BackupPlanRow(t, fixtures.HealthyDailyPlanID, backuptypes.BackupSelection{
				Resources: []string{fixtures.HealthyBucketARN, fixtures.ProdEFSARN, fixtures.OrdersProdARN},
			}),
			unit.BackupPlanRow(t, fixtures.AppDataPlanID, backuptypes.BackupSelection{
				Resources: []string{"arn:aws:dynamodb:us-east-1:123456789012:table/acme-app-sessions", fixtures.ProdEFSARN},
			}),
			unit.BackupPlanRow(t, fixtures.ProdCriticalPlanID, backuptypes.BackupSelection{
				Resources: []string{"arn:aws:rds:us-east-1:123456789012:db:acme-prod-secondary"},
			}),
		}},
	}

	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2 (two plans protect the graph-root EFS); ResourceIDs = %v", result.Count(), result.ResourceIDs())
	}
	wantPlanIDs := []string{fixtures.HealthyDailyPlanID, fixtures.AppDataPlanID}
	idSet := make(map[string]bool, len(result.ResourceIDs()))
	for _, id := range result.ResourceIDs() {
		idSet[id] = true
	}
	for _, want := range wantPlanIDs {
		if !idSet[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs())
		}
	}
}

func TestRelated_EFS_ENI_GraphRoot(t *testing.T) {
	source := efsGraphRootSource()
	checker := efsCheckerByTarget(t, "eni")
	cache := efsENICache()

	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 3 {
		t.Errorf("Count = %d, want 3 (ProdEFSEniA/B/C); ResourceIDs = %v", result.Count(), result.ResourceIDs())
	}
	wantENIs := []string{fixtures.ProdEFSEniAID, fixtures.ProdEFSEniBID, fixtures.ProdEFSEniCID}
	idSet := make(map[string]bool, len(result.ResourceIDs()))
	for _, id := range result.ResourceIDs() {
		idSet[id] = true
	}
	for _, want := range wantENIs {
		if !idSet[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs())
		}
	}
}

func TestRelated_EFS_SG_GraphRoot(t *testing.T) {
	source := efsGraphRootSource()
	checker := efsCheckerByTarget(t, "sg")
	cache := efsENICache()

	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2 (ProdEFSSecurityGroupA/B); ResourceIDs = %v", result.Count(), result.ResourceIDs())
	}
	wantSGs := []string{fixtures.ProdEFSSecurityGroupAID, fixtures.ProdEFSSecurityGroupBID}
	idSet := make(map[string]bool, len(result.ResourceIDs()))
	for _, id := range result.ResourceIDs() {
		idSet[id] = true
	}
	for _, want := range wantSGs {
		if !idSet[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs())
		}
	}
}

func TestRelated_EFS_Subnet_GraphRoot(t *testing.T) {
	source := efsGraphRootSource()
	checker := efsCheckerByTarget(t, "subnet")
	cache := efsENICache()

	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 3 {
		t.Errorf("Count = %d, want 3 (ProdEFSSubnetA/B/C); ResourceIDs = %v", result.Count(), result.ResourceIDs())
	}
	wantSubnets := []string{fixtures.ProdEFSSubnetAID, fixtures.ProdEFSSubnetBID, fixtures.ProdEFSSubnetCID}
	idSet := make(map[string]bool, len(result.ResourceIDs()))
	for _, id := range result.ResourceIDs() {
		idSet[id] = true
	}
	for _, want := range wantSubnets {
		if !idSet[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs())
		}
	}
}

func TestRelated_EFS_VPC_GraphRoot(t *testing.T) {
	source := efsGraphRootSource()
	checker := efsCheckerByTarget(t, "vpc")
	cache := efsENICache()

	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (ProdEFSVpcID); ResourceIDs = %v", result.Count(), result.ResourceIDs())
	}
	if len(result.ResourceIDs()) < 1 {
		t.Fatalf("ResourceIDs is empty, want [%s]", fixtures.ProdEFSVpcID)
	}
	if result.ResourceIDs()[0] != fixtures.ProdEFSVpcID {
		t.Errorf("ResourceIDs[0] = %q, want %q", result.ResourceIDs()[0], fixtures.ProdEFSVpcID)
	}
}

func TestRelated_EFS_ECSTask_GraphRoot(t *testing.T) {
	source := efsGraphRootSource()
	checker := efsCheckerByTarget(t, "ecs-task")
	cache := efsECSTaskCacheWithEFSIDs()

	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2 (taskA + taskB carry ProdEFSID); ResourceIDs = %v", result.Count(), result.ResourceIDs())
	}
	wantTasks := []string{
		"arn:aws:ecs:us-east-1:123456789012:task/acme-services/a1b2c3d4e5f6a1b2c3d4e5f6",
		"arn:aws:ecs:us-east-1:123456789012:task/acme-services/b2c3d4e5f6a1b2c3d4e5f601",
	}
	idSet := make(map[string]bool, len(result.ResourceIDs()))
	for _, id := range result.ResourceIDs() {
		idSet[id] = true
	}
	for _, want := range wantTasks {
		if !idSet[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs())
		}
	}
}

// References every fixture ID these tests rely on, so a fixture rename fails
// compilation here.

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

var (
	_ *kmstypes.KeyMetadata              = nil
	_ *cfntypes.Stack                    = nil
	_ *ec2types.NetworkInterface         = nil
	_ *cwtypes.MetricAlarm               = nil
	_ *lambdatypes.FunctionConfiguration = nil
	_ *efstypes.FileSystemDescription    = nil
)
