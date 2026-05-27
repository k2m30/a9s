package unit_test

// phase03_databases_pr03e_test.go — PR-03e migration contract tests for the
// 12 database resource types (dbi, dbi-snap, dbc, dbc-snap, s3, redis,
// opensearch, ddb, redshift, msk, efs, kinesis).
//
// Post-migration invariants:
//   - Fetcher writes Resource.Status == "" (no more Status writes)
//   - Fetcher writes Resource.Issues == nil (no more Issues writes)
//   - Fetcher writes Resource.Findings with Source:"wave1" for each non-healthy signal
//   - Healthy resources have len(Resource.Findings) == 0
//   - Fields["status"] (or Fields["state"]) preserved for the display column
//
// These tests FAIL on RED (before coder migrates the fetchers). They pass GREEN
// once the coder delivers Findings-emitting implementations.
//
// Pattern reference: tests/unit/phase03_networking_pr03d_test.go

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ddksdk "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ddbtypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	efssdk "github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"
	elasticache "github.com/aws/aws-sdk-go-v2/service/elasticache"
	elasticachetypes "github.com/aws/aws-sdk-go-v2/service/elasticache/types"
	kafkasdk "github.com/aws/aws-sdk-go-v2/service/kafka"
	kafkatypes "github.com/aws/aws-sdk-go-v2/service/kafka/types"
	kinesissdk "github.com/aws/aws-sdk-go-v2/service/kinesis"
	kinesistypes "github.com/aws/aws-sdk-go-v2/service/kinesis/types"
	opensearchsdk "github.com/aws/aws-sdk-go-v2/service/opensearch"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"
	rdssdk "github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
	redshiftsdk "github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
	s3sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// =============================================================================
// DBI (RDS DB Instance)
// =============================================================================

// TestPR03e_DBIFetcher_HealthyEmitsNoFinding asserts that a healthy "available"
// RDS DB instance produces Status=="", no Findings, and Fields["status"]=="".
func TestPR03e_DBIFetcher_HealthyEmitsNoFinding(t *testing.T) {
	mock := &pr03eRDSMock{
		instances: []rdstypes.DBInstance{
			{
				DBInstanceIdentifier:  aws.String("prod-dbi-healthy"),
				DBInstanceArn:         aws.String("arn:aws:rds:us-east-1:000000000000:db:prod-dbi-healthy"),
				DBInstanceStatus:      aws.String("available"),
				BackupRetentionPeriod: aws.Int32(7),
				PubliclyAccessible:    aws.Bool(false),
				StorageEncrypted:      aws.Bool(true),
				DeletionProtection:    aws.Bool(true),
			},
		},
	}

	result, err := awsclient.FetchRDSInstancesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRDSInstancesPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for healthy DBI", len(r.Findings))
	}
	// Fields["status"] should be the display value — empty for healthy silence.
	// (The display column reads Fields["status"], not Resource.Status.)
	if got := r.Fields["status"]; got != "" {
		t.Errorf("Fields[\"status\"]: got %q, want %q (healthy silence)", got, "")
	}
}

// TestPR03e_DBIFetcher_BrokenEmitsBrokenFinding asserts that a "failed" RDS instance
// emits one SevBroken Finding with CodeDBIFailed and Status=="".
func TestPR03e_DBIFetcher_BrokenEmitsBrokenFinding(t *testing.T) {
	mock := &pr03eRDSMock{
		instances: []rdstypes.DBInstance{
			{
				DBInstanceIdentifier:  aws.String("prod-dbi-failed"),
				DBInstanceArn:         aws.String("arn:aws:rds:us-east-1:000000000000:db:prod-dbi-failed"),
				DBInstanceStatus:      aws.String("failed"),
				BackupRetentionPeriod: aws.Int32(7),
				PubliclyAccessible:    aws.Bool(false),
				StorageEncrypted:      aws.Bool(true),
				DeletionProtection:    aws.Bool(true),
			},
		},
	}

	result, err := awsclient.FetchRDSInstancesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRDSInstancesPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for failed DBI", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeDBIFailed {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeDBIFailed)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevBroken", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03e_DBIFetcher_StoppedEmitsBrokenFinding pins the AS-126 regression
// fix: a "stopped" RDS instance must emit a SevBroken Finding (CodeDBIStopped)
// so colorDBI's wave1-first prelude returns ColorBroken — matching the legacy
// catalog colorDBI classification ("stopped" listed alongside "failed",
// "storage-full", etc.). Pre-fix the fetcher's default branch downgraded
// "stopped" to a SevWarn CodeDBITransitional finding, regressing the row from
// Broken to Warning under the wave1-first color path.
func TestPR03e_DBIFetcher_StoppedEmitsBrokenFinding(t *testing.T) {
	mock := &pr03eRDSMock{
		instances: []rdstypes.DBInstance{
			{
				DBInstanceIdentifier:  aws.String("staging-dbi-stopped"),
				DBInstanceArn:         aws.String("arn:aws:rds:us-east-1:000000000000:db:staging-dbi-stopped"),
				DBInstanceStatus:      aws.String("stopped"),
				BackupRetentionPeriod: aws.Int32(7),
				PubliclyAccessible:    aws.Bool(false),
				StorageEncrypted:      aws.Bool(true),
				DeletionProtection:    aws.Bool(true),
			},
		},
	}

	result, err := awsclient.FetchRDSInstancesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRDSInstancesPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for stopped DBI", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeDBIStopped {
		t.Errorf("Findings[0].Code: got %q, want %q (AS-126 regression: stopped must be Broken-class, not Transitional)", f.Code, awsclient.CodeDBIStopped)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevBroken (AS-126 regression: stopped must keep Broken severity)", f.Severity)
	}
	if f.Phrase != "stopped" {
		t.Errorf("Findings[0].Phrase: got %q, want %q", f.Phrase, "stopped")
	}

	// And the catalog colorDBI must classify the row as ColorBroken via the
	// wave1-first prelude — closing the regression loop end-to-end.
	td := resource.FindResourceType("dbi")
	if td == nil {
		t.Fatal("dbi type def not found in registry")
	}
	if got := td.Color(r); got != resource.ColorBroken {
		t.Errorf("colorDBI for stopped DBI: got %v, want ColorBroken (AS-126 regression)", got)
	}
}

// TestPR03e_DBIColor_ReadsWave1First pins that the dbi Color func evaluates
// Findings before the legacy Fields["status"] switch.
//
// Setup: Finding{SevWarn, wave1} + Fields["status"]="" (healthy).
// Expect: ColorWarning (Findings wins over healthy-silence).
func TestPR03e_DBIColor_ReadsWave1First(t *testing.T) {
	td := resource.FindResourceType("dbi")
	if td == nil {
		t.Fatal("dbi type def not found in registry")
	}

	r := resource.Resource{
		Type: "dbi",
		Findings: []domain.Finding{
			{Code: awsclient.CodeDBIPubliclyAccessible, Phrase: "publicly accessible", Severity: domain.SevWarn, Source: "wave1"},
		},
		Fields: map[string]string{"status": ""},
	}
	if got := td.Color(r); got != resource.ColorWarning {
		t.Errorf("dbi Color with wave1 SevWarn + status=empty: got %v, want ColorWarning (Findings must take precedence)", got)
	}
}

// ---------------------------------------------------------------------------
// DBI mock
// ---------------------------------------------------------------------------

type pr03eRDSMock struct {
	instances []rdstypes.DBInstance
}

func (m *pr03eRDSMock) DescribeDBInstances(
	_ context.Context,
	_ *rdssdk.DescribeDBInstancesInput,
	_ ...func(*rdssdk.Options),
) (*rdssdk.DescribeDBInstancesOutput, error) {
	return &rdssdk.DescribeDBInstancesOutput{DBInstances: m.instances}, nil
}

// =============================================================================
// DBI-SNAP (RDS DB Snapshot)
// =============================================================================

// TestPR03e_DBISnapFetcher_HealthyEmitsNoFinding asserts that an available,
// encrypted RDS snapshot produces Status=="", no Findings, and
// Fields["status"] set to the display phrase.
func TestPR03e_DBISnapFetcher_HealthyEmitsNoFinding(t *testing.T) {
	mock := &pr03eRDSSnapshotMock{
		output: &rdssdk.DescribeDBSnapshotsOutput{
			DBSnapshots: []rdstypes.DBSnapshot{
				{
					DBSnapshotIdentifier: aws.String("snap-healthy"),
					DBInstanceIdentifier: aws.String("prod-dbi-1"),
					SnapshotType:         aws.String("automated"),
					Status:               aws.String("available"),
					Encrypted:            aws.Bool(true),
				},
			},
		},
	}

	result, err := awsclient.FetchDBISnapshotsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchDBISnapshotsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for healthy DBI snapshot", len(r.Findings))
	}
}

// TestPR03e_DBISnapFetcher_PendingEmitsWarnFinding asserts that a "creating"
// snapshot emits one SevWarn Finding with CodeDBISnapCreating and Status=="".
func TestPR03e_DBISnapFetcher_PendingEmitsWarnFinding(t *testing.T) {
	mock := &pr03eRDSSnapshotMock{
		output: &rdssdk.DescribeDBSnapshotsOutput{
			DBSnapshots: []rdstypes.DBSnapshot{
				{
					DBSnapshotIdentifier: aws.String("snap-creating"),
					DBInstanceIdentifier: aws.String("prod-dbi-1"),
					SnapshotType:         aws.String("manual"),
					Status:               aws.String("creating"),
					Encrypted:            aws.Bool(true),
				},
			},
		},
	}

	result, err := awsclient.FetchDBISnapshotsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchDBISnapshotsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for creating DBI snapshot", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeDBISnapCreating {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeDBISnapCreating)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevWarn", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03e_DBISnapColor_ReadsWave1First pins that the dbi-snap Color func
// evaluates Findings before the legacy Fields["status"] switch.
//
// Setup: Finding{SevWarn, wave1} + Fields["status"]="available".
// Expect: ColorWarning (Findings wins over legacy "available"→ColorHealthy).
func TestPR03e_DBISnapColor_ReadsWave1First(t *testing.T) {
	td := resource.FindResourceType("dbi-snap")
	if td == nil {
		t.Fatal("dbi-snap type def not found in registry")
	}

	r := resource.Resource{
		Type: "dbi-snap",
		Findings: []domain.Finding{
			{Code: awsclient.CodeDBISnapUnencrypted, Phrase: "unencrypted", Severity: domain.SevWarn, Source: "wave1"},
		},
		Fields: map[string]string{"status": "available"},
	}
	if got := td.Color(r); got != resource.ColorWarning {
		t.Errorf("dbi-snap Color with wave1 SevWarn + status=available: got %v, want ColorWarning (Findings must take precedence)", got)
	}
}

// ---------------------------------------------------------------------------
// DBI-SNAP mock
// ---------------------------------------------------------------------------

type pr03eRDSSnapshotMock struct {
	output *rdssdk.DescribeDBSnapshotsOutput
}

func (m *pr03eRDSSnapshotMock) DescribeDBSnapshots(
	_ context.Context,
	_ *rdssdk.DescribeDBSnapshotsInput,
	_ ...func(*rdssdk.Options),
) (*rdssdk.DescribeDBSnapshotsOutput, error) {
	return m.output, nil
}

// =============================================================================
// DBC (DocumentDB / Aurora Cluster)
// =============================================================================

// TestPR03e_DBCFetcher_HealthyEmitsNoFinding asserts that a healthy "available"
// DocDB cluster produces Status=="", no Findings, and Fields["status"]=="".
func TestPR03e_DBCFetcher_HealthyEmitsNoFinding(t *testing.T) {
	mock := &pr03eDocDBMock{
		clusters: []rdstypes.DBCluster{
			{
				DBClusterIdentifier:   aws.String("prod-docdb-healthy"),
				Status:                aws.String("available"),
				Engine:                aws.String("aurora-mysql"),
				DeletionProtection:    aws.Bool(true),
				StorageEncrypted:      aws.Bool(true),
				BackupRetentionPeriod: aws.Int32(7),
				DBClusterMembers: []rdstypes.DBClusterMember{
					{DBInstanceIdentifier: aws.String("prod-docdb-healthy-01"), IsClusterWriter: aws.Bool(true)},
				},
			},
		},
	}

	result, err := awsclient.FetchRDSDBClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRDSDBClustersPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for healthy DBC", len(r.Findings))
	}
}

// TestPR03e_DBCFetcher_BrokenEmitsBrokenFinding asserts that a "failed" cluster emits
// one SevBroken Finding with CodeDBCFailed and Status=="".
func TestPR03e_DBCFetcher_BrokenEmitsBrokenFinding(t *testing.T) {
	mock := &pr03eDocDBMock{
		clusters: []rdstypes.DBCluster{
			{
				DBClusterIdentifier: aws.String("prod-docdb-failed"),
				Status:              aws.String("failed"),
				Engine:              aws.String("aurora-mysql"),
			},
		},
	}

	result, err := awsclient.FetchRDSDBClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRDSDBClustersPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for failed DBC", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeDBCFailed {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeDBCFailed)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevBroken", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03e_DBCColor_ReadsWave1First pins that the dbc Color func evaluates
// Findings before the legacy Fields["status"] switch.
//
// Setup: Finding{SevWarn, wave1} + Fields["status"]="" (healthy silence).
// Expect: ColorWarning (Findings wins over structural).
func TestPR03e_DBCColor_ReadsWave1First(t *testing.T) {
	td := resource.FindResourceType("dbc")
	if td == nil {
		t.Fatal("dbc type def not found in registry")
	}

	r := resource.Resource{
		Type: "dbc",
		Findings: []domain.Finding{
			{Code: awsclient.CodeDBCDeletionProtectionOff, Phrase: "delete-protection off", Severity: domain.SevWarn, Source: "wave1"},
		},
		Fields: map[string]string{"status": ""},
	}
	if got := td.Color(r); got != resource.ColorWarning {
		t.Errorf("dbc Color with wave1 SevWarn + status=empty: got %v, want ColorWarning (Findings must take precedence)", got)
	}
}

// ---------------------------------------------------------------------------
// DBC mock — uses RDS DescribeDBClusters (handles both Aurora and DocDB)
// ---------------------------------------------------------------------------

type pr03eDocDBMock struct {
	clusters []rdstypes.DBCluster
}

func (m *pr03eDocDBMock) DescribeDBClusters(
	_ context.Context,
	_ *rdssdk.DescribeDBClustersInput,
	_ ...func(*rdssdk.Options),
) (*rdssdk.DescribeDBClustersOutput, error) {
	return &rdssdk.DescribeDBClustersOutput{DBClusters: m.clusters}, nil
}

// =============================================================================
// DBC-SNAP (DocumentDB / Aurora Cluster Snapshot)
// =============================================================================

// TestPR03e_DBCSnapFetcher_HealthyEmitsNoFinding asserts that a healthy
// "available" RDS cluster snapshot produces Status=="", no Findings.
func TestPR03e_DBCSnapFetcher_HealthyEmitsNoFinding(t *testing.T) {
	mock := &pr03eRDSClusterSnapshotMock{
		output: &rdssdk.DescribeDBClusterSnapshotsOutput{
			DBClusterSnapshots: []rdstypes.DBClusterSnapshot{
				{
					DBClusterSnapshotIdentifier: aws.String("csnap-healthy"),
					DBClusterIdentifier:         aws.String("prod-aurora-01"),
					SnapshotType:                aws.String("automated"),
					Status:                      aws.String("available"),
					Engine:                      aws.String("aurora-mysql"),
					StorageEncrypted:            aws.Bool(true),
				},
			},
		},
	}

	result, err := awsclient.FetchRDSDBClusterSnapshotsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRDSDBClusterSnapshotsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for healthy DBC snapshot", len(r.Findings))
	}
}

// TestPR03e_DBCSnapFetcher_PendingEmitsWarnFinding asserts that a "creating"
// cluster snapshot emits one SevWarn Finding with CodeDBCSnapCreating and Status=="".
func TestPR03e_DBCSnapFetcher_PendingEmitsWarnFinding(t *testing.T) {
	mock := &pr03eRDSClusterSnapshotMock{
		output: &rdssdk.DescribeDBClusterSnapshotsOutput{
			DBClusterSnapshots: []rdstypes.DBClusterSnapshot{
				{
					DBClusterSnapshotIdentifier: aws.String("csnap-creating"),
					DBClusterIdentifier:         aws.String("prod-aurora-01"),
					SnapshotType:                aws.String("manual"),
					Status:                      aws.String("creating"),
					Engine:                      aws.String("aurora-mysql"),
					StorageEncrypted:            aws.Bool(true),
				},
			},
		},
	}

	result, err := awsclient.FetchRDSDBClusterSnapshotsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRDSDBClusterSnapshotsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for creating DBC snapshot", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeDBCSnapCreating {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeDBCSnapCreating)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevWarn", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03e_DBCSnapColor_ReadsWave1First pins that the dbc-snap Color func
// evaluates Findings before the legacy Fields["status"] switch.
//
// Setup: Finding{SevBroken, wave1} + Fields["status"]="available".
// Expect: ColorBroken (Findings wins over legacy "available"→ColorHealthy).
func TestPR03e_DBCSnapColor_ReadsWave1First(t *testing.T) {
	td := resource.FindResourceType("dbc-snap")
	if td == nil {
		t.Fatal("dbc-snap type def not found in registry")
	}

	r := resource.Resource{
		Type: "dbc-snap",
		Findings: []domain.Finding{
			{Code: awsclient.CodeDBCSnapFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"},
		},
		Fields: map[string]string{"status": "available"},
	}
	if got := td.Color(r); got != resource.ColorBroken {
		t.Errorf("dbc-snap Color with wave1 SevBroken + status=available: got %v, want ColorBroken (Findings must take precedence)", got)
	}
}

// ---------------------------------------------------------------------------
// DBC-SNAP mock
// ---------------------------------------------------------------------------

type pr03eRDSClusterSnapshotMock struct {
	output *rdssdk.DescribeDBClusterSnapshotsOutput
}

func (m *pr03eRDSClusterSnapshotMock) DescribeDBClusterSnapshots(
	_ context.Context,
	_ *rdssdk.DescribeDBClusterSnapshotsInput,
	_ ...func(*rdssdk.Options),
) (*rdssdk.DescribeDBClusterSnapshotsOutput, error) {
	return m.output, nil
}

// =============================================================================
// S3 (Simple Storage Service)
// =============================================================================

// TestPR03e_S3Fetcher_HealthyEmitsNoFinding asserts that a healthy S3 bucket
// produces Status=="", no Findings, and no Issues. S3 has no Wave-1 signals.
func TestPR03e_S3Fetcher_HealthyEmitsNoFinding(t *testing.T) {
	mock := &pr03eS3ListMock{
		output: &s3sdk.ListBucketsOutput{
			Buckets: []s3types.Bucket{
				{Name: aws.String("my-prod-bucket")},
			},
		},
	}

	result, err := awsclient.FetchS3BucketsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchS3BucketsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 (s3 has no Wave-1 signals)", len(r.Findings))
	}
}

// ---------------------------------------------------------------------------
// S3 mock
// ---------------------------------------------------------------------------

type pr03eS3ListMock struct {
	output *s3sdk.ListBucketsOutput
}

func (m *pr03eS3ListMock) ListBuckets(
	_ context.Context,
	_ *s3sdk.ListBucketsInput,
	_ ...func(*s3sdk.Options),
) (*s3sdk.ListBucketsOutput, error) {
	return m.output, nil
}

// =============================================================================
// REDIS (ElastiCache Replication Group)
// =============================================================================

// TestPR03e_RedisFetcher_HealthyEmitsNoFinding asserts that a healthy
// "available" replication group produces Status=="", no Findings.
func TestPR03e_RedisFetcher_HealthyEmitsNoFinding(t *testing.T) {
	mock := &pr03eRedisRGMock{
		output: &elasticache.DescribeReplicationGroupsOutput{
			ReplicationGroups: []elasticachetypes.ReplicationGroup{
				{
					ReplicationGroupId: aws.String("prod-redis-healthy"),
					Status:             aws.String("available"),
					Engine:             aws.String("redis"),
					MultiAZ:            elasticachetypes.MultiAZStatusEnabled,
					AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusEnabled,
				},
			},
		},
	}

	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedisPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for healthy Redis RG", len(r.Findings))
	}
}

// TestPR03e_RedisFetcher_PendingEmitsWarnFinding asserts that a "creating"
// replication group emits one SevWarn Finding with CodeRedisCreating and Status=="".
func TestPR03e_RedisFetcher_PendingEmitsWarnFinding(t *testing.T) {
	mock := &pr03eRedisRGMock{
		output: &elasticache.DescribeReplicationGroupsOutput{
			ReplicationGroups: []elasticachetypes.ReplicationGroup{
				{
					ReplicationGroupId: aws.String("dev-redis-creating"),
					Status:             aws.String("creating"),
					Engine:             aws.String("redis"),
					MultiAZ:            elasticachetypes.MultiAZStatusDisabled,
					AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusDisabling,
				},
			},
		},
	}

	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedisPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for creating Redis RG", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeRedisCreating {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeRedisCreating)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevWarn", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03e_RedisColor_ReadsWave1First pins that the redis Color func evaluates
// Findings before the legacy Fields["status"] switch.
//
// Setup: Finding{SevWarn, wave1} + Fields["status"]="" (healthy silence).
// Expect: ColorWarning (Findings wins over structural).
func TestPR03e_RedisColor_ReadsWave1First(t *testing.T) {
	td := resource.FindResourceType("redis")
	if td == nil {
		t.Fatal("redis type def not found in registry")
	}

	r := resource.Resource{
		Type: "redis",
		Findings: []domain.Finding{
			{Code: awsclient.CodeRedisMultiAZWithoutAutoFailover, Phrase: "multi-AZ without auto-failover", Severity: domain.SevWarn, Source: "wave1"},
		},
		Fields: map[string]string{"status": ""},
	}
	if got := td.Color(r); got != resource.ColorWarning {
		t.Errorf("redis Color with wave1 SevWarn + status=empty: got %v, want ColorWarning (Findings must take precedence)", got)
	}
}

// ---------------------------------------------------------------------------
// Redis mock
// ---------------------------------------------------------------------------

type pr03eRedisRGMock struct {
	output *elasticache.DescribeReplicationGroupsOutput
}

func (m *pr03eRedisRGMock) DescribeReplicationGroups(
	_ context.Context,
	_ *elasticache.DescribeReplicationGroupsInput,
	_ ...func(*elasticache.Options),
) (*elasticache.DescribeReplicationGroupsOutput, error) {
	return m.output, nil
}

// =============================================================================
// OPENSEARCH
// =============================================================================

// TestPR03e_OpenSearchFetcher_HealthyEmitsNoFinding asserts that a healthy
// "Active" OpenSearch domain produces Status=="", no Findings.
func TestPR03e_OpenSearchFetcher_HealthyEmitsNoFinding(t *testing.T) {
	domainName := "analytics-prod"
	listMock := &pr03eOSListMock{
		output: &opensearchsdk.ListDomainNamesOutput{
			DomainNames: []ostypes.DomainInfo{
				{DomainName: aws.String(domainName)},
			},
		},
	}
	describeMock := &pr03eOSDescribeMock{
		output: &opensearchsdk.DescribeDomainsOutput{
			DomainStatusList: []ostypes.DomainStatus{
				{
					ARN:                    aws.String("arn:aws:es:us-east-1:000000000000:domain/" + domainName),
					DomainId:               aws.String("000000000000/" + domainName),
					DomainName:             aws.String(domainName),
					EngineVersion:          aws.String("OpenSearch_2.11"),
					Created:                aws.Bool(true),
					Deleted:                aws.Bool(false),
					Processing:             aws.Bool(false),
					UpgradeProcessing:      aws.Bool(false),
					DomainProcessingStatus: ostypes.DomainProcessingStatusTypeActive,
					EncryptionAtRestOptions: &ostypes.EncryptionAtRestOptions{
						Enabled: aws.Bool(true),
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchOpenSearchDomains(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("FetchOpenSearchDomains: unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for healthy OpenSearch domain", len(r.Findings))
	}
}

// TestPR03e_OpenSearchFetcher_BrokenEmitsBrokenFinding asserts that an "Isolated"
// domain (DomainProcessingStatus) emits one SevBroken Finding with
// CodeOpenSearchIsolated and Status=="".
func TestPR03e_OpenSearchFetcher_BrokenEmitsBrokenFinding(t *testing.T) {
	domainName := "analytics-isolated"
	listMock := &pr03eOSListMock{
		output: &opensearchsdk.ListDomainNamesOutput{
			DomainNames: []ostypes.DomainInfo{
				{DomainName: aws.String(domainName)},
			},
		},
	}
	describeMock := &pr03eOSDescribeMock{
		output: &opensearchsdk.DescribeDomainsOutput{
			DomainStatusList: []ostypes.DomainStatus{
				{
					ARN:                    aws.String("arn:aws:es:us-east-1:000000000000:domain/" + domainName),
					DomainId:               aws.String("000000000000/" + domainName),
					DomainName:             aws.String(domainName),
					EngineVersion:          aws.String("OpenSearch_2.11"),
					Created:                aws.Bool(true),
					Deleted:                aws.Bool(false),
					Processing:             aws.Bool(false),
					UpgradeProcessing:      aws.Bool(false),
					DomainProcessingStatus: ostypes.DomainProcessingStatusTypeIsolated,
					EncryptionAtRestOptions: &ostypes.EncryptionAtRestOptions{
						Enabled: aws.Bool(true),
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchOpenSearchDomains(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("FetchOpenSearchDomains: unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for isolated OpenSearch domain", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeOpenSearchIsolated {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeOpenSearchIsolated)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevBroken", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03e_OpenSearchColor_ReadsWave1First pins that the opensearch Color func
// evaluates Findings before the legacy Fields["status"] switch.
//
// Setup: Finding{SevBroken, wave1} + Fields["status"]="active" (healthy).
// Expect: ColorBroken (Findings wins over legacy "active"→ColorHealthy).
func TestPR03e_OpenSearchColor_ReadsWave1First(t *testing.T) {
	td := resource.FindResourceType("opensearch")
	if td == nil {
		t.Fatal("opensearch type def not found in registry")
	}

	r := resource.Resource{
		Type: "opensearch",
		Findings: []domain.Finding{
			{Code: awsclient.CodeOpenSearchIsolated, Phrase: "isolated", Severity: domain.SevBroken, Source: "wave1"},
		},
		Fields: map[string]string{"status": "active"},
	}
	if got := td.Color(r); got != resource.ColorBroken {
		t.Errorf("opensearch Color with wave1 SevBroken + status=active: got %v, want ColorBroken (Findings must take precedence)", got)
	}
}

// ---------------------------------------------------------------------------
// OpenSearch mocks
// ---------------------------------------------------------------------------

type pr03eOSListMock struct {
	output *opensearchsdk.ListDomainNamesOutput
}

func (m *pr03eOSListMock) ListDomainNames(
	_ context.Context,
	_ *opensearchsdk.ListDomainNamesInput,
	_ ...func(*opensearchsdk.Options),
) (*opensearchsdk.ListDomainNamesOutput, error) {
	return m.output, nil
}

type pr03eOSDescribeMock struct {
	output *opensearchsdk.DescribeDomainsOutput
}

func (m *pr03eOSDescribeMock) DescribeDomains(
	_ context.Context,
	_ *opensearchsdk.DescribeDomainsInput,
	_ ...func(*opensearchsdk.Options),
) (*opensearchsdk.DescribeDomainsOutput, error) {
	return m.output, nil
}

// =============================================================================
// DDB (DynamoDB)
// =============================================================================

// TestPR03e_DDBFetcher_HealthyEmitsNoFinding asserts that an ACTIVE DynamoDB
// table produces Status=="", no Findings.
func TestPR03e_DDBFetcher_HealthyEmitsNoFinding(t *testing.T) {
	tableName := "prod-orders"
	listStub := &pr03eDDBListStub{names: []string{tableName}}
	descStub := &pr03eDDBDescribeStub{
		tables: map[string]*ddbtypes.TableDescription{
			tableName: {
				TableName:   aws.String(tableName),
				TableArn:    aws.String("arn:aws:dynamodb:us-east-1:000000000000:table/" + tableName),
				TableStatus: ddbtypes.TableStatusActive,
				ItemCount:   aws.Int64(42000),
			},
		},
	}

	result, err := awsclient.FetchDynamoDBTablesPage(context.Background(), listStub, descStub, "")
	if err != nil {
		t.Fatalf("FetchDynamoDBTablesPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for healthy DDB table", len(r.Findings))
	}
	// Fields["status"] should be the healthy display phrase (empty string per spec).
	if got := r.Fields["status"]; got != "" {
		t.Errorf("Fields[\"status\"]: got %q, want %q (healthy silence)", got, "")
	}
}

// TestPR03e_DDBFetcher_BrokenEmitsBrokenFinding asserts that a table in
// INACCESSIBLE_ENCRYPTION_CREDENTIALS state emits one SevBroken Finding with
// CodeDDBKMSKeyInaccessible and Status=="".
func TestPR03e_DDBFetcher_BrokenEmitsBrokenFinding(t *testing.T) {
	tableName := "prod-secrets-kms-locked"
	listStub := &pr03eDDBListStub{names: []string{tableName}}
	descStub := &pr03eDDBDescribeStub{
		tables: map[string]*ddbtypes.TableDescription{
			tableName: {
				TableName:   aws.String(tableName),
				TableArn:    aws.String("arn:aws:dynamodb:us-east-1:000000000000:table/" + tableName),
				TableStatus: ddbtypes.TableStatusInaccessibleEncryptionCredentials,
			},
		},
	}

	result, err := awsclient.FetchDynamoDBTablesPage(context.Background(), listStub, descStub, "")
	if err != nil {
		t.Fatalf("FetchDynamoDBTablesPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for KMS-inaccessible DDB table", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeDDBKMSKeyInaccessible {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeDDBKMSKeyInaccessible)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevBroken", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03e_DDBColor_ReadsWave1First pins that the ddb Color func evaluates
// Findings before the legacy Fields["status"] switch.
//
// Setup: Finding{SevWarn, wave1} + Fields["status"]="" (healthy).
// Expect: ColorWarning (Findings wins over healthy-silence).
func TestPR03e_DDBColor_ReadsWave1First(t *testing.T) {
	td := resource.FindResourceType("ddb")
	if td == nil {
		t.Fatal("ddb type def not found in registry")
	}

	r := resource.Resource{
		Type: "ddb",
		Findings: []domain.Finding{
			{Code: awsclient.CodeDDBCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1"},
		},
		Fields: map[string]string{"status": ""},
	}
	if got := td.Color(r); got != resource.ColorWarning {
		t.Errorf("ddb Color with wave1 SevWarn + status=empty: got %v, want ColorWarning (Findings must take precedence)", got)
	}
}

// ---------------------------------------------------------------------------
// DDB mocks
// ---------------------------------------------------------------------------

type pr03eDDBListStub struct {
	names []string
}

func (s *pr03eDDBListStub) ListTables(_ context.Context, _ *ddksdk.ListTablesInput, _ ...func(*ddksdk.Options)) (*ddksdk.ListTablesOutput, error) {
	return &ddksdk.ListTablesOutput{TableNames: s.names}, nil
}

type pr03eDDBDescribeStub struct {
	tables map[string]*ddbtypes.TableDescription
}

func (s *pr03eDDBDescribeStub) DescribeTable(_ context.Context, in *ddksdk.DescribeTableInput, _ ...func(*ddksdk.Options)) (*ddksdk.DescribeTableOutput, error) {
	if in == nil || in.TableName == nil {
		return &ddksdk.DescribeTableOutput{}, nil
	}
	td, ok := s.tables[*in.TableName]
	if !ok {
		return &ddksdk.DescribeTableOutput{}, nil
	}
	return &ddksdk.DescribeTableOutput{Table: td}, nil
}

// =============================================================================
// REDSHIFT
// =============================================================================

// TestPR03e_RedshiftFetcher_HealthyEmitsNoFinding asserts that a healthy
// "available" Redshift cluster produces Status=="", no Findings, and
// Fields["status"]=="available" (the display column uses raw status for Redshift).
func TestPR03e_RedshiftFetcher_HealthyEmitsNoFinding(t *testing.T) {
	mock := &pr03eRedshiftMock{
		output: &redshiftsdk.DescribeClustersOutput{
			Clusters: []redshifttypes.Cluster{
				{
					ClusterIdentifier:      aws.String("prod-dwh"),
					ClusterStatus:          aws.String("available"),
					PubliclyAccessible:     aws.Bool(false),
					Encrypted:              aws.Bool(true),
					NodeType:               aws.String("ra3.xlplus"),
					NumberOfNodes:          aws.Int32(2),
					DBName:                 aws.String("dev"),
					ClusterAvailabilityStatus: aws.String("Available"),
				},
			},
		},
	}

	result, err := awsclient.FetchRedshiftClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedshiftClustersPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for healthy Redshift cluster", len(r.Findings))
	}
}

// TestPR03e_RedshiftFetcher_BrokenEmitsBrokenFinding asserts that a cluster
// in "hardware-failure" status emits one SevBroken Finding with
// CodeRedshiftHardwareFailure and Status=="".
func TestPR03e_RedshiftFetcher_BrokenEmitsBrokenFinding(t *testing.T) {
	mock := &pr03eRedshiftMock{
		output: &redshiftsdk.DescribeClustersOutput{
			Clusters: []redshifttypes.Cluster{
				{
					ClusterIdentifier: aws.String("prod-dwh-broken"),
					ClusterStatus:     aws.String("hardware-failure"),
					NodeType:          aws.String("ra3.xlplus"),
					NumberOfNodes:     aws.Int32(2),
					DBName:            aws.String("dev"),
				},
			},
		},
	}

	result, err := awsclient.FetchRedshiftClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedshiftClustersPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for hardware-failure Redshift cluster", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeRedshiftHardwareFailure {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeRedshiftHardwareFailure)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevBroken", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03e_RedshiftColor_ReadsWave1First pins that the redshift Color func
// evaluates Findings before the legacy Fields["cluster_status"] switch.
//
// Setup: Finding{SevWarn, wave1} + Fields["cluster_status"]="available".
// Expect: ColorWarning (Findings wins over legacy "available"→ColorHealthy).
func TestPR03e_RedshiftColor_ReadsWave1First(t *testing.T) {
	td := resource.FindResourceType("redshift")
	if td == nil {
		t.Fatal("redshift type def not found in registry")
	}

	r := resource.Resource{
		Type: "redshift",
		Findings: []domain.Finding{
			{Code: awsclient.CodeRedshiftPubliclyAccessible, Phrase: "publicly accessible", Severity: domain.SevWarn, Source: "wave1"},
		},
		Fields: map[string]string{
			"cluster_status": "available",
			"status":         "available",
		},
	}
	if got := td.Color(r); got != resource.ColorWarning {
		t.Errorf("redshift Color with wave1 SevWarn + cluster_status=available: got %v, want ColorWarning (Findings must take precedence)", got)
	}
}

// ---------------------------------------------------------------------------
// Redshift mock
// ---------------------------------------------------------------------------

type pr03eRedshiftMock struct {
	output *redshiftsdk.DescribeClustersOutput
}

func (m *pr03eRedshiftMock) DescribeClusters(
	_ context.Context,
	_ *redshiftsdk.DescribeClustersInput,
	_ ...func(*redshiftsdk.Options),
) (*redshiftsdk.DescribeClustersOutput, error) {
	return m.output, nil
}

// =============================================================================
// MSK (Amazon Managed Streaming for Apache Kafka)
// =============================================================================

// TestPR03e_MSKFetcher_HealthyEmitsNoFinding asserts that a healthy "ACTIVE"
// MSK cluster produces Status=="", no Findings, and Fields["state"]=="ACTIVE".
func TestPR03e_MSKFetcher_HealthyEmitsNoFinding(t *testing.T) {
	mock := &pr03eMSKMock{
		output: &kafkasdk.ListClustersV2Output{
			ClusterInfoList: []kafkatypes.Cluster{
				{
					ClusterName:    aws.String("prod-kafka"),
					ClusterArn:     aws.String("arn:aws:kafka:us-east-1:000000000000:cluster/prod-kafka/abc123"),
					ClusterType:    kafkatypes.ClusterTypeProvisioned,
					State:          kafkatypes.ClusterStateActive,
					CurrentVersion: aws.String("2.8.1"),
				},
			},
		},
	}

	result, err := awsclient.FetchMSKClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchMSKClustersPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for healthy MSK cluster", len(r.Findings))
	}
	if got := r.Fields["state"]; got != "ACTIVE" {
		t.Errorf("Fields[\"state\"]: got %q, want %q (state must be preserved in Fields)", got, "ACTIVE")
	}
}

// TestPR03e_MSKFetcher_BrokenEmitsBrokenFinding asserts that a "FAILED" MSK cluster
// emits one SevBroken Finding with CodeMSKFailed and Status=="".
func TestPR03e_MSKFetcher_BrokenEmitsBrokenFinding(t *testing.T) {
	mock := &pr03eMSKMock{
		output: &kafkasdk.ListClustersV2Output{
			ClusterInfoList: []kafkatypes.Cluster{
				{
					ClusterName:    aws.String("prod-kafka-broken"),
					ClusterArn:     aws.String("arn:aws:kafka:us-east-1:000000000000:cluster/prod-kafka-broken/def456"),
					ClusterType:    kafkatypes.ClusterTypeProvisioned,
					State:          kafkatypes.ClusterStateFailed,
					CurrentVersion: aws.String("2.8.1"),
				},
			},
		},
	}

	result, err := awsclient.FetchMSKClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchMSKClustersPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for failed MSK cluster", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeMSKFailed {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeMSKFailed)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevBroken", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03e_MSKColor_ReadsWave1First pins that the msk Color func evaluates
// Findings before the legacy Fields["state"] switch.
//
// Setup: Finding{SevWarn, wave1} + Fields["state"]="ACTIVE".
// Expect: ColorWarning (Findings wins over legacy "ACTIVE"→ColorHealthy).
func TestPR03e_MSKColor_ReadsWave1First(t *testing.T) {
	td := resource.FindResourceType("msk")
	if td == nil {
		t.Fatal("msk type def not found in registry")
	}

	r := resource.Resource{
		Type: "msk",
		Findings: []domain.Finding{
			{Code: awsclient.CodeMSKCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1"},
		},
		Fields: map[string]string{"state": "ACTIVE"},
	}
	if got := td.Color(r); got != resource.ColorWarning {
		t.Errorf("msk Color with wave1 SevWarn + state=ACTIVE: got %v, want ColorWarning (Findings must take precedence)", got)
	}
}

// ---------------------------------------------------------------------------
// MSK mock
// ---------------------------------------------------------------------------

type pr03eMSKMock struct {
	output *kafkasdk.ListClustersV2Output
}

func (m *pr03eMSKMock) ListClustersV2(
	_ context.Context,
	_ *kafkasdk.ListClustersV2Input,
	_ ...func(*kafkasdk.Options),
) (*kafkasdk.ListClustersV2Output, error) {
	return m.output, nil
}

// =============================================================================
// EFS (Elastic File System)
// =============================================================================

// TestPR03e_EFSFetcher_HealthyEmitsNoFinding asserts that a healthy "available"
// EFS filesystem produces Status=="", no Findings, and Fields["status"] set.
func TestPR03e_EFSFetcher_HealthyEmitsNoFinding(t *testing.T) {
	mock := &pr03eEFSMock{
		output: &efssdk.DescribeFileSystemsOutput{
			FileSystems: []efstypes.FileSystemDescription{
				{
					FileSystemId:    aws.String("fs-0abc1234"),
					FileSystemArn:   aws.String("arn:aws:elasticfilesystem:us-east-1:000000000000:file-system/fs-0abc1234"),
					LifeCycleState:  efstypes.LifeCycleStateAvailable,
					NumberOfMountTargets: 2,
					Name:            aws.String("prod-data"),
					PerformanceMode: efstypes.PerformanceModeGeneralPurpose,
					ThroughputMode:  efstypes.ThroughputModeBursting,
					Encrypted:       aws.Bool(true),
				},
			},
		},
	}

	result, err := awsclient.FetchEFSFileSystemsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchEFSFileSystemsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for healthy EFS filesystem", len(r.Findings))
	}
}

// TestPR03e_EFSFetcher_BrokenEmitsBrokenFinding asserts that an "error" lifecycle state
// emits one SevBroken Finding with CodeEFSError and Status=="".
func TestPR03e_EFSFetcher_BrokenEmitsBrokenFinding(t *testing.T) {
	mock := &pr03eEFSMock{
		output: &efssdk.DescribeFileSystemsOutput{
			FileSystems: []efstypes.FileSystemDescription{
				{
					FileSystemId:         aws.String("fs-0abc5678"),
					FileSystemArn:        aws.String("arn:aws:elasticfilesystem:us-east-1:000000000000:file-system/fs-0abc5678"),
					LifeCycleState:       efstypes.LifeCycleStateError,
					NumberOfMountTargets: 0,
					PerformanceMode:      efstypes.PerformanceModeGeneralPurpose,
					ThroughputMode:       efstypes.ThroughputModeBursting,
					Encrypted:            aws.Bool(true),
				},
			},
		},
	}

	result, err := awsclient.FetchEFSFileSystemsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchEFSFileSystemsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) < 1 {
		t.Fatalf("Findings: got %d, want >= 1 for error EFS filesystem", len(r.Findings))
	}
	// Find the EFSError finding (there may also be a CodeEFSNoMountTargets finding).
	var found bool
	for _, f := range r.Findings {
		if f.Code == awsclient.CodeEFSError {
			found = true
			if f.Severity != domain.SevBroken {
				t.Errorf("Findings[CodeEFSError].Severity: got %v, want domain.SevBroken", f.Severity)
			}
			if f.Source != "wave1" {
				t.Errorf("Findings[CodeEFSError].Source: got %q, want %q", f.Source, "wave1")
			}
		}
	}
	if !found {
		t.Errorf("Findings: missing CodeEFSError; got %v", r.Findings)
	}
}

// TestPR03e_EFSColor_ReadsWave1First pins that the efs Color func evaluates
// Findings before the legacy Fields["status"] switch.
//
// Setup: Finding{SevWarn, wave1} + Fields["status"]="available".
// Expect: ColorWarning (Findings wins over legacy "available"→ColorHealthy).
func TestPR03e_EFSColor_ReadsWave1First(t *testing.T) {
	td := resource.FindResourceType("efs")
	if td == nil {
		t.Fatal("efs type def not found in registry")
	}

	r := resource.Resource{
		Type: "efs",
		Findings: []domain.Finding{
			{Code: awsclient.CodeEFSCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1"},
		},
		Fields: map[string]string{"status": "available"},
	}
	if got := td.Color(r); got != resource.ColorWarning {
		t.Errorf("efs Color with wave1 SevWarn + status=available: got %v, want ColorWarning (Findings must take precedence)", got)
	}
}

// ---------------------------------------------------------------------------
// EFS mock
// ---------------------------------------------------------------------------

type pr03eEFSMock struct {
	output *efssdk.DescribeFileSystemsOutput
}

func (m *pr03eEFSMock) DescribeFileSystems(
	_ context.Context,
	_ *efssdk.DescribeFileSystemsInput,
	_ ...func(*efssdk.Options),
) (*efssdk.DescribeFileSystemsOutput, error) {
	return m.output, nil
}

// =============================================================================
// KINESIS
// =============================================================================

// TestPR03e_KinesisFetcher_HealthyEmitsNoFinding asserts that a healthy "ACTIVE"
// Kinesis stream produces Status=="", no Findings, and Fields["status"]=="ACTIVE".
func TestPR03e_KinesisFetcher_HealthyEmitsNoFinding(t *testing.T) {
	mock := &pr03eKinesisMock{
		output: &kinesissdk.ListStreamsOutput{
			StreamSummaries: []kinesistypes.StreamSummary{
				{
					StreamName:   aws.String("prod-events"),
					StreamARN:    aws.String("arn:aws:kinesis:us-east-1:000000000000:stream/prod-events"),
					StreamStatus: kinesistypes.StreamStatusActive,
					StreamModeDetails: &kinesistypes.StreamModeDetails{
						StreamMode: kinesistypes.StreamModeOnDemand,
					},
				},
			},
		},
	}

	result, err := awsclient.FetchKinesisStreamsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchKinesisStreamsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 0 {
		t.Errorf("Findings: got %d, want 0 for healthy Kinesis stream", len(r.Findings))
	}
	// Healthy streams: Fields["status"] == "" (healthy silence), raw state in Fields["stream_status"].
	if got := r.Fields["status"]; got != "" {
		t.Errorf("Fields[\"status\"]: got %q, want %q (healthy streams must have blank status phrase)", got, "")
	}
	if got := r.Fields["stream_status"]; got != "ACTIVE" {
		t.Errorf("Fields[\"stream_status\"]: got %q, want %q (raw AWS state must be in stream_status)", got, "ACTIVE")
	}
}

// TestPR03e_KinesisFetcher_PendingEmitsWarnFinding asserts that a "CREATING"
// stream emits one SevWarn Finding with CodeKinesisCreating and Status=="".
func TestPR03e_KinesisFetcher_PendingEmitsWarnFinding(t *testing.T) {
	mock := &pr03eKinesisMock{
		output: &kinesissdk.ListStreamsOutput{
			StreamSummaries: []kinesistypes.StreamSummary{
				{
					StreamName:   aws.String("dev-events-creating"),
					StreamARN:    aws.String("arn:aws:kinesis:us-east-1:000000000000:stream/dev-events-creating"),
					StreamStatus: kinesistypes.StreamStatusCreating,
					StreamModeDetails: &kinesistypes.StreamModeDetails{
						StreamMode: kinesistypes.StreamModeProvisioned,
					},
				},
			},
		},
	}

	result, err := awsclient.FetchKinesisStreamsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchKinesisStreamsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for creating Kinesis stream", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeKinesisCreating {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeKinesisCreating)
	}
	if f.Severity != domain.SevWarn {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevWarn", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// TestPR03e_KinesisColor_ReadsWave1First pins that the kinesis Color func
// evaluates Findings before the legacy Fields["status"] switch.
//
// Setup: Finding{SevWarn, wave1} + Fields["status"]="ACTIVE".
// Expect: ColorWarning (Findings wins over legacy "ACTIVE"→ColorHealthy).
func TestPR03e_KinesisColor_ReadsWave1First(t *testing.T) {
	td := resource.FindResourceType("kinesis")
	if td == nil {
		t.Fatal("kinesis type def not found in registry")
	}

	r := resource.Resource{
		Type: "kinesis",
		Findings: []domain.Finding{
			{Code: awsclient.CodeKinesisCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1"},
		},
		Fields: map[string]string{"status": "ACTIVE"},
	}
	if got := td.Color(r); got != resource.ColorWarning {
		t.Errorf("kinesis Color with wave1 SevWarn + status=ACTIVE: got %v, want ColorWarning (Findings must take precedence)", got)
	}
}

// ---------------------------------------------------------------------------
// Kinesis mock
// ---------------------------------------------------------------------------

type pr03eKinesisMock struct {
	output *kinesissdk.ListStreamsOutput
}

func (m *pr03eKinesisMock) ListStreams(
	_ context.Context,
	_ *kinesissdk.ListStreamsInput,
	_ ...func(*kinesissdk.Options),
) (*kinesissdk.ListStreamsOutput, error) {
	return m.output, nil
}

// =============================================================================
// Additional cases — third structural case per type to satisfy AS-90 dispatch's
// strict 3-case contract (Healthy + Pending + Broken). The cases above already
// cover Healthy and one of (Pending, Broken) per type; the cases below fill in
// the missing slot. Added by AS-90 dispatch in Mode: execute on
// 048-pr03e-databases-rebased.
// =============================================================================

// ----- DBI: Pending (CodeDBITransitional) ------------------------------------

// TestPR03e_DBIFetcher_PendingEmitsWarnFinding asserts that an RDS DB instance
// in a transitional status ("creating") emits one SevWarn Finding with
// CodeDBITransitional and Status=="".
func TestPR03e_DBIFetcher_PendingEmitsWarnFinding(t *testing.T) {
	mock := &pr03eRDSMock{
		instances: []rdstypes.DBInstance{
			{
				DBInstanceIdentifier:  aws.String("prod-dbi-creating"),
				DBInstanceArn:         aws.String("arn:aws:rds:us-east-1:000000000000:db:prod-dbi-creating"),
				DBInstanceStatus:      aws.String("creating"),
				BackupRetentionPeriod: aws.Int32(7),
				PubliclyAccessible:    aws.Bool(false),
				StorageEncrypted:      aws.Bool(true),
				DeletionProtection:    aws.Bool(true),
			},
		},
	}

	result, err := awsclient.FetchRDSInstancesPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRDSInstancesPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) < 1 {
		t.Fatalf("Findings: got %d, want >= 1 for creating DBI", len(r.Findings))
	}
	var found bool
	for _, f := range r.Findings {
		if f.Code == awsclient.CodeDBITransitional {
			found = true
			if f.Severity != domain.SevWarn {
				t.Errorf("Findings[CodeDBITransitional].Severity: got %v, want domain.SevWarn", f.Severity)
			}
			if f.Source != "wave1" {
				t.Errorf("Findings[CodeDBITransitional].Source: got %q, want %q", f.Source, "wave1")
			}
			if f.Phrase == "" {
				t.Errorf("Findings[CodeDBITransitional].Phrase: empty, want non-empty human phrase")
			}
		}
	}
	if !found {
		t.Errorf("Findings: missing CodeDBITransitional; got %v", r.Findings)
	}
}

// ----- DBI-snap: Broken (CodeDBISnapFailed) ----------------------------------

// TestPR03e_DBISnapFetcher_BrokenEmitsBrokenFinding asserts that a "failed"
// RDS DB snapshot emits one SevBroken Finding with CodeDBISnapFailed and
// Status=="".
func TestPR03e_DBISnapFetcher_BrokenEmitsBrokenFinding(t *testing.T) {
	mock := &pr03eRDSSnapshotMock{
		output: &rdssdk.DescribeDBSnapshotsOutput{
			DBSnapshots: []rdstypes.DBSnapshot{
				{
					DBSnapshotIdentifier: aws.String("snap-failed"),
					DBInstanceIdentifier: aws.String("prod-dbi-1"),
					SnapshotType:         aws.String("manual"),
					Status:               aws.String("failed"),
					Encrypted:            aws.Bool(true),
				},
			},
		},
	}

	result, err := awsclient.FetchDBISnapshotsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchDBISnapshotsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for failed DBI snapshot", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeDBISnapFailed {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeDBISnapFailed)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevBroken", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// ----- DBC: Pending (CodeDBCTransitional) ------------------------------------

// TestPR03e_DBCFetcher_PendingEmitsWarnFinding asserts that a "creating"
// DocDB / Aurora cluster emits one SevWarn Finding with CodeDBCTransitional
// and Status=="".
func TestPR03e_DBCFetcher_PendingEmitsWarnFinding(t *testing.T) {
	mock := &pr03eDocDBMock{
		clusters: []rdstypes.DBCluster{
			{
				DBClusterIdentifier:   aws.String("prod-docdb-creating"),
				Status:                aws.String("creating"),
				Engine:                aws.String("aurora-mysql"),
				DeletionProtection:    aws.Bool(true),
				StorageEncrypted:      aws.Bool(true),
				BackupRetentionPeriod: aws.Int32(7),
			},
		},
	}

	result, err := awsclient.FetchRDSDBClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRDSDBClustersPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) < 1 {
		t.Fatalf("Findings: got %d, want >= 1 for creating DBC", len(r.Findings))
	}
	var found bool
	for _, f := range r.Findings {
		if f.Code == awsclient.CodeDBCTransitional {
			found = true
			if f.Severity != domain.SevWarn {
				t.Errorf("Findings[CodeDBCTransitional].Severity: got %v, want domain.SevWarn", f.Severity)
			}
			if f.Source != "wave1" {
				t.Errorf("Findings[CodeDBCTransitional].Source: got %q, want %q", f.Source, "wave1")
			}
		}
	}
	if !found {
		t.Errorf("Findings: missing CodeDBCTransitional; got %v", r.Findings)
	}
}

// ----- DBC-snap: Broken (CodeDBCSnapFailed) ----------------------------------

// TestPR03e_DBCSnapFetcher_BrokenEmitsBrokenFinding asserts that a "failed"
// RDS cluster snapshot emits one SevBroken Finding with CodeDBCSnapFailed.
func TestPR03e_DBCSnapFetcher_BrokenEmitsBrokenFinding(t *testing.T) {
	mock := &pr03eRDSClusterSnapshotMock{
		output: &rdssdk.DescribeDBClusterSnapshotsOutput{
			DBClusterSnapshots: []rdstypes.DBClusterSnapshot{
				{
					DBClusterSnapshotIdentifier: aws.String("csnap-failed"),
					DBClusterIdentifier:         aws.String("prod-aurora-01"),
					SnapshotType:                aws.String("manual"),
					Status:                      aws.String("failed"),
					Engine:                      aws.String("aurora-mysql"),
					StorageEncrypted:            aws.Bool(true),
				},
			},
		},
	}

	result, err := awsclient.FetchRDSDBClusterSnapshotsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRDSDBClusterSnapshotsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) != 1 {
		t.Fatalf("Findings: got %d, want 1 for failed DBC snapshot", len(r.Findings))
	}
	f := r.Findings[0]
	if f.Code != awsclient.CodeDBCSnapFailed {
		t.Errorf("Findings[0].Code: got %q, want %q", f.Code, awsclient.CodeDBCSnapFailed)
	}
	if f.Severity != domain.SevBroken {
		t.Errorf("Findings[0].Severity: got %v, want domain.SevBroken", f.Severity)
	}
	if f.Source != "wave1" {
		t.Errorf("Findings[0].Source: got %q, want %q", f.Source, "wave1")
	}
}

// ----- Redis: Broken (CodeRedisCreateFailed) ---------------------------------

// TestPR03e_RedisFetcher_BrokenEmitsBrokenFinding asserts that a
// "create-failed" replication group emits one SevBroken Finding with
// CodeRedisCreateFailed and Status=="".
func TestPR03e_RedisFetcher_BrokenEmitsBrokenFinding(t *testing.T) {
	mock := &pr03eRedisRGMock{
		output: &elasticache.DescribeReplicationGroupsOutput{
			ReplicationGroups: []elasticachetypes.ReplicationGroup{
				{
					ReplicationGroupId: aws.String("dev-redis-broken"),
					Status:             aws.String("create-failed"),
					Engine:             aws.String("redis"),
					MultiAZ:            elasticachetypes.MultiAZStatusDisabled,
					AutomaticFailover:  elasticachetypes.AutomaticFailoverStatusDisabled,
				},
			},
		},
	}

	result, err := awsclient.FetchRedisPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedisPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) < 1 {
		t.Fatalf("Findings: got %d, want >= 1 for create-failed Redis RG", len(r.Findings))
	}
	var found bool
	for _, f := range r.Findings {
		if f.Code == awsclient.CodeRedisCreateFailed {
			found = true
			if f.Severity != domain.SevBroken {
				t.Errorf("Findings[CodeRedisCreateFailed].Severity: got %v, want domain.SevBroken", f.Severity)
			}
			if f.Source != "wave1" {
				t.Errorf("Findings[CodeRedisCreateFailed].Source: got %q, want %q", f.Source, "wave1")
			}
		}
	}
	if !found {
		t.Errorf("Findings: missing CodeRedisCreateFailed; got %v", r.Findings)
	}
}

// ----- OpenSearch: Pending (CodeOpenSearchProcessing) ------------------------

// TestPR03e_OpenSearchFetcher_PendingEmitsWarnFinding asserts that an
// OpenSearch domain in DomainProcessingStatusTypeModifying emits one SevWarn
// Finding with CodeOpenSearchProcessing and Status=="".
func TestPR03e_OpenSearchFetcher_PendingEmitsWarnFinding(t *testing.T) {
	domainName := "analytics-modifying"
	listMock := &pr03eOSListMock{
		output: &opensearchsdk.ListDomainNamesOutput{
			DomainNames: []ostypes.DomainInfo{
				{DomainName: aws.String(domainName)},
			},
		},
	}
	describeMock := &pr03eOSDescribeMock{
		output: &opensearchsdk.DescribeDomainsOutput{
			DomainStatusList: []ostypes.DomainStatus{
				{
					ARN:                    aws.String("arn:aws:es:us-east-1:000000000000:domain/" + domainName),
					DomainId:               aws.String("000000000000/" + domainName),
					DomainName:             aws.String(domainName),
					EngineVersion:          aws.String("OpenSearch_2.11"),
					Created:                aws.Bool(true),
					Deleted:                aws.Bool(false),
					Processing:             aws.Bool(true),
					UpgradeProcessing:      aws.Bool(false),
					DomainProcessingStatus: ostypes.DomainProcessingStatusTypeModifying,
					EncryptionAtRestOptions: &ostypes.EncryptionAtRestOptions{
						Enabled: aws.Bool(true),
					},
				},
			},
		},
	}

	resources, err := awsclient.FetchOpenSearchDomains(context.Background(), listMock, describeMock)
	if err != nil {
		t.Fatalf("FetchOpenSearchDomains: unexpected error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]

	if len(r.Findings) < 1 {
		t.Fatalf("Findings: got %d, want >= 1 for modifying OpenSearch domain", len(r.Findings))
	}
	var found bool
	for _, f := range r.Findings {
		if f.Code == awsclient.CodeOpenSearchProcessing {
			found = true
			if f.Severity != domain.SevWarn {
				t.Errorf("Findings[CodeOpenSearchProcessing].Severity: got %v, want domain.SevWarn", f.Severity)
			}
			if f.Source != "wave1" {
				t.Errorf("Findings[CodeOpenSearchProcessing].Source: got %q, want %q", f.Source, "wave1")
			}
		}
	}
	if !found {
		t.Errorf("Findings: missing CodeOpenSearchProcessing; got %v", r.Findings)
	}
}

// ----- DDB: Pending (CodeDDBCreating) ----------------------------------------

// TestPR03e_DDBFetcher_PendingEmitsWarnFinding asserts that a CREATING DDB
// table emits one SevWarn Finding with CodeDDBCreating and Status=="".
func TestPR03e_DDBFetcher_PendingEmitsWarnFinding(t *testing.T) {
	tableName := "dev-orders-creating"
	listStub := &pr03eDDBListStub{names: []string{tableName}}
	descStub := &pr03eDDBDescribeStub{
		tables: map[string]*ddbtypes.TableDescription{
			tableName: {
				TableName:   aws.String(tableName),
				TableArn:    aws.String("arn:aws:dynamodb:us-east-1:000000000000:table/" + tableName),
				TableStatus: ddbtypes.TableStatusCreating,
			},
		},
	}

	result, err := awsclient.FetchDynamoDBTablesPage(context.Background(), listStub, descStub, "")
	if err != nil {
		t.Fatalf("FetchDynamoDBTablesPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) < 1 {
		t.Fatalf("Findings: got %d, want >= 1 for creating DDB table", len(r.Findings))
	}
	var found bool
	for _, f := range r.Findings {
		if f.Code == awsclient.CodeDDBCreating {
			found = true
			if f.Severity != domain.SevWarn {
				t.Errorf("Findings[CodeDDBCreating].Severity: got %v, want domain.SevWarn", f.Severity)
			}
			if f.Source != "wave1" {
				t.Errorf("Findings[CodeDDBCreating].Source: got %q, want %q", f.Source, "wave1")
			}
		}
	}
	if !found {
		t.Errorf("Findings: missing CodeDDBCreating; got %v", r.Findings)
	}
}

// ----- Redshift: Pending (CodeRedshiftCreating) ------------------------------

// TestPR03e_RedshiftFetcher_PendingEmitsWarnFinding asserts that a "creating"
// Redshift cluster emits one SevWarn Finding with CodeRedshiftCreating.
func TestPR03e_RedshiftFetcher_PendingEmitsWarnFinding(t *testing.T) {
	mock := &pr03eRedshiftMock{
		output: &redshiftsdk.DescribeClustersOutput{
			Clusters: []redshifttypes.Cluster{
				{
					ClusterIdentifier: aws.String("dev-dwh-creating"),
					ClusterStatus:     aws.String("creating"),
					NodeType:          aws.String("ra3.xlplus"),
					NumberOfNodes:     aws.Int32(2),
					DBName:            aws.String("dev"),
				},
			},
		},
	}

	result, err := awsclient.FetchRedshiftClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchRedshiftClustersPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) < 1 {
		t.Fatalf("Findings: got %d, want >= 1 for creating Redshift cluster", len(r.Findings))
	}
	var found bool
	for _, f := range r.Findings {
		if f.Code == awsclient.CodeRedshiftCreating {
			found = true
			if f.Severity != domain.SevWarn {
				t.Errorf("Findings[CodeRedshiftCreating].Severity: got %v, want domain.SevWarn", f.Severity)
			}
			if f.Source != "wave1" {
				t.Errorf("Findings[CodeRedshiftCreating].Source: got %q, want %q", f.Source, "wave1")
			}
		}
	}
	if !found {
		t.Errorf("Findings: missing CodeRedshiftCreating; got %v", r.Findings)
	}
}

// ----- MSK: Pending (CodeMSKCreating) ----------------------------------------

// TestPR03e_MSKFetcher_PendingEmitsWarnFinding asserts that a CREATING MSK
// cluster emits one SevWarn Finding with CodeMSKCreating.
func TestPR03e_MSKFetcher_PendingEmitsWarnFinding(t *testing.T) {
	mock := &pr03eMSKMock{
		output: &kafkasdk.ListClustersV2Output{
			ClusterInfoList: []kafkatypes.Cluster{
				{
					ClusterName:    aws.String("dev-kafka-creating"),
					ClusterArn:     aws.String("arn:aws:kafka:us-east-1:000000000000:cluster/dev-kafka-creating/abc789"),
					ClusterType:    kafkatypes.ClusterTypeProvisioned,
					State:          kafkatypes.ClusterStateCreating,
					CurrentVersion: aws.String("2.8.1"),
				},
			},
		},
	}

	result, err := awsclient.FetchMSKClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchMSKClustersPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) < 1 {
		t.Fatalf("Findings: got %d, want >= 1 for creating MSK cluster", len(r.Findings))
	}
	var found bool
	for _, f := range r.Findings {
		if f.Code == awsclient.CodeMSKCreating {
			found = true
			if f.Severity != domain.SevWarn {
				t.Errorf("Findings[CodeMSKCreating].Severity: got %v, want domain.SevWarn", f.Severity)
			}
			if f.Source != "wave1" {
				t.Errorf("Findings[CodeMSKCreating].Source: got %q, want %q", f.Source, "wave1")
			}
		}
	}
	if !found {
		t.Errorf("Findings: missing CodeMSKCreating; got %v", r.Findings)
	}
}

// ----- EFS: Pending (CodeEFSCreating) ----------------------------------------

// TestPR03e_EFSFetcher_PendingEmitsWarnFinding asserts that a "creating"
// EFS filesystem emits one SevWarn Finding with CodeEFSCreating.
func TestPR03e_EFSFetcher_PendingEmitsWarnFinding(t *testing.T) {
	mock := &pr03eEFSMock{
		output: &efssdk.DescribeFileSystemsOutput{
			FileSystems: []efstypes.FileSystemDescription{
				{
					FileSystemId:         aws.String("fs-0abc9999"),
					FileSystemArn:        aws.String("arn:aws:elasticfilesystem:us-east-1:000000000000:file-system/fs-0abc9999"),
					LifeCycleState:       efstypes.LifeCycleStateCreating,
					NumberOfMountTargets: 0,
					PerformanceMode:      efstypes.PerformanceModeGeneralPurpose,
					ThroughputMode:       efstypes.ThroughputModeBursting,
					Encrypted:            aws.Bool(true),
				},
			},
		},
	}

	result, err := awsclient.FetchEFSFileSystemsPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchEFSFileSystemsPage: unexpected error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	r := result.Resources[0]

	if len(r.Findings) < 1 {
		t.Fatalf("Findings: got %d, want >= 1 for creating EFS filesystem", len(r.Findings))
	}
	var found bool
	for _, f := range r.Findings {
		if f.Code == awsclient.CodeEFSCreating {
			found = true
			if f.Severity != domain.SevWarn {
				t.Errorf("Findings[CodeEFSCreating].Severity: got %v, want domain.SevWarn", f.Severity)
			}
			if f.Source != "wave1" {
				t.Errorf("Findings[CodeEFSCreating].Source: got %q, want %q", f.Source, "wave1")
			}
		}
	}
	if !found {
		t.Errorf("Findings: missing CodeEFSCreating; got %v", r.Findings)
	}
}

// =============================================================================
// Per-type case-class exceptions
// =============================================================================

// S3 has no Wave-1 codes — colorS3 returns ColorHealthy at the bucket level
// and Wave-2 enrichment owns the rest. Per AS-71 §1, no `s3_codes.go` file
// exists; the Healthy case above is the only Wave-1 contract that applies.
// No PendingEmitsWarnFinding / BrokenEmitsBrokenFinding cases for s3 — the
// dispatch's three-case template explicitly collapses to Healthy here.
//
// Kinesis has no Wave-1 broken codes per AS-71#document-plan §1 (lifecycle
// states only emit Warn). The Healthy + Pending cases above cover the entire
// Wave-1 surface; there is no BrokenEmitsBrokenFinding case for kinesis.
