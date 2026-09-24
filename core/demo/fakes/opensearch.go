// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package fakes

import (
	"context"
	"slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"

	"github.com/k2m30/a9s/v3/core/demo/fixtures"
)

// OpenSearchFake implements aws.OpenSearchAPI against fixture data loaded at construction time.
type OpenSearchFake struct {
	fix *fixtures.OpenSearchFixtures
}

// NewOpenSearch constructs an OpenSearchFake backed by fixture data from the fixtures package.
func NewOpenSearch() *OpenSearchFake {
	return &OpenSearchFake{fix: fixtures.NewOpenSearchFixtures()}
}

func (f *OpenSearchFake) ListDomainNames(_ context.Context, _ *opensearch.ListDomainNamesInput, _ ...func(*opensearch.Options)) (*opensearch.ListDomainNamesOutput, error) {
	domainNames := make([]ostypes.DomainInfo, 0, len(f.fix.Domains)+len(f.fix.UnavailableNames))
	for i := range f.fix.Domains {
		d := &f.fix.Domains[i]
		domainNames = append(domainNames, ostypes.DomainInfo{
			DomainName: d.DomainName,
			EngineType: ostypes.EngineTypeOpenSearch,
		})
	}
	// Unavailable domains are listed but absent from DescribeDomains — the
	// batched API's absent-from-response shape.
	for i := range f.fix.UnavailableNames {
		domainNames = append(domainNames, ostypes.DomainInfo{
			DomainName: &f.fix.UnavailableNames[i],
			EngineType: ostypes.EngineTypeOpenSearch,
		})
	}
	return &opensearch.ListDomainNamesOutput{DomainNames: domainNames}, nil
}

// DescribeDomains answers only the named domains and, as AWS does, rejects
// more than five names.
func (f *OpenSearchFake) DescribeDomains(_ context.Context, in *opensearch.DescribeDomainsInput, _ ...func(*opensearch.Options)) (*opensearch.DescribeDomainsOutput, error) {
	if len(in.DomainNames) > 5 {
		return nil, &ostypes.ValidationException{Message: aws.String("1 validation error detected: Value at 'domainNames' failed to satisfy constraint: Member must have length less than or equal to 5")}
	}
	var domains []ostypes.DomainStatus
	for _, d := range f.fix.Domains {
		if slices.Contains(in.DomainNames, aws.ToString(d.DomainName)) {
			domains = append(domains, d)
		}
	}
	return &opensearch.DescribeDomainsOutput{DomainStatusList: domains}, nil
}

// ListTags returns demo tags for the given domain ARN.
// For acme-logs it returns the aws:cloudformation:stack-name tag
// so that checkOpenSearchCFN resolves the acme-search-stack CFN stack.
func (f *OpenSearchFake) ListTags(_ context.Context, in *opensearch.ListTagsInput, _ ...func(*opensearch.Options)) (*opensearch.ListTagsOutput, error) {
	if in == nil || in.ARN == nil {
		return &opensearch.ListTagsOutput{}, nil
	}
	if err := validateARN(*in.ARN); err != nil {
		return nil, err
	}
	if *in.ARN == fixtures.GraphRootDomainARN {
		return &opensearch.ListTagsOutput{
			TagList: []ostypes.Tag{
				{
					Key:   aws.String("aws:cloudformation:stack-name"),
					Value: aws.String(fixtures.OpenSearchCFNStackName),
				},
				{
					Key:   aws.String("Environment"),
					Value: aws.String("production"),
				},
			},
		}, nil
	}
	return &opensearch.ListTagsOutput{}, nil
}

// DescribeDomainConfig returns the named domain's endpoint options as its
// fixture DomainStatus carries them.
func (f *OpenSearchFake) DescribeDomainConfig(_ context.Context, in *opensearch.DescribeDomainConfigInput, _ ...func(*opensearch.Options)) (*opensearch.DescribeDomainConfigOutput, error) {
	name := aws.ToString(in.DomainName)
	i := slices.IndexFunc(f.fix.Domains, func(d ostypes.DomainStatus) bool { return aws.ToString(d.DomainName) == name })
	if i < 0 {
		return nil, &ostypes.ResourceNotFoundException{Message: notFoundMessage("Domain", name)}
	}
	config := &ostypes.DomainConfig{}
	if opts := f.fix.Domains[i].DomainEndpointOptions; opts != nil {
		config.DomainEndpointOptions = &ostypes.DomainEndpointOptionsStatus{Options: opts}
	}
	return &opensearch.DescribeDomainConfigOutput{DomainConfig: config}, nil
}
