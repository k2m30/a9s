// aws_ecs_services_related_test.go contains unit tests for ECS Services related-resource checkers.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func ecsSvcCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("ecs-svc") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("ecs-svc related checker for %s is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("ecs-svc related checker for %s not found", target)
	return nil
}

// --- ECS Cluster checker (Pattern F — Fields-based) ---

func TestRelated_ECSSvc_Cluster_FromFields(t *testing.T) {
	checker := ecsSvcCheckerByTarget(t, "ecs")
	res := resource.Resource{
		ID:     "api-gateway",
		Fields: map[string]string{"cluster": "acme-services"},
	}
	result := checker(context.Background(), nil, res, nil)
	if result.Count != 1 {
		t.Fatalf("expected Count=1, got %d", result.Count)
	}
	if result.ResourceIDs[0] != "acme-services" {
		t.Errorf("expected ResourceIDs[0]=%q, got %q", "acme-services", result.ResourceIDs[0])
	}
}

func TestRelated_ECSSvc_Cluster_EmptyField(t *testing.T) {
	checker := ecsSvcCheckerByTarget(t, "ecs")
	res := resource.Resource{
		ID:     "api-gateway",
		Fields: map[string]string{},
	}
	result := checker(context.Background(), nil, res, nil)
	if result.Count != 0 {
		t.Errorf("expected Count=0 for empty cluster field, got %d", result.Count)
	}
}

// --- Target Groups checker (Pattern F — struct-based, LoadBalancers[].TargetGroupArn) ---

func TestRelated_ECSSvc_TargetGroups_FromLoadBalancers(t *testing.T) {
	tgArn := "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/api-tg/abc123"
	svc := ecstypes.Service{
		ServiceName: aws.String("api-service"),
		ClusterArn:  aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster"),
		LoadBalancers: []ecstypes.LoadBalancer{
			{TargetGroupArn: aws.String(tgArn)},
		},
	}
	res := resource.Resource{
		ID:        "api-service",
		Name:      "api-service",
		Fields:    map[string]string{"cluster": "my-cluster"},
		RawStruct: svc,
	}

	checker := ecsSvcCheckerByTarget(t, "tg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 {
		t.Fatalf("ResourceIDs len = %d, want 1", len(result.ResourceIDs))
	}
	// The name extracted from the ARN should be "api-tg"
	if result.ResourceIDs[0] != "api-tg" {
		t.Errorf("ResourceIDs[0] = %q, want %q", result.ResourceIDs[0], "api-tg")
	}
}

func TestRelated_ECSSvc_TargetGroups_NoLoadBalancers(t *testing.T) {
	svc := ecstypes.Service{
		ServiceName:   aws.String("api-service"),
		ClusterArn:    aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster"),
		LoadBalancers: []ecstypes.LoadBalancer{},
	}
	res := resource.Resource{
		ID:        "api-service",
		Name:      "api-service",
		Fields:    map[string]string{"cluster": "my-cluster"},
		RawStruct: svc,
	}

	checker := ecsSvcCheckerByTarget(t, "tg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ECSSvc_TargetGroups_InvalidRawStruct(t *testing.T) {
	res := resource.Resource{
		ID:        "api-service",
		Fields:    map[string]string{"cluster": "my-cluster"},
		RawStruct: "not-a-service",
	}

	checker := ecsSvcCheckerByTarget(t, "tg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 for invalid RawStruct", result.Count)
	}
}

// --- CloudWatch Alarms checker (Pattern C — cache, ServiceName+ClusterName dimensions) ---

func TestRelated_ECSSvc_Alarms_MatchServiceAndCluster(t *testing.T) {
	serviceName := "api-service"
	clusterName := "my-cluster"
	alarmRes := resource.Resource{
		ID: "ecs-svc-cpu-high",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("ecs-svc-cpu-high"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("ServiceName"), Value: aws.String(serviceName)},
				{Name: aws.String("ClusterName"), Value: aws.String(clusterName)},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	res := resource.Resource{
		ID:     serviceName,
		Name:   serviceName,
		Fields: map[string]string{"cluster": clusterName, "service_name": serviceName},
		RawStruct: ecstypes.Service{
			ServiceName: aws.String(serviceName),
			ClusterArn:  aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/" + clusterName),
		},
	}

	checker := ecsSvcCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "ecs-svc-cpu-high" {
		t.Errorf("ResourceIDs = %v, want [ecs-svc-cpu-high]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_ECSSvc_Alarms_NoMatch(t *testing.T) {
	alarmRes := resource.Resource{
		ID: "other-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("other-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("ServiceName"), Value: aws.String("different-service")},
				{Name: aws.String("ClusterName"), Value: aws.String("my-cluster")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	res := resource.Resource{
		ID:     "api-service",
		Name:   "api-service",
		Fields: map[string]string{"cluster": "my-cluster", "service_name": "api-service"},
		RawStruct: ecstypes.Service{
			ServiceName: aws.String("api-service"),
			ClusterArn:  aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/my-cluster"),
		},
	}

	checker := ecsSvcCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0", result.Count)
	}
}

func TestRelated_ECSSvc_Alarms_CacheMissNoClients(t *testing.T) {
	res := resource.Resource{
		ID:     "api-service",
		Name:   "api-service",
		Fields: map[string]string{"cluster": "my-cluster"},
		RawStruct: ecstypes.Service{
			ServiceName: aws.String("api-service"),
		},
	}

	checker := ecsSvcCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}

func TestRelated_ECSSvc_Alarms_EmptyServiceID(t *testing.T) {
	alarmRes := resource.Resource{
		ID: "ecs-svc-cpu-high",
		RawStruct: cwtypes.MetricAlarm{
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("ServiceName"), Value: aws.String("api-service")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{Resources: []resource.Resource{alarmRes}},
	}
	res := resource.Resource{
		ID:        "",
		Fields:    map[string]string{},
		RawStruct: ecstypes.Service{},
	}

	checker := ecsSvcCheckerByTarget(t, "alarm")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for empty service ID", result.Count)
	}
}

// --- CloudFormation Stacks checker (Pattern C — cache, aws:cloudformation:stack-name tag) ---

func TestRelated_ECSSvc_CFN_FromTags(t *testing.T) {
	cfnRes := resource.Resource{
		ID:   "my-stack",
		Name: "my-stack",
		RawStruct: cfntypes.Stack{
			StackName: aws.String("my-stack"),
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes}},
	}
	res := resource.Resource{
		ID:     "api-service",
		Name:   "api-service",
		Fields: map[string]string{"cluster": "my-cluster"},
		RawStruct: ecstypes.Service{
			ServiceName: aws.String("api-service"),
			Tags: []ecstypes.Tag{
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("my-stack")},
			},
		},
	}

	checker := ecsSvcCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-stack" {
		t.Errorf("ResourceIDs = %v, want [my-stack]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

func TestRelated_ECSSvc_CFN_NoTag(t *testing.T) {
	cfnRes := resource.Resource{
		ID:   "some-stack",
		Name: "some-stack",
		RawStruct: cfntypes.Stack{
			StackName: aws.String("some-stack"),
		},
	}
	cache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{Resources: []resource.Resource{cfnRes}},
	}
	res := resource.Resource{
		ID:     "api-service",
		Name:   "api-service",
		Fields: map[string]string{"cluster": "my-cluster"},
		RawStruct: ecstypes.Service{
			ServiceName: aws.String("api-service"),
			Tags:        []ecstypes.Tag{},
		},
	}

	checker := ecsSvcCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for service with no CFN tag", result.Count)
	}
}

func TestRelated_ECSSvc_CFN_CacheMissNoClients(t *testing.T) {
	res := resource.Resource{
		ID:     "api-service",
		Name:   "api-service",
		Fields: map[string]string{"cluster": "my-cluster"},
		RawStruct: ecstypes.Service{
			ServiceName: aws.String("api-service"),
			Tags: []ecstypes.Tag{
				{Key: aws.String("aws:cloudformation:stack-name"), Value: aws.String("my-stack")},
			},
		},
	}

	checker := ecsSvcCheckerByTarget(t, "cfn")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count)
	}
}
