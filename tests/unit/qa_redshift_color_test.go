package unit

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"

	"github.com/k2m30/a9s/v3/core/resource"
)

// d1RedshiftBaseline is a healthy cluster: available on both status axes,
// encrypted, private, with nothing pending or deferred.
func d1RedshiftBaseline() redshifttypes.Cluster {
	return redshifttypes.Cluster{
		ClusterIdentifier:         aws.String("acme-warehouse"),
		ClusterNamespaceArn:       aws.String("arn:aws:redshift:us-east-1:123456789012:namespace/acme-warehouse"),
		NodeType:                  aws.String("ra3.xlplus"),
		NumberOfNodes:             aws.Int32(2),
		ClusterStatus:             aws.String("available"),
		ClusterAvailabilityStatus: aws.String("Available"),
		Encrypted:                 aws.Bool(true),
		PubliclyAccessible:        aws.Bool(false),
	}
}

// TestRedshiftColor pins the cluster status → colour mapping for Redshift.
//
// The old table read Fields["cluster_status"] directly. The raw enum is still
// the input, but it now reaches the colour through the fetcher's findings, so
// the two status axes and the posture flags are compared the way production
// compares them.
func TestRedshiftColor(t *testing.T) {
	cases := []struct {
		name   string
		status string
		mutate func(*redshifttypes.Cluster)
		want   resource.Color
	}{
		{name: "available", status: "available", want: resource.ColorHealthy},
		{name: "empty_status", status: "", want: resource.ColorHealthy},

		{name: "creating", status: "creating", want: resource.ColorWarning},
		{name: "modifying", status: "modifying", want: resource.ColorWarning},
		{name: "resizing", status: "resizing", want: resource.ColorWarning},
		{name: "rebooting", status: "rebooting", want: resource.ColorWarning},
		{name: "renaming", status: "renaming", want: resource.ColorWarning},
		{name: "deleting", status: "deleting", want: resource.ColorWarning},

		{name: "incompatible_hsm", status: "incompatible-hsm", want: resource.ColorBroken},
		{name: "incompatible_network", status: "incompatible-network", want: resource.ColorBroken},
		{name: "incompatible_parameters", status: "incompatible-parameters", want: resource.ColorBroken},
		{name: "incompatible_restore", status: "incompatible-restore", want: resource.ColorBroken},
		{name: "hardware_failure", status: "hardware-failure", want: resource.ColorBroken},
		{name: "storage_full", status: "storage-full", want: resource.ColorBroken},

		{
			name:   "availability_unavailable",
			status: "available",
			mutate: func(c *redshifttypes.Cluster) { c.ClusterAvailabilityStatus = aws.String("Unavailable") },
			want:   resource.ColorBroken,
		},
		{
			name:   "availability_failed",
			status: "available",
			mutate: func(c *redshifttypes.Cluster) { c.ClusterAvailabilityStatus = aws.String("Failed") },
			want:   resource.ColorBroken,
		},
		{
			name:   "availability_maintenance",
			status: "available",
			mutate: func(c *redshifttypes.Cluster) { c.ClusterAvailabilityStatus = aws.String("Maintenance") },
			want:   resource.ColorWarning,
		},
		{
			name:   "availability_modifying",
			status: "available",
			mutate: func(c *redshifttypes.Cluster) { c.ClusterAvailabilityStatus = aws.String("Modifying") },
			want:   resource.ColorWarning,
		},
		{
			name:   "publicly_accessible",
			status: "available",
			mutate: func(c *redshifttypes.Cluster) { c.PubliclyAccessible = aws.Bool(true) },
			want:   resource.ColorWarning,
		},
		{
			name:   "unencrypted",
			status: "available",
			mutate: func(c *redshifttypes.Cluster) { c.Encrypted = aws.Bool(false) },
			want:   resource.ColorWarning,
		},
		{
			name:   "broken_status_outranks_posture_warnings",
			status: "hardware-failure",
			mutate: func(c *redshifttypes.Cluster) {
				c.Encrypted = aws.Bool(false)
				c.PubliclyAccessible = aws.Bool(true)
			},
			want: resource.ColorBroken,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cluster := d1RedshiftBaseline()
			cluster.ClusterStatus = aws.String(tc.status)
			if tc.mutate != nil {
				tc.mutate(&cluster)
			}
			d1AssertColor(t, fetchSingleCluster(t, cluster), tc.want)
		})
	}
}
