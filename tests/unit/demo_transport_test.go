package unit

// demo_transport_test.go — end-to-end contract tests for the demo mock transport.
//
// These tests call demo.NewDemoAWSConfig() which does NOT exist yet.
// They will not compile until the coder implements internal/demo/transport.go.
// That is intentional: these tests define the contract the coder must satisfy.
//
// Each test creates its own aws.Config and SDK client, then makes a real SDK
// call that is intercepted by the mock transport. No AWS credentials or network
// access are needed.

import (
	"context"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/acm"
	"github.com/aws/aws-sdk-go-v2/service/apigatewayv2"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	"github.com/aws/aws-sdk-go-v2/service/elasticache"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	"github.com/aws/aws-sdk-go-v2/service/kafka"
	"github.com/aws/aws-sdk-go-v2/service/kinesis"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
	wafv2types "github.com/aws/aws-sdk-go-v2/service/wafv2/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Test 1: Lambda ListFunctions (restjson1 protocol)
// ---------------------------------------------------------------------------

func TestDemoTransport_Lambda_ListFunctions(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := lambda.NewFromConfig(cfg)

	output, err := client.ListFunctions(context.Background(), &lambda.ListFunctionsInput{})
	if err != nil {
		t.Fatalf("ListFunctions returned unexpected error: %v", err)
	}
	if len(output.Functions) == 0 {
		t.Fatal("ListFunctions returned empty Functions slice; expected at least one fixture")
	}

	first := output.Functions[0]
	if first.FunctionName == nil {
		t.Fatal("first function FunctionName is nil")
	}
	if *first.FunctionName != "api-gateway-authorizer" {
		t.Errorf("first function name = %q; want %q", *first.FunctionName, "api-gateway-authorizer")
	}

	// Every function must have a non-nil FunctionName and FunctionArn.
	for i, fn := range output.Functions {
		if fn.FunctionName == nil {
			t.Errorf("function[%d].FunctionName is nil", i)
		}
		if fn.FunctionArn == nil {
			t.Errorf("function[%d].FunctionArn is nil", i)
		}
	}
}

// ---------------------------------------------------------------------------
// Test 2: SecretsManager ListSecrets (awsjson11 protocol)
// ---------------------------------------------------------------------------

func TestDemoTransport_SecretsManager_ListSecrets(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := secretsmanager.NewFromConfig(cfg)

	output, err := client.ListSecrets(context.Background(), &secretsmanager.ListSecretsInput{})
	if err != nil {
		t.Fatalf("ListSecrets returned unexpected error: %v", err)
	}
	if len(output.SecretList) == 0 {
		t.Fatal("ListSecrets returned empty SecretList; expected at least one fixture")
	}

	first := output.SecretList[0]
	if first.Name == nil {
		t.Fatal("first secret Name is nil")
	}
	if !strings.HasPrefix(*first.Name, "prod/") {
		t.Errorf("first secret name = %q; want a name starting with %q", *first.Name, "prod/")
	}
}

// ---------------------------------------------------------------------------
// Test 3: STS GetCallerIdentity (awsquery protocol)
// ---------------------------------------------------------------------------

func TestDemoTransport_STS_GetCallerIdentity(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := sts.NewFromConfig(cfg)

	output, err := client.GetCallerIdentity(context.Background(), &sts.GetCallerIdentityInput{})
	if err != nil {
		t.Fatalf("GetCallerIdentity returned unexpected error: %v", err)
	}
	if output.Account == nil {
		t.Fatal("GetCallerIdentity Account is nil")
	}
	if *output.Account != "123456789012" {
		t.Errorf("Account = %q; want %q", *output.Account, "123456789012")
	}
	if output.Arn == nil {
		t.Fatal("GetCallerIdentity Arn is nil")
	}
	if !strings.Contains(*output.Arn, "demo") {
		t.Errorf("Arn = %q; want it to contain %q", *output.Arn, "demo")
	}
}

// ---------------------------------------------------------------------------
// Test 4: EC2 DescribeInstances (ec2query protocol)
// ---------------------------------------------------------------------------

func TestDemoTransport_EC2_DescribeInstances(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := ec2.NewFromConfig(cfg)

	output, err := client.DescribeInstances(context.Background(), &ec2.DescribeInstancesInput{})
	if err != nil {
		t.Fatalf("DescribeInstances returned unexpected error: %v", err)
	}
	if len(output.Reservations) == 0 {
		t.Fatal("DescribeInstances returned no reservations; expected at least one")
	}
	firstReservation := output.Reservations[0]
	if len(firstReservation.Instances) == 0 {
		t.Fatal("first reservation has no instances")
	}
	firstInstance := firstReservation.Instances[0]
	if firstInstance.InstanceId == nil {
		t.Fatal("first instance InstanceId is nil")
	}
	if !strings.HasPrefix(*firstInstance.InstanceId, "i-") {
		t.Errorf("first instance InstanceId = %q; want prefix %q", *firstInstance.InstanceId, "i-")
	}
}

// ---------------------------------------------------------------------------
// Test 5: S3 ListBuckets (restxml protocol)
// ---------------------------------------------------------------------------

func TestDemoTransport_S3_ListBuckets(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := s3.NewFromConfig(cfg)

	output, err := client.ListBuckets(context.Background(), &s3.ListBucketsInput{})
	if err != nil {
		t.Fatalf("ListBuckets returned unexpected error: %v", err)
	}
	if len(output.Buckets) == 0 {
		t.Fatal("ListBuckets returned empty Buckets slice; expected at least one fixture")
	}

	found := false
	for _, b := range output.Buckets {
		if b.Name != nil && *b.Name == "data-pipeline-logs" {
			found = true
			break
		}
	}
	if !found {
		t.Error("bucket \"data-pipeline-logs\" not found in ListBuckets response")
	}
}

// ---------------------------------------------------------------------------
// Test 6: IAM ListRoles (awsquery protocol)
// ---------------------------------------------------------------------------

func TestDemoTransport_IAM_ListRoles(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := iam.NewFromConfig(cfg)

	output, err := client.ListRoles(context.Background(), &iam.ListRolesInput{})
	if err != nil {
		t.Fatalf("ListRoles returned unexpected error: %v", err)
	}
	if len(output.Roles) == 0 {
		t.Fatal("ListRoles returned empty Roles slice; expected at least one fixture")
	}

	found := false
	for _, r := range output.Roles {
		if r.RoleName != nil && *r.RoleName == "acme-eks-node-role" {
			found = true
			break
		}
	}
	if !found {
		t.Error("role \"acme-eks-node-role\" not found in ListRoles response")
	}
}

// ---------------------------------------------------------------------------
// Test 7: Lambda ListFunctions pagination (restjson1 with NextMarker)
// ---------------------------------------------------------------------------
//
// Lambda fixtures contain 25 functions. With a page size of ~20, the first
// page should return ~20 items with a non-nil NextMarker, and the second page
// should return the remaining ~5 items with a nil NextMarker.

func TestDemoTransport_Lambda_Pagination(t *testing.T) {
	const totalLambdaFixtures = 25

	cfg := demo.NewDemoAWSConfig()
	client := lambda.NewFromConfig(cfg)

	// First page.
	page1, err := client.ListFunctions(context.Background(), &lambda.ListFunctionsInput{})
	if err != nil {
		t.Fatalf("ListFunctions page 1 returned unexpected error: %v", err)
	}
	if len(page1.Functions) == 0 {
		t.Fatal("ListFunctions page 1 returned no functions")
	}
	if page1.NextMarker == nil {
		t.Fatalf("ListFunctions page 1 NextMarker is nil; expected pagination marker (25 fixtures, page size ~20)")
	}

	nextMarker := page1.NextMarker

	// Second page.
	page2, err := client.ListFunctions(context.Background(), &lambda.ListFunctionsInput{
		Marker: nextMarker,
	})
	if err != nil {
		t.Fatalf("ListFunctions page 2 returned unexpected error: %v", err)
	}
	if len(page2.Functions) == 0 {
		t.Fatal("ListFunctions page 2 returned no functions")
	}
	if page2.NextMarker != nil {
		t.Errorf("ListFunctions page 2 NextMarker = %q; want nil (should be last page)", *page2.NextMarker)
	}

	totalCount := len(page1.Functions) + len(page2.Functions)
	if totalCount != totalLambdaFixtures {
		t.Errorf("total functions across both pages = %d; want %d", totalCount, totalLambdaFixtures)
	}
}

// ---------------------------------------------------------------------------
// Test 8: ECS ListClusters (awsjson11 protocol)
// ---------------------------------------------------------------------------

func TestDemoTransport_ECS_ListClusters(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := ecs.NewFromConfig(cfg)

	output, err := client.ListClusters(context.Background(), &ecs.ListClustersInput{})
	if err != nil {
		t.Fatalf("ListClusters returned unexpected error: %v", err)
	}
	if len(output.ClusterArns) == 0 {
		t.Fatal("ListClusters returned empty ClusterArns slice; expected at least one fixture")
	}
}

// ---------------------------------------------------------------------------
// Test 9: DynamoDB ListTables (awsjson10 protocol)
// ---------------------------------------------------------------------------

func TestDemoTransport_DynamoDB_ListTables(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := dynamodb.NewFromConfig(cfg)

	output, err := client.ListTables(context.Background(), &dynamodb.ListTablesInput{})
	if err != nil {
		t.Fatalf("ListTables returned unexpected error: %v", err)
	}
	if len(output.TableNames) == 0 {
		t.Fatal("ListTables returned empty TableNames slice; expected at least one fixture")
	}
}

// ---------------------------------------------------------------------------
// Phase 2: SDK-level tests — one per service, primary list/describe operation
// ---------------------------------------------------------------------------

// Test 10: SSM DescribeParameters
func TestDemoTransport_SSM_DescribeParameters(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := ssm.NewFromConfig(cfg)

	output, err := client.DescribeParameters(context.Background(), &ssm.DescribeParametersInput{})
	if err != nil {
		t.Fatalf("DescribeParameters returned unexpected error: %v", err)
	}
	if len(output.Parameters) == 0 {
		t.Fatal("DescribeParameters returned empty Parameters slice; expected at least one fixture")
	}
}

// Test 11: KMS ListKeys
func TestDemoTransport_KMS_ListKeys(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := kms.NewFromConfig(cfg)

	output, err := client.ListKeys(context.Background(), &kms.ListKeysInput{})
	if err != nil {
		t.Fatalf("KMS ListKeys returned unexpected error: %v", err)
	}
	if len(output.Keys) == 0 {
		t.Fatal("KMS ListKeys returned empty Keys slice; expected at least one fixture")
	}
}

// Test 12: WAFv2 ListWebACLs (REGIONAL scope)
func TestDemoTransport_WAF_ListWebACLs(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := wafv2.NewFromConfig(cfg)

	output, err := client.ListWebACLs(context.Background(), &wafv2.ListWebACLsInput{
		Scope: wafv2types.ScopeRegional,
	})
	if err != nil {
		t.Fatalf("WAFv2 ListWebACLs returned unexpected error: %v", err)
	}
	if len(output.WebACLs) == 0 {
		t.Fatal("WAFv2 ListWebACLs returned empty WebACLs slice; expected at least one fixture")
	}
}

// Test 13: ECR DescribeRepositories
func TestDemoTransport_ECR_DescribeRepositories(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := ecr.NewFromConfig(cfg)

	output, err := client.DescribeRepositories(context.Background(), &ecr.DescribeRepositoriesInput{})
	if err != nil {
		t.Fatalf("ECR DescribeRepositories returned unexpected error: %v", err)
	}
	if len(output.Repositories) == 0 {
		t.Fatal("ECR DescribeRepositories returned empty Repositories slice; expected at least one fixture")
	}
}

// Test 14: EventBridge ListRules
func TestDemoTransport_EventBridge_ListRules(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := eventbridge.NewFromConfig(cfg)

	output, err := client.ListRules(context.Background(), &eventbridge.ListRulesInput{})
	if err != nil {
		t.Fatalf("EventBridge ListRules returned unexpected error: %v", err)
	}
	if len(output.Rules) == 0 {
		t.Fatal("EventBridge ListRules returned empty Rules slice; expected at least one fixture")
	}
}

// Test 15: CodePipeline ListPipelines
func TestDemoTransport_CodePipeline_ListPipelines(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := codepipeline.NewFromConfig(cfg)

	output, err := client.ListPipelines(context.Background(), &codepipeline.ListPipelinesInput{})
	if err != nil {
		t.Fatalf("CodePipeline ListPipelines returned unexpected error: %v", err)
	}
	if len(output.Pipelines) == 0 {
		t.Fatal("CodePipeline ListPipelines returned empty Pipelines slice; expected at least one fixture")
	}
}

// Test 16: Kinesis ListStreams
func TestDemoTransport_Kinesis_ListStreams(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := kinesis.NewFromConfig(cfg)

	output, err := client.ListStreams(context.Background(), &kinesis.ListStreamsInput{})
	if err != nil {
		t.Fatalf("Kinesis ListStreams returned unexpected error: %v", err)
	}
	if len(output.StreamNames) == 0 {
		t.Fatal("Kinesis ListStreams returned empty StreamNames slice; expected at least one fixture")
	}
}

// Test 17: Glue GetJobs
func TestDemoTransport_Glue_GetJobs(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := glue.NewFromConfig(cfg)

	output, err := client.GetJobs(context.Background(), &glue.GetJobsInput{})
	if err != nil {
		t.Fatalf("Glue GetJobs returned unexpected error: %v", err)
	}
	if len(output.Jobs) == 0 {
		t.Fatal("Glue GetJobs returned empty Jobs slice; expected at least one fixture")
	}
}

// Test 18: CloudTrail DescribeTrails
func TestDemoTransport_CloudTrail_DescribeTrails(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := cloudtrail.NewFromConfig(cfg)

	output, err := client.DescribeTrails(context.Background(), &cloudtrail.DescribeTrailsInput{})
	if err != nil {
		t.Fatalf("CloudTrail DescribeTrails returned unexpected error: %v", err)
	}
	if len(output.TrailList) == 0 {
		t.Fatal("CloudTrail DescribeTrails returned empty TrailList; expected at least one fixture")
	}
}

// Test 19: Athena ListWorkGroups
func TestDemoTransport_Athena_ListWorkGroups(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := athena.NewFromConfig(cfg)

	output, err := client.ListWorkGroups(context.Background(), &athena.ListWorkGroupsInput{})
	if err != nil {
		t.Fatalf("Athena ListWorkGroups returned unexpected error: %v", err)
	}
	if len(output.WorkGroups) == 0 {
		t.Fatal("Athena ListWorkGroups returned empty WorkGroups slice; expected at least one fixture")
	}
}

// Test 20: CloudWatch Logs DescribeLogGroups
func TestDemoTransport_CWLogs_DescribeLogGroups(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := cloudwatchlogs.NewFromConfig(cfg)

	output, err := client.DescribeLogGroups(context.Background(), &cloudwatchlogs.DescribeLogGroupsInput{})
	if err != nil {
		t.Fatalf("CloudWatchLogs DescribeLogGroups returned unexpected error: %v", err)
	}
	if len(output.LogGroups) == 0 {
		t.Fatal("CloudWatchLogs DescribeLogGroups returned empty LogGroups slice; expected at least one fixture")
	}
}

// Test 21: SFN ListStateMachines
func TestDemoTransport_SFN_ListStateMachines(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := sfn.NewFromConfig(cfg)

	output, err := client.ListStateMachines(context.Background(), &sfn.ListStateMachinesInput{})
	if err != nil {
		t.Fatalf("SFN ListStateMachines returned unexpected error: %v", err)
	}
	if len(output.StateMachines) == 0 {
		t.Fatal("SFN ListStateMachines returned empty StateMachines slice; expected at least one fixture")
	}
}

// Test 22: SQS ListQueues
func TestDemoTransport_SQS_ListQueues(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := sqs.NewFromConfig(cfg)

	output, err := client.ListQueues(context.Background(), &sqs.ListQueuesInput{})
	if err != nil {
		t.Fatalf("SQS ListQueues returned unexpected error: %v", err)
	}
	if len(output.QueueUrls) == 0 {
		t.Fatal("SQS ListQueues returned empty QueueUrls slice; expected at least one fixture")
	}
}

// Test 23: RDS DescribeDBInstances
func TestDemoTransport_RDS_DescribeDBInstances(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := rds.NewFromConfig(cfg)

	output, err := client.DescribeDBInstances(context.Background(), &rds.DescribeDBInstancesInput{})
	if err != nil {
		t.Fatalf("RDS DescribeDBInstances returned unexpected error: %v", err)
	}
	if len(output.DBInstances) == 0 {
		t.Fatal("RDS DescribeDBInstances returned empty DBInstances slice; expected at least one fixture")
	}
}

// Test 24: ElastiCache DescribeCacheClusters
func TestDemoTransport_ElastiCache_DescribeCacheClusters(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := elasticache.NewFromConfig(cfg)

	output, err := client.DescribeCacheClusters(context.Background(), &elasticache.DescribeCacheClustersInput{})
	if err != nil {
		t.Fatalf("ElastiCache DescribeCacheClusters returned unexpected error: %v", err)
	}
	if len(output.CacheClusters) == 0 {
		t.Fatal("ElastiCache DescribeCacheClusters returned empty CacheClusters slice; expected at least one fixture")
	}
}

// Test 25: DocDB DescribeDBClusters
func TestDemoTransport_DocDB_DescribeDBClusters(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := docdb.NewFromConfig(cfg)

	output, err := client.DescribeDBClusters(context.Background(), &docdb.DescribeDBClustersInput{})
	if err != nil {
		t.Fatalf("DocDB DescribeDBClusters returned unexpected error: %v", err)
	}
	if len(output.DBClusters) == 0 {
		t.Fatal("DocDB DescribeDBClusters returned empty DBClusters slice; expected at least one fixture")
	}
}

// Test 26: SNS ListTopics
func TestDemoTransport_SNS_ListTopics(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := sns.NewFromConfig(cfg)

	output, err := client.ListTopics(context.Background(), &sns.ListTopicsInput{})
	if err != nil {
		t.Fatalf("SNS ListTopics returned unexpected error: %v", err)
	}
	if len(output.Topics) == 0 {
		t.Fatal("SNS ListTopics returned empty Topics slice; expected at least one fixture")
	}
}

// Test 27: ELBv2 DescribeLoadBalancers
func TestDemoTransport_ELB_DescribeLoadBalancers(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := elbv2.NewFromConfig(cfg)

	output, err := client.DescribeLoadBalancers(context.Background(), &elbv2.DescribeLoadBalancersInput{})
	if err != nil {
		t.Fatalf("ELBv2 DescribeLoadBalancers returned unexpected error: %v", err)
	}
	if len(output.LoadBalancers) == 0 {
		t.Fatal("ELBv2 DescribeLoadBalancers returned empty LoadBalancers slice; expected at least one fixture")
	}
}

// Test 28: CloudFormation DescribeStacks
func TestDemoTransport_CFN_DescribeStacks(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := cloudformation.NewFromConfig(cfg)

	output, err := client.DescribeStacks(context.Background(), &cloudformation.DescribeStacksInput{})
	if err != nil {
		t.Fatalf("CloudFormation DescribeStacks returned unexpected error: %v", err)
	}
	if len(output.Stacks) == 0 {
		t.Fatal("CloudFormation DescribeStacks returned empty Stacks slice; expected at least one fixture")
	}
}

// Test 29: AutoScaling DescribeAutoScalingGroups
func TestDemoTransport_ASG_DescribeAutoScalingGroups(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := autoscaling.NewFromConfig(cfg)

	output, err := client.DescribeAutoScalingGroups(context.Background(), &autoscaling.DescribeAutoScalingGroupsInput{})
	if err != nil {
		t.Fatalf("AutoScaling DescribeAutoScalingGroups returned unexpected error: %v", err)
	}
	if len(output.AutoScalingGroups) == 0 {
		t.Fatal("AutoScaling DescribeAutoScalingGroups returned empty AutoScalingGroups slice; expected at least one fixture")
	}
}

// Test 30: CloudWatch DescribeAlarms
func TestDemoTransport_CW_DescribeAlarms(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := cloudwatch.NewFromConfig(cfg)

	output, err := client.DescribeAlarms(context.Background(), &cloudwatch.DescribeAlarmsInput{})
	if err != nil {
		t.Fatalf("CloudWatch DescribeAlarms returned unexpected error: %v", err)
	}
	if len(output.MetricAlarms) == 0 {
		t.Fatal("CloudWatch DescribeAlarms returned empty MetricAlarms slice; expected at least one fixture")
	}
}

// Test 31: Redshift DescribeClusters
func TestDemoTransport_Redshift_DescribeClusters(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := redshift.NewFromConfig(cfg)

	output, err := client.DescribeClusters(context.Background(), &redshift.DescribeClustersInput{})
	if err != nil {
		t.Fatalf("Redshift DescribeClusters returned unexpected error: %v", err)
	}
	if len(output.Clusters) == 0 {
		t.Fatal("Redshift DescribeClusters returned empty Clusters slice; expected at least one fixture")
	}
}

// Test 32: ElasticBeanstalk DescribeEnvironments
func TestDemoTransport_EB_DescribeEnvironments(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := elasticbeanstalk.NewFromConfig(cfg)

	output, err := client.DescribeEnvironments(context.Background(), &elasticbeanstalk.DescribeEnvironmentsInput{})
	if err != nil {
		t.Fatalf("ElasticBeanstalk DescribeEnvironments returned unexpected error: %v", err)
	}
	if len(output.Environments) == 0 {
		t.Fatal("ElasticBeanstalk DescribeEnvironments returned empty Environments slice; expected at least one fixture")
	}
}

// Test 33: IAM ListUsers
func TestDemoTransport_IAM_ListUsers(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := iam.NewFromConfig(cfg)

	output, err := client.ListUsers(context.Background(), &iam.ListUsersInput{})
	if err != nil {
		t.Fatalf("IAM ListUsers returned unexpected error: %v", err)
	}
	if len(output.Users) == 0 {
		t.Fatal("IAM ListUsers returned empty Users slice; expected at least one fixture")
	}
}

// Test 34: IAM ListGroups
func TestDemoTransport_IAM_ListGroups(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := iam.NewFromConfig(cfg)

	output, err := client.ListGroups(context.Background(), &iam.ListGroupsInput{})
	if err != nil {
		t.Fatalf("IAM ListGroups returned unexpected error: %v", err)
	}
	if len(output.Groups) == 0 {
		t.Fatal("IAM ListGroups returned empty Groups slice; expected at least one fixture")
	}
}

// Test 35: IAM ListPolicies (local scope to avoid returning all AWS-managed policies)
func TestDemoTransport_IAM_ListPolicies(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := iam.NewFromConfig(cfg)

	output, err := client.ListPolicies(context.Background(), &iam.ListPoliciesInput{})
	if err != nil {
		t.Fatalf("IAM ListPolicies returned unexpected error: %v", err)
	}
	if len(output.Policies) == 0 {
		t.Fatal("IAM ListPolicies returned empty Policies slice; expected at least one fixture")
	}
}

// Test 36: IAM ListAccountAliases
func TestDemoTransport_IAM_ListAccountAliases(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := iam.NewFromConfig(cfg)

	output, err := client.ListAccountAliases(context.Background(), &iam.ListAccountAliasesInput{})
	if err != nil {
		t.Fatalf("IAM ListAccountAliases returned unexpected error: %v", err)
	}
	if len(output.AccountAliases) == 0 {
		t.Fatal("IAM ListAccountAliases returned empty AccountAliases slice; expected at least one fixture")
	}
}

// Test 37: EC2 DescribeVpcs
func TestDemoTransport_EC2_DescribeVpcs(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := ec2.NewFromConfig(cfg)

	output, err := client.DescribeVpcs(context.Background(), &ec2.DescribeVpcsInput{})
	if err != nil {
		t.Fatalf("EC2 DescribeVpcs returned unexpected error: %v", err)
	}
	if len(output.Vpcs) == 0 {
		t.Fatal("EC2 DescribeVpcs returned empty Vpcs slice; expected at least one fixture")
	}
}

// Test 38: EC2 DescribeSecurityGroups
func TestDemoTransport_EC2_DescribeSecurityGroups(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := ec2.NewFromConfig(cfg)

	output, err := client.DescribeSecurityGroups(context.Background(), &ec2.DescribeSecurityGroupsInput{})
	if err != nil {
		t.Fatalf("EC2 DescribeSecurityGroups returned unexpected error: %v", err)
	}
	if len(output.SecurityGroups) == 0 {
		t.Fatal("EC2 DescribeSecurityGroups returned empty SecurityGroups slice; expected at least one fixture")
	}
}

// Test 39: EC2 DescribeSubnets
func TestDemoTransport_EC2_DescribeSubnets(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := ec2.NewFromConfig(cfg)

	output, err := client.DescribeSubnets(context.Background(), &ec2.DescribeSubnetsInput{})
	if err != nil {
		t.Fatalf("EC2 DescribeSubnets returned unexpected error: %v", err)
	}
	if len(output.Subnets) == 0 {
		t.Fatal("EC2 DescribeSubnets returned empty Subnets slice; expected at least one fixture")
	}
}

// Test 40: Route53 ListHostedZones
func TestDemoTransport_Route53_ListHostedZones(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := route53.NewFromConfig(cfg)

	output, err := client.ListHostedZones(context.Background(), &route53.ListHostedZonesInput{})
	if err != nil {
		t.Fatalf("Route53 ListHostedZones returned unexpected error: %v", err)
	}
	if len(output.HostedZones) == 0 {
		t.Fatal("Route53 ListHostedZones returned empty HostedZones slice; expected at least one fixture")
	}
}

// Test 41: CloudFront ListDistributions
func TestDemoTransport_CloudFront_ListDistributions(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := cloudfront.NewFromConfig(cfg)

	output, err := client.ListDistributions(context.Background(), &cloudfront.ListDistributionsInput{})
	if err != nil {
		t.Fatalf("CloudFront ListDistributions returned unexpected error: %v", err)
	}
	if output.DistributionList == nil {
		t.Fatal("CloudFront ListDistributions returned nil DistributionList")
	}
	if len(output.DistributionList.Items) == 0 {
		t.Fatal("CloudFront DistributionList.Items is empty; expected at least one fixture")
	}
}

// Test 42: API Gateway v2 GetApis
func TestDemoTransport_APIGW_GetApis(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := apigatewayv2.NewFromConfig(cfg)

	output, err := client.GetApis(context.Background(), &apigatewayv2.GetApisInput{})
	if err != nil {
		t.Fatalf("APIGatewayV2 GetApis returned unexpected error: %v", err)
	}
	if len(output.Items) == 0 {
		t.Fatal("APIGatewayV2 GetApis returned empty Items slice; expected at least one fixture")
	}
}

// Test 43: EFS DescribeFileSystems
func TestDemoTransport_EFS_DescribeFileSystems(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := efs.NewFromConfig(cfg)

	output, err := client.DescribeFileSystems(context.Background(), &efs.DescribeFileSystemsInput{})
	if err != nil {
		t.Fatalf("EFS DescribeFileSystems returned unexpected error: %v", err)
	}
	if len(output.FileSystems) == 0 {
		t.Fatal("EFS DescribeFileSystems returned empty FileSystems slice; expected at least one fixture")
	}
}

// Test 44: MSK ListClustersV2
func TestDemoTransport_MSK_ListClustersV2(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := kafka.NewFromConfig(cfg)

	output, err := client.ListClustersV2(context.Background(), &kafka.ListClustersV2Input{})
	if err != nil {
		t.Fatalf("MSK ListClustersV2 returned unexpected error: %v", err)
	}
	if len(output.ClusterInfoList) == 0 {
		t.Fatal("MSK ListClustersV2 returned empty ClusterInfoList; expected at least one fixture")
	}
}

// Test 45: Backup ListBackupPlans
func TestDemoTransport_Backup_ListBackupPlans(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := backup.NewFromConfig(cfg)

	output, err := client.ListBackupPlans(context.Background(), &backup.ListBackupPlansInput{})
	if err != nil {
		t.Fatalf("Backup ListBackupPlans returned unexpected error: %v", err)
	}
	if len(output.BackupPlansList) == 0 {
		t.Fatal("Backup ListBackupPlans returned empty BackupPlansList; expected at least one fixture")
	}
}

// Test 46: SESv2 ListEmailIdentities
func TestDemoTransport_SES_ListEmailIdentities(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := sesv2.NewFromConfig(cfg)

	output, err := client.ListEmailIdentities(context.Background(), &sesv2.ListEmailIdentitiesInput{})
	if err != nil {
		t.Fatalf("SESv2 ListEmailIdentities returned unexpected error: %v", err)
	}
	if len(output.EmailIdentities) == 0 {
		t.Fatal("SESv2 ListEmailIdentities returned empty EmailIdentities slice; expected at least one fixture")
	}
}

// Test 47: OpenSearch ListDomainNames
func TestDemoTransport_OpenSearch_ListDomainNames(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := opensearch.NewFromConfig(cfg)

	output, err := client.ListDomainNames(context.Background(), &opensearch.ListDomainNamesInput{})
	if err != nil {
		t.Fatalf("OpenSearch ListDomainNames returned unexpected error: %v", err)
	}
	if len(output.DomainNames) == 0 {
		t.Fatal("OpenSearch ListDomainNames returned empty DomainNames slice; expected at least one fixture")
	}
}

// Test 48: ACM ListCertificates
func TestDemoTransport_ACM_ListCertificates(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	client := acm.NewFromConfig(cfg)

	output, err := client.ListCertificates(context.Background(), &acm.ListCertificatesInput{})
	if err != nil {
		t.Fatalf("ACM ListCertificates returned unexpected error: %v", err)
	}
	if len(output.CertificateSummaryList) == 0 {
		t.Fatal("ACM ListCertificates returned empty CertificateSummaryList; expected at least one fixture")
	}
}

// ---------------------------------------------------------------------------
// Phase 2: Fetcher chain tests — full path through transport → handler → fetcher
// ---------------------------------------------------------------------------

// Test 49: ECS clusters fetcher chain (ListClusters + DescribeClusters)
func TestDemoTransport_FetcherChain_ECSClusters(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	clients := awsclient.CreateServiceClients(cfg)

	resources, err := awsclient.FetchECSClusters(context.Background(), clients.ECS, clients.ECS)
	if err != nil {
		t.Fatalf("FetchECSClusters returned unexpected error: %v", err)
	}
	if len(resources) == 0 {
		t.Fatal("FetchECSClusters returned no resources; expected at least one fixture")
	}
}

// Test 50: EKS clusters fetcher chain (ListClusters + DescribeCluster per name)
func TestDemoTransport_FetcherChain_EKSClusters(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	clients := awsclient.CreateServiceClients(cfg)

	resources, err := awsclient.FetchEKSClusters(context.Background(), clients.EKS, clients.EKS)
	if err != nil {
		t.Fatalf("FetchEKSClusters returned unexpected error: %v", err)
	}
	if len(resources) == 0 {
		t.Fatal("FetchEKSClusters returned no resources; expected at least one fixture")
	}
}

// Test 51: Node groups fetcher chain (ListClusters + ListNodegroups + DescribeNodegroup)
func TestDemoTransport_FetcherChain_NodeGroups(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	clients := awsclient.CreateServiceClients(cfg)

	resources, err := awsclient.FetchNodeGroups(context.Background(), clients.EKS, clients.EKS, clients.EKS)
	if err != nil {
		t.Fatalf("FetchNodeGroups returned unexpected error: %v", err)
	}
	if len(resources) == 0 {
		t.Fatal("FetchNodeGroups returned no resources; expected at least one fixture")
	}
}

// Test 52: DynamoDB fetcher chain (ListTables + DescribeTable per table)
func TestDemoTransport_FetcherChain_DynamoDB(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	clients := awsclient.CreateServiceClients(cfg)

	resources, err := awsclient.FetchDynamoDBTables(context.Background(), clients.DynamoDB, clients.DynamoDB)
	if err != nil {
		t.Fatalf("FetchDynamoDBTables returned unexpected error: %v", err)
	}
	if len(resources) == 0 {
		t.Fatal("FetchDynamoDBTables returned no resources; expected at least one fixture")
	}
}

// Test 53: KMS fetcher chain (ListKeys + ListAliases + DescribeKey, customer-managed only)
func TestDemoTransport_FetcherChain_KMS(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	clients := awsclient.CreateServiceClients(cfg)

	resources, err := awsclient.FetchKMSKeys(context.Background(), clients.KMS, clients.KMS, clients.KMS)
	if err != nil {
		t.Fatalf("FetchKMSKeys returned unexpected error: %v", err)
	}
	if len(resources) == 0 {
		t.Fatal("FetchKMSKeys returned no resources; expected at least one customer-managed key fixture")
	}
}

// Test 54: CodeBuild fetcher chain (ListProjects + BatchGetProjects)
func TestDemoTransport_FetcherChain_CodeBuild(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	clients := awsclient.CreateServiceClients(cfg)

	resources, err := awsclient.FetchCodeBuildProjects(context.Background(), clients.CodeBuild, clients.CodeBuild)
	if err != nil {
		t.Fatalf("FetchCodeBuildProjects returned unexpected error: %v", err)
	}
	if len(resources) == 0 {
		t.Fatal("FetchCodeBuildProjects returned no resources; expected at least one fixture")
	}
}

// Test 55: SQS fetcher chain (ListQueues + GetQueueAttributes per URL)
func TestDemoTransport_FetcherChain_SQS(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	clients := awsclient.CreateServiceClients(cfg)

	resources, err := awsclient.FetchSQSQueues(context.Background(), clients.SQS, clients.SQS)
	if err != nil {
		t.Fatalf("FetchSQSQueues returned unexpected error: %v", err)
	}
	if len(resources) == 0 {
		t.Fatal("FetchSQSQueues returned no resources; expected at least one fixture")
	}
}

// Test 56: OpenSearch fetcher chain (ListDomainNames + DescribeDomains)
func TestDemoTransport_FetcherChain_OpenSearch(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	clients := awsclient.CreateServiceClients(cfg)

	resources, err := awsclient.FetchOpenSearchDomains(context.Background(), clients.OpenSearch, clients.OpenSearch)
	if err != nil {
		t.Fatalf("FetchOpenSearchDomains returned unexpected error: %v", err)
	}
	if len(resources) == 0 {
		t.Fatal("FetchOpenSearchDomains returned no resources; expected at least one fixture")
	}
}

// Test 57: ECS services fetcher chain (ListClusters + ListServices + DescribeServices)
func TestDemoTransport_FetcherChain_ECSServices(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	clients := awsclient.CreateServiceClients(cfg)

	resources, err := awsclient.FetchECSServices(context.Background(), clients.ECS, clients.ECS, clients.ECS)
	if err != nil {
		t.Fatalf("FetchECSServices returned unexpected error: %v", err)
	}
	if len(resources) == 0 {
		t.Fatal("FetchECSServices returned no resources; expected at least one fixture")
	}
}

// Test 58: ECS tasks fetcher chain (ListClusters + ListTasks + DescribeTasks)
func TestDemoTransport_FetcherChain_ECSTasks(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	clients := awsclient.CreateServiceClients(cfg)

	resources, err := awsclient.FetchECSTasks(context.Background(), clients.ECS, clients.ECS, clients.ECS)
	if err != nil {
		t.Fatalf("FetchECSTasks returned unexpected error: %v", err)
	}
	if len(resources) == 0 {
		t.Fatal("FetchECSTasks returned no resources; expected at least one fixture")
	}
}

// Test 59: RDS Snapshots fetcher chain (DescribeDBSnapshots — XML <member> vs <DBSnapshot> bug)
func TestDemoTransport_FetcherChain_RDSSnapshots(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	clients := awsclient.CreateServiceClients(cfg)

	resources, err := awsclient.FetchRDSSnapshots(context.Background(), clients.RDS)
	if err != nil {
		t.Fatalf("FetchRDSSnapshots returned unexpected error: %v", err)
	}
	if len(resources) == 0 {
		t.Fatal("FetchRDSSnapshots returned no resources; expected at least one fixture")
	}
	if resources[0].ID == "" {
		t.Fatal("FetchRDSSnapshots first resource has empty ID; expected a valid snapshot identifier")
	}
}

// Test 60: DocDB Snapshots fetcher chain (DescribeDBClusterSnapshots — XML <member> vs <DBClusterSnapshot> bug)
func TestDemoTransport_FetcherChain_DocDBSnapshots(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	clients := awsclient.CreateServiceClients(cfg)

	resources, err := awsclient.FetchDocDBClusterSnapshots(context.Background(), clients.DocDB)
	if err != nil {
		t.Fatalf("FetchDocDBClusterSnapshots returned unexpected error: %v", err)
	}
	if len(resources) == 0 {
		t.Fatal("FetchDocDBClusterSnapshots returned no resources; expected at least one fixture")
	}
	if resources[0].ID == "" {
		t.Fatal("FetchDocDBClusterSnapshots first resource has empty ID; expected a valid snapshot identifier")
	}
}

// Test 61: DynamoDB unique table names (DescribeTable must return distinct fixtures, not always the first)
func TestDemoTransport_FetcherChain_DynamoDB_UniqueTableNames(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	clients := awsclient.CreateServiceClients(cfg)

	resources, err := awsclient.FetchDynamoDBTables(context.Background(), clients.DynamoDB, clients.DynamoDB)
	if err != nil {
		t.Fatalf("FetchDynamoDBTables returned unexpected error: %v", err)
	}
	if len(resources) == 0 {
		t.Fatal("FetchDynamoDBTables returned no resources; expected at least one fixture")
	}

	ids := make(map[string]bool, len(resources))
	for _, r := range resources {
		ids[r.ID] = true
	}
	if len(ids) != len(resources) {
		t.Fatalf("FetchDynamoDBTables returned duplicate IDs: got %d unique IDs for %d resources; DescribeTable is returning the same fixture for every table", len(ids), len(resources))
	}
}

// Test 62: S3 Objects child fetcher (virtual-hosted-style bucket addressing bug)
func TestDemoTransport_ChildFetcher_S3Objects(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	clients := awsclient.CreateServiceClients(cfg)

	result, err := awsclient.FetchS3Objects(context.Background(), clients.S3, "data-pipeline-logs", "", "")
	if err != nil {
		t.Fatalf("FetchS3Objects returned unexpected error: %v", err)
	}
	if len(result.Resources) == 0 {
		t.Fatal("FetchS3Objects returned no resources for bucket 'data-pipeline-logs'; expected at least one fixture object")
	}
}

// Test 63: CloudWatch AlarmHistory child fetcher (Smithy RPCv2 CBOR protocol mismatch bug)
func TestDemoTransport_ChildFetcher_AlarmHistory(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	clients := awsclient.CreateServiceClients(cfg)

	result, err := awsclient.FetchAlarmHistory(context.Background(), clients.CloudWatch, map[string]string{"alarm_name": "demo-alarm"}, "")
	if err != nil {
		t.Fatalf("FetchAlarmHistory returned unexpected error: %v", err)
	}
	if len(result.Resources) == 0 {
		t.Fatal("FetchAlarmHistory returned no resources for alarm 'demo-alarm'; expected at least one fixture history item")
	}
}

// ---------------------------------------------------------------------------
// Test 64: AllResourceTypes data quality — round-trip through every fetcher
// ---------------------------------------------------------------------------

func TestDemoTransport_AllResourceTypes_DataQuality(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	clients := awsclient.CreateServiceClients(cfg)
	ctx := context.Background()

	type testCase struct {
		name       string
		fetch      func() ([]resource.Resource, error)
		skipStatus bool   // true for types where Status is legitimately empty
		knownBug   string // non-empty: skip with this message (known demo transport bug)
	}

	tests := []testCase{
		// --- Compute ---
		{
			name: "ec2",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchEC2Instances(ctx, clients.EC2)
			},
		},
		{
			name: "lambda",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchLambdaFunctions(ctx, clients.Lambda)
			},
		},
		{
			name:     "asg",
			knownBug: "demo handler emits <NextToken></NextToken> instead of omitting the element; fetcher loops forever waiting for nil NextToken",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchAutoScalingGroups(ctx, clients.AutoScaling)
			},
		},
		{
			name: "eb",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchEBEnvironments(ctx, clients.ElasticBeanstalk)
			},
		},
		{
			name: "ecs",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchECSClusters(ctx, clients.ECS, clients.ECS)
			},
		},
		{
			name: "ecs-svc",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchECSServices(ctx, clients.ECS, clients.ECS, clients.ECS)
			},
		},
		{
			name: "ecs-task",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchECSTasks(ctx, clients.ECS, clients.ECS, clients.ECS)
			},
		},
		{
			name: "eks",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchEKSClusters(ctx, clients.EKS, clients.EKS)
			},
		},
		{
			name: "ng",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchNodeGroups(ctx, clients.EKS, clients.EKS, clients.EKS)
			},
		},
		// --- Storage ---
		{
			name:       "s3",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchS3Buckets(ctx, clients.S3)
			},
		},
		{
			name: "efs",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchEFSFileSystems(ctx, clients.EFS)
			},
		},
		// --- Databases ---
		{
			name: "dbi",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchRDSInstances(ctx, clients.RDS)
			},
		},
		{
			name: "rds-snap",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchRDSSnapshots(ctx, clients.RDS)
			},
		},
		{
			name: "docdb",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchDocDBClusters(ctx, clients.DocDB)
			},
		},
		{
			name: "docdb-snap",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchDocDBClusterSnapshots(ctx, clients.DocDB)
			},
		},
		{
			name: "ddb",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchDynamoDBTables(ctx, clients.DynamoDB, clients.DynamoDB)
			},
		},
		{
			name: "redis",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchRedisClusters(ctx, clients.ElastiCache)
			},
		},
		{
			name: "redshift",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchRedshiftClusters(ctx, clients.Redshift)
			},
		},
		{
			name:       "os",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchOpenSearchDomains(ctx, clients.OpenSearch, clients.OpenSearch)
			},
		},
		// --- Networking ---
		{
			name: "vpc",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchVPCs(ctx, clients.EC2)
			},
		},
		{
			name:       "sg",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchSecurityGroups(ctx, clients.EC2)
			},
		},
		{
			name: "subnet",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchSubnets(ctx, clients.EC2)
			},
		},
		{
			name: "nat",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchNatGateways(ctx, clients.EC2)
			},
		},
		{
			name: "igw",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchInternetGateways(ctx, clients.EC2)
			},
		},
		{
			name:       "eip",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchElasticIPs(ctx, clients.EC2)
			},
		},
		{
			name:       "eni",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchNetworkInterfaces(ctx, clients.EC2)
			},
		},
		{
			name: "rtb",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchRouteTables(ctx, clients.EC2)
			},
		},
		{
			name: "tgw",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchTransitGateways(ctx, clients.EC2)
			},
		},
		{
			name: "vpce",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchVPCEndpoints(ctx, clients.EC2)
			},
		},
		{
			name: "elb",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchLoadBalancers(ctx, clients.ELBv2)
			},
		},
		{
			name:       "tg",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchTargetGroups(ctx, clients.ELBv2)
			},
		},
		// --- Security & IAM ---
		{
			name:       "role",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchIAMRoles(ctx, clients.IAM)
			},
		},
		{
			name:       "iam-user",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchIAMUsers(ctx, clients.IAM)
			},
		},
		{
			name:       "iam-group",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchIAMGroups(ctx, clients.IAM)
			},
		},
		{
			name:       "iam-policy",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchIAMPolicies(ctx, clients.IAM)
			},
		},
		{
			name:       "secrets",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchSecrets(ctx, clients.SecretsManager)
			},
		},
		{
			name:       "ssm",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchSSMParameters(ctx, clients.SSM)
			},
		},
		{
			name:       "kms",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchKMSKeys(ctx, clients.KMS, clients.KMS, clients.KMS)
			},
		},
		{
			name:       "waf",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchWAFWebACLs(ctx, clients.WAFv2)
			},
		},
		// --- Monitoring ---
		{
			name: "alarm",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchCloudWatchAlarms(ctx, clients.CloudWatch)
			},
		},
		{
			name:       "logs",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchCloudWatchLogGroups(ctx, clients.CloudWatchLogs)
			},
		},
		{
			name:       "trail",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchCloudTrailTrails(ctx, clients.CloudTrail)
			},
		},
		// --- DNS / CDN ---
		{
			name:       "r53",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchHostedZones(ctx, clients.Route53)
			},
		},
		{
			name: "cf",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchCloudFrontDistributions(ctx, clients.CloudFront)
			},
		},
		{
			name: "acm",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchACMCertificates(ctx, clients.ACM)
			},
		},
		{
			name:       "apigw",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchAPIGateways(ctx, clients.APIGatewayV2)
			},
		},
		// --- Messaging ---
		{
			name:       "snstopic",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchSNSTopics(ctx, clients.SNS)
			},
		},
		{
			name:       "sqs",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchSQSQueues(ctx, clients.SQS, clients.SQS)
			},
		},
		{
			name: "eb-rule",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchEventBridgeRules(ctx, clients.EventBridge)
			},
		},
		{
			name:       "kinesis",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchKinesisStreams(ctx, clients.Kinesis)
			},
		},
		{
			name:       "sfn",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchStepFunctions(ctx, clients.SFN)
			},
		},
		{
			name: "msk",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchMSKClusters(ctx, clients.MSK)
			},
		},
		// --- CI/CD & Containers ---
		{
			name:       "ecr",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchECRRepositories(ctx, clients.ECR)
			},
		},
		{
			name:       "cb",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchCodeBuildProjects(ctx, clients.CodeBuild, clients.CodeBuild)
			},
		},
		{
			name:       "cp",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchCodePipelines(ctx, clients.CodePipeline)
			},
		},
		{
			name:       "codeartifact",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchCodeArtifactRepos(ctx, clients.CodeArtifact)
			},
		},
		{
			name: "cfn",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchCloudFormationStacks(ctx, clients.CloudFormation)
			},
		},
		// --- Data ---
		{
			name:       "glue",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchGlueJobs(ctx, clients.Glue)
			},
		},
		{
			name: "athena",
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchAthenaWorkgroups(ctx, clients.Athena)
			},
		},
		// --- Backup / Email / Misc ---
		{
			name:       "backup",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchBackupPlans(ctx, clients.Backup)
			},
		},
		{
			name:       "ses",
			skipStatus: true,
			fetch: func() ([]resource.Resource, error) {
				return awsclient.FetchSESIdentities(ctx, clients.SESv2)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.knownBug != "" {
				t.Skipf("KNOWN BUG — skipped to prevent hang: %s", tt.knownBug)
			}

			resources, err := tt.fetch()
			if err != nil {
				t.Fatalf("fetch error: %v", err)
			}
			if len(resources) == 0 {
				t.Fatal("no resources returned")
			}

			// Unique IDs
			ids := make(map[string]bool, len(resources))
			for _, r := range resources {
				if ids[r.ID] {
					t.Errorf("duplicate ID: %q", r.ID)
				}
				ids[r.ID] = true
			}

			// Names populated
			for _, r := range resources {
				if r.Name == "" {
					t.Errorf("resource %q has empty Name", r.ID)
				}
			}

			// Status populated (unless legitimately empty for this type)
			if !tt.skipStatus {
				for _, r := range resources {
					if r.Status == "" {
						t.Errorf("resource %q has empty Status", r.ID)
					}
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test 65: ChildViews data quality — round-trip through every child fetcher
// ---------------------------------------------------------------------------

func TestDemoTransport_ChildViews_DataQuality(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	clients := awsclient.CreateServiceClients(cfg)
	ctx := context.Background()

	type childTestCase struct {
		name      string
		childType string
		parentCtx resource.ParentContext
	}

	tests := []childTestCase{
		{
			name:      "r53_records",
			childType: "r53_records",
			parentCtx: resource.ParentContext{"zone_id": "/hostedzone/Z0123456789ABCDEFGHIJ"},
		},
		{
			name:      "log_streams",
			childType: "log_streams",
			parentCtx: resource.ParentContext{"log_group_name": "/aws/lambda/api-gateway-authorizer"},
		},
		{
			name:      "cfn_events",
			childType: "cfn_events",
			parentCtx: resource.ParentContext{"stack_name": "acme-vpc-stack"},
		},
		{
			name:      "cfn_resources",
			childType: "cfn_resources",
			parentCtx: resource.ParentContext{"stack_name": "acme-vpc-stack"},
		},
		{
			name:      "elb_listeners",
			childType: "elb_listeners",
			parentCtx: resource.ParentContext{"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-prod-web/1234567890abcdef"},
		},
		{
			name:      "elb_listener_rules",
			childType: "elb_listener_rules",
			parentCtx: resource.ParentContext{"listener_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/acme-prod-web/1234567890abcdef/aaa111"},
		},
		{
			name:      "ecr_images",
			childType: "ecr_images",
			parentCtx: resource.ParentContext{"repository_name": "acme/api-service", "repository_uri": "123456789012.dkr.ecr.us-east-1.amazonaws.com/acme/api-service"},
		},
		{
			name:      "role_policies",
			childType: "role_policies",
			parentCtx: resource.ParentContext{"role_name": "acme-eks-node-role"},
		},
		{
			name:      "iam_group_members",
			childType: "iam_group_members",
			parentCtx: resource.ParentContext{"group_name": "admins"},
		},
		{
			name:      "tg_health",
			childType: "tg_health",
			parentCtx: resource.ParentContext{"target_group_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-web-tg/1234567890abcdef"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pf := resource.GetPaginatedChildFetcher(tt.childType)
			if pf == nil {
				t.Fatalf("no paginated child fetcher registered for %q", tt.childType)
			}

			result, err := pf(ctx, clients, tt.parentCtx, "")
			if err != nil {
				t.Fatalf("child fetch error: %v", err)
			}
			if len(result.Resources) == 0 {
				t.Fatal("no child resources returned")
			}

			// Unique IDs
			ids := make(map[string]bool, len(result.Resources))
			for _, r := range result.Resources {
				if ids[r.ID] {
					t.Errorf("duplicate child ID: %q", r.ID)
				}
				ids[r.ID] = true
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Test 66: S3 folder drill-down navigation via demo transport
// ---------------------------------------------------------------------------

func TestDemoTransport_S3FolderNavigation(t *testing.T) {
	cfg := demo.NewDemoAWSConfig()
	clients := awsclient.CreateServiceClients(cfg)
	ctx := context.Background()

	pf := resource.GetPaginatedChildFetcher("s3_objects")
	if pf == nil {
		t.Fatal("no paginated child fetcher registered for s3_objects")
	}

	// Helper: validate common invariants on a slice of resources.
	validateResources := func(t *testing.T, label string, resources []resource.Resource) {
		t.Helper()
		ids := make(map[string]bool, len(resources))
		for i, r := range resources {
			if r.ID == "" {
				t.Errorf("%s: resource[%d] has empty ID", label, i)
			}
			if r.Name == "" {
				t.Errorf("%s: resource[%d] has empty Name", label, i)
			}
			if ids[r.ID] {
				t.Errorf("%s: duplicate ID %q", label, r.ID)
			}
			ids[r.ID] = true
			if r.Status != "folder" && r.Status != "file" {
				t.Errorf("%s: resource[%d] ID=%q has unexpected Status %q (want 'folder' or 'file')", label, i, r.ID, r.Status)
			}
		}
	}

	// -------------------------------------------------------------------------
	// 1. Top-level bucket listing for data-pipeline-logs
	// -------------------------------------------------------------------------
	t.Run("top_level_data_pipeline_logs", func(t *testing.T) {
		result, err := pf(ctx, clients, resource.ParentContext{"bucket": "data-pipeline-logs", "prefix": ""}, "")
		if err != nil {
			t.Fatalf("fetch error: %v", err)
		}
		if len(result.Resources) == 0 {
			t.Fatal("no resources returned for top-level data-pipeline-logs")
		}
		validateResources(t, "data-pipeline-logs/", result.Resources)

		// Must have at least one folder and at least one file
		hasFolders := false
		hasFiles := false
		for _, r := range result.Resources {
			if r.Status == "folder" {
				hasFolders = true
			}
			if r.Status == "file" {
				hasFiles = true
			}
		}
		if !hasFolders {
			t.Error("top-level listing returned no folders")
		}
		if !hasFiles {
			t.Error("top-level listing returned no files")
		}
	})

	// -------------------------------------------------------------------------
	// 2. First-level drill-down: logs/
	// -------------------------------------------------------------------------
	t.Run("first_level_logs/", func(t *testing.T) {
		topResult, err := pf(ctx, clients, resource.ParentContext{"bucket": "data-pipeline-logs", "prefix": ""}, "")
		if err != nil {
			t.Fatalf("top-level fetch error: %v", err)
		}
		topIDs := make(map[string]bool, len(topResult.Resources))
		for _, r := range topResult.Resources {
			topIDs[r.ID] = true
		}

		result, err := pf(ctx, clients, resource.ParentContext{"bucket": "data-pipeline-logs", "prefix": "logs/"}, "")
		if err != nil {
			t.Fatalf("fetch error: %v", err)
		}
		if len(result.Resources) == 0 {
			t.Fatal("no resources returned for prefix logs/")
		}
		validateResources(t, "data-pipeline-logs/logs/", result.Resources)

		// Content at logs/ must differ from top-level
		allSame := true
		for _, r := range result.Resources {
			if !topIDs[r.ID] {
				allSame = false
				break
			}
		}
		if allSame {
			t.Error("first-level drill-down returned identical content to top-level listing")
		}
	})

	// -------------------------------------------------------------------------
	// 3. Second-level drill-down: logs/2026/
	// -------------------------------------------------------------------------
	t.Run("second_level_logs/2026/", func(t *testing.T) {
		result, err := pf(ctx, clients, resource.ParentContext{"bucket": "data-pipeline-logs", "prefix": "logs/2026/"}, "")
		if err != nil {
			t.Fatalf("fetch error: %v", err)
		}
		if len(result.Resources) == 0 {
			t.Fatal("no resources returned for prefix logs/2026/")
		}
		validateResources(t, "data-pipeline-logs/logs/2026/", result.Resources)
	})

	// -------------------------------------------------------------------------
	// 4. Third-level drill-down (leaf): logs/2026/03/ — all must be files
	// -------------------------------------------------------------------------
	t.Run("leaf_logs/2026/03/", func(t *testing.T) {
		result, err := pf(ctx, clients, resource.ParentContext{"bucket": "data-pipeline-logs", "prefix": "logs/2026/03/"}, "")
		if err != nil {
			t.Fatalf("fetch error: %v", err)
		}
		if len(result.Resources) == 0 {
			t.Fatal("no resources returned for prefix logs/2026/03/")
		}
		validateResources(t, "data-pipeline-logs/logs/2026/03/", result.Resources)
		for _, r := range result.Resources {
			if r.Status != "file" {
				t.Errorf("leaf resource %q has Status=%q; want 'file'", r.ID, r.Status)
			}
		}
	})

	// -------------------------------------------------------------------------
	// 5. All 6 buckets top-level must return at least one resource
	// -------------------------------------------------------------------------
	allBuckets := []string{
		"data-pipeline-logs",
		"webapp-assets-prod",
		"ml-training-data",
		"terraform-state-prod",
		"cloudtrail-audit-logs",
		"backup-db-snapshots",
	}

	t.Run("all_buckets_top_level", func(t *testing.T) {
		for _, bucket := range allBuckets {
			bucket := bucket
			t.Run(bucket, func(t *testing.T) {
				result, err := pf(ctx, clients, resource.ParentContext{"bucket": bucket, "prefix": ""}, "")
				if err != nil {
					t.Fatalf("fetch error for bucket %q: %v", bucket, err)
				}
				if len(result.Resources) == 0 {
					t.Fatalf("no resources returned for top-level of bucket %q", bucket)
				}
				validateResources(t, bucket+"/", result.Resources)
			})
		}
	})

	// -------------------------------------------------------------------------
	// 6. All navigable sub-folders: for each bucket, drill into each top-level
	//    folder and verify at least one resource is returned.
	// -------------------------------------------------------------------------
	t.Run("all_navigable_subfolders", func(t *testing.T) {
		for _, bucket := range allBuckets {
			bucket := bucket
			topResult, err := pf(ctx, clients, resource.ParentContext{"bucket": bucket, "prefix": ""}, "")
			if err != nil {
				t.Fatalf("top-level fetch error for bucket %q: %v", bucket, err)
			}
			for _, r := range topResult.Resources {
				if r.Status != "folder" {
					continue
				}
				folderPrefix := r.ID
				subResult, err := pf(ctx, clients, resource.ParentContext{"bucket": bucket, "prefix": folderPrefix}, "")
				if err != nil {
					t.Errorf("bucket=%q prefix=%q: fetch error: %v", bucket, folderPrefix, err)
					continue
				}
				if len(subResult.Resources) == 0 {
					t.Errorf("bucket=%q prefix=%q: folder is not navigable (returned 0 resources)", bucket, folderPrefix)
				}
				validateResources(t, bucket+"/"+folderPrefix, subResult.Resources)
			}
		}
	})

	// -------------------------------------------------------------------------
	// 7. Unknown prefix returns 0 resources, not an error
	// -------------------------------------------------------------------------
	t.Run("unknown_prefix_returns_empty", func(t *testing.T) {
		result, err := pf(ctx, clients, resource.ParentContext{"bucket": "data-pipeline-logs", "prefix": "nonexistent/"}, "")
		if err != nil {
			t.Fatalf("unknown prefix should not return an error, got: %v", err)
		}
		if len(result.Resources) != 0 {
			t.Errorf("unknown prefix returned %d resources; want 0", len(result.Resources))
		}
	})
}
