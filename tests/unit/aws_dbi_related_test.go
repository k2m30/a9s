// aws_dbi_related_test.go — related-target discovery tests for dbi.
//
// Tests pin every target from docs/resources/dbi.md §2 and impl-plan §1.5.
// Each test exercises the registered RelatedChecker directly via resource.GetRelated("dbi").
//
// ENI test (dbi-related-eni) also pins Delta A from impl-plan §3:
// filter pair must be requester-id=amazon-rds + vpc-id=<VpcId>, plus client-side
// description prefix match against DBInstanceIdentifier.
package unit

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// dbiENIFakeEC2 — minimal EC2API implementation for ENI tests
// ---------------------------------------------------------------------------

// dbiENIFakeEC2 records the filters passed to DescribeNetworkInterfaces so the
// test can verify the spec-mandated filter set (Delta A).
// It embeds EC2API to satisfy all other methods with zero-value returns.
type dbiENIFakeEC2 struct {
	awsclient.EC2API
	output      *ec2.DescribeNetworkInterfacesOutput
	err         error
	lastFilters []ec2types.Filter
	callCount   int
}

func (f *dbiENIFakeEC2) DescribeNetworkInterfaces(
	_ context.Context,
	in *ec2.DescribeNetworkInterfacesInput,
	_ ...func(*ec2.Options),
) (*ec2.DescribeNetworkInterfacesOutput, error) {
	f.callCount++
	if in != nil {
		f.lastFilters = in.Filters
	}
	if f.err != nil {
		return nil, f.err
	}
	if f.output != nil {
		return f.output, nil
	}
	return &ec2.DescribeNetworkInterfacesOutput{}, nil
}

// Compile-time check.
var _ awsclient.EC2API = (*dbiENIFakeEC2)(nil)

// ---------------------------------------------------------------------------
// Helper: look up a named checker from the "dbi" registry.
// ---------------------------------------------------------------------------

func dbiCheckerByTarget(t *testing.T, target string) resource.RelatedChecker {
	t.Helper()
	for _, def := range resource.GetRelated("dbi") {
		if def.TargetType == target {
			if def.Checker == nil {
				t.Fatalf("dbi related checker for %q is nil", target)
			}
			return def.Checker
		}
	}
	t.Fatalf("dbi related checker for %q not found in registry", target)
	return nil
}

// ---------------------------------------------------------------------------
// Helper: build a resource.Resource wrapping a DBInstance.
// ---------------------------------------------------------------------------

func dbiRes(db rdstypes.DBInstance) resource.Resource {
	id := ""
	if db.DBInstanceIdentifier != nil {
		id = *db.DBInstanceIdentifier
	}
	return resource.Resource{
		ID:        id,
		Name:      id,
		RawStruct: db,
	}
}

// ---------------------------------------------------------------------------
// TEST: dbi-related-alarm — §2 alarm checker (3 matching, 2 unrelated)
// ---------------------------------------------------------------------------

// TestDBIRelated_Alarm_ThreeMatchTwoUnrelated verifies that checkDbiAlarm returns
// count=3 when the alarm cache contains 3 alarms with DBInstanceIdentifier dimension
// matching "prod-db" and 2 unrelated alarms.
func TestDBIRelated_Alarm_ThreeMatchTwoUnrelated(t *testing.T) {
	db := dbiBaselineHealthy()
	res := dbiRes(db)

	cache := resource.ResourceCache{
		"alarm": {
			Resources: []resource.Resource{
				{ID: "alarm-cpu-1", RawStruct: cwtypes.MetricAlarm{Dimensions: []cwtypes.Dimension{{Name: aws.String("DBInstanceIdentifier"), Value: aws.String("prod-db")}}}},
				{ID: "alarm-cpu-2", RawStruct: cwtypes.MetricAlarm{Dimensions: []cwtypes.Dimension{{Name: aws.String("DBInstanceIdentifier"), Value: aws.String("prod-db")}}}},
				{ID: "alarm-storage", RawStruct: cwtypes.MetricAlarm{Dimensions: []cwtypes.Dimension{{Name: aws.String("DBInstanceIdentifier"), Value: aws.String("prod-db")}}}},
				{ID: "alarm-unrelated-1", RawStruct: cwtypes.MetricAlarm{Dimensions: []cwtypes.Dimension{{Name: aws.String("DBInstanceIdentifier"), Value: aws.String("other-db")}}}},
				{ID: "alarm-unrelated-2", RawStruct: cwtypes.MetricAlarm{Dimensions: []cwtypes.Dimension{{Name: aws.String("InstanceId"), Value: aws.String("i-12345")}}}},
			},
		},
	}

	checker := dbiCheckerByTarget(t, "alarm")
	got := checker(context.Background(), nil, res, cache)
	if got.Count != 3 {
		t.Errorf("dbi-related-alarm: Count = %d, want 3", got.Count)
	}
	if len(got.ResourceIDs) != 3 {
		t.Errorf("dbi-related-alarm: len(ResourceIDs) = %d, want 3", len(got.ResourceIDs))
	}
}

// TestDBIRelated_Alarm_NoAPICall verifies that checkDbiAlarm does NOT make any
// AWS API calls when the alarm cache is populated — purely a cache scan.
func TestDBIRelated_Alarm_NoAPICall(t *testing.T) {
	db := dbiBaselineHealthy()
	res := dbiRes(db)

	cache := resource.ResourceCache{
		"alarm": {Resources: []resource.Resource{}},
	}

	checker := dbiCheckerByTarget(t, "alarm")
	// nil clients signals no API calls should be made; checker must not panic.
	got := checker(context.Background(), nil, res, cache)
	if got.Err != nil {
		t.Errorf("dbi-related-alarm: unexpected error with nil clients: %v", got.Err)
	}
	_ = got
}

// ---------------------------------------------------------------------------
// TEST: dbi-related-dbc-aurora — §2 dbc checker with Aurora member
// ---------------------------------------------------------------------------

// TestDBIRelated_DBC_Aurora verifies that checkDbiDBC returns count=1 for an
// Aurora DB instance with a non-empty DBClusterIdentifier.
func TestDBIRelated_DBC_Aurora(t *testing.T) {
	db := dbiBaselineHealthy()
	db.Engine = aws.String("aurora-postgresql")
	db.DBClusterIdentifier = aws.String("cluster-xyz")
	res := dbiRes(db)

	cache := resource.ResourceCache{
		"dbc": {
			Resources: []resource.Resource{
				{ID: "cluster-xyz", Name: "cluster-xyz"},
				{ID: "cluster-other", Name: "cluster-other"},
			},
		},
	}

	checker := dbiCheckerByTarget(t, "dbc")
	got := checker(context.Background(), nil, res, cache)
	if got.Count != 1 {
		t.Errorf("dbi-related-dbc-aurora: Count = %d, want 1", got.Count)
	}
}

// ---------------------------------------------------------------------------
// TEST: dbi-related-dbc-rds-engine — §2 dbc checker with classic RDS (no cluster)
// ---------------------------------------------------------------------------

// TestDBIRelated_DBC_RDSEngine verifies that checkDbiDBC returns count=0 for a
// classic RDS instance with an empty DBClusterIdentifier.
func TestDBIRelated_DBC_RDSEngine(t *testing.T) {
	db := dbiBaselineHealthy()
	db.DBClusterIdentifier = nil
	res := dbiRes(db)

	cache := resource.ResourceCache{
		"dbc": {
			Resources: []resource.Resource{
				{ID: "cluster-xyz"},
			},
		},
	}

	checker := dbiCheckerByTarget(t, "dbc")
	got := checker(context.Background(), nil, res, cache)
	if got.Count != 0 {
		t.Errorf("dbi-related-dbc-rds-engine: Count = %d, want 0 (no cluster for classic RDS)", got.Count)
	}
}

// ---------------------------------------------------------------------------
// TEST: dbi-related-eni — §2 + Delta A filter verification
// ---------------------------------------------------------------------------

// TestDBIRelated_ENI_FilterAndClientSideMatch verifies:
//  1. DescribeNetworkInterfaces is called exactly once with the spec-mandated
//     filters: requester-id=amazon-rds AND vpc-id=vpc-abc (Delta A).
//  2. Client-side description prefix match is applied: the ENI referencing
//     "other-db" is excluded even though it passed the server-side filters.
//  3. Count = 2 (the two ENIs referencing "prod-db").
//
// NOTE: This test will FAIL until the coder implements Delta A (requester-id +
// vpc-id filter pair plus client-side description prefix check).
func TestDBIRelated_ENI_FilterAndClientSideMatch(t *testing.T) {
	db := dbiBaselineHealthy()
	// DBSubnetGroup.VpcId = "vpc-abc" is already set in baseline.
	res := dbiRes(db)

	fakeEC2 := &dbiENIFakeEC2{
		output: &ec2.DescribeNetworkInterfacesOutput{
			NetworkInterfaces: []ec2types.NetworkInterface{
				{NetworkInterfaceId: aws.String("eni-aaaa"), Description: aws.String("RDSNetworkInterface for prod-db primary")},
				{NetworkInterfaceId: aws.String("eni-bbbb"), Description: aws.String("RDSNetworkInterface for prod-db replica")},
				// This one must be filtered out client-side — describes a different DB instance.
				{NetworkInterfaceId: aws.String("eni-cccc"), Description: aws.String("RDSNetworkInterface for other-db primary")},
			},
		},
	}

	clients := &awsclient.ServiceClients{EC2: fakeEC2}
	checker := dbiCheckerByTarget(t, "eni")
	got := checker(context.Background(), clients, res, resource.ResourceCache{})

	// Exactly one API call.
	if fakeEC2.callCount != 1 {
		t.Errorf("dbi-related-eni: DescribeNetworkInterfaces called %d times, want 1", fakeEC2.callCount)
	}

	// Verify spec-mandated filter set (Delta A).
	hasRequesterIDFilter := false
	hasVpcIDFilter := false
	for _, f := range fakeEC2.lastFilters {
		if f.Name == nil {
			continue
		}
		switch *f.Name {
		case "requester-id":
			for _, v := range f.Values {
				if v == "amazon-rds" {
					hasRequesterIDFilter = true
				}
			}
		case "vpc-id":
			for _, v := range f.Values {
				if v == "vpc-abc" {
					hasVpcIDFilter = true
				}
			}
		}
	}
	if !hasRequesterIDFilter {
		t.Error("dbi-related-eni: filter requester-id=amazon-rds not present (Delta A not implemented)")
	}
	if !hasVpcIDFilter {
		t.Error("dbi-related-eni: filter vpc-id=vpc-abc not present (Delta A not implemented)")
	}

	// Client-side filter drops the eni-cccc (other-db description).
	if got.Count != 2 {
		t.Errorf("dbi-related-eni: Count = %d, want 2 (client-side filter must drop other-db ENI)", got.Count)
	}
}

// ---------------------------------------------------------------------------
// TEST: dbi-related-kms-encrypted — §2
// ---------------------------------------------------------------------------

// TestDBIRelated_KMS_Encrypted verifies that checkDbiKMS returns count=1 when
// the KMS key ID UUID matches a cached kms resource ID.
func TestDBIRelated_KMS_Encrypted(t *testing.T) {
	db := dbiBaselineHealthy()
	// KmsKeyId = "arn:aws:kms:us-east-1:123456789012:key/abcd-1234" is set in baseline.
	res := dbiRes(db)

	cache := resource.ResourceCache{
		"kms": {
			Resources: []resource.Resource{
				{ID: "abcd-1234"},
				{ID: "other-key-uuid"},
			},
		},
	}

	checker := dbiCheckerByTarget(t, "kms")
	got := checker(context.Background(), nil, res, cache)
	if got.Count != 1 {
		t.Errorf("dbi-related-kms-encrypted: Count = %d, want 1", got.Count)
	}
}

// ---------------------------------------------------------------------------
// TEST: dbi-related-kms-unencrypted — §2
// ---------------------------------------------------------------------------

// TestDBIRelated_KMS_Unencrypted verifies that checkDbiKMS returns count=0 when
// StorageEncrypted=false (KmsKeyId absent).
func TestDBIRelated_KMS_Unencrypted(t *testing.T) {
	db := dbiBaselineHealthy()
	db.StorageEncrypted = aws.Bool(false)
	db.KmsKeyId = nil
	res := dbiRes(db)

	checker := dbiCheckerByTarget(t, "kms")
	got := checker(context.Background(), nil, res, resource.ResourceCache{})
	if got.Count > 0 {
		t.Errorf("dbi-related-kms-unencrypted: Count = %d, want 0", got.Count)
	}
}

// ---------------------------------------------------------------------------
// TEST: dbi-related-logs — §2 naming-convention prefix match
// ---------------------------------------------------------------------------

// TestDBIRelated_Logs_TwoExportsTwoMatches verifies that checkDBILogs returns
// count=2 when the logs cache contains the two expected log groups matching
// /aws/rds/instance/prod-db/<export-type>, plus one unrelated group.
func TestDBIRelated_Logs_TwoExportsTwoMatches(t *testing.T) {
	db := dbiBaselineHealthy()
	db.EnabledCloudwatchLogsExports = []string{"error", "slowquery"}
	res := dbiRes(db)

	cache := resource.ResourceCache{
		"logs": {
			Resources: []resource.Resource{
				{ID: "/aws/rds/instance/prod-db/error"},
				{ID: "/aws/rds/instance/prod-db/slowquery"},
				{ID: "/aws/lambda/my-function"},
			},
		},
	}

	checker := dbiCheckerByTarget(t, "logs")
	got := checker(context.Background(), nil, res, cache)
	if got.Count != 2 {
		t.Errorf("dbi-related-logs: Count = %d, want 2", got.Count)
	}
}

// ---------------------------------------------------------------------------
// TEST: dbi-related-rds-snap — §2 reverse-cache lookup
// ---------------------------------------------------------------------------

// TestDBIRelated_RDSSnap_FourMatches verifies that checkDbiRDSSnap returns
// count=4 when the cache contains 4 snapshots referencing "prod-db" and 1 for
// another instance.
func TestDBIRelated_RDSSnap_FourMatches(t *testing.T) {
	db := dbiBaselineHealthy()
	res := dbiRes(db)

	cache := resource.ResourceCache{
		"rds-snap": {
			Resources: []resource.Resource{
				{ID: "rds:prod-db-2026-04-01", RawStruct: rdstypes.DBSnapshot{DBInstanceIdentifier: aws.String("prod-db")}},
				{ID: "rds:prod-db-2026-03-01", RawStruct: rdstypes.DBSnapshot{DBInstanceIdentifier: aws.String("prod-db")}},
				{ID: "rds:prod-db-2026-02-01", RawStruct: rdstypes.DBSnapshot{DBInstanceIdentifier: aws.String("prod-db")}},
				{ID: "rds:prod-db-2026-01-01", RawStruct: rdstypes.DBSnapshot{DBInstanceIdentifier: aws.String("prod-db")}},
				{ID: "rds:other-db-2026-04-01", RawStruct: rdstypes.DBSnapshot{DBInstanceIdentifier: aws.String("other-db")}},
			},
		},
	}

	checker := dbiCheckerByTarget(t, "rds-snap")
	got := checker(context.Background(), nil, res, cache)
	if got.Count != 4 {
		t.Errorf("dbi-related-rds-snap: Count = %d, want 4", got.Count)
	}
}

// ---------------------------------------------------------------------------
// TEST: dbi-related-role — §2 (MonitoringRoleArn + AssociatedRoles)
// ---------------------------------------------------------------------------

// TestDBIRelated_Role_MonitoringAndAssociated verifies that checkDbiRole returns
// count=2 when MonitoringRoleArn and one AssociatedRoles entry are present.
func TestDBIRelated_Role_MonitoringAndAssociated(t *testing.T) {
	db := dbiBaselineHealthy()
	db.MonitoringRoleArn = aws.String("arn:aws:iam::123456789012:role/rds-monitoring")
	db.AssociatedRoles = []rdstypes.DBInstanceRole{
		{RoleArn: aws.String("arn:aws:iam::123456789012:role/s3-import"), FeatureName: aws.String("s3Import")},
	}
	res := dbiRes(db)

	cache := resource.ResourceCache{
		"role": {
			Resources: []resource.Resource{
				{ID: "rds-monitoring"},
				{ID: "s3-import"},
				{ID: "unrelated-role"},
			},
		},
	}

	checker := dbiCheckerByTarget(t, "role")
	got := checker(context.Background(), nil, res, cache)
	if got.Count != 2 {
		t.Errorf("dbi-related-role: Count = %d, want 2", got.Count)
	}
}

// ---------------------------------------------------------------------------
// TEST: dbi-related-secrets-managed — §2
// ---------------------------------------------------------------------------

// TestDBIRelated_Secrets_ManagedPassword verifies that checkDbiSecrets returns
// count=1 when MasterUserSecret.SecretArn is present and matches the secrets cache.
func TestDBIRelated_Secrets_ManagedPassword(t *testing.T) {
	const secretARN = "arn:aws:secretsmanager:us-east-1:123456789012:secret/db-master-AbCdE"
	db := dbiBaselineHealthy()
	db.MasterUserSecret = &rdstypes.MasterUserSecret{
		SecretArn: aws.String(secretARN),
	}
	res := dbiRes(db)

	cache := resource.ResourceCache{
		"secrets": {
			Resources: []resource.Resource{
				{
					ID:     "db-master-AbCdE",
					Fields: map[string]string{"arn": secretARN},
					RawStruct: smtypes.SecretListEntry{
						ARN:  aws.String(secretARN),
						Name: aws.String("db-master-AbCdE"),
					},
				},
				{
					ID:     "other-secret",
					Fields: map[string]string{"arn": "arn:aws:secretsmanager:us-east-1:123456789012:secret/other"},
					RawStruct: smtypes.SecretListEntry{
						ARN: aws.String("arn:aws:secretsmanager:us-east-1:123456789012:secret/other"),
					},
				},
			},
		},
	}

	checker := dbiCheckerByTarget(t, "secrets")
	got := checker(context.Background(), nil, res, cache)
	if got.Count != 1 {
		t.Errorf("dbi-related-secrets-managed: Count = %d, want 1", got.Count)
	}
}

// ---------------------------------------------------------------------------
// TEST: dbi-related-secrets-classic-auth — §2
// ---------------------------------------------------------------------------

// TestDBIRelated_Secrets_ClassicAuth verifies that checkDbiSecrets returns count=0
// when MasterUserSecret is nil (classic password-auth instance).
func TestDBIRelated_Secrets_ClassicAuth(t *testing.T) {
	db := dbiBaselineHealthy()
	db.MasterUserSecret = nil
	res := dbiRes(db)

	checker := dbiCheckerByTarget(t, "secrets")
	got := checker(context.Background(), nil, res, resource.ResourceCache{})
	if got.Count > 0 {
		t.Errorf("dbi-related-secrets-classic-auth: Count = %d, want 0", got.Count)
	}
}

// ---------------------------------------------------------------------------
// TEST: dbi-related-sg — §2
// ---------------------------------------------------------------------------

// TestDBIRelated_SG_TwoGroups verifies that checkDbiSG returns count=2 when
// VpcSecurityGroups contains two group IDs.
func TestDBIRelated_SG_TwoGroups(t *testing.T) {
	db := dbiBaselineHealthy()
	db.VpcSecurityGroups = []rdstypes.VpcSecurityGroupMembership{
		{VpcSecurityGroupId: aws.String("sg-1")},
		{VpcSecurityGroupId: aws.String("sg-2")},
	}
	res := dbiRes(db)

	cache := resource.ResourceCache{
		"sg": {
			Resources: []resource.Resource{
				{ID: "sg-1"},
				{ID: "sg-2"},
				{ID: "sg-other"},
			},
		},
	}

	checker := dbiCheckerByTarget(t, "sg")
	got := checker(context.Background(), nil, res, cache)
	if got.Count != 2 {
		t.Errorf("dbi-related-sg: Count = %d, want 2", got.Count)
	}
	if len(got.ResourceIDs) != 2 {
		t.Errorf("dbi-related-sg: len(ResourceIDs) = %d, want 2", len(got.ResourceIDs))
	}
}

// ---------------------------------------------------------------------------
// TEST: dbi-related-subnet — §2 (three subnets)
// ---------------------------------------------------------------------------

// TestDBIRelated_Subnet_ThreeSubnets verifies that checkDbiSubnets returns
// count=3 when DBSubnetGroup.Subnets has three entries.
func TestDBIRelated_Subnet_ThreeSubnets(t *testing.T) {
	db := dbiBaselineHealthy()
	db.DBSubnetGroup = &rdstypes.DBSubnetGroup{
		VpcId: aws.String("vpc-abc"),
		Subnets: []rdstypes.Subnet{
			{SubnetIdentifier: aws.String("sub-a")},
			{SubnetIdentifier: aws.String("sub-b")},
			{SubnetIdentifier: aws.String("sub-c")},
		},
	}
	res := dbiRes(db)

	cache := resource.ResourceCache{
		"subnet": {
			Resources: []resource.Resource{
				{ID: "sub-a"},
				{ID: "sub-b"},
				{ID: "sub-c"},
			},
		},
	}

	checker := dbiCheckerByTarget(t, "subnet")
	got := checker(context.Background(), nil, res, cache)
	if got.Count != 3 {
		t.Errorf("dbi-related-subnet: Count = %d, want 3", got.Count)
	}
}

// ---------------------------------------------------------------------------
// TEST: dbi-related-vpc — §2
// ---------------------------------------------------------------------------

// TestDBIRelated_VPC_OneVPC verifies that checkDbiVPC returns count=1 and
// the correct VPC ID.
func TestDBIRelated_VPC_OneVPC(t *testing.T) {
	db := dbiBaselineHealthy()
	// DBSubnetGroup.VpcId = "vpc-abc" is set in baseline.
	res := dbiRes(db)

	cache := resource.ResourceCache{
		"vpc": {
			Resources: []resource.Resource{
				{ID: "vpc-abc"},
				{ID: "vpc-other"},
			},
		},
	}

	checker := dbiCheckerByTarget(t, "vpc")
	got := checker(context.Background(), nil, res, cache)
	if got.Count != 1 {
		t.Errorf("dbi-related-vpc: Count = %d, want 1", got.Count)
	}
	if len(got.ResourceIDs) != 1 || got.ResourceIDs[0] != "vpc-abc" {
		t.Errorf("dbi-related-vpc: ResourceIDs = %v, want [vpc-abc]", got.ResourceIDs)
	}
}

// ---------------------------------------------------------------------------
// TEST: dbi-related-ct-events — §2 (universal pivot; row present in registry)
// ---------------------------------------------------------------------------

// TestDBIRelated_CTEvents_RegistryHasEntry verifies that a ct-events entry is
// present in the dbi related registry. The checker is registered centrally by
// ct_events.go and applies to every type; this test confirms the registration
// is visible for dbi.
func TestDBIRelated_CTEvents_RegistryHasEntry(t *testing.T) {
	defs := resource.GetRelated("dbi")
	for _, def := range defs {
		if def.TargetType == "ct-events" {
			if def.Checker == nil {
				t.Fatal("dbi ct-events RelatedDef has nil Checker")
			}
			return // pass
		}
	}
	t.Fatal("dbi related registry has no entry for ct-events — universal pivot not registered")
}

// ---------------------------------------------------------------------------
// Invariant: every dbi RelatedCheckResult satisfies ValidateRelatedResult
// ---------------------------------------------------------------------------

// TestDBIRelated_AllCheckersPassShapeValidation verifies that all dbi related
// checkers return results that pass resource.ValidateRelatedResult shape invariants,
// when given a minimal baseline instance and no cache.
func TestDBIRelated_AllCheckersPassShapeValidation(t *testing.T) {
	db := dbiBaselineHealthy()
	db.MasterUserSecret = nil
	db.AssociatedRoles = nil
	db.MonitoringRoleArn = aws.String("")
	res := dbiRes(db)
	cache := resource.ResourceCache{}

	for _, def := range resource.GetRelated("dbi") {
		def := def
		t.Run(def.TargetType, func(t *testing.T) {
			got := def.Checker(context.Background(), nil, res, cache)
			if err := resource.ValidateRelatedResult(got); err != nil {
				t.Errorf("checker %q: %v", def.TargetType, err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// TEST: All 11 direct targets are registered under "dbi"
// ---------------------------------------------------------------------------

// TestDBIRelated_AllSpecTargetsRegistered verifies that all 11 direct targets
// from docs/resources/dbi.md §2 are registered. (ct-events is the 12th and is
// registered universally by ct_events.go, tested separately above.)
func TestDBIRelated_AllSpecTargetsRegistered(t *testing.T) {
	required := []string{"alarm", "dbc", "eni", "kms", "logs", "rds-snap", "role", "secrets", "sg", "subnet", "vpc"}
	defs := resource.GetRelated("dbi")
	registered := make(map[string]bool, len(defs))
	for _, def := range defs {
		registered[def.TargetType] = true
	}
	for _, target := range required {
		if !registered[target] {
			t.Errorf("dbi related registry is missing required target %q", target)
		}
	}
}
