// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// EBFake implements aws.ElasticBeanstalkAPI against fixture data loaded at construction time.
type EBFake struct {
	fix *fixtures.EBFixtures
}

// NewEB constructs an EBFake backed by fixture data from the fixtures package.
func NewEB() *EBFake {
	return &EBFake{fix: fixtures.NewEBFixtures()}
}

func (f *EBFake) DescribeEnvironments(_ context.Context, _ *elasticbeanstalk.DescribeEnvironmentsInput, _ ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentsOutput, error) {
	return &elasticbeanstalk.DescribeEnvironmentsOutput{Environments: f.fix.Environments}, nil
}

// DescribeEnvironmentHealth is a no-op stub for demo mode.
// Wave 2 enrichment is skipped in demo mode; this satisfies the ElasticBeanstalkAPI interface.
func (f *EBFake) DescribeEnvironmentHealth(_ context.Context, _ *elasticbeanstalk.DescribeEnvironmentHealthInput, _ ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentHealthOutput, error) {
	return &elasticbeanstalk.DescribeEnvironmentHealthOutput{}, nil
}

// DescribeConfigurationSettings is a no-op stub for demo mode.
func (f *EBFake) DescribeConfigurationSettings(_ context.Context, _ *elasticbeanstalk.DescribeConfigurationSettingsInput, _ ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeConfigurationSettingsOutput, error) {
	return &elasticbeanstalk.DescribeConfigurationSettingsOutput{}, nil
}

// DescribeEnvironmentResources is a no-op stub for demo mode.
func (f *EBFake) DescribeEnvironmentResources(_ context.Context, _ *elasticbeanstalk.DescribeEnvironmentResourcesInput, _ ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentResourcesOutput, error) {
	return &elasticbeanstalk.DescribeEnvironmentResourcesOutput{}, nil
}

// DescribeApplicationVersions is a no-op stub satisfying EBDescribeApplicationVersionsAPI.
// Demo mode does not model Elastic Beanstalk application versions.
func (f *EBFake) DescribeApplicationVersions(_ context.Context, _ *elasticbeanstalk.DescribeApplicationVersionsInput, _ ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeApplicationVersionsOutput, error) {
	return &elasticbeanstalk.DescribeApplicationVersionsOutput{}, nil
}
