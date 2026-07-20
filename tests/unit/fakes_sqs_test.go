// fakes_sqs_test.go is the single source for SQS SDK-interface fakes shared
// across tests/unit (package unit).
//
// fakeSQSListQueuesOnly and fakeSQSGetQueueAttributesWithKMS stay in
// aws_related_checker_round2_test.go — they live in package unit_test, which
// cannot reach these unexported package-unit fakes.
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

	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// fakeSQSListQueues implements awsclient.SQSListQueuesAPI.
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
type fakeSQSListQueues struct {
	Err      error
	Output   *sqs.ListQueuesOutput
	Pages    []*sqs.ListQueuesOutput
	PageFunc func(call int) (*sqs.ListQueuesOutput, error)

	Calls     int
	Inputs    []*sqs.ListQueuesInput
	LastInput *sqs.ListQueuesInput
}

func (f *fakeSQSListQueues) ListQueues(
	_ context.Context,
	params *sqs.ListQueuesInput,
	_ ...func(*sqs.Options),
) (*sqs.ListQueuesOutput, error) {
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
			return &sqs.ListQueuesOutput{}, nil
		}
		f.Calls++
		return f.Pages[idx], nil
	}
	if f.Output != nil {
		return f.Output, nil
	}
	return &sqs.ListQueuesOutput{}, nil
}

// fakeSQSGetQueueAttributes implements awsclient.SQSGetQueueAttributesAPI.
//
// Configure at most one response mode:
//   - Err: always return this error for every call
//   - ByURL: keyed by the request's QueueUrl, falling back to an empty
//     Attributes map for an unrecognized URL
//   - Func: full control, receives the 1-based call number and the
//     request's QueueUrl, takes precedence over ByURL/Err when set
//
// Calls is always recorded.
type fakeSQSGetQueueAttributes struct {
	Err   error
	ByURL map[string]*sqs.GetQueueAttributesOutput
	Func  func(call int, queueURL string) (*sqs.GetQueueAttributesOutput, error)

	Calls int
}

func (f *fakeSQSGetQueueAttributes) GetQueueAttributes(
	_ context.Context,
	params *sqs.GetQueueAttributesInput,
	_ ...func(*sqs.Options),
) (*sqs.GetQueueAttributesOutput, error) {
	f.Calls++
	url := ""
	if params != nil && params.QueueUrl != nil {
		url = *params.QueueUrl
	}

	if f.Func != nil {
		return f.Func(f.Calls, url)
	}
	if f.Err != nil {
		return nil, f.Err
	}
	if out, ok := f.ByURL[url]; ok {
		return out, nil
	}
	return &sqs.GetQueueAttributesOutput{Attributes: map[string]string{}}, nil
}
