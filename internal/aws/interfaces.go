// Package aws defines narrow interfaces for each AWS service operation used by a9s.
// These interfaces enable dependency injection and testability.
package aws

import (
	"context"

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
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	"github.com/aws/aws-sdk-go-v2/service/codebuild"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	"github.com/aws/aws-sdk-go-v2/service/eks"
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

	"github.com/k2m30/a9s/v3/internal/resource"
)

// EC2DescribeInstancesAPI defines the interface for the EC2 DescribeInstances operation.
type EC2DescribeInstancesAPI interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
}

// EC2FetchInstancesAPI combines DescribeInstances and DescribeInstanceStatus,
// which are both required by FetchEC2InstancesPage (status enrichment uses the second).
// EC2DescribeInstanceStatusAPI is defined in the Wave 2 enrichment section below.
type EC2FetchInstancesAPI interface {
	EC2DescribeInstancesAPI
	EC2DescribeInstanceStatusAPI
}

// S3ListBucketsAPI defines the interface for the S3 ListBuckets operation.
type S3ListBucketsAPI interface {
	ListBuckets(ctx context.Context, params *s3.ListBucketsInput, optFns ...func(*s3.Options)) (*s3.ListBucketsOutput, error)
}

// S3ListObjectsV2API defines the interface for the S3 ListObjectsV2 operation.
type S3ListObjectsV2API interface {
	ListObjectsV2(ctx context.Context, params *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

// S3GetBucketLocationAPI defines the interface for the S3 GetBucketLocation operation.
type S3GetBucketLocationAPI interface {
	GetBucketLocation(ctx context.Context, params *s3.GetBucketLocationInput, optFns ...func(*s3.Options)) (*s3.GetBucketLocationOutput, error)
}

// S3GetBucketNotificationConfigurationAPI defines the interface for
// the S3 GetBucketNotificationConfiguration operation.
type S3GetBucketNotificationConfigurationAPI interface {
	GetBucketNotificationConfiguration(ctx context.Context, params *s3.GetBucketNotificationConfigurationInput, optFns ...func(*s3.Options)) (*s3.GetBucketNotificationConfigurationOutput, error)
}

// RDSDescribeDBInstancesAPI defines the interface for the RDS DescribeDBInstances operation.
type RDSDescribeDBInstancesAPI interface {
	DescribeDBInstances(ctx context.Context, params *rds.DescribeDBInstancesInput, optFns ...func(*rds.Options)) (*rds.DescribeDBInstancesOutput, error)
}

// ElastiCacheDescribeCacheClustersAPI defines the interface for the ElastiCache DescribeCacheClusters operation.
type ElastiCacheDescribeCacheClustersAPI interface {
	DescribeCacheClusters(ctx context.Context, params *elasticache.DescribeCacheClustersInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeCacheClustersOutput, error)
}

// DocDBDescribeDBClustersAPI defines the interface for the DocumentDB DescribeDBClusters operation.
type DocDBDescribeDBClustersAPI interface {
	DescribeDBClusters(ctx context.Context, params *docdb.DescribeDBClustersInput, optFns ...func(*docdb.Options)) (*docdb.DescribeDBClustersOutput, error)
}

// EKSListClustersAPI defines the interface for the EKS ListClusters operation.
type EKSListClustersAPI interface {
	ListClusters(ctx context.Context, params *eks.ListClustersInput, optFns ...func(*eks.Options)) (*eks.ListClustersOutput, error)
}

// EKSDescribeClusterAPI defines the interface for the EKS DescribeCluster operation.
type EKSDescribeClusterAPI interface {
	DescribeCluster(ctx context.Context, params *eks.DescribeClusterInput, optFns ...func(*eks.Options)) (*eks.DescribeClusterOutput, error)
}

// SecretsManagerListSecretsAPI defines the interface for the SecretsManager ListSecrets operation.
type SecretsManagerListSecretsAPI interface {
	ListSecrets(ctx context.Context, params *secretsmanager.ListSecretsInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.ListSecretsOutput, error)
}

// SecretsManagerGetSecretValueAPI defines the interface for the SecretsManager GetSecretValue operation.
type SecretsManagerGetSecretValueAPI interface {
	GetSecretValue(ctx context.Context, params *secretsmanager.GetSecretValueInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetSecretValueOutput, error)
}

// EC2DescribeVpcsAPI defines the interface for the EC2 DescribeVpcs operation.
type EC2DescribeVpcsAPI interface {
	DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
}

// EC2DescribeSecurityGroupsAPI defines the interface for the EC2 DescribeSecurityGroups operation.
type EC2DescribeSecurityGroupsAPI interface {
	DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
}

// EKSListNodegroupsAPI defines the interface for the EKS ListNodegroups operation.
type EKSListNodegroupsAPI interface {
	ListNodegroups(ctx context.Context, params *eks.ListNodegroupsInput, optFns ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error)
}

// EKSDescribeNodegroupAPI defines the interface for the EKS DescribeNodegroup operation.
type EKSDescribeNodegroupAPI interface {
	DescribeNodegroup(ctx context.Context, params *eks.DescribeNodegroupInput, optFns ...func(*eks.Options)) (*eks.DescribeNodegroupOutput, error)
}

// EC2DescribeSubnetsAPI defines the interface for the EC2 DescribeSubnets operation.
type EC2DescribeSubnetsAPI interface {
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
}

// EC2DescribeRouteTablesAPI defines the interface for the EC2 DescribeRouteTables operation.
type EC2DescribeRouteTablesAPI interface {
	DescribeRouteTables(ctx context.Context, params *ec2.DescribeRouteTablesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeRouteTablesOutput, error)
}

// EC2DescribeNatGatewaysAPI defines the interface for the EC2 DescribeNatGateways operation.
type EC2DescribeNatGatewaysAPI interface {
	DescribeNatGateways(ctx context.Context, params *ec2.DescribeNatGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNatGatewaysOutput, error)
}

// EC2DescribeInternetGatewaysAPI defines the interface for the EC2 DescribeInternetGateways operation.
type EC2DescribeInternetGatewaysAPI interface {
	DescribeInternetGateways(ctx context.Context, params *ec2.DescribeInternetGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInternetGatewaysOutput, error)
}

// LambdaListFunctionsAPI defines the interface for the Lambda ListFunctions operation.
type LambdaListFunctionsAPI interface {
	ListFunctions(ctx context.Context, params *lambda.ListFunctionsInput, optFns ...func(*lambda.Options)) (*lambda.ListFunctionsOutput, error)
}

// LambdaListEventSourceMappingsAPI defines the interface for the Lambda
// ListEventSourceMappings operation.
type LambdaListEventSourceMappingsAPI interface {
	ListEventSourceMappings(ctx context.Context, params *lambda.ListEventSourceMappingsInput, optFns ...func(*lambda.Options)) (*lambda.ListEventSourceMappingsOutput, error)
}

// CloudWatchDescribeAlarmsAPI defines the interface for the CloudWatch DescribeAlarms operation.
type CloudWatchDescribeAlarmsAPI interface {
	DescribeAlarms(ctx context.Context, params *cloudwatch.DescribeAlarmsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.DescribeAlarmsOutput, error)
}

// SNSListTopicsAPI defines the interface for the SNS ListTopics operation.
type SNSListTopicsAPI interface {
	ListTopics(ctx context.Context, params *sns.ListTopicsInput, optFns ...func(*sns.Options)) (*sns.ListTopicsOutput, error)
}

// SQSListQueuesAPI defines the interface for the SQS ListQueues operation.
type SQSListQueuesAPI interface {
	ListQueues(ctx context.Context, params *sqs.ListQueuesInput, optFns ...func(*sqs.Options)) (*sqs.ListQueuesOutput, error)
}

// SQSGetQueueAttributesAPI defines the interface for the SQS GetQueueAttributes operation.
type SQSGetQueueAttributesAPI interface {
	GetQueueAttributes(ctx context.Context, params *sqs.GetQueueAttributesInput, optFns ...func(*sqs.Options)) (*sqs.GetQueueAttributesOutput, error)
}

// ELBv2DescribeLoadBalancersAPI defines the interface for the ELBv2 DescribeLoadBalancers operation.
type ELBv2DescribeLoadBalancersAPI interface {
	DescribeLoadBalancers(ctx context.Context, params *elbv2.DescribeLoadBalancersInput, optFns ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancersOutput, error)
}

// ELBv2DescribeTargetGroupsAPI defines the interface for the ELBv2 DescribeTargetGroups operation.
type ELBv2DescribeTargetGroupsAPI interface {
	DescribeTargetGroups(ctx context.Context, params *elbv2.DescribeTargetGroupsInput, optFns ...func(*elbv2.Options)) (*elbv2.DescribeTargetGroupsOutput, error)
}

// ECSListClustersAPI defines the interface for the ECS ListClusters operation.
type ECSListClustersAPI interface {
	ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
}

// ECSDescribeClustersAPI defines the interface for the ECS DescribeClusters operation.
type ECSDescribeClustersAPI interface {
	DescribeClusters(ctx context.Context, params *ecs.DescribeClustersInput, optFns ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error)
}

// ECSListServicesAPI defines the interface for the ECS ListServices operation.
type ECSListServicesAPI interface {
	ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error)
}

// ECSDescribeServicesAPI defines the interface for the ECS DescribeServices operation.
type ECSDescribeServicesAPI interface {
	DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
}

// CFNDescribeStacksAPI defines the interface for the CloudFormation DescribeStacks operation.
type CFNDescribeStacksAPI interface {
	DescribeStacks(ctx context.Context, params *cloudformation.DescribeStacksInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStacksOutput, error)
}

// CFNDescribeStackEventsAPI defines the interface for the CloudFormation DescribeStackEvents operation.
type CFNDescribeStackEventsAPI interface {
	DescribeStackEvents(ctx context.Context, params *cloudformation.DescribeStackEventsInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStackEventsOutput, error)
}

// CFNListStackResourcesAPI defines the interface for the CloudFormation ListStackResources operation.
type CFNListStackResourcesAPI interface {
	ListStackResources(ctx context.Context, params *cloudformation.ListStackResourcesInput, optFns ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error)
}

// IAMListRolesAPI defines the interface for the IAM ListRoles operation.
type IAMListRolesAPI interface {
	ListRoles(ctx context.Context, params *iam.ListRolesInput, optFns ...func(*iam.Options)) (*iam.ListRolesOutput, error)
}

// CWLogsDescribeLogGroupsAPI defines the interface for the CloudWatchLogs DescribeLogGroups operation.
type CWLogsDescribeLogGroupsAPI interface {
	DescribeLogGroups(ctx context.Context, params *cloudwatchlogs.DescribeLogGroupsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
}

// SSMDescribeParametersAPI defines the interface for the SSM DescribeParameters operation.
type SSMDescribeParametersAPI interface {
	DescribeParameters(ctx context.Context, params *ssm.DescribeParametersInput, optFns ...func(*ssm.Options)) (*ssm.DescribeParametersOutput, error)
}

// SSMGetParameterAPI defines the interface for the SSM GetParameter operation.
type SSMGetParameterAPI interface {
	GetParameter(ctx context.Context, params *ssm.GetParameterInput, optFns ...func(*ssm.Options)) (*ssm.GetParameterOutput, error)
}

// DDBListTablesAPI defines the interface for the DynamoDB ListTables operation.
type DDBListTablesAPI interface {
	ListTables(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error)
}

// DDBDescribeTableAPI defines the interface for the DynamoDB DescribeTable operation.
type DDBDescribeTableAPI interface {
	DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
}

// DynamoDBDescribeContinuousBackupsAPI defines the interface for the DynamoDB DescribeContinuousBackups operation.
type DynamoDBDescribeContinuousBackupsAPI interface {
	DescribeContinuousBackups(ctx context.Context, params *dynamodb.DescribeContinuousBackupsInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeContinuousBackupsOutput, error)
}

// EC2DescribeAddressesAPI defines the interface for the EC2 DescribeAddresses operation.
type EC2DescribeAddressesAPI interface {
	DescribeAddresses(ctx context.Context, params *ec2.DescribeAddressesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error)
}

// ACMListCertificatesAPI defines the interface for the ACM ListCertificates operation.
type ACMListCertificatesAPI interface {
	ListCertificates(ctx context.Context, params *acm.ListCertificatesInput, optFns ...func(*acm.Options)) (*acm.ListCertificatesOutput, error)
}

// ACMDescribeCertificateAPI defines the interface for the ACM DescribeCertificate operation.
type ACMDescribeCertificateAPI interface {
	DescribeCertificate(ctx context.Context, params *acm.DescribeCertificateInput, optFns ...func(*acm.Options)) (*acm.DescribeCertificateOutput, error)
}

// ASGDescribeAutoScalingGroupsAPI defines the interface for the AutoScaling DescribeAutoScalingGroups operation.
type ASGDescribeAutoScalingGroupsAPI interface {
	DescribeAutoScalingGroups(ctx context.Context, params *autoscaling.DescribeAutoScalingGroupsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeAutoScalingGroupsOutput, error)
}

// ECSListTasksAPI defines the interface for the ECS ListTasks operation.
type ECSListTasksAPI interface {
	ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
}

// ECSDescribeTasksAPI defines the interface for the ECS DescribeTasks operation.
type ECSDescribeTasksAPI interface {
	DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
}

// IAMListPoliciesAPI defines the interface for the IAM ListPolicies operation.
type IAMListPoliciesAPI interface {
	ListPolicies(ctx context.Context, params *iam.ListPoliciesInput, optFns ...func(*iam.Options)) (*iam.ListPoliciesOutput, error)
}

// RDSDescribeDBSnapshotsAPI defines the interface for the RDS DescribeDBSnapshots operation.
type RDSDescribeDBSnapshotsAPI interface {
	DescribeDBSnapshots(ctx context.Context, params *rds.DescribeDBSnapshotsInput, optFns ...func(*rds.Options)) (*rds.DescribeDBSnapshotsOutput, error)
}

// RDSDescribeEventsAPI defines the interface for the RDS DescribeEvents operation.
type RDSDescribeEventsAPI interface {
	DescribeEvents(ctx context.Context, params *rds.DescribeEventsInput, optFns ...func(*rds.Options)) (*rds.DescribeEventsOutput, error)
}

// EC2DescribeTransitGatewaysAPI defines the interface for the EC2 DescribeTransitGateways operation.
type EC2DescribeTransitGatewaysAPI interface {
	DescribeTransitGateways(ctx context.Context, params *ec2.DescribeTransitGatewaysInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTransitGatewaysOutput, error)
}

// EC2DescribeVpcEndpointsAPI defines the interface for the EC2 DescribeVpcEndpoints operation.
type EC2DescribeVpcEndpointsAPI interface {
	DescribeVpcEndpoints(ctx context.Context, params *ec2.DescribeVpcEndpointsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVpcEndpointsOutput, error)
}

// EC2DescribeNetworkInterfacesAPI defines the interface for the EC2 DescribeNetworkInterfaces operation.
type EC2DescribeNetworkInterfacesAPI interface {
	DescribeNetworkInterfaces(ctx context.Context, params *ec2.DescribeNetworkInterfacesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeNetworkInterfacesOutput, error)
}

// SNSListSubscriptionsAPI defines the interface for the SNS ListSubscriptions operation.
type SNSListSubscriptionsAPI interface {
	ListSubscriptions(ctx context.Context, params *sns.ListSubscriptionsInput, optFns ...func(*sns.Options)) (*sns.ListSubscriptionsOutput, error)
}

// SNSListSubscriptionsByTopicAPI defines the interface for the SNS ListSubscriptionsByTopic operation.
type SNSListSubscriptionsByTopicAPI interface {
	ListSubscriptionsByTopic(ctx context.Context, params *sns.ListSubscriptionsByTopicInput, optFns ...func(*sns.Options)) (*sns.ListSubscriptionsByTopicOutput, error)
}

// IAMListUsersAPI defines the interface for the IAM ListUsers operation.
type IAMListUsersAPI interface {
	ListUsers(ctx context.Context, params *iam.ListUsersInput, optFns ...func(*iam.Options)) (*iam.ListUsersOutput, error)
}

// IAMListGroupsAPI defines the interface for the IAM ListGroups operation.
type IAMListGroupsAPI interface {
	ListGroups(ctx context.Context, params *iam.ListGroupsInput, optFns ...func(*iam.Options)) (*iam.ListGroupsOutput, error)
}

// DocDBDescribeDBClusterSnapshotsAPI defines the interface for the DocumentDB DescribeDBClusterSnapshots operation.
type DocDBDescribeDBClusterSnapshotsAPI interface {
	DescribeDBClusterSnapshots(ctx context.Context, params *docdb.DescribeDBClusterSnapshotsInput, optFns ...func(*docdb.Options)) (*docdb.DescribeDBClusterSnapshotsOutput, error)
}

// --- Batch 2a interfaces ---

// CloudFrontListDistributionsAPI defines the interface for the CloudFront ListDistributions operation.
type CloudFrontListDistributionsAPI interface {
	ListDistributions(ctx context.Context, params *cloudfront.ListDistributionsInput, optFns ...func(*cloudfront.Options)) (*cloudfront.ListDistributionsOutput, error)
}

// CloudFrontGetDistributionConfigAPI defines the interface for the CloudFront GetDistributionConfig operation.
// Used by Wave 2 enrichment to inspect per-distribution security configuration.
type CloudFrontGetDistributionConfigAPI interface {
	GetDistributionConfig(ctx context.Context, params *cloudfront.GetDistributionConfigInput, optFns ...func(*cloudfront.Options)) (*cloudfront.GetDistributionConfigOutput, error)
}

// Route53ListHostedZonesAPI defines the interface for the Route53 ListHostedZones operation.
type Route53ListHostedZonesAPI interface {
	ListHostedZones(ctx context.Context, params *route53.ListHostedZonesInput, optFns ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error)
}

// Route53ListResourceRecordSetsAPI defines the interface for the Route53 ListResourceRecordSets operation.
type Route53ListResourceRecordSetsAPI interface {
	ListResourceRecordSets(ctx context.Context, params *route53.ListResourceRecordSetsInput, optFns ...func(*route53.Options)) (*route53.ListResourceRecordSetsOutput, error)
}

// APIGatewayV2GetApisAPI defines the interface for the API Gateway V2 GetApis operation.
type APIGatewayV2GetApisAPI interface {
	GetApis(ctx context.Context, params *apigatewayv2.GetApisInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApisOutput, error)
}

// APIGatewayV2GetStagesAPI defines the interface for the API Gateway V2 GetStages operation.
// Used by Wave 2 enrichment to inspect stage-level configuration per API.
type APIGatewayV2GetStagesAPI interface {
	GetStages(ctx context.Context, params *apigatewayv2.GetStagesInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.GetStagesOutput, error)
}

// ECRDescribeRepositoriesAPI defines the interface for the ECR DescribeRepositories operation.
type ECRDescribeRepositoriesAPI interface {
	DescribeRepositories(ctx context.Context, params *ecr.DescribeRepositoriesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeRepositoriesOutput, error)
}

// ECRDescribeImagesAPI defines the interface for the ECR DescribeImages operation.
type ECRDescribeImagesAPI interface {
	DescribeImages(ctx context.Context, params *ecr.DescribeImagesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error)
}

// EFSDescribeFileSystemsAPI defines the interface for the EFS DescribeFileSystems operation.
type EFSDescribeFileSystemsAPI interface {
	DescribeFileSystems(ctx context.Context, params *efs.DescribeFileSystemsInput, optFns ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error)
}

// EFSDescribeMountTargetsAPI defines the interface for the EFS DescribeMountTargets operation.
type EFSDescribeMountTargetsAPI interface {
	DescribeMountTargets(ctx context.Context, params *efs.DescribeMountTargetsInput, optFns ...func(*efs.Options)) (*efs.DescribeMountTargetsOutput, error)
}

// EventBridgeListRulesAPI defines the interface for the EventBridge ListRules operation.
type EventBridgeListRulesAPI interface {
	ListRules(ctx context.Context, params *eventbridge.ListRulesInput, optFns ...func(*eventbridge.Options)) (*eventbridge.ListRulesOutput, error)
}

// EventBridgeListTargetsByRuleAPI defines the interface for the EventBridge ListTargetsByRule operation.
type EventBridgeListTargetsByRuleAPI interface {
	ListTargetsByRule(ctx context.Context, params *eventbridge.ListTargetsByRuleInput, optFns ...func(*eventbridge.Options)) (*eventbridge.ListTargetsByRuleOutput, error)
}

// SFNListStateMachinesAPI defines the interface for the SFN ListStateMachines operation.
type SFNListStateMachinesAPI interface {
	ListStateMachines(ctx context.Context, params *sfn.ListStateMachinesInput, optFns ...func(*sfn.Options)) (*sfn.ListStateMachinesOutput, error)
}

// SFNListExecutionsAPI defines the interface for the SFN ListExecutions operation.
type SFNListExecutionsAPI interface {
	ListExecutions(ctx context.Context, params *sfn.ListExecutionsInput, optFns ...func(*sfn.Options)) (*sfn.ListExecutionsOutput, error)
}

// SFNGetExecutionHistoryAPI defines the interface for the SFN GetExecutionHistory operation.
type SFNGetExecutionHistoryAPI interface {
	GetExecutionHistory(ctx context.Context, params *sfn.GetExecutionHistoryInput, optFns ...func(*sfn.Options)) (*sfn.GetExecutionHistoryOutput, error)
}

// CodePipelineListPipelinesAPI defines the interface for the CodePipeline ListPipelines operation.
type CodePipelineListPipelinesAPI interface {
	ListPipelines(ctx context.Context, params *codepipeline.ListPipelinesInput, optFns ...func(*codepipeline.Options)) (*codepipeline.ListPipelinesOutput, error)
}

// CodePipelineGetPipelineStateAPI defines the interface for the CodePipeline GetPipelineState operation.
type CodePipelineGetPipelineStateAPI interface {
	GetPipelineState(ctx context.Context, params *codepipeline.GetPipelineStateInput, optFns ...func(*codepipeline.Options)) (*codepipeline.GetPipelineStateOutput, error)
}

// --- Batch 2b interfaces ---

// KinesisListStreamsAPI defines the interface for the Kinesis ListStreams operation.
type KinesisListStreamsAPI interface {
	ListStreams(ctx context.Context, params *kinesis.ListStreamsInput, optFns ...func(*kinesis.Options)) (*kinesis.ListStreamsOutput, error)
}

// WAFv2ListWebACLsAPI defines the interface for the WAFv2 ListWebACLs operation.
type WAFv2ListWebACLsAPI interface {
	ListWebACLs(ctx context.Context, params *wafv2.ListWebACLsInput, optFns ...func(*wafv2.Options)) (*wafv2.ListWebACLsOutput, error)
}

// GlueGetJobsAPI defines the interface for the Glue GetJobs operation.
type GlueGetJobsAPI interface {
	GetJobs(ctx context.Context, params *glue.GetJobsInput, optFns ...func(*glue.Options)) (*glue.GetJobsOutput, error)
}

// GlueGetJobRunsAPI defines the interface for the Glue GetJobRuns operation.
type GlueGetJobRunsAPI interface {
	GetJobRuns(ctx context.Context, params *glue.GetJobRunsInput, optFns ...func(*glue.Options)) (*glue.GetJobRunsOutput, error)
}

// EBDescribeEnvironmentsAPI defines the interface for the Elastic Beanstalk DescribeEnvironments operation.
type EBDescribeEnvironmentsAPI interface {
	DescribeEnvironments(ctx context.Context, params *elasticbeanstalk.DescribeEnvironmentsInput, optFns ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentsOutput, error)
}

// SESv2ListEmailIdentitiesAPI defines the interface for the SES v2 ListEmailIdentities operation.
type SESv2ListEmailIdentitiesAPI interface {
	ListEmailIdentities(ctx context.Context, params *sesv2.ListEmailIdentitiesInput, optFns ...func(*sesv2.Options)) (*sesv2.ListEmailIdentitiesOutput, error)
}

// SESv2GetAccountAPI defines the interface for the SES v2 GetAccount operation.
type SESv2GetAccountAPI interface {
	GetAccount(ctx context.Context, params *sesv2.GetAccountInput, optFns ...func(*sesv2.Options)) (*sesv2.GetAccountOutput, error)
}

// RedshiftDescribeClustersAPI defines the interface for the Redshift DescribeClusters operation.
type RedshiftDescribeClustersAPI interface {
	DescribeClusters(ctx context.Context, params *redshift.DescribeClustersInput, optFns ...func(*redshift.Options)) (*redshift.DescribeClustersOutput, error)
}

// CloudTrailDescribeTrailsAPI defines the interface for the CloudTrail DescribeTrails operation.
type CloudTrailDescribeTrailsAPI interface {
	DescribeTrails(ctx context.Context, params *cloudtrail.DescribeTrailsInput, optFns ...func(*cloudtrail.Options)) (*cloudtrail.DescribeTrailsOutput, error)
	GetTrailStatus(ctx context.Context, params *cloudtrail.GetTrailStatusInput, optFns ...func(*cloudtrail.Options)) (*cloudtrail.GetTrailStatusOutput, error)
}

// AthenaListWorkGroupsAPI defines the interface for the Athena ListWorkGroups operation.
type AthenaListWorkGroupsAPI interface {
	ListWorkGroups(ctx context.Context, params *athena.ListWorkGroupsInput, optFns ...func(*athena.Options)) (*athena.ListWorkGroupsOutput, error)
}

// CodeArtifactListRepositoriesAPI defines the interface for the CodeArtifact ListRepositories operation.
type CodeArtifactListRepositoriesAPI interface {
	ListRepositories(ctx context.Context, params *codeartifact.ListRepositoriesInput, optFns ...func(*codeartifact.Options)) (*codeartifact.ListRepositoriesOutput, error)
}

// --- Batch 3 interfaces ---

// CodeBuildListProjectsAPI defines the interface for the CodeBuild ListProjects operation.
type CodeBuildListProjectsAPI interface {
	ListProjects(ctx context.Context, params *codebuild.ListProjectsInput, optFns ...func(*codebuild.Options)) (*codebuild.ListProjectsOutput, error)
}

// CodeBuildBatchGetProjectsAPI defines the interface for the CodeBuild BatchGetProjects operation.
type CodeBuildBatchGetProjectsAPI interface {
	BatchGetProjects(ctx context.Context, params *codebuild.BatchGetProjectsInput, optFns ...func(*codebuild.Options)) (*codebuild.BatchGetProjectsOutput, error)
}

// OpenSearchListDomainNamesAPI defines the interface for the OpenSearch ListDomainNames operation.
type OpenSearchListDomainNamesAPI interface {
	ListDomainNames(ctx context.Context, params *opensearch.ListDomainNamesInput, optFns ...func(*opensearch.Options)) (*opensearch.ListDomainNamesOutput, error)
}

// OpenSearchDescribeDomainsAPI defines the interface for the OpenSearch DescribeDomains operation.
type OpenSearchDescribeDomainsAPI interface {
	DescribeDomains(ctx context.Context, params *opensearch.DescribeDomainsInput, optFns ...func(*opensearch.Options)) (*opensearch.DescribeDomainsOutput, error)
}

// KMSListKeysAPI defines the interface for the KMS ListKeys operation.
type KMSListKeysAPI interface {
	ListKeys(ctx context.Context, params *kms.ListKeysInput, optFns ...func(*kms.Options)) (*kms.ListKeysOutput, error)
}

// KMSDescribeKeyAPI defines the interface for the KMS DescribeKey operation.
type KMSDescribeKeyAPI interface {
	DescribeKey(ctx context.Context, params *kms.DescribeKeyInput, optFns ...func(*kms.Options)) (*kms.DescribeKeyOutput, error)
}

// KMSListAliasesAPI defines the interface for the KMS ListAliases operation.
type KMSListAliasesAPI interface {
	ListAliases(ctx context.Context, params *kms.ListAliasesInput, optFns ...func(*kms.Options)) (*kms.ListAliasesOutput, error)
}

// KMSGetKeyRotationStatusAPI defines the interface for the KMS GetKeyRotationStatus operation.
type KMSGetKeyRotationStatusAPI interface {
	GetKeyRotationStatus(ctx context.Context, params *kms.GetKeyRotationStatusInput, optFns ...func(*kms.Options)) (*kms.GetKeyRotationStatusOutput, error)
}

// MSKListClustersV2API defines the interface for the Kafka ListClustersV2 operation.
type MSKListClustersV2API interface {
	ListClustersV2(ctx context.Context, params *kafka.ListClustersV2Input, optFns ...func(*kafka.Options)) (*kafka.ListClustersV2Output, error)
}

// KafkaDescribeClusterV2API defines the interface for the Kafka DescribeClusterV2 operation.
type KafkaDescribeClusterV2API interface {
	DescribeClusterV2(ctx context.Context, params *kafka.DescribeClusterV2Input, optFns ...func(*kafka.Options)) (*kafka.DescribeClusterV2Output, error)
}

// BackupListBackupPlansAPI defines the interface for the Backup ListBackupPlans operation.
type BackupListBackupPlansAPI interface {
	ListBackupPlans(ctx context.Context, params *backup.ListBackupPlansInput, optFns ...func(*backup.Options)) (*backup.ListBackupPlansOutput, error)
}

// BackupListBackupJobsAPI defines the interface for the Backup ListBackupJobs operation.
type BackupListBackupJobsAPI interface {
	ListBackupJobs(ctx context.Context, params *backup.ListBackupJobsInput, optFns ...func(*backup.Options)) (*backup.ListBackupJobsOutput, error)
}

// --- Child view interfaces ---

// CWLogsDescribeLogStreamsAPI defines the interface for the CloudWatchLogs DescribeLogStreams operation.
type CWLogsDescribeLogStreamsAPI interface {
	DescribeLogStreams(ctx context.Context, params *cloudwatchlogs.DescribeLogStreamsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeLogStreamsOutput, error)
}

// CWLogsGetLogEventsAPI defines the interface for the CloudWatchLogs GetLogEvents operation.
type CWLogsGetLogEventsAPI interface {
	GetLogEvents(ctx context.Context, params *cloudwatchlogs.GetLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error)
}

// ELBv2DescribeTargetHealthAPI defines the interface for the ELBv2 DescribeTargetHealth operation.
type ELBv2DescribeTargetHealthAPI interface {
	DescribeTargetHealth(ctx context.Context, params *elbv2.DescribeTargetHealthInput, optFns ...func(*elbv2.Options)) (*elbv2.DescribeTargetHealthOutput, error)
}

// ELBv2DescribeListenersAPI defines the interface for the ELBv2 DescribeListeners operation.
type ELBv2DescribeListenersAPI interface {
	DescribeListeners(ctx context.Context, params *elbv2.DescribeListenersInput, optFns ...func(*elbv2.Options)) (*elbv2.DescribeListenersOutput, error)
}

// CWLogsFilterLogEventsAPI defines the interface for the CloudWatchLogs FilterLogEvents operation.
type CWLogsFilterLogEventsAPI interface {
	FilterLogEvents(ctx context.Context, params *cloudwatchlogs.FilterLogEventsInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.FilterLogEventsOutput, error)
}

// ECSDescribeTaskDefinitionAPI defines the interface for the ECS DescribeTaskDefinition operation.
type ECSDescribeTaskDefinitionAPI interface {
	DescribeTaskDefinition(ctx context.Context, params *ecs.DescribeTaskDefinitionInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTaskDefinitionOutput, error)
}

// LambdaGetFunctionAPI defines the interface for the Lambda GetFunction operation.
type LambdaGetFunctionAPI interface {
	GetFunction(ctx context.Context, params *lambda.GetFunctionInput, optFns ...func(*lambda.Options)) (*lambda.GetFunctionOutput, error)
}

// ASGDescribeScalingActivitiesAPI defines the interface for the AutoScaling DescribeScalingActivities operation.
type ASGDescribeScalingActivitiesAPI interface {
	DescribeScalingActivities(ctx context.Context, params *autoscaling.DescribeScalingActivitiesInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeScalingActivitiesOutput, error)
}

// CloudWatchDescribeAlarmHistoryAPI defines the interface for the CloudWatch DescribeAlarmHistory operation.
type CloudWatchDescribeAlarmHistoryAPI interface {
	DescribeAlarmHistory(ctx context.Context, params *cloudwatch.DescribeAlarmHistoryInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.DescribeAlarmHistoryOutput, error)
}

// CodeBuildListBuildsForProjectAPI defines the interface for the CodeBuild ListBuildsForProject operation.
type CodeBuildListBuildsForProjectAPI interface {
	ListBuildsForProject(ctx context.Context, params *codebuild.ListBuildsForProjectInput, optFns ...func(*codebuild.Options)) (*codebuild.ListBuildsForProjectOutput, error)
}

// CodeBuildBatchGetBuildsAPI defines the interface for the CodeBuild BatchGetBuilds operation.
type CodeBuildBatchGetBuildsAPI interface {
	BatchGetBuilds(ctx context.Context, params *codebuild.BatchGetBuildsInput, optFns ...func(*codebuild.Options)) (*codebuild.BatchGetBuildsOutput, error)
}

// IAMListAttachedRolePoliciesAPI defines the interface for the IAM ListAttachedRolePolicies operation.
type IAMListAttachedRolePoliciesAPI interface {
	ListAttachedRolePolicies(ctx context.Context, params *iam.ListAttachedRolePoliciesInput, optFns ...func(*iam.Options)) (*iam.ListAttachedRolePoliciesOutput, error)
}

// IAMListRolePoliciesAPI defines the interface for the IAM ListRolePolicies operation.
type IAMListRolePoliciesAPI interface {
	ListRolePolicies(ctx context.Context, params *iam.ListRolePoliciesInput, optFns ...func(*iam.Options)) (*iam.ListRolePoliciesOutput, error)
}

// IAMGetGroupAPI defines the interface for the IAM GetGroup operation.
type IAMGetGroupAPI interface {
	GetGroup(ctx context.Context, params *iam.GetGroupInput, optFns ...func(*iam.Options)) (*iam.GetGroupOutput, error)
}

// IAMListGroupPoliciesAPI defines the interface for the IAM ListGroupPolicies operation.
type IAMListGroupPoliciesAPI interface {
	ListGroupPolicies(ctx context.Context, params *iam.ListGroupPoliciesInput, optFns ...func(*iam.Options)) (*iam.ListGroupPoliciesOutput, error)
}

// IAMGetPolicyAPI defines the interface for the IAM GetPolicy operation.
type IAMGetPolicyAPI interface {
	GetPolicy(ctx context.Context, params *iam.GetPolicyInput, optFns ...func(*iam.Options)) (*iam.GetPolicyOutput, error)
}

// IAMGetPolicyVersionAPI defines the interface for the IAM GetPolicyVersion operation.
type IAMGetPolicyVersionAPI interface {
	GetPolicyVersion(ctx context.Context, params *iam.GetPolicyVersionInput, optFns ...func(*iam.Options)) (*iam.GetPolicyVersionOutput, error)
}

// IAMGetRolePolicyAPI defines the interface for the IAM GetRolePolicy operation.
type IAMGetRolePolicyAPI interface {
	GetRolePolicy(ctx context.Context, params *iam.GetRolePolicyInput, optFns ...func(*iam.Options)) (*iam.GetRolePolicyOutput, error)
}

// ELBv2DescribeRulesAPI defines the interface for the ELBv2 DescribeRules operation.
type ELBv2DescribeRulesAPI interface {
	DescribeRules(ctx context.Context, params *elbv2.DescribeRulesInput, optFns ...func(*elbv2.Options)) (*elbv2.DescribeRulesOutput, error)
}

// --- Identity interfaces ---

// STSGetCallerIdentityAPI defines the interface for the STS GetCallerIdentity operation.
type STSGetCallerIdentityAPI interface {
	GetCallerIdentity(ctx context.Context, params *sts.GetCallerIdentityInput, optFns ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

// IAMListAccountAliasesAPI defines the interface for the IAM ListAccountAliases operation.
type IAMListAccountAliasesAPI interface {
	ListAccountAliases(ctx context.Context, params *iam.ListAccountAliasesInput, optFns ...func(*iam.Options)) (*iam.ListAccountAliasesOutput, error)
}

// IAMListAttachedUserPoliciesAPI defines the interface for the IAM ListAttachedUserPolicies operation.
type IAMListAttachedUserPoliciesAPI interface {
	ListAttachedUserPolicies(ctx context.Context, params *iam.ListAttachedUserPoliciesInput, optFns ...func(*iam.Options)) (*iam.ListAttachedUserPoliciesOutput, error)
}

// IAMListAttachedGroupPoliciesAPI defines the interface for the IAM ListAttachedGroupPolicies operation.
type IAMListAttachedGroupPoliciesAPI interface {
	ListAttachedGroupPolicies(ctx context.Context, params *iam.ListAttachedGroupPoliciesInput, optFns ...func(*iam.Options)) (*iam.ListAttachedGroupPoliciesOutput, error)
}

// IAMListGroupsForUserAPI defines the interface for the IAM ListGroupsForUser operation.
type IAMListGroupsForUserAPI interface {
	ListGroupsForUser(ctx context.Context, params *iam.ListGroupsForUserInput, optFns ...func(*iam.Options)) (*iam.ListGroupsForUserOutput, error)
}

// IAMListEntitiesForPolicyAPI defines the interface for the IAM ListEntitiesForPolicy operation.
type IAMListEntitiesForPolicyAPI interface {
	ListEntitiesForPolicy(ctx context.Context, params *iam.ListEntitiesForPolicyInput, optFns ...func(*iam.Options)) (*iam.ListEntitiesForPolicyOutput, error)
}

// EC2DescribeVolumesAPI defines the interface for the EC2 DescribeVolumes operation.
type EC2DescribeVolumesAPI interface {
	DescribeVolumes(ctx context.Context, params *ec2.DescribeVolumesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error)
}

// EC2DescribeSnapshotsAPI defines the interface for the EC2 DescribeSnapshots operation.
type EC2DescribeSnapshotsAPI interface {
	DescribeSnapshots(ctx context.Context, params *ec2.DescribeSnapshotsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeSnapshotsOutput, error)
}

// EC2DescribeImagesAPI defines the interface for the EC2 DescribeImages operation.
type EC2DescribeImagesAPI interface {
	DescribeImages(ctx context.Context, params *ec2.DescribeImagesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeImagesOutput, error)
}

// CloudTrailLookupEventsAPI defines the interface for the CloudTrail LookupEvents operation.
type CloudTrailLookupEventsAPI interface {
	LookupEvents(ctx context.Context, params *cloudtrail.LookupEventsInput, optFns ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error)
}

// EC2DescribeTransitGatewayAttachmentsAPI defines the interface for the EC2
// DescribeTransitGatewayAttachments operation.
type EC2DescribeTransitGatewayAttachmentsAPI interface {
	DescribeTransitGatewayAttachments(ctx context.Context, params *ec2.DescribeTransitGatewayAttachmentsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayAttachmentsOutput, error)
}

// WAFv2ListResourcesForWebACLAPI defines the interface for the WAFv2
// ListResourcesForWebACL operation.
type WAFv2ListResourcesForWebACLAPI interface {
	ListResourcesForWebACL(ctx context.Context, params *wafv2.ListResourcesForWebACLInput, optFns ...func(*wafv2.Options)) (*wafv2.ListResourcesForWebACLOutput, error)
}

// EC2API is the aggregate interface covering all 16 EC2 operations used by a9s fetchers.
// *ec2.Client structurally satisfies this interface; the EC2 fake must implement all 16 methods.
type EC2API interface {
	EC2DescribeInstancesAPI
	EC2DescribeVpcsAPI
	EC2DescribeSecurityGroupsAPI
	EC2DescribeSubnetsAPI
	EC2DescribeRouteTablesAPI
	EC2DescribeNatGatewaysAPI
	EC2DescribeInternetGatewaysAPI
	EC2DescribeAddressesAPI
	EC2DescribeTransitGatewaysAPI
	EC2DescribeTransitGatewayAttachmentsAPI
	EC2DescribeVpcEndpointsAPI
	EC2DescribeNetworkInterfacesAPI
	EC2DescribeVolumesAPI
	EC2DescribeSnapshotsAPI
	EC2DescribeImagesAPI
	EC2DescribeInstanceStatusAPI  // Wave 2 enrichment
	EC2DescribeVolumeStatusAPI    // Wave 2 enrichment
	EC2DescribeFlowLogsAPI        // Wave 2 enrichment
}

// S3API is the aggregate interface covering all S3 operations used by a9s fetchers.
// *s3.Client structurally satisfies this interface.
type S3API interface {
	S3ListBucketsAPI
	S3ListObjectsV2API
	S3GetBucketNotificationConfigurationAPI
	S3GetPublicAccessBlockAPI // Wave 2 enrichment
}

// CloudTrailAPI is the aggregate interface covering all CloudTrail operations used by a9s fetchers.
// *cloudtrail.Client structurally satisfies this interface.
type CloudTrailAPI interface {
	CloudTrailDescribeTrailsAPI
	CloudTrailLookupEventsAPI
}

// RDSAPI is the aggregate interface covering all RDS operations used by a9s fetchers.
// *rds.Client structurally satisfies this interface.
type RDSAPI interface {
	RDSDescribeDBInstancesAPI
	RDSDescribeDBSnapshotsAPI
	RDSDescribeEventsAPI
	RDSDescribePendingMaintenanceAPI // Wave 2 enrichment
}

// ElastiCacheAPI is the aggregate interface covering all ElastiCache operations used by a9s fetchers.
// *elasticache.Client structurally satisfies this interface.
type ElastiCacheAPI interface {
	ElastiCacheDescribeCacheClustersAPI
}

// DynamoDBAPI is the aggregate interface covering all DynamoDB operations used by a9s fetchers.
// *dynamodb.Client structurally satisfies this interface.
type DynamoDBAPI interface {
	DDBListTablesAPI
	DDBDescribeTableAPI
	DynamoDBDescribeContinuousBackupsAPI
}

// DocDBAPI is the aggregate interface covering all DocumentDB operations used by a9s fetchers.
// *docdb.Client structurally satisfies this interface.
type DocDBAPI interface {
	DocDBDescribeDBClustersAPI
	DocDBDescribeDBClusterSnapshotsAPI
}

// LambdaAPI is the aggregate interface covering all Lambda operations used by a9s fetchers.
// *lambda.Client structurally satisfies this interface.
type LambdaAPI interface {
	LambdaListFunctionsAPI
	LambdaListEventSourceMappingsAPI
	LambdaGetFunctionAPI
}

// ECSAPI is the aggregate interface covering all ECS operations used by a9s fetchers.
// *ecs.Client structurally satisfies this interface.
type ECSAPI interface {
	ECSListClustersAPI
	ECSDescribeClustersAPI
	ECSListServicesAPI
	ECSDescribeServicesAPI
	ECSListTasksAPI
	ECSDescribeTasksAPI
	ECSDescribeTaskDefinitionAPI
}

// EKSAPI is the aggregate interface covering all EKS operations used by a9s fetchers.
// *eks.Client structurally satisfies this interface.
type EKSAPI interface {
	EKSListClustersAPI
	EKSDescribeClusterAPI
	EKSListNodegroupsAPI
	EKSDescribeNodegroupAPI
}

// ASGAPI is the aggregate interface covering all AutoScaling operations used by a9s fetchers.
// *autoscaling.Client structurally satisfies this interface.
type ASGAPI interface {
	ASGDescribeAutoScalingGroupsAPI
	ASGDescribeScalingActivitiesAPI
}

// ElasticBeanstalkAPI is the aggregate interface covering all ElasticBeanstalk operations used by a9s fetchers.
// *elasticbeanstalk.Client structurally satisfies this interface.
type ElasticBeanstalkAPI interface {
	EBDescribeEnvironmentsAPI
	ElasticBeanstalkDescribeEnvironmentHealthAPI // Wave 2 enrichment
}

// ELBv2API is the aggregate interface covering all ELBv2 operations used by a9s fetchers.
// *elbv2.Client structurally satisfies this interface.
type ELBv2API interface {
	ELBv2DescribeLoadBalancersAPI
	ELBv2DescribeTargetGroupsAPI
	ELBv2DescribeTargetHealthAPI
	ELBv2DescribeListenersAPI
	ELBv2DescribeRulesAPI
	ELBv2DescribeLoadBalancerAttributesAPI // Wave 2 enrichment
}

// IAMAPI is the aggregate interface covering all IAM operations used by a9s fetchers.
// *iam.Client structurally satisfies this interface.
type IAMAPI interface {
	IAMListRolesAPI
	IAMListPoliciesAPI
	IAMListUsersAPI
	IAMListGroupsAPI
	IAMListAttachedRolePoliciesAPI
	IAMListRolePoliciesAPI
	IAMListAttachedUserPoliciesAPI
	IAMListAttachedGroupPoliciesAPI
	IAMListGroupsForUserAPI
	IAMListEntitiesForPolicyAPI
	IAMListAccountAliasesAPI
	IAMGetGroupAPI
	IAMListGroupPoliciesAPI
	IAMGetPolicyAPI
	IAMGetPolicyVersionAPI
	IAMGetRolePolicyAPI
}

// WAFv2API is the aggregate interface covering all WAFv2 operations used by a9s fetchers.
// *wafv2.Client structurally satisfies this interface.
type WAFv2API interface {
	WAFv2ListWebACLsAPI
	WAFv2ListResourcesForWebACLAPI
}

// SecretsManagerAPI is the aggregate interface covering all SecretsManager operations used by a9s fetchers.
// *secretsmanager.Client structurally satisfies this interface.
type SecretsManagerAPI interface {
	SecretsManagerListSecretsAPI
	SecretsManagerGetSecretValueAPI
}

// SSMAPI is the aggregate interface covering all SSM operations used by a9s fetchers.
// *ssm.Client structurally satisfies this interface.
type SSMAPI interface {
	SSMDescribeParametersAPI
	SSMGetParameterAPI
}

// KMSAPI is the aggregate interface covering all KMS operations used by a9s fetchers.
// *kms.Client structurally satisfies this interface.
type KMSAPI interface {
	KMSListKeysAPI
	KMSDescribeKeyAPI
	KMSListAliasesAPI
	KMSGetKeyRotationStatusAPI
}

// Route53API is the aggregate interface covering all Route53 operations used by a9s fetchers.
// *route53.Client structurally satisfies this interface.
type Route53API interface {
	Route53ListHostedZonesAPI
	Route53ListResourceRecordSetsAPI
}

// CloudFrontAPI is the aggregate interface covering all CloudFront operations used by a9s fetchers.
// *cloudfront.Client structurally satisfies this interface.
type CloudFrontAPI interface {
	CloudFrontListDistributionsAPI
	CloudFrontGetDistributionConfigAPI // Wave 2 enrichment
}

// ACMAPI is the aggregate interface covering all ACM operations used by a9s fetchers.
// *acm.Client structurally satisfies this interface.
type ACMAPI interface {
	ACMListCertificatesAPI
	ACMDescribeCertificateAPI
}

// APIGatewayV2API is the aggregate interface covering all APIGatewayV2 operations used by a9s fetchers.
// *apigatewayv2.Client structurally satisfies this interface.
type APIGatewayV2API interface {
	APIGatewayV2GetApisAPI
	APIGatewayV2GetStagesAPI // Wave 2 enrichment
}

// CFNAPI is the aggregate interface covering all CloudFormation operations used by a9s fetchers.
// *cloudformation.Client structurally satisfies this interface.
type CFNAPI interface {
	CFNDescribeStacksAPI
	CFNDescribeStackEventsAPI
	CFNListStackResourcesAPI
}

// CodeBuildAPI is the aggregate interface covering all CodeBuild operations used by a9s fetchers.
// *codebuild.Client structurally satisfies this interface.
type CodeBuildAPI interface {
	CodeBuildListProjectsAPI
	CodeBuildBatchGetProjectsAPI
	CodeBuildListBuildsForProjectAPI
	CodeBuildBatchGetBuildsAPI
}

// CodePipelineAPI is the aggregate interface covering all CodePipeline operations used by a9s fetchers.
// *codepipeline.Client structurally satisfies this interface.
type CodePipelineAPI interface {
	CodePipelineListPipelinesAPI
	CodePipelineGetPipelineStateAPI
}

// ECRAPI is the aggregate interface covering all ECR operations used by a9s fetchers.
// *ecr.Client structurally satisfies this interface.
type ECRAPI interface {
	ECRDescribeRepositoriesAPI
	ECRDescribeImagesAPI
}

// CodeArtifactAPI is the aggregate interface covering all CodeArtifact operations used by a9s fetchers.
// *codeartifact.Client structurally satisfies this interface.
type CodeArtifactAPI interface {
	CodeArtifactListRepositoriesAPI
}

// CloudWatchAPI is the aggregate interface covering all CloudWatch operations used by a9s fetchers.
// *cloudwatch.Client structurally satisfies this interface.
type CloudWatchAPI interface {
	CloudWatchDescribeAlarmsAPI
	CloudWatchDescribeAlarmHistoryAPI
}

// CWLogsAPI is the aggregate interface covering all CloudWatchLogs operations used by a9s fetchers.
// *cloudwatchlogs.Client structurally satisfies this interface.
type CWLogsAPI interface {
	CWLogsDescribeLogGroupsAPI
	CWLogsDescribeLogStreamsAPI
	CWLogsGetLogEventsAPI
	CWLogsFilterLogEventsAPI
}

// SQSAPI is the aggregate interface covering SQS operations used by a9s enrichers.
// Fetchers that need ListQueues perform a runtime type assertion to SQSListQueuesAPI.
// *sqs.Client structurally satisfies this interface.
type SQSAPI interface {
	SQSGetQueueAttributesAPI
}

// SNSAPI is the aggregate interface covering SNS operations used by a9s enrichers.
// Fetchers that need ListTopics or ListSubscriptions perform runtime type assertions.
// *sns.Client structurally satisfies this interface.
type SNSAPI interface {
	SNSListSubscriptionsByTopicAPI
}

// EventBridgeAPI is the aggregate interface covering all EventBridge operations used by a9s fetchers.
// *eventbridge.Client structurally satisfies this interface.
type EventBridgeAPI interface {
	EventBridgeListRulesAPI
	EventBridgeListTargetsByRuleAPI
}

// KinesisAPI is the aggregate interface covering all Kinesis operations used by a9s fetchers.
// *kinesis.Client structurally satisfies this interface.
type KinesisAPI interface {
	KinesisListStreamsAPI
}

// SFNAPI is the aggregate interface covering all SFN operations used by a9s fetchers.
// *sfn.Client structurally satisfies this interface.
type SFNAPI interface {
	SFNListStateMachinesAPI
	SFNListExecutionsAPI
	SFNGetExecutionHistoryAPI
}

// MSKAPI is the aggregate interface covering all MSK operations used by a9s fetchers.
// *kafka.Client structurally satisfies this interface.
type MSKAPI interface {
	MSKListClustersV2API
	KafkaDescribeClusterV2API
}

// GlueAPI is the aggregate interface covering all Glue operations used by a9s fetchers.
// *glue.Client structurally satisfies this interface.
type GlueAPI interface {
	GlueGetJobsAPI
	GlueGetJobRunsAPI
}

// AthenaAPI is the aggregate interface covering all Athena operations used by a9s fetchers.
// *athena.Client structurally satisfies this interface.
type AthenaAPI interface {
	AthenaListWorkGroupsAPI
}

// OpenSearchAPI is the aggregate interface covering all OpenSearch operations used by a9s fetchers.
// *opensearch.Client structurally satisfies this interface.
type OpenSearchAPI interface {
	OpenSearchListDomainNamesAPI
	OpenSearchDescribeDomainsAPI
}

// RedshiftAPI is the aggregate interface covering all Redshift operations used by a9s fetchers.
// *redshift.Client structurally satisfies this interface.
type RedshiftAPI interface {
	RedshiftDescribeClustersAPI
}

// BackupAPI is the aggregate interface covering all Backup operations used by a9s fetchers.
// *backup.Client structurally satisfies this interface.
type BackupAPI interface {
	BackupListBackupPlansAPI
	BackupListBackupJobsAPI
}

// SESv2API is the aggregate interface covering all SESv2 operations used by a9s fetchers.
// *sesv2.Client structurally satisfies this interface.
type SESv2API interface {
	SESv2ListEmailIdentitiesAPI
	SESv2GetAccountAPI
}

// EFSAPI is the aggregate interface covering all EFS operations used by a9s fetchers.
// *efs.Client structurally satisfies this interface.
type EFSAPI interface {
	EFSDescribeFileSystemsAPI
	EFSDescribeMountTargetsAPI
}

// ElasticBeanstalkDescribeEnvironmentHealthAPI defines the interface for the
// Elastic Beanstalk DescribeEnvironmentHealth operation.
// Used by EnrichEBEnvironmentHealth (Wave 2 enrichment).
type ElasticBeanstalkDescribeEnvironmentHealthAPI interface {
	DescribeEnvironmentHealth(ctx context.Context, params *elasticbeanstalk.DescribeEnvironmentHealthInput, optFns ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentHealthOutput, error)
}

// ELBv2DescribeLoadBalancerAttributesAPI defines the interface for the ELBv2
// DescribeLoadBalancerAttributes operation.
// Used by EnrichELBAttributes (Wave 2 enrichment).
type ELBv2DescribeLoadBalancerAttributesAPI interface {
	DescribeLoadBalancerAttributes(ctx context.Context, params *elbv2.DescribeLoadBalancerAttributesInput, optFns ...func(*elbv2.Options)) (*elbv2.DescribeLoadBalancerAttributesOutput, error)
}

// --- Wave 2 enrichment interfaces (#196) ---

// EC2DescribeInstanceStatusAPI defines the interface for the EC2 DescribeInstanceStatus operation.
type EC2DescribeInstanceStatusAPI interface {
	DescribeInstanceStatus(ctx context.Context, params *ec2.DescribeInstanceStatusInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceStatusOutput, error)
}

// EC2DescribeVolumeStatusAPI defines the interface for the EC2 DescribeVolumeStatus operation.
type EC2DescribeVolumeStatusAPI interface {
	DescribeVolumeStatus(ctx context.Context, params *ec2.DescribeVolumeStatusInput, optFns ...func(*ec2.Options)) (*ec2.DescribeVolumeStatusOutput, error)
}

// EC2DescribeFlowLogsAPI defines the interface for the EC2 DescribeFlowLogs operation.
// Used by EnrichVPCFlowLogs to check whether flow logs are active for each VPC.
type EC2DescribeFlowLogsAPI interface {
	DescribeFlowLogs(ctx context.Context, params *ec2.DescribeFlowLogsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeFlowLogsOutput, error)
}

// RDSDescribePendingMaintenanceAPI defines the interface for the RDS DescribePendingMaintenanceActions operation.
type RDSDescribePendingMaintenanceAPI interface {
	DescribePendingMaintenanceActions(ctx context.Context, params *rds.DescribePendingMaintenanceActionsInput, optFns ...func(*rds.Options)) (*rds.DescribePendingMaintenanceActionsOutput, error)
}

// S3GetPublicAccessBlockAPI defines the interface for the S3 GetPublicAccessBlock operation.
// Used by EnrichS3PublicAccessBlock to check per-bucket PAB configuration.
type S3GetPublicAccessBlockAPI interface {
	GetPublicAccessBlock(ctx context.Context, params *s3.GetPublicAccessBlockInput, optFns ...func(*s3.Options)) (*s3.GetPublicAccessBlockOutput, error)
}

// EnricherResult is the typed return value of a Wave 2 enricher.
//
//   - IssueCount is the number of resources the enricher classifies as issue-worthy
//     for the menu badge. Severity "!" findings contribute; severity "~" findings
//     (informational) do NOT contribute to IssueCount.
//   - Truncated is true when the enricher only inspected a subset (e.g., capped at
//     EnrichmentCap) so the count is a lower bound.
//   - Findings is a map from resource.Resource.ID → EnrichmentFinding for every
//     affected resource the enricher observed. For account-wide enrichers (RDS,
//     EC2 status checks, EBS), Findings may contain entries for resources that
//     are NOT in the input `resources` slice — banner derivation uses this
//     information. Enrichers that receive API identifiers in a different form
//     (e.g., ARNs) MUST normalize to Resource.ID; if no match can be determined,
//     the affected resource is skipped silently.
//     MAY be empty when no issues are found. MUST NOT be nil on success — use
//     make(map[string]resource.EnrichmentFinding) for the empty case.
type EnricherResult struct {
	IssueCount int
	Truncated  bool
	Findings   map[string]resource.EnrichmentFinding
}

// EnricherFunc is a pluggable function that makes additional API calls for a
// resource type and returns a typed EnricherResult. The resources slice contains
// retained first-page resources from Wave 1 probes.
type EnricherFunc func(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error)
