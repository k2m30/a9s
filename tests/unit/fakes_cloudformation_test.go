// fakes_cloudformation_test.go is the single source for CloudFormation
// SDK-interface fakes shared across tests/unit.
//
// Convention (docs/go-codebase-checklist.md, DRY section): one configurable
// fake per SDK interface, in a service-named fakes_<service>_test.go file —
// never re-implement the same interface under a new name in another file, and
// never add a wave/batch-named fake file.
package unit

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
)

// fakeCFNListStackResources implements awsclient.CFNListStackResourcesAPI.
//
// Configure at most one response mode:
//   - Err: always return this error
//   - Output: always return this single output (no pagination)
//   - Pages: sequential paged outputs, one per call, in order
//   - PageFunc: full control, receives the 1-based call number
//
// LastInput is always recorded.
type fakeCFNListStackResources struct {
	Err      error
	Output   *cloudformation.ListStackResourcesOutput
	Pages    []*cloudformation.ListStackResourcesOutput
	PageFunc func(call int) (*cloudformation.ListStackResourcesOutput, error)

	Calls     int
	LastInput *cloudformation.ListStackResourcesInput
}

func (f *fakeCFNListStackResources) ListStackResources(
	_ context.Context,
	params *cloudformation.ListStackResourcesInput,
	_ ...func(*cloudformation.Options),
) (*cloudformation.ListStackResourcesOutput, error) {
	f.LastInput = params

	if f.PageFunc != nil {
		f.Calls++
		return f.PageFunc(f.Calls)
	}
	if f.Err != nil {
		return nil, f.Err
	}
	if len(f.Pages) > 0 {
		idx := f.Calls
		if idx >= len(f.Pages) {
			return &cloudformation.ListStackResourcesOutput{}, nil
		}
		f.Calls++
		return f.Pages[idx], nil
	}
	if f.Output != nil {
		return f.Output, nil
	}
	return &cloudformation.ListStackResourcesOutput{}, nil
}
