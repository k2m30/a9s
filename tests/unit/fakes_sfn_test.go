// fakes_sfn_test.go is the single source for Step Functions SDK-interface
// fakes shared across tests/unit.
//
// Convention (docs/go-codebase-checklist.md, DRY section): one configurable
// fake per SDK interface, in a service-named fakes_<service>_test.go file --
// never re-implement the same interface under a new name in another file,
// and never add another wave/batch-named fake file (fakes_us1_batchN_test.go,
// fakes_related_checkers_branch_coverage_test.go, fakes_related_checkers_misc_test.go are historical
// accretion naming, not a pattern to extend).
package unit

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/sfn"
)

// fakeSFNListStateMachines implements awsclient.SFNListStateMachinesAPI.
//
// Configure at most one response mode:
//   - Err: always return this error
//   - Output: always return this single output (no pagination)
//   - Pages: sequential paged outputs, one per call, in order
//   - PageFunc: full control, receives the 1-based call number
//
// Calls, Inputs and LastInput are always recorded.
type fakeSFNListStateMachines struct {
	Err      error
	Output   *sfn.ListStateMachinesOutput
	Pages    []*sfn.ListStateMachinesOutput
	PageFunc func(call int) (*sfn.ListStateMachinesOutput, error)

	Calls     int
	Inputs    []*sfn.ListStateMachinesInput
	LastInput *sfn.ListStateMachinesInput
}

func (f *fakeSFNListStateMachines) ListStateMachines(
	_ context.Context,
	params *sfn.ListStateMachinesInput,
	_ ...func(*sfn.Options),
) (*sfn.ListStateMachinesOutput, error) {
	f.Inputs = append(f.Inputs, params)
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
			return &sfn.ListStateMachinesOutput{}, nil
		}
		f.Calls++
		return f.Pages[idx], nil
	}
	if f.Output != nil {
		return f.Output, nil
	}
	return &sfn.ListStateMachinesOutput{}, nil
}
