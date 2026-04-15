package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("trail", []string{"trail_name", "s3_bucket", "home_region", "multi_region"})

	resource.RegisterPaginated("trail", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		resources, err := FetchCloudTrailTrails(ctx, c.CloudTrail)
		if err != nil {
			return resource.FetchResult{}, err
		}
		return resource.FetchResult{
			Resources:  resources,
			Pagination: &resource.PaginationMeta{IsTruncated: false, TotalHint: len(resources), PageSize: len(resources)},
		}, nil
	})
}

// FetchCloudTrailTrails calls DescribeTrails and GetTrailStatus (per trail)
// so the list row can classify `is_logging=false` / `latest_delivery_error` as
// broken. GetTrailStatus is the authoritative source for logging health; the
// DescribeTrails response alone has no runtime signal.
func FetchCloudTrailTrails(ctx context.Context, api CloudTrailDescribeTrailsAPI) ([]resource.Resource, error) {
	output, err := api.DescribeTrails(ctx, &cloudtrail.DescribeTrailsInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching CloudTrail trails: %w", err)
	}

	var resources []resource.Resource

	for _, trail := range output.TrailList {
		trailName := ""
		if trail.Name != nil {
			trailName = *trail.Name
		}

		trailARN := ""
		if trail.TrailARN != nil {
			trailARN = *trail.TrailARN
		}

		s3Bucket := ""
		if trail.S3BucketName != nil {
			s3Bucket = *trail.S3BucketName
		}

		homeRegion := ""
		if trail.HomeRegion != nil {
			homeRegion = *trail.HomeRegion
		}

		multiRegion := "false"
		if trail.IsMultiRegionTrail != nil && *trail.IsMultiRegionTrail {
			multiRegion = "true"
		}

		orgTrail := "false"
		if trail.IsOrganizationTrail != nil && *trail.IsOrganizationTrail {
			orgTrail = "true"
		}

		logValidation := "false"
		if trail.LogFileValidationEnabled != nil && *trail.LogFileValidationEnabled {
			logValidation = "true"
		}

		// Per-trail GetTrailStatus for runtime logging health. Failures here
		// degrade the row gracefully (unknown status) rather than aborting
		// the whole list.
		isLogging := ""
		latestDeliveryError := ""
		if trailARN != "" {
			statusOut, statusErr := api.GetTrailStatus(ctx, &cloudtrail.GetTrailStatusInput{Name: &trailARN})
			if statusErr == nil && statusOut != nil {
				if statusOut.IsLogging != nil {
					if *statusOut.IsLogging {
						isLogging = "true"
					} else {
						isLogging = "false"
					}
				}
				if statusOut.LatestDeliveryError != nil {
					latestDeliveryError = *statusOut.LatestDeliveryError
				}
			}
		}

		r := resource.Resource{
			ID:     trailName,
			Name:   trailName,
			Status: "",
			Fields: map[string]string{
				"trail_name":            trailName,
				"trail_arn":             trailARN,
				"s3_bucket":             s3Bucket,
				"home_region":           homeRegion,
				"multi_region":          multiRegion,
				"org_trail":             orgTrail,
				"log_validation":        logValidation,
				"is_logging":            isLogging,
				"latest_delivery_error": latestDeliveryError,
			},
			RawStruct: trail,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
