package unit

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
)

// d1DbcSnapBaseline is a healthy Aurora cluster snapshot: available, encrypted,
// and recent enough that the manual-snapshot age predicate stays silent.
func d1DbcSnapBaseline() rdstypes.DBClusterSnapshot {
	return rdstypes.DBClusterSnapshot{
		DBClusterSnapshotIdentifier: aws.String("acme-aurora-prod-snap"),
		DBClusterSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:cluster-snapshot:acme-aurora-prod-snap"),
		DBClusterIdentifier:         aws.String("acme-aurora-prod"),
		Engine:                      aws.String("aurora-postgresql"),
		Status:                      aws.String("available"),
		StorageEncrypted:            aws.Bool(true),
		SnapshotType:                aws.String("manual"),
		SnapshotCreateTime:          aws.Time(time.Now().AddDate(0, 0, -3)),
	}
}

// TestDbcSnapColor pins the status → colour mapping for DB cluster snapshots.
//
// Three rows that the old table recorded as Healthy are Warning here, and the
// difference is the point of the conversion rather than a change of intent: an
// unencrypted snapshot and a manual snapshot nobody has restored in a year both
// produce a wave-1 finding. They read as green only when the colour is asked of
// a Fields map that carries no findings at all.
//
// The suffix rows ("failed (+1)", "creating: 47%") are gone: the suffix is
// added by StatusPhrase for display, and a severity comparison never sees it.
func TestDbcSnapColor(t *testing.T) {
	cases := []struct {
		name   string
		status string
		mutate func(*rdstypes.DBClusterSnapshot)
		want   resource.Color
	}{
		{name: "available", status: "available", want: resource.ColorHealthy},
		{name: "empty_status", status: "", want: resource.ColorHealthy},

		{name: "creating", status: "creating", want: resource.ColorWarning},
		// A snapshot being copied is not yet restorable. AWS returns "copying"
		// for both snapshot types, and neither findings predicate has a branch
		// for it, so the row reads as ready when it is not.
		{name: "copying", status: "copying", want: resource.ColorWarning},
		{name: "failed", status: "failed", want: resource.ColorBroken},
		{name: "incompatible_restore", status: "incompatible-restore", want: resource.ColorBroken},
		{name: "incompatible_parameters", status: "incompatible-parameters", want: resource.ColorBroken},

		{
			name:   "unencrypted",
			status: "available",
			mutate: func(s *rdstypes.DBClusterSnapshot) { s.StorageEncrypted = aws.Bool(false) },
			want:   resource.ColorWarning,
		},
		{
			name:   "manual_old",
			status: "available",
			mutate: func(s *rdstypes.DBClusterSnapshot) {
				s.SnapshotCreateTime = aws.Time(time.Now().AddDate(-2, 0, 0))
			},
			want: resource.ColorWarning,
		},
		{
			name:   "automated_old_is_not_an_issue",
			status: "available",
			mutate: func(s *rdstypes.DBClusterSnapshot) {
				s.SnapshotType = aws.String("automated")
				s.SnapshotCreateTime = aws.Time(time.Now().AddDate(-2, 0, 0))
			},
			want: resource.ColorHealthy,
		},
		{
			name:   "no_create_time_is_not_an_issue",
			status: "available",
			mutate: func(s *rdstypes.DBClusterSnapshot) { s.SnapshotCreateTime = nil },
			want:   resource.ColorHealthy,
		},
		{
			name:   "broken_status_outranks_unencrypted",
			status: "failed",
			mutate: func(s *rdstypes.DBClusterSnapshot) { s.StorageEncrypted = aws.Bool(false) },
			want:   resource.ColorBroken,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := d1DbcSnapBaseline()
			snap.Status = aws.String(tc.status)
			if tc.mutate != nil {
				tc.mutate(&snap)
			}
			mock := &dbcSnapRDSSinglePageMock{
				output: &rds.DescribeDBClusterSnapshotsOutput{
					DBClusterSnapshots: []rdstypes.DBClusterSnapshot{snap},
				},
			}
			page, err := awsclient.FetchRDSDBClusterSnapshotsPage(context.Background(), mock, "")
			if err != nil {
				t.Fatalf("FetchRDSDBClusterSnapshotsPage: %v", err)
			}
			if len(page.Resources) != 1 {
				t.Fatalf("expected 1 resource, got %d", len(page.Resources))
			}
			d1AssertColor(t, page.Resources[0], tc.want)
		})
	}
}
