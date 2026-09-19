// fakes_acm_test.go is the single source for ACM SDK-interface fakes shared
// across tests/unit.
//
// Convention (docs/go-codebase-checklist.md, DRY section): one configurable
// fake per SDK interface, in a service-named fakes_<service>_test.go file —
// never re-implement the same interface under a new name in another file, and
// never add a wave/batch-named fake file.
package unit

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/acm"
)

// fakeACMListCertificates implements awsclient.ACMListCertificatesAPI.
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
type fakeACMListCertificates struct {
	Err      error
	Output   *acm.ListCertificatesOutput
	Pages    []*acm.ListCertificatesOutput
	PageFunc func(call int) (*acm.ListCertificatesOutput, error)

	Calls     int
	Inputs    []*acm.ListCertificatesInput
	LastInput *acm.ListCertificatesInput
}

func (f *fakeACMListCertificates) ListCertificates(
	_ context.Context,
	params *acm.ListCertificatesInput,
	_ ...func(*acm.Options),
) (*acm.ListCertificatesOutput, error) {
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
			return &acm.ListCertificatesOutput{}, nil
		}
		f.Calls++
		return f.Pages[idx], nil
	}
	if f.Output != nil {
		return f.Output, nil
	}
	return &acm.ListCertificatesOutput{}, nil
}
