package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/opensearch"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("opensearch", []string{"domain_name", "engine_version", "instance_type", "instance_count", "endpoint"})
	resource.Register("opensearch", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchOpenSearchDomains(ctx, c.OpenSearch, c.OpenSearch)
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
		return nil, err
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
		return nil, err
	}

	var resources []resource.Resource

	for _, domain := range descOutput.DomainStatusList {
		domainName := ""
		if domain.DomainName != nil {
			domainName = *domain.DomainName
		}

		domainID := ""
		if domain.DomainId != nil {
			domainID = *domain.DomainId
		}

		arn := ""
		if domain.ARN != nil {
			arn = *domain.ARN
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

		// Build DetailData
		detail := map[string]string{
			"Domain Name":    domainName,
			"Domain ID":      domainID,
			"ARN":            arn,
			"Engine Version": engineVersion,
			"Instance Type":  instanceType,
			"Instance Count": instanceCount,
			"Endpoint":       endpoint,
		}

		// Build RawJSON
		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(domain, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

		r := resource.Resource{
			ID:     domainName,
			Name:   domainName,
			Status: "",
			Fields: map[string]string{
				"domain_name":    domainName,
				"engine_version": engineVersion,
				"instance_type":  instanceType,
				"instance_count": instanceCount,
				"endpoint":       endpoint,
			},
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  domain,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
