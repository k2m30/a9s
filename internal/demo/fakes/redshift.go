package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/redshift"

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

// DescribeLoggingStatus returns a zero LoggingStatus — demo mode does not
// model Redshift audit-log configuration.
func (f *RedshiftFake) DescribeLoggingStatus(_ context.Context, _ *redshift.DescribeLoggingStatusInput, _ ...func(*redshift.Options)) (*redshift.DescribeLoggingStatusOutput, error) {
	return &redshift.DescribeLoggingStatusOutput{}, nil
}

// DescribeClusterSubnetGroups returns an empty list — demo mode does not
// model Redshift cluster subnet groups.
func (f *RedshiftFake) DescribeClusterSubnetGroups(_ context.Context, _ *redshift.DescribeClusterSubnetGroupsInput, _ ...func(*redshift.Options)) (*redshift.DescribeClusterSubnetGroupsOutput, error) {
	return &redshift.DescribeClusterSubnetGroupsOutput{}, nil
}
