// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudfront"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchCloudFrontDistributionsPage fetches a single page of CloudFront distributions.
func FetchCloudFrontDistributionsPage(ctx context.Context, api CloudFrontListDistributionsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &cloudfront.ListDistributionsInput{
		MaxItems: aws.Int32(DefaultPageSize),
	}
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

			distARN := ""
			if dist.ARN != nil {
				distARN = *dist.ARN
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

			aliases := ""
			if dist.Aliases != nil && len(dist.Aliases.Items) > 0 {
				aliases = strings.Join(dist.Aliases.Items, ", ")
			}

			priceClass := string(dist.PriceClass)

			// lambda_function_arns — required for the lambda:cf related-panel
			// pivot (checkLambdaCF). DistributionSummary already carries both
			// cache-behavior lists in the ListDistributions response, so this
			// is a zero-extra-call join (no need for GetDistributionConfig).
			var lambdaARNs []string
			collectLambdaARNs := func(lfa *cftypes.LambdaFunctionAssociations) {
				if lfa == nil {
					return
				}
				for _, item := range lfa.Items {
					if item.LambdaFunctionARN != nil && *item.LambdaFunctionARN != "" {
						lambdaARNs = append(lambdaARNs, *item.LambdaFunctionARN)
					}
				}
			}
			if dist.DefaultCacheBehavior != nil {
				collectLambdaARNs(dist.DefaultCacheBehavior.LambdaFunctionAssociations)
			}
			if dist.CacheBehaviors != nil {
				for _, cb := range dist.CacheBehaviors.Items {
					collectLambdaARNs(cb.LambdaFunctionAssociations)
				}
			}

			r := withWave1(resource.Resource{
				ID:   distID,
				Name: distID,
				Fields: map[string]string{
					"distribution_id":      distID,
					"arn":                  distARN,
					"domain_name":          domainName,
					"status":               status,
					"enabled":              enabled,
					"aliases":              aliases,
					"price_class":          priceClass,
					"lambda_function_arns": strings.Join(lambdaARNs, ","),
				},
				RawStruct: dist,
			}, cfWave1Findings(enabled, status)...)

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

// cfWave1Findings is the distribution's deployment status, then whether it
// is switched off: Enabled is a second axis beside Status.
func cfWave1Findings(enabled, status string) []domain.Finding {
	var posture []domain.Finding
	if enabled == "false" {
		posture = append(posture, wave1Finding(cfCodeDisabled))
	}
	return withState("cf", status, posture)
}

// cfStatuses is DistributionSummary.Status as the CloudFront API reference
// documents it: "When the status is Deployed, the distribution's information
// is propagated to all CloudFront edge locations", and InProgress before it.
var cfStatuses = []string{"Deployed", "InProgress"} //nolint:gochecknoglobals // static AWS status domain
