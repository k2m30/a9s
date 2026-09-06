// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"

	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchCloudTrailTrails calls DescribeTrails and GetTrailStatus (per trail)
// so the list row can classify `is_logging=false`, non-empty
// `latest_delivery_error`, or a stale `latest_delivery_time` (>1h on a
// logging trail) as broken. GetTrailStatus is the authoritative source for
// logging health; the DescribeTrails response alone has no runtime signal.
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
		latestDeliveryTime := ""
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
				if statusOut.LatestDeliveryTime != nil {
					latestDeliveryTime = statusOut.LatestDeliveryTime.Format(time.RFC3339)
				}
			}
		}

		r := resource.Resource{
			ID:   trailName,
			Name: trailName,
			Fields: map[string]string{
				"trail_name":                  trailName,
				"trail_arn":                   trailARN,
				"s3_bucket":                   s3Bucket,
				"home_region":                 homeRegion,
				"multi_region":                multiRegion,
				"org_trail":                   orgTrail,
				"log_file_validation_enabled": logValidation,
				"is_logging":                  isLogging,
				"latest_delivery_error":       latestDeliveryError,
				"latest_delivery_time":        latestDeliveryTime,
			},
			Findings:  trailWave1Wave2Findings(isLogging, latestDeliveryError, latestDeliveryTime, logValidation),
			RawStruct: trail,
		}
		trailPostureFindings(&r, trail)

		resources = append(resources, r)
	}

	return resources, nil
}
