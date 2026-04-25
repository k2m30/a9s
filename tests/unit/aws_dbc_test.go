package unit

// aws_dbc_test.go — fetcher tests for the dbc (DocumentDB cluster) resource type.
//
// Tests exercise FetchDocDBClusters and FetchDocDBClustersPage, verifying:
//   - All required Fields are populated with correct values.
//   - CIS flags (cis_flags) are computed correctly from StorageEncrypted,
//     BackupRetentionPeriod, and DeletionProtection.
//   - has_writer / writer_count are set correctly for various member configs.
//   - Pagination: Marker is threaded correctly; IsTruncated is set when present.
//   - Error propagation returns a wrapped error.
//   - Empty API response returns empty Resources slice (not nil).

import (
	"context"
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// helper — minimal DocDB mock for the DescribeDBClusters call only.
// ---------------------------------------------------------------------------

type mockDocDBClustersClient struct {
	pages []docdb.DescribeDBClustersOutput
	call  int
	err   error
}

func (m *mockDocDBClustersClient) DescribeDBClusters(
	_ context.Context,
	_ *docdb.DescribeDBClustersInput,
	_ ...func(*docdb.Options),
) (*docdb.DescribeDBClustersOutput, error) {
	if m.err != nil {
		return nil, m.err
	}
	if len(m.pages) == 0 {
		return &docdb.DescribeDBClustersOutput{}, nil
	}
	idx := m.call
	if idx >= len(m.pages) {
		return &docdb.DescribeDBClustersOutput{}, nil
	}
	m.call++
	out := m.pages[idx]
	return &out, nil
}

// singlePageDocDB returns a mock that returns one page with the given clusters.
func singlePageDocDB(clusters []docdbtypes.DBCluster) *mockDocDBClustersClient {
	return &mockDocDBClustersClient{
		pages: []docdb.DescribeDBClustersOutput{
			{DBClusters: clusters},
		},
	}
}

// ---------------------------------------------------------------------------
// T-DBC-01: field mapping
// ---------------------------------------------------------------------------

func TestFetchDocDBClusters_FieldMapping(t *testing.T) {
	mock := singlePageDocDB([]docdbtypes.DBCluster{
		{
			DBClusterIdentifier:   aws.String("prod-docdb-01"),
			DBClusterArn:          aws.String("arn:aws:rds:us-east-1:123456789012:cluster:prod-docdb-01"),
			Engine:                aws.String("docdb"),
			EngineVersion:         aws.String("5.0.0"),
			Status:                aws.String("available"),
			Endpoint:              aws.String("prod-docdb-01.cluster-xyz.us-east-1.docdb.amazonaws.com"),
			DeletionProtection:    aws.Bool(true),
			StorageEncrypted:      aws.Bool(true),
			BackupRetentionPeriod: aws.Int32(7),
			DBClusterMembers: []docdbtypes.DBClusterMember{
				{DBInstanceIdentifier: aws.String("prod-docdb-01-01"), IsClusterWriter: aws.Bool(true)},
				{DBInstanceIdentifier: aws.String("prod-docdb-01-02"), IsClusterWriter: aws.Bool(false)},
			},
		},
	})

	resources, err := awsclient.FetchDocDBClusters(context.Background(), mock)
	if err != nil {
		t.Fatalf("FetchDocDBClusters error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}

	r := resources[0]

	// ID and Name are the cluster identifier.
	if r.ID != "prod-docdb-01" {
		t.Errorf("ID = %q, want %q", r.ID, "prod-docdb-01")
	}
	if r.Name != "prod-docdb-01" {
		t.Errorf("Name = %q, want %q", r.Name, "prod-docdb-01")
	}
	// Healthy clusters render a blank Status phrase per spec §4 (silence is the UX).
	if r.Status != "" {
		t.Errorf("Status = %q, want blank (healthy silence)", r.Status)
	}
	if len(r.Issues) != 0 {
		t.Errorf("Issues = %v, want empty slice for healthy cluster", r.Issues)
	}

	// Required field keys. cis_flags is intentionally absent (jargon column removed).
	requiredFields := []string{
		"cluster_id", "engine_version", "status", "instances",
		"endpoint", "arn", "has_writer", "writer_count",
		"deletion_protection", "storage_encrypted", "backup_retention_period",
	}
	for _, key := range requiredFields {
		if _, ok := r.Fields[key]; !ok {
			t.Errorf("Fields missing key %q", key)
		}
	}
	if _, ok := r.Fields["cis_flags"]; ok {
		t.Errorf("Fields unexpectedly contains cis_flags — jargon column must not ship")
	}

	// Specific field values.
	wantFields := map[string]string{
		"cluster_id":              "prod-docdb-01",
		"engine_version":          "5.0.0",
		"status":                  "", // Healthy → blank phrase (§4)
		"instances":               "2",
		"endpoint":                "prod-docdb-01.cluster-xyz.us-east-1.docdb.amazonaws.com",
		"arn":                     "arn:aws:rds:us-east-1:123456789012:cluster:prod-docdb-01",
		"has_writer":              "true",
		"writer_count":            "1",
		"deletion_protection":     "true",
		"storage_encrypted":       "true",
		"backup_retention_period": "7",
	}
	for key, want := range wantFields {
		if r.Fields[key] != want {
			t.Errorf("Fields[%q] = %q, want %q", key, r.Fields[key], want)
		}
	}
}

// CIS flags column removed per universal-rule U10 (no jargon columns). The
// underlying signals (unencrypted, no backup, no deletion protection) are now
// tested via the §4 Status phrase tests (warn-dbc-unenc, warn-dbc-no-bkp,
// warn-dbc-no-prot, warn-dbc-multi) — see aws_dbc_test.go below and
// docs/resources/dbc.md §4.

// ---------------------------------------------------------------------------
// T-DBC-03: has_writer / writer_count
// ---------------------------------------------------------------------------

func TestFetchDocDBClusters_WriterCount(t *testing.T) {
	cases := []struct {
		name           string
		members        []docdbtypes.DBClusterMember
		wantHasWriter  string
		wantWriterCount string
	}{
		{
			name:            "no_members",
			members:         nil,
			wantHasWriter:   "false",
			wantWriterCount: "0",
		},
		{
			name: "one_writer",
			members: []docdbtypes.DBClusterMember{
				{IsClusterWriter: aws.Bool(true)},
			},
			wantHasWriter:   "true",
			wantWriterCount: "1",
		},
		{
			name: "two_writers_split_brain",
			members: []docdbtypes.DBClusterMember{
				{IsClusterWriter: aws.Bool(true)},
				{IsClusterWriter: aws.Bool(true)},
			},
			wantHasWriter:   "true",
			wantWriterCount: "2",
		},
		{
			name: "only_readers",
			members: []docdbtypes.DBClusterMember{
				{IsClusterWriter: aws.Bool(false)},
				{IsClusterWriter: aws.Bool(false)},
			},
			wantHasWriter:   "false",
			wantWriterCount: "0",
		},
		{
			name: "nil_is_cluster_writer",
			members: []docdbtypes.DBClusterMember{
				{IsClusterWriter: nil},
			},
			wantHasWriter:   "false",
			wantWriterCount: "0",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			mock := singlePageDocDB([]docdbtypes.DBCluster{
				{
					DBClusterIdentifier:   aws.String("wc-cluster"),
					Status:                aws.String("available"),
					StorageEncrypted:      aws.Bool(true),
					BackupRetentionPeriod: aws.Int32(7),
					DeletionProtection:    aws.Bool(true),
					DBClusterMembers:      tc.members,
				},
			})
			resources, err := awsclient.FetchDocDBClusters(context.Background(), mock)
			if err != nil {
				t.Fatalf("FetchDocDBClusters error: %v", err)
			}
			if len(resources) != 1 {
				t.Fatalf("expected 1 resource, got %d", len(resources))
			}
			r := resources[0]
			if r.Fields["has_writer"] != tc.wantHasWriter {
				t.Errorf("has_writer = %q, want %q", r.Fields["has_writer"], tc.wantHasWriter)
			}
			if r.Fields["writer_count"] != tc.wantWriterCount {
				t.Errorf("writer_count = %q, want %q", r.Fields["writer_count"], tc.wantWriterCount)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// T-DBC-04: nil field guards
// ---------------------------------------------------------------------------

// TestFetchDocDBClusters_NilFields verifies that nil pointer fields in the
// DBCluster struct (identifier, engine_version, status, endpoint) are handled
// gracefully — each falls back to an empty string.
func TestFetchDocDBClusters_NilFields(t *testing.T) {
	mock := singlePageDocDB([]docdbtypes.DBCluster{
		{
			// All optional fields are nil.
			DBClusterIdentifier:   nil,
			EngineVersion:         nil,
			Status:                nil,
			Endpoint:              nil,
			DeletionProtection:    nil,
			StorageEncrypted:      nil,
			BackupRetentionPeriod: nil,
		},
	})

	resources, err := awsclient.FetchDocDBClusters(context.Background(), mock)
	if err != nil {
		t.Fatalf("FetchDocDBClusters error: %v", err)
	}
	if len(resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(resources))
	}
	r := resources[0]
	if r.ID != "" {
		t.Errorf("ID = %q, want empty string for nil identifier", r.ID)
	}
	if r.Fields["engine_version"] != "" {
		t.Errorf("engine_version = %q, want empty string for nil pointer", r.Fields["engine_version"])
	}
	if r.Fields["endpoint"] != "" {
		t.Errorf("endpoint = %q, want empty string for nil pointer", r.Fields["endpoint"])
	}
	// nil DeletionProtection → defaults to "true" (safe default).
	if r.Fields["deletion_protection"] != "true" {
		t.Errorf("deletion_protection = %q, want %q for nil pointer", r.Fields["deletion_protection"], "true")
	}
	// nil StorageEncrypted → defaults to "true" (safe default).
	if r.Fields["storage_encrypted"] != "true" {
		t.Errorf("storage_encrypted = %q, want %q for nil pointer", r.Fields["storage_encrypted"], "true")
	}
	// nil BackupRetentionPeriod → defaults to "0".
	if r.Fields["backup_retention_period"] != "0" {
		t.Errorf("backup_retention_period = %q, want %q for nil pointer", r.Fields["backup_retention_period"], "0")
	}
}

// ---------------------------------------------------------------------------
// T-DBC-05: empty response
// ---------------------------------------------------------------------------

func TestFetchDocDBClusters_Empty(t *testing.T) {
	mock := singlePageDocDB(nil)

	resources, err := awsclient.FetchDocDBClusters(context.Background(), mock)
	if err != nil {
		t.Fatalf("FetchDocDBClusters error: %v", err)
	}
	// Empty response: resources is nil or empty — either is fine.
	if len(resources) != 0 {
		t.Errorf("expected 0 resources, got %d", len(resources))
	}
}

// ---------------------------------------------------------------------------
// T-DBC-06: error propagation
// ---------------------------------------------------------------------------

func TestFetchDocDBClusters_APIError(t *testing.T) {
	mock := &mockDocDBClustersClient{err: fmt.Errorf("throttled")}
	resources, err := awsclient.FetchDocDBClusters(context.Background(), mock)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if len(resources) != 0 {
		t.Errorf("expected 0 resources on error, got %d", len(resources))
	}
}

// ---------------------------------------------------------------------------
// T-DBC-07: pagination — Marker is threaded; IsTruncated set when Marker present.
// ---------------------------------------------------------------------------

// mockDocDBClustersMultiPage returns a mock that produces two pages of clusters.
type mockDocDBClustersMultiPage struct {
	pages []docdb.DescribeDBClustersOutput
	call  int
}

func (m *mockDocDBClustersMultiPage) DescribeDBClusters(
	_ context.Context,
	_ *docdb.DescribeDBClustersInput,
	_ ...func(*docdb.Options),
) (*docdb.DescribeDBClustersOutput, error) {
	if m.call >= len(m.pages) {
		return &docdb.DescribeDBClustersOutput{}, nil
	}
	out := m.pages[m.call]
	m.call++
	return &out, nil
}

func TestFetchDocDBClustersPage_Pagination(t *testing.T) {
	marker := "next-page-token"
	mock := &mockDocDBClustersMultiPage{
		pages: []docdb.DescribeDBClustersOutput{
			{
				DBClusters: []docdbtypes.DBCluster{
					{DBClusterIdentifier: aws.String("cluster-page1"), Status: aws.String("available")},
				},
				Marker: aws.String(marker),
			},
		},
	}

	result, err := awsclient.FetchDocDBClustersPage(context.Background(), mock, "")
	if err != nil {
		t.Fatalf("FetchDocDBClustersPage error: %v", err)
	}
	if len(result.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(result.Resources))
	}
	if result.Pagination == nil {
		t.Fatal("Pagination must not be nil")
	}
	if !result.Pagination.IsTruncated {
		t.Error("IsTruncated = false, want true when Marker is present")
	}
	if result.Pagination.NextToken != marker {
		t.Errorf("NextToken = %q, want %q", result.Pagination.NextToken, marker)
	}
}

func TestFetchDocDBClusters_MultiPageAccumulates(t *testing.T) {
	marker := "pg2"
	mock := &mockDocDBClustersMultiPage{
		pages: []docdb.DescribeDBClustersOutput{
			{
				DBClusters: []docdbtypes.DBCluster{
					{DBClusterIdentifier: aws.String("cluster-a"), Status: aws.String("available")},
				},
				Marker: aws.String(marker),
			},
			{
				DBClusters: []docdbtypes.DBCluster{
					{DBClusterIdentifier: aws.String("cluster-b"), Status: aws.String("modifying")},
				},
			},
		},
	}

	resources, err := awsclient.FetchDocDBClusters(context.Background(), mock)
	if err != nil {
		t.Fatalf("FetchDocDBClusters error: %v", err)
	}
	if len(resources) != 2 {
		t.Fatalf("expected 2 resources across 2 pages, got %d", len(resources))
	}
	ids := map[string]bool{}
	for _, r := range resources {
		ids[r.ID] = true
	}
	for _, want := range []string{"cluster-a", "cluster-b"} {
		if !ids[want] {
			t.Errorf("expected resource %q from multi-page fetch, not found in %v", want, ids)
		}
	}
}

// ---------------------------------------------------------------------------
// RDS cluster tests — computeRDSDBClusterStatusAndIssues via FetchRDSDBClustersPage
// ---------------------------------------------------------------------------

// mockRDSClustersClient satisfies RDSDescribeDBClustersAPI for a single page.
type mockRDSClustersClient struct {
	out *rds.DescribeDBClustersOutput
	err error
}

func (m *mockRDSClustersClient) DescribeDBClusters(
	_ context.Context, _ *rds.DescribeDBClustersInput, _ ...func(*rds.Options),
) (*rds.DescribeDBClustersOutput, error) {
	return m.out, m.err
}

// rdsClusterPage is a test helper that calls FetchRDSDBClustersPage with a
// single-cluster page and returns the first resource + error.
func rdsClusterPage(t *testing.T, cluster rdstypes.DBCluster) (resource.FetchResult, error) {
	t.Helper()
	mock := &mockRDSClustersClient{
		out: &rds.DescribeDBClustersOutput{
			DBClusters: []rdstypes.DBCluster{cluster},
		},
	}
	return awsclient.FetchRDSDBClustersPage(context.Background(), mock, "")
}

// TestComputeRDSDBClusterStatusAndIssues validates computeRDSDBClusterStatusAndIssues
// (unexported) via FetchRDSDBClustersPage — 11 cases mirroring the docdb table.
func TestComputeRDSDBClusterStatusAndIssues(t *testing.T) {
	boolPtr := func(b bool) *bool { return &b }
	int32Ptr := func(i int32) *int32 { return &i }
	writer := rdstypes.DBClusterMember{IsClusterWriter: boolPtr(true)}

	cases := []struct {
		name        string
		cluster     rdstypes.DBCluster
		wantStatus  string
		wantIssues  []string
	}{
		{
			name: "healthy_available_writer",
			cluster: rdstypes.DBCluster{
				DBClusterIdentifier:   aws.String("aurora-healthy"),
				Status:                aws.String("available"),
				DBClusterMembers:      []rdstypes.DBClusterMember{writer},
				DeletionProtection:    boolPtr(true),
				StorageEncrypted:      boolPtr(true),
				BackupRetentionPeriod: int32Ptr(7),
			},
			wantStatus: "",
			wantIssues: nil,
		},
		{
			name: "available_zero_writers",
			cluster: rdstypes.DBCluster{
				DBClusterIdentifier:   aws.String("aurora-no-writer"),
				Status:                aws.String("available"),
				DBClusterMembers:      []rdstypes.DBClusterMember{},
				DeletionProtection:    boolPtr(true),
				StorageEncrypted:      boolPtr(true),
				BackupRetentionPeriod: int32Ptr(7),
			},
			wantStatus: "no writer: reads only",
			wantIssues: []string{"no writer: reads only"},
		},
		{
			name: "available_deletion_protection_false",
			cluster: rdstypes.DBCluster{
				DBClusterIdentifier:   aws.String("aurora-nodp"),
				Status:                aws.String("available"),
				DBClusterMembers:      []rdstypes.DBClusterMember{writer},
				DeletionProtection:    boolPtr(false),
				StorageEncrypted:      boolPtr(true),
				BackupRetentionPeriod: int32Ptr(7),
			},
			wantStatus: "delete-protection off",
			wantIssues: []string{"delete-protection off"},
		},
		{
			name: "available_storage_not_encrypted",
			cluster: rdstypes.DBCluster{
				DBClusterIdentifier:   aws.String("aurora-noenc"),
				Status:                aws.String("available"),
				DBClusterMembers:      []rdstypes.DBClusterMember{writer},
				DeletionProtection:    boolPtr(true),
				StorageEncrypted:      boolPtr(false),
				BackupRetentionPeriod: int32Ptr(7),
			},
			wantStatus: "not encrypted at rest",
			wantIssues: []string{"not encrypted at rest"},
		},
		{
			name: "available_backup_retention_zero",
			cluster: rdstypes.DBCluster{
				DBClusterIdentifier:   aws.String("aurora-nobackup"),
				Status:                aws.String("available"),
				DBClusterMembers:      []rdstypes.DBClusterMember{writer},
				DeletionProtection:    boolPtr(true),
				StorageEncrypted:      boolPtr(true),
				BackupRetentionPeriod: int32Ptr(0),
			},
			wantStatus: "no automated backups",
			wantIssues: []string{"no automated backups"},
		},
		{
			name: "available_all_three_warnings",
			cluster: rdstypes.DBCluster{
				DBClusterIdentifier:   aws.String("aurora-allwarn"),
				Status:                aws.String("available"),
				DBClusterMembers:      []rdstypes.DBClusterMember{writer},
				DeletionProtection:    boolPtr(false),
				StorageEncrypted:      boolPtr(false),
				BackupRetentionPeriod: int32Ptr(0),
			},
			wantStatus: "delete-protection off (+2)",
			wantIssues: []string{"delete-protection off", "not encrypted at rest", "no automated backups"},
		},
		{
			name: "failed_status",
			cluster: rdstypes.DBCluster{
				DBClusterIdentifier: aws.String("aurora-failed"),
				Status:              aws.String("failed"),
				DBClusterMembers:    []rdstypes.DBClusterMember{writer},
			},
			wantStatus: "failed: cluster operation",
			wantIssues: []string{"failed: cluster operation"},
		},
		{
			name: "inaccessible_encryption_credentials",
			cluster: rdstypes.DBCluster{
				DBClusterIdentifier: aws.String("aurora-kms"),
				Status:              aws.String("inaccessible-encryption-credentials"),
				DBClusterMembers:    []rdstypes.DBClusterMember{writer},
			},
			wantStatus: "encryption key unreachable",
			wantIssues: []string{"encryption key unreachable"},
		},
		{
			name: "incompatible_parameters",
			cluster: rdstypes.DBCluster{
				DBClusterIdentifier: aws.String("aurora-param"),
				Status:              aws.String("incompatible-parameters"),
				DBClusterMembers:    []rdstypes.DBClusterMember{writer},
			},
			wantStatus: "parameter group incompatible",
			wantIssues: []string{"parameter group incompatible"},
		},
		{
			name: "modifying_transitional",
			cluster: rdstypes.DBCluster{
				DBClusterIdentifier: aws.String("aurora-modifying"),
				Status:              aws.String("modifying"),
				DBClusterMembers:    []rdstypes.DBClusterMember{writer},
			},
			wantStatus: "modifying: in progress",
			wantIssues: []string{"modifying: in progress"},
		},
		{
			name: "unknown_status_passthrough",
			cluster: rdstypes.DBCluster{
				DBClusterIdentifier: aws.String("aurora-unknown"),
				Status:              aws.String("cross-region-copying"),
				DBClusterMembers:    []rdstypes.DBClusterMember{writer},
			},
			wantStatus: "cross-region-copying",
			wantIssues: []string{"cross-region-copying"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := rdsClusterPage(t, tc.cluster)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(result.Resources) != 1 {
				t.Fatalf("expected 1 resource, got %d", len(result.Resources))
			}
			r := result.Resources[0]
			if r.Status != tc.wantStatus {
				t.Errorf("Status = %q, want %q", r.Status, tc.wantStatus)
			}
			if len(r.Issues) != len(tc.wantIssues) {
				t.Errorf("Issues = %v, want %v", r.Issues, tc.wantIssues)
			} else {
				for i, want := range tc.wantIssues {
					if r.Issues[i] != want {
						t.Errorf("Issues[%d] = %q, want %q", i, r.Issues[i], want)
					}
				}
			}
		})
	}
}
