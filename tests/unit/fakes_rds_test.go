// fakes_rds_test.go is the single source for RDS SDK-interface fakes shared
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

	"github.com/aws/aws-sdk-go-v2/service/rds"
)

// fakeRDSDescribeDBInstances implements awsclient.RDSDescribeDBInstancesAPI.
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
type fakeRDSDescribeDBInstances struct {
	Err      error
	Output   *rds.DescribeDBInstancesOutput
	Pages    []*rds.DescribeDBInstancesOutput
	PageFunc func(call int) (*rds.DescribeDBInstancesOutput, error)

	Calls     int
	Inputs    []*rds.DescribeDBInstancesInput
	LastInput *rds.DescribeDBInstancesInput
}

func (f *fakeRDSDescribeDBInstances) DescribeDBInstances(
	_ context.Context,
	params *rds.DescribeDBInstancesInput,
	_ ...func(*rds.Options),
) (*rds.DescribeDBInstancesOutput, error) {
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
			return &rds.DescribeDBInstancesOutput{}, nil
		}
		f.Calls++
		return f.Pages[idx], nil
	}
	if f.Output != nil {
		return f.Output, nil
	}
	return &rds.DescribeDBInstancesOutput{}, nil
}

// fakeRDSDescribeDBSnapshots implements awsclient.RDSDescribeDBSnapshotsAPI.
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
type fakeRDSDescribeDBSnapshots struct {
	Err      error
	Output   *rds.DescribeDBSnapshotsOutput
	Pages    []*rds.DescribeDBSnapshotsOutput
	PageFunc func(call int) (*rds.DescribeDBSnapshotsOutput, error)

	Calls     int
	Inputs    []*rds.DescribeDBSnapshotsInput
	LastInput *rds.DescribeDBSnapshotsInput
}

func (f *fakeRDSDescribeDBSnapshots) DescribeDBSnapshots(
	_ context.Context,
	params *rds.DescribeDBSnapshotsInput,
	_ ...func(*rds.Options),
) (*rds.DescribeDBSnapshotsOutput, error) {
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
			return &rds.DescribeDBSnapshotsOutput{}, nil
		}
		f.Calls++
		return f.Pages[idx], nil
	}
	if f.Output != nil {
		return f.Output, nil
	}
	return &rds.DescribeDBSnapshotsOutput{}, nil
}
