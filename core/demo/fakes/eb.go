// Package fakes provides in-process fake implementations of AWS service
// interfaces for use in demo mode and tests.
package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
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

// DescribeEnvironmentHealth returns the fixture-registered health causes for
// the requested environment name (see EBFixtures.EnvironmentHealthCauses).
// Required for EnrichEBEnvironmentHealth's Wave-2 issue check.
func (f *EBFake) DescribeEnvironmentHealth(_ context.Context, input *elasticbeanstalk.DescribeEnvironmentHealthInput, _ ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentHealthOutput, error) {
	if input == nil || input.EnvironmentName == nil {
		return &elasticbeanstalk.DescribeEnvironmentHealthOutput{}, nil
	}
	return &elasticbeanstalk.DescribeEnvironmentHealthOutput{
		Causes: f.fix.EnvironmentHealthCauses[aws.ToString(input.EnvironmentName)],
	}, nil
}

// DescribeConfigurationSettings returns the fixture-registered configuration
// set for the requested application/environment pair, keyed by
// "applicationName/environmentName" (see EBFixtures.ConfigurationSettings).
func (f *EBFake) DescribeConfigurationSettings(_ context.Context, input *elasticbeanstalk.DescribeConfigurationSettingsInput, _ ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeConfigurationSettingsOutput, error) {
	if input == nil || input.ApplicationName == nil || input.EnvironmentName == nil {
		return &elasticbeanstalk.DescribeConfigurationSettingsOutput{}, nil
	}
	key := aws.ToString(input.ApplicationName) + "/" + aws.ToString(input.EnvironmentName)
	return &elasticbeanstalk.DescribeConfigurationSettingsOutput{
		ConfigurationSettings: f.fix.ConfigurationSettings[key],
	}, nil
}

// DescribeEnvironmentResources returns the fixture-registered resources for
// the requested environment name (see EBFixtures.EnvironmentResources).
func (f *EBFake) DescribeEnvironmentResources(_ context.Context, input *elasticbeanstalk.DescribeEnvironmentResourcesInput, _ ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeEnvironmentResourcesOutput, error) {
	if input == nil || input.EnvironmentName == nil {
		return &elasticbeanstalk.DescribeEnvironmentResourcesOutput{}, nil
	}
	return &elasticbeanstalk.DescribeEnvironmentResourcesOutput{
		EnvironmentResources: f.fix.EnvironmentResources[aws.ToString(input.EnvironmentName)],
	}, nil
}

// DescribeApplicationVersions returns the fixture-registered versions for
// the requested application name (see EBFixtures.ApplicationVersions).
func (f *EBFake) DescribeApplicationVersions(_ context.Context, input *elasticbeanstalk.DescribeApplicationVersionsInput, _ ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeApplicationVersionsOutput, error) {
	if input == nil || input.ApplicationName == nil {
		return &elasticbeanstalk.DescribeApplicationVersionsOutput{}, nil
	}
	return &elasticbeanstalk.DescribeApplicationVersionsOutput{
		ApplicationVersions: f.fix.ApplicationVersions[aws.ToString(input.ApplicationName)],
	}, nil
}
