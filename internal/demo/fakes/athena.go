package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/athena"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
)

// AthenaFake implements aws.AthenaAPI against fixture data loaded at construction time.
type AthenaFake struct {
	fix *fixtures.AthenaFixtures
}

// NewAthena constructs an AthenaFake backed by fixture data from the fixtures package.
func NewAthena() *AthenaFake {
	return &AthenaFake{fix: fixtures.NewAthenaFixtures()}
}

func (f *AthenaFake) ListWorkGroups(_ context.Context, _ *athena.ListWorkGroupsInput, _ ...func(*athena.Options)) (*athena.ListWorkGroupsOutput, error) {
	return &athena.ListWorkGroupsOutput{WorkGroups: f.fix.WorkGroups}, nil
}

// GetWorkGroup is a stub satisfying AthenaGetWorkGroupAPI.
// Demo mode returns a workgroup with EnforceWorkGroupConfiguration enabled and encryption configured,
// representing a healthy default workgroup.
func (f *AthenaFake) GetWorkGroup(_ context.Context, _ *athena.GetWorkGroupInput, _ ...func(*athena.Options)) (*athena.GetWorkGroupOutput, error) {
	return &athena.GetWorkGroupOutput{}, nil
}
