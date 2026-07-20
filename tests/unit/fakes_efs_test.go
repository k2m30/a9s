// fakes_efs_test.go is the single source for EFS SDK-interface fakes shared
// across tests/unit.
//
// Convention (docs/go-codebase-checklist.md §DRY): one configurable fake per
// SDK interface, in a service-named fakes_<service>_test.go file — never
// re-implement the same interface under a new name in another file, and never
// add another wave/batch-named fake file (fakes_us1_batchN_test.go,
// fakes_related_checkers_branch_coverage_test.go, fakes_related_checkers_misc_test.go are historical
// accretion naming, not a pattern to extend).
package unit

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/efs"
)

// fakeEFSDescribeFileSystems implements awsclient.EFSDescribeFileSystemsAPI.
//
// Configure at most one response mode:
//   - Err: always return this error
//   - Output: always return this single output (no pagination)
//   - Pages: sequential paged outputs, one per call, in order (empty output
//     once exhausted)
//   - PageFunc: full control, receives the 1-based call number, takes
//     precedence over Pages/Output/Err when set
//
// Calls, Inputs and LastInput are always recorded so tests can assert on
// call count and forwarded pagination tokens regardless of response mode.
type fakeEFSDescribeFileSystems struct {
	Err      error
	Output   *efs.DescribeFileSystemsOutput
	Pages    []*efs.DescribeFileSystemsOutput
	PageFunc func(call int) (*efs.DescribeFileSystemsOutput, error)

	Calls     int
	Inputs    []*efs.DescribeFileSystemsInput
	LastInput *efs.DescribeFileSystemsInput
}

func (f *fakeEFSDescribeFileSystems) DescribeFileSystems(
	_ context.Context,
	params *efs.DescribeFileSystemsInput,
	_ ...func(*efs.Options),
) (*efs.DescribeFileSystemsOutput, error) {
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
			return &efs.DescribeFileSystemsOutput{}, nil
		}
		f.Calls++
		return f.Pages[idx], nil
	}
	if f.Output != nil {
		return f.Output, nil
	}
	return &efs.DescribeFileSystemsOutput{}, nil
}
