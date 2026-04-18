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

// ElastiCacheDescribeReplicationGroupsAPI defines the interface for the
// ElastiCache DescribeReplicationGroups operation. Used by redis→kms (kms key)
// and redis→secrets (AUTH token / user group ARN).
type ElastiCacheDescribeReplicationGroupsAPI interface {
	DescribeReplicationGroups(ctx context.Context, params *elasticache.DescribeReplicationGroupsInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeReplicationGroupsOutput, error)
}

// ElastiCacheDescribeCacheSubnetGroupsAPI defines the interface for the
// ElastiCache DescribeCacheSubnetGroups operation. Used by redis→subnet/vpc.
type ElastiCacheDescribeCacheSubnetGroupsAPI interface {
	DescribeCacheSubnetGroups(ctx context.Context, params *elasticache.DescribeCacheSubnetGroupsInput, optFns ...func(*elasticache.Options)) (*elasticache.DescribeCacheSubnetGroupsOutput, error)
}

// ElastiCacheListTagsForResourceAPI defines the interface for the
// ElastiCache ListTagsForResource operation. Used by redis→cfn for
// extracting the aws:cloudformation:stack-name tag.
type ElastiCacheListTagsForResourceAPI interface {
	ListTagsForResource(ctx context.Context, params *elasticache.ListTagsForResourceInput, optFns ...func(*elasticache.Options)) (*elasticache.ListTagsForResourceOutput, error)
}

// RDSDescribeDBSubnetGroupsAPI defines the interface for the RDS
// DescribeDBSubnetGroups operation. Used by dbi→eni path for VPC/subnet
// resolution when the subnet group is needed.
type RDSDescribeDBSubnetGroupsAPI interface {
	DescribeDBSubnetGroups(ctx context.Context, params *rds.DescribeDBSubnetGroupsInput, optFns ...func(*rds.Options)) (*rds.DescribeDBSubnetGroupsOutput, error)
}

// DocDBDescribeDBSubnetGroupsAPI defines the interface for the DocumentDB
// DescribeDBSubnetGroups operation. Used by dbc→subnet/vpc.
type DocDBDescribeDBSubnetGroupsAPI interface {
	DescribeDBSubnetGroups(ctx context.Context, params *docdb.DescribeDBSubnetGroupsInput, optFns ...func(*docdb.Options)) (*docdb.DescribeDBSubnetGroupsOutput, error)
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

// SecretsManagerGetResourcePolicyAPI defines the interface for the
// SecretsManager GetResourcePolicy operation. Used by secrets→role to read
// the secret's resource policy and extract allowed principals (role ARNs).
type SecretsManagerGetResourcePolicyAPI interface {
	GetResourcePolicy(ctx context.Context, params *secretsmanager.GetResourcePolicyInput, optFns ...func(*secretsmanager.Options)) (*secretsmanager.GetResourcePolicyOutput, error)
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

// LambdaListTagsAPI defines the interface for the Lambda ListTags operation.
type LambdaListTagsAPI interface {
	ListTags(ctx context.Context, params *lambda.ListTagsInput, optFns ...func(*lambda.Options)) (*lambda.ListTagsOutput, error)
}

// CloudWatchDescribeAlarmsAPI defines the interface for the CloudWatch DescribeAlarms operation.
type CloudWatchDescribeAlarmsAPI interface {
	DescribeAlarms(ctx context.Context, params *cloudwatch.DescribeAlarmsInput, optFns ...func(*cloudwatch.Options)) (*cloudwatch.DescribeAlarmsOutput, error)
}

// SNSGetTopicAttributesAPI defines the interface for the SNS GetTopicAttributes
// operation. Used by sns→kms (KmsMasterKeyId) and sns→role (Policy document).
type SNSGetTopicAttributesAPI interface {
	GetTopicAttributes(ctx context.Context, params *sns.GetTopicAttributesInput, optFns ...func(*sns.Options)) (*sns.GetTopicAttributesOutput, error)
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

// IAMGetRoleAPI defines the interface for the IAM GetRole operation.
type IAMGetRoleAPI interface {
	GetRole(ctx context.Context, params *iam.GetRoleInput, optFns ...func(*iam.Options)) (*iam.GetRoleOutput, error)
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

// SSMDescribeInstanceInformationAPI defines the interface for the SSM DescribeInstanceInformation operation.
type SSMDescribeInstanceInformationAPI interface {
	DescribeInstanceInformation(ctx context.Context, params *ssm.DescribeInstanceInformationInput, optFns ...func(*ssm.Options)) (*ssm.DescribeInstanceInformationOutput, error)
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

// DynamoDBDescribeKinesisStreamingDestinationAPI defines the interface for the DynamoDB DescribeKinesisStreamingDestination operation.
type DynamoDBDescribeKinesisStreamingDestinationAPI interface {
	DescribeKinesisStreamingDestination(ctx context.Context, params *dynamodb.DescribeKinesisStreamingDestinationInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeKinesisStreamingDestinationOutput, error)
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

// IAMGetLoginProfileAPI defines the interface for the IAM GetLoginProfile operation.
// Used by Wave 2 EnrichIAMUserMFA to detect console users without MFA (CIS IAM.5).
type IAMGetLoginProfileAPI interface {
	GetLoginProfile(ctx context.Context, params *iam.GetLoginProfileInput, optFns ...func(*iam.Options)) (*iam.GetLoginProfileOutput, error)
}

// IAMListMFADevicesAPI defines the interface for the IAM ListMFADevices operation.
// Used by Wave 2 EnrichIAMUserMFA to detect console users without MFA (CIS IAM.5).
type IAMListMFADevicesAPI interface {
	ListMFADevices(ctx context.Context, params *iam.ListMFADevicesInput, optFns ...func(*iam.Options)) (*iam.ListMFADevicesOutput, error)
}

// IAMListAccessKeysAPI defines the interface for the IAM ListAccessKeys operation.
// Used by Wave 2 EnrichIAMUserMFA to detect stale access keys (>90d rotation).
type IAMListAccessKeysAPI interface {
	ListAccessKeys(ctx context.Context, params *iam.ListAccessKeysInput, optFns ...func(*iam.Options)) (*iam.ListAccessKeysOutput, error)
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

// Route53GetHostedZoneAPI defines the interface for the Route53 GetHostedZone operation.
// Used by Wave 2 enrichment to retrieve VPC associations for private hosted zones.
type Route53GetHostedZoneAPI interface {
	GetHostedZone(ctx context.Context, params *route53.GetHostedZoneInput, optFns ...func(*route53.Options)) (*route53.GetHostedZoneOutput, error)
}

// WAFGetLoggingConfigurationAPI defines the interface for the WAFv2 GetLoggingConfiguration operation.
// Used by Wave 2 enrichment to detect WebACLs with no logging configured.
type WAFGetLoggingConfigurationAPI interface {
	GetLoggingConfiguration(ctx context.Context, params *wafv2.GetLoggingConfigurationInput, optFns ...func(*wafv2.Options)) (*wafv2.GetLoggingConfigurationOutput, error)
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

// ECRGetRepositoryPolicyAPI defines the interface for the ECR GetRepositoryPolicy operation.
// Used by checkECRRole to extract IAM roles from the repository's resource-based policy.
type ECRGetRepositoryPolicyAPI interface {
	GetRepositoryPolicy(ctx context.Context, params *ecr.GetRepositoryPolicyInput, optFns ...func(*ecr.Options)) (*ecr.GetRepositoryPolicyOutput, error)
}

// ECRDescribeImagesAPI defines the interface for the ECR DescribeImages operation.
type ECRDescribeImagesAPI interface {
	DescribeImages(ctx context.Context, params *ecr.DescribeImagesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeImagesOutput, error)
}

// ECRDescribeImageScanFindingsAPI defines the interface for the ECR DescribeImageScanFindings operation.
// Used by the Wave 2 EnrichECRRepository enricher.
type ECRDescribeImageScanFindingsAPI interface {
	DescribeImageScanFindings(ctx context.Context, params *ecr.DescribeImageScanFindingsInput, optFns ...func(*ecr.Options)) (*ecr.DescribeImageScanFindingsOutput, error)
}

// ECRListImagesAPI defines the interface for the ECR ListImages operation.
// Used by the Wave 2 EnrichECRRepository enricher to enumerate image IDs per repository
// before calling DescribeImageScanFindings on each image.
type ECRListImagesAPI interface {
	ListImages(ctx context.Context, params *ecr.ListImagesInput, optFns ...func(*ecr.Options)) (*ecr.ListImagesOutput, error)
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

// EventBridgeListRuleNamesByTargetAPI defines the interface for the EventBridge ListRuleNamesByTarget operation.
type EventBridgeListRuleNamesByTargetAPI interface {
	ListRuleNamesByTarget(ctx context.Context, params *eventbridge.ListRuleNamesByTargetInput, optFns ...func(*eventbridge.Options)) (*eventbridge.ListRuleNamesByTargetOutput, error)
}

// SFNListStateMachinesAPI defines the interface for the SFN ListStateMachines operation.
type SFNListStateMachinesAPI interface {
	ListStateMachines(ctx context.Context, params *sfn.ListStateMachinesInput, optFns ...func(*sfn.Options)) (*sfn.ListStateMachinesOutput, error)
}

// SFNDescribeStateMachineAPI defines the interface for the SFN DescribeStateMachine
// operation. Used by sfn→role, sfn→kms (EncryptionConfiguration), sfn→lambda
// (parses ASL definition for Resource ARNs pointing at Lambda functions).
type SFNDescribeStateMachineAPI interface {
	DescribeStateMachine(ctx context.Context, params *sfn.DescribeStateMachineInput, optFns ...func(*sfn.Options)) (*sfn.DescribeStateMachineOutput, error)
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

// CodePipelineGetPipelineAPI defines the interface for the CodePipeline GetPipeline operation.
// Used by related-panel checkers that need the full stage/action structure for
// pipeline→* cross-references (cb, role, s3, sns, cfn, ecr, ecs-svc, lambda, kms, logs).
type CodePipelineGetPipelineAPI interface {
	GetPipeline(ctx context.Context, params *codepipeline.GetPipelineInput, optFns ...func(*codepipeline.Options)) (*codepipeline.GetPipelineOutput, error)
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

// EBDescribeApplicationVersionsAPI defines the interface for the Elastic Beanstalk DescribeApplicationVersions operation.
type EBDescribeApplicationVersionsAPI interface {
	DescribeApplicationVersions(ctx context.Context, params *elasticbeanstalk.DescribeApplicationVersionsInput, optFns ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeApplicationVersionsOutput, error)
}

// SESv2ListEmailIdentitiesAPI defines the interface for the SES v2 ListEmailIdentities operation.
type SESv2ListEmailIdentitiesAPI interface {
	ListEmailIdentities(ctx context.Context, params *sesv2.ListEmailIdentitiesInput, optFns ...func(*sesv2.Options)) (*sesv2.ListEmailIdentitiesOutput, error)
}

// SESv2GetAccountAPI defines the interface for the SES v2 GetAccount operation.
type SESv2GetAccountAPI interface {
	GetAccount(ctx context.Context, params *sesv2.GetAccountInput, optFns ...func(*sesv2.Options)) (*sesv2.GetAccountOutput, error)
}

// SESv2GetConfigurationSetEventDestinationsAPI defines the interface for the SES v2 GetConfigurationSetEventDestinations operation.
type SESv2GetConfigurationSetEventDestinationsAPI interface {
	GetConfigurationSetEventDestinations(ctx context.Context, params *sesv2.GetConfigurationSetEventDestinationsInput, optFns ...func(*sesv2.Options)) (*sesv2.GetConfigurationSetEventDestinationsOutput, error)
}

// RedshiftDescribeClustersAPI defines the interface for the Redshift DescribeClusters operation.
type RedshiftDescribeClustersAPI interface {
	DescribeClusters(ctx context.Context, params *redshift.DescribeClustersInput, optFns ...func(*redshift.Options)) (*redshift.DescribeClustersOutput, error)
}

// RedshiftDescribeLoggingStatusAPI defines the interface for the Redshift
// DescribeLoggingStatus operation. Used by redshift→s3 (audit bucket) and
// redshift→logs (CloudWatch log group).
type RedshiftDescribeLoggingStatusAPI interface {
	DescribeLoggingStatus(ctx context.Context, params *redshift.DescribeLoggingStatusInput, optFns ...func(*redshift.Options)) (*redshift.DescribeLoggingStatusOutput, error)
}

// RedshiftDescribeClusterSubnetGroupsAPI defines the interface for the Redshift
// DescribeClusterSubnetGroups operation. Used by redshift→subnet to resolve
// the subnets inside a ClusterSubnetGroupName.
type RedshiftDescribeClusterSubnetGroupsAPI interface {
	DescribeClusterSubnetGroups(ctx context.Context, params *redshift.DescribeClusterSubnetGroupsInput, optFns ...func(*redshift.Options)) (*redshift.DescribeClusterSubnetGroupsOutput, error)
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

// KMSListGrantsAPI defines the interface for the KMS ListGrants operation.
type KMSListGrantsAPI interface {
	ListGrants(ctx context.Context, params *kms.ListGrantsInput, optFns ...func(*kms.Options)) (*kms.ListGrantsOutput, error)
}

// KMSGetKeyPolicyAPI defines the interface for the KMS GetKeyPolicy operation.
type KMSGetKeyPolicyAPI interface {
	GetKeyPolicy(ctx context.Context, params *kms.GetKeyPolicyInput, optFns ...func(*kms.Options)) (*kms.GetKeyPolicyOutput, error)
}

// MSKListClustersV2API defines the interface for the Kafka ListClustersV2 operation.
type MSKListClustersV2API interface {
	ListClustersV2(ctx context.Context, params *kafka.ListClustersV2Input, optFns ...func(*kafka.Options)) (*kafka.ListClustersV2Output, error)
}

// KafkaDescribeClusterV2API defines the interface for the Kafka DescribeClusterV2 operation.
type KafkaDescribeClusterV2API interface {
	DescribeClusterV2(ctx context.Context, params *kafka.DescribeClusterV2Input, optFns ...func(*kafka.Options)) (*kafka.DescribeClusterV2Output, error)
}

// BackupGetBackupPlanAPI defines the interface for the Backup GetBackupPlan
// operation. Used by backup→role / backup→kms / backup→sns to read the plan's
// rules (target vault names) and associated IAM role/KMS/SNS config.
type BackupGetBackupPlanAPI interface {
	GetBackupPlan(ctx context.Context, params *backup.GetBackupPlanInput, optFns ...func(*backup.Options)) (*backup.GetBackupPlanOutput, error)
}

// BackupListBackupSelectionsAPI defines the interface for the Backup
// ListBackupSelections operation. Used by backup→role to enumerate the
// plan's selections (each carries the IAM role ARN used to perform backups).
type BackupListBackupSelectionsAPI interface {
	ListBackupSelections(ctx context.Context, params *backup.ListBackupSelectionsInput, optFns ...func(*backup.Options)) (*backup.ListBackupSelectionsOutput, error)
}

// BackupDescribeBackupVaultAPI defines the interface for the Backup
// DescribeBackupVault operation. Used by backup→kms to resolve the KMS key
// ARN encrypting the plan's target vault.
type BackupDescribeBackupVaultAPI interface {
	DescribeBackupVault(ctx context.Context, params *backup.DescribeBackupVaultInput, optFns ...func(*backup.Options)) (*backup.DescribeBackupVaultOutput, error)
}

// BackupGetBackupVaultNotificationsAPI defines the interface for the Backup
// GetBackupVaultNotifications operation. Used by backup→sns to resolve the
// SNS topic ARN configured for job-event notifications on the vault.
type BackupGetBackupVaultNotificationsAPI interface {
	GetBackupVaultNotifications(ctx context.Context, params *backup.GetBackupVaultNotificationsInput, optFns ...func(*backup.Options)) (*backup.GetBackupVaultNotificationsOutput, error)
}

// BackupListRecoveryPointsByResourceAPI defines the interface for the Backup
// ListRecoveryPointsByResource operation. Used by {rds,docdb}-snap→backup to
// trace a snapshot back to the backup plan (via RecoveryPoint.CreatedBy.BackupPlanId).
type BackupListRecoveryPointsByResourceAPI interface {
	ListRecoveryPointsByResource(ctx context.Context, params *backup.ListRecoveryPointsByResourceInput, optFns ...func(*backup.Options)) (*backup.ListRecoveryPointsByResourceOutput, error)
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

// CWLogsDescribeMetricFiltersAPI defines the interface for the CloudWatchLogs DescribeMetricFilters operation.
// Used by Wave 2 EnrichLogsMetricFilters to detect audit log groups missing metric filters.
type CWLogsDescribeMetricFiltersAPI interface {
	DescribeMetricFilters(ctx context.Context, params *cloudwatchlogs.DescribeMetricFiltersInput, optFns ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.DescribeMetricFiltersOutput, error)
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

// WAFv2GetWebACLAPI defines the interface for the WAFv2 GetWebACL operation.
// Used by EnrichWAFLogging to count BLOCK rules per WebACL.
type WAFv2GetWebACLAPI interface {
	GetWebACL(ctx context.Context, params *wafv2.GetWebACLInput, optFns ...func(*wafv2.Options)) (*wafv2.GetWebACLOutput, error)
}

// EC2DescribeTransitGatewayRouteTablesAPI defines the interface for the EC2
// DescribeTransitGatewayRouteTables operation.
type EC2DescribeTransitGatewayRouteTablesAPI interface {
	DescribeTransitGatewayRouteTables(ctx context.Context, params *ec2.DescribeTransitGatewayRouteTablesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeTransitGatewayRouteTablesOutput, error)
}

// EC2API is the aggregate interface covering all EC2 operations used by a9s fetchers.
// *ec2.Client structurally satisfies this interface.
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
	EC2DescribeTransitGatewayVpcAttachmentsAPI
	EC2DescribeTransitGatewayRouteTablesAPI
	EC2DescribeVpcEndpointsAPI
	EC2DescribeNetworkInterfacesAPI
	EC2DescribeVolumesAPI
	EC2DescribeSnapshotsAPI
	EC2DescribeImagesAPI
	EC2DescribeInstanceStatusAPI        // Wave 2 enrichment
	EC2DescribeVolumeStatusAPI          // Wave 2 enrichment
	EC2DescribeFlowLogsAPI              // Wave 2 enrichment
	EC2DescribeLaunchTemplateVersionsAPI // asg→ami, asg→role, asg→sg
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
	RDSDescribeDBSubnetGroupsAPI
}

// ElastiCacheAPI is the aggregate interface covering all ElastiCache operations used by a9s fetchers.
// *elasticache.Client structurally satisfies this interface.
type ElastiCacheAPI interface {
	ElastiCacheDescribeCacheClustersAPI
	ElastiCacheDescribeReplicationGroupsAPI
	ElastiCacheDescribeCacheSubnetGroupsAPI
	ElastiCacheListTagsForResourceAPI
}

// DynamoDBAPI is the aggregate interface covering all DynamoDB operations used by a9s fetchers.
// *dynamodb.Client structurally satisfies this interface.
type DynamoDBAPI interface {
	DDBListTablesAPI
	DDBDescribeTableAPI
	DynamoDBDescribeContinuousBackupsAPI
	DynamoDBDescribeKinesisStreamingDestinationAPI
}

// DocDBAPI is the aggregate interface covering all DocumentDB operations used by a9s fetchers.
// *docdb.Client structurally satisfies this interface.
type DocDBAPI interface {
	DocDBDescribeDBClustersAPI
	DocDBDescribeDBClusterSnapshotsAPI
	DocDBDescribeDBSubnetGroupsAPI
}

// LambdaAPI is the aggregate interface covering all Lambda operations used by a9s fetchers.
// *lambda.Client structurally satisfies this interface.
type LambdaAPI interface {
	LambdaListFunctionsAPI
	LambdaListEventSourceMappingsAPI
	LambdaGetFunctionAPI
	LambdaListTagsAPI
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
	ASGDescribeLaunchConfigurationsAPI
	ASGDescribeNotificationConfigurationsAPI
	ASGDescribeLifecycleHooksAPI
}

// ElasticBeanstalkAPI is the aggregate interface covering all ElasticBeanstalk operations used by a9s fetchers.
// *elasticbeanstalk.Client structurally satisfies this interface.
type ElasticBeanstalkAPI interface {
	EBDescribeEnvironmentsAPI
	ElasticBeanstalkDescribeEnvironmentHealthAPI // Wave 2 enrichment
	EBDescribeConfigurationSettingsAPI
	EBDescribeEnvironmentResourcesAPI
	EBDescribeApplicationVersionsAPI
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
	// Wave 2 enrichment interfaces.
	IAMGetLoginProfileAPI
	IAMListMFADevicesAPI
	IAMListAccessKeysAPI
	IAMGetInstanceProfileAPI // asg→role, eb→role via IamInstanceProfile
}

// WAFv2API is the aggregate interface covering all WAFv2 operations used by a9s fetchers.
// *wafv2.Client structurally satisfies this interface.
type WAFv2API interface {
	WAFv2ListWebACLsAPI
	WAFv2ListResourcesForWebACLAPI
	WAFGetLoggingConfigurationAPI // Wave 2 enrichment
	// WAFv2GetWebACLAPI is intentionally excluded from the aggregate — EnrichWAFLogging
	// calls GetWebACL via type assertion so test fakes that only cover logging do not
	// need to implement it.
}

// SecretsManagerAPI is the aggregate interface covering all SecretsManager operations used by a9s fetchers.
// *secretsmanager.Client structurally satisfies this interface.
type SecretsManagerAPI interface {
	SecretsManagerListSecretsAPI
	SecretsManagerGetSecretValueAPI
	SecretsManagerGetResourcePolicyAPI
}

// SSMAPI is the aggregate interface covering all SSM operations used by a9s fetchers.
// *ssm.Client structurally satisfies this interface.
type SSMAPI interface {
	SSMDescribeParametersAPI
	SSMGetParameterAPI
	SSMDescribeInstanceInformationAPI
}

// KMSAPI is the aggregate interface covering all KMS operations used by a9s fetchers.
// *kms.Client structurally satisfies this interface.
type KMSAPI interface {
	KMSListKeysAPI
	KMSDescribeKeyAPI
	KMSListAliasesAPI
	KMSGetKeyRotationStatusAPI
	KMSListGrantsAPI
	KMSGetKeyPolicyAPI
}

// Route53API is the aggregate interface covering all Route53 operations used by a9s fetchers.
// *route53.Client structurally satisfies this interface.
type Route53API interface {
	Route53ListHostedZonesAPI
	Route53ListResourceRecordSetsAPI
	Route53GetHostedZoneAPI // Wave 2 enrichment
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
	CodePipelineGetPipelineAPI
}

// ECRAPI is the aggregate interface covering all ECR operations used by a9s fetchers.
// *ecr.Client structurally satisfies this interface.
type ECRAPI interface {
	ECRDescribeRepositoriesAPI
	ECRDescribeImagesAPI
	ECRDescribeImageScanFindingsAPI // Wave 2 enrichment
	ECRGetRepositoryPolicyAPI       // related-panel: ecr→role
}

// CodeArtifactGetRepositoryPermissionsPolicyAPI defines the interface for the CodeArtifact
// GetRepositoryPermissionsPolicy operation. Used by EnrichCodeArtifactRepository (Wave 2 enrichment).
type CodeArtifactGetRepositoryPermissionsPolicyAPI interface {
	GetRepositoryPermissionsPolicy(ctx context.Context, params *codeartifact.GetRepositoryPermissionsPolicyInput, optFns ...func(*codeartifact.Options)) (*codeartifact.GetRepositoryPermissionsPolicyOutput, error)
}

// CodeArtifactGetDomainPermissionsPolicyAPI defines the interface for the CodeArtifact GetDomainPermissionsPolicy operation.
type CodeArtifactGetDomainPermissionsPolicyAPI interface {
	GetDomainPermissionsPolicy(ctx context.Context, params *codeartifact.GetDomainPermissionsPolicyInput, optFns ...func(*codeartifact.Options)) (*codeartifact.GetDomainPermissionsPolicyOutput, error)
}

// CodeArtifactDescribeDomainAPI defines the interface for the CodeArtifact DescribeDomain operation.
// Used by checkCodeartifactKMS to resolve the KMS encryption key for the repository's domain.
type CodeArtifactDescribeDomainAPI interface {
	DescribeDomain(ctx context.Context, params *codeartifact.DescribeDomainInput, optFns ...func(*codeartifact.Options)) (*codeartifact.DescribeDomainOutput, error)
}

// CodeArtifactListPackagesAPI defines the interface for the CodeArtifact ListPackages operation.
// Used by EnrichCodeArtifactRepository to count packages per repository.
type CodeArtifactListPackagesAPI interface {
	ListPackages(ctx context.Context, params *codeartifact.ListPackagesInput, optFns ...func(*codeartifact.Options)) (*codeartifact.ListPackagesOutput, error)
}

// CodeArtifactAPI is the aggregate interface covering all CodeArtifact operations used by a9s fetchers.
// *codeartifact.Client structurally satisfies this interface.
type CodeArtifactAPI interface {
	CodeArtifactListRepositoriesAPI
	CodeArtifactGetRepositoryPermissionsPolicyAPI // Wave 2 enrichment
	CodeArtifactDescribeRepositoryAPI
	CodeArtifactGetDomainPermissionsPolicyAPI
	CodeArtifactDescribeDomainAPI
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
	CWLogsDescribeMetricFiltersAPI // Wave 2 enrichment
}

// SQSAPI is the aggregate interface covering SQS operations used by a9s enrichers.
// Fetchers that need ListQueues perform a runtime type assertion to SQSListQueuesAPI.
// *sqs.Client structurally satisfies this interface.
type SQSAPI interface {
	SQSGetQueueAttributesAPI
}

// SNSAPI is the aggregate interface covering SNS operations used by a9s
// enrichers (GetTopicAttributes, ListSubscriptionsByTopic).
//
// Operations NOT in this aggregate that fetchers/enrichers may need:
//   - ListTopics (paginated)         — used by SNS top-level fetcher
//   - ListSubscriptions (paginated)  — used by sns-sub fetcher
//
// Fetchers that need those operations type-assert clients.SNS to
// SNSListTopicsAPI / SNSListSubscriptionsAPI at the call site.
//
// *sns.Client structurally satisfies all of the above.
type SNSAPI interface {
	SNSListSubscriptionsByTopicAPI
	SNSGetTopicAttributesAPI
	SNSListTagsForResourceAPI
	SNSGetSubscriptionAttributesAPI
}

// EventBridgeAPI is the aggregate interface covering all EventBridge operations used by a9s fetchers.
// *eventbridge.Client structurally satisfies this interface.
type EventBridgeAPI interface {
	EventBridgeListRulesAPI
	EventBridgeListTargetsByRuleAPI
	EventBridgeListRuleNamesByTargetAPI
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
	SFNDescribeStateMachineAPI
	SFNListTagsForResourceAPI
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

// AthenaGetWorkGroupAPI defines the interface for the Athena GetWorkGroup operation.
// Used by EnrichAthenaWorkGroup (Wave 2 enrichment).
type AthenaGetWorkGroupAPI interface {
	GetWorkGroup(ctx context.Context, params *athena.GetWorkGroupInput, optFns ...func(*athena.Options)) (*athena.GetWorkGroupOutput, error)
}

// AthenaAPI is the aggregate interface covering all Athena operations used by a9s fetchers.
// *athena.Client structurally satisfies this interface.
type AthenaAPI interface {
	AthenaListWorkGroupsAPI
	AthenaGetWorkGroupAPI // Wave 2 enrichment
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
	RedshiftDescribeLoggingStatusAPI
	RedshiftDescribeClusterSubnetGroupsAPI
}

// BackupAPI is the aggregate interface covering all Backup operations used by a9s fetchers.
// *backup.Client structurally satisfies this interface.
type BackupAPI interface {
	BackupListBackupPlansAPI
	BackupListBackupJobsAPI
	BackupGetBackupPlanAPI
	BackupListBackupSelectionsAPI
	BackupDescribeBackupVaultAPI
	BackupGetBackupVaultNotificationsAPI
	BackupListRecoveryPointsByResourceAPI
}

// SESv2API is the aggregate interface covering all SESv2 operations used by a9s fetchers.
// *sesv2.Client structurally satisfies this interface.
type SESv2API interface {
	SESv2ListEmailIdentitiesAPI
	SESv2GetAccountAPI
	SESv2GetEmailIdentityAPI
	SESv2GetConfigurationSetEventDestinationsAPI
}

// EFSAPI is the aggregate interface covering all EFS operations used by a9s fetchers.
// *efs.Client structurally satisfies this interface.
type EFSAPI interface {
	EFSDescribeFileSystemsAPI
	EFSDescribeMountTargetsAPI
	EFSDescribeAccessPointsAPI
}

// EFSDescribeAccessPointsAPI defines the interface for the EFS
// DescribeAccessPoints operation.
type EFSDescribeAccessPointsAPI interface {
	DescribeAccessPoints(ctx context.Context, params *efs.DescribeAccessPointsInput, optFns ...func(*efs.Options)) (*efs.DescribeAccessPointsOutput, error)
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
//   - IssueCount: number of resources classified issue-worthy for the menu badge
//     (severity "!" findings; "~" informational do NOT count).
//
//   - Truncated: GLOBAL signal — true when ANY part of the enricher's walk was
//     cut short (EnrichmentCap hit, page cap hit, or API errors skipped records).
//     Kept for back-compat and banner aggregation. Prefer TruncatedIDs for
//     per-resource resolution.
//
//   - TruncatedIDs: per-resource truncation. Key = Resource.ID that could not be
//     fully inspected (API error on that resource, page cap hit during a
//     per-parent paginated walk, etc.). The UI renders "?" on just that row
//     instead of a global banner. An ID appearing here MUST NOT also appear in
//     Findings unless the partial data was still usable.
//
//   - Findings: map from Resource.ID → EnrichmentFinding. May contain entries
//     for resources NOT in the input slice (account-wide enrichers). Enrichers
//     that receive API identifiers in a different form (e.g., ARNs) MUST
//     normalize to Resource.ID before writing to Findings.
//
//   - FieldUpdates: map from Resource.ID → (fieldKey → value). Same normalization
//     rule applies.
//
// MAY have empty maps but MUST NOT be nil for any reference field on
// success — initialize each with `make(...)` before returning.
type EnricherResult struct {
	IssueCount   int
	Truncated    bool
	TruncatedIDs map[string]bool
	Findings     map[string]resource.EnrichmentFinding
	// FieldUpdates carries per-resource Fields[] mutations the enricher wants
	// merged into the cached row. Keyed by resource ID, then by field key.
	// Used by list columns and Color funcs that need access to Wave-2-derived
	// data without subscribing to the Findings stream separately.
	// MUST NOT be nil if the enricher writes any updates; use
	// make(map[string]map[string]string).
	FieldUpdates map[string]map[string]string
}

// EnricherFunc is a pluggable function that makes additional API calls for a
// resource type and returns a typed EnricherResult. The resources slice contains
// retained first-page resources from Wave 1 probes.
type EnricherFunc func(ctx context.Context, clients *ServiceClients, resources []resource.Resource) (EnricherResult, error)

// --- messaging-dev partition appended interfaces ---

// SNSListTagsForResourceAPI for sns→cfn (Tags -> aws:cloudformation:stack-name).
type SNSListTagsForResourceAPI interface {
	ListTagsForResource(ctx context.Context, params *sns.ListTagsForResourceInput, optFns ...func(*sns.Options)) (*sns.ListTagsForResourceOutput, error)
}

// SFNListTagsForResourceAPI for sfn→cfn (Tags -> aws:cloudformation:stack-name).
type SFNListTagsForResourceAPI interface {
	ListTagsForResource(ctx context.Context, params *sfn.ListTagsForResourceInput, optFns ...func(*sfn.Options)) (*sfn.ListTagsForResourceOutput, error)
}

// SESv2GetEmailIdentityAPI for ses→{kms, role} (GetEmailIdentity returns DkimAttributes,
// ConfigurationSetName, Tags, MailFromAttributes).
type SESv2GetEmailIdentityAPI interface {
	GetEmailIdentity(ctx context.Context, params *sesv2.GetEmailIdentityInput, optFns ...func(*sesv2.Options)) (*sesv2.GetEmailIdentityOutput, error)
}

// CodeArtifactDescribeRepositoryAPI for codeartifact→* enrichers.
type CodeArtifactDescribeRepositoryAPI interface {
	DescribeRepository(ctx context.Context, params *codeartifact.DescribeRepositoryInput, optFns ...func(*codeartifact.Options)) (*codeartifact.DescribeRepositoryOutput, error)
}

// EBDescribeConfigurationSettingsAPI for eb→{role, s3, sg, elb, tg} via ConfigurationSettings option values.
type EBDescribeConfigurationSettingsAPI interface {
	DescribeConfigurationSettings(ctx context.Context, params *elasticbeanstalk.DescribeConfigurationSettingsInput, optFns ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeConfigurationSettingsOutput, error)
}

// EBDescribeEnvironmentResourcesAPI for eb→{elb, asg, ec2, tg} via EnvironmentResources.
type EBDescribeEnvironmentResourcesAPI interface {
	DescribeEnvironmentResources(ctx context.Context, params *elasticbeanstalk.DescribeEnvironmentResourcesInput, optFns ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentResourcesOutput, error)
}

// SNSGetSubscriptionAttributesAPI for sns-sub→kms and sns-sub→policy.
type SNSGetSubscriptionAttributesAPI interface {
	GetSubscriptionAttributes(ctx context.Context, params *sns.GetSubscriptionAttributesInput, optFns ...func(*sns.Options)) (*sns.GetSubscriptionAttributesOutput, error)
}

// ASGDescribeLaunchConfigurationsAPI for asg→ami, asg→role, asg→sg via LaunchConfiguration.
type ASGDescribeLaunchConfigurationsAPI interface {
	DescribeLaunchConfigurations(ctx context.Context, params *autoscaling.DescribeLaunchConfigurationsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeLaunchConfigurationsOutput, error)
}

// ASGDescribeNotificationConfigurationsAPI for asg→sns via NotificationConfigurations.
type ASGDescribeNotificationConfigurationsAPI interface {
	DescribeNotificationConfigurations(ctx context.Context, params *autoscaling.DescribeNotificationConfigurationsInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeNotificationConfigurationsOutput, error)
}

// ASGDescribeLifecycleHooksAPI for asg→sns via LifecycleHooks.NotificationTargetARN.
type ASGDescribeLifecycleHooksAPI interface {
	DescribeLifecycleHooks(ctx context.Context, params *autoscaling.DescribeLifecycleHooksInput, optFns ...func(*autoscaling.Options)) (*autoscaling.DescribeLifecycleHooksOutput, error)
}

// EC2DescribeLaunchTemplateVersionsAPI for asg→ami, asg→role, asg→sg via LaunchTemplate.
type EC2DescribeLaunchTemplateVersionsAPI interface {
	DescribeLaunchTemplateVersions(ctx context.Context, params *ec2.DescribeLaunchTemplateVersionsInput, optFns ...func(*ec2.Options)) (*ec2.DescribeLaunchTemplateVersionsOutput, error)
}

// IAMGetInstanceProfileAPI for asg→role and eb→role via IamInstanceProfile.
type IAMGetInstanceProfileAPI interface {
	GetInstanceProfile(ctx context.Context, params *iam.GetInstanceProfileInput, optFns ...func(*iam.Options)) (*iam.GetInstanceProfileOutput, error)
}
