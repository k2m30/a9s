package fakes

import (
	"context"
	"slices"
	"sort"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/mwaa"
	mwaatypes "github.com/aws/aws-sdk-go-v2/service/mwaa/types"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// MWAAFake implements aws.MWAAAPI against fixture data loaded at construction time.
type MWAAFake struct {
	fix *fixtures.MWAAFixtures
}

// NewMWAA constructs an MWAAFake backed by fixture data from the fixtures package.
func NewMWAA() *MWAAFake {
	return &MWAAFake{fix: fixtures.NewMWAAFixtures()}
}

// ListEnvironments returns the fixture's environment names, sorted for
// deterministic demo/test output. MWAA's real ListEnvironments returns
// names only — every other field comes from GetEnvironment.
func (f *MWAAFake) ListEnvironments(_ context.Context, _ *mwaa.ListEnvironmentsInput, _ ...func(*mwaa.Options)) (*mwaa.ListEnvironmentsOutput, error) {
	names := make([]string, 0, len(f.fix.Environments)+len(f.fix.DeniedNames))
	for name := range f.fix.Environments {
		names = append(names, name)
	}
	names = append(names, f.fix.DeniedNames...)
	sort.Strings(names)
	return &mwaa.ListEnvironmentsOutput{Environments: names}, nil
}

// GetEnvironment returns the fixture's environment for the given name.
// Unknown names return ResourceNotFoundException, mirroring the real MWAA
// API — MWAA has no ARN-typed parameters, so this is the fake's strictness
// witness (analogous to SFNFake's ARN-shape validation).
func (f *MWAAFake) GetEnvironment(_ context.Context, input *mwaa.GetEnvironmentInput, _ ...func(*mwaa.Options)) (*mwaa.GetEnvironmentOutput, error) {
	name := aws.ToString(input.Name)
	if slices.Contains(f.fix.DeniedNames, name) {
		return nil, &mwaatypes.AccessDeniedException{
			Message: aws.String("User is not authorized to perform: airflow:GetEnvironment on resource: " + name),
		}
	}
	env, ok := f.fix.Environments[name]
	if !ok {
		return nil, &mwaatypes.ResourceNotFoundException{
			Message: aws.String("Environment " + name + " not found"),
		}
	}
	return &mwaa.GetEnvironmentOutput{Environment: &env}, nil
}
