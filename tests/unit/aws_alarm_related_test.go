package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func alarmCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("alarm") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("alarm related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("alarm related checker for %s not found", target)
	return nil
}

func TestRelated_Alarm_SNS_Found(t *testing.T) {
	snsARN := "arn:aws:sns:us-east-1:123456789012:my-topic"
	raw := cwtypes.MetricAlarm{
		AlarmActions: []string{snsARN},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"sns": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "arn:aws:sns:us-east-1:123456789012:my-topic", Name: "arn:aws:sns:us-east-1:123456789012:my-topic"},
		}},
	}

	checker := alarmCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != snsARN {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), snsARN)
	}
}

func TestRelated_Alarm_SNS_OKActions(t *testing.T) {
	snsARN := "arn:aws:sns:us-east-1:123456789012:ok-topic"
	raw := cwtypes.MetricAlarm{
		OKActions: []string{snsARN},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"sns": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "arn:aws:sns:us-east-1:123456789012:ok-topic", Name: "arn:aws:sns:us-east-1:123456789012:ok-topic"},
		}},
	}

	checker := alarmCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
}

func TestRelated_Alarm_SNS_InsufficientDataActions(t *testing.T) {
	snsARN := "arn:aws:sns:us-east-1:123456789012:insufficient-topic"
	raw := cwtypes.MetricAlarm{
		InsufficientDataActions: []string{snsARN},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"sns": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "arn:aws:sns:us-east-1:123456789012:insufficient-topic", Name: "arn:aws:sns:us-east-1:123456789012:insufficient-topic"},
		}},
	}

	checker := alarmCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
}

func TestRelated_Alarm_SNS_FiltersNonSNS(t *testing.T) {
	// Lambda ARN in actions should not be counted
	lambdaARN := "arn:aws:lambda:us-east-1:123456789012:function:my-func"
	raw := cwtypes.MetricAlarm{
		AlarmActions: []string{lambdaARN},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"sns": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "arn:aws:sns:us-east-1:123456789012:my-topic", Name: "arn:aws:sns:us-east-1:123456789012:my-topic"},
		}},
	}

	checker := alarmCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (Lambda ARN should not match SNS)", result.Count())
	}
}

func TestRelated_Alarm_SNS_Deduplicates(t *testing.T) {
	snsARN := "arn:aws:sns:us-east-1:123456789012:shared-topic"
	raw := cwtypes.MetricAlarm{
		AlarmActions:            []string{snsARN},
		OKActions:               []string{snsARN},
		InsufficientDataActions: []string{snsARN},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"sns": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "arn:aws:sns:us-east-1:123456789012:shared-topic", Name: "arn:aws:sns:us-east-1:123456789012:shared-topic"},
		}},
	}

	checker := alarmCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (same ARN in all actions should deduplicate)", result.Count())
	}
}

func TestRelated_Alarm_SNS_NoActions(t *testing.T) {
	raw := cwtypes.MetricAlarm{}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"sns": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "arn:aws:sns:us-east-1:123456789012:my-topic", Name: "arn:aws:sns:us-east-1:123456789012:my-topic"},
		}},
	}

	checker := alarmCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_Alarm_SNS_InvalidRawStruct(t *testing.T) {
	res := resource.Resource{ID: "test-alarm", RawStruct: "not-a-metric-alarm"}

	checker := alarmCheckerByTarget(t, "sns")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 for invalid RawStruct", result.Count())
	}
}

func TestRelated_Alarm_ASG_MatchByDimension(t *testing.T) {
	raw := cwtypes.MetricAlarm{
		Namespace: aws.String("AWS/AutoScaling"),
		Dimensions: []cwtypes.Dimension{
			{
				Name:  aws.String("AutoScalingGroupName"),
				Value: aws.String("my-asg"),
			},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "my-asg"},
		}},
	}

	checker := alarmCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
}

func TestRelated_Alarm_ASG_NoMatch(t *testing.T) {
	raw := cwtypes.MetricAlarm{
		Namespace: aws.String("AWS/AutoScaling"),
		Dimensions: []cwtypes.Dimension{
			{
				Name:  aws.String("AutoScalingGroupName"),
				Value: aws.String("my-asg"),
			},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "other-asg"},
		}},
	}

	checker := alarmCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_Alarm_ASG_NoDimension(t *testing.T) {
	raw := cwtypes.MetricAlarm{
		Namespace: aws.String("AWS/Lambda"),
		Dimensions: []cwtypes.Dimension{
			{
				Name:  aws.String("FunctionName"),
				Value: aws.String("my-lambda"),
			},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}
	cache := resource.ResourceCache{
		"asg": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "my-asg"},
		}},
	}

	checker := alarmCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no AutoScalingGroupName dimension)", result.Count())
	}
}

func TestRelated_Alarm_ASG_NilCache(t *testing.T) {
	raw := cwtypes.MetricAlarm{
		Namespace: aws.String("AWS/AutoScaling"),
		Dimensions: []cwtypes.Dimension{
			{
				Name:  aws.String("AutoScalingGroupName"),
				Value: aws.String("my-asg"),
			},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}
	cache := resource.ResourceCache{}

	checker := alarmCheckerByTarget(t, "asg")
	result := checker(context.Background(), nil, res, cache)

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (empty cache)", result.Count())
	}
}

func TestRelated_Alarm_APIGW_MatchByApiName(t *testing.T) {
	raw := cwtypes.MetricAlarm{
		Namespace: aws.String("AWS/ApiGateway"),
		Dimensions: []cwtypes.Dimension{
			{Name: aws.String("ApiName"), Value: aws.String("my-api")},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"apigw": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "my-api", Name: "my-api"},
		}},
	}

	checker := alarmCheckerByTarget(t, "apigw")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "my-api" {
		t.Errorf("ResourceIDs = %v, want [my-api]", result.ResourceIDs())
	}
}

func TestRelated_Alarm_APIGW_MatchByApiId(t *testing.T) {
	raw := cwtypes.MetricAlarm{
		Namespace: aws.String("AWS/ApiGateway"),
		Dimensions: []cwtypes.Dimension{
			{Name: aws.String("ApiId"), Value: aws.String("abc123xyz")},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"apigw": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "abc123xyz", Name: "abc123xyz"},
		}},
	}

	checker := alarmCheckerByTarget(t, "apigw")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (ApiId dimension)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "abc123xyz" {
		t.Errorf("ResourceIDs = %v, want [abc123xyz]", result.ResourceIDs())
	}
}

func TestRelated_Alarm_APIGW_NoDimension(t *testing.T) {
	raw := cwtypes.MetricAlarm{
		Namespace: aws.String("AWS/Lambda"),
		Dimensions: []cwtypes.Dimension{
			{Name: aws.String("FunctionName"), Value: aws.String("my-func")},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"apigw": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "my-api", Name: "my-api"},
		}},
	}

	checker := alarmCheckerByTarget(t, "apigw")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no ApiName or ApiId dimension)", result.Count())
	}
}

func TestRelated_Alarm_APIGW_InvalidRawStruct(t *testing.T) {
	res := resource.Resource{ID: "test-alarm", RawStruct: "not-an-alarm"}

	checker := alarmCheckerByTarget(t, "apigw")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (invalid RawStruct)", result.Count())
	}
}

func TestRelated_Alarm_CB_MatchByProjectName(t *testing.T) {
	raw := cwtypes.MetricAlarm{
		Namespace: aws.String("AWS/CodeBuild"),
		Dimensions: []cwtypes.Dimension{
			{Name: aws.String("ProjectName"), Value: aws.String("my-build")},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"cb": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "my-build", Name: "my-build"},
		}},
	}

	checker := alarmCheckerByTarget(t, "cb")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "my-build" {
		t.Errorf("ResourceIDs = %v, want [my-build]", result.ResourceIDs())
	}
}

func TestRelated_Alarm_CB_NoDimension(t *testing.T) {
	raw := cwtypes.MetricAlarm{}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"cb": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "my-build", Name: "my-build"},
		}},
	}

	checker := alarmCheckerByTarget(t, "cb")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_Alarm_DBI_MatchByDBInstanceIdentifier(t *testing.T) {
	raw := cwtypes.MetricAlarm{
		Namespace: aws.String("AWS/RDS"),
		Dimensions: []cwtypes.Dimension{
			{Name: aws.String("DBInstanceIdentifier"), Value: aws.String("prod-postgres-01")},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"dbi": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "prod-postgres-01", Name: "prod-postgres-01"},
		}},
	}

	checker := alarmCheckerByTarget(t, "dbi")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "prod-postgres-01" {
		t.Errorf("ResourceIDs = %v, want [prod-postgres-01]", result.ResourceIDs())
	}
}

func TestRelated_Alarm_DBI_NoDimension(t *testing.T) {
	raw := cwtypes.MetricAlarm{}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"dbi": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "prod-postgres-01", Name: "prod-postgres-01"},
		}},
	}

	checker := alarmCheckerByTarget(t, "dbi")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_Alarm_EC2_MatchByInstanceId(t *testing.T) {
	raw := cwtypes.MetricAlarm{
		Namespace: aws.String("AWS/EC2"),
		Dimensions: []cwtypes.Dimension{
			{Name: aws.String("InstanceId"), Value: aws.String("i-0a1b2c3d4e5f67890")},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "i-0a1b2c3d4e5f67890", Name: "i-0a1b2c3d4e5f67890"},
		}},
	}

	checker := alarmCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "i-0a1b2c3d4e5f67890" {
		t.Errorf("ResourceIDs = %v, want [i-0a1b2c3d4e5f67890]", result.ResourceIDs())
	}
}

func TestRelated_Alarm_EC2_NoDimension(t *testing.T) {
	raw := cwtypes.MetricAlarm{}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"ec2": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "i-0a1b2c3d4e5f67890", Name: "i-0a1b2c3d4e5f67890"},
		}},
	}

	checker := alarmCheckerByTarget(t, "ec2")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

// TestRelated_Alarm_ECS_MatchByClusterName: EKS and ECS alarms share the
// ClusterName dimension, so only the namespace says which cluster type it
// is: an alarm without its namespace would count an EKS cluster name as an
// ECS cluster.
func TestRelated_Alarm_ECS_MatchByClusterName(t *testing.T) {
	raw := cwtypes.MetricAlarm{
		Namespace: aws.String("AWS/ECS"),
		Dimensions: []cwtypes.Dimension{
			{Name: aws.String("ClusterName"), Value: aws.String("prod-cluster")},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"ecs": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "prod-cluster", Name: "prod-cluster"},
		}},
	}

	checker := alarmCheckerByTarget(t, "ecs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "prod-cluster" {
		t.Errorf("ResourceIDs = %v, want [prod-cluster]", result.ResourceIDs())
	}
}

func TestRelated_Alarm_ECS_NoDimension(t *testing.T) {
	raw := cwtypes.MetricAlarm{}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"ecs": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "prod-cluster", Name: "prod-cluster"},
		}},
	}

	checker := alarmCheckerByTarget(t, "ecs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

// checkAlarmEKS requires both ClusterName dimension AND an EKS/ContainerInsights namespace.

func TestRelated_Alarm_EKS_MatchByClusterNameAndEKSNamespace(t *testing.T) {
	raw := cwtypes.MetricAlarm{
		Namespace: aws.String("AWS/EKS"),
		Dimensions: []cwtypes.Dimension{
			{Name: aws.String("ClusterName"), Value: aws.String("my-eks-cluster")},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"eks": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "my-eks-cluster", Name: "my-eks-cluster"},
		}},
	}

	checker := alarmCheckerByTarget(t, "eks")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (EKS namespace + ClusterName)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "my-eks-cluster" {
		t.Errorf("ResourceIDs = %v, want [my-eks-cluster]", result.ResourceIDs())
	}
}

func TestRelated_Alarm_EKS_MatchByContainerInsightsNamespace(t *testing.T) {
	raw := cwtypes.MetricAlarm{
		Namespace: aws.String("ContainerInsights"),
		Dimensions: []cwtypes.Dimension{
			{Name: aws.String("ClusterName"), Value: aws.String("my-eks-cluster")},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"eks": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "my-eks-cluster", Name: "my-eks-cluster"},
		}},
	}

	checker := alarmCheckerByTarget(t, "eks")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (ContainerInsights namespace)", result.Count())
	}
}

// ECS alarm with ClusterName dimension but AWS/ECS namespace — must NOT match EKS.
func TestRelated_Alarm_EKS_NoMatchECSNamespace(t *testing.T) {
	raw := cwtypes.MetricAlarm{
		Namespace: aws.String("AWS/ECS"),
		Dimensions: []cwtypes.Dimension{
			{Name: aws.String("ClusterName"), Value: aws.String("my-ecs-cluster")},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"eks": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "my-ecs-cluster", Name: "my-ecs-cluster"},
		}},
	}

	checker := alarmCheckerByTarget(t, "eks")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (ECS namespace should not match EKS checker)", result.Count())
	}
}

func TestRelated_Alarm_EKS_NoDimension(t *testing.T) {
	raw := cwtypes.MetricAlarm{
		Namespace: aws.String("AWS/EKS"),
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"eks": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "my-eks-cluster", Name: "my-eks-cluster"},
		}},
	}

	checker := alarmCheckerByTarget(t, "eks")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no ClusterName dimension)", result.Count())
	}
}

func TestRelated_Alarm_KMS_MatchByKeyId(t *testing.T) {
	raw := cwtypes.MetricAlarm{
		Namespace: aws.String("AWS/KMS"),
		Dimensions: []cwtypes.Dimension{
			{Name: aws.String("KeyId"), Value: aws.String("mrk-1234567890abcdef1234567890abcdef")},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"kms": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "mrk-1234567890abcdef1234567890abcdef", Name: "mrk-1234567890abcdef1234567890abcdef"},
		}},
	}

	checker := alarmCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "mrk-1234567890abcdef1234567890abcdef" {
		t.Errorf("ResourceIDs = %v, want [mrk-1234567890abcdef1234567890abcdef]", result.ResourceIDs())
	}
}

func TestRelated_Alarm_KMS_NoDimension(t *testing.T) {
	raw := cwtypes.MetricAlarm{}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"kms": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "mrk-1234567890abcdef1234567890abcdef", Name: "mrk-1234567890abcdef1234567890abcdef"},
		}},
	}

	checker := alarmCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_Alarm_Lambda_MatchByFunctionName(t *testing.T) {
	raw := cwtypes.MetricAlarm{
		Namespace: aws.String("AWS/Lambda"),
		Dimensions: []cwtypes.Dimension{
			{Name: aws.String("FunctionName"), Value: aws.String("my-processor")},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "my-processor", Name: "my-processor"},
		}},
	}

	checker := alarmCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "my-processor" {
		t.Errorf("ResourceIDs = %v, want [my-processor]", result.ResourceIDs())
	}
}

func TestRelated_Alarm_Lambda_NoDimension(t *testing.T) {
	raw := cwtypes.MetricAlarm{}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "my-processor", Name: "my-processor"},
		}},
	}

	checker := alarmCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_Alarm_Logs_MatchByLogGroupName(t *testing.T) {
	raw := cwtypes.MetricAlarm{
		Namespace: aws.String("AWS/Logs"),
		Dimensions: []cwtypes.Dimension{
			{Name: aws.String("LogGroupName"), Value: aws.String("/aws/lambda/my-func")},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "/aws/lambda/my-func", Name: "/aws/lambda/my-func"},
		}},
	}

	checker := alarmCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "/aws/lambda/my-func" {
		t.Errorf("ResourceIDs = %v, want [/aws/lambda/my-func]", result.ResourceIDs())
	}
}

func TestRelated_Alarm_Logs_NoDimension(t *testing.T) {
	raw := cwtypes.MetricAlarm{}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "/aws/lambda/my-func", Name: "/aws/lambda/my-func"},
		}},
	}

	checker := alarmCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_Alarm_S3_MatchByBucketName(t *testing.T) {
	raw := cwtypes.MetricAlarm{
		Namespace: aws.String("AWS/S3"),
		Dimensions: []cwtypes.Dimension{
			{Name: aws.String("BucketName"), Value: aws.String("my-data-bucket")},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"s3": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "my-data-bucket", Name: "my-data-bucket"},
		}},
	}

	checker := alarmCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "my-data-bucket" {
		t.Errorf("ResourceIDs = %v, want [my-data-bucket]", result.ResourceIDs())
	}
}

func TestRelated_Alarm_S3_NoDimension(t *testing.T) {
	raw := cwtypes.MetricAlarm{}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	checker := alarmCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

// Step Functions publishes its metrics under the state machine's ARN, which
// names the row the list holds that ARN for.
func TestRelated_Alarm_SFN_MatchByStateMachineArn(t *testing.T) {
	raw := cwtypes.MetricAlarm{
		Namespace: aws.String("AWS/States"),
		Dimensions: []cwtypes.Dimension{
			{Name: aws.String("StateMachineArn"), Value: aws.String("arn:aws:states:us-east-1:123456789012:stateMachine:my-workflow")},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"sfn": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "my-workflow", Name: "my-workflow", Fields: map[string]string{"arn": "arn:aws:states:us-east-1:123456789012:stateMachine:my-workflow"}},
		}},
	}

	checker := alarmCheckerByTarget(t, "sfn")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "my-workflow" {
		t.Errorf("ResourceIDs = %v, want [my-workflow] (name extracted from ARN)", result.ResourceIDs())
	}
}

// A dimension carrying the state machine's name, rather than its ARN, names
// the row of that name.
func TestRelated_Alarm_SFN_MatchByPlainName(t *testing.T) {
	raw := cwtypes.MetricAlarm{
		Namespace: aws.String("AWS/States"),
		Dimensions: []cwtypes.Dimension{
			{Name: aws.String("StateMachineArn"), Value: aws.String("my-workflow")},
		},
	}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"sfn": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "my-workflow", Name: "my-workflow", Fields: map[string]string{"arn": "arn:aws:states:us-east-1:123456789012:stateMachine:my-workflow"}},
		}},
	}

	checker := alarmCheckerByTarget(t, "sfn")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (plain name, no ARN separator)", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "my-workflow" {
		t.Errorf("ResourceIDs = %v, want [my-workflow]", result.ResourceIDs())
	}
}

func TestRelated_Alarm_SFN_NoDimension(t *testing.T) {
	raw := cwtypes.MetricAlarm{}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"sfn": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "my-workflow", Name: "my-workflow", Fields: map[string]string{"arn": "arn:aws:states:us-east-1:123456789012:stateMachine:my-workflow"}},
		}},
	}

	checker := alarmCheckerByTarget(t, "sfn")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

// AWS/WAFV2's WebACL dimension is the ACL's VisibilityConfig.MetricName,
// which the console fills with the ACL's name unless the owner changes it;
// the ACL list carries no metric name, so each ACL is read with GetWebACL.
func TestRelated_Alarm_WAF_MatchByWebACL(t *testing.T) {
	const aclID = "5e6f7a8b-9c0d-4e1f-8a2b-3c4d5e6f7a8b"
	fake := &t568WAF{acl: wafv2types.WebACL{
		Name: aws.String("my-waf-acl"), Id: aws.String(aclID),
		ARN:              aws.String("arn:aws:wafv2:us-east-1:123456789012:regional/webacl/my-waf-acl/" + aclID),
		DefaultAction:    &wafv2types.DefaultAction{Allow: &wafv2types.AllowAction{}},
		VisibilityConfig: &wafv2types.VisibilityConfig{MetricName: aws.String("my-waf-acl"), CloudWatchMetricsEnabled: true, SampledRequestsEnabled: true},
	}}
	acl := resource.Resource{ID: aclID, Name: "my-waf-acl", Type: "waf",
		Fields:    map[string]string{"name": "my-waf-acl", "id": aclID, "scope": "REGIONAL"},
		RawStruct: wafv2types.WebACLSummary{Name: aws.String("my-waf-acl"), Id: aws.String(aclID), ARN: fake.acl.ARN}}
	cache := resource.ResourceCache{"waf": {Resources: []resource.Resource{acl}}}
	alarm := t568Alarm("test-alarm", "AWS/WAFV2", "BlockedRequests", "WebACL", "my-waf-acl", "Region", "us-east-1", "Rule", "ALL")
	clients := &awsclient.ServiceClients{WAFv2: fake, Region: "us-east-1"}

	t568RequireExact(t, "alarm → waf", alarmCheckerByTarget(t, "waf")(context.Background(), clients, alarm, cache), aclID)
}

func TestRelated_Alarm_WAF_NoDimension(t *testing.T) {
	raw := cwtypes.MetricAlarm{}
	res := resource.Resource{ID: "test-alarm", RawStruct: raw}

	cache := resource.ResourceCache{
		"waf": resource.ResourceCacheEntry{Resources: []resource.Resource{
			{ID: "my-waf-acl", Name: "my-waf-acl"},
		}},
	}

	checker := alarmCheckerByTarget(t, "waf")
	result := checker(context.Background(), nil, res, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_Alarm_CTEvents_NoMatchWhenEventNameLacksAlarm(t *testing.T) {
	// docs/resources/alarm.md, `ct-events`: the field is Fields["source"].
	evRes := resource.Resource{
		ID: "ct-event-abc",
		Fields: map[string]string{
			"source":     "monitoring.amazonaws.com",
			"event_name": "DescribeMetrics",
		},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{Resources: []resource.Resource{evRes}},
	}
	src := resource.Resource{ID: "my-alarm", Fields: map[string]string{}}

	checker := alarmCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, src, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (event_name does not contain 'Alarm')", result.Count())
	}
}

func TestRelated_Alarm_CTEvents_EmptySourceID(t *testing.T) {
	checker := alarmCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, resource.Resource{ID: ""}, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty alarm name)", result.Count())
	}
}
