// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

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
	// TemplateBodies maps stack name to its template JSON (for GetTemplate).
	// Stacks with no entry get the fake's GenericTemplateBody fallback —
	// GetTemplate has no "not configured" absent-case in real AWS (a stack
	// always has a template), so every fixture stack must resolve to some body.
	TemplateBodies map[string]string
}

// GenericTemplateBody is the GetTemplate fallback for any fixture stack with
// no entry in TemplateBodies.
const GenericTemplateBody = `{
  "AWSTemplateFormatVersion": "2010-09-09",
  "Resources": {
    "DemoResource": {
      "Type": "AWS::CloudFormation::WaitConditionHandle"
    }
  }
}`

func mustParseCFNTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// stackEvent is one DescribeStackEvents entry. AWS gives the stack's own
// events the stack ID as PhysicalResourceId, which is how the latest operation
// is found; an empty logicalID makes it such an event.
func stackEvent(stackName, stackID, eventID, at, logicalID, resType, physicalID string, status cfntypes.ResourceStatus, reason string) cfntypes.StackEvent {
	if logicalID == "" {
		logicalID, resType, physicalID = stackName, "AWS::CloudFormation::Stack", stackID
	}
	ev := cfntypes.StackEvent{
		EventId:            aws.String(eventID),
		StackName:          aws.String(stackName),
		StackId:            aws.String(stackID),
		Timestamp:          aws.Time(mustParseCFNTime(at)),
		LogicalResourceId:  aws.String(logicalID),
		ResourceType:       aws.String(resType),
		PhysicalResourceId: aws.String(physicalID),
		ResourceStatus:     status,
	}
	if reason != "" {
		ev.ResourceStatusReason = aws.String(reason)
	}
	return ev
}

// minimalCreateEventSequence returns the 4 canonical CREATE events for a stack:
// stack-create-in-progress, primary-resource-create-in-progress,
// primary-resource-create-complete, stack-create-complete, so every graph-root
// stack has a non-empty cfn_events child view.
// startTime is the ISO-8601 timestamp of the first event; subsequent events are
// spaced 15/45/48 minutes apart.
func minimalCreateEventSequence(stackName, startTime, primaryResLogicalID, primaryResType string) []cfntypes.StackEvent {
	t0 := mustParseCFNTime(startTime)
	return []cfntypes.StackEvent{
		{
			EventId:              aws.String("evt-" + stackName + "-001"),
			StackName:            aws.String(stackName),
			Timestamp:            aws.Time(t0),
			LogicalResourceId:    aws.String(stackName),
			ResourceType:         aws.String("AWS::CloudFormation::Stack"),
			ResourceStatus:       cfntypes.ResourceStatusCreateInProgress,
			ResourceStatusReason: aws.String("User Initiated"),
		},
		{
			EventId:           aws.String("evt-" + stackName + "-002"),
			StackName:         aws.String(stackName),
			Timestamp:         aws.Time(t0.Add(15 * time.Minute)),
			LogicalResourceId: aws.String(primaryResLogicalID),
			ResourceType:      aws.String(primaryResType),
			ResourceStatus:    cfntypes.ResourceStatusCreateInProgress,
		},
		{
			EventId:           aws.String("evt-" + stackName + "-003"),
			StackName:         aws.String(stackName),
			Timestamp:         aws.Time(t0.Add(45 * time.Minute)),
			LogicalResourceId: aws.String(primaryResLogicalID),
			ResourceType:      aws.String(primaryResType),
			ResourceStatus:    cfntypes.ResourceStatusCreateComplete,
		},
		{
			EventId:           aws.String("evt-" + stackName + "-004"),
			StackName:         aws.String(stackName),
			Timestamp:         aws.Time(t0.Add(48 * time.Minute)),
			LogicalResourceId: aws.String(stackName),
			ResourceType:      aws.String("AWS::CloudFormation::Stack"),
			ResourceStatus:    cfntypes.ResourceStatusCreateComplete,
		},
	}
}

// NewCFNFixtures constructs CFNFixtures from the canonical demo data.
var sharedCFNFixtures = shared(func() *CFNFixtures {
	const prodCIDeployRoleARN = "arn:aws:iam::123456789012:role/prod-ci-deploy-role"

	stacks := []cfntypes.Stack{
		{
			StackName:                   aws.String("acme-vpc-stack"),
			StackStatus:                 cfntypes.StackStatusCreateComplete,
			StackStatusReason:           aws.String("Stack CREATE_COMPLETE"),
			CreationTime:                aws.Time(mustParseCFNTime("2024-10-15T09:00:00+00:00")),
			LastUpdatedTime:             aws.Time(mustParseCFNTime("2025-06-20T14:30:00+00:00")),
			Description:                 aws.String("Core VPC networking stack for Acme Corp production"),
			StackId:                     aws.String(cfnVPCStackID),
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
			StackName:                   aws.String("acme-eks-cluster"),
			EnableTerminationProtection: aws.Bool(true),
			StackStatus:                 cfntypes.StackStatusUpdateComplete,
			CreationTime:                aws.Time(mustParseCFNTime("2025-01-10T11:00:00+00:00")),
			LastUpdatedTime:             aws.Time(mustParseCFNTime("2026-03-15T08:45:00+00:00")),
			Description:                 aws.String("EKS cluster and managed node groups"),
			StackId:                     aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-eks-cluster/22222222-2222-2222-2222-222222222222"),
			RoleARN:                     aws.String(prodCIDeployRoleARN),
		},
		{
			StackName:                   aws.String("acme-rds-aurora"),
			EnableTerminationProtection: aws.Bool(true),
			StackStatus:                 cfntypes.StackStatusCreateComplete,
			CreationTime:                aws.Time(mustParseCFNTime("2025-03-05T16:20:00+00:00")),
			Description:                 aws.String("Aurora PostgreSQL cluster for API backend"),
			StackId:                     aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-rds-aurora/33333333-3333-3333-3333-333333333333"),
			RoleARN:                     aws.String(prodCIDeployRoleARN),
			// CFNOutputSecret: the master password was exported as a
			// plain stack output, readable by anyone who can describe the
			// stack. The endpoint output beside it is the healthy shape.
			Outputs: []cfntypes.Output{
				{
					OutputKey:   aws.String("ClusterEndpoint"),
					OutputValue: aws.String("acme-aurora.cluster-abc123.us-east-1.rds.amazonaws.com"),
					Description: aws.String("Writer endpoint for the Aurora cluster"),
				},
				{
					OutputKey:   aws.String("db-master-password"),
					OutputValue: aws.String("Tr0ub4dor&3xample"),
					Description: aws.String("Database master password"),
				},
			},
		},
		{
			StackName:       aws.String("acme-monitoring"),
			StackStatus:     cfntypes.StackStatusUpdateRollbackComplete,
			CreationTime:    aws.Time(mustParseCFNTime("2025-02-01T10:00:00+00:00")),
			LastUpdatedTime: aws.Time(mustParseCFNTime("2026-03-18T22:15:00+00:00")),
			Description:     aws.String("CloudWatch alarms and dashboards"),
			StackId:         aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-monitoring/44444444-4444-4444-4444-444444444444"),
			RoleARN:         aws.String(prodCIDeployRoleARN),
			// ParentId — required for the cfn:cfn related-panel pivot
			// (checkCFNCFN nested-stack reverse scan). Marks acme-monitoring
			// as a nested stack of acme-vpc-stack.
			ParentId: aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-vpc-stack/11111111-1111-1111-1111-111111111111"),
			// NotificationARNs — required for the cfn:sns related-panel pivot
			// (checkCfnSNS). Reuses the shared ops-alerts SNS topic.
			NotificationARNs: []string{relatedAlarmSNSARN},
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
			StackName:                   aws.String("stack-drifted-prod"),
			EnableTerminationProtection: aws.Bool(true),
			StackStatus:                 cfntypes.StackStatusUpdateComplete,
			CreationTime:                aws.Time(mustParseCFNTime("2025-05-01T09:00:00+00:00")),
			LastUpdatedTime:             aws.Time(mustParseCFNTime("2026-04-10T12:00:00+00:00")),
			Description:                 aws.String("Production services stack — detected drift from expected configuration"),
			StackId:                     aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/stack-drifted-prod/66666666-6666-6666-6666-666666666666"),
			RoleARN:                     aws.String(prodCIDeployRoleARN),
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
			StackName:                   aws.String("stack-stuck-update"),
			EnableTerminationProtection: aws.Bool(true),
			StackStatus:                 cfntypes.StackStatusUpdateInProgress,
			CreationTime:                aws.Time(mustParseCFNTime("2025-08-15T14:00:00+00:00")),
			LastUpdatedTime:             aws.Time(time.Now().Add(-3 * time.Hour)),
			Description:                 aws.String("Database migration stack — update stalled on RDS parameter group change"),
			StackId:                     aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/stack-stuck-update/77777777-7777-7777-7777-777777777777"),
			RoleARN:                     aws.String(prodCIDeployRoleARN),
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
			StackName:                   aws.String(S3CFNStackName),
			EnableTerminationProtection: aws.Bool(true),
			StackStatus:                 cfntypes.StackStatusCreateComplete,
			CreationTime:                aws.Time(mustParseCFNTime("2025-01-10T10:00:00+00:00")),
			Description:                 aws.String("S3 demo bucket stack managed by CloudFormation"),
			StackId:                     aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/" + S3CFNStackName + "/88888888-8888-8888-8888-888888888888"),
			RoleARN:                     aws.String(prodCIDeployRoleARN),
			Tags: []cfntypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("production")},
			},
		},
		// OpenSearch graph-root CFN stack — required for opensearch→cfn related-panel pivot.
		// The acme-logs domain's ListTags fake returns aws:cloudformation:stack-name=acme-search-stack.
		{
			StackName:                   aws.String(OpenSearchCFNStackName),
			EnableTerminationProtection: aws.Bool(true),
			StackStatus:                 cfntypes.StackStatusCreateComplete,
			CreationTime:                aws.Time(mustParseCFNTime("2025-09-01T10:00:00+00:00")),
			Description:                 aws.String("OpenSearch cluster for acme-logs (full-text search + audit logging)"),
			StackId:                     aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/" + OpenSearchCFNStackName + "/99999999-9999-9999-9999-999999999999"),
			RoleARN:                     aws.String(prodCIDeployRoleARN),
			Tags: []cfntypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("production")},
				{Key: aws.String("Service"), Value: aws.String("search")},
			},
		},
		// Redshift acme-warehouse CFN stack — required for redshift→cfn related-panel pivot.
		// The acme-warehouse cluster carries the aws:cloudformation:stack-name tag
		// pointing to "acme-warehouse-stack" so checkRedshiftCFN resolves a non-zero count.
		{
			StackName:                   aws.String("acme-warehouse-stack"),
			EnableTerminationProtection: aws.Bool(true),
			StackStatus:                 cfntypes.StackStatusCreateComplete,
			CreationTime:                aws.Time(mustParseCFNTime("2025-03-10T09:00:00+00:00")),
			Description:                 aws.String("Redshift analytics cluster stack for Acme Corp warehouse"),
			StackId:                     aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-warehouse-stack/aaaa1111-bbbb-2222-cccc-333333333333"),
			RoleARN:                     aws.String(prodCIDeployRoleARN),
			Tags: []cfntypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("production")},
				{Key: aws.String("Service"), Value: aws.String("analytics")},
			},
		},
		// Redshift acme-reporting CFN stack — required for redshift→cfn related-panel pivot (second graph-root).
		{
			StackName:                   aws.String("acme-reporting-stack"),
			EnableTerminationProtection: aws.Bool(true),
			StackStatus:                 cfntypes.StackStatusCreateComplete,
			CreationTime:                aws.Time(mustParseCFNTime("2025-07-22T14:30:00+00:00")),
			Description:                 aws.String("Redshift reporting cluster stack for Acme Corp reporting"),
			StackId:                     aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-reporting-stack/dddd4444-eeee-5555-ffff-666666666666"),
			RoleARN:                     aws.String(prodCIDeployRoleARN),
			Tags: []cfntypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("production")},
				{Key: aws.String("Service"), Value: aws.String("reporting")},
			},
		},
		// Redis prod CFN stack — required for redis→cfn related-panel pivot.
		// The prod-redis-sessions RG carries the aws:cloudformation:stack-name tag
		// pointing to ProdRedisCFNStack so checkRedisCFN resolves a non-zero count.
		{
			StackName:                   aws.String(ProdRedisCFNStack),
			EnableTerminationProtection: aws.Bool(true),
			StackStatus:                 cfntypes.StackStatusCreateComplete,
			CreationTime:                aws.Time(mustParseCFNTime("2025-03-15T08:00:00+00:00")),
			Description:                 aws.String("ElastiCache Redis cluster for production session storage"),
			StackId:                     aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/" + ProdRedisCFNStack + "/aaaabbbb-cccc-dddd-eeee-ffffffffffff"),
			RoleARN:                     aws.String(prodCIDeployRoleARN),
			Tags: []cfntypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("production")},
				{Key: aws.String("Service"), Value: aws.String("sessions")},
			},
		},
		// EFS prod-app-data CFN stack — required for efs→cfn related-panel pivot.
		// The prod-efs-app-data filesystem carries the aws:cloudformation:stack-name tag
		// pointing to ProdEFSCFNStackName so checkEFSCFN resolves a non-zero count.
		{
			StackName:                   aws.String(ProdEFSCFNStackName),
			EnableTerminationProtection: aws.Bool(true),
			StackStatus:                 cfntypes.StackStatusCreateComplete,
			CreationTime:                aws.Time(mustParseCFNTime("2025-02-01T10:00:00+00:00")),
			Description:                 aws.String("EFS filesystem for production app data shared storage"),
			StackId:                     aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/" + ProdEFSCFNStackName + "/bbbbcccc-dddd-eeee-ffff-000000000001"),
			RoleARN:                     aws.String(prodCIDeployRoleARN),
			Tags: []cfntypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("production")},
				{Key: aws.String("Service"), Value: aws.String("storage")},
			},
		},
		// Elastic Beanstalk auto-generated stack for acme-prod-api — required
		// for eb:cfn related-panel pivot. checkEbCFN matches stack names with
		// the "awseb-{envID}" prefix; e-acmeprodapi is acme-prod-api's
		// EnvironmentId (eb.go).
		{
			StackName:                   aws.String("awseb-e-acmeprodapi-stack"),
			EnableTerminationProtection: aws.Bool(true),
			StackStatus:                 cfntypes.StackStatusUpdateComplete,
			CreationTime:                aws.Time(mustParseCFNTime("2025-01-10T09:00:00+00:00")),
			Description:                 aws.String("Elastic Beanstalk-managed stack for acme-prod-api"),
			StackId:                     aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/awseb-e-acmeprodapi-stack/cccc5555-dddd-6666-eeee-777777777777"),
			RoleARN:                     aws.String(prodCIDeployRoleARN),
		},
		// DescribeStacks still reports a recently torn-down stack for a
		// retention window after deletion.
		{
			StackName:                   aws.String("acme-decommissioned-poc"),
			EnableTerminationProtection: aws.Bool(true),
			StackStatus:                 cfntypes.StackStatusDeleteComplete,
			StackStatusReason:           aws.String("User Initiated"),
			CreationTime:                aws.Time(mustParseCFNTime("2025-08-01T10:00:00+00:00")),
			LastUpdatedTime:             aws.Time(mustParseCFNTime("2026-04-01T09:00:00+00:00")),
			Description:                 aws.String("Decommissioned proof-of-concept environment"),
			StackId:                     aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-decommissioned-poc/dddd6666-eeee-7777-ffff-888888888888"),
			RoleARN:                     aws.String(prodCIDeployRoleARN),
		},
		{
			StackName:                   aws.String("acme-experimental-queue"),
			EnableTerminationProtection: aws.Bool(true),
			StackStatus:                 cfntypes.StackStatusCreateFailed,
			StackStatusReason:           aws.String("The following resource(s) failed to create: [QueuePolicy]. Insufficient permissions."),
			CreationTime:                aws.Time(mustParseCFNTime("2026-04-26T09:00:00+00:00")),
			LastUpdatedTime:             aws.Time(mustParseCFNTime("2026-04-26T09:12:00+00:00")),
			Description:                 aws.String("Experimental SQS queue stack — initial create failed"),
			StackId:                     aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-experimental-queue/eeee7777-ffff-8888-9999-aaaaaaaaaaaa"),
			RoleARN:                     aws.String(prodCIDeployRoleARN),
			Tags: []cfntypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("dev")},
			},
		},
		{
			StackName:                   aws.String(CFNTerminationProtectionOff),
			EnableTerminationProtection: aws.Bool(false),
			StackStatus:                 cfntypes.StackStatusUpdateRollbackFailed,
			StackStatusReason:           aws.String("The following resource(s) failed to update: [BillingTable]."),
			CreationTime:                aws.Time(mustParseCFNTime("2025-10-02T08:30:00+00:00")),
			LastUpdatedTime:             aws.Time(mustParseCFNTime("2026-04-14T16:05:00+00:00")),
			Description:                 aws.String("Billing API: Lambda functions, API Gateway and DynamoDB tables"),
			StackId:                     aws.String(cfnBillingStackID),
			RoleARN:                     aws.String(prodCIDeployRoleARN),
			Tags: []cfntypes.Tag{
				{Key: aws.String("Environment"), Value: aws.String("production")},
				{Key: aws.String("Team"), Value: aws.String("payments")},
			},
		},
		{
			StackName:                   aws.String(CFNDeletingStack),
			EnableTerminationProtection: aws.Bool(false),
			StackStatus:                 cfntypes.StackStatusDeleteInProgress,
			StackStatusReason:           aws.String("User Initiated"),
			CreationTime:                aws.Time(mustParseCFNTime("2025-06-11T13:00:00+00:00")),
			LastUpdatedTime:             aws.Time(mustParseCFNTime("2026-04-27T10:40:00+00:00")),
			Description:                 aws.String("Nightly batch workers, replaced by the Step Functions pipeline"),
			StackId:                     aws.String("arn:aws:cloudformation:us-east-1:123456789012:stack/acme-retired-batch/0000aaaa-1111-bbbb-2222-cccc3333dddd"),
			RoleARN:                     aws.String(prodCIDeployRoleARN),
		},
		{
			StackName:                   aws.String(CFNDeleteFailedStack),
			EnableTerminationProtection: aws.Bool(false),
			StackStatus:                 cfntypes.StackStatusDeleteFailed,
			StackStatusReason:           aws.String("The following resource(s) failed to delete: [ArtifactBucket]."),
			CreationTime:                aws.Time(mustParseCFNTime("2025-03-04T09:00:00+00:00")),
			LastUpdatedTime:             aws.Time(mustParseCFNTime("2026-04-20T15:22:00+00:00")),
			Description:                 aws.String("Legacy ETL jobs and their artifact bucket"),
			StackId:                     aws.String(cfnRetiredETLStackID),
			RoleARN:                     aws.String(prodCIDeployRoleARN),
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
			stackEvent("acme-vpc-stack", cfnVPCStackID, "evt-002", "2026-03-20T10:02:00+00:00", "", "", "", cfntypes.ResourceStatusUpdateComplete, ""),
			stackEvent("acme-vpc-stack", cfnVPCStackID, "evt-001", "2026-03-20T10:00:00+00:00", "", "", "", cfntypes.ResourceStatusUpdateInProgress, "User Initiated"),
			stackEvent("acme-vpc-stack", cfnVPCStackID, "evt-006", "2026-02-11T09:20:00+00:00", "", "", "", cfntypes.ResourceStatus("UPDATE_ROLLBACK_COMPLETE"), ""),
			stackEvent("acme-vpc-stack", cfnVPCStackID, "evt-005", "2026-02-11T09:15:00+00:00", "", "", "", cfntypes.ResourceStatus("UPDATE_ROLLBACK_IN_PROGRESS"), "The following resource(s) failed to update: [PublicSubnet1]."),
			stackEvent("acme-vpc-stack", cfnVPCStackID, "evt-004", "2026-02-11T09:12:00+00:00", "PublicSubnet1", "AWS::EC2::Subnet", "subnet-0aaa111111111111a", cfntypes.ResourceStatusUpdateFailed, "Resource handler returned message: \"The CIDR '10.0.0.0/23' conflicts with another subnet\" (HandlerErrorCode: AlreadyExists)"),
			stackEvent("acme-vpc-stack", cfnVPCStackID, "evt-003", "2026-02-11T09:10:00+00:00", "", "", "", cfntypes.ResourceStatusUpdateInProgress, "User Initiated"),
		},
		CFNTerminationProtectionOff: {
			stackEvent(CFNTerminationProtectionOff, cfnBillingStackID, "evt-billing-005", "2026-04-14T16:05:00+00:00", "", "", "", cfntypes.ResourceStatus("UPDATE_ROLLBACK_FAILED"), "The following resource(s) failed to update: [BillingTable]."),
			stackEvent(CFNTerminationProtectionOff, cfnBillingStackID, "evt-billing-004", "2026-04-14T16:03:00+00:00", "BillingTable", "AWS::DynamoDB::Table", "acme-billing-invoices", cfntypes.ResourceStatusUpdateFailed, "Resource handler returned message: \"Subscriber limit exceeded: Provisioned throughput decreases are limited within a given UTC day\""),
			stackEvent(CFNTerminationProtectionOff, cfnBillingStackID, "evt-billing-003", "2026-04-14T16:01:00+00:00", "", "", "", cfntypes.ResourceStatus("UPDATE_ROLLBACK_IN_PROGRESS"), "The following resource(s) failed to update: [BillingFunction]."),
			stackEvent(CFNTerminationProtectionOff, cfnBillingStackID, "evt-billing-002", "2026-04-14T15:58:00+00:00", "BillingFunction", "AWS::Lambda::Function", "acme-billing-api-BillingFunction-4D5E6F", cfntypes.ResourceStatusUpdateFailed, "Resource handler returned message: \"Unzipped size must be smaller than 262144000 bytes\""),
			stackEvent(CFNTerminationProtectionOff, cfnBillingStackID, "evt-billing-001", "2026-04-14T15:55:00+00:00", "", "", "", cfntypes.ResourceStatusUpdateInProgress, "User Initiated"),
		},
		CFNDeleteFailedStack: {
			stackEvent(CFNDeleteFailedStack, cfnRetiredETLStackID, "evt-etl-003", "2026-04-20T15:22:00+00:00", "", "", "", cfntypes.ResourceStatusDeleteFailed, "The following resource(s) failed to delete: [ArtifactBucket]."),
			stackEvent(CFNDeleteFailedStack, cfnRetiredETLStackID, "evt-etl-002", "2026-04-20T15:21:00+00:00", "ArtifactBucket", "AWS::S3::Bucket", "acme-retired-etl-artifactbucket-1a2b3c", cfntypes.ResourceStatusDeleteFailed, "The bucket you tried to delete is not empty"),
			stackEvent(CFNDeleteFailedStack, cfnRetiredETLStackID, "evt-etl-001", "2026-04-20T15:18:00+00:00", "", "", "", cfntypes.ResourceStatusDeleteInProgress, "User Initiated"),
		},
		// acme-search-stack events — required so opensearch→cfn drill lands
		// on a non-empty cfn_events child view. Without these, the cfn→cfn_events
		// Children[Key="enter"] navigation from any opensearch→cfn pivot
		// fetches DescribeStackEvents and gets nothing back.
		OpenSearchCFNStackName: minimalCreateEventSequence(OpenSearchCFNStackName, "2026-02-10T09:00:00+00:00", "SearchDomain", "AWS::OpenSearchService::Domain"),
		// All other graph-root CFN stacks — required so every parent→cfn drill
		// lands on a non-empty cfn_events child view.
		S3CFNStackName:         minimalCreateEventSequence(S3CFNStackName, "2025-01-10T10:00:00+00:00", "DemoBucket", "AWS::S3::Bucket"),
		ProdRedisCFNStack:      minimalCreateEventSequence(ProdRedisCFNStack, "2025-07-04T12:00:00+00:00", "RedisCluster", "AWS::ElastiCache::ReplicationGroup"),
		ProdEFSCFNStackName:    minimalCreateEventSequence(ProdEFSCFNStackName, "2025-09-01T08:00:00+00:00", "AppDataFS", "AWS::EFS::FileSystem"),
		"acme-warehouse-stack": minimalCreateEventSequence("acme-warehouse-stack", "2025-11-12T15:00:00+00:00", "WarehouseCluster", "AWS::Redshift::Cluster"),
		"acme-reporting-stack": minimalCreateEventSequence("acme-reporting-stack", "2025-11-20T15:00:00+00:00", "ReportingCluster", "AWS::Redshift::Cluster"),
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
			// FlowLogsBucket — required for the cfn:s3 related-panel pivot
			// (checkCfnS3 → ListStackResources → AWS::S3::Bucket).
			{
				LogicalResourceId:    aws.String("FlowLogsBucket"),
				PhysicalResourceId:   aws.String(LogsBucketName),
				ResourceType:         aws.String("AWS::S3::Bucket"),
				ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
				LastUpdatedTimestamp: aws.Time(time.Date(2024, 10, 15, 9, 10, 0, 0, time.UTC)),
			},
			// FlowLogsRule — required for the cfn:eb-rule related-panel pivot
			// (checkCfnEBRule → ListStackResources → AWS::Events::Rule). The
			// PhysicalResourceId of an Events::Rule is the rule name, matching
			// the nightly-db-backup fixture (eventbridge.go).
			{
				LogicalResourceId:    aws.String("FlowLogsRule"),
				PhysicalResourceId:   aws.String("nightly-db-backup"),
				ResourceType:         aws.String("AWS::Events::Rule"),
				ResourceStatus:       cfntypes.ResourceStatusCreateComplete,
				LastUpdatedTimestamp: aws.Time(time.Date(2024, 10, 15, 9, 12, 0, 0, time.UTC)),
			},
		},
	}

	templateBodies := map[string]string{
		// Matches acme-vpc-stack's ListStackResources fixture above (VPC,
		// PublicSubnet1, FlowLogsBucket, FlowLogsRule).
		"acme-vpc-stack": `{
  "AWSTemplateFormatVersion": "2010-09-09",
  "Description": "Core VPC networking stack for Acme Corp production",
  "Parameters": {
    "VpcCidr": {"Type": "String", "Default": "10.0.0.0/16"},
    "Environment": {"Type": "String"}
  },
  "Resources": {
    "VPC": {
      "Type": "AWS::EC2::VPC",
      "Properties": {"CidrBlock": {"Ref": "VpcCidr"}}
    },
    "PublicSubnet1": {
      "Type": "AWS::EC2::Subnet",
      "Properties": {"VpcId": {"Ref": "VPC"}, "CidrBlock": "10.0.0.0/24"}
    },
    "FlowLogsBucket": {
      "Type": "AWS::S3::Bucket"
    },
    "FlowLogsRule": {
      "Type": "AWS::Events::Rule",
      "Properties": {"ScheduleExpression": "cron(0 3 * * ? *)"}
    }
  },
  "Outputs": {
    "VpcId": {"Value": {"Ref": "VPC"}, "Description": "Production VPC ID"}
  }
}`,
	}

	return &CFNFixtures{
		Stacks:         stacks,
		StackEvents:    stackEvents,
		StackResources: stackResources,
		TemplateBodies: templateBodies,
	}
})

func NewCFNFixtures() *CFNFixtures {
	return sharedCFNFixtures()
}

// Stacks carrying the cfn posture findings. Each names the ONE demo stack
// that carries its finding; every other non-nested stack is set to the
// healthy value for that condition.
const (
	// CFNTerminationProtectionOff — a live top-level stack that can be
	// deleted in one call; its update failed and so did the rollback, so it
	// is also the failed-live stack whose posture stands beside its state.
	CFNTerminationProtectionOff = "acme-billing-api"
	CFNDeletingStack            = "acme-retired-batch"
	CFNDeleteFailedStack        = "acme-retired-etl"

	cfnVPCStackID        = "arn:aws:cloudformation:us-east-1:123456789012:stack/acme-vpc-stack/11111111-1111-1111-1111-111111111111"
	cfnBillingStackID    = "arn:aws:cloudformation:us-east-1:123456789012:stack/acme-billing-api/ffff8888-aaaa-9999-bbbb-cccccccccccc"
	cfnRetiredETLStackID = "arn:aws:cloudformation:us-east-1:123456789012:stack/acme-retired-etl/1111bbbb-2222-cccc-3333-dddd4444eeee"
	// CFNOutputSecret — a stack output whose value looks like a credential.
	CFNOutputSecret = "acme-rds-aurora"
)

func init() {
	Register(Pin{ShortName: "cfn", Rows: 19, Issues: 8})
}
