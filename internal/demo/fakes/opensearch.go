package fakes

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/opensearch"
	ostypes "github.com/aws/aws-sdk-go-v2/service/opensearch/types"

	"github.com/k2m30/a9s/v3/internal/demo/fixtures"
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
	domainNames := make([]ostypes.DomainInfo, 0, len(f.fix.Domains))
	for i := range f.fix.Domains {
		d := &f.fix.Domains[i]
		domainNames = append(domainNames, ostypes.DomainInfo{
			DomainName: d.DomainName,
			EngineType: ostypes.EngineTypeOpenSearch,
		})
	}
	return &opensearch.ListDomainNamesOutput{DomainNames: domainNames}, nil
}

func (f *OpenSearchFake) DescribeDomains(_ context.Context, _ *opensearch.DescribeDomainsInput, _ ...func(*opensearch.Options)) (*opensearch.DescribeDomainsOutput, error) {
	domains := make([]ostypes.DomainStatus, len(f.fix.Domains))
	copy(domains, f.fix.Domains)
	return &opensearch.DescribeDomainsOutput{DomainStatusList: domains}, nil
}

// ListTags returns demo tags for the given domain ARN.
// For the graph-root domain (acme-logs) it returns the aws:cloudformation:stack-name tag
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

// DescribeDomainConfig returns demo domain config for the given domain name.
// For the graph-root domain (acme-logs) it returns a CustomEndpointCertificateArn
// so that checkOpenSearchACM resolves the acme-logs.internal.com ACM certificate.
func (f *OpenSearchFake) DescribeDomainConfig(_ context.Context, in *opensearch.DescribeDomainConfigInput, _ ...func(*opensearch.Options)) (*opensearch.DescribeDomainConfigOutput, error) {
	if in == nil || in.DomainName == nil {
		return &opensearch.DescribeDomainConfigOutput{
			DomainConfig: &ostypes.DomainConfig{},
		}, nil
	}
	if *in.DomainName == fixtures.GraphRootDomain {
		return &opensearch.DescribeDomainConfigOutput{
			DomainConfig: &ostypes.DomainConfig{
				DomainEndpointOptions: &ostypes.DomainEndpointOptionsStatus{
					Options: &ostypes.DomainEndpointOptions{
						EnforceHTTPS:                 aws.Bool(true),
						CustomEndpointEnabled:         aws.Bool(true),
						CustomEndpoint:                aws.String("acme-logs.internal.com"),
						CustomEndpointCertificateArn:  aws.String(fixtures.OpenSearchACMCertARN),
					},
				},
			},
		}, nil
	}
	return &opensearch.DescribeDomainConfigOutput{
		DomainConfig: &ostypes.DomainConfig{},
	}, nil
}
