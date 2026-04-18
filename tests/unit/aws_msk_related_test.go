package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func mskCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("msk") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("msk related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("msk related checker for %s not found", target)
	return nil
}

// --- Navigable Fields ---

func TestNavigableFields_MSK_KmsKey(t *testing.T) {
	nav := resource.IsFieldNavigable("msk", "Provisioned.EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId")
	if nav == nil {
		t.Fatal("expected Provisioned.EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId to be navigable for msk")
	}
	if nav.TargetType != "kms" {
		t.Errorf("expected TargetType=kms, got %q", nav.TargetType)
	}
}

// --- CloudWatch Alarms checker (Pattern C — cache, "Cluster Name" dimension) ---

func TestRelated_MSK_Alarms_Found(t *testing.T) {
	const clusterName = "analytics-kafka-cluster"

	alarmRes := resource.Resource{
		ID: "msk-cpu-utilization",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("msk-cpu-utilization"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("Cluster Name"), Value: aws.String(clusterName)},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   clusterName,
		Name: clusterName,
		Fields: map[string]string{
			"cluster_name": clusterName,
		},
	}

	checker := mskCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "msk-cpu-utilization" {
		t.Errorf("ResourceIDs = %v, want [msk-cpu-utilization]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_MSK_Alarms_NotFound(t *testing.T) {
	const clusterName = "analytics-kafka-cluster"

	alarmRes := resource.Resource{
		ID: "other-cluster-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("other-cluster-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("Cluster Name"), Value: aws.String("different-cluster")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	source := resource.Resource{
		ID:   clusterName,
		Name: clusterName,
		Fields: map[string]string{
			"cluster_name": clusterName,
		},
	}

	checker := mskCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_MSK_Alarms_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "analytics-kafka-cluster",
		Name: "analytics-kafka-cluster",
		Fields: map[string]string{
			"cluster_name": "analytics-kafka-cluster",
		},
	}

	checker := mskCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

// --- checkMSKLambda (scan lambda cache for event_source_arn match) ---

func TestRelated_MSK_Lambda_Found(t *testing.T) {
	const clusterARN = "arn:aws:kafka:us-east-1:123456789012:cluster/analytics-kafka-cluster/abc-123"
	lambdaRes := resource.Resource{
		ID:   "kafka-consumer",
		Name: "kafka-consumer",
		Fields: map[string]string{
			"event_source_arn": clusterARN,
		},
	}
	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{lambdaRes}},
	}
	source := resource.Resource{
		ID:   "analytics-kafka-cluster",
		Name: "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{
			ClusterName: aws.String("analytics-kafka-cluster"),
			ClusterArn:  aws.String(clusterARN),
		},
	}

	checker := mskCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "kafka-consumer" {
		t.Errorf("ResourceIDs = %v, want [kafka-consumer]", result.ResourceIDs)
	}
}

func TestRelated_MSK_Lambda_NotFound(t *testing.T) {
	lambdaRes := resource.Resource{
		ID:   "unrelated-fn",
		Name: "unrelated-fn",
		Fields: map[string]string{
			"event_source_arn": "arn:aws:sqs:us-east-1:123456789012:other-queue",
		},
	}
	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{Resources: []resource.Resource{lambdaRes}},
	}
	source := resource.Resource{
		ID:   "analytics-kafka-cluster",
		Name: "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{
			ClusterName: aws.String("analytics-kafka-cluster"),
			ClusterArn:  aws.String("arn:aws:kafka:us-east-1:123456789012:cluster/analytics-kafka-cluster/abc-123"),
		},
	}

	checker := mskCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no lambda event-source match)", result.Count)
	}
}

func TestRelated_MSK_Lambda_NoClusterARN(t *testing.T) {
	source := resource.Resource{
		ID:   "analytics-kafka-cluster",
		Name: "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{
			ClusterName: aws.String("analytics-kafka-cluster"),
			ClusterArn:  nil,
		},
	}
	checker := mskCheckerByTarget(t, "lambda")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no cluster ARN)", result.Count)
	}
}

// --- checkMSKCFN (Tags map, match aws:cloudformation:stack-name to CFN cache) ---

func TestRelated_MSK_CFN_Found(t *testing.T) {
	cfnRes := resource.Resource{
		ID:   "analytics-stack",
		Name: "analytics-stack",
		RawStruct: cfntypes.Stack{
			StackName: aws.String("analytics-stack"),
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes}},
	}
	source := resource.Resource{
		ID:   "analytics-kafka-cluster",
		Name: "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{
			ClusterName: aws.String("analytics-kafka-cluster"),
			Tags: map[string]string{
				"aws:cloudformation:stack-name": "analytics-stack",
			},
		},
	}

	checker := mskCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "analytics-stack" {
		t.Errorf("ResourceIDs = %v, want [analytics-stack]", result.ResourceIDs)
	}
}

func TestRelated_MSK_CFN_NoTag(t *testing.T) {
	cfnRes := resource.Resource{
		ID:   "analytics-stack",
		Name: "analytics-stack",
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes}},
	}
	source := resource.Resource{
		ID:   "analytics-kafka-cluster",
		Name: "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{
			ClusterName: aws.String("analytics-kafka-cluster"),
			Tags:        map[string]string{},
		},
	}

	checker := mskCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, cache)
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no cfn stack tag)", result.Count)
	}
}

func TestRelated_MSK_CFN_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "analytics-kafka-cluster",
		Name: "analytics-kafka-cluster",
		RawStruct: kafkatypes.Cluster{
			ClusterName: aws.String("analytics-kafka-cluster"),
			Tags: map[string]string{
				"aws:cloudformation:stack-name": "analytics-stack",
			},
		},
	}
	checker := mskCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (cache miss)", result.Count)
	}
}
