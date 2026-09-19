package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

type fakeRedshiftClient struct {
	loggingOutput *redshift.DescribeLoggingStatusOutput
	loggingErr    error
	subnetOutput  *redshift.DescribeClusterSubnetGroupsOutput
	subnetErr     error
}

func (f *fakeRedshiftClient) DescribeClusters(
	_ context.Context,
	_ *redshift.DescribeClustersInput,
	_ ...func(*redshift.Options),
) (*redshift.DescribeClustersOutput, error) {
	return &redshift.DescribeClustersOutput{}, nil
}

func (f *fakeRedshiftClient) DescribeLoggingStatus(
	_ context.Context,
	_ *redshift.DescribeLoggingStatusInput,
	_ ...func(*redshift.Options),
) (*redshift.DescribeLoggingStatusOutput, error) {
	return f.loggingOutput, f.loggingErr
}

func (f *fakeRedshiftClient) DescribeClusterSubnetGroups(
	_ context.Context,
	_ *redshift.DescribeClusterSubnetGroupsInput,
	_ ...func(*redshift.Options),
) (*redshift.DescribeClusterSubnetGroupsOutput, error) {
	return f.subnetOutput, f.subnetErr
}

func redshiftCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("redshift") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("redshift related checker for %q is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("redshift related checker for %q not found", target)
	return nil
}

func redshiftSrcResource(cluster redshifttypes.Cluster) resource.Resource {
	id := ""
	if cluster.ClusterIdentifier != nil {
		id = *cluster.ClusterIdentifier
	}
	return resource.Resource{
		ID:        id,
		Name:      id,
		Fields:    map[string]string{"cluster_id": id},
		RawStruct: cluster,
	}
}

func redshiftFixtureWarehouse(t *testing.T) redshifttypes.Cluster {
	t.Helper()
	for _, c := range fixtures.NewRedshiftFixtures().Clusters {
		if c.ClusterIdentifier != nil && *c.ClusterIdentifier == fixtures.AcmeWarehouseID {
			return c
		}
	}
	t.Fatal("acme-warehouse fixture not found")
	return redshifttypes.Cluster{}
}

func serviceClientsWithRedshift(fake *fakeRedshiftClient) *awsclient.ServiceClients {
	return &awsclient.ServiceClients{
		Redshift: fake,
	}
}

func containsID(ids []string, target string) bool {
	for _, id := range ids {
		if id == target {
			return true
		}
	}
	return false
}

func redshiftAlarmResource(alarmName, dimName, dimValue string) resource.Resource {
	return resource.Resource{
		ID:   alarmName,
		Name: alarmName,
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String(alarmName),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String(dimName), Value: aws.String(dimValue)},
			},
		},
	}
}

func TestRelated_Redshift_Alarm_MatchesByDimensionClusterIdentifier(t *testing.T) {
	clusterID := fixtures.AcmeWarehouseID
	alarmCache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				redshiftAlarmResource("alarm-cpu-1", "ClusterIdentifier", clusterID),
				redshiftAlarmResource("alarm-disk-2", "ClusterIdentifier", clusterID),
				redshiftAlarmResource("alarm-other", "ClusterIdentifier", "other-cluster"),
			},
		},
	}

	checker := redshiftCheckerByTarget(t, "alarm")
	src := redshiftSrcResource(redshiftFixtureWarehouse(t))
	result := checker(context.Background(), nil, src, alarmCache)

	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2", result.Count())
	}
	for _, want := range []string{"alarm-cpu-1", "alarm-disk-2"} {
		if !containsID(result.ResourceIDs(), want) {
			t.Errorf("ResourceIDs = %v, want to contain %q", result.ResourceIDs(), want)
		}
	}
	if containsID(result.ResourceIDs(), "alarm-other") {
		t.Errorf("ResourceIDs = %v, must NOT contain %q (wrong cluster)", result.ResourceIDs(), "alarm-other")
	}
}

func TestRelated_Redshift_Alarm_NoMatchReturnsZero(t *testing.T) {
	alarmCache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				redshiftAlarmResource("alarm-other", "ClusterIdentifier", "some-other-cluster"),
			},
		},
	}

	checker := redshiftCheckerByTarget(t, "alarm")
	src := redshiftSrcResource(redshiftFixtureWarehouse(t))
	result := checker(context.Background(), nil, src, alarmCache)

	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0", result.Count())
	}
}

func TestRelated_Redshift_CFN_MatchesByStackNameTag(t *testing.T) {
	stackName := "acme-warehouse-stack"
	cfnCache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: stackName, Name: stackName, Fields: map[string]string{"stack_name": stackName}},
				{ID: "unrelated-stack", Name: "unrelated-stack", Fields: map[string]string{"stack_name": "unrelated-stack"}},
			},
		},
	}

	checker := redshiftCheckerByTarget(t, "cfn")
	src := redshiftSrcResource(redshiftFixtureWarehouse(t))
	result := checker(context.Background(), nil, src, cfnCache)

	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if !containsID(result.ResourceIDs(), stackName) {
		t.Errorf("ResourceIDs = %v, want to contain %q", result.ResourceIDs(), stackName)
	}
}

func TestRelated_Redshift_CFN_NoTagReturnsZero(t *testing.T) {
	cluster := redshifttypes.Cluster{
		ClusterIdentifier: aws.String("no-cfn-cluster"),
		ClusterStatus:     aws.String("available"),
		Tags:              []redshifttypes.Tag{},
	}
	cfnCache := resource.ResourceCache{
		"cfn": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "some-stack", Name: "some-stack"},
			},
		},
	}

	checker := redshiftCheckerByTarget(t, "cfn")
	src := redshiftSrcResource(cluster)
	result := checker(context.Background(), nil, src, cfnCache)

	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no CFN tag)", result.Count())
	}
}

func TestRelated_Redshift_KMS_ExtractsBareKeyID(t *testing.T) {
	checker := redshiftCheckerByTarget(t, "kms")
	src := redshiftSrcResource(redshiftFixtureWarehouse(t))
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if !containsID(result.ResourceIDs(), fixtures.RedshiftKMSKeyID1) {
		t.Errorf("ResourceIDs = %v, want to contain %q (bare key ID)", result.ResourceIDs(), fixtures.RedshiftKMSKeyID1)
	}
	if containsID(result.ResourceIDs(), fixtures.RedshiftKMSKeyARN1) {
		t.Errorf("ResourceIDs must NOT contain the full ARN %q — return bare ID only", fixtures.RedshiftKMSKeyARN1)
	}
}

func TestRelated_Redshift_KMS_NoKeyReturnsZero(t *testing.T) {
	cluster := redshifttypes.Cluster{
		ClusterIdentifier: aws.String("no-kms-cluster"),
		ClusterStatus:     aws.String("available"),
		KmsKeyId:          nil,
	}

	checker := redshiftCheckerByTarget(t, "kms")
	src := redshiftSrcResource(cluster)
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no KMS key)", result.Count())
	}
}

func TestRelated_Redshift_Logs_CloudWatchMultiExport(t *testing.T) {
	fake := &fakeRedshiftClient{
		loggingOutput: &redshift.DescribeLoggingStatusOutput{
			LoggingEnabled:     aws.Bool(true),
			LogDestinationType: redshifttypes.LogDestinationTypeCloudwatch,
			LogExports:         []string{"connectionlog", "userlog", "useractivitylog"},
		},
	}
	clients := serviceClientsWithRedshift(fake)

	checker := redshiftCheckerByTarget(t, "logs")
	src := redshiftSrcResource(redshiftFixtureWarehouse(t))
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if result.Count() != 3 {
		t.Errorf("Count = %d, want 3 (one per LogExport)", result.Count())
	}
	wantIDs := []string{
		"/aws/redshift/cluster/" + fixtures.AcmeWarehouseID + "/connectionlog",
		"/aws/redshift/cluster/" + fixtures.AcmeWarehouseID + "/userlog",
		"/aws/redshift/cluster/" + fixtures.AcmeWarehouseID + "/useractivitylog",
	}
	for _, want := range wantIDs {
		if !containsID(result.ResourceIDs(), want) {
			t.Errorf("ResourceIDs = %v, want to contain %q", result.ResourceIDs(), want)
		}
	}
}

func TestRelated_Redshift_Logs_S3ModeReturnsZero(t *testing.T) {
	fake := &fakeRedshiftClient{
		loggingOutput: &redshift.DescribeLoggingStatusOutput{
			LoggingEnabled:     aws.Bool(true),
			LogDestinationType: redshifttypes.LogDestinationTypeS3,
			BucketName:         aws.String(fixtures.RedshiftAuditBucket),
		},
	}
	clients := serviceClientsWithRedshift(fake)

	checker := redshiftCheckerByTarget(t, "logs")
	src := redshiftSrcResource(redshiftFixtureWarehouse(t))
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (S3 mode → no CW log groups)", result.Count())
	}
}

func TestRelated_Redshift_Logs_DisabledReturnsZero(t *testing.T) {
	fake := &fakeRedshiftClient{
		loggingOutput: &redshift.DescribeLoggingStatusOutput{
			LoggingEnabled: aws.Bool(false),
		},
	}
	clients := serviceClientsWithRedshift(fake)

	checker := redshiftCheckerByTarget(t, "logs")
	src := redshiftSrcResource(redshiftFixtureWarehouse(t))
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (logging disabled)", result.Count())
	}
}

func TestRelated_Redshift_Logs_NilClientsReturnsNegOne(t *testing.T) {
	checker := redshiftCheckerByTarget(t, "logs")
	src := redshiftSrcResource(redshiftFixtureWarehouse(t))
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients)", result.Count())
	}
}

func TestRelated_Redshift_Role_ExtractsBareRoleNames(t *testing.T) {
	checker := redshiftCheckerByTarget(t, "role")
	src := redshiftSrcResource(redshiftFixtureWarehouse(t))
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2 (two IAM roles on acme-warehouse)", result.Count())
	}
	if !containsID(result.ResourceIDs(), "redshift-copy-role") {
		t.Errorf("ResourceIDs = %v, want to contain %q", result.ResourceIDs(), "redshift-copy-role")
	}
	if !containsID(result.ResourceIDs(), "redshift-unload-role") {
		t.Errorf("ResourceIDs = %v, want to contain %q", result.ResourceIDs(), "redshift-unload-role")
	}
	for _, id := range result.ResourceIDs() {
		if len(id) >= 4 && id[:4] == "arn:" {
			t.Errorf("ResourceID %q starts with 'arn:' — checker must return bare role names", id)
		}
	}
}

func TestRelated_Redshift_Role_NoRolesReturnsZero(t *testing.T) {
	cluster := redshifttypes.Cluster{
		ClusterIdentifier: aws.String("no-roles-cluster"),
		ClusterStatus:     aws.String("available"),
		IamRoles:          []redshifttypes.ClusterIamRole{},
	}

	checker := redshiftCheckerByTarget(t, "role")
	src := redshiftSrcResource(cluster)
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no IAM roles)", result.Count())
	}
}

func TestRelated_Redshift_S3_BucketWhenS3Logging(t *testing.T) {
	fake := &fakeRedshiftClient{
		loggingOutput: &redshift.DescribeLoggingStatusOutput{
			LoggingEnabled:     aws.Bool(true),
			LogDestinationType: redshifttypes.LogDestinationTypeS3,
			BucketName:         aws.String(fixtures.RedshiftAuditBucket),
		},
	}
	clients := serviceClientsWithRedshift(fake)

	checker := redshiftCheckerByTarget(t, "s3")
	src := redshiftSrcResource(redshiftFixtureWarehouse(t))
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if !containsID(result.ResourceIDs(), fixtures.RedshiftAuditBucket) {
		t.Errorf("ResourceIDs = %v, want to contain %q", result.ResourceIDs(), fixtures.RedshiftAuditBucket)
	}
}

func TestRelated_Redshift_S3_ReturnsZeroWhenCloudWatchMode(t *testing.T) {
	fake := &fakeRedshiftClient{
		loggingOutput: &redshift.DescribeLoggingStatusOutput{
			LoggingEnabled:     aws.Bool(true),
			LogDestinationType: redshifttypes.LogDestinationTypeCloudwatch,
			LogExports:         []string{"connectionlog"},
		},
	}
	clients := serviceClientsWithRedshift(fake)

	checker := redshiftCheckerByTarget(t, "s3")
	src := redshiftSrcResource(redshiftFixtureWarehouse(t))
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (CloudWatch mode — no audit bucket)", result.Count())
	}
}

func TestRelated_Redshift_S3_LogDestinationUnsetReturnsZero(t *testing.T) {
	fake := &fakeRedshiftClient{
		loggingOutput: &redshift.DescribeLoggingStatusOutput{
			LoggingEnabled:     aws.Bool(true),
			LogDestinationType: redshifttypes.LogDestinationTypeS3,
			BucketName:         nil,
		},
	}
	clients := serviceClientsWithRedshift(fake)

	checker := redshiftCheckerByTarget(t, "s3")
	src := redshiftSrcResource(redshiftFixtureWarehouse(t))
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (BucketName nil)", result.Count())
	}
}

func TestRelated_Redshift_Secrets_MatchesByARN(t *testing.T) {
	secretID := "redshift-warehouse-secret"
	secretsCache := resource.ResourceCache{
		"secrets": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{
					ID:     secretID,
					Name:   secretID,
					Fields: map[string]string{"arn": fixtures.AcmeWarehouseSecretARN},
				},
				{
					ID:     "other-secret",
					Name:   "other-secret",
					Fields: map[string]string{"arn": "arn:aws:secretsmanager:us-east-1:123456789012:secret:unrelated"},
				},
			},
		},
	}

	checker := redshiftCheckerByTarget(t, "secrets")
	src := redshiftSrcResource(redshiftFixtureWarehouse(t))
	result := checker(context.Background(), nil, src, secretsCache)

	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	if !containsID(result.ResourceIDs(), secretID) {
		t.Errorf("ResourceIDs = %v, want to contain %q", result.ResourceIDs(), secretID)
	}
}

func TestRelated_Redshift_Secrets_NoARNReturnsZero(t *testing.T) {
	cluster := redshifttypes.Cluster{
		ClusterIdentifier:       aws.String("no-secret-cluster"),
		ClusterStatus:           aws.String("available"),
		MasterPasswordSecretArn: nil,
	}
	secretsCache := resource.ResourceCache{
		"secrets": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "some-secret", Name: "some-secret", Fields: map[string]string{"arn": "arn:aws:..."}},
			},
		},
	}

	checker := redshiftCheckerByTarget(t, "secrets")
	src := redshiftSrcResource(cluster)
	result := checker(context.Background(), nil, src, secretsCache)

	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no MasterPasswordSecretArn)", result.Count())
	}
}

func TestRelated_Redshift_SG_ExtractsTwoSGIDs(t *testing.T) {
	checker := redshiftCheckerByTarget(t, "sg")
	src := redshiftSrcResource(redshiftFixtureWarehouse(t))
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2", result.Count())
	}
	for _, want := range []string{fixtures.RedshiftWarehouseSGID1, fixtures.RedshiftWarehouseSGID2} {
		if !containsID(result.ResourceIDs(), want) {
			t.Errorf("ResourceIDs = %v, want to contain %q", result.ResourceIDs(), want)
		}
	}
}

func TestRelated_Redshift_SG_NoSGsReturnsZero(t *testing.T) {
	cluster := redshifttypes.Cluster{
		ClusterIdentifier: aws.String("no-sg-cluster"),
		ClusterStatus:     aws.String("available"),
		VpcSecurityGroups: []redshifttypes.VpcSecurityGroupMembership{},
	}

	checker := redshiftCheckerByTarget(t, "sg")
	src := redshiftSrcResource(cluster)
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no SGs)", result.Count())
	}
}

func TestRelated_Redshift_Subnet_ResolvesSubnetsViaAPI(t *testing.T) {
	fake := &fakeRedshiftClient{
		subnetOutput: &redshift.DescribeClusterSubnetGroupsOutput{
			ClusterSubnetGroups: []redshifttypes.ClusterSubnetGroup{
				{
					ClusterSubnetGroupName: aws.String(fixtures.RedshiftProdSubnetGroup),
					Subnets: []redshifttypes.Subnet{
						{SubnetIdentifier: aws.String("subnet-prod-a")},
						{SubnetIdentifier: aws.String("subnet-prod-b")},
					},
				},
			},
		},
	}
	clients := serviceClientsWithRedshift(fake)

	checker := redshiftCheckerByTarget(t, "subnet")
	src := redshiftSrcResource(redshiftFixtureWarehouse(t))
	result := checker(context.Background(), clients, src, resource.ResourceCache{})

	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if result.Count() != 2 {
		t.Errorf("Count = %d, want 2", result.Count())
	}
	for _, want := range []string{"subnet-prod-a", "subnet-prod-b"} {
		if !containsID(result.ResourceIDs(), want) {
			t.Errorf("ResourceIDs = %v, want to contain %q", result.ResourceIDs(), want)
		}
	}
}

func TestRelated_Redshift_Subnet_NilClientsReturnsNegOne(t *testing.T) {
	checker := redshiftCheckerByTarget(t, "subnet")
	src := redshiftSrcResource(redshiftFixtureWarehouse(t))
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.State() != domain.RelatedUnknown {
		t.Errorf("Count = %d, want -1 (nil clients → cannot call API)", result.Count())
	}
}

func TestRelated_Redshift_VPC_ReturnsVPCID(t *testing.T) {
	checker := redshiftCheckerByTarget(t, "vpc")
	src := redshiftSrcResource(redshiftFixtureWarehouse(t))
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if result.Count() != 1 {
		t.Errorf("Count = %d, want 1", result.Count())
	}
	prodVPCID := "vpc-0abc123def456789a"
	if !containsID(result.ResourceIDs(), prodVPCID) {
		t.Errorf("ResourceIDs = %v, want to contain %q", result.ResourceIDs(), prodVPCID)
	}
}

func TestRelated_Redshift_VPC_NoVPCIDReturnsZero(t *testing.T) {
	cluster := redshifttypes.Cluster{
		ClusterIdentifier: aws.String("no-vpc-cluster"),
		ClusterStatus:     aws.String("available"),
		VpcId:             nil,
	}

	checker := redshiftCheckerByTarget(t, "vpc")
	src := redshiftSrcResource(cluster)
	result := checker(context.Background(), nil, src, resource.ResourceCache{})

	if result.Err() != nil {
		t.Errorf("unexpected error: %v", result.Err())
	}
	if result.Count() != 0 {
		t.Errorf("Count = %d, want 0 (no VPC ID)", result.Count())
	}
}

func TestRelated_Redshift_AllPivotsRegistered(t *testing.T) {
	// "ct-events" is declared in redshift's own catalog entry Related list
	// (core/aws/catalog_databases.go), like every top-level type, bringing the
	// canonical Redshift pivot count to 11 per docs/related-resources.md.
	expected := []string{"alarm", "sg", "vpc", "role", "kms", "cfn", "secrets", "logs", "s3", "subnet", "ct-events"}
	registered := make(map[string]struct{})
	for _, def := range resource.GetRelated("redshift") {
		registered[def.TargetType] = struct{}{}
	}
	for _, target := range expected {
		if _, ok := registered[target]; !ok {
			t.Errorf("redshift related pivot %q not registered", target)
		}
	}
}
