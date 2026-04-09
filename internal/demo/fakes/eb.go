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
