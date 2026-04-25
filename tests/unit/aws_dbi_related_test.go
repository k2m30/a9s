package unit

// aws_dbi_related_test.go — per-target related-resource checker tests for dbi.
//
// One test per §2 target. All tests use the prod-dbi-1 fixture as the anchor
// resource. Each test constructs a ResourceCache with the minimum sibling data
// needed to verify the checker's discovery logic, then asserts Count and
// ResourceIDs.
//
// Nil-RawStruct tests verify each checker returns the correct error sentinel.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	ec2svc "github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// dbiProdResource builds the prod-dbi-1 Resource by running it through the
// fetcher so RawStruct is typed rdstypes.DBInstance.
func dbiProdResource(t *testing.T) resource.Resource {
	t.Helper()
	inst := findDBI(t, fixtures.ProdDbiID)
	mock := &mockRDSPageClient{instances: []rdstypes.DBInstance{inst}}
	result, err := awsclient.FetchRDSInstancesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("dbiProdResource: FetchRDSInstancesPage error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("dbiProdResource: expected 1 resource, got %d", len(result.Resources))
	}
	return result.Resources[0]
}

// dbiAuroraResource builds the prod-dbi-aurora-1 Resource.
func dbiAuroraResource(t *testing.T) resource.Resource {
	t.Helper()
	inst := findDBI(t, fixtures.ProdDbiAuroraID)
	mock := &mockRDSPageClient{instances: []rdstypes.DBInstance{inst}}
	result, err := awsclient.FetchRDSInstancesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("dbiAuroraResource: error: %v", err)
	}
	return result.Resources[0]
}

// dbiCheckerByTarget returns the RelatedChecker registered for a given target type.
func dbiCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("dbi") {
		if def.TargetType == target {
			return def.Checker
		}
	}
	t.Fatalf("no checker registered for dbi→%s", target)
	return nil
}

// ---------------------------------------------------------------------------
// sg
// ---------------------------------------------------------------------------

// TestDBI_Related_SG_ReturnsVpcSecurityGroupIDs verifies checkDbiSG returns
// the SG IDs from VpcSecurityGroups.
func TestDBI_Related_SG_ReturnsVpcSecurityGroupIDs(t *testing.T) {
	res := dbiProdResource(t)
	checker := dbiCheckerByTarget(t, "sg")

	// sg cache entry so ValidateRelatedResultAgainstCache can pass.
	cache := resource.ResourceCache{
		"sg": resource.ResourceCacheEntry{
			Resources: []resource.Resource{{ID: "sg-0ccc333333333333c"}},
		},
	}
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "sg-0ccc333333333333c" {
		t.Errorf("ResourceIDs = %v, want [sg-0ccc333333333333c]", result.ResourceIDs)
	}
}

// TestDBI_Related_SG_NilRawStruct verifies nil RawStruct returns Count=-1.
func TestDBI_Related_SG_NilRawStruct(t *testing.T) {
	res := resource.Resource{ID: "x", RawStruct: nil}
	checker := dbiCheckerByTarget(t, "sg")
	result := checker(context.Background(), nil, res, resource.ResourceCache{})
	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 for nil RawStruct", result.Count)
	}
}

// ---------------------------------------------------------------------------
// kms
// ---------------------------------------------------------------------------

// TestDBI_Related_KMS_ReturnsKeyUUID verifies checkDbiKMS extracts the UUID
// suffix from the KmsKeyId ARN.
func TestDBI_Related_KMS_ReturnsKeyUUID(t *testing.T) {
	res := dbiProdResource(t)
	checker := dbiCheckerByTarget(t, "kms")

	wantUUID := "a1b2c3d4-5678-90ab-cdef-111111111111"
	cache := resource.ResourceCache{
		"kms": resource.ResourceCacheEntry{
			Resources: []resource.Resource{{ID: wantUUID}},
		},
	}
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != wantUUID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, wantUUID)
	}
}

// TestDBI_Related_KMS_UnencryptedInstance verifies Count=0 when KmsKeyId is nil.
func TestDBI_Related_KMS_UnencryptedInstance(t *testing.T) {
	inst := findDBI(t, fixtures.WarnDbiUnencryptedID)
	mock := &mockRDSPageClient{instances: []rdstypes.DBInstance{inst}}
	result, err := awsclient.FetchRDSInstancesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRDSInstancesPage error: %v", err)
	}
	res := result.Resources[0]
	checker := dbiCheckerByTarget(t, "kms")

	got := checker(context.Background(), nil, res, resource.ResourceCache{})
	if got.Count != 0 {
		t.Errorf("Count = %d, want 0 for unencrypted instance", got.Count)
	}
}

// ---------------------------------------------------------------------------
// subnet
// ---------------------------------------------------------------------------

// TestDBI_Related_Subnets_ReturnsBothSubnetIDs verifies checkDbiSubnets returns
// both subnet IDs from the DBSubnetGroup.
func TestDBI_Related_Subnets_ReturnsBothSubnetIDs(t *testing.T) {
	res := dbiProdResource(t)
	checker := dbiCheckerByTarget(t, "subnet")

	cache := resource.ResourceCache{
		"subnet": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "subnet-0aaa111111111111a"},
				{ID: "subnet-0ccc333333333333c"},
			},
		},
	}
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
}

// ---------------------------------------------------------------------------
// vpc
// ---------------------------------------------------------------------------

// TestDBI_Related_VPC_ReturnsVpcID verifies checkDbiVPC returns the VPC ID.
func TestDBI_Related_VPC_ReturnsVpcID(t *testing.T) {
	res := dbiProdResource(t)
	checker := dbiCheckerByTarget(t, "vpc")

	wantVPC := "vpc-0abc123def456789a"
	cache := resource.ResourceCache{
		"vpc": resource.ResourceCacheEntry{
			Resources: []resource.Resource{{ID: wantVPC}},
		},
	}
	result := checker(context.Background(), nil, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != wantVPC {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, wantVPC)
	}
}

// ---------------------------------------------------------------------------
// alarm
// ---------------------------------------------------------------------------

// TestDBI_Related_Alarm_MatchesByDBInstanceIdentifierDimension verifies
// checkDbiAlarm scans the alarm cache and matches on the DBInstanceIdentifier
// dimension.
func TestDBI_Related_Alarm_MatchesByDBInstanceIdentifierDimension(t *testing.T) {
	res := dbiProdResource(t)
	checker := dbiCheckerByTarget(t, "alarm")

	matchingAlarm := resource.Resource{
		ID:   "rds-cpu-utilization",
		Name: "rds-cpu-utilization",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("rds-cpu-utilization"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("DBInstanceIdentifier"), Value: aws.String(fixtures.ProdDbiID)},
			},
		},
	}
	otherAlarm := resource.Resource{
		ID:   "ec2-cpu-alarm",
		Name: "ec2-cpu-alarm",
		RawStruct: cwtypes.MetricAlarm{
			AlarmName: aws.String("ec2-cpu-alarm"),
			Dimensions: []cwtypes.Dimension{
				{Name: aws.String("InstanceId"), Value: aws.String("i-0a1b2c3d")},
			},
		},
	}
	cache := resource.ResourceCache{
		"alarm": resource.ResourceCacheEntry{
			Resources: []resource.Resource{matchingAlarm, otherAlarm},
		},
	}

	result := checker(context.Background(), &awsclient.ServiceClients{}, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "rds-cpu-utilization" {
		t.Errorf("ResourceIDs = %v, want [rds-cpu-utilization]", result.ResourceIDs)
	}
}

// ---------------------------------------------------------------------------
// dbi-snap
// ---------------------------------------------------------------------------

// TestDBI_Related_DBISnap_MatchesByDBInstanceIdentifier verifies
// checkDbiDBISnap finds snapshots whose DBInstanceIdentifier matches res.ID.
func TestDBI_Related_DBISnap_MatchesByDBInstanceIdentifier(t *testing.T) {
	res := dbiProdResource(t)
	checker := dbiCheckerByTarget(t, "dbi-snap")

	snap1ID := "rds:" + fixtures.ProdDbiID + "-2026-04-15"
	snap2ID := "pre-migration-" + fixtures.ProdDbiID
	snap1 := resource.Resource{
		ID:        snap1ID,
		RawStruct: rdstypes.DBSnapshot{DBInstanceIdentifier: aws.String(fixtures.ProdDbiID)},
	}
	snap2 := resource.Resource{
		ID:        snap2ID,
		RawStruct: rdstypes.DBSnapshot{DBInstanceIdentifier: aws.String(fixtures.ProdDbiID)},
	}
	otherSnap := resource.Resource{
		ID:        "rds:other-db-2026-01-01",
		RawStruct: rdstypes.DBSnapshot{DBInstanceIdentifier: aws.String("other-db")},
	}
	cache := resource.ResourceCache{
		"dbi-snap": resource.ResourceCacheEntry{
			Resources: []resource.Resource{snap1, snap2, otherSnap},
		},
	}

	result := checker(context.Background(), &awsclient.ServiceClients{}, res, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
}

// ---------------------------------------------------------------------------
// logs
// ---------------------------------------------------------------------------

// TestDBI_Related_Logs_MatchesByRDSNamingConvention verifies checkDBILogs
// returns log groups matching /aws/rds/instance/{id}/{export-type}.
func TestDBI_Related_Logs_MatchesByRDSNamingConvention(t *testing.T) {
	res := dbiProdResource(t)
	checker := dbiCheckerByTarget(t, "logs")

	lg1 := resource.Resource{ID: "/aws/rds/instance/" + fixtures.ProdDbiID + "/postgresql"}
	lg2 := resource.Resource{ID: "/aws/rds/instance/" + fixtures.ProdDbiID + "/upgrade"}
	otherLG := resource.Resource{ID: "/aws/lambda/api-service"}
	cache := resource.ResourceCache{
		"logs": resource.ResourceCacheEntry{
			Resources: []resource.Resource{lg1, lg2, otherLG},
		},
	}

	result := checker(context.Background(), &awsclient.ServiceClients{}, res, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2", result.Count)
	}
}

// ---------------------------------------------------------------------------
// secrets
// ---------------------------------------------------------------------------

// TestDBI_Related_Secrets_MatchesByMasterUserSecretARN verifies checkDbiSecrets
// matches the cached secret whose ARN equals MasterUserSecret.SecretArn.
func TestDBI_Related_Secrets_MatchesByMasterUserSecretARN(t *testing.T) {
	res := dbiProdResource(t)
	checker := dbiCheckerByTarget(t, "secrets")

	secretRes := resource.Resource{
		ID:   "rds!db-prod-dbi-1-ABCDEF",
		Name: "rds!db-prod-dbi-1-ABCDEF",
		Fields: map[string]string{
			"arn": fixtures.ProdDbiMasterSecretARN,
		},
		RawStruct: smtypes.SecretListEntry{
			ARN:  aws.String(fixtures.ProdDbiMasterSecretARN),
			Name: aws.String("rds!db-prod-dbi-1-ABCDEF"),
		},
	}
	otherSecret := resource.Resource{
		ID:     "unrelated-secret",
		Fields: map[string]string{"arn": "arn:aws:secretsmanager:us-east-1:123456789012:secret:other-XXXXXX"},
		RawStruct: smtypes.SecretListEntry{
			ARN: aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret:other-XXXXXX"),
		},
	}
	cache := resource.ResourceCache{
		"secrets": resource.ResourceCacheEntry{
			Resources: []resource.Resource{secretRes, otherSecret},
		},
	}

	result := checker(context.Background(), &awsclient.ServiceClients{}, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "rds!db-prod-dbi-1-ABCDEF" {
		t.Errorf("ResourceIDs = %v, want [rds!db-prod-dbi-1-ABCDEF]", result.ResourceIDs)
	}
}

// TestDBI_Related_Secrets_NoMasterUserSecret verifies Count=0 when
// MasterUserSecret is nil (classic password-auth instance). Constructs a
// minimal inline fixture because the production Aurora fixture now carries a
// MasterUserSecret (required so the Aurora dbi graph-root covers every
// related pivot — see scenario_dbi_visual_test.go).
func TestDBI_Related_Secrets_NoMasterUserSecret(t *testing.T) {
	raw := rdstypes.DBInstance{
		DBInstanceIdentifier: aws.String("classic-password-auth-dbi"),
		Engine:               aws.String("postgres"),
		// MasterUserSecret intentionally nil.
	}
	res := resource.Resource{
		ID:        "classic-password-auth-dbi",
		Name:      "classic-password-auth-dbi",
		RawStruct: raw,
	}
	checker := dbiCheckerByTarget(t, "secrets")

	result := checker(context.Background(), &awsclient.ServiceClients{}, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (no MasterUserSecret)", result.Count)
	}
}

// ---------------------------------------------------------------------------
// dbc
// ---------------------------------------------------------------------------

// TestDBI_Related_DBC_Aurora_ReturnsClusterID verifies checkDbiDBC returns the
// Aurora cluster ID when DBClusterIdentifier is set.
func TestDBI_Related_DBC_Aurora_ReturnsClusterID(t *testing.T) {
	res := dbiAuroraResource(t)
	checker := dbiCheckerByTarget(t, "dbc")

	clusterID := "prod-aurora-cluster"
	cache := resource.ResourceCache{
		"dbc": resource.ResourceCacheEntry{
			Resources: []resource.Resource{{ID: clusterID, Name: clusterID}},
		},
	}

	result := checker(context.Background(), &awsclient.ServiceClients{}, res, cache)

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1 for Aurora member", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != clusterID {
		t.Errorf("ResourceIDs = %v, want [%s]", result.ResourceIDs, clusterID)
	}
}

// TestDBI_Related_DBC_NonAurora_ReturnsZero verifies checkDbiDBC returns
// Count=0 for a non-Aurora instance (no DBClusterIdentifier).
func TestDBI_Related_DBC_NonAurora_ReturnsZero(t *testing.T) {
	res := dbiProdResource(t)
	checker := dbiCheckerByTarget(t, "dbc")

	result := checker(context.Background(), &awsclient.ServiceClients{}, res, resource.ResourceCache{})

	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 for non-Aurora instance", result.Count)
	}
}

// ---------------------------------------------------------------------------
// role
// ---------------------------------------------------------------------------

// TestDBI_Related_Role_ReturnsAssociatedAndMonitoringRoles verifies
// checkDbiRole returns both AssociatedRoles and MonitoringRoleArn role names.
func TestDBI_Related_Role_ReturnsAssociatedAndMonitoringRoles(t *testing.T) {
	res := dbiProdResource(t)
	checker := dbiCheckerByTarget(t, "role")

	cache := resource.ResourceCache{
		"role": resource.ResourceCacheEntry{
			Resources: []resource.Resource{
				{ID: "rds-monitoring-role"},
				{ID: "rds-enhanced-monitoring"},
			},
		},
	}

	result := checker(context.Background(), nil, res, cache)

	if result.Count != 2 {
		t.Errorf("Count = %d, want 2 (AssociatedRoles + MonitoringRoleArn)", result.Count)
	}

	found := make(map[string]bool)
	for _, id := range result.ResourceIDs {
		found[id] = true
	}
	for _, want := range []string{"rds-monitoring-role", "rds-enhanced-monitoring"} {
		if !found[want] {
			t.Errorf("ResourceIDs missing %q; got %v", want, result.ResourceIDs)
		}
	}
}

// ---------------------------------------------------------------------------
// eni
// ---------------------------------------------------------------------------

// mockEC2ENIClient implements EC2API for ENI-checker tests.
type mockEC2ENIClient struct {
	awsclient.EC2API
	output *ec2svc.DescribeNetworkInterfacesOutput
	err    error
}

func (m *mockEC2ENIClient) DescribeNetworkInterfaces(
	_ context.Context,
	_ *ec2svc.DescribeNetworkInterfacesInput,
	_ ...func(*ec2svc.Options),
) (*ec2svc.DescribeNetworkInterfacesOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.output, nil
}

// TestDBI_Related_ENI_ReturnsNetworkInterfaceID verifies checkDbiENI calls
// DescribeNetworkInterfaces and returns the ENI IDs.
func TestDBI_Related_ENI_ReturnsNetworkInterfaceID(t *testing.T) {
	res := dbiProdResource(t)
	checker := dbiCheckerByTarget(t, "eni")

	fakeEC2 := &mockEC2ENIClient{
		output: &ec2svc.DescribeNetworkInterfacesOutput{
			NetworkInterfaces: []ec2types.NetworkInterface{
				{NetworkInterfaceId: aws.String("eni-0a1b2c3d4e5f60001")},
			},
		},
	}
	clients := &awsclient.ServiceClients{EC2: fakeEC2}

	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != 1 {
		t.Errorf("Count = %d, want 1", result.Count)
	}
	if len(result.ResourceIDs) == 0 || result.ResourceIDs[0] != "eni-0a1b2c3d4e5f60001" {
		t.Errorf("ResourceIDs = %v, want [eni-0a1b2c3d4e5f60001]", result.ResourceIDs)
	}
}

// TestDBI_Related_ENI_NilEC2Client verifies Count=-1 when EC2 client is nil.
func TestDBI_Related_ENI_NilEC2Client(t *testing.T) {
	res := dbiProdResource(t)
	checker := dbiCheckerByTarget(t, "eni")

	clients := &awsclient.ServiceClients{EC2: nil}
	result := checker(context.Background(), clients, res, resource.ResourceCache{})

	if result.Count != -1 {
		t.Errorf("Count = %d, want -1 for nil EC2 client", result.Count)
	}
}

// ---------------------------------------------------------------------------
// ct-events
// ---------------------------------------------------------------------------

// TestDBI_Related_CTEvents_MatchesByResourceName verifies checkDbiCTEvents
// scans the ct-events cache for events where fields["resource_name"] matches
// res.ID and sets FetchFilter["ResourceName"] = res.ID.
func TestDBI_Related_CTEvents_MatchesByResourceName(t *testing.T) {
	res := dbiProdResource(t)
	checker := dbiCheckerByTarget(t, "ct-events")

	ev1 := resource.Resource{
		ID:   "evt-dbi-001",
		Name: "evt-dbi-001",
		Fields: map[string]string{
			"resource_name": fixtures.ProdDbiID,
		},
		RawStruct: cloudtrailtypes.Event{
			EventId: aws.String("evt-dbi-001"),
			Resources: []cloudtrailtypes.Resource{
				{ResourceName: aws.String(fixtures.ProdDbiID)},
			},
		},
	}
	ev2 := resource.Resource{
		ID:   "evt-dbi-002",
		Name: "evt-dbi-002",
		Fields: map[string]string{
			"resource_name": fixtures.ProdDbiID,
		},
		RawStruct: cloudtrailtypes.Event{
			EventId: aws.String("evt-dbi-002"),
			Resources: []cloudtrailtypes.Resource{
				{ResourceName: aws.String(fixtures.ProdDbiID)},
			},
		},
	}
	otherEv := resource.Resource{
		ID:   "evt-other",
		Name: "evt-other",
		Fields: map[string]string{
			"resource_name": "some-other-db",
		},
		RawStruct: cloudtrailtypes.Event{
			EventId: aws.String("evt-other"),
			Resources: []cloudtrailtypes.Resource{
				{ResourceName: aws.String("some-other-db")},
			},
		},
	}
	cache := resource.ResourceCache{
		"ct-events": resource.ResourceCacheEntry{
			Resources: []resource.Resource{ev1, ev2, otherEv},
		},
	}

	result := checker(context.Background(), nil, res, cache)

	if result.Count < 2 {
		t.Errorf("Count = %d, want >=2 (events matching prod-dbi-1)", result.Count)
	}
	if result.FetchFilter == nil || result.FetchFilter["ResourceName"] != fixtures.ProdDbiID {
		t.Errorf("FetchFilter[ResourceName] = %q, want %q", result.FetchFilter["ResourceName"], fixtures.ProdDbiID)
	}
}

// TestDBI_Related_CTEvents_NoMatchEmptyCache verifies Count=-1 when cache is
// empty and no clients are available (cache miss).
func TestDBI_Related_CTEvents_NoMatchEmptyCache(t *testing.T) {
	res := dbiProdResource(t)
	checker := dbiCheckerByTarget(t, "ct-events")

	result := checker(context.Background(), nil, res, resource.ResourceCache{})

	// -1 = unknown (cache miss, no clients) OR FetchFilter set for navigation.
	// Either is acceptable; the key invariant is Count != 0 (0 would claim definite absence).
	if result.Count == 0 {
		t.Errorf("Count = 0 on empty cache — should be -1 (unknown) when cache has no ct-events entry")
	}
	if result.FetchFilter == nil || result.FetchFilter["ResourceName"] != fixtures.ProdDbiID {
		t.Errorf("FetchFilter[ResourceName] = %q, want %q even on cache miss", result.FetchFilter["ResourceName"], fixtures.ProdDbiID)
	}
}
