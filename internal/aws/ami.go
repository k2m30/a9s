package aws

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ami", []string{"image_id", "name", "state", "architecture", "platform", "root_device_type", "creation_date", "public", "deprecated"})

	resource.RegisterPaginated("ami", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchAMIsPage(ctx, c.EC2, continuationToken)
	})

	resource.RegisterRelated("ami", []resource.RelatedDef{
		{TargetType: "ec2", DisplayName: "EC2 Instances", Checker: checkAMIEC2, NeedsTargetCache: true},
		{TargetType: "ebs-snap", DisplayName: "EBS Snapshots", Checker: checkAMIEBSSnaps, NeedsTargetCache: false},
		{TargetType: "asg", DisplayName: "Auto Scaling Groups", Checker: checkAMIASG, NeedsTargetCache: true},
		{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkAMICFN, NeedsTargetCache: true},
		{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkAMIKMS},
		{TargetType: "ng", DisplayName: "EKS Node Groups", Checker: checkAMING, NeedsTargetCache: true},
	})

	// ec2types.Image: BlockDeviceMappings[].Ebs.SnapshotId
	resource.RegisterNavigableFields("ami", []resource.NavigableField{
		{FieldPath: "BlockDeviceMappings.Ebs.SnapshotId", TargetType: "ebs-snap"},
	})
}

// FetchAMIs calls the EC2 DescribeImages API and returns all pages of AMIs.
// Used by tests; the production path uses the per-page fetcher for pagination.
func FetchAMIs(ctx context.Context, api EC2DescribeImagesAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchAMIsPage(ctx, api, token)
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

// FetchAMIsPage calls the EC2 DescribeImages API and returns a single page
// of AMIs. Only returns AMIs owned by the caller ("self").
// Pass an empty continuationToken for the first page.
func FetchAMIsPage(ctx context.Context, api EC2DescribeImagesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeImagesInput{
		Owners:     []string{"self"},
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeImages(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching AMIs: %w", err)
	}

	var resources []resource.Resource
	for _, img := range output.Images {
		resources = append(resources, imageResource(img))
	}

	// Build pagination metadata
	nextToken := ""
	isTruncated := false
	if output.NextToken != nil {
		nextToken = *output.NextToken
		isTruncated = true
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

// FetchAMIByID fetches one AMI by exact image ID. Unlike the generic AMI list
// fetcher, this path does not restrict Owners so public and third-party images
// referenced by EC2 instances can still open real detail views.
func FetchAMIByID(ctx context.Context, api EC2DescribeImagesAPI, imageID string) (resource.Resource, error) {
	output, err := api.DescribeImages(ctx, &ec2.DescribeImagesInput{
		ImageIds:          []string{imageID},
		IncludeDeprecated: aws.Bool(true),
	})
	if err != nil {
		return resource.Resource{}, fmt.Errorf("fetching AMI %s: %w", imageID, err)
	}
	if len(output.Images) == 0 {
		return resource.Resource{}, fmt.Errorf("AMI %s not found", imageID)
	}
	return imageResource(output.Images[0]), nil
}


func imageResource(img ec2types.Image) resource.Resource {
	imageID := ""
	if img.ImageId != nil {
		imageID = *img.ImageId
	}

	name := ""
	if img.Name != nil {
		name = *img.Name
	}

	state := string(img.State)
	architecture := string(img.Architecture)

	platform := ""
	if img.PlatformDetails != nil {
		platform = *img.PlatformDetails
	}

	rootDeviceType := string(img.RootDeviceType)

	creationDate := ""
	if img.CreationDate != nil {
		creationDate = *img.CreationDate
	}

	public := "false"
	if img.Public != nil && *img.Public {
		public = "true"
	}

	// Compute deprecated: "yes (Nmo ago)" if past, "soon" if within 90d, "" otherwise
	deprecated := ""
	if img.DeprecationTime != nil && *img.DeprecationTime != "" {
		if t, err := time.Parse(time.RFC3339, *img.DeprecationTime); err == nil {
			until := time.Until(t)
			switch {
			case until < 0:
				months := int(-until.Hours() / (24 * 30))
				if months < 1 {
					deprecated = "yes (<1mo ago)"
				} else {
					deprecated = fmt.Sprintf("yes (%dmo ago)", months)
				}
			case until < 90*24*time.Hour:
				deprecated = "soon"
			}
		}
	}

	return resource.Resource{
		ID:     imageID,
		Name:   name,
		Status: state,
		Fields: map[string]string{
			"image_id":         imageID,
			"name":             name,
			"state":            state,
			"architecture":     architecture,
			"platform":         platform,
			"root_device_type": rootDeviceType,
			"creation_date":    creationDate,
			"public":           public,
			"deprecated":       deprecated,
		},
		RawStruct: img,
	}
}
