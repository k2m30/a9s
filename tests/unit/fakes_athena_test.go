// fakes_athena_test.go is the single source for Athena SDK-interface fakes
// shared across tests/unit.
//
// Convention (docs/go-codebase-checklist.md §DRY): one configurable fake per
// SDK interface, in a service-named fakes_<service>_test.go file — never
// re-implement the same interface under a new name in another file, and never
// add another wave/batch-named fake file (fakes_us1_batchN_test.go,
// fakes_wave5_test.go, fakes_coverage_restore_test.go are historical
// accretion naming, not a pattern to extend).
package unit

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/athena"
)

// fakeAthenaListWorkGroups implements awsclient.AthenaListWorkGroupsAPI.
//
// Configure at most one response mode:
//   - Err: always return this error
//   - Output: always return this single output (no pagination)
//   - Pages: sequential paged outputs, one per call, in order
//   - PageFunc: full control, receives the 1-based call number
//
// Calls, Inputs and LastInput are always recorded so tests can assert on
// call count and forwarded pagination tokens regardless of response mode.
type fakeAthenaListWorkGroups struct {
	Err      error
	Output   *athena.ListWorkGroupsOutput
	Pages    []*athena.ListWorkGroupsOutput
	PageFunc func(call int) (*athena.ListWorkGroupsOutput, error)

	Calls     int
	Inputs    []*athena.ListWorkGroupsInput
	LastInput *athena.ListWorkGroupsInput
}

func (f *fakeAthenaListWorkGroups) ListWorkGroups(
	_ context.Context,
	params *athena.ListWorkGroupsInput,
	_ ...func(*athena.Options),
) (*athena.ListWorkGroupsOutput, error) {
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
			return &athena.ListWorkGroupsOutput{}, nil
		}
		f.Calls++
		return f.Pages[idx], nil
	}
	if f.Output != nil {
		return f.Output, nil
	}
	return &athena.ListWorkGroupsOutput{}, nil
}
