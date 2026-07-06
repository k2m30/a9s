package unit_test

// aws_related_branch_gaps_test.go closes branch-coverage gaps on checkers
// that already execute in the suite but have <70% branch coverage:
//   - checkASGSG            internal/aws/asg_related_extra.go:81  (50.0%)
//   - checkCbSecrets        internal/aws/cb_related.go:309        (68.4%)
//   - checkDbcSnapVPC       internal/aws/dbc_snap_related.go:83   (66.7%)
//   - checkEIPECSTask       internal/aws/eip_related.go:233       (63.6%)
//   - checkEIPECSSvc        internal/aws/eip_related.go:254       (64.7%)
//   - checkEIPECS           internal/aws/eip_related.go:283       (64.7%)
//   - checkVPCELogs         internal/aws/vpce_related.go:149      (66.7%)
//
// Pins existing behavior (these are GREEN on write, not RED regressions).

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	asgtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cbtypes "github.com/aws/aws-sdk-go-v2/service/codebuild/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// checkASGSG (internal/aws/asg_related_extra.go:81) — 50.0% branch coverage.
// No prior test in the suite invokes asgCheckerByTarget(t, "sg") at all.
// Branches: wrong RawStruct(-1); nil/non-*ServiceClients clients(-1);
// LaunchConfigurationName set -> DescribeLaunchConfigurations path (success,
// error, empty-result); LaunchTemplate direct path; MixedInstancesPolicy
// fallback path; neither LC nor LT -> Count 0; LT DescribeLaunchTemplateVersions
// error; NetworkInterfaces[].Groups flattened alongside SecurityGroupIds.
// ---------------------------------------------------------------------------

func TestRelated_ASGSG_WrongRawStruct(t *testing.T) {
	res := resource.Resource{ID: "my-asg", RawStruct: "not-an-asg"}
	checker := asgCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

func TestRelated_ASGSG_NilClients(t *testing.T) {
	res := resource.Resource{
		ID:        "my-asg",
		RawStruct: asgtypes.AutoScalingGroup{AutoScalingGroupName: aws.String("my-asg")},
	}
	checker := asgCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count)
	}
}

func TestRelated_ASGSG_LaunchConfigPath_ReturnsSecurityGroups(t *testing.T) {
	const lcName = "my-launch-config"
	fakeASG := newFakeASGWithLaunchConfig(lcName, "instance-profile", []string{"sg-0abc1111", "sg-0abc2222"})
	clients := &awsclient.ServiceClients{AutoScaling: fakeASG, EC2: &fakeEC2Batch2{}}
	res := resource.Resource{
		ID: "my-asg",
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName:     aws.String("my-asg"),
			LaunchConfigurationName: aws.String(lcName),
		},
	}

	checker := asgCheckerByTarget(t, "sg")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	wantIDs := map[string]bool{"sg-0abc1111": true, "sg-0abc2222": true}
	for _, id := range result.ResourceIDs {
		if !wantIDs[id] {
			t.Errorf("unexpected SG ID %q in result %v", id, result.ResourceIDs)
		}
	}
}

func TestRelated_ASGSG_LaunchConfigPath_DescribeError(t *testing.T) {
	wantErr := errors.New("boom: describe launch configurations failed")
	fakeASG := &fakeASGBatch2{
		describeLaunchConfigsFn: func(_ *autoscaling.DescribeLaunchConfigurationsInput) (*autoscaling.DescribeLaunchConfigurationsOutput, error) {
			return nil, wantErr
		},
	}
	clients := &awsclient.ServiceClients{AutoScaling: fakeASG, EC2: &fakeEC2Batch2{}}
	res := resource.Resource{
		ID: "my-asg",
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName:     aws.String("my-asg"),
			LaunchConfigurationName: aws.String("my-launch-config"),
		},
	}

	checker := asgCheckerByTarget(t, "sg")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (DescribeLaunchConfigurations error)", result.Count)
	}
	if result.Err == nil {
		t.Error("expected Err to be set")
	}
}

func TestRelated_ASGSG_LaunchConfigPath_EmptyResult(t *testing.T) {
	fakeASG := &fakeASGBatch2{} // DescribeLaunchConfigurations returns empty output by default
	clients := &awsclient.ServiceClients{AutoScaling: fakeASG, EC2: &fakeEC2Batch2{}}
	res := resource.Resource{
		ID: "my-asg",
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName:     aws.String("my-asg"),
			LaunchConfigurationName: aws.String("my-launch-config"),
		},
	}

	checker := asgCheckerByTarget(t, "sg")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (empty LaunchConfigurations result)", result.Count)
	}
}

func TestRelated_ASGSG_NoLCNoLT_ReturnsZero(t *testing.T) {
	clients := &awsclient.ServiceClients{AutoScaling: &fakeASGBatch2{}, EC2: &fakeEC2Batch2{}}
	res := resource.Resource{
		ID: "my-asg",
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
		},
	}

	checker := asgCheckerByTarget(t, "sg")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no LaunchConfigurationName, no LaunchTemplate)", result.Count)
	}
}

func TestRelated_ASGSG_DirectLaunchTemplatePath_ReturnsSecurityGroupIDs(t *testing.T) {
	fakeEC2 := newFakeEC2WithLaunchTemplateVersions([]ec2types.LaunchTemplateVersion{
		{
			LaunchTemplateData: &ec2types.ResponseLaunchTemplateData{
				SecurityGroupIds: []string{"sg-lt0001"},
				NetworkInterfaces: []ec2types.LaunchTemplateInstanceNetworkInterfaceSpecification{
					{Groups: []string{"sg-lt0002", "sg-lt0003"}},
				},
			},
		},
	})
	clients := &awsclient.ServiceClients{EC2: fakeEC2, AutoScaling: &fakeASGBatch2{}}
	res := resource.Resource{
		ID: "my-asg",
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			LaunchTemplate: &asgtypes.LaunchTemplateSpecification{
				LaunchTemplateId: aws.String("lt-0abc1234567890abc"),
				Version:          aws.String("$Latest"),
			},
		},
	}

	checker := asgCheckerByTarget(t, "sg")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Count != 3 {
		t.Errorf("Count = %d, want 3 (1 SecurityGroupIds + 2 NetworkInterfaces.Groups)", result.Count)
	}
	wantIDs := map[string]bool{"sg-lt0001": true, "sg-lt0002": true, "sg-lt0003": true}
	for _, id := range result.ResourceIDs {
		if !wantIDs[id] {
			t.Errorf("unexpected SG ID %q in result %v", id, result.ResourceIDs)
		}
	}
}

func TestRelated_ASGSG_MixedInstancesPolicyFallback_ReturnsSecurityGroups(t *testing.T) {
	fakeEC2 := newFakeEC2WithLaunchTemplateVersions([]ec2types.LaunchTemplateVersion{
		{
			LaunchTemplateData: &ec2types.ResponseLaunchTemplateData{
				SecurityGroupIds: []string{"sg-mip0001"},
			},
		},
	})
	clients := &awsclient.ServiceClients{EC2: fakeEC2, AutoScaling: &fakeASGBatch2{}}
	res := resource.Resource{
		ID: "my-asg",
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			MixedInstancesPolicy: &asgtypes.MixedInstancesPolicy{
				LaunchTemplate: &asgtypes.LaunchTemplate{
					LaunchTemplateSpecification: &asgtypes.LaunchTemplateSpecification{
						LaunchTemplateId: aws.String("lt-0mip1234567890abc"),
						Version:          aws.String("$Latest"),
					},
				},
			},
		},
	}

	checker := asgCheckerByTarget(t, "sg")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "sg-mip0001" {
		t.Errorf("ResourceIDs = %v, want [sg-mip0001]", result.ResourceIDs)
	}
}

func TestRelated_ASGSG_LaunchTemplateVersionsError(t *testing.T) {
	wantErr := errors.New("boom: describe launch template versions failed")
	fakeEC2 := &fakeEC2Batch2{
		describeLaunchTemplateVersionsFn: func(_ *ec2.DescribeLaunchTemplateVersionsInput) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
			return nil, wantErr
		},
	}
	clients := &awsclient.ServiceClients{EC2: fakeEC2, AutoScaling: &fakeASGBatch2{}}
	res := resource.Resource{
		ID: "my-asg",
		RawStruct: asgtypes.AutoScalingGroup{
			AutoScalingGroupName: aws.String("my-asg"),
			LaunchTemplate: &asgtypes.LaunchTemplateSpecification{
				LaunchTemplateId: aws.String("lt-0abc1234567890abc"),
				Version:          aws.String("$Latest"),
			},
		},
	}

	checker := asgCheckerByTarget(t, "sg")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (DescribeLaunchTemplateVersions error)", result.Count)
	}
	if result.Err == nil {
		t.Error("expected Err to be set")
	}
}

// ---------------------------------------------------------------------------
// checkCbSecrets (internal/aws/cb_related.go:309) — 68.4% branch coverage.
// Existing tests only cover a plain (non-ARN) secret name and the "no
// SECRETS_MANAGER vars" case. Uncovered: wrong RawStruct(-1); nil
// Environment(0); ARN-form values with ":secret:" segment (with and without
// trailing ":json-key" suffix); nil env.Value skip.
// ---------------------------------------------------------------------------

func TestRelated_CbSecrets_WrongRawStruct(t *testing.T) {
	res := resource.Resource{ID: "my-project", RawStruct: "not-a-project"}
	checker := cbCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct)", result.Count)
	}
}

func TestRelated_CbSecrets_NilEnvironment(t *testing.T) {
	res := resource.Resource{
		ID:        "my-project",
		RawStruct: cbtypes.Project{Environment: nil},
	}
	checker := cbCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil Environment)", result.Count)
	}
}

func TestRelated_CbSecrets_ARNWithSecretSegment_ExtractsName(t *testing.T) {
	res := resource.Resource{
		ID: "my-project",
		RawStruct: cbtypes.Project{
			Environment: &cbtypes.ProjectEnvironment{
				EnvironmentVariables: []cbtypes.EnvironmentVariable{
					{
						Name:  aws.String("DB_PASSWORD"),
						Value: aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db/password-AbCdEf"),
						Type:  cbtypes.EnvironmentVariableTypeSecretsManager,
					},
				},
			},
		},
	}
	checker := cbCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "prod/db/password-AbCdEf" {
		t.Errorf("ResourceIDs = %v, want [prod/db/password-AbCdEf]", result.ResourceIDs)
	}
}

func TestRelated_CbSecrets_ARNWithJSONKeySuffix_StripsSuffix(t *testing.T) {
	res := resource.Resource{
		ID: "my-project",
		RawStruct: cbtypes.Project{
			Environment: &cbtypes.ProjectEnvironment{
				EnvironmentVariables: []cbtypes.EnvironmentVariable{
					{
						Name:  aws.String("API_KEY"),
						Value: aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/api/creds-XyZ123:apiKey"),
						Type:  cbtypes.EnvironmentVariableTypeSecretsManager,
					},
				},
			},
		},
	}
	checker := cbCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "prod/api/creds-XyZ123" {
		t.Errorf("ResourceIDs = %v, want [prod/api/creds-XyZ123] (json-key suffix stripped)", result.ResourceIDs)
	}
}

func TestRelated_CbSecrets_NilValueSkipped(t *testing.T) {
	res := resource.Resource{
		ID: "my-project",
		RawStruct: cbtypes.Project{
			Environment: &cbtypes.ProjectEnvironment{
				EnvironmentVariables: []cbtypes.EnvironmentVariable{
					{Name: aws.String("MISSING_VALUE"), Value: nil, Type: cbtypes.EnvironmentVariableTypeSecretsManager},
				},
			},
		},
	}
	checker := cbCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil Value skipped)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkDbcSnapVPC (internal/aws/dbc_snap_related.go:83) — 66.7% branch coverage.
// Existing tests (qa_related_field_extraction_test.go) cover only the
// docdbtypes.DBClusterSnapshot branch (with and without VpcId) and the
// no-RawStruct fallthrough. The rdstypes.DBClusterSnapshot branch (with and
// without VpcId) is never exercised.
// ---------------------------------------------------------------------------

func TestRelated_DbcSnapVPC_RDSType_ReturnsVpcID(t *testing.T) {
	res := resource.Resource{
		ID:        "rds:cluster-snapshot:aurora-snap",
		RawStruct: rdstypes.DBClusterSnapshot{VpcId: aws.String("vpc-rds9999")},
	}
	checker := fieldExtractionChecker(t, "dbc-snap", "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "vpc-rds9999" {
		t.Errorf("ResourceIDs = %v, want [vpc-rds9999]", result.ResourceIDs)
	}
}

func TestRelated_DbcSnapVPC_RDSType_NilVpcID_ReturnsZero(t *testing.T) {
	res := resource.Resource{
		ID:        "rds:cluster-snapshot:aurora-snap",
		RawStruct: rdstypes.DBClusterSnapshot{VpcId: nil},
	}
	checker := fieldExtractionChecker(t, "dbc-snap", "vpc")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil VpcId, rds type)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkEIPECSTask / checkEIPECSSvc / checkEIPECS (internal/aws/eip_related.go)
// Existing tests cover only the empty-EIP-ID (Count 0) and no-ENI (Count 0)
// early returns. Uncovered: eipMatchingECSTask error propagation (-1, Err
// set); truncated-cache no-match (ApproximateZero); a genuine match resolving
// the task/service-name/cluster; ECSSvc/ECS Group-prefix and ClusterArn
// guard branches.
// ---------------------------------------------------------------------------

func eipSrcWithENI(eniID string) resource.Resource {
	return resource.Resource{
		ID: "eipalloc-0match000000000001",
		RawStruct: ec2types.Address{
			AllocationId:       aws.String("eipalloc-0match000000000001"),
			NetworkInterfaceId: aws.String(eniID),
		},
	}
}

func TestRelated_EIPECSTask_CacheMissNoClients_ReturnsZeroNotUnknown(t *testing.T) {
	source := eipSrcWithENI("eni-0deadbeef00000001")
	// No "ecs-task" cache entry and no usable clients: FetchRelatedTarget falls
	// through to GetPaginatedFetcher, which cannot run without real clients and
	// returns an empty, non-error result — eipMatchingECSTask then reports "no
	// match found" (Count 0), not "unknown" (-1), for this checker.
	checker := eipCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (cache miss, no clients, no match)", result.Count)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_EIPECSTask_TruncatedNoMatch_ApproximateZero(t *testing.T) {
	source := eipSrcWithENI("eni-0deadbeef00000002")
	otherTask := resource.Resource{
		ID: "task-other",
		RawStruct: ecstypes.Task{
			Attachments: []ecstypes.Attachment{
				{Details: []ecstypes.KeyValuePair{
					{Name: aws.String("networkInterfaceId"), Value: aws.String("eni-different")},
				}},
			},
		},
	}
	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{otherTask},
			IsTruncated: true,
		},
	}
	checker := eipCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 || !result.Approximate {
		t.Errorf("Count = %d, Approximate = %v, want Count=0, Approximate=true", result.Count, result.Approximate)
	}
}

func TestRelated_EIPECSTask_Match_ReturnsTaskID(t *testing.T) {
	const eniID = "eni-0deadbeef00000003"
	source := eipSrcWithENI(eniID)
	matchedTask := resource.Resource{
		ID: "task-0abc1234567890def",
		RawStruct: ecstypes.Task{
			Attachments: []ecstypes.Attachment{
				{Details: []ecstypes.KeyValuePair{
					{Name: aws.String("networkInterfaceId"), Value: aws.String(eniID)},
				}},
			},
		},
	}
	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{Resources: []resource.Resource{matchedTask}},
	}
	checker := eipCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "task-0abc1234567890def" {
		t.Errorf("ResourceIDs = %v, want [task-0abc1234567890def]", result.ResourceIDs)
	}
}

func TestRelated_EIPECSSvc_Match_ReturnsServiceName(t *testing.T) {
	const eniID = "eni-0deadbeef00000004"
	source := eipSrcWithENI(eniID)
	matchedTask := resource.Resource{
		ID: "task-0abc1234567890def",
		RawStruct: ecstypes.Task{
			Group: aws.String("service:prod-web-svc"),
			Attachments: []ecstypes.Attachment{
				{Details: []ecstypes.KeyValuePair{
					{Name: aws.String("networkInterfaceId"), Value: aws.String(eniID)},
				}},
			},
		},
	}
	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{Resources: []resource.Resource{matchedTask}},
	}
	checker := eipCheckerByTarget(t, "ecs-svc")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "prod-web-svc" {
		t.Errorf("ResourceIDs = %v, want [prod-web-svc]", result.ResourceIDs)
	}
}

func TestRelated_EIPECSSvc_MatchedTaskNoServiceGroupPrefix_ReturnsZero(t *testing.T) {
	const eniID = "eni-0deadbeef00000005"
	source := eipSrcWithENI(eniID)
	matchedTask := resource.Resource{
		ID: "task-standalone",
		RawStruct: ecstypes.Task{
			Group: aws.String("family:standalone-task"),
			Attachments: []ecstypes.Attachment{
				{Details: []ecstypes.KeyValuePair{
					{Name: aws.String("networkInterfaceId"), Value: aws.String(eniID)},
				}},
			},
		},
	}
	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{Resources: []resource.Resource{matchedTask}},
	}
	checker := eipCheckerByTarget(t, "ecs-svc")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (Group does not have service: prefix)", result.Count)
	}
}

func TestRelated_EIPECS_Match_ReturnsClusterNameFromArn(t *testing.T) {
	const eniID = "eni-0deadbeef00000006"
	const clusterArn = "arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster"
	const wantClusterName = "prod-cluster"
	source := eipSrcWithENI(eniID)
	matchedTask := resource.Resource{
		ID: "task-0abc1234567890def",
		RawStruct: ecstypes.Task{
			ClusterArn: aws.String(clusterArn),
			Attachments: []ecstypes.Attachment{
				{Details: []ecstypes.KeyValuePair{
					{Name: aws.String("networkInterfaceId"), Value: aws.String(eniID)},
				}},
			},
		},
	}
	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{Resources: []resource.Resource{matchedTask}},
	}
	checker := eipCheckerByTarget(t, "ecs")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != wantClusterName {
		t.Errorf("ResourceIDs = %v, want [%s] (arnLastSegment extracts cluster name, not full ARN)", result.ResourceIDs, wantClusterName)
	}
}

func TestRelated_EIPECS_MatchedTaskNilClusterArn_ReturnsZero(t *testing.T) {
	const eniID = "eni-0deadbeef00000007"
	source := eipSrcWithENI(eniID)
	matchedTask := resource.Resource{
		ID: "task-noclusterarn",
		RawStruct: ecstypes.Task{
			ClusterArn: nil,
			Attachments: []ecstypes.Attachment{
				{Details: []ecstypes.KeyValuePair{
					{Name: aws.String("networkInterfaceId"), Value: aws.String(eniID)},
				}},
			},
		},
	}
	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{Resources: []resource.Resource{matchedTask}},
	}
	checker := eipCheckerByTarget(t, "ecs")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil ClusterArn on matched task)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkVPCELogs (internal/aws/vpce_related.go:149) — 66.7% branch coverage.
// Existing tests cover nil-clients(-1) and empty-ID(0). Uncovered: the
// success path (LogGroupName direct hit; LogDestination ARN parsing via
// ":log-group:" segment with and without a trailing colon; dedup by name)
// and the DescribeFlowLogs error path.
// ---------------------------------------------------------------------------

func TestRelated_VPCELogs_DescribeError(t *testing.T) {
	wantErr := errors.New("boom: describe flow logs failed")
	fakeEC2 := &fakeEC2VPCELogsError{err: wantErr}
	clients := &awsclient.ServiceClients{EC2: fakeEC2}
	res := vpceSrcInterfaceResource()

	checker := vpceCheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (DescribeFlowLogs error)", result.Count)
	}
	if result.Err == nil {
		t.Error("expected Err to be set")
	}
}

func TestRelated_VPCELogs_MatchByExplicitLogGroupName(t *testing.T) {
	fakeEC2 := &fakeEC2VPCELogsFlowLogs{
		flowLogs: []ec2types.FlowLog{
			{LogGroupName: aws.String("/aws/vpc/flow-logs/vpce-abc123")},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fakeEC2}
	res := vpceSrcInterfaceResource()

	checker := vpceCheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "/aws/vpc/flow-logs/vpce-abc123" {
		t.Errorf("ResourceIDs = %v, want [/aws/vpc/flow-logs/vpce-abc123]", result.ResourceIDs)
	}
}

func TestRelated_VPCELogs_MatchByLogDestinationARN_WithTrailingColon(t *testing.T) {
	fakeEC2 := &fakeEC2VPCELogsFlowLogs{
		flowLogs: []ec2types.FlowLog{
			{LogDestination: aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/vpc/flow-logs/vpce-abc123:*")},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fakeEC2}
	res := vpceSrcInterfaceResource()

	checker := vpceCheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "/aws/vpc/flow-logs/vpce-abc123" {
		t.Errorf("ResourceIDs = %v, want [/aws/vpc/flow-logs/vpce-abc123] (trailing colon stripped)", result.ResourceIDs)
	}
}

func TestRelated_VPCELogs_MatchByLogDestinationARN_NoTrailingColon(t *testing.T) {
	fakeEC2 := &fakeEC2VPCELogsFlowLogs{
		flowLogs: []ec2types.FlowLog{
			{LogDestination: aws.String("arn:aws:logs:us-east-1:123456789012:log-group:/aws/vpc/flow-logs/vpce-noColon")},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fakeEC2}
	res := vpceSrcInterfaceResource()

	checker := vpceCheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Err != nil {
		t.Fatalf("unexpected error: %v", result.Err)
	}
	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "/aws/vpc/flow-logs/vpce-noColon" {
		t.Errorf("ResourceIDs = %v, want [/aws/vpc/flow-logs/vpce-noColon]", result.ResourceIDs)
	}
}

func TestRelated_VPCELogs_DedupSameNameAcrossFlowLogs(t *testing.T) {
	fakeEC2 := &fakeEC2VPCELogsFlowLogs{
		flowLogs: []ec2types.FlowLog{
			{LogGroupName: aws.String("/aws/vpc/flow-logs/shared")},
			{LogGroupName: aws.String("/aws/vpc/flow-logs/shared")},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fakeEC2}
	res := vpceSrcInterfaceResource()

	checker := vpceCheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 (deduped log group name)", result.Count)
	}
}

func TestRelated_VPCELogs_NoFlowLogsReturnsZero(t *testing.T) {
	fakeEC2 := &fakeEC2VPCELogsFlowLogs{flowLogs: nil}
	clients := &awsclient.ServiceClients{EC2: fakeEC2}
	res := vpceSrcInterfaceResource()

	checker := vpceCheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no flow logs found)", result.Count)
	}
}

// fakeEC2VPCELogsFlowLogs is a minimal EC2API fake exercising only
// DescribeFlowLogs for checkVPCELogs branch coverage; all other methods
// return safe empty stubs since checkVPCELogs never calls them.
type fakeEC2VPCELogsFlowLogs struct {
	flowLogs []ec2types.FlowLog
}

func (f *fakeEC2VPCELogsFlowLogs) DescribeFlowLogs(_ context.Context, _ *ec2.DescribeFlowLogsInput, _ ...func(*ec2.Options)) (*ec2.DescribeFlowLogsOutput, error) {
	return &ec2.DescribeFlowLogsOutput{FlowLogs: f.flowLogs}, nil
}
func (f *fakeEC2VPCELogsFlowLogs) DescribeInstances(_ context.Context, _ *ec2.DescribeInstancesInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error) {
	return &ec2.DescribeInstancesOutput{}, nil
}
func (f *fakeEC2VPCELogsFlowLogs) DescribeInstanceStatus(_ context.Context, _ *ec2.DescribeInstanceStatusInput, _ ...func(*ec2.Options)) (*ec2.DescribeInstanceStatusOutput, error) {
	return &ec2.DescribeInstanceStatusOutput{}, nil
}
func (f *fakeEC2VPCELogsFlowLogs) DescribeVpcs(_ context.Context, _ *ec2.DescribeVpcsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error) {
	return &ec2.DescribeVpcsOutput{}, nil
}
func (f *fakeEC2VPCELogsFlowLogs) DescribeSecurityGroups(_ context.Context, _ *ec2.DescribeSecurityGroupsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error) {
	return &ec2.DescribeSecurityGroupsOutput{}, nil
}
func (f *fakeEC2VPCELogsFlowLogs) DescribeSubnets(_ context.Context, _ *ec2.DescribeSubnetsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error) {
	return &ec2.DescribeSubnetsOutput{}, nil
}
func (f *fakeEC2VPCELogsFlowLogs) DescribeRouteTables(_ context.Context, _ *ec2.DescribeRouteTablesInput, _ ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error) {
	return &ec2.DescribeRouteTablesOutput{}, nil
}
func (f *fakeEC2VPCELogsFlowLogs) DescribeNatGateways(_ context.Context, _ *ec2.DescribeNatGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error) {
	return &ec2.DescribeNatGatewaysOutput{}, nil
}
func (f *fakeEC2VPCELogsFlowLogs) DescribeInternetGateways(_ context.Context, _ *ec2.DescribeInternetGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error) {
	return &ec2.DescribeInternetGatewaysOutput{}, nil
}
func (f *fakeEC2VPCELogsFlowLogs) DescribeAddresses(_ context.Context, _ *ec2.DescribeAddressesInput, _ ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error) {
	return &ec2.DescribeAddressesOutput{}, nil
}
func (f *fakeEC2VPCELogsFlowLogs) DescribeTransitGateways(_ context.Context, _ *ec2.DescribeTransitGatewaysInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewaysOutput, error) {
	return &ec2.DescribeTransitGatewaysOutput{}, nil
}
func (f *fakeEC2VPCELogsFlowLogs) DescribeTransitGatewayAttachments(_ context.Context, _ *ec2.DescribeTransitGatewayAttachmentsInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayAttachmentsOutput, error) {
	return &ec2.DescribeTransitGatewayAttachmentsOutput{}, nil
}
func (f *fakeEC2VPCELogsFlowLogs) DescribeVpcEndpoints(_ context.Context, _ *ec2.DescribeVpcEndpointsInput, _ ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointsOutput, error) {
	return &ec2.DescribeVpcEndpointsOutput{}, nil
}
func (f *fakeEC2VPCELogsFlowLogs) DescribeNetworkInterfaces(_ context.Context, _ *ec2.DescribeNetworkInterfacesInput, _ ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error) {
	return &ec2.DescribeNetworkInterfacesOutput{}, nil
}
func (f *fakeEC2VPCELogsFlowLogs) DescribeVolumes(_ context.Context, _ *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	return &ec2.DescribeVolumesOutput{}, nil
}
func (f *fakeEC2VPCELogsFlowLogs) DescribeSnapshots(_ context.Context, _ *ec2.DescribeSnapshotsInput, _ ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error) {
	return &ec2.DescribeSnapshotsOutput{}, nil
}
func (f *fakeEC2VPCELogsFlowLogs) DescribeImages(_ context.Context, _ *ec2.DescribeImagesInput, _ ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error) {
	return &ec2.DescribeImagesOutput{}, nil
}
func (f *fakeEC2VPCELogsFlowLogs) DescribeVolumeStatus(_ context.Context, _ *ec2.DescribeVolumeStatusInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumeStatusOutput, error) {
	return &ec2.DescribeVolumeStatusOutput{}, nil
}
func (f *fakeEC2VPCELogsFlowLogs) DescribeTransitGatewayVpcAttachments(_ context.Context, _ *ec2.DescribeTransitGatewayVpcAttachmentsInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayVpcAttachmentsOutput, error) {
	return &ec2.DescribeTransitGatewayVpcAttachmentsOutput{}, nil
}
func (f *fakeEC2VPCELogsFlowLogs) DescribeTransitGatewayRouteTables(_ context.Context, _ *ec2.DescribeTransitGatewayRouteTablesInput, _ ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayRouteTablesOutput, error) {
	return &ec2.DescribeTransitGatewayRouteTablesOutput{}, nil
}
func (f *fakeEC2VPCELogsFlowLogs) DescribeLaunchTemplateVersions(_ context.Context, _ *ec2.DescribeLaunchTemplateVersionsInput, _ ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplateVersionsOutput, error) {
	return &ec2.DescribeLaunchTemplateVersionsOutput{}, nil
}

// fakeEC2VPCELogsError is a minimal EC2API fake whose DescribeFlowLogs always
// fails, for exercising checkVPCELogs's error-propagation branch.
type fakeEC2VPCELogsError struct {
	fakeEC2VPCELogsFlowLogs
	err error
}

func (f *fakeEC2VPCELogsError) DescribeFlowLogs(_ context.Context, _ *ec2.DescribeFlowLogsInput, _ ...func(*ec2.Options)) (*ec2.DescribeFlowLogsOutput, error) {
	return nil, f.err
}
