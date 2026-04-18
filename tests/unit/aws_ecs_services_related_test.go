// aws_ecs_services_related_test.go contains unit tests for ECS Services related-resource checkers.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	sfnsvc "github.com/aws/aws-sdk-go-v2/service/sfn"

	_ "github.com/k2m30/a9s/v3/internal/aws"
	awsclient "github.com/k2m30/a9s/v3/internal/aws"
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

// ---------------------------------------------------------------------------
// ecs-svc→eb-rule (Pattern C+reverse: cache["eb-rule"] scan, EventPattern match)
// ---------------------------------------------------------------------------

// ecsSvcSourceResource builds an ECS service resource used as the parent.
func ecsSvcSourceResource(serviceName, clusterName, taskDefARN string) resource.Resource {
	return resource.Resource{
		ID:   serviceName,
		Name: serviceName,
		Fields: map[string]string{
			"cluster":     clusterName,
			"service_arn": "arn:aws:ecs:us-east-1:123456789012:service/" + clusterName + "/" + serviceName,
		},
		RawStruct: ecstypes.Service{
			ServiceName:    aws.String(serviceName),
			ClusterArn:     aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/" + clusterName),
			TaskDefinition: aws.String(taskDefARN),
		},
	}
}

// ecsEbRuleResource builds an eb-rule cache entry with an EventPattern that
// references the given ECS service by group ("service:{svcName}") and cluster ARN.
func ecsEbRuleResource(ruleName, svcName, clusterName string) resource.Resource {
	pattern := `{"source":["aws.ecs"],"detail":{"group":["service:` + svcName + `"],"clusterArn":["arn:aws:ecs:us-east-1:123456789012:cluster/` + clusterName + `"]}}`
	return resource.Resource{
		ID:   ruleName,
		Name: ruleName,
		RawStruct: eventbridgetypes.Rule{
			Name:         aws.String(ruleName),
			EventPattern: aws.String(pattern),
		},
	}
}

// ecsEbRuleResourceUnrelated builds an eb-rule cache entry for a different ECS service.
func ecsEbRuleResourceUnrelated(ruleName string) resource.Resource {
	pattern := `{"source":["aws.ecs"],"detail":{"group":["service:other-service"]}}`
	return resource.Resource{
		ID:   ruleName,
		Name: ruleName,
		RawStruct: eventbridgetypes.Rule{
			Name:         aws.String(ruleName),
			EventPattern: aws.String(pattern),
		},
	}
}

// TestRelated_ECSSvc_EbRule_Match verifies that an EventBridge rule whose EventPattern
// references this ECS service by group name is returned with Count=1.
func TestRelated_ECSSvc_EbRule_Match(t *testing.T) {
	const svcName = "api-service"
	const clusterName = "prod-cluster"
	const ruleName = "ecs-api-rule"

	cache := resource.ResourceCache{
		"eb-rule": resource.ResourceCacheEntry{
			Resources: []resource.Resource{ecsEbRuleResource(ruleName, svcName, clusterName)},
		},
	}

	checker := ecsSvcCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, ecsSvcSourceResource(svcName, clusterName, "arn:aws:ecs:us-east-1:123456789012:task-definition/api:5"), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != ruleName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, ruleName)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_ECSSvc_EbRule_Match_Truncated verifies that IsTruncated propagates
// to Approximate=true while Count still reflects found matches.
func TestRelated_ECSSvc_EbRule_Match_Truncated(t *testing.T) {
	const svcName = "api-service"
	const clusterName = "prod-cluster"
	const ruleName = "ecs-api-rule"

	cache := resource.ResourceCache{
		"eb-rule": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{ecsEbRuleResource(ruleName, svcName, clusterName)},
			IsTruncated: true,
		},
	}

	checker := ecsSvcCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, ecsSvcSourceResource(svcName, clusterName, "arn:aws:ecs:us-east-1:123456789012:task-definition/api:5"), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if !result.Approximate {
		t.Error("Approximate = false, want true (cache is truncated)")
	}
}

// TestRelated_ECSSvc_EbRule_Empty verifies that a cache containing only unrelated
// rules returns Count=0.
func TestRelated_ECSSvc_EbRule_Empty(t *testing.T) {
	const svcName = "api-service"
	const clusterName = "prod-cluster"

	cache := resource.ResourceCache{
		"eb-rule": resource.ResourceCacheEntry{
			Resources: []resource.Resource{ecsEbRuleResourceUnrelated("unrelated-rule")},
		},
	}

	checker := ecsSvcCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, ecsSvcSourceResource(svcName, clusterName, ""), cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no rules reference this service)", result.Count)
	}
}

// TestRelated_ECSSvc_EbRule_MissingCache verifies that a missing "eb-rule" cache
// key returns the zero-value (Count=0, not -1), per the cache-miss contract.
func TestRelated_ECSSvc_EbRule_MissingCache(t *testing.T) {
	checker := ecsSvcCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, ecsSvcSourceResource("api-service", "prod-cluster", ""), resource.ResourceCache{})

	// cache miss → entry not present, checker returns RelatedCheckResult{} which has Count=0
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (cache key missing returns zero-value)", result.Count)
	}
}

// TestRelated_ECSSvc_EbRule_FetchFilter verifies that the checker does NOT populate
// FetchFilter — reverse-scan checkers must not set FetchFilter (Fix 3).
func TestRelated_ECSSvc_EbRule_FetchFilter(t *testing.T) {
	const svcName = "api-service"

	cache := resource.ResourceCache{
		"eb-rule": resource.ResourceCacheEntry{Resources: []resource.Resource{}},
	}

	checker := ecsSvcCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, ecsSvcSourceResource(svcName, "prod-cluster", ""), cache)

	if len(result.FetchFilter) != 0 {
		t.Errorf("FetchFilter = %v, want empty (reverse-scan checkers must not set FetchFilter)", result.FetchFilter)
	}
}

// ---------------------------------------------------------------------------
// ecs-svc→sfn (Pattern C+reverse: cache["sfn"] scan, sfnDescribe + ASL parse)
// ---------------------------------------------------------------------------

// fakeSFNBatch4 satisfies awsclient.SFNAPI via embedding. Only DescribeStateMachine
// is overridden — it returns the pre-configured output keyed by state machine ARN.
type fakeSFNBatch4 struct {
	awsclient.SFNAPI
	describeOutputByARN map[string]*sfnsvc.DescribeStateMachineOutput
}

func (f *fakeSFNBatch4) DescribeStateMachine(_ context.Context, input *sfnsvc.DescribeStateMachineInput, _ ...func(*sfnsvc.Options)) (*sfnsvc.DescribeStateMachineOutput, error) {
	if input.StateMachineArn == nil {
		return &sfnsvc.DescribeStateMachineOutput{}, nil
	}
	if f.describeOutputByARN != nil {
		if out, ok := f.describeOutputByARN[*input.StateMachineArn]; ok {
			return out, nil
		}
	}
	return &sfnsvc.DescribeStateMachineOutput{}, nil
}

// sfnResourceWithARN builds a cache entry for cache["sfn"] with an ARN in Fields.
func sfnResourceWithARN(name, arn string) resource.Resource {
	return resource.Resource{
		ID:   name,
		Name: name,
		Fields: map[string]string{
			"arn": arn,
		},
	}
}

// sfnASLWithECSFamily returns an ASL definition JSON that contains a Task state
// calling ecs:runTask with the given task definition family.
func sfnASLWithECSFamily(family string) string {
	return `{"Comment":"test","StartAt":"Run","States":{"Run":{"Type":"Task","Resource":"arn:aws:states:::ecs:runTask.sync","Parameters":{"TaskDefinition":"` + family + `","LaunchType":"FARGATE"},"End":true}}}`
}

// TestRelated_ECSSvc_SFN_Match verifies that a state machine whose ASL definition
// references this service's task definition family is returned with Count=1.
func TestRelated_ECSSvc_SFN_Match(t *testing.T) {
	const svcName = "api-service"
	const clusterName = "prod-cluster"
	const taskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:5"
	const sfnARN = "arn:aws:states:us-east-1:123456789012:stateMachine:api-pipeline"

	fakeSFN := &fakeSFNBatch4{
		describeOutputByARN: map[string]*sfnsvc.DescribeStateMachineOutput{
			sfnARN: {
				StateMachineArn: aws.String(sfnARN),
				Definition:      aws.String(sfnASLWithECSFamily("api-task")),
			},
		},
	}
	clients := &awsclient.ServiceClients{SFN: fakeSFN}

	cache := resource.ResourceCache{
		"sfn": resource.ResourceCacheEntry{
			Resources: []resource.Resource{sfnResourceWithARN("api-pipeline", sfnARN)},
		},
	}

	checker := ecsSvcCheckerByTarget(t, "sfn")
	result := checker(context.Background(), clients, ecsSvcSourceResource(svcName, clusterName, taskDefARN), cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "api-pipeline" {
		t.Errorf("ResourceIDs = %v, want [api-pipeline]", result.ResourceIDs)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_ECSSvc_SFN_Empty verifies that a state machine with a non-matching
// task family returns Count=0.
func TestRelated_ECSSvc_SFN_Empty(t *testing.T) {
	const taskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:5"
	const sfnARN = "arn:aws:states:us-east-1:123456789012:stateMachine:other-pipeline"

	fakeSFN := &fakeSFNBatch4{
		describeOutputByARN: map[string]*sfnsvc.DescribeStateMachineOutput{
			sfnARN: {
				Definition: aws.String(sfnASLWithECSFamily("totally-different-task")),
			},
		},
	}
	clients := &awsclient.ServiceClients{SFN: fakeSFN}

	cache := resource.ResourceCache{
		"sfn": resource.ResourceCacheEntry{
			Resources: []resource.Resource{sfnResourceWithARN("other-pipeline", sfnARN)},
		},
	}

	checker := ecsSvcCheckerByTarget(t, "sfn")
	result := checker(context.Background(), clients, ecsSvcSourceResource("api-service", "prod-cluster", taskDefARN), cache)

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (task family does not match)", result.Count)
	}
}

// TestRelated_ECSSvc_SFN_MissingCache verifies that a missing "sfn" cache key
// returns the zero-value (Count=0), not Count=-1.
func TestRelated_ECSSvc_SFN_MissingCache(t *testing.T) {
	const taskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:5"

	checker := ecsSvcCheckerByTarget(t, "sfn")
	result := checker(context.Background(), nil, ecsSvcSourceResource("api-service", "prod-cluster", taskDefARN), resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (cache key missing returns zero-value)", result.Count)
	}
}

// TestRelated_ECSSvc_SFN_WrongRawStruct verifies that a wrong parent RawStruct
// type returns Count=-1 (assertStruct guard).
func TestRelated_ECSSvc_SFN_WrongRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "api-service",
		RawStruct: "not-a-service",
	}
	checker := ecsSvcCheckerByTarget(t, "sfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// ecs-svc→ecr (Pattern A: DescribeTaskDefinition → container image ECR URIs)
// ---------------------------------------------------------------------------

// ecsSvcWithTaskDef builds an ECS service resource with a TaskDefinition ARN set.
func ecsSvcWithTaskDef(svcName, taskDefARN string) resource.Resource {
	return resource.Resource{
		ID:   svcName,
		Name: svcName,
		Fields: map[string]string{
			"cluster": "prod-cluster",
		},
		RawStruct: ecstypes.Service{
			ServiceName:    aws.String(svcName),
			ClusterArn:     aws.String("arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster"),
			TaskDefinition: aws.String(taskDefARN),
		},
	}
}

// TestRelated_ECSSvc_ECR_Match verifies that when DescribeTaskDefinition returns
// a container image referencing an ECR URI, the repo name is returned in ResourceIDs.
func TestRelated_ECSSvc_ECR_Match(t *testing.T) {
	const taskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:5"
	const ecrImage = "123456789012.dkr.ecr.us-east-1.amazonaws.com/my-repo:v1.2.3"
	const expectedRepo = "my-repo"

	td := &ecstypes.TaskDefinition{
		ContainerDefinitions: []ecstypes.ContainerDefinition{
			{Image: aws.String(ecrImage)},
		},
	}
	fakeECS := newFakeECSWithTaskDefinition(td)
	clients := &awsclient.ServiceClients{ECS: fakeECS}

	checker := ecsSvcCheckerByTarget(t, "ecr")
	result := checker(context.Background(), clients, ecsSvcWithTaskDef("api-service", taskDefARN), resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != expectedRepo {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, expectedRepo)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_ECSSvc_ECR_NoECRImages verifies that when DescribeTaskDefinition
// returns only non-ECR images (e.g. nginx:latest), Count=0.
func TestRelated_ECSSvc_ECR_NoECRImages(t *testing.T) {
	const taskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:5"

	td := &ecstypes.TaskDefinition{
		ContainerDefinitions: []ecstypes.ContainerDefinition{
			{Image: aws.String("nginx:latest")},
			{Image: aws.String("redis:7-alpine")},
		},
	}
	fakeECS := newFakeECSWithTaskDefinition(td)
	clients := &awsclient.ServiceClients{ECS: fakeECS}

	checker := ecsSvcCheckerByTarget(t, "ecr")
	result := checker(context.Background(), clients, ecsSvcWithTaskDef("api-service", taskDefARN), resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no ECR images)", result.Count)
	}
}

// TestRelated_ECSSvc_ECR_MultipleContainers verifies that when multiple containers
// reference ECR images, all distinct repo names are returned.
func TestRelated_ECSSvc_ECR_MultipleContainers(t *testing.T) {
	const taskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/multi-task:3"

	td := &ecstypes.TaskDefinition{
		ContainerDefinitions: []ecstypes.ContainerDefinition{
			{Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/app-repo:v2")},
			{Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/sidecar-repo:latest")},
		},
	}
	fakeECS := newFakeECSWithTaskDefinition(td)
	clients := &awsclient.ServiceClients{ECS: fakeECS}

	checker := ecsSvcCheckerByTarget(t, "ecr")
	result := checker(context.Background(), clients, ecsSvcWithTaskDef("multi-service", taskDefARN), resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	if len(result.ResourceIDs) != 2 {
		t.Errorf("ResourceIDs = %v, want 2 entries", result.ResourceIDs)
	}
}

// TestRelated_ECSSvc_ECR_NoTaskDef verifies that a service with no TaskDefinition
// set returns Count=0 (not an error).
func TestRelated_ECSSvc_ECR_NoTaskDef(t *testing.T) {
	source := resource.Resource{
		ID:   "api-service",
		Name: "api-service",
		Fields: map[string]string{"cluster": "prod-cluster"},
		RawStruct: ecstypes.Service{
			ServiceName: aws.String("api-service"),
		},
	}
	checker := ecsSvcCheckerByTarget(t, "ecr")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no task definition)", result.Count)
	}
}

// TestRelated_ECSSvc_ECR_NoClient verifies that nil/missing clients returns Count=-1.
func TestRelated_ECSSvc_ECR_NoClient(t *testing.T) {
	const taskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:5"

	checker := ecsSvcCheckerByTarget(t, "ecr")
	result := checker(context.Background(), nil, ecsSvcWithTaskDef("api-service", taskDefARN), resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no ECS client)", result.Count)
	}
}

// TestRelated_ECSSvc_ECR_WrongRawStruct verifies that a wrong parent RawStruct
// type returns Count=-1 (assertStruct guard).
func TestRelated_ECSSvc_ECR_WrongRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "api-service",
		RawStruct: "not-a-service",
	}
	checker := ecsSvcCheckerByTarget(t, "ecr")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// ecs-svc→secrets (Pattern A: DescribeTaskDefinition → Secrets[].ValueFrom ARNs)
// ---------------------------------------------------------------------------

// TestRelated_ECSSvc_Secrets_Match verifies that secretsmanager ARNs in
// Secrets[].ValueFrom are returned as ResourceIDs.
func TestRelated_ECSSvc_Secrets_Match(t *testing.T) {
	const taskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:5"
	const secretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:my-db-password-aBcDef"

	td := &ecstypes.TaskDefinition{
		ContainerDefinitions: []ecstypes.ContainerDefinition{
			{
				Image: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/app:v1"),
				Secrets: []ecstypes.Secret{
					{
						Name:      aws.String("DB_PASSWORD"),
						ValueFrom: aws.String(secretARN),
					},
				},
			},
		},
	}
	fakeECS := newFakeECSWithTaskDefinition(td)
	clients := &awsclient.ServiceClients{ECS: fakeECS}

	checker := ecsSvcCheckerByTarget(t, "secrets")
	result := checker(context.Background(), clients, ecsSvcWithTaskDef("api-service", taskDefARN), resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != secretARN {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, secretARN)
	}
	if result.Err != nil {
		t.Errorf("unexpected error: %v", result.Err)
	}
}

// TestRelated_ECSSvc_Secrets_RepositoryCredentials verifies that a
// RepositoryCredentials.CredentialsParameter secretsmanager ARN is also returned.
func TestRelated_ECSSvc_Secrets_RepositoryCredentials(t *testing.T) {
	const taskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:5"
	const credARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:ecr-creds-xYzAbC"

	td := &ecstypes.TaskDefinition{
		ContainerDefinitions: []ecstypes.ContainerDefinition{
			{
				Image: aws.String("private-registry.example.com/app:v1"),
				RepositoryCredentials: &ecstypes.RepositoryCredentials{
					CredentialsParameter: aws.String(credARN),
				},
			},
		},
	}
	fakeECS := newFakeECSWithTaskDefinition(td)
	clients := &awsclient.ServiceClients{ECS: fakeECS}

	checker := ecsSvcCheckerByTarget(t, "secrets")
	result := checker(context.Background(), clients, ecsSvcWithTaskDef("api-service", taskDefARN), resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != credARN {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, credARN)
	}
}

// TestRelated_ECSSvc_Secrets_NoSecrets verifies that a task definition with no
// secret references returns Count=0.
func TestRelated_ECSSvc_Secrets_NoSecrets(t *testing.T) {
	const taskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:5"

	td := &ecstypes.TaskDefinition{
		ContainerDefinitions: []ecstypes.ContainerDefinition{
			{Image: aws.String("nginx:latest")},
		},
	}
	fakeECS := newFakeECSWithTaskDefinition(td)
	clients := &awsclient.ServiceClients{ECS: fakeECS}

	checker := ecsSvcCheckerByTarget(t, "secrets")
	result := checker(context.Background(), clients, ecsSvcWithTaskDef("api-service", taskDefARN), resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no secret references)", result.Count)
	}
}

// TestRelated_ECSSvc_Secrets_NoTaskDef verifies that a service with no TaskDefinition
// set returns Count=0 (not an error).
func TestRelated_ECSSvc_Secrets_NoTaskDef(t *testing.T) {
	source := resource.Resource{
		ID:   "api-service",
		Name: "api-service",
		Fields: map[string]string{"cluster": "prod-cluster"},
		RawStruct: ecstypes.Service{
			ServiceName: aws.String("api-service"),
		},
	}
	checker := ecsSvcCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no task definition)", result.Count)
	}
}

// TestRelated_ECSSvc_Secrets_NoClient verifies that nil/missing clients returns Count=-1.
func TestRelated_ECSSvc_Secrets_NoClient(t *testing.T) {
	const taskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:5"

	checker := ecsSvcCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, ecsSvcWithTaskDef("api-service", taskDefARN), resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (no ECS client)", result.Count)
	}
}

// TestRelated_ECSSvc_Secrets_NonSMARNSkipped verifies that non-secretsmanager
// ValueFrom values (e.g. SSM Parameter Store ARNs) are not returned.
func TestRelated_ECSSvc_Secrets_NonSMARNSkipped(t *testing.T) {
	const taskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:5"
	const ssmARN = "arn:aws:ssm:us-east-1:123456789012:parameter/my-param"

	td := &ecstypes.TaskDefinition{
		ContainerDefinitions: []ecstypes.ContainerDefinition{
			{
				Secrets: []ecstypes.Secret{
					{Name: aws.String("MY_PARAM"), ValueFrom: aws.String(ssmARN)},
				},
			},
		},
	}
	fakeECS := newFakeECSWithTaskDefinition(td)
	clients := &awsclient.ServiceClients{ECS: fakeECS}

	checker := ecsSvcCheckerByTarget(t, "secrets")
	result := checker(context.Background(), clients, ecsSvcWithTaskDef("api-service", taskDefARN), resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (SSM ARN should be skipped)", result.Count)
	}
}
