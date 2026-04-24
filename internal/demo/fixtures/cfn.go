package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cfntypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"
)

// CFNFixtures holds typed fixture data for CloudFormation.
type CFNFixtures struct {
	Stacks []cfntypes.Stack
	// StackEvents maps stack name to its events (for DescribeStackEvents).
	StackEvents map[string][]cfntypes.StackEvent
	// StackResources maps stack name to its resources (for ListStackResources).
	StackResources map[string][]cfntypes.StackResourceSummary
}

func mustParseCFNTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// NewCFNFixtures constructs CFNFixtures from the canonical demo data.
func NewCFNFixtures() *CFNFixtures {
	const prodCIDeployRoleARN = "arn:aws:iam::123456789012:role/prod-ci-deploy-role"

	stacks := []cfntypes.Stack{
		{
			StackName:                   aws.String("acme-vpc-stack"),
			StackStatus:                 cfntypes.StackStatusCreateComplete,
			StackStatusReason:           aws.String("Stack CREATE_COMPLETE"),
			CreationTime:                aws.Time(mustParseCFNTime("2024-10-15T09:00:00+00:00")),
			LastUpdatedTime:             aws.Time(mustParseCFNTime("2025-06-20T14:30:00+00:00")),
			Description:                 aws.String("Core VPC networking stack for Acme Corp production"),
			StackId:                     aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-vpc-stack/11111111-1111-1111-1111-111111111111"),
			RoleARN:                     aws.String(prodCIDeployRoleARN),
			Capabilities:                []cfntypes.Capability{cfntypes.CapabilityCapabilityIam},
			EnableTerminationProtection: aws.Bool(true),
			DriftInformation: &cfntypes.StackDriftInformation{
				StackDriftStatus: cfntypes.StackDriftStatusInSync,
			},
			Parameters: []cfntypes.Parameter{
				{ParameterKey: aws.String("VpcCidr"), ParameterValue: aws.String("10.0.0.0/16")},
				{ParameterKey: aws.String("Environment"), ParameterValue: aws.String("production")},
			},
			Outputs: []cfntypes.Output{
				{OutputKey: aws.String("VpcId"), OutputValue: aws.String("vpc-0abc123def456789a"), Description: aws.String("Production VPC ID")},
			},
			Tags: []cfntypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("production")},
			},
		},
		{
			StackName:       aws.String("acme-eks-cluster"),
			StackStatus:     cfntypes.StackStatusUpdateComplete,
			CreationTime:    aws.Time(mustParseCFNTime("2025-01-10T11:00:00+00:00")),
			LastUpdatedTime: aws.Time(mustParseCFNTime("2026-03-15T08:45:00+00:00")),
			Description:     aws.String("EKS cluster and managed node groups"),
			StackId:         aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-eks-cluster/22222222-2222-2222-2222-222222222222"),
			RoleARN:         aws.String(prodCIDeployRoleARN),
		},
		{
			StackName:    aws.String("acme-rds-aurora"),
			StackStatus:  cfntypes.StackStatusCreateComplete,
			CreationTime: aws.Time(mustParseCFNTime("2025-03-05T16:20:00+00:00")),
			Description:  aws.String("Aurora PostgreSQL cluster for API backend"),
			StackId:      aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-rds-aurora/33333333-3333-3333-3333-333333333333"),
			RoleARN:      aws.String(prodCIDeployRoleARN),
		},
		{
			StackName:       aws.String("acme-monitoring"),
			StackStatus:     cfntypes.StackStatusUpdateRollbackComplete,
			CreationTime:    aws.Time(mustParseCFNTime("2025-02-01T10:00:00+00:00")),
			LastUpdatedTime: aws.Time(mustParseCFNTime("2026-03-18T22:15:00+00:00")),
			Description:     aws.String("CloudWatch alarms and dashboards"),
			StackId:         aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-monitoring/44444444-4444-4444-4444-444444444444"),
			RoleARN:         aws.String(prodCIDeployRoleARN),
		},
		{
			StackName:         aws.String("acme-legacy-api"),
			StackStatus:       cfntypes.StackStatusRollbackComplete,
			StackStatusReason: aws.String("The following resource(s) failed to create: [ApiFunction]. Rollback requested by user."),
			CreationTime:      aws.Time(mustParseCFNTime("2026-03-21T11:00:00+00:00")),
			LastUpdatedTime:   aws.Time(mustParseCFNTime("2026-03-21T11:47:00+00:00")),
			Description:       aws.String("Legacy API migration stack — initial deployment rolled back"),
			StackId:           aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-legacy-api/55555555-5555-5555-5555-555555555555"),
			RoleARN:           aws.String(prodCIDeployRoleARN),
			Capabilities:      []cfntypes.Capability{cfntypes.CapabilityCapabilityIam},
			DriftInformation: &cfntypes.StackDriftInformation{
				StackDriftStatus: cfntypes.StackDriftStatusNotChecked,
			},
			Tags: []cfntypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("production")},
				{Key: aws.String("Team"), Value: aws.String("backend")},
			},
		},
		// Drifted stack → Warning (drift detection triggered 2 days ago)
		{
			StackName:       aws.String("stack-drifted-prod"),
			StackStatus:     cfntypes.StackStatusUpdateComplete,
			CreationTime:    aws.Time(mustParseCFNTime("2025-05-01T09:00:00+00:00")),
			LastUpdatedTime: aws.Time(mustParseCFNTime("2026-04-10T12:00:00+00:00")),
			Description:     aws.String("Production services stack — detected drift from expected configuration"),
			StackId:         aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/stack-drifted-prod/66666666-6666-6666-6666-666666666666"),
			RoleARN:         aws.String(prodCIDeployRoleARN),
			DriftInformation: &cfntypes.StackDriftInformation{
				StackDriftStatus:   cfntypes.StackDriftStatusDrifted,
				LastCheckTimestamp: aws.Time(time.Now().AddDate(0, 0, -2)),
			},
			Tags: []cfntypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("production")},
				{Key: aws.String("Team"), Value: aws.String("platform")},
			},
		},
		// Stuck UPDATE_IN_PROGRESS (started >2h ago) → Broken
		{
			StackName:       aws.String("stack-stuck-update"),
			StackStatus:     cfntypes.StackStatusUpdateInProgress,
			CreationTime:    aws.Time(mustParseCFNTime("2025-08-15T14:00:00+00:00")),
			LastUpdatedTime: aws.Time(time.Now().Add(-3 * time.Hour)),
			Description:     aws.String("Database migration stack — update stalled on RDS parameter group change"),
			StackId:         aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/stack-stuck-update/77777777-7777-7777-7777-777777777777"),
			RoleARN:         aws.String(prodCIDeployRoleARN),
			DriftInformation: &cfntypes.StackDriftInformation{
				StackDriftStatus: cfntypes.StackDriftStatusInSync,
			},
			Tags: []cfntypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("production")},
				{Key: aws.String("Team"), Value: aws.String("data")},
			},
		},
		// S3 healthy-bucket CFN stack (checkS3CFN pivot).
		// The healthy bucket carries the aws:cloudformation:stack-name tag pointing here.
		{
			StackName:    aws.String(S3CFNStackName),
			StackStatus:  cfntypes.StackStatusCreateComplete,
			CreationTime: aws.Time(mustParseCFNTime("2025-01-10T10:00:00+00:00")),
			Description:  aws.String("S3 demo bucket stack managed by CloudFormation"),
			StackId:      aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/" + S3CFNStackName + "/88888888-8888-8888-8888-888888888888"),
			RoleARN:      aws.String(prodCIDeployRoleARN),
			Tags: []cfntypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("production")},
			},
		},
		// OpenSearch graph-root CFN stack — required for opensearch→cfn related-panel pivot.
		// The acme-logs domain's ListTags fake returns aws:cloudformation:stack-name=acme-search-stack.
		{
			StackName:    aws.String(OpenSearchCFNStackName),
			StackStatus:  cfntypes.StackStatusCreateComplete,
			CreationTime: aws.Time(mustParseCFNTime("2025-09-01T10:00:00+00:00")),
			Description:  aws.String("OpenSearch cluster for acme-logs (full-text search + audit logging)"),
			StackId:      aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/" + OpenSearchCFNStackName + "/99999999-9999-9999-9999-999999999999"),
			RoleARN:      aws.String(prodCIDeployRoleARN),
			Tags: []cfntypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("production")},
				{Key: aws.String("Service"), Value: aws.String("search")},
			},
		},
		// Redshift acme-warehouse CFN stack — required for redshift→cfn related-panel pivot.
		// The acme-warehouse cluster carries the aws:cloudformation:stack-name tag
		// pointing to "acme-warehouse-stack" so checkRedshiftCFN resolves a non-zero count.
		{
			StackName:    aws.String("acme-warehouse-stack"),
			StackStatus:  cfntypes.StackStatusCreateComplete,
			CreationTime: aws.Time(mustParseCFNTime("2025-03-10T09:00:00+00:00")),
			Description:  aws.String("Redshift analytics cluster stack for Acme Corp warehouse"),
			StackId:      aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-warehouse-stack/aaaa1111-bbbb-2222-cccc-333333333333"),
			RoleARN:      aws.String(prodCIDeployRoleARN),
			Tags: []cfntypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("production")},
				{Key: aws.String("Service"), Value: aws.String("analytics")},
			},
		},
		// Redshift acme-reporting CFN stack — required for redshift→cfn related-panel pivot (second graph-root).
		{
			StackName:    aws.String("acme-reporting-stack"),
			StackStatus:  cfntypes.StackStatusCreateComplete,
			CreationTime: aws.Time(mustParseCFNTime("2025-07-22T14:30:00+00:00")),
			Description:  aws.String("Redshift reporting cluster stack for Acme Corp reporting"),
			StackId:      aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-reporting-stack/dddd4444-eeee-5555-ffff-666666666666"),
			RoleARN:      aws.String(prodCIDeployRoleARN),
			Tags: []cfntypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("production")},
				{Key: aws.String("Service"), Value: aws.String("reporting")},
			},
		},
		// Redis prod CFN stack — required for redis→cfn related-panel pivot.
		// The prod-redis-sessions RG carries the aws:cloudformation:stack-name tag
		// pointing to ProdRedisCFNStack so checkRedisCFN resolves a non-zero count.
		{
			StackName:    aws.String(ProdRedisCFNStack),
			StackStatus:  cfntypes.StackStatusCreateComplete,
			CreationTime: aws.Time(mustParseCFNTime("2025-03-15T08:00:00+00:00")),
			Description:  aws.String("ElastiCache Redis cluster for production session storage"),
			StackId:      aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/" + ProdRedisCFNStack + "/aaaabbbb-cccc-dddd-eeee-ffffffffffff"),
			RoleARN:      aws.String(prodCIDeployRoleARN),
			Tags: []cfntypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("production")},
				{Key: aws.String("Service"), Value: aws.String("sessions")},
			},
		},
		// EFS prod-app-data CFN stack — required for efs→cfn related-panel pivot.
		// The prod-efs-app-data filesystem carries the aws:cloudformation:stack-name tag
		// pointing to ProdEFSCFNStackName so checkEFSCFN resolves a non-zero count.
		{
			StackName:    aws.String(ProdEFSCFNStackName),
			StackStatus:  cfntypes.StackStatusCreateComplete,
			CreationTime: aws.Time(mustParseCFNTime("2025-02-01T10:00:00+00:00")),
			Description:  aws.String("EFS filesystem for production app data shared storage"),
			StackId:      aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/" + ProdEFSCFNStackName + "/bbbbcccc-dddd-eeee-ffff-000000000001"),
			RoleARN:      aws.String(prodCIDeployRoleARN),
			Tags: []cfntypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("production")},
				{Key: aws.String("Service"), Value: aws.String("storage")},
			},
		},
	}

	stackEvents := map[string][]cfntypes.StackEvent{
		"acme-legacy-api": {
			{
				EventId:              aws.String("evt-legacy-001"),
				StackName:            aws.String("acme-legacy-api"),
				Timestamp:            aws.Time(mustParseCFNTime("2026-03-21T11:00:00+00:00")),
				LogicalResourceId:    aws.String("acme-legacy-api"),
				ResourceType:         aws.String("AWS::CloudFormation::Stack"),
				ResourceStatus:       cfntypes.ResourceStatusCreateInProgress,
				ResourceStatusReason: aws.String("User Initiated"),
			},
			{
				EventId:              aws.String("evt-legacy-002"),
				StackName:            aws.String("acme-legacy-api"),
				Timestamp:            aws.Time(mustParseCFNTime("2026-03-21T11:20:00+00:00")),
				LogicalResourceId:    aws.String("ApiFunction"),
				ResourceType:         aws.String("AWS::Lambda::Function"),
				ResourceStatus:       cfntypes.ResourceStatusCreateFailed,
				ResourceStatusReason: aws.String("Resource handler returned message: \"Layer arn:aws:lambda:us-east-1:123456789012:layer:legacy-utils:3 does not exist.\" (HandlerErrorCode: NotFound)"),
			},
			{
				EventId:           aws.String("evt-legacy-003"),
				StackName:         aws.String("acme-legacy-api"),
				Timestamp:         aws.Time(mustParseCFNTime("2026-03-21T11:25:00+00:00")),
				LogicalResourceId: aws.String("acme-legacy-api"),
				ResourceType:      aws.String("AWS::CloudFormation::Stack"),
				ResourceStatus:    cfntypes.ResourceStatusRollbackInProgress,
			},
			{
				EventId:           aws.String("evt-legacy-004"),
				StackName:         aws.String("acme-legacy-api"),
				Timestamp:         aws.Time(mustParseCFNTime("2026-03-21T11:47:00+00:00")),
				LogicalResourceId: aws.String("acme-legacy-api"),
				ResourceType:      aws.String("AWS::CloudFormation::Stack"),
				ResourceStatus:    cfntypes.ResourceStatusRollbackComplete,
			},
		},
		"acme-vpc-stack": {
			{
				EventId:              aws.String("evt-001"),
				StackName:            aws.String("acme-vpc-stack"),
				Timestamp:            aws.Time(mustParseCFNTime("2026-03-20T10:00:00+00:00")),
				LogicalResourceId:    aws.String("acme-vpc-stack"),
				ResourceType:         aws.String("AWS::CloudFormation::Stack"),
				ResourceStatus:       cfntypes.ResourceStatusUpdateInProgress,
				ResourceStatusReason: aws.String("User Initiated"),
			},
			{
				EventId:           aws.String("evt-002"),
				StackName:         aws.String("acme-vpc-stack"),
				Timestamp:         aws.Time(mustParseCFNTime("2026-03-20T10:02:00+00:00")),
				LogicalResourceId: aws.String("acme-vpc-stack"),
				ResourceType:      aws.String("AWS::CloudFormation::Stack"),
				ResourceStatus:    cfntypes.ResourceStatusUpdateComplete,
			},
		},
	}

	stackResources := map[string][]cfntypes.StackResourceSummary{
		"acme-legacy-api": {
			{
				LogicalResourceId:    aws.String("ApiFunction"),
				ResourceType:         aws.String("AWS::Lambda::Function"),
				ResourceStatus:       cfntypes.ResourceStatusCreateFailed,
				ResourceStatusReason: aws.String("Layer does not exist"),
				LastUpdatedTimestamp: aws.Time(mustParseCFNTime("2026-03-21T11:20:00+00:00")),
			},
			{
				LogicalResourceId:    aws.String("ApiGateway"),
				PhysicalResourceId:   aws.String("abc123xyz"),
				ResourceType:         aws.String("AWS::ApiGatewayV2::Api"),
				ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
				LastUpdatedTimestamp: aws.Time(mustParseCFNTime("2026-03-21T11:10:00+00:00")),
			},
			{
				LogicalResourceId:    aws.String("FunctionRole"),
				PhysicalResourceId:   aws.String("acme-legacy-api-FunctionRole-1A2B3C"),
				ResourceType:         aws.String("AWS::IAM::Role"),
				ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
				LastUpdatedTimestamp: aws.Time(mustParseCFNTime("2026-03-21T11:05:00+00:00")),
			},
		},
		"acme-vpc-stack": {
			{
				LogicalResourceId:    aws.String("VPC"),
				PhysicalResourceId:   aws.String("vpc-0abc123def456789a"),
				ResourceType:         aws.String("AWS::EC2::VPC"),
				ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
				LastUpdatedTimestamp: aws.Time(time.Date(2024, 10, 15, 9, 5, 0, 0, time.UTC)),
			},
			{
				LogicalResourceId:    aws.String("PublicSubnet1"),
				PhysicalResourceId:   aws.String("subnet-0aaa111111111111a"),
				ResourceType:         aws.String("AWS::EC2::Subnet"),
				ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
				LastUpdatedTimestamp: aws.Time(time.Date(2024, 10, 15, 9, 8, 0, 0, time.UTC)),
			},
		},
	}

	return &CFNFixtures{
		Stacks:         stacks,
		StackEvents:    stackEvents,
		StackResources: stackResources,
	}
}
