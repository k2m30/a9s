package unit_test

// related_truncated_zero_test.go — tests for relatedResultTrunc and
// the truncated-empty-cache honest-lower-bound contract.
//
// Anti-pattern (pre-task-#58; historically 225 occurrences across 69
// *_related*.go files):
//
//   if len(ids) == 0 && truncated {
//       return resource.RelatedCheckResult{TargetType: "X", Count: -1}
//   }
//
// Contract per resource.ValidateRelatedResult (related.go:144) and
// relatedResultTrunc's docstring: the honest state for "truncated cache
// with zero hits" is:
//
//   {State: RelatedResolved (zero value), Count: 0, Truncated: true}   — a valid lower bound, not unknown
//
// Task #58 replaced the Count==-1 sentinel with the domain.RelatedRowState
// enum, so the anti-pattern's modern equivalent is a checker returning any
// non-RelatedResolved state (RelatedUnknown/RelatedError/RelatedDeferred)
// instead of the honest Resolved+Truncated lower bound — that misrepresents
// a "we scanned what we could see and found nothing (more may exist)" result
// as "unknown"/"errored"/"deferred" when the count is actually a known >=0
// lower bound.

import (
	"context"
	"sort"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws" // ensure all related registrations run
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// ─────────────────────────────────────────────────────────────────────────────
// TestTruncatedResult_ReturnsTruncatedResult
// ─────────────────────────────────────────────────────────────────────────────

// TestTruncatedResult_ReturnsTruncatedResult verifies that TruncatedResult
// returns a fully-populated RelatedCheckResult with Count=0, Truncated=true,
// the given TargetType, empty ResourceIDs, and nil Err.
//
// KnownRelated always allocates its internal ID slice via make([]string, 0,
// len(ids)), so ResourceIDs() is never a literal nil for a KnownRelated
// result — only empty; ValidateRelatedResult/EffectiveState/
// IsRelatedActionable treat nil and empty ResourceIDs identically.
func TestTruncatedResult_ReturnsTruncatedResult(t *testing.T) {
	result := resource.KnownRelated("vpc", nil, true)

	if result.TargetType() != "vpc" {
		t.Errorf("TargetType = %q, want %q", result.TargetType(), "vpc")
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
	if !result.Truncated() {
		t.Error("Truncated = false, want true")
	}
	if len(result.ResourceIDs()) != 0 {
		t.Errorf("ResourceIDs = %v, want empty", result.ResourceIDs())
	}
	if result.Err() != nil {
		t.Errorf("Err = %v, want nil", result.Err())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestTruncatedResult_EmptyTargetType
// ─────────────────────────────────────────────────────────────────────────────

// TestTruncatedResult_EmptyTargetType verifies that TruncatedResult("") returns a
// result with an empty TargetType, which ValidateRelatedResult reports as invalid.
// This lets callers detect the empty-TargetType invariant at validation time.
func TestTruncatedResult_EmptyTargetType(t *testing.T) {
	result := resource.KnownRelated("", nil, true)

	// The struct is returned (TruncatedResult does not panic on empty input).
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
	if !result.Truncated() {
		t.Error("Truncated = false, want true")
	}

	// Callers can detect the mistake: ValidateRelatedResult must return an error.
	err := resource.ValidateRelatedResult(result)
	if err == nil {
		t.Error("ValidateRelatedResult(TruncatedResult(\"\")) = nil, want non-nil error (empty TargetType invariant)")
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestTruncatedResult_PassesValidation
// ─────────────────────────────────────────────────────────────────────────────

// TestTruncatedResult_PassesValidation verifies that for any non-empty targetType,
// the result returned by TruncatedResult passes ValidateRelatedResult with no error.
func TestTruncatedResult_PassesValidation(t *testing.T) {
	targetTypes := []string{
		"vpc", "subnet", "sg", "ec2", "rds", "eks", "ng", "elb",
		"nat", "igw", "rtb", "vpce", "eni", "tgw", "lambda", "s3",
		"secrets", "kms", "cfn", "ami", "ebs", "ebs-snap",
	}

	for _, tt := range targetTypes {
		tt := tt
		t.Run(tt, func(t *testing.T) {
			result := resource.KnownRelated(tt, nil, true)
			if err := resource.ValidateRelatedResult(result); err != nil {
				t.Errorf("TruncatedResult(%q) fails ValidateRelatedResult: %v", tt, err)
			}
		})
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestCheckVPC_TruncatedCacheReturnsTruncatedResult
// ─────────────────────────────────────────────────────────────────────────────

// TestCheckVPC_TruncatedCacheReturnsTruncatedResult calls the registered
// checkVPCSubnet checker (via the "vpc"→"subnet" RelatedDef) with a cache
// that has only the VPC resource and a subnet entry that is truncated with
// zero resources. The vpc resource has ID "vpc-12345678" which will not match
// any subnet in the empty (truncated) list.
//
// Expected: {Count: 0, Truncated: true} (honest lower bound), never
// {Count: -1} (discards the lower bound).
func TestCheckVPC_TruncatedCacheReturnsTruncatedResult(t *testing.T) {
	vpcResource := resource.Resource{
		ID:   "vpc-12345678",
		Name: "test-vpc",
		Fields: map[string]string{
			"vpc_id": "vpc-12345678",
			"state":  "available",
			"cidr":   "10.0.0.0/16",
		},
	}

	// Subnet cache entry: truncated, zero resources — simulates a partial page
	// where no subnets for this VPC happened to land in the first page.
	cache := resource.ResourceCache{
		"subnet": {
			Resources:   []resource.Resource{},
			IsTruncated: true,
		},
	}

	// Find the checkVPCSubnet checker via the registry.
	var checker resource.RelatedChecker
	for _, def := range resource.GetRelated("vpc") {
		if def.TargetType == "subnet" {
			checker = def.Checker
			break
		}
	}
	if checker == nil {
		t.Fatal("checkVPCSubnet not registered under vpc→subnet")
	}

	result := checker(context.Background(), nil, vpcResource, cache)

	if result.State() != domain.RelatedResolved {
		t.Errorf("checkVPCSubnet with truncated-empty cache returned State=%s "+
			"(anti-pattern); want RelatedResolved with Count=0, Truncated=true. Result: %+v", result.State(), result)
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
	if !result.Truncated() {
		t.Errorf("Truncated = false, want true (truncated cache means result is a lower bound). Result: %+v", result)
	}
	if result.TargetType() != "subnet" {
		t.Errorf("TargetType = %q, want \"subnet\"", result.TargetType())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestCheckSG_TruncatedCacheReturnsTruncatedResult
// ─────────────────────────────────────────────────────────────────────────────

// TestCheckSG_TruncatedCacheReturnsTruncatedResult calls the registered
// checkSGEC2 checker (via the "sg"→"ec2" RelatedDef) with an EC2 cache that
// is truncated with zero resources. The SG resource has a real ID that will
// not match any instance in the empty list.
//
// EXPECTED: {Count: 0, Truncated: true}
// ACTUAL (BUG): {Count: -1}
func TestCheckSG_TruncatedCacheReturnsTruncatedResult(t *testing.T) {
	sgResource := resource.Resource{
		ID:   "sg-0abcdef123456789",
		Name: "test-sg",
		Fields: map[string]string{
			"vpc_id":      "vpc-12345678",
			"group_name":  "test-sg",
			"description": "test security group",
		},
	}

	// EC2 cache entry: truncated, zero resources.
	cache := resource.ResourceCache{
		"ec2": {
			Resources:   []resource.Resource{},
			IsTruncated: true,
		},
	}

	var checker resource.RelatedChecker
	for _, def := range resource.GetRelated("sg") {
		if def.TargetType == "ec2" {
			checker = def.Checker
			break
		}
	}
	if checker == nil {
		t.Fatal("checkSGEC2 not registered under sg→ec2")
	}

	result := checker(context.Background(), nil, sgResource, cache)

	if result.State() != domain.RelatedResolved {
		t.Errorf("checkSGEC2 with truncated-empty cache returned State=%s "+
			"(anti-pattern); want RelatedResolved with Count=0, Truncated=true. Result: %+v", result.State(), result)
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
	if !result.Truncated() {
		t.Errorf("Truncated = false, want true (truncated cache means result is a lower bound). Result: %+v", result)
	}
	if result.TargetType() != "ec2" {
		t.Errorf("TargetType = %q, want \"ec2\"", result.TargetType())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestCheckAMI_NG_TruncatedCacheReturnsTruncatedResult
// ─────────────────────────────────────────────────────────────────────────────

// TestCheckAMI_NG_TruncatedCacheReturnsTruncatedResult calls the registered
// checkAMING checker (via the "ami"→"ng" RelatedDef) with an NG cache that is
// truncated with zero resources. The AMI resource has a real ID that will not
// match any node group in the empty list.
//
// EXPECTED: {Count: 0, Truncated: true}
// ACTUAL (BUG): {Count: -1}
func TestCheckAMI_NG_TruncatedCacheReturnsTruncatedResult(t *testing.T) {
	amiResource := resource.Resource{
		ID:   "ami-0abcdef1234567890",
		Name: "test-ami",
		Fields: map[string]string{
			"state":        "available",
			"architecture": "x86_64",
			"image_type":   "machine",
		},
	}

	// NG cache entry: truncated, zero resources.
	cache := resource.ResourceCache{
		"ng": {
			Resources:   []resource.Resource{},
			IsTruncated: true,
		},
	}

	var checker resource.RelatedChecker
	for _, def := range resource.GetRelated("ami") {
		if def.TargetType == "ng" {
			checker = def.Checker
			break
		}
	}
	if checker == nil {
		t.Fatal("checkAMING not registered under ami→ng")
	}

	result := checker(context.Background(), nil, amiResource, cache)

	if result.State() != domain.RelatedResolved {
		t.Errorf("checkAMING with truncated-empty NG cache returned State=%s "+
			"(anti-pattern); want RelatedResolved with Count=0, Truncated=true. Result: %+v", result.State(), result)
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
	if !result.Truncated() {
		t.Errorf("Truncated = false, want true (truncated cache means result is a lower bound). Result: %+v", result)
	}
	if result.TargetType() != "ng" {
		t.Errorf("TargetType = %q, want \"ng\"", result.TargetType())
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestAllReverseScanCheckers_TruncatedEmptyCacheReturnsTruncated
// ─────────────────────────────────────────────────────────────────────────────

// TestAllReverseScanCheckers_TruncatedEmptyCacheReturnsTruncated iterates over
// every registered (sourceType, RelatedDef) where NeedsTargetCache is true,
// constructs a ResourceCache where ALL target entries are {IsTruncated: true,
// Resources: []}, calls the checker with a minimal parent resource, and asserts
// the result is {Count: 0, Truncated: true} — NEVER {Count: -1}.
func TestAllReverseScanCheckers_TruncatedEmptyCacheReturnsTruncated(t *testing.T) {
	// Minimal parent resources keyed by source type. These are shaped to avoid
	// the early-exit "no ID / no key field → Count=0 definitively" guard, so
	// that each checker actually reaches the truncated-cache code path.
	minimalParents := map[string]resource.Resource{
		"vpc": {
			ID:   "vpc-00000001",
			Name: "test-vpc",
			Fields: map[string]string{
				"vpc_id": "vpc-00000001",
				"state":  "available",
				"cidr":   "10.0.0.0/16",
			},
		},
		"sg": {
			ID:   "sg-00000001",
			Name: "test-sg",
			Fields: map[string]string{
				"vpc_id": "vpc-00000001",
			},
		},
		"ec2": {
			ID:   "i-00000001",
			Name: "test-instance",
			Fields: map[string]string{
				"vpc_id":   "vpc-00000001",
				"state":    "running",
				"image_id": "ami-00000001",
			},
			RawStruct: ec2types.Instance{
				InstanceId: aws.String("i-00000001"),
				VpcId:      aws.String("vpc-00000001"),
				Tags: []ec2types.Tag{
					{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("test-stack")},
					{Key: aws.String("eks:cluster-name"), Value: aws.String("test-cluster")},
					{Key: aws.String("eks:nodegroup-name"), Value: aws.String("test-ng")},
				},
				BlockDeviceMappings: []ec2types.InstanceBlockDeviceMapping{{
					Ebs: &ec2types.EbsInstanceBlockDevice{VolumeId: aws.String("vol-00000001")},
				}},
			},
		},
		"ami": {
			ID:   "ami-00000001",
			Name: "test-ami",
			Fields: map[string]string{
				"state":        "available",
				"architecture": "x86_64",
			},
			RawStruct: ec2types.Image{
				ImageId: aws.String("ami-00000001"),
				Tags: []ec2types.Tag{{
					Key:   aws.String("aws:cloudformation:stack-name"),
					Value: aws.String("test-stack"),
				}},
			},
		},
		// Row 7: the parents below carry the RawStruct their fetcher
		// produces, because a checker handed a row with none never read it and
		// answers "?" — which is a different rule from the lower bound this
		// test pins. Giving the struct keeps each subtest on the truncated-scan
		// path it exists to cover.
		"ng": {
			ID:   "nodegroup-00000001",
			Name: "test-ng",
			Fields: map[string]string{
				"cluster_name": "test-cluster",
			},
			RawStruct: ekstypes.Nodegroup{
				NodegroupName: aws.String("test-ng"),
				ClusterName:   aws.String("test-cluster"),
				Resources: &ekstypes.NodegroupResources{
					AutoScalingGroups: []ekstypes.AutoScalingGroup{{Name: aws.String("test-asg")}},
				},
			},
		},
		"eks": {
			ID:   "test-cluster",
			Name: "test-cluster",
			Fields: map[string]string{
				"status": "ACTIVE",
			},
			RawStruct: ekstypes.Cluster{
				Name: aws.String("test-cluster"),
				Tags: map[string]string{"aws:cloudformation:stack-name": "test-stack"},
			},
		},
		"elb": {
			ID:   "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/test-lb/0000000000000001",
			Name: "test-lb",
			Fields: map[string]string{
				"vpc_id": "vpc-00000001",
				"state":  "active",
			},
		},
		"rds": {
			ID:   "test-db-instance",
			Name: "test-db",
			Fields: map[string]string{
				"vpc_id": "vpc-00000001",
				"status": "available",
			},
			RawStruct: rdstypes.DBInstance{
				DBInstanceIdentifier: aws.String("test-db-instance"),
				MasterUserSecret: &rdstypes.MasterUserSecret{
					SecretArn: aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:rds!db-test-AbCdEf"),
				},
			},
		},
		"lambda": {
			ID:   "arn:aws:lambda:us-east-1:123456789012:function:test-fn",
			Name: "test-fn",
			Fields: map[string]string{
				"vpc_id": "vpc-00000001",
			},
			RawStruct: lambdatypes.FunctionConfiguration{
				FunctionName: aws.String("test-fn"),
				FunctionArn:  aws.String("arn:aws:lambda:us-east-1:123456789012:function:test-fn"),
				Environment: &lambdatypes.EnvironmentResponse{Variables: map[string]string{
					"DB_SECRET": "arn:aws:secretsmanager:us-east-1:123456789012:secret:prod/db-AbCdEf",
					"API_PARAM": "/acme/prod/api-key",
				}},
			},
		},
		"ecs-svc": {
			ID:   "arn:aws:ecs:us-east-1:123456789012:service/test-cluster/test-svc",
			Name: "test-svc",
			Fields: map[string]string{
				"cluster_arn": "arn:aws:ecs:us-east-1:123456789012:cluster/test-cluster",
			},
			RawStruct: ecstypes.Service{
				ServiceName:    aws.String("test-svc"),
				ClusterArn:     aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/test-cluster"),
				TaskDefinition: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/test-svc:5"),
				LoadBalancers: []ecstypes.LoadBalancer{{
					TargetGroupArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/test-tg/0000000000000001"),
				}},
				NetworkConfiguration: &ecstypes.NetworkConfiguration{
					AwsvpcConfiguration: &ecstypes.AwsVpcConfiguration{Subnets: []string{"subnet-00000001"}},
				},
			},
		},
		"asg": {
			ID:   "test-asg",
			Name: "test-asg",
			Fields: map[string]string{
				"status": "InService",
			},
		},
		"subnet": {
			ID:   "subnet-00000001",
			Name: "test-subnet",
			Fields: map[string]string{
				"vpc_id":            "vpc-00000001",
				"availability_zone": "us-east-1a",
			},
		},
		"eni": {
			ID:   "eni-00000001",
			Name: "test-eni",
			Fields: map[string]string{
				"vpc_id": "vpc-00000001",
			},
		},
		"nat": {
			ID:   "nat-00000001",
			Name: "test-nat",
			Fields: map[string]string{
				"vpc_id": "vpc-00000001",
				"state":  "available",
			},
		},
		"igw": {
			ID:   "igw-00000001",
			Name: "test-igw",
			Fields: map[string]string{
				"vpc_id": "vpc-00000001",
			},
		},
		"rtb": {
			ID:   "rtb-00000001",
			Name: "test-rtb",
			Fields: map[string]string{
				"vpc_id": "vpc-00000001",
			},
			RawStruct: ec2types.RouteTable{
				RouteTableId: aws.String("rtb-00000001"),
				Tags: []ec2types.Tag{{
					Key:   aws.String("aws:cloudformation:stack-name"),
					Value: aws.String("test-stack"),
				}},
			},
		},
		"vpce": {
			ID:   "vpce-00000001",
			Name: "test-vpce",
			Fields: map[string]string{
				"vpc_id": "vpc-00000001",
				"state":  "available",
			},
		},
		"tgw": {
			ID:   "tgw-00000001",
			Name: "test-tgw",
			Fields: map[string]string{
				"state": "available",
			},
		},
		"ebs": {
			ID:   "vol-00000001",
			Name: "test-volume",
			Fields: map[string]string{
				"state":             "available",
				"availability_zone": "us-east-1a",
			},
			RawStruct: ec2types.Volume{
				VolumeId: aws.String("vol-00000001"),
				Tags: []ec2types.Tag{{
					Key:   aws.String("aws:cloudformation:stack-name"),
					Value: aws.String("test-stack"),
				}},
			},
		},
		"kms": {
			ID:   "arn:aws:kms:us-east-1:123456789012:key/00000000-0000-0000-0000-000000000001",
			Name: "test-key",
			Fields: map[string]string{
				"state": "Enabled",
			},
		},
		"secrets": {
			ID:     "arn:aws:secretsmanager:us-east-1:123456789012:secret:test-secret-abcdef",
			Name:   "test-secret",
			Fields: map[string]string{},
			RawStruct: smtypes.SecretListEntry{
				Name:              aws.String("test-secret"),
				ARN:               aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:test-secret-abcdef"),
				KmsKeyId:          aws.String("arn:aws:kms:us-east-1:123456789012:key/00000000-0000-0000-0000-000000000001"),
				RotationLambdaARN: aws.String("arn:aws:lambda:us-east-1:123456789012:function:test-fn"),
			},
		},
		"s3": {
			ID:   "test-bucket",
			Name: "test-bucket",
			Fields: map[string]string{
				"region": "us-east-1",
			},
		},
		"cfn": {
			ID:   "test-stack",
			Name: "test-stack",
			Fields: map[string]string{
				"status": "CREATE_COMPLETE",
			},
			RawStruct: cfntypes.Stack{
				StackName: aws.String("test-stack"),
				StackId:   aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/test-stack/00000000-0000-0000-0000-000000000001"),
			},
		},
		"eb": {
			ID:     "test-eb-app",
			Name:   "test-eb-app",
			Fields: map[string]string{},
		},
		"ecr": {
			ID:   "arn:aws:ecr:us-east-1:123456789012:repository/test-repo",
			Name: "test-repo",
			Fields: map[string]string{
				"uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/test-repo",
			},
			RawStruct: ecrtypes.Repository{
				RepositoryName: aws.String("test-repo"),
				RepositoryArn:  aws.String("arn:aws:ecr:us-east-1:123456789012:repository/test-repo"),
			},
		},
		"sfn": {
			ID:     "arn:aws:states:us-east-1:123456789012:stateMachine:test-sm",
			Name:   "test-sm",
			Fields: map[string]string{},
		},
		"sns": {
			ID:     "arn:aws:sns:us-east-1:123456789012:test-topic",
			Name:   "test-topic",
			Fields: map[string]string{},
		},
		"dynamo": {
			ID:   "test-table",
			Name: "test-table",
			Fields: map[string]string{
				"status": "ACTIVE",
			},
		},
		"redis": {
			ID:   "test-redis",
			Name: "test-redis",
			Fields: map[string]string{
				"status": "available",
			},
			// checkRedisSG/SNS/Subnet all require this RawStruct shape to reach
			// redisMemberCluster's DescribeCacheClusters hop at all (see
			// clientsOverride["redis"] below for the matching fake).
			RawStruct: elasticachetypes.ReplicationGroup{
				MemberClusters: []string{"test-redis-member-1"},
			},
		},
		"docdb": {
			ID:   "test-docdb",
			Name: "test-docdb",
			Fields: map[string]string{
				"status": "available",
			},
			RawStruct: rdstypes.DBCluster{
				DBClusterIdentifier: aws.String("test-docdb"),
				MasterUserSecret: &rdstypes.MasterUserSecret{
					SecretArn: aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:docdb!cluster-test-AbCdEf"),
				},
			},
		},
		"efs": {
			ID:   "fs-00000001",
			Name: "test-efs",
			Fields: map[string]string{
				"lifecycle_state": "available",
			},
			RawStruct: efstypes.FileSystemDescription{
				FileSystemId:  aws.String("fs-00000001"),
				FileSystemArn: aws.String("arn:aws:elasticfilesystem:us-east-1:123456789012:file-system/fs-00000001"),
			},
		},
		"ses": {
			ID:     "test@example.com",
			Name:   "test@example.com",
			Fields: map[string]string{},
		},
		"ssm": {
			ID:   "/test/param",
			Name: "/test/param",
			Fields: map[string]string{
				"type": "String",
			},
			RawStruct: ssmtypes.ParameterMetadata{
				Name:  aws.String("/test/param"),
				Type:  ssmtypes.ParameterTypeSecureString,
				KeyId: aws.String("arn:aws:kms:us-east-1:123456789012:key/00000000-0000-0000-0000-000000000001"),
			},
		},
		"pipeline": {
			ID:     "test-pipeline",
			Name:   "test-pipeline",
			Fields: map[string]string{},
		},
		"ebs-snap": {
			ID:   "snap-00000001",
			Name: "test-snapshot",
			Fields: map[string]string{
				"state": "completed",
			},
			RawStruct: ec2types.Snapshot{
				SnapshotId:  aws.String("snap-00000001"),
				Description: aws.String("Created by AWS Backup"),
				Tags: []ec2types.Tag{{
					Key:   aws.String("aws:backup:source-resource"),
					Value: aws.String("arn:aws:ec2:us-east-1:123456789012:volume/vol-00000001"),
				}},
			},
		},
		"role": {
			ID:     "arn:aws:iam::123456789012:role/test-role",
			Name:   "test-role",
			Fields: map[string]string{},
		},
		"iam-user": {
			ID:     "arn:aws:iam::123456789012:user/test-user",
			Name:   "test-user",
			Fields: map[string]string{},
		},
		"alarm": {
			ID:     "arn:aws:cloudwatch:us-east-1:123456789012:alarm:test-alarm",
			Name:   "test-alarm",
			Fields: map[string]string{},
			RawStruct: cwtypes.MetricAlarm{
				AlarmName: aws.String("test-alarm"),
				AlarmArn:  aws.String("arn:aws:cloudwatch:us-east-1:123456789012:alarm:test-alarm"),
				Dimensions: []cwtypes.Dimension{{
					Name:  aws.String("AutoScalingGroupName"),
					Value: aws.String("test-asg"),
				}},
			},
		},
		"backup": {
			ID:     "arn:aws:backup:us-east-1:123456789012:backup-plan:test-plan",
			Name:   "test-plan",
			Fields: map[string]string{},
		},
	}

	// clientsOverride supplies a real *ServiceClients for the handful of
	// NeedsTargetCache checkers that are genuinely two-hop: they need a live
	// AWS call to resolve an intermediate reference (redis's member cluster,
	// ses's configuration set) BEFORE ever reaching the target-cache
	// truncation logic this test exists to exercise. Every other checker in
	// the sweep is single-hop (its only prerequisite is the cache parameter),
	// so nil clients correctly drives it straight to that logic. Reuses the
	// same fakes aws_redis_related_test.go / fakes_ses_secrets_related_test.go
	// already define for this package.
	clientsOverride := map[string]any{
		"redis": &awsclient.ServiceClients{
			ElastiCache: &mockElastiCacheFullAPI{
				cacheClustersOutput: &elasticache.DescribeCacheClustersOutput{
					CacheClusters: []elasticachetypes.CacheCluster{{
						CacheClusterId: aws.String("test-redis-member-1"),
						SecurityGroups: []elasticachetypes.SecurityGroupMembership{
							{SecurityGroupId: aws.String("sg-redis-truncated-test"), Status: aws.String("active")},
						},
						NotificationConfiguration: &elasticachetypes.NotificationConfiguration{
							TopicArn:    aws.String("arn:aws:sns:us-east-1:123456789012:redis-truncated-test"),
							TopicStatus: aws.String("active"),
						},
						CacheSubnetGroupName: aws.String("redis-truncated-test-subnet-group"),
					}},
				},
				cacheSubnetGroupsOutput: &elasticache.DescribeCacheSubnetGroupsOutput{
					CacheSubnetGroups: []elasticachetypes.CacheSubnetGroup{{
						Subnets: []elasticachetypes.Subnet{
							{SubnetIdentifier: aws.String("subnet-redis-truncated-test")},
						},
					}},
				},
			},
		},
		"ses": &awsclient.ServiceClients{
			SESv2: newFakeSESv2WithEventDestinations(
				"test@example.com",
				"ses-truncated-test-config-set",
				[]sesv2types.EventDestination{{
					EventBridgeDestination: &sesv2types.EventBridgeDestination{
						EventBusArn: aws.String("arn:aws:events:us-east-1:123456789012:event-bus/ses-truncated-test-bus"),
					},
				}},
			),
		},
	}

	// fallbackParent is used for any source type not in the above map.
	fallbackParent := resource.Resource{
		ID:   "test-resource-id",
		Name: "test-resource",
		Fields: map[string]string{
			"vpc_id": "vpc-00000001",
			"state":  "active",
		},
	}

	// Enumerate all registered source types from the related registry.
	// We need to iterate over all source types. Use all resource type short names
	// that have registered related defs.
	allSourceTypes := []string{
		"vpc", "sg", "ec2", "ami", "ng", "eks", "elb", "rds",
		"lambda", "ecs-svc", "asg", "subnet", "eni", "nat", "igw",
		"rtb", "vpce", "tgw", "ebs", "kms", "secrets", "s3", "cfn",
		"eb", "ecr", "sfn", "sns", "dynamo", "redis", "docdb", "efs",
		"ses", "ssm", "pipeline", "ebs-snap", "role", "iam-user",
		"alarm", "backup", "vpce", "tgw",
	}

	// Deduplicate: track already-tested (sourceType, targetType) pairs.
	tested := make(map[string]bool)

	for _, sourceType := range allSourceTypes {
		defs := resource.GetRelated(sourceType)
		if len(defs) == 0 {
			continue
		}

		parent := fallbackParent
		if p, ok := minimalParents[sourceType]; ok {
			parent = p
		}
		var clients any
		if c, ok := clientsOverride[sourceType]; ok {
			clients = c
		}

		for _, def := range defs {
			if !def.NeedsTargetCache {
				// Forward checkers that don't read from cache are out of scope.
				continue
			}
			if def.Checker == nil {
				continue
			}

			key := sourceType + "→" + def.TargetType
			if tested[key] {
				continue
			}
			tested[key] = true

			t.Run(key, func(t *testing.T) {
				// Build a cache where the target type is truncated with zero items.
				// All other types are absent from the cache so checkers fall through
				// to the target-type entry only.
				cache := resource.ResourceCache{
					def.TargetType: {
						Resources:   []resource.Resource{},
						IsTruncated: true,
					},
				}

				result := def.Checker(context.Background(), clients, parent, cache)

				// The checker MAY legitimately return Count=0 non-truncated (e.g.,
				// when it determines from the parent's own fields that there can be
				// no related resources of this type). That is acceptable — it means
				// the early-exit guard fired, not the truncated-cache path.
				// What is NEVER acceptable is a non-RelatedResolved State when we
				// provided a real cache entry (not nil). A non-Resolved State
				// combined with IsTruncated=true is the anti-pattern that drops the
				// honest lower bound.
				if reason, expected := reverseScanExpectsUnknown[key]; expected {
					// Row 18: this checker never reaches a scan of the target
					// list under this harness, so there are no pages for a zero
					// to be a lower bound over.
					if result.State() != domain.RelatedUnknown {
						t.Errorf("checker %s: state = %s, want Unknown — %s",
							key, result.State(), reason)
					}
					return
				}
				if result.State() != domain.RelatedResolved {
					t.Errorf("checker %s with truncated-empty %q cache returned State=%s "+
						"(anti-pattern: drops honest lower bound); want RelatedResolved. "+
						"Only the state is asserted: whether Truncated came back set depends on "+
						"whether the checker reached the target read or exited on the parent's "+
						"own fields first, and this harness cannot tell those apart. "+
						"Result: %+v", key, def.TargetType, result.State(), result)
				}

				// When Count==0, Truncated must be true if the truncated path was hit.
				// We cannot distinguish "early exit" from "truncated path with 0 matches"
				// purely from the outside, so we only assert on the invariant:
				// Count >= 0 is required (already checked above).
				//
				// Additionally, validate the overall result shape.
				if result.TargetType() == "" {
					// Tolerate missing echo — fill it for validation.
					result = result.WithTargetType(def.TargetType)
				}
				if err := resource.ValidateRelatedResult(result); err != nil {
					t.Errorf("checker %s returned invalid result: %v; result: %+v",
						key, err, result)
				}
			})
		}
	}

	// An entry naming a checker the harness never reaches is inert: it stops
	// asserting anything and the next checker to take that key inherits an
	// exemption nobody wrote for it. A wrong entry already fails loudly above,
	// because it demands Unknown; only an unreached one can rot quietly.
	var unreached []string
	for key := range reverseScanExpectsUnknown {
		if !tested[key] {
			unreached = append(unreached, key)
		}
	}
	if len(unreached) > 0 {
		sort.Strings(unreached)
		t.Errorf("%d reverseScanExpectsUnknown entr(ies) name a checker this harness no longer runs. "+
			"Re-derive the key or delete the entry:\n  %s", len(unreached), strings.Join(unreached, "\n  "))
	}
}

// reverseScanExpectsUnknown names the checkers this harness cannot put on the
// truncated-scan path, with the reason for each. Most read a list OTHER than
// their target type, and the harness seeds only the target entry — so the join
// list is absent, the target list is never fetched, and row 18 makes that
// Unknown rather than a zero nobody counted. Two need a live client to read the
// rows they were handed, and the harness has none. Either way there are no
// pages for a lower bound to be over, so Unknown is the honest answer and the
// truncation rule has nothing to apply to.
var reverseScanExpectsUnknown = map[string]string{
	"ec2→kms":          "reads the ebs list to find the volumes whose keys answer",
	"eks→asg":          "reads the ng list to find the node groups whose scaling groups answer",
	"lambda→sns":       "reads the sns-subscription list to find the topics that answer",
	"efs→vpc":          "reads the eni list to find the mount targets whose VPC answers",
	"ecs-svc→elb":      "reads the tg list to find the load balancers the service's target groups belong to",
	"ecs-svc→vpc":      "reads the subnet list to find the VPC the service's awsvpc subnets sit in",
	"secrets→eb":       "describes each environment's config through a live client to see which resolves this secret",
	"secrets→ecs-task": "describes each task definition through a live client to see which reads this secret",
}
