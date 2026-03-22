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
	resource.Register("cf", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchCloudFrontDistributions(ctx, c.CloudFront)
	})
}

// FetchCloudFrontDistributions calls the CloudFront ListDistributions API and converts
// the response into a slice of generic Resource structs.
func FetchCloudFrontDistributions(ctx context.Context, api CloudFrontListDistributionsAPI) ([]resource.Resource, error) {
	output, err := api.ListDistributions(ctx, &cloudfront.ListDistributionsInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching CloudFront distributions: %w", err)
	}

	if output.DistributionList == nil {
		return nil, nil
	}

	var resources []resource.Resource

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

		r := resource.Resource{
			ID:     distID,
			Name:   distID,
			Status: status,
			Fields: map[string]string{
				"distribution_id": distID,
				"domain_name":     domainName,
				"status":          status,
				"enabled":         enabled,
				"aliases":         aliases,
				"price_class":     priceClass,
			},
			RawStruct:  dist,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
