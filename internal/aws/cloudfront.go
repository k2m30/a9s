package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/cloudfront"

	"github.com/k2m30/a9s/internal/resource"
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
		return nil, err
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

		comment := ""
		if dist.Comment != nil {
			comment = *dist.Comment
		}

		arn := ""
		if dist.ARN != nil {
			arn = *dist.ARN
		}

		// Extract aliases
		aliases := ""
		if dist.Aliases != nil && len(dist.Aliases.Items) > 0 {
			aliases = strings.Join(dist.Aliases.Items, ", ")
		}

		priceClass := string(dist.PriceClass)
		httpVersion := string(dist.HttpVersion)

		lastModified := ""
		if dist.LastModifiedTime != nil {
			lastModified = dist.LastModifiedTime.Format("2006-01-02 15:04:05")
		}

		// Build DetailData
		detail := map[string]string{
			"Distribution ID": distID,
			"Domain Name":     domainName,
			"Status":          status,
			"Enabled":         enabled,
			"Comment":         comment,
			"ARN":             arn,
			"Aliases":         aliases,
			"Price Class":     priceClass,
			"HTTP Version":    httpVersion,
			"Last Modified":   lastModified,
		}

		// Build RawJSON
		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(dist, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

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
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  dist,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
