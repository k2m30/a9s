package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/opensearch"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("opensearch", []string{"domain_name", "engine_version", "instance_type", "instance_count", "endpoint", "status", "domain_processing_status"})

	resource.RegisterPaginated("opensearch", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		resources, err := FetchOpenSearchDomains(ctx, c.OpenSearch, c.OpenSearch)
		if err != nil {
			return resource.FetchResult{}, err
		}
		return resource.FetchResult{
			Resources:  resources,
			Pagination: &resource.PaginationMeta{IsTruncated: false, TotalHint: len(resources), PageSize: len(resources)},
		}, nil
	})
}

// FetchOpenSearchDomains performs a two-step fetch:
// 1. ListDomainNames to get domain names
// 2. DescribeDomains to get full domain status details
func FetchOpenSearchDomains(
	ctx context.Context,
	listAPI OpenSearchListDomainNamesAPI,
	describeAPI OpenSearchDescribeDomainsAPI,
) ([]resource.Resource, error) {
	listOutput, err := listAPI.ListDomainNames(ctx, &opensearch.ListDomainNamesInput{})
	if err != nil {
		return nil, fmt.Errorf("listing OpenSearch domains: %w", err)
	}

	if len(listOutput.DomainNames) == 0 {
		return []resource.Resource{}, nil
	}

	// Collect domain names for the DescribeDomains call
	domainNames := make([]string, 0, len(listOutput.DomainNames))
	for _, d := range listOutput.DomainNames {
		if d.DomainName != nil {
			domainNames = append(domainNames, *d.DomainName)
		}
	}

	descOutput, err := describeAPI.DescribeDomains(ctx, &opensearch.DescribeDomainsInput{
		DomainNames: domainNames,
	})
	if err != nil {
		return nil, fmt.Errorf("describing OpenSearch domains: %w", err)
	}

	var resources []resource.Resource

	for _, domain := range descOutput.DomainStatusList {
		domainName := ""
		if domain.DomainName != nil {
			domainName = *domain.DomainName
		}

		engineVersion := ""
		if domain.EngineVersion != nil {
			engineVersion = *domain.EngineVersion
		}

		endpoint := ""
		if domain.Endpoint != nil {
			endpoint = *domain.Endpoint
		}

		instanceType := ""
		instanceCount := ""
		if domain.ClusterConfig != nil {
			instanceType = string(domain.ClusterConfig.InstanceType)
			if domain.ClusterConfig.InstanceCount != nil {
				instanceCount = fmt.Sprintf("%d", *domain.ClusterConfig.InstanceCount)
			}
		}

		processing := "false"
		if domain.Processing != nil && *domain.Processing {
			processing = "true"
		}
		upgradeProcessing := "false"
		if domain.UpgradeProcessing != nil && *domain.UpgradeProcessing {
			upgradeProcessing = "true"
		}
		deleted := "false"
		if domain.Deleted != nil && *domain.Deleted {
			deleted = "true"
		}
		processingStatus := ""
		if domain.DomainProcessingStatus != "" {
			processingStatus = string(domain.DomainProcessingStatus)
		}
		status := "available"
		if deleted == "true" {
			status = "deleted"
		} else if processing == "true" || upgradeProcessing == "true" {
			status = "processing"
		}

		r := resource.Resource{
			ID:     domainName,
			Name:   domainName,
			Status: status,
			Fields: map[string]string{
				"domain_name":              domainName,
				"engine_version":           engineVersion,
				"instance_type":            instanceType,
				"instance_count":           instanceCount,
				"endpoint":                 endpoint,
				"status":                   status,
				"processing":               processing,
				"upgrade_processing":       upgradeProcessing,
				"deleted":                  deleted,
				"domain_processing_status": processingStatus,
			},
			RawStruct: domain,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
