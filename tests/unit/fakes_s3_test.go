// fakes_s3_test.go is the single source for S3 SDK-interface fakes shared
// across tests/unit.
//
// Convention (docs/go-codebase-checklist.md, DRY section): one configurable
// fake per SDK interface, in a service-named fakes_<service>_test.go file —
// never re-implement the same interface under a new name in another file, and
// never add a wave/batch-named fake file.
package unit

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// fakeS3ListObjectsV2 implements awsclient.S3ListObjectsV2API.
//
// Configure at most one response mode:
//   - Err: always return this error
//   - Output: always return this single output (no pagination)
//   - Pages: sequential paged outputs, one per call, in order
//   - PageFunc: full control, receives the 1-based call number
//
// Calls is always incremented on every invocation, regardless of mode.
type fakeS3ListObjectsV2 struct {
	Err      error
	Output   *s3.ListObjectsV2Output
	Pages    []*s3.ListObjectsV2Output
	PageFunc func(call int) (*s3.ListObjectsV2Output, error)

	Calls int
}

func (f *fakeS3ListObjectsV2) ListObjectsV2(
	_ context.Context,
	_ *s3.ListObjectsV2Input,
	_ ...func(*s3.Options),
) (*s3.ListObjectsV2Output, error) {
	if f.PageFunc != nil {
		f.Calls++
		return f.PageFunc(f.Calls)
	}
	if len(f.Pages) > 0 {
		idx := f.Calls
		if idx >= len(f.Pages) {
			return &s3.ListObjectsV2Output{}, nil
		}
		f.Calls++
		return f.Pages[idx], nil
	}
	if f.Err != nil {
		return nil, f.Err
	}
	if f.Output != nil {
		return f.Output, nil
	}
	return &s3.ListObjectsV2Output{}, nil
}

// fakeS3ListBuckets implements awsclient.S3ListBucketsAPI.
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
type fakeS3ListBuckets struct {
	Err      error
	Output   *s3.ListBucketsOutput
	Pages    []*s3.ListBucketsOutput
	PageFunc func(call int) (*s3.ListBucketsOutput, error)

	Calls     int
	Inputs    []*s3.ListBucketsInput
	LastInput *s3.ListBucketsInput
}

func (f *fakeS3ListBuckets) ListBuckets(
	_ context.Context,
	params *s3.ListBucketsInput,
	_ ...func(*s3.Options),
) (*s3.ListBucketsOutput, error) {
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
			return &s3.ListBucketsOutput{}, nil
		}
		f.Calls++
		return f.Pages[idx], nil
	}
	if f.Output != nil {
		return f.Output, nil
	}
	return &s3.ListBucketsOutput{}, nil
}
