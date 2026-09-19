package unit

// computeDBCSnapFindings and computeRDSDBClusterSnapshotFindings are
// unexported; these tests reach them through the per-SDK page fetchers.
//
// DBClusterSnapshot carries no progress or failure-reason field, so a creating
// snapshot's Issues holds just "creating". Old automated snapshots belong to the
// cross-ref enricher's past-retention rule, not to the fetcher.

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

type dbcSnapDocDBSinglePageMock struct {
	output *docdb.DescribeDBClusterSnapshotsOutput
}

func (m *dbcSnapDocDBSinglePageMock) DescribeDBClusterSnapshots(
	_ context.Context,
	_ *docdb.DescribeDBClusterSnapshotsInput,
	_ ...func(*docdb.Options),
) (*docdb.DescribeDBClusterSnapshotsOutput, error) {
	return m.output, nil
}

type dbcSnapRDSSinglePageMock struct {
	output *rds.DescribeDBClusterSnapshotsOutput
}

func (m *dbcSnapRDSSinglePageMock) DescribeDBClusterSnapshots(
	_ context.Context,
	_ *rds.DescribeDBClusterSnapshotsInput,
	_ ...func(*rds.Options),
) (*rds.DescribeDBClusterSnapshotsOutput, error) {
	return m.output, nil
}

func findingPhrases(findings []domain.Finding) []string {
	phrases := make([]string, len(findings))
	for i, f := range findings {
		phrases[i] = f.Phrase
	}
	return phrases
}

func TestComputeDBCSnapStatusAndIssues(t *testing.T) {
	now := time.Now().UTC()
	age400d := now.Add(-400 * 24 * time.Hour)
	age10d := now.Add(-10 * 24 * time.Hour)

	cases := []struct {
		name       string
		snap       docdbtypes.DBClusterSnapshot
		wantStatus string
		wantIssues []string
	}{
		{
			name: "healthy_available",
			snap: docdbtypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("snap-healthy"),
				Status:                      aws.String("available"),
				SnapshotType:                aws.String("automated"),
				SnapshotCreateTime:          &age10d,
			},
			wantStatus: "",
			wantIssues: nil,
		},
		{
			name: "creating_status",
			snap: docdbtypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("snap-creating"),
				Status:                      aws.String("creating"),
				SnapshotType:                aws.String("manual"),
				SnapshotCreateTime:          &age10d,
			},
			wantStatus: "creating",
			wantIssues: []string{"creating"},
		},
		{
			name: "failed_status",
			snap: docdbtypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("snap-failed"),
				Status:                      aws.String("failed"),
				SnapshotType:                aws.String("manual"),
				SnapshotCreateTime:          &age10d,
			},
			wantStatus: "failed",
			wantIssues: []string{"failed"},
		},
		{
			name: "incompatible_restore_status",
			snap: docdbtypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("snap-incompatible"),
				Status:                      aws.String("incompatible-restore"),
				SnapshotType:                aws.String("manual"),
				SnapshotCreateTime:          &age10d,
			},
			wantStatus: "incompatible-restore",
			wantIssues: []string{"incompatible-restore"},
		},
		{
			name: "manual_old_available",
			snap: docdbtypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("snap-manual-old"),
				Status:                      aws.String("available"),
				SnapshotType:                aws.String("manual"),
				SnapshotCreateTime:          &age400d,
			},
			wantStatus: "manual, unused 400d",
			wantIssues: []string{"manual, unused 400d"},
		},
		{
			name: "manual_young_available",
			snap: docdbtypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("snap-manual-young"),
				Status:                      aws.String("available"),
				SnapshotType:                aws.String("manual"),
				SnapshotCreateTime:          &age10d,
			},
			wantStatus: "",
			wantIssues: nil,
		},
		{
			name: "automated_old_available_healthy",
			snap: docdbtypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("snap-automated-old"),
				Status:                      aws.String("available"),
				SnapshotType:                aws.String("automated"),
				SnapshotCreateTime:          &age400d,
			},
			wantStatus: "",
			wantIssues: nil,
		},
		{
			name: "failed_with_manual_age_suppressed",
			snap: docdbtypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("snap-failed-manual-old"),
				Status:                      aws.String("failed"),
				SnapshotType:                aws.String("manual"),
				SnapshotCreateTime:          &age400d,
			},
			wantStatus: "failed",
			wantIssues: []string{"failed"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := &dbcSnapDocDBSinglePageMock{
				output: &docdb.DescribeDBClusterSnapshotsOutput{DBClusterSnapshots: []docdbtypes.DBClusterSnapshot{tc.snap}},
			}
			result, err := awsclient.FetchDocDBClusterSnapshotsPage(context.Background(), mock, "")
			if err != nil {
				t.Fatalf("FetchDocDBClusterSnapshotsPage: unexpected error: %v", err)
			}
			if len(result.Resources) != 1 {
				t.Fatalf("expected 1 resource, got %d", len(result.Resources))
			}
			r := result.Resources[0]
			gotStatus := r.Fields["status"]
			gotIssues := findingPhrases(r.Findings)

			if gotStatus != tc.wantStatus {
				t.Errorf("Fields[status]:\n  got:  %q\n  want: %q", gotStatus, tc.wantStatus)
			}

			if len(gotIssues) != len(tc.wantIssues) {
				t.Errorf("Findings phrases:\n  got:  %v (len=%d)\n  want: %v (len=%d)",
					gotIssues, len(gotIssues), tc.wantIssues, len(tc.wantIssues))
				return
			}
			for i, want := range tc.wantIssues {
				if gotIssues[i] != want {
					t.Errorf("Findings[%d].Phrase:\n  got:  %q\n  want: %q", i, gotIssues[i], want)
				}
			}
		})
	}
}

func TestComputeRDSDBClusterSnapshotStatusAndIssues(t *testing.T) {
	now := time.Now().UTC()
	age400d := now.Add(-400 * 24 * time.Hour)
	age10d := now.Add(-10 * 24 * time.Hour)

	cases := []struct {
		name       string
		snap       rdstypes.DBClusterSnapshot
		wantStatus string
		wantIssues []string
	}{
		{
			name: "healthy_available",
			snap: rdstypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("dbc-snap-healthy"),
				Status:                      aws.String("available"),
				SnapshotType:                aws.String("automated"),
				SnapshotCreateTime:          &age10d,
			},
			wantStatus: "",
			wantIssues: nil,
		},
		{
			name: "creating_status",
			snap: rdstypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("dbc-snap-creating"),
				Status:                      aws.String("creating"),
				SnapshotType:                aws.String("manual"),
				SnapshotCreateTime:          &age10d,
			},
			wantStatus: "creating",
			wantIssues: []string{"creating"},
		},
		{
			name: "failed_status",
			snap: rdstypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("dbc-snap-failed"),
				Status:                      aws.String("failed"),
				SnapshotType:                aws.String("manual"),
				SnapshotCreateTime:          &age10d,
			},
			wantStatus: "failed",
			wantIssues: []string{"failed"},
		},
		{
			name: "incompatible_restore_status",
			snap: rdstypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("dbc-snap-incompat"),
				Status:                      aws.String("incompatible-restore"),
				SnapshotType:                aws.String("manual"),
				SnapshotCreateTime:          &age10d,
			},
			wantStatus: "incompatible-restore",
			wantIssues: []string{"incompatible-restore"},
		},
		{
			name: "manual_old_available",
			snap: rdstypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("dbc-snap-manual-old"),
				Status:                      aws.String("available"),
				SnapshotType:                aws.String("manual"),
				SnapshotCreateTime:          &age400d,
			},
			wantStatus: "manual, unused 400d",
			wantIssues: []string{"manual, unused 400d"},
		},
		{
			name: "manual_young_available",
			snap: rdstypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("dbc-snap-manual-young"),
				Status:                      aws.String("available"),
				SnapshotType:                aws.String("manual"),
				SnapshotCreateTime:          &age10d,
			},
			wantStatus: "",
			wantIssues: nil,
		},
		{
			name: "automated_old_available_healthy",
			snap: rdstypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("dbc-snap-auto-old"),
				Status:                      aws.String("available"),
				SnapshotType:                aws.String("automated"),
				SnapshotCreateTime:          &age400d,
			},
			wantStatus: "",
			wantIssues: nil,
		},
		{
			name: "failed_with_manual_age_suppressed",
			snap: rdstypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("dbc-snap-failed-manual-old"),
				Status:                      aws.String("failed"),
				SnapshotType:                aws.String("manual"),
				SnapshotCreateTime:          &age400d,
			},
			wantStatus: "failed",
			wantIssues: []string{"failed"},
		},
		{
			name: "nil_status",
			snap: rdstypes.DBClusterSnapshot{
				DBClusterSnapshotIdentifier: aws.String("dbc-snap-nil-status"),
				Status:                      nil,
				SnapshotType:                aws.String("automated"),
				SnapshotCreateTime:          &age10d,
			},
			wantStatus: "",
			wantIssues: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := &dbcSnapRDSSinglePageMock{
				output: &rds.DescribeDBClusterSnapshotsOutput{DBClusterSnapshots: []rdstypes.DBClusterSnapshot{tc.snap}},
			}
			result, err := awsclient.FetchRDSDBClusterSnapshotsPage(context.Background(), mock, "")
			if err != nil {
				t.Fatalf("FetchRDSDBClusterSnapshotsPage: unexpected error: %v", err)
			}
			if len(result.Resources) != 1 {
				t.Fatalf("expected 1 resource, got %d", len(result.Resources))
			}
			r := result.Resources[0]
			gotStatus := r.Fields["status"]
			gotIssues := findingPhrases(r.Findings)

			if gotStatus != tc.wantStatus {
				t.Errorf("Fields[status]:\n  got:  %q\n  want: %q", gotStatus, tc.wantStatus)
			}

			if len(gotIssues) != len(tc.wantIssues) {
				t.Errorf("Findings phrases:\n  got:  %v (len=%d)\n  want: %v (len=%d)",
					gotIssues, len(gotIssues), tc.wantIssues, len(tc.wantIssues))
				return
			}
			for i, want := range tc.wantIssues {
				if gotIssues[i] != want {
					t.Errorf("Findings[%d].Phrase:\n  got:  %q\n  want: %q",
						i, gotIssues[i], want)
				}
			}
		})
	}
}

type dbcSnapDocDBMock struct {
	fullDocDBMock
	dbClusterSnapshotsPages []docdb.DescribeDBClusterSnapshotsOutput
	dbClusterSnapshotsCall  int
}

func (m *dbcSnapDocDBMock) DescribeDBClusterSnapshots(
	_ context.Context,
	_ *docdb.DescribeDBClusterSnapshotsInput,
	_ ...func(*docdb.Options),
) (*docdb.DescribeDBClusterSnapshotsOutput, error) {
	if m.dbClusterSnapshotsCall >= len(m.dbClusterSnapshotsPages) {
		return &docdb.DescribeDBClusterSnapshotsOutput{}, nil
	}
	out := m.dbClusterSnapshotsPages[m.dbClusterSnapshotsCall]
	m.dbClusterSnapshotsCall++
	return &out, nil
}

type dbcSnapRDSMock struct {
	fullRDSMock
	dbClusterSnapshotsPages []rds.DescribeDBClusterSnapshotsOutput
	dbClusterSnapshotsCall  int
}

func (m *dbcSnapRDSMock) DescribeDBClusterSnapshots(
	_ context.Context,
	_ *rds.DescribeDBClusterSnapshotsInput,
	_ ...func(*rds.Options),
) (*rds.DescribeDBClusterSnapshotsOutput, error) {
	if m.dbClusterSnapshotsCall >= len(m.dbClusterSnapshotsPages) {
		return &rds.DescribeDBClusterSnapshotsOutput{}, nil
	}
	out := m.dbClusterSnapshotsPages[m.dbClusterSnapshotsCall]
	m.dbClusterSnapshotsCall++
	return &out, nil
}

// DocDB and RDS DescribeDBClusterSnapshots can both return the same snapshot
// identifier. The fetcher keeps the first occurrence and appends DocDB rows
// first, so the engine-correct DocDB RawStruct survives.
func TestDBCSnapFetcher_DedupesAcrossDualAPIByID(t *testing.T) {
	fetcher := resource.GetPaginatedFetcher("dbc-snap")
	if fetcher == nil {
		t.Fatal("no paginated fetcher registered for dbc-snap — init() not invoked")
	}

	now := time.Now().UTC()
	age10d := now.Add(-10 * 24 * time.Hour)

	t.Run("overlap_keeps_docdb_side", func(t *testing.T) {
		const sharedID = "shared-snap-01"
		docdbMock := &dbcSnapDocDBMock{
			dbClusterSnapshotsPages: []docdb.DescribeDBClusterSnapshotsOutput{
				{
					DBClusterSnapshots: []docdbtypes.DBClusterSnapshot{
						{
							DBClusterSnapshotIdentifier: aws.String(sharedID),
							DBClusterIdentifier:         aws.String("shared-cluster"),
							Engine:                      aws.String("docdb"),
							Status:                      aws.String("available"),
							SnapshotType:                aws.String("automated"),
							SnapshotCreateTime:          &age10d,
						},
					},
				},
			},
		}
		rdsMock := &dbcSnapRDSMock{
			dbClusterSnapshotsPages: []rds.DescribeDBClusterSnapshotsOutput{
				{
					DBClusterSnapshots: []rdstypes.DBClusterSnapshot{
						{
							DBClusterSnapshotIdentifier: aws.String(sharedID),
							DBClusterIdentifier:         aws.String("shared-cluster"),
							Engine:                      aws.String("aurora-postgresql"),
							Status:                      aws.String("available"),
							SnapshotType:                aws.String("automated"),
							SnapshotCreateTime:          &age10d,
						},
					},
				},
			},
		}
		clients := &awsclient.ServiceClients{DocDB: docdbMock, RDS: rdsMock}

		result, err := fetcher(context.Background(), clients, "")
		if err != nil {
			t.Fatalf("fetcher error: %v", err)
		}
		if len(result.Resources) != 1 {
			t.Fatalf("len(Resources) = %d, want 1 (overlap deduped)", len(result.Resources))
		}
		r := result.Resources[0]
		if r.ID != sharedID {
			t.Errorf("Resources[0].ID = %q, want %q", r.ID, sharedID)
		}
		if _, ok := r.RawStruct.(docdbtypes.DBClusterSnapshot); !ok {
			t.Errorf("Resources[0].RawStruct type = %T, want docdbtypes.DBClusterSnapshot (DocDB-side appended first must win)", r.RawStruct)
		}
	})

	t.Run("no_overlap_keeps_both", func(t *testing.T) {
		docdbMock := &dbcSnapDocDBMock{
			dbClusterSnapshotsPages: []docdb.DescribeDBClusterSnapshotsOutput{
				{
					DBClusterSnapshots: []docdbtypes.DBClusterSnapshot{
						{
							DBClusterSnapshotIdentifier: aws.String("docdb-only-snap-01"),
							DBClusterIdentifier:         aws.String("docdb-cluster-01"),
							Engine:                      aws.String("docdb"),
							Status:                      aws.String("available"),
							SnapshotType:                aws.String("automated"),
							SnapshotCreateTime:          &age10d,
						},
					},
				},
			},
		}
		rdsMock := &dbcSnapRDSMock{
			dbClusterSnapshotsPages: []rds.DescribeDBClusterSnapshotsOutput{
				{
					DBClusterSnapshots: []rdstypes.DBClusterSnapshot{
						{
							DBClusterSnapshotIdentifier: aws.String("rds-only-snap-01"),
							DBClusterIdentifier:         aws.String("aurora-cluster-01"),
							Engine:                      aws.String("aurora-mysql"),
							Status:                      aws.String("available"),
							SnapshotType:                aws.String("automated"),
							SnapshotCreateTime:          &age10d,
						},
					},
				},
			},
		}
		clients := &awsclient.ServiceClients{DocDB: docdbMock, RDS: rdsMock}

		result, err := fetcher(context.Background(), clients, "")
		if err != nil {
			t.Fatalf("fetcher error: %v", err)
		}
		if len(result.Resources) != 2 {
			t.Fatalf("len(Resources) = %d, want 2 (no overlap)", len(result.Resources))
		}
		ids := map[string]bool{}
		for _, r := range result.Resources {
			ids[r.ID] = true
		}
		for _, want := range []string{"docdb-only-snap-01", "rds-only-snap-01"} {
			if !ids[want] {
				t.Errorf("expected resource %q in result, got %v", want, ids)
			}
		}
	})

	t.Run("overlap_plus_rds_only_keeps_two", func(t *testing.T) {
		const sharedID = "shared-snap-02"
		docdbMock := &dbcSnapDocDBMock{
			dbClusterSnapshotsPages: []docdb.DescribeDBClusterSnapshotsOutput{
				{
					DBClusterSnapshots: []docdbtypes.DBClusterSnapshot{
						{
							DBClusterSnapshotIdentifier: aws.String(sharedID),
							DBClusterIdentifier:         aws.String("shared-cluster-02"),
							Engine:                      aws.String("docdb"),
							Status:                      aws.String("available"),
							SnapshotType:                aws.String("automated"),
							SnapshotCreateTime:          &age10d,
						},
					},
				},
			},
		}
		rdsMock := &dbcSnapRDSMock{
			dbClusterSnapshotsPages: []rds.DescribeDBClusterSnapshotsOutput{
				{
					DBClusterSnapshots: []rdstypes.DBClusterSnapshot{
						{
							DBClusterSnapshotIdentifier: aws.String(sharedID),
							DBClusterIdentifier:         aws.String("shared-cluster-02"),
							Engine:                      aws.String("aurora-postgresql"),
							Status:                      aws.String("available"),
							SnapshotType:                aws.String("automated"),
							SnapshotCreateTime:          &age10d,
						},
						{
							DBClusterSnapshotIdentifier: aws.String("rds-only-snap-02"),
							DBClusterIdentifier:         aws.String("aurora-cluster-02"),
							Engine:                      aws.String("aurora-mysql"),
							Status:                      aws.String("available"),
							SnapshotType:                aws.String("automated"),
							SnapshotCreateTime:          &age10d,
						},
					},
				},
			},
		}
		clients := &awsclient.ServiceClients{DocDB: docdbMock, RDS: rdsMock}

		result, err := fetcher(context.Background(), clients, "")
		if err != nil {
			t.Fatalf("fetcher error: %v", err)
		}
		if len(result.Resources) != 2 {
			t.Fatalf("len(Resources) = %d, want 2 (deduped overlap + unique rds snap)", len(result.Resources))
		}
		var sharedRow *resource.Resource
		ids := map[string]bool{}
		for i := range result.Resources {
			r := &result.Resources[i]
			ids[r.ID] = true
			if r.ID == sharedID {
				sharedRow = r
			}
		}
		for _, want := range []string{sharedID, "rds-only-snap-02"} {
			if !ids[want] {
				t.Errorf("expected resource %q in result, got %v", want, ids)
			}
		}
		if sharedRow == nil {
			t.Fatal("shared snapshot row missing from result")
		}
		if _, ok := sharedRow.RawStruct.(docdbtypes.DBClusterSnapshot); !ok {
			t.Errorf("shared row RawStruct type = %T, want docdbtypes.DBClusterSnapshot (DocDB-side appended first must win)", sharedRow.RawStruct)
		}
	})
}
