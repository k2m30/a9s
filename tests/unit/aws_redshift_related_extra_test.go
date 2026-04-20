// aws_redshift_related_extra_test.go covers Redshift related checkers:
// checkRedshiftSG, checkRedshiftVPC, checkRedshiftRole, checkRedshiftKMS,
// checkRedshiftSecrets, checkRedshiftLogs, checkRedshiftS3, checkRedshiftSubnet.
// Checkers that require live Redshift API calls use inline fake clients.
package unit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Inline fake Redshift client for DescribeLoggingStatus + DescribeClusterSubnetGroups.
// Implements RedshiftAPI (DescribeClusters + DescribeLoggingStatus + DescribeClusterSubnetGroups).
// ---------------------------------------------------------------------------

type fakeRedshiftClient struct {
	clustersOut  *redshift.DescribeClustersOutput
	clustersErr  error
	loggingOut   *redshift.DescribeLoggingStatusOutput
	loggingErr   error
	subnetOut    *redshift.DescribeClusterSubnetGroupsOutput
	subnetErr    error
}

func (f *fakeRedshiftClient) DescribeClusters(_ context.Context, _ *redshift.DescribeClustersInput, _ ...func(*redshift.Options)) (*redshift.DescribeClustersOutput, error) {
	return f.clustersOut, f.clustersErr
}

func (f *fakeRedshiftClient) DescribeLoggingStatus(_ context.Context, _ *redshift.DescribeLoggingStatusInput, _ ...func(*redshift.Options)) (*redshift.DescribeLoggingStatusOutput, error) {
	return f.loggingOut, f.loggingErr
}

func (f *fakeRedshiftClient) DescribeClusterSubnetGroups(_ context.Context, _ *redshift.DescribeClusterSubnetGroupsInput, _ ...func(*redshift.Options)) (*redshift.DescribeClusterSubnetGroupsOutput, error) {
	return f.subnetOut, f.subnetErr
}

// fakeRedshiftServiceClients builds a *awsclient.ServiceClients with only Redshift populated.
func fakeRedshiftServiceClients(rs *fakeRedshiftClient) *awsclient.ServiceClients {
	return &awsclient.ServiceClients{Redshift: rs}
}

// ---------------------------------------------------------------------------
// checkRedshiftSG — VpcSecurityGroups from RawStruct (Pattern F)
// ---------------------------------------------------------------------------

func TestRelated_Redshift_SG_Found(t *testing.T) {
	source := resource.Resource{
		ID:   "analytics-prod",
		Name: "analytics-prod",
		RawStruct: redshifttypes.Cluster{
			ClusterIdentifier: aws.String("analytics-prod"),
			VpcSecurityGroups: []redshifttypes.VpcSecurityGroupMembership{
				{VpcSecurityGroupId: aws.String("sg-aaa111")},
				{VpcSecurityGroupId: aws.String("sg-bbb222")},
			},
		},
	}

	checker := redshiftCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	found := map[string]bool{}
	for _, id := range result.ResourceIDs {
		found[id] = true
	}
	if !found["sg-aaa111"] || !found["sg-bbb222"] {
		t.Errorf("ResourceIDs = %v, want sg-aaa111 and sg-bbb222", result.ResourceIDs)
	}
}

func TestRelated_Redshift_SG_Empty(t *testing.T) {
	source := resource.Resource{
		ID:   "analytics-prod",
		Name: "analytics-prod",
		RawStruct: redshifttypes.Cluster{
			ClusterIdentifier: aws.String("analytics-prod"),
			VpcSecurityGroups: []redshifttypes.VpcSecurityGroupMembership{},
		},
	}

	checker := redshiftCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no SGs)", result.Count)
	}
}

func TestRelated_Redshift_SG_InvalidRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "analytics-prod",
		RawStruct: "not-a-cluster",
	}

	checker := redshiftCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkRedshiftVPC — VpcId from RawStruct (Pattern F)
// ---------------------------------------------------------------------------

func TestRelated_Redshift_VPC_Found(t *testing.T) {
	source := resource.Resource{
		ID:   "analytics-prod",
		Name: "analytics-prod",
		RawStruct: redshifttypes.Cluster{
			ClusterIdentifier: aws.String("analytics-prod"),
			VpcId:             aws.String("vpc-0abc1234def56789a"),
		},
	}

	checker := redshiftCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "vpc-0abc1234def56789a" {
		t.Errorf("ResourceIDs = %v, want [vpc-0abc1234def56789a]", result.ResourceIDs)
	}
}

func TestRelated_Redshift_VPC_NilVpcId(t *testing.T) {
	source := resource.Resource{
		ID:   "analytics-prod",
		Name: "analytics-prod",
		RawStruct: redshifttypes.Cluster{
			ClusterIdentifier: aws.String("analytics-prod"),
			VpcId:             nil,
		},
	}

	checker := redshiftCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil VpcId)", result.Count)
	}
}

func TestRelated_Redshift_VPC_InvalidRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "analytics-prod",
		RawStruct: "not-a-cluster",
	}

	checker := redshiftCheckerByTarget(t, "vpc")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	// assertStruct fails → Count:-1
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkRedshiftRole — IamRoles ARN last-segment extraction (Pattern F)
// ---------------------------------------------------------------------------

func TestRelated_Redshift_Role_FoundMultiple(t *testing.T) {
	source := resource.Resource{
		ID:   "analytics-prod",
		Name: "analytics-prod",
		RawStruct: redshifttypes.Cluster{
			ClusterIdentifier: aws.String("analytics-prod"),
			IamRoles: []redshifttypes.ClusterIamRole{
				{IamRoleArn: aws.String("arn:aws:iam::123456789012:role/RedshiftS3ReadRole")},
				{IamRoleArn: aws.String("arn:aws:iam::123456789012:role/RedshiftGlueRole")},
			},
		},
	}

	checker := redshiftCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	found := map[string]bool{}
	for _, id := range result.ResourceIDs {
		found[id] = true
	}
	if !found["RedshiftS3ReadRole"] || !found["RedshiftGlueRole"] {
		t.Errorf("ResourceIDs = %v, want [RedshiftS3ReadRole, RedshiftGlueRole]", result.ResourceIDs)
	}
}

func TestRelated_Redshift_Role_Empty(t *testing.T) {
	source := resource.Resource{
		ID:   "analytics-prod",
		Name: "analytics-prod",
		RawStruct: redshifttypes.Cluster{
			ClusterIdentifier: aws.String("analytics-prod"),
			IamRoles:          []redshifttypes.ClusterIamRole{},
		},
	}

	checker := redshiftCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no IAM roles)", result.Count)
	}
}

func TestRelated_Redshift_Role_InvalidRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "analytics-prod",
		RawStruct: "not-a-cluster",
	}

	checker := redshiftCheckerByTarget(t, "role")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkRedshiftKMS — KmsKeyId ARN last-segment (Pattern F)
// ---------------------------------------------------------------------------

func TestRelated_Redshift_KMS_FoundFullARN(t *testing.T) {
	source := resource.Resource{
		ID:   "analytics-prod",
		Name: "analytics-prod",
		RawStruct: redshifttypes.Cluster{
			ClusterIdentifier: aws.String("analytics-prod"),
			KmsKeyId:          aws.String("arn:aws:kms:us-east-1:123456789012:key/a1b2c3d4-1234-5678-abcd-111111111111"),
		},
	}

	checker := redshiftCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "a1b2c3d4-1234-5678-abcd-111111111111" {
		t.Errorf("ResourceIDs = %v, want [a1b2c3d4-1234-5678-abcd-111111111111]", result.ResourceIDs)
	}
}

func TestRelated_Redshift_KMS_NilKeyId(t *testing.T) {
	source := resource.Resource{
		ID:   "analytics-prod",
		Name: "analytics-prod",
		RawStruct: redshifttypes.Cluster{
			ClusterIdentifier: aws.String("analytics-prod"),
			KmsKeyId:          nil,
		},
	}

	checker := redshiftCheckerByTarget(t, "kms")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil KmsKeyId)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkRedshiftSecrets — MasterPasswordSecretArn → secrets cache by ARN
// ---------------------------------------------------------------------------

func TestRelated_Redshift_Secrets_FoundInCache(t *testing.T) {
	const secretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret:redshift-analytics-prod-AbCdEf"
	secretRes := resource.Resource{
		ID:   "redshift-analytics-prod",
		Name: "redshift-analytics-prod",
		Fields: map[string]string{
			"arn": secretARN,
		},
	}
	cache := resource.ResourceCache{
		"secrets": resource.ResourceCacheEntry{Resources: []resource.Resource{secretRes}},
	}
	source := resource.Resource{
		ID:   "analytics-prod",
		Name: "analytics-prod",
		RawStruct: redshifttypes.Cluster{
			ClusterIdentifier:       aws.String("analytics-prod"),
			MasterPasswordSecretArn: aws.String(secretARN),
		},
	}

	checker := redshiftCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, source, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "redshift-analytics-prod" {
		t.Errorf("ResourceIDs = %v, want [redshift-analytics-prod]", result.ResourceIDs)
	}
}

func TestRelated_Redshift_Secrets_NoSecretARN(t *testing.T) {
	source := resource.Resource{
		ID:   "analytics-prod",
		Name: "analytics-prod",
		RawStruct: redshifttypes.Cluster{
			ClusterIdentifier:       aws.String("analytics-prod"),
			MasterPasswordSecretArn: nil,
		},
	}

	checker := redshiftCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no secret ARN)", result.Count)
	}
}

func TestRelated_Redshift_Secrets_InvalidRawStruct(t *testing.T) {
	source := resource.Resource{
		ID:        "analytics-prod",
		RawStruct: "not-a-cluster",
	}

	checker := redshiftCheckerByTarget(t, "secrets")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (wrong RawStruct type)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkRedshiftLogs — DescribeLoggingStatus: CloudWatch path yields log group
// checkRedshiftS3 — DescribeLoggingStatus: BucketName when logging to S3
//
// Both require a live Redshift API call (DescribeLoggingStatus). Without a
// fake Redshift client those are -1 (nil clients). The nil-clients branch is
// meaningful because it distinguishes "client not configured" from "no logs".
// ---------------------------------------------------------------------------

func TestRelated_Redshift_Logs_NilClients(t *testing.T) {
	source := resource.Resource{
		ID:   "analytics-prod",
		Name: "analytics-prod",
	}

	checker := redshiftCheckerByTarget(t, "logs")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients → cannot call DescribeLoggingStatus)", result.Count)
	}
}

func TestRelated_Redshift_S3_NilClients(t *testing.T) {
	source := resource.Resource{
		ID:   "analytics-prod",
		Name: "analytics-prod",
	}

	checker := redshiftCheckerByTarget(t, "s3")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients → cannot call DescribeLoggingStatus)", result.Count)
	}
}

func TestRelated_Redshift_Subnet_NilClients(t *testing.T) {
	source := resource.Resource{
		ID:   "analytics-prod",
		Name: "analytics-prod",
		RawStruct: redshifttypes.Cluster{
			ClusterIdentifier:      aws.String("analytics-prod"),
			ClusterSubnetGroupName: aws.String("my-subnet-group"),
		},
	}

	checker := redshiftCheckerByTarget(t, "subnet")
	result := checker(context.Background(), nil, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil clients → cannot call DescribeClusterSubnetGroups)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkRedshiftLogs — fake client: CloudWatch logging path
// ---------------------------------------------------------------------------

// TestRelated_Redshift_Logs_CloudWatchEnabled: DescribeLoggingStatus returns
// LoggingEnabled=true, LogDestinationType=cloudwatch → log group prefix returned.
func TestRelated_Redshift_Logs_CloudWatchEnabled(t *testing.T) {
	clients := fakeRedshiftServiceClients(&fakeRedshiftClient{
		loggingOut: &redshift.DescribeLoggingStatusOutput{
			LoggingEnabled:     aws.Bool(true),
			LogDestinationType: redshifttypes.LogDestinationTypeCloudwatch,
		},
	})

	source := resource.Resource{ID: "analytics-prod", Name: "analytics-prod"}
	checker := redshiftCheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "/aws/redshift/cluster/analytics-prod" {
		t.Errorf("ResourceIDs = %v, want [/aws/redshift/cluster/analytics-prod]", result.ResourceIDs)
	}
}

// TestRelated_Redshift_Logs_LoggingDisabled: LoggingEnabled=false → Count: 0.
func TestRelated_Redshift_Logs_LoggingDisabled(t *testing.T) {
	clients := fakeRedshiftServiceClients(&fakeRedshiftClient{
		loggingOut: &redshift.DescribeLoggingStatusOutput{
			LoggingEnabled: aws.Bool(false),
		},
	})

	source := resource.Resource{ID: "analytics-prod", Name: "analytics-prod"}
	checker := redshiftCheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (logging disabled)", result.Count)
	}
}

// TestRelated_Redshift_Logs_S3Destination: LoggingEnabled=true but
// LogDestinationType=s3 → Count: 0 (no CW log group, audit goes to S3).
func TestRelated_Redshift_Logs_S3Destination(t *testing.T) {
	clients := fakeRedshiftServiceClients(&fakeRedshiftClient{
		loggingOut: &redshift.DescribeLoggingStatusOutput{
			LoggingEnabled:     aws.Bool(true),
			LogDestinationType: redshifttypes.LogDestinationTypeS3,
		},
	})

	source := resource.Resource{ID: "analytics-prod", Name: "analytics-prod"}
	checker := redshiftCheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (S3 destination, not CloudWatch)", result.Count)
	}
}

// TestRelated_Redshift_Logs_APIError: DescribeLoggingStatus returns error → Count: -1.
func TestRelated_Redshift_Logs_APIError(t *testing.T) {
	clients := fakeRedshiftServiceClients(&fakeRedshiftClient{
		loggingErr: errors.New("redshift: DescribeLoggingStatus throttled"),
	})

	source := resource.Resource{ID: "analytics-prod", Name: "analytics-prod"}
	checker := redshiftCheckerByTarget(t, "logs")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (API error)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkRedshiftS3 — fake client: S3 bucket logging path
// ---------------------------------------------------------------------------

// TestRelated_Redshift_S3_BucketNameSet: LoggingEnabled=true, BucketName set
// → bucket ID returned.
func TestRelated_Redshift_S3_BucketNameSet(t *testing.T) {
	clients := fakeRedshiftServiceClients(&fakeRedshiftClient{
		loggingOut: &redshift.DescribeLoggingStatusOutput{
			LoggingEnabled: aws.Bool(true),
			BucketName:     aws.String("my-redshift-audit-logs"),
		},
	})

	source := resource.Resource{ID: "analytics-prod", Name: "analytics-prod"}
	checker := redshiftCheckerByTarget(t, "s3")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) != 1 || result.ResourceIDs[0] != "my-redshift-audit-logs" {
		t.Errorf("ResourceIDs = %v, want [my-redshift-audit-logs]", result.ResourceIDs)
	}
}

// TestRelated_Redshift_S3_LoggingDisabled: LoggingEnabled=false → Count: 0.
func TestRelated_Redshift_S3_LoggingDisabled(t *testing.T) {
	clients := fakeRedshiftServiceClients(&fakeRedshiftClient{
		loggingOut: &redshift.DescribeLoggingStatusOutput{
			LoggingEnabled: aws.Bool(false),
		},
	})

	source := resource.Resource{ID: "analytics-prod", Name: "analytics-prod"}
	checker := redshiftCheckerByTarget(t, "s3")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (logging disabled)", result.Count)
	}
}

// TestRelated_Redshift_S3_NilBucketName: LoggingEnabled=true but BucketName=nil
// → Count: 0 (no S3 bucket configured).
func TestRelated_Redshift_S3_NilBucketName(t *testing.T) {
	clients := fakeRedshiftServiceClients(&fakeRedshiftClient{
		loggingOut: &redshift.DescribeLoggingStatusOutput{
			LoggingEnabled: aws.Bool(true),
			BucketName:     nil,
		},
	})

	source := resource.Resource{ID: "analytics-prod", Name: "analytics-prod"}
	checker := redshiftCheckerByTarget(t, "s3")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (nil BucketName)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// checkRedshiftSubnet — fake client: DescribeClusterSubnetGroups path
// ---------------------------------------------------------------------------

// TestRelated_Redshift_Subnet_SubnetsFound: DescribeClusterSubnetGroups returns
// a subnet group with two subnets → both subnet IDs returned.
func TestRelated_Redshift_Subnet_SubnetsFound(t *testing.T) {
	clients := fakeRedshiftServiceClients(&fakeRedshiftClient{
		subnetOut: &redshift.DescribeClusterSubnetGroupsOutput{
			ClusterSubnetGroups: []redshifttypes.ClusterSubnetGroup{
				{
					ClusterSubnetGroupName: aws.String("my-subnet-group"),
					Subnets: []redshifttypes.Subnet{
						{SubnetIdentifier: aws.String("subnet-aaa111")},
						{SubnetIdentifier: aws.String("subnet-bbb222")},
					},
				},
			},
		},
	})

	source := resource.Resource{
		ID:   "analytics-prod",
		Name: "analytics-prod",
		RawStruct: redshifttypes.Cluster{
			ClusterIdentifier:      aws.String("analytics-prod"),
			ClusterSubnetGroupName: aws.String("my-subnet-group"),
		},
	}
	checker := redshiftCheckerByTarget(t, "subnet")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
	found := map[string]bool{}
	for _, id := range result.ResourceIDs {
		found[id] = true
	}
	if !found["subnet-aaa111"] || !found["subnet-bbb222"] {
		t.Errorf("ResourceIDs = %v, want subnet-aaa111 and subnet-bbb222", result.ResourceIDs)
	}
}

// TestRelated_Redshift_Subnet_EmptySubnetGroup: DescribeClusterSubnetGroups returns
// group with no subnets → Count: 0.
func TestRelated_Redshift_Subnet_EmptySubnetGroup(t *testing.T) {
	clients := fakeRedshiftServiceClients(&fakeRedshiftClient{
		subnetOut: &redshift.DescribeClusterSubnetGroupsOutput{
			ClusterSubnetGroups: []redshifttypes.ClusterSubnetGroup{
				{
					ClusterSubnetGroupName: aws.String("my-subnet-group"),
					Subnets:                []redshifttypes.Subnet{},
				},
			},
		},
	})

	source := resource.Resource{
		ID:   "analytics-prod",
		Name: "analytics-prod",
		RawStruct: redshifttypes.Cluster{
			ClusterIdentifier:      aws.String("analytics-prod"),
			ClusterSubnetGroupName: aws.String("my-subnet-group"),
		},
	}
	checker := redshiftCheckerByTarget(t, "subnet")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no subnets in group)", result.Count)
	}
}

// TestRelated_Redshift_Subnet_APIError: DescribeClusterSubnetGroups returns error
// → Count: -1.
func TestRelated_Redshift_Subnet_APIError(t *testing.T) {
	clients := fakeRedshiftServiceClients(&fakeRedshiftClient{
		subnetErr: errors.New("redshift: DescribeClusterSubnetGroups throttled"),
	})

	source := resource.Resource{
		ID:   "analytics-prod",
		Name: "analytics-prod",
		RawStruct: redshifttypes.Cluster{
			ClusterIdentifier:      aws.String("analytics-prod"),
			ClusterSubnetGroupName: aws.String("my-subnet-group"),
		},
	}
	checker := redshiftCheckerByTarget(t, "subnet")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (API error)", result.Count)
	}
}

// TestRelated_Redshift_Subnet_NilSubnetGroupName: ClusterSubnetGroupName=nil
// in RawStruct → Count: -1 (can't call API without group name).
func TestRelated_Redshift_Subnet_NilSubnetGroupName(t *testing.T) {
	clients := fakeRedshiftServiceClients(&fakeRedshiftClient{})

	source := resource.Resource{
		ID:   "analytics-prod",
		Name: "analytics-prod",
		RawStruct: redshifttypes.Cluster{
			ClusterIdentifier:      aws.String("analytics-prod"),
			ClusterSubnetGroupName: nil,
		},
	}
	checker := redshiftCheckerByTarget(t, "subnet")
	result := checker(context.Background(), clients, source, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 (nil ClusterSubnetGroupName)", result.Count)
	}
}
