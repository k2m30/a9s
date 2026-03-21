package unit_test

import (
	"strings"
	"testing"
	"time"

	acmtypes "github.com/aws/aws-sdk-go-v2/service/acm/types"
	autoscalingtypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
	lambdatypes "github.com/aws/aws-sdk-go-v2/service/lambda/types"
	sesv2types "github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func ptrFloat64(f float64) *float64 { return &f }

var svcTestTime = time.Date(2025, 6, 15, 10, 30, 0, 0, time.UTC)

// ---------------------------------------------------------------------------
// Realistic SDK struct builders for service types
// ---------------------------------------------------------------------------

func realisticLambdaFunction() lambdatypes.FunctionConfiguration {
	return lambdatypes.FunctionConfiguration{
		FunctionName: ptrString("my-api-handler"),
		FunctionArn:  ptrString("arn:aws:lambda:us-east-1:123456789012:function:my-api-handler"),
		Runtime:      lambdatypes.RuntimePython312,
		Handler:      ptrString("index.handler"),
		MemorySize:   ptrInt32(256),
		Timeout:      ptrInt32(30),
		CodeSize:     5242880,
		Description:  ptrString("API request handler"),
		Role:         ptrString("arn:aws:iam::123456789012:role/lambda-exec-role"),
		State:        lambdatypes.StateActive,
		LastModified: ptrString("2025-06-15T10:30:00.000+0000"),
	}
}

func realisticAlarm() cwtypes.MetricAlarm {
	return cwtypes.MetricAlarm{
		AlarmName:          ptrString("HighCPUAlarm"),
		AlarmArn:           ptrString("arn:aws:cloudwatch:us-east-1:123456789012:alarm:HighCPUAlarm"),
		StateValue:         cwtypes.StateValueAlarm,
		MetricName:         ptrString("CPUUtilization"),
		Namespace:          ptrString("AWS/EC2"),
		Statistic:          cwtypes.StatisticAverage,
		Period:             ptrInt32(300),
		EvaluationPeriods:  ptrInt32(3),
		Threshold:          ptrFloat64(80.0),
		ComparisonOperator: cwtypes.ComparisonOperatorGreaterThanOrEqualToThreshold,
	}
}

func realisticSNSTopic() snstypes.Topic {
	return snstypes.Topic{
		TopicArn: ptrString("arn:aws:sns:us-east-1:123456789012:my-notifications"),
	}
}

func realisticELB() elbv2types.LoadBalancer {
	return elbv2types.LoadBalancer{
		LoadBalancerName: ptrString("my-app-alb"),
		LoadBalancerArn:  ptrString("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/my-app-alb/50dc6c495c0c9188"),
		DNSName:          ptrString("my-app-alb-123456789.us-east-1.elb.amazonaws.com"),
		Type:             elbv2types.LoadBalancerTypeEnumApplication,
		Scheme:           elbv2types.LoadBalancerSchemeEnumInternetFacing,
		State: &elbv2types.LoadBalancerState{
			Code: elbv2types.LoadBalancerStateEnumActive,
		},
		VpcId:       ptrString("vpc-0abc1234"),
		CreatedTime: ptrTime(svcTestTime),
	}
}

func realisticTargetGroup() elbv2types.TargetGroup {
	return elbv2types.TargetGroup{
		TargetGroupName:    ptrString("my-app-tg"),
		TargetGroupArn:     ptrString("arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/my-app-tg/50dc6c495c0c9188"),
		Port:               ptrInt32(8080),
		Protocol:           elbv2types.ProtocolEnumHttp,
		VpcId:              ptrString("vpc-0abc1234"),
		TargetType:         elbv2types.TargetTypeEnumInstance,
		HealthCheckPath:    ptrString("/health"),
		HealthCheckEnabled: ptrBool(true),
	}
}

func realisticECSClusterStruct() ecstypes.Cluster {
	return ecstypes.Cluster{
		ClusterName:         ptrString("prod-cluster"),
		ClusterArn:          ptrString("arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster"),
		Status:              ptrString("ACTIVE"),
		RunningTasksCount:   5,
		PendingTasksCount:   1,
		ActiveServicesCount: 3,
	}
}

func realisticECSService() ecstypes.Service {
	return ecstypes.Service{
		ServiceName:    ptrString("api-service"),
		ServiceArn:     ptrString("arn:aws:ecs:us-east-1:123456789012:service/prod-cluster/api-service"),
		ClusterArn:     ptrString("arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster"),
		Status:         ptrString("ACTIVE"),
		DesiredCount:   3,
		RunningCount:   3,
		LaunchType:     ecstypes.LaunchTypeFargate,
		TaskDefinition: ptrString("arn:aws:ecs:us-east-1:123456789012:task-definition/api-service:42"),
	}
}

func realisticECSTask() ecstypes.Task {
	return ecstypes.Task{
		TaskArn:           ptrString("arn:aws:ecs:us-east-1:123456789012:task/prod-cluster/abc123def456"),
		ClusterArn:        ptrString("arn:aws:ecs:us-east-1:123456789012:cluster/prod-cluster"),
		LastStatus:        ptrString("RUNNING"),
		DesiredStatus:     ptrString("RUNNING"),
		TaskDefinitionArn: ptrString("arn:aws:ecs:us-east-1:123456789012:task-definition/api-service:42"),
		LaunchType:        ecstypes.LaunchTypeFargate,
		Cpu:               ptrString("256"),
		Memory:            ptrString("512"),
		StartedAt:         ptrTime(svcTestTime),
	}
}

func realisticCFNStack() cfntypes.Stack {
	return cfntypes.Stack{
		StackName:    ptrString("my-app-stack"),
		StackId:      ptrString("arn:aws:cloudformation:us-east-1:123456789012:stack/my-app-stack/guid-1234"),
		StackStatus:  cfntypes.StackStatusCreateComplete,
		CreationTime: ptrTime(svcTestTime),
		Description:  ptrString("Application infrastructure stack"),
	}
}

func realisticIAMRole() iamtypes.Role {
	return iamtypes.Role{
		RoleName:           ptrString("lambda-exec-role"),
		RoleId:             ptrString("AROAEXAMPLEROLEID"),
		Arn:                ptrString("arn:aws:iam::123456789012:role/lambda-exec-role"),
		Path:               ptrString("/"),
		CreateDate:         ptrTime(svcTestTime),
		Description:        ptrString("Execution role for Lambda functions"),
		MaxSessionDuration: ptrInt32(3600),
	}
}

func realisticLogGroup() cwlogstypes.LogGroup {
	return cwlogstypes.LogGroup{
		LogGroupName:  ptrString("/aws/lambda/my-api-handler"),
		LogGroupArn:   ptrString("arn:aws:logs:us-east-1:123456789012:log-group:/aws/lambda/my-api-handler:*"),
		StoredBytes:   ptrInt64(1073741824),
		RetentionInDays: ptrInt32(30),
		CreationTime:  ptrInt64(1718444400000),
	}
}

func realisticSSMParameter() ssmtypes.ParameterMetadata {
	return ssmtypes.ParameterMetadata{
		Name:             ptrString("/app/config/db-host"),
		Type:             ssmtypes.ParameterTypeString,
		Version:          1,
		LastModifiedDate: ptrTime(svcTestTime),
		Description:      ptrString("Database host parameter"),
	}
}

func realisticDDBTable() ddbtypes.TableDescription {
	return ddbtypes.TableDescription{
		TableName:        ptrString("users-table"),
		TableArn:         ptrString("arn:aws:dynamodb:us-east-1:123456789012:table/users-table"),
		TableStatus:      ddbtypes.TableStatusActive,
		ItemCount:        ptrInt64(50000),
		TableSizeBytes:   ptrInt64(10485760),
		CreationDateTime: ptrTime(svcTestTime),
	}
}

func realisticACMCertificate() acmtypes.CertificateSummary {
	return acmtypes.CertificateSummary{
		DomainName:     ptrString("example.com"),
		CertificateArn: ptrString("arn:aws:acm:us-east-1:123456789012:certificate/12345678-1234-1234-1234-123456789012"),
		Status:         acmtypes.CertificateStatusIssued,
		Type:           acmtypes.CertificateTypeAmazonIssued,
		CreatedAt:      ptrTime(svcTestTime),
	}
}

func realisticASG() autoscalingtypes.AutoScalingGroup {
	return autoscalingtypes.AutoScalingGroup{
		AutoScalingGroupName: ptrString("my-app-asg"),
		AutoScalingGroupARN:  ptrString("arn:aws:autoscaling:us-east-1:123456789012:autoScalingGroup:guid:autoScalingGroupName/my-app-asg"),
		MinSize:              ptrInt32(2),
		MaxSize:              ptrInt32(10),
		DesiredCapacity:      ptrInt32(4),
		AvailabilityZones:    []string{"us-east-1a", "us-east-1b"},
		CreatedTime:          ptrTime(svcTestTime),
	}
}

func realisticIAMUser() iamtypes.User {
	return iamtypes.User{
		UserName:   ptrString("deploy-user"),
		UserId:     ptrString("AIDAEXAMPLEUSERID"),
		Arn:        ptrString("arn:aws:iam::123456789012:user/deploy-user"),
		Path:       ptrString("/"),
		CreateDate: ptrTime(svcTestTime),
	}
}

func realisticIAMGroup() iamtypes.Group {
	return iamtypes.Group{
		GroupName:  ptrString("developers"),
		GroupId:    ptrString("AGPAEXAMPLEGROUPID"),
		Arn:        ptrString("arn:aws:iam::123456789012:group/developers"),
		Path:       ptrString("/"),
		CreateDate: ptrTime(svcTestTime),
	}
}

// realisticSESIdentity returns an sesv2types.IdentityInfo matching the type
// produced by internal/aws/ses.go FetchSESIdentities.
func realisticSESIdentity() sesv2types.IdentityInfo {
	return sesv2types.IdentityInfo{
		IdentityName:       ptrString("example.com"),
		IdentityType:       sesv2types.IdentityTypeDomain,
		SendingEnabled:     true,
		VerificationStatus: sesv2types.VerificationStatusSuccess,
	}
}

// ---------------------------------------------------------------------------
// Lambda
// ---------------------------------------------------------------------------

func TestQA_Detail_Lambda_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	fn := realisticLambdaFunction()
	res := buildResource("my-api-handler", "my-api-handler", fn)
	m := newDetailModel(res, "lambda", configForType("lambda"))

	view := m.View()
	for _, expected := range []string{
		"FunctionName", "my-api-handler",
		"Runtime", "python3.12",
		"Handler", "index.handler",
		"MemorySize", "256",
		"Timeout", "30",
		"State", "Active",
		"Description", "API request handler",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("Lambda detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_Lambda_NilFields(t *testing.T) {
	ensureNoColor(t)
	fn := lambdatypes.FunctionConfiguration{}
	res := buildResource("empty-fn", "empty-fn", fn)
	m := newDetailModel(res, "lambda", configForType("lambda"))

	view := m.View()
	if view == "" {
		t.Error("Lambda detail view should not be empty even with nil fields")
	}
}

func TestQA_Detail_Lambda_FrameTitle(t *testing.T) {
	fn := realisticLambdaFunction()
	res := buildResource("my-api-handler", "my-api-handler", fn)
	m := newDetailModel(res, "lambda", configForType("lambda"))

	if title := m.FrameTitle(); title != "my-api-handler" {
		t.Errorf("FrameTitle expected %q, got %q", "my-api-handler", title)
	}
}

// ---------------------------------------------------------------------------
// Alarm (CloudWatch MetricAlarm)
// ---------------------------------------------------------------------------

func TestQA_Detail_Alarm_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	alarm := realisticAlarm()
	res := buildResource("HighCPUAlarm", "HighCPUAlarm", alarm)
	m := newDetailModel(res, "alarm", configForType("alarm"))

	view := m.View()
	for _, expected := range []string{
		"AlarmName", "HighCPUAlarm",
		"StateValue", "ALARM",
		"MetricName", "CPUUtilization",
		"Namespace", "AWS/EC2",
		"Statistic", "Average",
		"Period", "300",
		"EvaluationPeriods", "3",
		"Threshold", "80",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("Alarm detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_Alarm_NilFields(t *testing.T) {
	ensureNoColor(t)
	alarm := cwtypes.MetricAlarm{}
	res := buildResource("empty-alarm", "empty-alarm", alarm)
	m := newDetailModel(res, "alarm", configForType("alarm"))

	view := m.View()
	if view == "" {
		t.Error("Alarm detail view should not be empty even with nil fields")
	}
}

func TestQA_Detail_Alarm_FrameTitle(t *testing.T) {
	alarm := realisticAlarm()
	res := buildResource("HighCPUAlarm", "HighCPUAlarm", alarm)
	m := newDetailModel(res, "alarm", configForType("alarm"))

	if title := m.FrameTitle(); title != "HighCPUAlarm" {
		t.Errorf("FrameTitle expected %q, got %q", "HighCPUAlarm", title)
	}
}

// ---------------------------------------------------------------------------
// SNS Topic
// ---------------------------------------------------------------------------

func TestQA_Detail_SNSTopic_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	topic := realisticSNSTopic()
	res := buildResource("arn:aws:sns:us-east-1:123456789012:my-notifications", "my-notifications", topic)
	m := newDetailModel(res, "sns", configForType("sns"))

	view := m.View()
	if !strings.Contains(view, "TopicArn") {
		t.Errorf("SNS detail should contain TopicArn, got:\n%s", view)
	}
	if !strings.Contains(view, "my-notifications") {
		t.Errorf("SNS detail should contain topic name, got:\n%s", view)
	}
}

func TestQA_Detail_SNSTopic_NilFields(t *testing.T) {
	ensureNoColor(t)
	topic := snstypes.Topic{}
	res := buildResource("empty-topic", "empty-topic", topic)
	m := newDetailModel(res, "sns", configForType("sns"))

	view := m.View()
	if view == "" {
		t.Error("SNS detail view should not be empty even with nil fields")
	}
}

func TestQA_Detail_SNSTopic_FrameTitle(t *testing.T) {
	topic := realisticSNSTopic()
	res := buildResource("arn:aws:sns:us-east-1:123456789012:my-notifications", "my-notifications", topic)
	m := newDetailModel(res, "sns", configForType("sns"))

	if title := m.FrameTitle(); title != "my-notifications" {
		t.Errorf("FrameTitle expected %q, got %q", "my-notifications", title)
	}
}

// ---------------------------------------------------------------------------
// SQS Queue (uses Fields map, not SDK struct)
// ---------------------------------------------------------------------------

func TestQA_Detail_SQS_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	res := buildResourceWithFields("queue-id", "my-queue", map[string]string{
		"QueueUrl":                    "https://sqs.us-east-1.amazonaws.com/123456789012/my-queue",
		"ApproximateNumberOfMessages": "42",
		"VisibilityTimeout":           "30",
		"CreatedTimestamp":             "1718444400",
		"MaximumMessageSize":           "262144",
	})
	// SQS uses map[string]string, not SDK struct — use nil config for Fields-map fallback
	m := newDetailModel(res, "sqs", nil)

	view := m.View()
	for _, expected := range []string{
		"QueueUrl", "my-queue",
		"ApproximateNumberOf", "42",
		"VisibilityTimeout", "30",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("SQS detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_SQS_NilFields(t *testing.T) {
	ensureNoColor(t)
	res := buildResourceWithFields("empty-queue", "empty-queue", map[string]string{})
	m := newDetailModel(res, "sqs", nil)

	// Should not panic
	view := m.View()
	_ = view
}

func TestQA_Detail_SQS_FrameTitle(t *testing.T) {
	res := buildResourceWithFields("queue-id", "my-queue", map[string]string{
		"QueueUrl": "https://sqs.us-east-1.amazonaws.com/123456789012/my-queue",
	})
	m := newDetailModel(res, "sqs", nil)

	if title := m.FrameTitle(); title != "my-queue" {
		t.Errorf("FrameTitle expected %q, got %q", "my-queue", title)
	}
}

// ---------------------------------------------------------------------------
// ELB (Application Load Balancer)
// ---------------------------------------------------------------------------

func TestQA_Detail_ELB_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	elb := realisticELB()
	res := buildResource("my-app-alb", "my-app-alb", elb)
	m := newDetailModel(res, "elb", configForType("elb"))

	view := m.View()
	for _, expected := range []string{
		"LoadBalancerName", "my-app-alb",
		"DNSName", "my-app-alb-123456789.us-east-1.elb.amazonaws.com",
		"Type", "application",
		"Scheme", "internet-facing",
		"VpcId", "vpc-0abc1234",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("ELB detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_ELB_NilFields(t *testing.T) {
	ensureNoColor(t)
	elb := elbv2types.LoadBalancer{}
	res := buildResource("empty-elb", "empty-elb", elb)
	m := newDetailModel(res, "elb", configForType("elb"))

	view := m.View()
	if view == "" {
		t.Error("ELB detail view should not be empty even with nil fields")
	}
}

func TestQA_Detail_ELB_FrameTitle(t *testing.T) {
	elb := realisticELB()
	res := buildResource("my-app-alb", "my-app-alb", elb)
	m := newDetailModel(res, "elb", configForType("elb"))

	if title := m.FrameTitle(); title != "my-app-alb" {
		t.Errorf("FrameTitle expected %q, got %q", "my-app-alb", title)
	}
}

// ---------------------------------------------------------------------------
// TG (Target Group)
// ---------------------------------------------------------------------------

func TestQA_Detail_TG_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	tg := realisticTargetGroup()
	res := buildResource("my-app-tg", "my-app-tg", tg)
	m := newDetailModel(res, "tg", configForType("tg"))

	view := m.View()
	for _, expected := range []string{
		"TargetGroupName", "my-app-tg",
		"Port", "8080",
		"Protocol", "HTTP",
		"VpcId", "vpc-0abc1234",
		"TargetType", "instance",
		"HealthCheckPath", "/health",
		"HealthCheckEnabled", "Yes",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("TG detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_TG_NilFields(t *testing.T) {
	ensureNoColor(t)
	tg := elbv2types.TargetGroup{}
	res := buildResource("empty-tg", "empty-tg", tg)
	m := newDetailModel(res, "tg", configForType("tg"))

	view := m.View()
	if view == "" {
		t.Error("TG detail view should not be empty even with nil fields")
	}
}

func TestQA_Detail_TG_FrameTitle(t *testing.T) {
	tg := realisticTargetGroup()
	res := buildResource("my-app-tg", "my-app-tg", tg)
	m := newDetailModel(res, "tg", configForType("tg"))

	if title := m.FrameTitle(); title != "my-app-tg" {
		t.Errorf("FrameTitle expected %q, got %q", "my-app-tg", title)
	}
}

// ---------------------------------------------------------------------------
// ECS Cluster
// ---------------------------------------------------------------------------

func TestQA_Detail_ECS_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	cluster := realisticECSClusterStruct()
	res := buildResource("prod-cluster", "prod-cluster", cluster)
	m := newDetailModel(res, "ecs", configForType("ecs"))

	view := m.View()
	for _, expected := range []string{
		"ClusterName", "prod-cluster",
		"Status", "ACTIVE",
		"RunningTasksCount", "5",
		"PendingTasksCount", "1",
		"ActiveServicesCount", "3",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("ECS Cluster detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_ECS_NilFields(t *testing.T) {
	ensureNoColor(t)
	cluster := ecstypes.Cluster{}
	res := buildResource("empty-cluster", "empty-cluster", cluster)
	m := newDetailModel(res, "ecs", configForType("ecs"))

	view := m.View()
	if view == "" {
		t.Error("ECS Cluster detail view should not be empty even with nil fields")
	}
}

func TestQA_Detail_ECS_FrameTitle(t *testing.T) {
	cluster := realisticECSClusterStruct()
	res := buildResource("prod-cluster", "prod-cluster", cluster)
	m := newDetailModel(res, "ecs", configForType("ecs"))

	if title := m.FrameTitle(); title != "prod-cluster" {
		t.Errorf("FrameTitle expected %q, got %q", "prod-cluster", title)
	}
}

// ---------------------------------------------------------------------------
// ECS Service
// ---------------------------------------------------------------------------

func TestQA_Detail_ECSSvc_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	svc := realisticECSService()
	res := buildResource("api-service", "api-service", svc)
	m := newDetailModel(res, "ecs-svc", configForType("ecs-svc"))

	view := m.View()
	for _, expected := range []string{
		"ServiceName", "api-service",
		"Status", "ACTIVE",
		"DesiredCount", "3",
		"RunningCount", "3",
		"LaunchType", "FARGATE",
		"TaskDefinition", "api-service:42",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("ECS Service detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_ECSSvc_NilFields(t *testing.T) {
	ensureNoColor(t)
	svc := ecstypes.Service{}
	res := buildResource("empty-svc", "empty-svc", svc)
	m := newDetailModel(res, "ecs-svc", configForType("ecs-svc"))

	view := m.View()
	if view == "" {
		t.Error("ECS Service detail view should not be empty even with nil fields")
	}
}

func TestQA_Detail_ECSSvc_FrameTitle(t *testing.T) {
	svc := realisticECSService()
	res := buildResource("api-service", "api-service", svc)
	m := newDetailModel(res, "ecs-svc", configForType("ecs-svc"))

	if title := m.FrameTitle(); title != "api-service" {
		t.Errorf("FrameTitle expected %q, got %q", "api-service", title)
	}
}

// ---------------------------------------------------------------------------
// ECS Task
// ---------------------------------------------------------------------------

func TestQA_Detail_ECSTask_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	task := realisticECSTask()
	res := buildResource("abc123def456", "abc123def456", task)
	m := newDetailModel(res, "ecs-task", configForType("ecs-task"))

	view := m.View()
	for _, expected := range []string{
		"LastStatus", "RUNNING",
		"DesiredStatus", "RUNNING",
		"LaunchType", "FARGATE",
		"Cpu", "256",
		"Memory", "512",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("ECS Task detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_ECSTask_NilFields(t *testing.T) {
	ensureNoColor(t)
	task := ecstypes.Task{}
	res := buildResource("empty-task", "empty-task", task)
	m := newDetailModel(res, "ecs-task", configForType("ecs-task"))

	view := m.View()
	if view == "" {
		t.Error("ECS Task detail view should not be empty even with nil fields")
	}
}

func TestQA_Detail_ECSTask_FrameTitle(t *testing.T) {
	task := realisticECSTask()
	res := buildResource("abc123def456", "abc123def456", task)
	m := newDetailModel(res, "ecs-task", configForType("ecs-task"))

	if title := m.FrameTitle(); title != "abc123def456" {
		t.Errorf("FrameTitle expected %q, got %q", "abc123def456", title)
	}
}

// ---------------------------------------------------------------------------
// CloudFormation Stack
// ---------------------------------------------------------------------------

func TestQA_Detail_CFN_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	stack := realisticCFNStack()
	res := buildResource("my-app-stack", "my-app-stack", stack)
	m := newDetailModel(res, "cfn", configForType("cfn"))

	view := m.View()
	for _, expected := range []string{
		"StackName", "my-app-stack",
		"StackStatus", "CREATE_COMPLETE",
		"Description", "Application infrastructure stack",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("CFN detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_CFN_NilFields(t *testing.T) {
	ensureNoColor(t)
	stack := cfntypes.Stack{
		StackName:    ptrString("empty-stack"),
		StackStatus:  cfntypes.StackStatusCreateComplete,
		CreationTime: ptrTime(svcTestTime),
	}
	res := buildResource("empty-stack", "empty-stack", stack)
	m := newDetailModel(res, "cfn", configForType("cfn"))

	view := m.View()
	if view == "" {
		t.Error("CFN detail view should not be empty even with minimal fields")
	}
}

func TestQA_Detail_CFN_FrameTitle(t *testing.T) {
	stack := realisticCFNStack()
	res := buildResource("my-app-stack", "my-app-stack", stack)
	m := newDetailModel(res, "cfn", configForType("cfn"))

	if title := m.FrameTitle(); title != "my-app-stack" {
		t.Errorf("FrameTitle expected %q, got %q", "my-app-stack", title)
	}
}

// ---------------------------------------------------------------------------
// IAM Role
// ---------------------------------------------------------------------------

func TestQA_Detail_Role_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	role := realisticIAMRole()
	res := buildResource("lambda-exec-role", "lambda-exec-role", role)
	m := newDetailModel(res, "role", configForType("role"))

	view := m.View()
	for _, expected := range []string{
		"RoleName", "lambda-exec-role",
		"RoleId", "AROAEXAMPLEROLEID",
		"Arn", "arn:aws:iam",
		"Path", "/",
		"Description", "Execution role for Lambda",
		"MaxSessionDuration", "3600",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("IAM Role detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_Role_NilFields(t *testing.T) {
	ensureNoColor(t)
	role := iamtypes.Role{
		RoleName:   ptrString("empty-role"),
		RoleId:     ptrString("AROAEMPTY"),
		Arn:        ptrString("arn:aws:iam::123456789012:role/empty-role"),
		Path:       ptrString("/"),
		CreateDate: ptrTime(svcTestTime),
	}
	res := buildResource("empty-role", "empty-role", role)
	m := newDetailModel(res, "role", configForType("role"))

	view := m.View()
	if view == "" {
		t.Error("IAM Role detail view should not be empty even with minimal fields")
	}
}

func TestQA_Detail_Role_FrameTitle(t *testing.T) {
	role := realisticIAMRole()
	res := buildResource("lambda-exec-role", "lambda-exec-role", role)
	m := newDetailModel(res, "role", configForType("role"))

	if title := m.FrameTitle(); title != "lambda-exec-role" {
		t.Errorf("FrameTitle expected %q, got %q", "lambda-exec-role", title)
	}
}

// ---------------------------------------------------------------------------
// CloudWatch Logs
// ---------------------------------------------------------------------------

func TestQA_Detail_Logs_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	lg := realisticLogGroup()
	res := buildResource("/aws/lambda/my-api-handler", "/aws/lambda/my-api-handler", lg)
	m := newDetailModel(res, "logs", configForType("logs"))

	view := m.View()
	for _, expected := range []string{
		"LogGroupName", "/aws/lambda/my-api-handler",
		"StoredBytes", "1073741824",
		"RetentionInDays", "30",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("Logs detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_Logs_NilFields(t *testing.T) {
	ensureNoColor(t)
	lg := cwlogstypes.LogGroup{}
	res := buildResource("empty-lg", "empty-lg", lg)
	m := newDetailModel(res, "logs", configForType("logs"))

	view := m.View()
	if view == "" {
		t.Error("Logs detail view should not be empty even with nil fields")
	}
}

func TestQA_Detail_Logs_FrameTitle(t *testing.T) {
	lg := realisticLogGroup()
	res := buildResource("/aws/lambda/my-api-handler", "/aws/lambda/my-api-handler", lg)
	m := newDetailModel(res, "logs", configForType("logs"))

	if title := m.FrameTitle(); title != "/aws/lambda/my-api-handler" {
		t.Errorf("FrameTitle expected %q, got %q", "/aws/lambda/my-api-handler", title)
	}
}

// ---------------------------------------------------------------------------
// SSM Parameter Store
// ---------------------------------------------------------------------------

func TestQA_Detail_SSM_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	param := realisticSSMParameter()
	res := buildResource("/app/config/db-host", "/app/config/db-host", param)
	m := newDetailModel(res, "ssm", configForType("ssm"))

	view := m.View()
	for _, expected := range []string{
		"Name", "/app/config/db-host",
		"Type", "String",
		"Version", "1",
		"Description", "Database host parameter",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("SSM detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_SSM_NilFields(t *testing.T) {
	ensureNoColor(t)
	param := ssmtypes.ParameterMetadata{}
	res := buildResource("empty-param", "empty-param", param)
	m := newDetailModel(res, "ssm", configForType("ssm"))

	view := m.View()
	if view == "" {
		t.Error("SSM detail view should not be empty even with nil fields")
	}
}

func TestQA_Detail_SSM_FrameTitle(t *testing.T) {
	param := realisticSSMParameter()
	res := buildResource("/app/config/db-host", "/app/config/db-host", param)
	m := newDetailModel(res, "ssm", configForType("ssm"))

	if title := m.FrameTitle(); title != "/app/config/db-host" {
		t.Errorf("FrameTitle expected %q, got %q", "/app/config/db-host", title)
	}
}

// ---------------------------------------------------------------------------
// DynamoDB Table
// ---------------------------------------------------------------------------

func TestQA_Detail_DDB_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	table := realisticDDBTable()
	res := buildResource("users-table", "users-table", table)
	m := newDetailModel(res, "ddb", configForType("ddb"))

	view := m.View()
	for _, expected := range []string{
		"TableName", "users-table",
		"TableStatus", "ACTIVE",
		"ItemCount", "50000",
		"TableSizeBytes", "10485760",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("DDB detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_DDB_NilFields(t *testing.T) {
	ensureNoColor(t)
	table := ddbtypes.TableDescription{}
	res := buildResource("empty-table", "empty-table", table)
	m := newDetailModel(res, "ddb", configForType("ddb"))

	view := m.View()
	if view == "" {
		t.Error("DDB detail view should not be empty even with nil fields")
	}
}

func TestQA_Detail_DDB_FrameTitle(t *testing.T) {
	table := realisticDDBTable()
	res := buildResource("users-table", "users-table", table)
	m := newDetailModel(res, "ddb", configForType("ddb"))

	if title := m.FrameTitle(); title != "users-table" {
		t.Errorf("FrameTitle expected %q, got %q", "users-table", title)
	}
}

// ---------------------------------------------------------------------------
// ACM Certificate
// ---------------------------------------------------------------------------

func TestQA_Detail_ACM_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	cert := realisticACMCertificate()
	res := buildResource("example.com", "example.com", cert)
	m := newDetailModel(res, "acm", configForType("acm"))

	view := m.View()
	for _, expected := range []string{
		"DomainName", "example.com",
		"Status", "ISSUED",
		"Type", "AMAZON_ISSUED",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("ACM detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_ACM_NilFields(t *testing.T) {
	ensureNoColor(t)
	cert := acmtypes.CertificateSummary{}
	res := buildResource("empty-cert", "empty-cert", cert)
	m := newDetailModel(res, "acm", configForType("acm"))

	view := m.View()
	if view == "" {
		t.Error("ACM detail view should not be empty even with nil fields")
	}
}

func TestQA_Detail_ACM_FrameTitle(t *testing.T) {
	cert := realisticACMCertificate()
	res := buildResource("example.com", "example.com", cert)
	m := newDetailModel(res, "acm", configForType("acm"))

	if title := m.FrameTitle(); title != "example.com" {
		t.Errorf("FrameTitle expected %q, got %q", "example.com", title)
	}
}

// ---------------------------------------------------------------------------
// Auto Scaling Group
// ---------------------------------------------------------------------------

func TestQA_Detail_ASG_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	asg := realisticASG()
	res := buildResource("my-app-asg", "my-app-asg", asg)
	m := newDetailModel(res, "asg", configForType("asg"))

	view := m.View()
	for _, expected := range []string{
		"AutoScalingGroupName", "my-app-asg",
		"MinSize", "2",
		"MaxSize", "10",
		"DesiredCapacity", "4",
		"AvailabilityZones",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("ASG detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_ASG_NilFields(t *testing.T) {
	ensureNoColor(t)
	asg := autoscalingtypes.AutoScalingGroup{
		AutoScalingGroupName: ptrString("empty-asg"),
		MinSize:              ptrInt32(0),
		MaxSize:              ptrInt32(0),
		DesiredCapacity:      ptrInt32(0),
		CreatedTime:          ptrTime(svcTestTime),
		AvailabilityZones:    []string{},
	}
	res := buildResource("empty-asg", "empty-asg", asg)
	m := newDetailModel(res, "asg", configForType("asg"))

	view := m.View()
	if view == "" {
		t.Error("ASG detail view should not be empty even with minimal fields")
	}
}

func TestQA_Detail_ASG_FrameTitle(t *testing.T) {
	asg := realisticASG()
	res := buildResource("my-app-asg", "my-app-asg", asg)
	m := newDetailModel(res, "asg", configForType("asg"))

	if title := m.FrameTitle(); title != "my-app-asg" {
		t.Errorf("FrameTitle expected %q, got %q", "my-app-asg", title)
	}
}

// ---------------------------------------------------------------------------
// IAM User
// ---------------------------------------------------------------------------

func TestQA_Detail_IAMUser_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	user := realisticIAMUser()
	res := buildResource("deploy-user", "deploy-user", user)
	m := newDetailModel(res, "iam-user", configForType("iam-user"))

	view := m.View()
	for _, expected := range []string{
		"UserName", "deploy-user",
		"UserId", "AIDAEXAMPLEUSERID",
		"Arn", "arn:aws:iam",
		"Path", "/",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("IAM User detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_IAMUser_NilFields(t *testing.T) {
	ensureNoColor(t)
	user := iamtypes.User{
		UserName:   ptrString("empty-user"),
		UserId:     ptrString("AIDAEMPTY"),
		Arn:        ptrString("arn:aws:iam::123456789012:user/empty-user"),
		Path:       ptrString("/"),
		CreateDate: ptrTime(svcTestTime),
	}
	res := buildResource("empty-user", "empty-user", user)
	m := newDetailModel(res, "iam-user", configForType("iam-user"))

	view := m.View()
	if view == "" {
		t.Error("IAM User detail view should not be empty even with minimal fields")
	}
}

func TestQA_Detail_IAMUser_FrameTitle(t *testing.T) {
	user := realisticIAMUser()
	res := buildResource("deploy-user", "deploy-user", user)
	m := newDetailModel(res, "iam-user", configForType("iam-user"))

	if title := m.FrameTitle(); title != "deploy-user" {
		t.Errorf("FrameTitle expected %q, got %q", "deploy-user", title)
	}
}

// ---------------------------------------------------------------------------
// IAM Group
// ---------------------------------------------------------------------------

func TestQA_Detail_IAMGroup_ViewContainsExpectedFields(t *testing.T) {
	ensureNoColor(t)
	group := realisticIAMGroup()
	res := buildResource("developers", "developers", group)
	m := newDetailModel(res, "iam-group", configForType("iam-group"))

	view := m.View()
	for _, expected := range []string{
		"GroupName", "developers",
		"GroupId", "AGPAEXAMPLEGROUPID",
		"Arn", "arn:aws:iam",
		"Path", "/",
	} {
		if !strings.Contains(view, expected) {
			t.Errorf("IAM Group detail should contain %q, got:\n%s", expected, view)
		}
	}
}

func TestQA_Detail_IAMGroup_NilFields(t *testing.T) {
	ensureNoColor(t)
	group := iamtypes.Group{
		GroupName:  ptrString("empty-group"),
		GroupId:    ptrString("AGPAEMPTY"),
		Arn:        ptrString("arn:aws:iam::123456789012:group/empty-group"),
		Path:       ptrString("/"),
		CreateDate: ptrTime(svcTestTime),
	}
	res := buildResource("empty-group", "empty-group", group)
	m := newDetailModel(res, "iam-group", configForType("iam-group"))

	view := m.View()
	if view == "" {
		t.Error("IAM Group detail view should not be empty even with minimal fields")
	}
}

func TestQA_Detail_IAMGroup_FrameTitle(t *testing.T) {
	group := realisticIAMGroup()
	res := buildResource("developers", "developers", group)
	m := newDetailModel(res, "iam-group", configForType("iam-group"))

	if title := m.FrameTitle(); title != "developers" {
		t.Errorf("FrameTitle expected %q, got %q", "developers", title)
	}
}
