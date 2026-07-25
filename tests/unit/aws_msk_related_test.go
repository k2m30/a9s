package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
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
	nav := resource.IsFieldNavigableForTest("msk", "Provisioned.EncryptionInfo.EncryptionAtRest.DataVolumeKMSKeyId")
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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "msk-cpu-utilization" {
		t.Errorf("ResourceIDs = %v, want [msk-cpu-utilization]", result.ResourceIDs())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
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

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count())
	}
}

// --- checkMSKLambda (checker-owned lambda:ListEventSourceMappings call
// filtered by EventSourceArn=<ClusterARN>, mapped against the lambda cache —
// spec docs/resources/msk.md §lambda L46-49) ---

func TestRelated_MSK_Lambda_Found(t *testing.T) {
	const clusterARN = "arn:aws:kafka:us-east-1:123456789012:cluster/analytics-kafka-cluster/abc-123"
	const fnArn = "arn:aws:lambda:us-east-1:123456789012:function:kafka-consumer"

	fake := &fakeLambdaListEventSourceMappingsByArn{
		wantEventSourceArn: clusterARN,
		mappings: []lambdatypes.EventSourceMappingConfiguration{
			{FunctionArn: aws.String(fnArn)},
		},
	}
	clients := &awsclient.ServiceClients{Lambda: fake}

	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "kafka-consumer", Name: "kafka-consumer", Fields: map[string]string{"arn": fnArn}},
			},
		},
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
	result := checker(context.Background(), clients, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "kafka-consumer" {
		t.Errorf("ResourceIDs = %v, want [kafka-consumer]", result.ResourceIDs())
	}
	if fake.calls != 1 {
		t.Errorf("ListEventSourceMappings called %d times, want exactly 1", fake.calls)
	}
}

// TestRelated_MSK_Lambda_MappedFnNotInCache_FallsBackToARNBareName pins the
// union contract (core/aws/related_common.go
// lambdaEventSourceMappingLambdaCheck): a ListEventSourceMappings-confirmed
// FunctionArn that is NOT the one resolved in a non-truncated lambda
// ResourceCache is not dropped — it is still counted via the bare function
// name parsed from its own ARN. Renamed from TestRelated_MSK_Lambda_NotFound
// (pre-fix behavior asserted a definitive Count=0 here; the API result is
// authoritative regardless of cache membership).
func TestRelated_MSK_Lambda_MappedFnNotInCache_FallsBackToARNBareName(t *testing.T) {
	const clusterARN = "arn:aws:kafka:us-east-1:123456789012:cluster/analytics-kafka-cluster/abc-123"
	const mappedFnArn = "arn:aws:lambda:us-east-1:123456789012:function:kafka-consumer"
	const cachedFnArn = "arn:aws:lambda:us-east-1:123456789012:function:unrelated-fn"

	fake := &fakeLambdaListEventSourceMappingsByArn{
		wantEventSourceArn: clusterARN,
		mappings: []lambdatypes.EventSourceMappingConfiguration{
			{FunctionArn: aws.String(mappedFnArn)},
		},
	}
	clients := &awsclient.ServiceClients{Lambda: fake}

	cache := resource.ResourceCache{
		"lambda": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "unrelated-fn", Name: "unrelated-fn", Fields: map[string]string{"arn": cachedFnArn}},
			},
		},
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
	result := checker(context.Background(), clients, source, cache)
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1 (the API-confirmed mapping is authoritative even though its function isn't the one cached — union contract)", result.Count())
	}
	found := false
	for _, id := range result.ResourceIDs() {
		if id == "kafka-consumer" {
			found = true
		}
	}
	if !found {
		t.Errorf("ResourceIDs = %v, want to contain %q (bare function name parsed from the cache-missing mapping's own ARN)", result.ResourceIDs(), "kafka-consumer")
	}
	if fake.calls != 1 {
		t.Errorf("ListEventSourceMappings called %d times, want exactly 1", fake.calls)
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
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no cluster ARN)", result.Count())
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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "analytics-stack" {
		t.Errorf("ResourceIDs = %v, want [analytics-stack]", result.ResourceIDs())
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
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no cfn stack tag)", result.Count())
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
	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (cache miss)", result.Count())
	}
}
