// fakes_cloudfront_pkgunit_test.go is the single source for CloudFront
// ListDistributions SDK-interface fakes shared across tests/unit (package
// unit). fakes_cloudfront_test.go (package unit_test) hosts the wider
// fakeCloudFrontAPI scenario fixture — it cannot serve the unexported
// package-unit call sites this file covers, hence the two-file split.
//
// Convention (docs/go-codebase-checklist.md, DRY section): one configurable
// fake per SDK interface, in a service-named fakes_<service>_test.go file —
// never re-implement the same interface under a new name in another file, and
// never add a wave/batch-named fake file.
package unit

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
)

// fakeCloudFrontListDistributions implements
// awsclient.CloudFrontListDistributionsAPI.
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
type fakeCloudFrontListDistributions struct {
	Err      error
	Output   *cloudfront.ListDistributionsOutput
	Pages    []*cloudfront.ListDistributionsOutput
	PageFunc func(call int) (*cloudfront.ListDistributionsOutput, error)

	Calls     int
	Inputs    []*cloudfront.ListDistributionsInput
	LastInput *cloudfront.ListDistributionsInput
}

func (f *fakeCloudFrontListDistributions) ListDistributions(
	_ context.Context,
	params *cloudfront.ListDistributionsInput,
	_ ...func(*cloudfront.Options),
) (*cloudfront.ListDistributionsOutput, error) {
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
			return &cloudfront.ListDistributionsOutput{}, nil
		}
		f.Calls++
		return f.Pages[idx], nil
	}
	if f.Output != nil {
		return f.Output, nil
	}
	return &cloudfront.ListDistributionsOutput{}, nil
}
