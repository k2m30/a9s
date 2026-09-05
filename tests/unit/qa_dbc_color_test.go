package unit

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// d1DbcBaseline is a posture-clean Aurora cluster: every predicate in
// computeRDSDBClusterFindings and the shared rds_posture pack answers "no
// finding". Each case mutates exactly the field its row is about, so a
// finding that appears is the one the case names and nothing else.
func d1DbcBaseline() rdstypes.DBCluster {
	return rdstypes.DBCluster{
		DBClusterIdentifier:              aws.String("acme-aurora-prod"),
		DBClusterArn:                     aws.String("arn:aws:rds:us-east-1:123456789012:cluster:acme-aurora-prod"),
		Engine:                           aws.String("aurora-postgresql"),
		Status:                           aws.String("available"),
		StorageEncrypted:                 aws.Bool(true),
		DeletionProtection:               aws.Bool(true),
		BackupRetentionPeriod:            aws.Int32(7),
		MultiAZ:                          aws.Bool(true),
		AutoMinorVersionUpgrade:          aws.Bool(true),
		IAMDatabaseAuthenticationEnabled: aws.Bool(true),
		MasterUsername:                   aws.String("acmeadmin"),
		DBClusterMembers: []rdstypes.DBClusterMember{
			{DBInstanceIdentifier: aws.String("acme-aurora-prod-1"), IsClusterWriter: aws.Bool(true)},
		},
	}
}

// TestDbcColor pins the status → colour mapping for DB clusters.
//
// The row colour is the worst severity among the cluster's findings. Each case
// feeds an SDK DBCluster through the dbc fetcher and compares the colour its
// findings imply against the colour the row expects.
//
// Three families of case from the phrase-matching era are gone with the
// classifier that needed them: an empty or nil Fields map (no SDK struct
// produces one), the "(+N)" suffix cases (the suffix is a display artefact of
// StatusPhrase, never seen by a severity comparison), and the wave-2
// "maintenance overdue" phrase (the enricher owns it; prowler_w2_dbc covers it).
func TestDbcColor(t *testing.T) {
	cases := []struct {
		name   string
		status string
		mutate func(*rdstypes.DBCluster)
		want   resource.Color
	}{
		{name: "available", status: "available", want: resource.ColorHealthy},

		{name: "failed_cluster_operation", status: "failed", want: resource.ColorBroken},
		{name: "encryption_key_unreachable", status: "inaccessible-encryption-credentials", want: resource.ColorBroken},
		{name: "parameter_group_incompatible", status: "incompatible-parameters", want: resource.ColorBroken},
		{
			name:   "no_writer_reads_only",
			status: "available",
			mutate: func(c *rdstypes.DBCluster) {
				c.DBClusterMembers = []rdstypes.DBClusterMember{
					{DBInstanceIdentifier: aws.String("acme-aurora-prod-1"), IsClusterWriter: aws.Bool(false)},
				}
			},
			want: resource.ColorBroken,
		},

		{
			name:   "delete_protection_off",
			status: "available",
			mutate: func(c *rdstypes.DBCluster) { c.DeletionProtection = aws.Bool(false) },
			want:   resource.ColorWarning,
		},
		{
			name:   "not_encrypted_at_rest",
			status: "available",
			mutate: func(c *rdstypes.DBCluster) { c.StorageEncrypted = aws.Bool(false) },
			want:   resource.ColorWarning,
		},
		{
			name:   "no_automated_backups",
			status: "available",
			mutate: func(c *rdstypes.DBCluster) { c.BackupRetentionPeriod = aws.Int32(0) },
			want:   resource.ColorWarning,
		},

		{name: "creating", status: "creating", want: resource.ColorWarning},
		{name: "modifying", status: "modifying", want: resource.ColorWarning},
		{name: "deleting", status: "deleting", want: resource.ColorWarning},

		// A status a9s does not enumerate is reported, not swallowed: the
		// fetcher passes the raw keyword through as a warning. The old table
		// expected Healthy here because the phrase classifier's default arm was
		// "no match, stay green" — that default made every future AWS status
		// invisible, which is the opposite of future-proof.
		{name: "unknown_status_is_reported", status: "some-future-aws-status", want: resource.ColorWarning},

		{
			name:   "broken_status_outranks_a_posture_warning",
			status: "failed",
			mutate: func(c *rdstypes.DBCluster) { c.StorageEncrypted = aws.Bool(false) },
			want:   resource.ColorBroken,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cluster := d1DbcBaseline()
			cluster.Status = aws.String(tc.status)
			if tc.mutate != nil {
				tc.mutate(&cluster)
			}
			page, err := rdsClusterPage(t, cluster)
			if err != nil {
				t.Fatalf("FetchRDSDBClustersPage: %v", err)
			}
			if len(page.Resources) != 1 {
				t.Fatalf("expected 1 resource, got %d", len(page.Resources))
			}
			d1AssertColor(t, page.Resources[0], tc.want)
		})
	}
}
