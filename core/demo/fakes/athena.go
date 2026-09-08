// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fakes

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/athena"
	athenatypes "github.com/aws/aws-sdk-go-v2/service/athena/types"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
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
	if input == nil || input.WorkGroup == nil {
		return &athena.GetWorkGroupOutput{}, nil
	}
	if !f.hasWorkGroup(*input.WorkGroup) {
		// Not the shared wording: InvalidRequestException is also what athena
		// answers for a query it could not parse, so the classifier reads this
		// sentence and not the code alone (core/aws/partial_errors.go's
		// notFoundCodes). It is athena's own.
		return nil, &athenatypes.InvalidRequestException{
			Message: aws.String("WorkGroup " + *input.WorkGroup + " is not found."),
		}
	}
	if out, ok := f.fix.WorkGroupDetails[*input.WorkGroup]; ok && out != nil {
		return out, nil
	}
	return &athena.GetWorkGroupOutput{}, nil
}

// hasWorkGroup reports whether the fixtures register this workgroup. A
// workgroup listed without a per-config detail is still a workgroup.
func (f *AthenaFake) hasWorkGroup(name string) bool {
	return slices.ContainsFunc(f.fix.WorkGroups, func(w athenatypes.WorkGroupSummary) bool {
		return aws.ToString(w.Name) == name
	})
}
