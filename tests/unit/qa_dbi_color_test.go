package unit

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/resource"
)

// TestDbiColor tests the Color function for DB Instances (dbi).
//
// The production Color func reads "status" (the canonical fetcher key; the
// legacy "db_instance_status" fallback was removed in #284) and delegates to
// rdsInstanceColor. It also honors backup_retention_period, publicly_accessible,
// storage_encrypted, and deletion_protection — all tested below.
func TestDbiColor(t *testing.T) {
	// Each case feeds the status to the dbi fetcher, which runs the type's own
	// findings predicate, and the expected colour is computed from the severity
	// of the finding that comes back. The bare-Fields cases this table used to
	// carry — empty status, nil Fields, the legacy db_instance_status key and
	// its precedence — are gone with the raw-field classifier they pinned.
	cases := []struct {
		name   string
		status string
		mutate func(*rdstypes.DBInstance)
		want   resource.Color
	}{
		{name: "available", status: "available", want: resource.ColorHealthy},

		{name: "failed", status: "failed", want: resource.ColorBroken},
		// An instance you must restart before it can serve traffic is broken,
		// not paused; this row moved here from the field-key table.
		{name: "stopped", status: "stopped", want: resource.ColorBroken},
		{name: "storage_full", status: "storage-full", want: resource.ColorBroken},
		{name: "incompatible_parameters", status: "incompatible-parameters", want: resource.ColorBroken},
		{name: "inaccessible_encryption_credentials", status: "inaccessible-encryption-credentials", want: resource.ColorBroken},
		{name: "restore_error", status: "restore-error", want: resource.ColorBroken},
		{name: "incompatible_network", status: "incompatible-network", want: resource.ColorBroken},

		{name: "creating", status: "creating", want: resource.ColorWarning},
		{name: "modifying", status: "modifying", want: resource.ColorWarning},
		{name: "backing_up", status: "backing-up", want: resource.ColorWarning},
		{name: "rebooting", status: "rebooting", want: resource.ColorWarning},
		{name: "upgrading", status: "upgrading", want: resource.ColorWarning},
		{name: "stopping", status: "stopping", want: resource.ColorWarning},
		{name: "starting", status: "starting", want: resource.ColorWarning},
		{name: "deleting", status: "deleting", want: resource.ColorWarning},

		{
			name:   "publicly_accessible_is_a_warning",
			status: "available",
			mutate: func(i *rdstypes.DBInstance) { i.PubliclyAccessible = aws.Bool(true) },
			want:   resource.ColorWarning,
		},
		{
			name:   "broken_status_outranks_a_posture_warning",
			status: "failed",
			mutate: func(i *rdstypes.DBInstance) { i.PubliclyAccessible = aws.Bool(true) },
			want:   resource.ColorBroken,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inst := findDBI(t, fixtures.ProdDbiID)
			inst.DBInstanceStatus = aws.String(tc.status)
			if tc.mutate != nil {
				tc.mutate(&inst)
			}
			d1AssertColor(t, fetchSingleResource(t, inst), tc.want)
		})
	}
}
