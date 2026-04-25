package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/redshift"
	redshifttypes "github.com/aws/aws-sdk-go-v2/service/redshift/types"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// RedshiftFake implements aws.RedshiftAPI against fixture data loaded at construction time.
type RedshiftFake struct {
	fix *fixtures.RedshiftFixtures
}

// NewRedshift constructs a RedshiftFake backed by fixture data from the fixtures package.
func NewRedshift() *RedshiftFake {
	return &RedshiftFake{fix: fixtures.NewRedshiftFixtures()}
}

func (f *RedshiftFake) DescribeClusters(_ context.Context, _ *redshift.DescribeClustersInput, _ ...func(*redshift.Options)) (*redshift.DescribeClustersOutput, error) {
	return &redshift.DescribeClustersOutput{Clusters: f.fix.Clusters}, nil
}

// DescribeLoggingStatus routes by ClusterIdentifier:
//   - acme-warehouse → CloudWatch logging enabled (connectionlog, userlog, useractivitylog)
//   - acme-reporting → S3 logging enabled (BucketName = RedshiftAuditBucket)
//   - all others     → logging disabled
func (f *RedshiftFake) DescribeLoggingStatus(_ context.Context, in *redshift.DescribeLoggingStatusInput, _ ...func(*redshift.Options)) (*redshift.DescribeLoggingStatusOutput, error) {
	if in == nil || in.ClusterIdentifier == nil {
		return &redshift.DescribeLoggingStatusOutput{}, nil
	}
	clusterID := *in.ClusterIdentifier

	switch clusterID {
	case fixtures.AcmeWarehouseID:
		return &redshift.DescribeLoggingStatusOutput{
			LoggingEnabled:     aws.Bool(true),
			LogDestinationType: redshifttypes.LogDestinationTypeCloudwatch,
			LogExports:         []string{"connectionlog", "userlog", "useractivitylog"},
		}, nil
	case fixtures.AcmeReportingID:
		return &redshift.DescribeLoggingStatusOutput{
			LoggingEnabled:     aws.Bool(true),
			LogDestinationType: redshifttypes.LogDestinationTypeS3,
			BucketName:         aws.String(fixtures.RedshiftAuditBucket),
			S3KeyPrefix:        aws.String("audit/"),
		}, nil
	default:
		return &redshift.DescribeLoggingStatusOutput{
			LoggingEnabled: aws.Bool(false),
		}, nil
	}
}

// DescribeClusterSubnetGroups routes by ClusterSubnetGroupName:
//   - redshift-prod-subnet-group    → prod subnets (subnet-prod-a, subnet-prod-b)
//   - redshift-staging-subnet-group → staging subnets (subnet-staging-a, subnet-staging-b)
//   - any other name                → empty list
func (f *RedshiftFake) DescribeClusterSubnetGroups(_ context.Context, in *redshift.DescribeClusterSubnetGroupsInput, _ ...func(*redshift.Options)) (*redshift.DescribeClusterSubnetGroupsOutput, error) {
	if in == nil || in.ClusterSubnetGroupName == nil || *in.ClusterSubnetGroupName == "" {
		return &redshift.DescribeClusterSubnetGroupsOutput{}, nil
	}
	name := *in.ClusterSubnetGroupName

	switch name {
	case fixtures.RedshiftProdSubnetGroup:
		return &redshift.DescribeClusterSubnetGroupsOutput{
			ClusterSubnetGroups: []redshifttypes.ClusterSubnetGroup{
				{
					ClusterSubnetGroupName: aws.String(fixtures.RedshiftProdSubnetGroup),
					Description:            aws.String("Redshift subnet group for production clusters"),
					VpcId:                  aws.String("vpc-0abc123def456789a"),
					SubnetGroupStatus:      aws.String("Complete"),
					Subnets: []redshifttypes.Subnet{
						{SubnetIdentifier: aws.String("subnet-prod-a"), SubnetStatus: aws.String("Active")},
						{SubnetIdentifier: aws.String("subnet-prod-b"), SubnetStatus: aws.String("Active")},
					},
				},
			},
		}, nil
	case fixtures.RedshiftStagingSubnetGroup:
		return &redshift.DescribeClusterSubnetGroupsOutput{
			ClusterSubnetGroups: []redshifttypes.ClusterSubnetGroup{
				{
					ClusterSubnetGroupName: aws.String(fixtures.RedshiftStagingSubnetGroup),
					Description:            aws.String("Redshift subnet group for staging clusters"),
					VpcId:                  aws.String("vpc-0def456789abc123d"),
					SubnetGroupStatus:      aws.String("Complete"),
					Subnets: []redshifttypes.Subnet{
						{SubnetIdentifier: aws.String("subnet-staging-a"), SubnetStatus: aws.String("Active")},
						{SubnetIdentifier: aws.String("subnet-staging-b"), SubnetStatus: aws.String("Active")},
					},
				},
			},
		}, nil
	default:
		return &redshift.DescribeClusterSubnetGroupsOutput{}, nil
	}
}
