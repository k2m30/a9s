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
	"github.com/aws/aws-sdk-go-v2/service/wafv2"
)

// EC2DescribeInstancesAPI defines the interface for the EC2 DescribeInstances operation.
type EC2DescribeInstancesAPI interface {
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
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

// DDBListTablesAPI defines the interface for the DynamoDB ListTables operation.
type DDBListTablesAPI interface {
	ListTables(ctx context.Context, params *dynamodb.ListTablesInput, optFns ...func(*dynamodb.Options)) (*dynamodb.ListTablesOutput, error)
}

// DDBDescribeTableAPI defines the interface for the DynamoDB DescribeTable operation.
type DDBDescribeTableAPI interface {
	DescribeTable(ctx context.Context, params *dynamodb.DescribeTableInput, optFns ...func(*dynamodb.Options)) (*dynamodb.DescribeTableOutput, error)
}

// EC2DescribeAddressesAPI defines the interface for the EC2 DescribeAddresses operation.
type EC2DescribeAddressesAPI interface {
	DescribeAddresses(ctx context.Context, params *ec2.DescribeAddressesInput, optFns ...func(*ec2.Options)) (*ec2.DescribeAddressesOutput, error)
}

// ACMListCertificatesAPI defines the interface for the ACM ListCertificates operation.
type ACMListCertificatesAPI interface {
	ListCertificates(ctx context.Context, params *acm.ListCertificatesInput, optFns ...func(*acm.Options)) (*acm.ListCertificatesOutput, error)
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

// Route53ListHostedZonesAPI defines the interface for the Route53 ListHostedZones operation.
type Route53ListHostedZonesAPI interface {
	ListHostedZones(ctx context.Context, params *route53.ListHostedZonesInput, optFns ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error)
}

// APIGatewayV2GetApisAPI defines the interface for the API Gateway V2 GetApis operation.
type APIGatewayV2GetApisAPI interface {
	GetApis(ctx context.Context, params *apigatewayv2.GetApisInput, optFns ...func(*apigatewayv2.Options)) (*apigatewayv2.GetApisOutput, error)
}

// ECRDescribeRepositoriesAPI defines the interface for the ECR DescribeRepositories operation.
type ECRDescribeRepositoriesAPI interface {
	DescribeRepositories(ctx context.Context, params *ecr.DescribeRepositoriesInput, optFns ...func(*ecr.Options)) (*ecr.DescribeRepositoriesOutput, error)
}

// EFSDescribeFileSystemsAPI defines the interface for the EFS DescribeFileSystems operation.
type EFSDescribeFileSystemsAPI interface {
	DescribeFileSystems(ctx context.Context, params *efs.DescribeFileSystemsInput, optFns ...func(*efs.Options)) (*efs.DescribeFileSystemsOutput, error)
}

// EventBridgeListRulesAPI defines the interface for the EventBridge ListRules operation.
type EventBridgeListRulesAPI interface {
	ListRules(ctx context.Context, params *eventbridge.ListRulesInput, optFns ...func(*eventbridge.Options)) (*eventbridge.ListRulesOutput, error)
}

// SFNListStateMachinesAPI defines the interface for the SFN ListStateMachines operation.
type SFNListStateMachinesAPI interface {
	ListStateMachines(ctx context.Context, params *sfn.ListStateMachinesInput, optFns ...func(*sfn.Options)) (*sfn.ListStateMachinesOutput, error)
}

// CodePipelineListPipelinesAPI defines the interface for the CodePipeline ListPipelines operation.
type CodePipelineListPipelinesAPI interface {
	ListPipelines(ctx context.Context, params *codepipeline.ListPipelinesInput, optFns ...func(*codepipeline.Options)) (*codepipeline.ListPipelinesOutput, error)
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

// EBDescribeEnvironmentsAPI defines the interface for the Elastic Beanstalk DescribeEnvironments operation.
type EBDescribeEnvironmentsAPI interface {
	DescribeEnvironments(ctx context.Context, params *elasticbeanstalk.DescribeEnvironmentsInput, optFns ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentsOutput, error)
}

// SESv2ListEmailIdentitiesAPI defines the interface for the SES v2 ListEmailIdentities operation.
type SESv2ListEmailIdentitiesAPI interface {
	ListEmailIdentities(ctx context.Context, params *sesv2.ListEmailIdentitiesInput, optFns ...func(*sesv2.Options)) (*sesv2.ListEmailIdentitiesOutput, error)
}

// RedshiftDescribeClustersAPI defines the interface for the Redshift DescribeClusters operation.
type RedshiftDescribeClustersAPI interface {
	DescribeClusters(ctx context.Context, params *redshift.DescribeClustersInput, optFns ...func(*redshift.Options)) (*redshift.DescribeClustersOutput, error)
}

// CloudTrailDescribeTrailsAPI defines the interface for the CloudTrail DescribeTrails operation.
type CloudTrailDescribeTrailsAPI interface {
	DescribeTrails(ctx context.Context, params *cloudtrail.DescribeTrailsInput, optFns ...func(*cloudtrail.Options)) (*cloudtrail.DescribeTrailsOutput, error)
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

// MSKListClustersV2API defines the interface for the Kafka ListClustersV2 operation.
type MSKListClustersV2API interface {
	ListClustersV2(ctx context.Context, params *kafka.ListClustersV2Input, optFns ...func(*kafka.Options)) (*kafka.ListClustersV2Output, error)
}

// BackupListBackupPlansAPI defines the interface for the Backup ListBackupPlans operation.
type BackupListBackupPlansAPI interface {
	ListBackupPlans(ctx context.Context, params *backup.ListBackupPlansInput, optFns ...func(*backup.Options)) (*backup.ListBackupPlansOutput, error)
}
