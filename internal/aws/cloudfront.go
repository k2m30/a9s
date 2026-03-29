package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/cloudfront"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("cf", []string{"distribution_id", "domain_name", "status", "enabled", "aliases", "price_class"})

	resource.RegisterPaginated("cf", func(ctx context.Context, clients interface{}, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchCloudFrontDistributionsPage(ctx, c.CloudFront, continuationToken)
	})
}

// FetchCloudFrontDistributions calls the CloudFront ListDistributions API and converts
// the response into a slice of generic Resource structs.
func FetchCloudFrontDistributions(ctx context.Context, api CloudFrontListDistributionsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchCloudFrontDistributionsPage(ctx, api, token)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Resources...)
		if result.Pagination == nil || !result.Pagination.IsTruncated {
			break
		}
		token = result.Pagination.NextToken
	}
	return all, nil
}

// FetchCloudFrontDistributionsPage fetches a single page of CloudFront distributions.
func FetchCloudFrontDistributionsPage(ctx context.Context, api CloudFrontListDistributionsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &cloudfront.ListDistributionsInput{}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := api.ListDistributions(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching CloudFront distributions: %w", err)
	}

	var resources []resource.Resource

	if output.DistributionList != nil {
		for _, dist := range output.DistributionList.Items {
			distID := ""
			if dist.Id != nil {
				distID = *dist.Id
			}

			domainName := ""
			if dist.DomainName != nil {
				domainName = *dist.DomainName
			}

			status := ""
			if dist.Status != nil {
				status = *dist.Status
			}

			enabled := "false"
			if dist.Enabled != nil && *dist.Enabled {
				enabled = "true"
			}

			// Extract aliases
			aliases := ""
			if dist.Aliases != nil && len(dist.Aliases.Items) > 0 {
				aliases = strings.Join(dist.Aliases.Items, ", ")
			}

			priceClass := string(dist.PriceClass)

			colorStatus := status
			if dist.Enabled != nil && !*dist.Enabled {
				colorStatus = "Disabled"
			}

			r := resource.Resource{
				ID:     distID,
				Name:   distID,
				Status: colorStatus,
				Fields: map[string]string{
					"distribution_id": distID,
					"domain_name":     domainName,
					"status":          status,
					"enabled":         enabled,
					"aliases":         aliases,
					"price_class":     priceClass,
				},
				RawStruct: dist,
			}

			resources = append(resources, r)
		}
	}

	nextToken := ""
	isTruncated := false
	if output.DistributionList != nil && output.DistributionList.IsTruncated != nil && *output.DistributionList.IsTruncated {
		isTruncated = true
		if output.DistributionList.NextMarker != nil {
			nextToken = *output.DistributionList.NextMarker
		}
	}

	totalHint := len(resources)
	if isTruncated {
		totalHint = -1
	}

	return resource.FetchResult{
		Resources: resources,
		Pagination: &resource.PaginationMeta{
			IsTruncated: isTruncated,
			NextToken:   nextToken,
			PageSize:    len(resources),
			TotalHint:   totalHint,
		},
	}, nil
}
