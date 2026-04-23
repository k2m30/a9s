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

// GetWorkGroup returns the pre-built WorkGroup detail from fixtures for the
// requested workgroup name, or an empty output when no fixture detail exists
// (healthy default representation — list-only workgroups without per-config).
func (f *AthenaFake) GetWorkGroup(_ context.Context, input *athena.GetWorkGroupInput, _ ...func(*athena.Options)) (*athena.GetWorkGroupOutput, error) {
	if input != nil && input.WorkGroup != nil {
		if out, ok := f.fix.WorkGroupDetails[*input.WorkGroup]; ok && out != nil {
			return out, nil
		}
	}
	return &athena.GetWorkGroupOutput{}, nil
}
