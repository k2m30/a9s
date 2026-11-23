// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"

	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchCloudTrailTrails calls DescribeTrails. The response carries no runtime
// signal; logging health comes from GetTrailStatus, which EnrichTrail reads
// per trail so a status that cannot be read marks the trail not inspected.
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

		r := withWave1("trail", resource.Resource{
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
			},
			RawStruct: trail,
		}, trailWave1Wave2Findings("", "", "", logValidation)...)
		trailPostureFindings(&r, trail)

		resources = append(resources, r)
	}

	return resources, nil
}
