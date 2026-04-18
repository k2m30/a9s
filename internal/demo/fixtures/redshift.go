package fixtures

import (
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"
)

const (
	redshiftProdVPCID    = "vpc-0abc123def456789a"
	redshiftStagingVPCID = "vpc-0def456789abc123d"
)

// RedshiftFixtures holds typed fixture data for Redshift.
type RedshiftFixtures struct {
	Clusters []redshifttypes.Cluster
}

func mustParseRedshiftTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

// NewRedshiftFixtures constructs RedshiftFixtures from the canonical demo data.
func NewRedshiftFixtures() *RedshiftFixtures {
	return &RedshiftFixtures{
		Clusters: []redshifttypes.Cluster{
			{
				ClusterIdentifier:   aws.String("acme-warehouse"),
				ClusterStatus:       aws.String("available"),
				NodeType:            aws.String("ra3.xlplus"),
				NumberOfNodes:       aws.Int32(4),
				DBName:              aws.String("analytics"),
				MasterUsername:      aws.String("admin"),
				ClusterCreateTime:   aws.Time(mustParseRedshiftTime("2025-03-10T09:00:00+00:00")),
				ClusterNamespaceArn: aws.String("arn:aws:redshift:us-east-1:123456789012:namespace:acme-warehouse"),
				AvailabilityZone:    aws.String("us-east-1a"),
				VpcId:               aws.String(redshiftProdVPCID),
				Endpoint: &redshifttypes.Endpoint{
					Address: aws.String("acme-warehouse.c9xyz123.us-east-1.redshift.amazonaws.com"),
					Port:    aws.Int32(5439),
				},
			},
			{
				ClusterIdentifier:   aws.String("acme-reporting"),
				ClusterStatus:       aws.String("available"),
				NodeType:            aws.String("ra3.xlplus"),
				NumberOfNodes:       aws.Int32(2),
				DBName:              aws.String("reporting"),
				MasterUsername:      aws.String("admin"),
				ClusterCreateTime:   aws.Time(mustParseRedshiftTime("2025-07-22T14:30:00+00:00")),
				ClusterNamespaceArn: aws.String("arn:aws:redshift:us-east-1:123456789012:namespace:acme-reporting"),
				AvailabilityZone:    aws.String("us-east-1b"),
				VpcId:               aws.String(redshiftProdVPCID),
				Endpoint: &redshifttypes.Endpoint{
					Address: aws.String("acme-reporting.c9xyz123.us-east-1.redshift.amazonaws.com"),
					Port:    aws.Int32(5439),
				},
			},
			{
				ClusterIdentifier:   aws.String("staging-dwh"),
				ClusterStatus:       aws.String("paused"),
				NodeType:            aws.String("dc2.large"),
				NumberOfNodes:       aws.Int32(2),
				DBName:              aws.String("staging"),
				MasterUsername:      aws.String("stgadmin"),
				ClusterCreateTime:   aws.Time(mustParseRedshiftTime("2025-10-15T08:00:00+00:00")),
				ClusterNamespaceArn: aws.String("arn:aws:redshift:us-east-1:123456789012:namespace:staging-dwh"),
				AvailabilityZone:    aws.String("us-east-1a"),
				VpcId:               aws.String(redshiftStagingVPCID),
				Endpoint: &redshifttypes.Endpoint{
					Address: aws.String("staging-dwh.c9xyz123.us-east-1.redshift.amazonaws.com"),
					Port:    aws.Int32(5439),
				},
			},
			// Issue: ClusterStatus=resizing → Warning (online resize in progress)
			{
				ClusterIdentifier:   aws.String("redshift-resizing"),
				ClusterStatus:       aws.String("resizing"),
				NodeType:            aws.String("ra3.xlplus"),
				NumberOfNodes:       aws.Int32(4),
				DBName:              aws.String("analytics"),
				MasterUsername:      aws.String("admin"),
				ClusterCreateTime:   aws.Time(mustParseRedshiftTime("2025-05-01T10:00:00+00:00")),
				ClusterNamespaceArn: aws.String("arn:aws:redshift:us-east-1:123456789012:namespace:redshift-resizing"),
				AvailabilityZone:    aws.String("us-east-1a"),
				VpcId:               aws.String(redshiftProdVPCID),
				Endpoint: &redshifttypes.Endpoint{
					Address: aws.String("redshift-resizing.c9xyz123.us-east-1.redshift.amazonaws.com"),
					Port:    aws.Int32(5439),
				},
			},
			// Issue: ClusterStatus=incompatible-network → Broken (VPC routing broken)
			{
				ClusterIdentifier:   aws.String("redshift-incompatible-network"),
				ClusterStatus:       aws.String("incompatible-network"),
				NodeType:            aws.String("ra3.xlplus"),
				NumberOfNodes:       aws.Int32(2),
				DBName:              aws.String("prod"),
				MasterUsername:      aws.String("admin"),
				ClusterCreateTime:   aws.Time(mustParseRedshiftTime("2025-11-10T08:00:00+00:00")),
				ClusterNamespaceArn: aws.String("arn:aws:redshift:us-east-1:123456789012:namespace:redshift-incompatible-network"),
				AvailabilityZone:    aws.String("us-east-1b"),
				VpcId:               aws.String(redshiftProdVPCID),
			},
			// Issue: ClusterStatus=storage-full → Broken (disk exhausted)
			{
				ClusterIdentifier:   aws.String("redshift-storage-full"),
				ClusterStatus:       aws.String("storage-full"),
				NodeType:            aws.String("dc2.large"),
				NumberOfNodes:       aws.Int32(2),
				DBName:              aws.String("dwh"),
				MasterUsername:      aws.String("admin"),
				ClusterCreateTime:   aws.Time(mustParseRedshiftTime("2025-08-20T14:00:00+00:00")),
				ClusterNamespaceArn: aws.String("arn:aws:redshift:us-east-1:123456789012:namespace:redshift-storage-full"),
				AvailabilityZone:    aws.String("us-east-1a"),
				VpcId:               aws.String(redshiftProdVPCID),
				Endpoint: &redshifttypes.Endpoint{
					Address: aws.String("redshift-storage-full.c9xyz123.us-east-1.redshift.amazonaws.com"),
					Port:    aws.Int32(5439),
				},
			},
		},
	}
}
