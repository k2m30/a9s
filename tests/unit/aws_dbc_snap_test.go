package unit

// aws_dbc_snap_test.go — Table-driven unit tests for ComputeDBCSnapStatusAndIssues.
//
// Spec: docs/resources/dbc-snap.md §3.1 + §4
//
// ComputeDBCSnapStatusAndIssues is the fetcher-local §4 phrase computer added
// by the coder's refactor of internal/aws/dbc_snap.go. It follows the same
// contract as ComputeDBISnapStatusAndIssues (dbi-snap) but with the dbc-snap
// signal set:
//
//   Broken: failed, incompatible-* (exit early, Issues=[keyword])
//   Warning: creating (Issues=["creating"])
//   Warning: manual age > 365d (Issues=["manual, unused <N>d"])
//   Healthy: ("", nil) — Status="available" AND not manual-old
//
// The manual-age rule applies ONLY to manual snapshots; automated-old is not
// a fetcher-local signal (it is the cross-ref enricher's past-retention rule).
//
// DBClusterSnapshot has no PercentProgress-style cause for the "creating" state
// (spec §4 note: "no per-snapshot failure-reason field on DBClusterSnapshot"),
// so the Issues slice carries just "creating", not "creating: <pct>%".
//
// Import path for the function under test:
//
//	awsclient "github.com/k2m30/a9s/v3/internal/aws"

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestComputeDBCSnapStatusAndIssues pins the §4 phrase output and Issues slice
// for each signal in docs/resources/dbc-snap.md §3.1.
func TestComputeDBCSnapStatusAndIssues(t *testing.T) {
	now := time.Now().UTC()
	age400d := now.Add(-400 * 24 * time.Hour)
	age10d := now.Add(-10 * 24 * time.Hour)

	cases := []struct {
		name        string
		snap        docdbtypes.DBClusterSnapshot
		wantStatus  string
		wantIssues  []string
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
			// DBClusterSnapshot has no PercentProgress-style field exposed for
			// "creating" on the §4 table; Issues carries just "creating".
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
			// manual-old: Status="available", SnapshotType="manual", age > 365d.
			// List text per spec §4 table: "manual, unused <N>d" where N is the
			// actual age in days. Snap is 400d old so phrase is "manual, unused 400d".
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
			// manual-young: Status="available", SnapshotType="manual", age=10d → healthy.
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
			// automated-old: manual-age rule applies ONLY to manual snapshots per §3.1.
			// An automated snapshot 400d old with available status is HEALTHY (the
			// cross-ref enricher handles automated past-retention, not this function).
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
			// Broken precedence wins; the manual-age Warning is suppressed when
			// Status=failed (parity with dbi-snap §0.1).
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
			gotStatus, gotIssues := awsclient.ComputeDBCSnapStatusAndIssues(tc.snap)

			if gotStatus != tc.wantStatus {
				t.Errorf("ComputeDBCSnapStatusAndIssues status:\n  got:  %q\n  want: %q", gotStatus, tc.wantStatus)
			}

			if len(gotIssues) != len(tc.wantIssues) {
				t.Errorf("ComputeDBCSnapStatusAndIssues issues length:\n  got:  %v (len=%d)\n  want: %v (len=%d)",
					gotIssues, len(gotIssues), tc.wantIssues, len(tc.wantIssues))
				return
			}
			for i, want := range tc.wantIssues {
				if gotIssues[i] != want {
					t.Errorf("ComputeDBCSnapStatusAndIssues issues[%d]:\n  got:  %q\n  want: %q", i, gotIssues[i], want)
				}
			}
		})
	}
}

// TestComputeRDSDBClusterSnapshotStatusAndIssues pins the §4 phrase output and
// Issues slice for ComputeRDSDBClusterSnapshotStatusAndIssues (rdstypes shape).
// Algorithm mirrors ComputeDBCSnapStatusAndIssues — same precedence ladder.
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
			// manual-old: available + manual + age > 365d → "manual, unused Nd".
			// 400d old → phrase is "manual, unused 400d".
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
			// manual-young: available + manual + age=10d → healthy (no issue).
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
			// automated-old: manual-age rule applies ONLY to manual snapshots.
			// An automated snapshot 400d old is healthy at the fetcher level.
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
			// Broken precedence: failed suppresses the manual-age Warning.
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
			// nil Status: should not panic; returns ("", nil).
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
			gotStatus, gotIssues := awsclient.ComputeRDSDBClusterSnapshotStatusAndIssues(tc.snap)

			if gotStatus != tc.wantStatus {
				t.Errorf("ComputeRDSDBClusterSnapshotStatusAndIssues status:\n  got:  %q\n  want: %q", gotStatus, tc.wantStatus)
			}

			if len(gotIssues) != len(tc.wantIssues) {
				t.Errorf("ComputeRDSDBClusterSnapshotStatusAndIssues issues length:\n  got:  %v (len=%d)\n  want: %v (len=%d)",
					gotIssues, len(gotIssues), tc.wantIssues, len(tc.wantIssues))
				return
			}
			for i, want := range tc.wantIssues {
				if gotIssues[i] != want {
					t.Errorf("ComputeRDSDBClusterSnapshotStatusAndIssues issues[%d]:\n  got:  %q\n  want: %q",
						i, gotIssues[i], want)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AS-145: dbc-snap dual-SDK dedup-by-ID — DocDB-side wins.
// ---------------------------------------------------------------------------

// dbcSnapDocDBMock embeds fullDocDBMock (defined in aws_dbc_test.go, same
// package) so it inherits all DocDBAPI methods, then overrides
// DescribeDBClusterSnapshots to return scripted snapshot pages. Used to drive
// the dbc-snap paginated fetcher through the dual-SDK overlap path.
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

// dbcSnapRDSMock embeds fullRDSMock and overrides DescribeDBClusterSnapshots
// for the RDS side of the dual-SDK dedup test.
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

// TestDBCSnapFetcher_DedupesAcrossDualAPIByID pins the AS-145 production fix
// for cluster snapshots: when DocDB and RDS DescribeDBClusterSnapshots both
// return the same DBClusterSnapshotIdentifier on the same fetch tick, the
// dbc-snap fetcher must dedup by Resource.ID with first-occurrence wins.
// DocDB-side rows are appended first, so the docdb-side row must be preserved
// (engine-correct RawStruct used by detail enrichment / related-panel pivots).
func TestDBCSnapFetcher_DedupesAcrossDualAPIByID(t *testing.T) {
	fetcher := resource.GetPaginatedFetcher("dbc-snap")
	if fetcher == nil {
		t.Fatal("no paginated fetcher registered for dbc-snap — init() not invoked")
	}

	now := time.Now().UTC()
	age10d := now.Add(-10 * 24 * time.Hour)

	// Sub-test 1: same snapshot identifier on both DocDB and RDS pages — 1 row,
	// docdb-side RawStruct preserved.
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

	// Sub-test 2: distinct snapshot identifiers on each side — both rows preserved.
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

	// Sub-test 3: shared snapshot id + RDS-only unique on the same tick —
	// deduped pair plus the unique RDS row both survive.
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
