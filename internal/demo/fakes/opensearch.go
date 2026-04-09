package fakes

import (
	"context"

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
