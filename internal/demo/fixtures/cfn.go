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
	}

	stackEvents := map[string][]cfntypes.StackEvent{
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
		"acme-vpc-stack": {
			{
				LogicalResourceId:  aws.String("VPC"),
				PhysicalResourceId: aws.String("vpc-0abc123def456789a"),
				ResourceType:       aws.String("AWS::EC2::VPC"),
				ResourceStatus:     cfntypes.ResourceStatusCreateComplete,
				LastUpdatedTimestamp: aws.Time(time.Date(2024, 10, 15, 9, 5, 0, 0, time.UTC)),
			},
			{
				LogicalResourceId:  aws.String("PublicSubnet1"),
				PhysicalResourceId: aws.String("subnet-0aaa111111111111a"),
				ResourceType:       aws.String("AWS::EC2::Subnet"),
				ResourceStatus:     cfntypes.ResourceStatusCreateComplete,
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
