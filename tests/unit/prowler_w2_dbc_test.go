package unit

// prowler_w2_dbc_test.go — dbc posture row 17 of the w2 Prowler batch:
// single-AZ, auto minor version upgrade off, IAM database authentication off,
// default master username.
//
// The dbc list is fed by two fetchers — DocumentDB clusters and RDS (Aurora /
// Multi-AZ) clusters. One resource type must not answer differently depending
// on which SDK a row arrived from, so the shared conditions are asserted
// against both fetchers with the same expected code, phrase and severity.
//
// docdbtypes.DBCluster carries MultiAZ and MasterUsername but has no
// AutoMinorVersionUpgrade or IAMDatabaseAuthenticationEnabled field, so those
// two conditions are RDS-only. The absence is a property of the DocumentDB
// API, not a gap in the classifier — asserting them on DocumentDB would
// demand a value AWS does not return.

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

const (
	w2DBCCodeSingleAZ        = "dbc.single-az"
	w2DBCCodeMinorUpgradeOff = "dbc.minor-upgrade-off"
	w2DBCCodeIAMAuthOff      = "dbc.iam-auth-off"
	w2DBCCodeDefaultMaster   = "dbc.default-master-user"
)

type w2DocDBClustersFake struct {
	clusters []docdbtypes.DBCluster
}

func (f *w2DocDBClustersFake) DescribeDBClusters(_ context.Context, _ *docdb.DescribeDBClustersInput, _ ...func(*docdb.Options)) (*docdb.DescribeDBClustersOutput, error) {
	return &docdb.DescribeDBClustersOutput{DBClusters: f.clusters}, nil
}

type w2RDSClustersFake struct {
	clusters []rdstypes.DBCluster
}

func (f *w2RDSClustersFake) DescribeDBClusters(_ context.Context, _ *rds.DescribeDBClustersInput, _ ...func(*rds.Options)) (*rds.DescribeDBClustersOutput, error) {
	return &rds.DescribeDBClustersOutput{DBClusters: f.clusters}, nil
}

// w2DocDBCluster returns an otherwise-healthy available DocumentDB cluster
// with one writer member.
func w2DocDBCluster(id string) docdbtypes.DBCluster {
	return docdbtypes.DBCluster{
		DBClusterIdentifier:   aws.String(id),
		DBClusterArn:          aws.String("arn:aws:rds:eu-central-1:123456789012:cluster:" + id),
		Status:                aws.String("available"),
		Engine:                aws.String("docdb"),
		EngineVersion:         aws.String("5.0.0"),
		MultiAZ:               aws.Bool(true),
		MasterUsername:        aws.String("acme_ops"),
		DeletionProtection:    aws.Bool(true),
		StorageEncrypted:      aws.Bool(true),
		BackupRetentionPeriod: aws.Int32(7),
		DBClusterMembers: []docdbtypes.DBClusterMember{
			{DBInstanceIdentifier: aws.String(id + "-1"), IsClusterWriter: aws.Bool(true)},
		},
	}
}

// w2AuroraCluster returns an otherwise-healthy available Aurora cluster with
// one writer member.
func w2AuroraCluster(id string) rdstypes.DBCluster {
	return rdstypes.DBCluster{
		DBClusterIdentifier:              aws.String(id),
		DBClusterArn:                     aws.String("arn:aws:rds:eu-central-1:123456789012:cluster:" + id),
		Status:                           aws.String("available"),
		Engine:                           aws.String("aurora-postgresql"),
		EngineVersion:                    aws.String("16.3"),
		MultiAZ:                          aws.Bool(true),
		AutoMinorVersionUpgrade:          aws.Bool(true),
		IAMDatabaseAuthenticationEnabled: aws.Bool(true),
		MasterUsername:                   aws.String("acme_ops"),
		DeletionProtection:               aws.Bool(true),
		StorageEncrypted:                 aws.Bool(true),
		BackupRetentionPeriod:            aws.Int32(7),
		DBClusterMembers: []rdstypes.DBClusterMember{
			{DBInstanceIdentifier: aws.String(id + "-1"), IsClusterWriter: aws.Bool(true)},
		},
	}
}

func w2DocDBFetch(t *testing.T, clusters ...docdbtypes.DBCluster) map[string]resource.Resource {
	t.Helper()
	out, err := awsclient.FetchDocDBClustersPage(context.Background(), &w2DocDBClustersFake{clusters: clusters}, "")
	if err != nil {
		t.Fatalf("FetchDocDBClustersPage: %v", err)
	}
	return w2ByID(out.Resources)
}

func w2AuroraFetch(t *testing.T, clusters ...rdstypes.DBCluster) map[string]resource.Resource {
	t.Helper()
	out, err := awsclient.FetchRDSDBClustersPage(context.Background(), &w2RDSClustersFake{clusters: clusters}, "")
	if err != nil {
		t.Fatalf("FetchRDSDBClustersPage: %v", err)
	}
	return w2ByID(out.Resources)
}

func w2ByID(rs []resource.Resource) map[string]resource.Resource {
	byID := make(map[string]resource.Resource, len(rs))
	for _, r := range rs {
		byID[r.ID] = r
	}
	return byID
}

// ---------------------------------------------------------------------------
// single-AZ — asserted identically on both cluster fetchers
// ---------------------------------------------------------------------------

func TestW2DBCSingleAZOnBothFetchers(t *testing.T) {
	doc := w2DocDBCluster("acme-docs-cluster")
	doc.MultiAZ = aws.Bool(false)
	docGot := w2DocDBFetch(t, doc, w2DocDBCluster("acme-docs-healthy"))

	aur := w2AuroraCluster("acme-orders-cluster")
	aur.MultiAZ = aws.Bool(false)
	aurGot := w2AuroraFetch(t, aur, w2AuroraCluster("acme-orders-healthy"))

	w2AssertFinding(t, docGot["acme-docs-cluster"].Findings, w2DBCCodeSingleAZ, "single-AZ", domain.SevWarn, "wave1")
	w2AssertFinding(t, aurGot["acme-orders-cluster"].Findings, w2DBCCodeSingleAZ, "single-AZ", domain.SevWarn, "wave1")

	w2AssertNoCode(t, docGot["acme-docs-healthy"].Findings, w2DBCCodeSingleAZ)
	w2AssertNoCode(t, aurGot["acme-orders-healthy"].Findings, w2DBCCodeSingleAZ)

	w2AssertFindingDef(t, "dbc", w2DBCCodeSingleAZ, "single-AZ", domain.SevWarn, "wave1")
}

// ---------------------------------------------------------------------------
// default master username — asserted identically on both cluster fetchers
// ---------------------------------------------------------------------------

func TestW2DBCDefaultMasterUserOnBothFetchers(t *testing.T) {
	doc := w2DocDBCluster("acme-docs-cluster")
	doc.MasterUsername = aws.String("master")
	docGot := w2DocDBFetch(t, doc, w2DocDBCluster("acme-docs-healthy"))

	aur := w2AuroraCluster("acme-orders-cluster")
	aur.MasterUsername = aws.String("Postgres")
	aurGot := w2AuroraFetch(t, aur, w2AuroraCluster("acme-orders-healthy"))

	w2AssertFinding(t, docGot["acme-docs-cluster"].Findings, w2DBCCodeDefaultMaster, "default master username", domain.SevWarn, "wave1")
	w2AssertFinding(t, aurGot["acme-orders-cluster"].Findings, w2DBCCodeDefaultMaster, "default master username", domain.SevWarn, "wave1")

	w2AssertNoCode(t, docGot["acme-docs-healthy"].Findings, w2DBCCodeDefaultMaster)
	w2AssertNoCode(t, aurGot["acme-orders-healthy"].Findings, w2DBCCodeDefaultMaster)

	w2AssertFindingDef(t, "dbc", w2DBCCodeDefaultMaster, "default master username", domain.SevWarn, "wave1")
}

// ---------------------------------------------------------------------------
// RDS-only conditions
// ---------------------------------------------------------------------------

func TestW2DBCMinorUpgradeOff(t *testing.T) {
	off := w2AuroraCluster("acme-orders-cluster")
	off.AutoMinorVersionUpgrade = aws.Bool(false)

	got := w2AuroraFetch(t, off, w2AuroraCluster("acme-orders-healthy"))

	w2AssertFinding(t, got["acme-orders-cluster"].Findings, w2DBCCodeMinorUpgradeOff, "auto minor version upgrade off", domain.SevWarn, "wave1")
	w2AssertNoCode(t, got["acme-orders-healthy"].Findings, w2DBCCodeMinorUpgradeOff)
	w2AssertFindingDef(t, "dbc", w2DBCCodeMinorUpgradeOff, "auto minor version upgrade off", domain.SevWarn, "wave1")
}

func TestW2DBCIAMAuthOff(t *testing.T) {
	off := w2AuroraCluster("acme-orders-cluster")
	off.IAMDatabaseAuthenticationEnabled = aws.Bool(false)

	got := w2AuroraFetch(t, off, w2AuroraCluster("acme-orders-healthy"))

	w2AssertFinding(t, got["acme-orders-cluster"].Findings, w2DBCCodeIAMAuthOff, "IAM database authentication off", domain.SevWarn, "wave1")
	w2AssertNoCode(t, got["acme-orders-healthy"].Findings, w2DBCCodeIAMAuthOff)
	w2AssertFindingDef(t, "dbc", w2DBCCodeIAMAuthOff, "IAM database authentication off", domain.SevWarn, "wave1")
}

// DocumentDB clusters have no auto-minor-upgrade or IAM-auth field. Emitting
// either would be a finding invented from a zero value.
func TestW2DBCDocDBDoesNotInventRDSOnlyConditions(t *testing.T) {
	got := w2DocDBFetch(t, w2DocDBCluster("acme-docs-cluster"))
	w2AssertNoCode(t, got["acme-docs-cluster"].Findings, w2DBCCodeMinorUpgradeOff)
	w2AssertNoCode(t, got["acme-docs-cluster"].Findings, w2DBCCodeIAMAuthOff)
}

// ---------------------------------------------------------------------------
// nil pointers and lifecycle
// ---------------------------------------------------------------------------

// An absent pointer is unknown, not misconfigured — the batch contract's
// default for every row that does not say otherwise.
func TestW2DBCNilPointersEmitNothing(t *testing.T) {
	doc := w2DocDBCluster("acme-docs-nil")
	doc.MultiAZ = nil
	doc.MasterUsername = nil

	aur := w2AuroraCluster("acme-orders-nil")
	aur.MultiAZ = nil
	aur.AutoMinorVersionUpgrade = nil
	aur.IAMDatabaseAuthenticationEnabled = nil
	aur.MasterUsername = nil

	docGot := w2DocDBFetch(t, doc)
	aurGot := w2AuroraFetch(t, aur)

	for _, code := range []string{w2DBCCodeSingleAZ, w2DBCCodeDefaultMaster} {
		w2AssertNoCode(t, docGot["acme-docs-nil"].Findings, code)
	}
	for _, code := range []string{w2DBCCodeSingleAZ, w2DBCCodeMinorUpgradeOff, w2DBCCodeIAMAuthOff, w2DBCCodeDefaultMaster} {
		w2AssertNoCode(t, aurGot["acme-orders-nil"].Findings, code)
	}
}

func TestW2DBCDeletingClusterEmitsNoPostureFinding(t *testing.T) {
	doc := w2DocDBCluster("acme-docs-deleting")
	doc.Status = aws.String("deleting")
	doc.MultiAZ = aws.Bool(false)
	doc.MasterUsername = aws.String("root")

	aur := w2AuroraCluster("acme-orders-deleting")
	aur.Status = aws.String("deleting")
	aur.MultiAZ = aws.Bool(false)
	aur.MasterUsername = aws.String("root")

	docGot := w2DocDBFetch(t, doc)
	aurGot := w2AuroraFetch(t, aur)

	w2AssertNoCode(t, docGot["acme-docs-deleting"].Findings, w2DBCCodeSingleAZ)
	w2AssertNoCode(t, docGot["acme-docs-deleting"].Findings, w2DBCCodeDefaultMaster)
	w2AssertNoCode(t, aurGot["acme-orders-deleting"].Findings, w2DBCCodeSingleAZ)
	w2AssertNoCode(t, aurGot["acme-orders-deleting"].Findings, w2DBCCodeDefaultMaster)
}

// Two problems on one cluster stay two findings on both fetchers.
func TestW2DBCTwoConditionsOnOneCluster(t *testing.T) {
	doc := w2DocDBCluster("acme-docs-bad")
	doc.MultiAZ = aws.Bool(false)
	doc.MasterUsername = aws.String("admin")

	aur := w2AuroraCluster("acme-orders-bad")
	aur.MultiAZ = aws.Bool(false)
	aur.MasterUsername = aws.String("admin")

	docGot := w2DocDBFetch(t, doc)
	aurGot := w2AuroraFetch(t, aur)

	w2AssertFinding(t, docGot["acme-docs-bad"].Findings, w2DBCCodeSingleAZ, "single-AZ", domain.SevWarn, "wave1")
	w2AssertFinding(t, docGot["acme-docs-bad"].Findings, w2DBCCodeDefaultMaster, "default master username", domain.SevWarn, "wave1")
	w2AssertFinding(t, aurGot["acme-orders-bad"].Findings, w2DBCCodeSingleAZ, "single-AZ", domain.SevWarn, "wave1")
	w2AssertFinding(t, aurGot["acme-orders-bad"].Findings, w2DBCCodeDefaultMaster, "default master username", domain.SevWarn, "wave1")
}
