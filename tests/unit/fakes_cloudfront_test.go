// fakes_cloudfront_test.go is the single source for CloudFront
// SDK-interface fakes shared across tests/unit (package unit_test).
//
// Convention (docs/go-codebase-checklist.md, DRY section): one configurable
// fake per SDK interface, in a service-named fakes_<service>_test.go file --
// never re-implement the same interface under a new name in another file,
// and never add another wave/batch-named fake file (fakes_us1_batchN_test.go,
// fakes_wave5_test.go, fakes_coverage_restore_test.go are historical
// accretion naming, not a pattern to extend).
package unit_test

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cloudfronttypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"
)

// fakeCloudFrontAPI implements awsclient.CloudFrontAPI (ListDistributions,
// GetDistributionConfig) plus awsclient.CloudFrontListDistributionsByWebACLIdAPI.
//
// ListDistributions: configure ListOutput/ListErr (unconditional).
//
// GetDistributionConfig: configure GetConfigOutput/GetConfigErr for an
// unconditional response.
//
// ListDistributionsByWebACLId: configure at most one mode —
//   - WebACLErr/WebACLOutput: unconditional error or output, ignoring the
//     request's WebACLId entirely
//   - WantWebACLArn: reject any request whose WebACLId doesn't match this
//     exact value with an empty distribution list — for regression tests
//     that must prove a checker forwards the full ARN, not a bare ID
//   - DistByWebACLID: keyed by the request's WebACLId, returning matching
//     distribution IDs
type fakeCloudFrontAPI struct {
	ListOutput *cloudfront.ListDistributionsOutput
	ListErr    error

	GetConfigOutput *cloudfront.GetDistributionConfigOutput
	GetConfigErr    error

	WebACLErr      error
	WebACLOutput   *cloudfront.ListDistributionsByWebACLIdOutput
	WantWebACLArn  string
	DistByWebACLID map[string][]string
}

func (f *fakeCloudFrontAPI) ListDistributions(
	_ context.Context, _ *cloudfront.ListDistributionsInput, _ ...func(*cloudfront.Options),
) (*cloudfront.ListDistributionsOutput, error) {
	if f.ListErr != nil {
		return nil, f.ListErr
	}
	if f.ListOutput != nil {
		return f.ListOutput, nil
	}
	return &cloudfront.ListDistributionsOutput{}, nil
}

func (f *fakeCloudFrontAPI) GetDistributionConfig(
	_ context.Context, _ *cloudfront.GetDistributionConfigInput, _ ...func(*cloudfront.Options),
) (*cloudfront.GetDistributionConfigOutput, error) {
	if f.GetConfigErr != nil {
		return nil, f.GetConfigErr
	}
	if f.GetConfigOutput != nil {
		return f.GetConfigOutput, nil
	}
	return &cloudfront.GetDistributionConfigOutput{}, nil
}

func (f *fakeCloudFrontAPI) ListDistributionsByWebACLId(
	_ context.Context, params *cloudfront.ListDistributionsByWebACLIdInput, _ ...func(*cloudfront.Options),
) (*cloudfront.ListDistributionsByWebACLIdOutput, error) {
	if f.WebACLErr != nil {
		return nil, f.WebACLErr
	}
	if f.WebACLOutput != nil {
		return f.WebACLOutput, nil
	}
	if f.WantWebACLArn != "" {
		if params == nil || params.WebACLId == nil || *params.WebACLId != f.WantWebACLArn {
			return &cloudfront.ListDistributionsByWebACLIdOutput{DistributionList: &cloudfronttypes.DistributionList{}}, nil
		}
	} else if params == nil || params.WebACLId == nil {
		return &cloudfront.ListDistributionsByWebACLIdOutput{}, nil
	}

	webACLID := ""
	if params != nil && params.WebACLId != nil {
		webACLID = *params.WebACLId
	}
	ids := f.DistByWebACLID[webACLID]
	if len(ids) == 0 {
		return &cloudfront.ListDistributionsByWebACLIdOutput{DistributionList: &cloudfronttypes.DistributionList{}}, nil
	}
	items := make([]cloudfronttypes.DistributionSummary, 0, len(ids))
	for _, id := range ids {
		items = append(items, cloudfronttypes.DistributionSummary{Id: aws.String(id)})
	}
	return &cloudfront.ListDistributionsByWebACLIdOutput{DistributionList: &cloudfronttypes.DistributionList{Items: items}}, nil
}
