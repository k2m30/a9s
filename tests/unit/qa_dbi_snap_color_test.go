package unit

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// d1DbiSnapBaseline is a healthy RDS snapshot: available and encrypted, so
// ComputeDBISnapStatusAndIssues returns nothing for it.
func d1DbiSnapBaseline() rdstypes.DBSnapshot {
	return rdstypes.DBSnapshot{
		DBSnapshotIdentifier: aws.String("acme-pg-prod-snap"),
		DBSnapshotArn:        aws.String("arn:aws:rds:us-east-1:123456789012:snapshot:acme-pg-prod-snap"),
		DBInstanceIdentifier: aws.String("acme-pg-prod"),
		Status:               aws.String("available"),
		Encrypted:            aws.Bool(true),
		SnapshotType:         aws.String("manual"),
		PercentProgress:      aws.Int32(100),
	}
}

// TestDbiSnapColor pins the status → colour mapping for DB instance snapshots.
//
// Each case feeds an SDK DBSnapshot through the dbi-snap fetcher; the expected
// colour is the one the worst finding's severity implies. The "empty status"
// case survives the conversion because a snapshot with no Status is a shape the
// SDK really returns, unlike the empty Fields map the old table also carried.
func TestDbiSnapColor(t *testing.T) {
	cases := []struct {
		name   string
		status string
		mutate func(*rdstypes.DBSnapshot)
		want   resource.Color
	}{
		{name: "available", status: "available", want: resource.ColorHealthy},
		{name: "empty_status", status: "", want: resource.ColorHealthy},

		{name: "creating", status: "creating", want: resource.ColorWarning},
		// A snapshot being copied is not yet restorable, so the row has to say
		// so. The phrase classifier warned on "copying"; the findings predicate
		// has no branch for it.
		{name: "copying", status: "copying", want: resource.ColorWarning},

		{name: "failed", status: "failed", want: resource.ColorBroken},
		{name: "incompatible_restore", status: "incompatible-restore", want: resource.ColorBroken},
		{name: "incompatible_parameters", status: "incompatible-parameters", want: resource.ColorBroken},

		{
			name:   "unencrypted",
			status: "available",
			mutate: func(s *rdstypes.DBSnapshot) { s.Encrypted = aws.Bool(false) },
			want:   resource.ColorWarning,
		},
		{
			name:   "broken_status_outranks_unencrypted",
			status: "failed",
			mutate: func(s *rdstypes.DBSnapshot) { s.Encrypted = aws.Bool(false) },
			want:   resource.ColorBroken,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := d1DbiSnapBaseline()
			snap.Status = aws.String(tc.status)
			if tc.mutate != nil {
				tc.mutate(&snap)
			}
			rows := fetchSnap(t, snap)
			if len(rows) != 1 {
				t.Fatalf("expected 1 row, got %d", len(rows))
			}
			d1AssertColor(t, resource.Resource{ID: rows[0].id, Findings: rows[0].findings}, tc.want)
		})
	}
}
