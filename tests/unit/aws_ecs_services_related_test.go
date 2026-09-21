// aws_ecs_services_related_test.go contains unit tests for ECS Services related-resource checkers.
package unit_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	sfnsvc "github.com/aws/aws-sdk-go-v2/service/sfn"

	_ "github.com/k2m30/a9s/v3/core/aws"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
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

func TestRelated_ECSSvc_Cluster_FromFields(t *testing.T) {
	checker := ecsSvcCheckerByTarget(t, "ecs")
	res := resource.Resource{
		ID:     "api-gateway",
		Fields: map[string]string{"cluster": "acme-services"},
	}
	result := checker(context.Background(), nil, res, nil)
	if result.Count() != 1 {
		t.Fatalf("expected Count=1, got %d", result.Count())
	}
	if result.ResourceIDs()[0] != "acme-services" {
		t.Errorf("expected ResourceIDs[0]=%q, got %q", "acme-services", result.ResourceIDs()[0])
	}
}

func TestRelated_ECSSvc_Cluster_EmptyField(t *testing.T) {
	checker := ecsSvcCheckerByTarget(t, "ecs")
	res := resource.Resource{
		ID:     "api-gateway",
		Fields: map[string]string{},
	}
	result := checker(context.Background(), nil, res, nil)
	if result.Count() != 0 {
		t.Errorf("expected Count=0 for empty cluster field, got %d", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 {
		t.Fatalf("ResourceIDs len = %d, want 1", len(result.ResourceIDs()))
	}
	if result.ResourceIDs()[0] != "api-tg" {
		t.Errorf("ResourceIDs[0] = %q, want %q", result.ResourceIDs()[0], "api-tg")
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
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

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 for invalid RawStruct", result.Count())
	}
}

func TestRelated_ECSSvc_Alarms_MatchServiceAndCluster(t *testing.T) {
	serviceName := "api-service"
	clusterName := "my-cluster"
	alarmRes := resource.Resource{
		ID: "ecs-svc-cpu-high",
		RawStruct: cwtypes.MetricAlarm{
			Namespace: aws.String("ECS/ContainerInsights"),
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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "ecs-svc-cpu-high" {
		t.Errorf("ResourceIDs = %v, want [ecs-svc-cpu-high]", result.ResourceIDs())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
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

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count())
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for empty service ID", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "my-stack" {
		t.Errorf("ResourceIDs = %v, want [my-stack]", result.ResourceIDs())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for service with no CFN tag", result.Count())
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

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (unknown)", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != ruleName {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), ruleName)
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if !result.Truncated() {
		t.Error("Truncated = false, want true (cache is truncated)")
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no rules reference this service)", result.Count())
	}
}

// Reverse-scan checkers leave FetchFilter unset.
func TestRelated_ECSSvc_EbRule_FetchFilter(t *testing.T) {
	const svcName = "api-service"

	cache := resource.ResourceCache{
		"eb-rule": resource.ResourceCacheEntry{Resources: []resource.Resource{}},
	}

	checker := ecsSvcCheckerByTarget(t, "eb-rule")
	result := checker(context.Background(), nil, ecsSvcSourceResource(svcName, "prod-cluster", ""), cache)

	if len(result.FetchFilter()) != 0 {
		t.Errorf("FetchFilter = %v, want empty (reverse-scan checkers must not set FetchFilter)", result.FetchFilter())
	}
}

type fakeSFNForECSSvc struct {
	awsclient.SFNAPI
	describeOutputByARN map[string]*sfnsvc.DescribeStateMachineOutput
}

func (f *fakeSFNForECSSvc) DescribeStateMachine(_ context.Context, input *sfnsvc.DescribeStateMachineInput, _ ...func(*sfnsvc.Options)) (*sfnsvc.DescribeStateMachineOutput, error) {
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

func sfnResourceWithARN(name, arn string) resource.Resource {
	return resource.Resource{
		ID:   name,
		Name: name,
		Fields: map[string]string{
			"arn": arn,
		},
	}
}

func sfnASLWithECSFamily(family string) string {
	return `{"Comment":"test","StartAt":"Run","States":{"Run":{"Type":"Task","Resource":"arn:aws:states:::ecs:runTask.sync","Parameters":{"TaskDefinition":"` + family + `","LaunchType":"FARGATE"},"End":true}}}`
}

func TestRelated_ECSSvc_SFN_Match(t *testing.T) {
	const svcName = "api-service"
	const clusterName = "prod-cluster"
	const taskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:5"
	const sfnARN = "arn:aws:states:us-east-1:123456789012:stateMachine:api-pipeline"

	fakeSFN := &fakeSFNForECSSvc{
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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "api-pipeline" {
		t.Errorf("ResourceIDs = %v, want [api-pipeline]", result.ResourceIDs())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

func TestRelated_ECSSvc_SFN_Empty(t *testing.T) {
	const taskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:5"
	const sfnARN = "arn:aws:states:us-east-1:123456789012:stateMachine:other-pipeline"

	fakeSFN := &fakeSFNForECSSvc{
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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (task family does not match)", result.Count())
	}
}

func TestRelated_ECSSvc_SFN_WrongRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "api-service",
		RawStruct: "not-a-service",
	}
	checker := ecsSvcCheckerByTarget(t, "sfn")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count())
	}
}

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
	// An image belongs to the repository whose URI it carries, so the answer
	// is the repository row of that URI.
	cache := resource.ResourceCache{"ecr": resource.ResourceCacheEntry{Resources: []resource.Resource{
		{ID: expectedRepo, Name: expectedRepo, Fields: map[string]string{"uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/" + expectedRepo}},
	}}}

	checker := ecsSvcCheckerByTarget(t, "ecr")
	result := checker(context.Background(), clients, ecsSvcWithTaskDef("api-service", taskDefARN), cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != expectedRepo {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs(), expectedRepo)
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no ECR images)", result.Count())
	}
}

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
	cache := resource.ResourceCache{"ecr": resource.ResourceCacheEntry{Resources: []resource.Resource{
		{ID: "app-repo", Name: "app-repo", Fields: map[string]string{"uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/app-repo"}},
		{ID: "sidecar-repo", Name: "sidecar-repo", Fields: map[string]string{"uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/sidecar-repo"}},
	}}}

	checker := ecsSvcCheckerByTarget(t, "ecr")
	result := checker(context.Background(), clients, ecsSvcWithTaskDef("multi-service", taskDefARN), cache)

	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2", result.Count())
	}
	if len(result.ResourceIDs()) != 2 {
		t.Errorf("ResourceIDs = %v, want 2 entries", result.ResourceIDs())
	}
}

func TestRelated_ECSSvc_ECR_NoTaskDef(t *testing.T) {
	source := resource.Resource{
		ID:     "api-service",
		Name:   "api-service",
		Fields: map[string]string{"cluster": "prod-cluster"},
		RawStruct: ecstypes.Service{
			ServiceName: aws.String("api-service"),
		},
	}
	checker := ecsSvcCheckerByTarget(t, "ecr")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no task definition)", result.Count())
	}
}

func TestRelated_ECSSvc_ECR_NoClient(t *testing.T) {
	const taskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:5"

	checker := ecsSvcCheckerByTarget(t, "ecr")
	result := checker(context.Background(), nil, ecsSvcWithTaskDef("api-service", taskDefARN), resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (no ECS client)", result.Count())
	}
}

func TestRelated_ECSSvc_ECR_WrongRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "api-service",
		RawStruct: "not-a-service",
	}
	checker := ecsSvcCheckerByTarget(t, "ecr")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	// secrets rows are keyed by the secret name, without the "-XXXXXX" the ARN
	// carries.
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "my-db-password" {
		t.Errorf("ResourceIDs = %v, want [my-db-password]", result.ResourceIDs())
	}
	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
}

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

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "ecr-creds" {
		t.Errorf("ResourceIDs = %v, want [ecr-creds]", result.ResourceIDs())
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no secret references)", result.Count())
	}
}

func TestRelated_ECSSvc_Secrets_NoTaskDef(t *testing.T) {
	source := resource.Resource{
		ID:     "api-service",
		Name:   "api-service",
		Fields: map[string]string{"cluster": "prod-cluster"},
		RawStruct: ecstypes.Service{
			ServiceName: aws.String("api-service"),
		},
	}
	checker := ecsSvcCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no task definition)", result.Count())
	}
}

func TestRelated_ECSSvc_Secrets_NoClient(t *testing.T) {
	const taskDefARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:5"

	checker := ecsSvcCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, ecsSvcWithTaskDef("api-service", taskDefARN), resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (no ECS client)", result.Count())
	}
}

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

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (SSM ARN should be skipped)", result.Count())
	}
}

func TestRelated_ECSSvc_CTEvents_NoMatch(t *testing.T) {
	evRes := resource.Resource{
		ID:   "evt-other",
		Name: "evt-other",
		RawStruct: cloudtrailtypes.Event{
			EventId: aws.String("evt-other"),
			Resources: []cloudtrailtypes.Resource{
				{ResourceName: aws.String("arn:aws:ecs:us-east-1:123456789012:service/prod/different-service")},
			},
		},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{Resources: []resource.Resource{evRes}},
	}
	source := resource.Resource{ID: "api-service", Name: "api-service"}

	checker := ecsSvcCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_ECSSvc_CTEvents_EmptyServiceID(t *testing.T) {
	source := resource.Resource{ID: "", Name: ""}

	checker := ecsSvcCheckerByTarget(t, "ct-events")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (empty service ID)", result.Count())
	}
}

func TestRelated_ECSSvc_Tasks_MatchByGroup(t *testing.T) {
	const svcName = "api-service"
	taskRes := resource.Resource{
		ID:   "task-abc123",
		Name: "task-abc123",
		RawStruct: ecstypes.Task{
			TaskArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task/prod/task-abc123"),
			Group:   aws.String("service:" + svcName),
		},
	}
	otherTaskRes := resource.Resource{
		ID:   "task-other",
		Name: "task-other",
		RawStruct: ecstypes.Task{
			TaskArn: aws.String("arn:aws:ecs:us-east-1:123456789012:task/prod/task-other"),
			Group:   aws.String("service:other-service"),
		},
	}
	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{Resources: []resource.Resource{taskRes, otherTaskRes}},
	}
	source := resource.Resource{ID: svcName, Name: svcName}

	checker := ecsSvcCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "task-abc123" {
		t.Errorf("ResourceIDs = %v, want [task-abc123]", result.ResourceIDs())
	}
}

func TestRelated_ECSSvc_Tasks_NoMatch(t *testing.T) {
	taskRes := resource.Resource{
		ID:   "task-other",
		Name: "task-other",
		RawStruct: ecstypes.Task{
			Group: aws.String("service:different-service"),
		},
	}
	cache := resource.ResourceCache{
		"ecs-task": resource.ResourceCacheEntry{Resources: []resource.Resource{taskRes}},
	}
	source := resource.Resource{ID: "api-service", Name: "api-service"}

	checker := ecsSvcCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_ECSSvc_Tasks_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{ID: "api-service", Name: "api-service"}

	checker := ecsSvcCheckerByTarget(t, "ecs-task")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (cache miss, no clients)", result.Count())
	}
}

func TestRelated_ECSSvc_Subnet_FromRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:   "api-service",
		Name: "api-service",
		RawStruct: ecstypes.Service{
			ServiceName: aws.String("api-service"),
			NetworkConfiguration: &ecstypes.NetworkConfiguration{
				AwsvpcConfiguration: &ecstypes.AwsVpcConfiguration{
					Subnets: []string{"subnet-aaa111", "subnet-bbb222"},
				},
			},
		},
	}

	checker := ecsSvcCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2", result.Count())
	}
	found := map[string]bool{}
	for _, id := range result.ResourceIDs() {
		found[id] = true
	}
	if !found["subnet-aaa111"] || !found["subnet-bbb222"] {
		t.Errorf("ResourceIDs = %v, want [subnet-aaa111, subnet-bbb222]", result.ResourceIDs())
	}
}

func TestRelated_ECSSvc_Subnet_NoNetworkConfig(t *testing.T) {
	source := resource.Resource{
		ID:   "api-service",
		Name: "api-service",
		RawStruct: ecstypes.Service{
			ServiceName:          aws.String("api-service"),
			NetworkConfiguration: nil,
		},
	}

	checker := ecsSvcCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no NetworkConfiguration)", result.Count())
	}
}

func TestRelated_ECSSvc_Subnet_InvalidRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "api-service",
		RawStruct: "not-an-ecs-service",
	}

	checker := ecsSvcCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 for invalid RawStruct", result.Count())
	}
}

func TestRelated_ECSSvc_VPC_MatchViaSubnetCache(t *testing.T) {
	subnetRes := resource.Resource{
		ID:     "subnet-aaa111",
		Name:   "subnet-aaa111",
		Fields: map[string]string{"vpc_id": "vpc-abc123"},
	}
	cache := resource.ResourceCache{
		"subnet": resource.ResourceCacheEntry{Resources: []resource.Resource{subnetRes}},
	}
	source := resource.Resource{
		ID:   "api-service",
		Name: "api-service",
		RawStruct: ecstypes.Service{
			ServiceName: aws.String("api-service"),
			NetworkConfiguration: &ecstypes.NetworkConfiguration{
				AwsvpcConfiguration: &ecstypes.AwsVpcConfiguration{
					Subnets: []string{"subnet-aaa111"},
				},
			},
		},
	}

	checker := ecsSvcCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "vpc-abc123" {
		t.Errorf("ResourceIDs = %v, want [vpc-abc123]", result.ResourceIDs())
	}
}

func TestRelated_ECSSvc_VPC_NoNetworkConfig(t *testing.T) {
	source := resource.Resource{
		ID:   "api-service",
		Name: "api-service",
		RawStruct: ecstypes.Service{
			ServiceName:          aws.String("api-service"),
			NetworkConfiguration: nil,
		},
	}

	checker := ecsSvcCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no network configuration)", result.Count())
	}
}

func TestRelated_ECSSvc_VPC_CacheMissNoClients(t *testing.T) {
	source := resource.Resource{
		ID:   "api-service",
		Name: "api-service",
		RawStruct: ecstypes.Service{
			ServiceName: aws.String("api-service"),
			NetworkConfiguration: &ecstypes.NetworkConfiguration{
				AwsvpcConfiguration: &ecstypes.AwsVpcConfiguration{
					Subnets: []string{"subnet-aaa111"},
				},
			},
		},
	}

	checker := ecsSvcCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (subnet cache miss, no clients)", result.Count())
	}
}

func TestRelated_ECSSvc_VPC_InvalidRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "api-service",
		RawStruct: "not-an-ecs-service",
	}

	checker := ecsSvcCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 for invalid RawStruct", result.Count())
	}
}

func TestRelated_ECSSvc_SG_Found(t *testing.T) {
	source := resource.Resource{
		ID:   "api-service",
		Name: "api-service",
		RawStruct: ecstypes.Service{
			ServiceName: aws.String("api-service"),
			NetworkConfiguration: &ecstypes.NetworkConfiguration{
				AwsvpcConfiguration: &ecstypes.AwsVpcConfiguration{
					SecurityGroups: []string{"sg-aaabbb111", "sg-cccddd222"},
				},
			},
		},
	}

	checker := ecsSvcCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2", result.Count())
	}
	found := map[string]bool{}
	for _, id := range result.ResourceIDs() {
		found[id] = true
	}
	if !found["sg-aaabbb111"] || !found["sg-cccddd222"] {
		t.Errorf("ResourceIDs = %v, want sg-aaabbb111 and sg-cccddd222", result.ResourceIDs())
	}
}

func TestRelated_ECSSvc_SG_NilNetworkConfiguration(t *testing.T) {
	source := resource.Resource{
		ID:   "api-service",
		Name: "api-service",
		RawStruct: ecstypes.Service{
			ServiceName:          aws.String("api-service"),
			NetworkConfiguration: nil,
		},
	}

	checker := ecsSvcCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (nil NetworkConfiguration)", result.Count())
	}
}

func TestRelated_ECSSvc_SG_WrongRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "api-service",
		RawStruct: "not-an-ecs-service",
	}

	checker := ecsSvcCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count())
	}
}

func TestRelated_ECSSvc_Logs_MatchByFamily(t *testing.T) {
	const family = "api-task"
	logRes := resource.Resource{
		ID:   "/aws/ecs/api-task",
		Name: "/aws/ecs/api-task",
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}
	source := resource.Resource{
		ID:   "api-service",
		Name: "api-service",
		RawStruct: ecstypes.Service{
			ServiceName:    aws.String("api-service"),
			TaskDefinition: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/" + family + ":7"),
		},
	}

	checker := ecsSvcCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "/aws/ecs/api-task" {
		t.Errorf("ResourceIDs = %v, want [/aws/ecs/api-task]", result.ResourceIDs())
	}
}

func TestRelated_ECSSvc_Logs_NoMatchDifferentFamily(t *testing.T) {
	logRes := resource.Resource{
		ID:   "/aws/ecs/other-task",
		Name: "/aws/ecs/other-task",
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{Resources: []resource.Resource{logRes}},
	}
	source := resource.Resource{
		ID:   "api-service",
		Name: "api-service",
		RawStruct: ecstypes.Service{
			ServiceName:    aws.String("api-service"),
			TaskDefinition: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:3"),
		},
	}

	checker := ecsSvcCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no matching log group)", result.Count())
	}
}

func TestRelated_ECSSvc_Logs_NilTaskDef(t *testing.T) {
	source := resource.Resource{
		ID:   "api-service",
		Name: "api-service",
		RawStruct: ecstypes.Service{
			ServiceName:    aws.String("api-service"),
			TaskDefinition: nil,
		},
	}

	checker := ecsSvcCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (nil TaskDefinition)", result.Count())
	}
}

func TestRelated_ECSSvc_Logs_NilCache(t *testing.T) {
	source := resource.Resource{
		ID:   "api-service",
		Name: "api-service",
		RawStruct: ecstypes.Service{
			ServiceName:    aws.String("api-service"),
			TaskDefinition: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:3"),
		},
	}

	checker := ecsSvcCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil cache, nil clients)", result.Count())
	}
}

func TestRelated_ECSSvc_Logs_TruncatedCacheNoMatch(t *testing.T) {
	logRes := resource.Resource{
		ID:   "/aws/ecs/other-task",
		Name: "/aws/ecs/other-task",
	}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{
			Resources:   []resource.Resource{logRes},
			IsTruncated: true,
		},
	}
	source := resource.Resource{
		ID:   "api-service",
		Name: "api-service",
		RawStruct: ecstypes.Service{
			ServiceName:    aws.String("api-service"),
			TaskDefinition: aws.String("arn:aws:ecs:us-east-1:123456789012:task-definition/api-task:3"),
		},
	}

	checker := ecsSvcCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, cache)

	if !result.Truncated() {
		t.Errorf("Truncated = false, want true (truncated cache, no match)")
	}
}

func TestRelated_ECSSvc_ELB_FoundViaTG(t *testing.T) {
	const tgARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/api-tg/abc123"
	const elbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/api-alb/xyz789"

	tgRes := resource.Resource{
		ID:   "api-tg",
		Name: "api-tg",
		RawStruct: elbv2types.TargetGroup{
			TargetGroupArn:   aws.String(tgARN),
			LoadBalancerArns: []string{elbARN},
		},
	}
	// The elb fetcher keys rows by bare load balancer name, so the checker
	// cross-references Fields["load_balancer_arn"].
	elbRes := resource.Resource{
		ID:     "api-alb",
		Name:   "api-alb",
		Fields: map[string]string{"load_balancer_arn": elbARN},
	}
	cache := resource.ResourceCache{
		"tg":  resource.ResourceCacheEntry{Resources: []resource.Resource{tgRes}},
		"elb": resource.ResourceCacheEntry{Resources: []resource.Resource{elbRes}},
	}
	source := resource.Resource{
		ID:   "api-service",
		Name: "api-service",
		RawStruct: ecstypes.Service{
			ServiceName: aws.String("api-service"),
			LoadBalancers: []ecstypes.LoadBalancer{
				{TargetGroupArn: aws.String(tgARN)},
			},
		},
	}

	checker := ecsSvcCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if len(result.ResourceIDs()) != 1 || result.ResourceIDs()[0] != "api-alb" {
		t.Errorf("ResourceIDs = %v, want [api-alb]", result.ResourceIDs())
	}
}

func TestRelated_ECSSvc_ELB_NoLoadBalancers(t *testing.T) {
	source := resource.Resource{
		ID:   "api-service",
		Name: "api-service",
		RawStruct: ecstypes.Service{
			ServiceName:   aws.String("api-service"),
			LoadBalancers: nil,
		},
	}

	checker := ecsSvcCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no LoadBalancers)", result.Count())
	}
}

func TestRelated_ECSSvc_ELB_TGInCacheButNoELBMatch(t *testing.T) {
	const tgARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/api-tg/abc123"
	const elbARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/api-alb/xyz789"
	const otherELBARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/other-alb/000"

	tgRes := resource.Resource{
		ID:   "api-tg",
		Name: "api-tg",
		RawStruct: elbv2types.TargetGroup{
			TargetGroupArn:   aws.String(tgARN),
			LoadBalancerArns: []string{elbARN},
		},
	}
	elbRes := resource.Resource{
		ID:     "other-alb",
		Name:   "other-alb",
		Fields: map[string]string{"load_balancer_arn": otherELBARN},
	}
	cache := resource.ResourceCache{
		"tg":  resource.ResourceCacheEntry{Resources: []resource.Resource{tgRes}},
		"elb": resource.ResourceCacheEntry{Resources: []resource.Resource{elbRes}},
	}
	source := resource.Resource{
		ID:   "api-service",
		Name: "api-service",
		RawStruct: ecstypes.Service{
			ServiceName: aws.String("api-service"),
			LoadBalancers: []ecstypes.LoadBalancer{
				{TargetGroupArn: aws.String(tgARN)},
			},
		},
	}

	checker := ecsSvcCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, source, cache)

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no matching ELB ARN)", result.Count())
	}
}

func TestRelated_ECSSvc_ELB_NilTGCache(t *testing.T) {
	const tgARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/api-tg/abc123"
	source := resource.Resource{
		ID:   "api-service",
		Name: "api-service",
		RawStruct: ecstypes.Service{
			ServiceName: aws.String("api-service"),
			LoadBalancers: []ecstypes.LoadBalancer{
				{TargetGroupArn: aws.String(tgARN)},
			},
		},
	}

	checker := ecsSvcCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (nil TG cache with nil clients)", result.Count())
	}
}

func TestRelated_ECSSvc_ELB_WrongRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "api-service",
		RawStruct: "not-an-ecs-service",
	}

	checker := ecsSvcCheckerByTarget(t, "elb")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (wrong RawStruct type)", result.Count())
	}
}

// ensure elbv2types import is used
var _ = elbv2types.TargetGroup{}

// ensure awsclient import is used
var _ = awsclient.ServiceClients{}
