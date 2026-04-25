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
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
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
