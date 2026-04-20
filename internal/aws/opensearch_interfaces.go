package aws

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/service/opensearch"
)

// OpenSearchListDomainNamesAPI defines the interface for the OpenSearch ListDomainNames operation.
type OpenSearchListDomainNamesAPI interface {
	ListDomainNames(ctx context.Context, params *opensearch.ListDomainNamesInput, optFns ...func(*opensearch.Options)) (*opensearch.ListDomainNamesOutput, error)
}

// OpenSearchDescribeDomainsAPI defines the interface for the OpenSearch DescribeDomains operation.
type OpenSearchDescribeDomainsAPI interface {
	DescribeDomains(ctx context.Context, params *opensearch.DescribeDomainsInput, optFns ...func(*opensearch.Options)) (*opensearch.DescribeDomainsOutput, error)
}

// OpenSearchDescribeDomainConfigAPI defines the interface for DescribeDomainConfig.
type OpenSearchDescribeDomainConfigAPI interface {
	DescribeDomainConfig(ctx context.Context, params *opensearch.DescribeDomainConfigInput, optFns ...func(*opensearch.Options)) (*opensearch.DescribeDomainConfigOutput, error)
}

// OpenSearchListTagsAPI defines the interface for the OpenSearch ListTags operation.
type OpenSearchListTagsAPI interface {
	ListTags(ctx context.Context, params *opensearch.ListTagsInput, optFns ...func(*opensearch.Options)) (*opensearch.ListTagsOutput, error)
}

// OpenSearchAPI is the aggregate interface covering all OpenSearch operations used by a9s fetchers.
// *opensearch.Client structurally satisfies this interface.
type OpenSearchAPI interface {
	OpenSearchListDomainNamesAPI
	OpenSearchDescribeDomainsAPI
}
