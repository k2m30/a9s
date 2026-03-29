package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("trail", []string{"trail_name", "s3_bucket", "home_region", "multi_region"})

	resource.RegisterPaginated("trail", func(ctx context.Context, clients interface{}, continuationToken string) (resource.FetchResult, error) {
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

// FetchCloudTrailTrails calls the CloudTrail DescribeTrails API and converts
// the response into a slice of generic Resource structs.
// Uses DescribeTrails only (no GetTrailStatus).
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

		r := resource.Resource{
			ID:     trailName,
			Name:   trailName,
			Status: "",
			Fields: map[string]string{
				"trail_name":     trailName,
				"trail_arn":      trailARN,
				"s3_bucket":      s3Bucket,
				"home_region":    homeRegion,
				"multi_region":   multiRegion,
				"org_trail":      orgTrail,
				"log_validation": logValidation,
			},
			RawStruct: trail,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
