package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ami", []string{"image_id", "name", "state", "architecture", "platform", "root_device_type", "creation_date", "public"})
	resource.Register("ami", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchAMIs(ctx, c.EC2)
	})
}

// FetchAMIs calls the EC2 DescribeImages API and returns a slice of
// generic Resource structs. Only returns AMIs owned by the caller ("self").
func FetchAMIs(ctx context.Context, api EC2DescribeImagesAPI) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	for {
		output, err := api.DescribeImages(ctx, &ec2.DescribeImagesInput{
			Owners:    []string{"self"},
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("fetching AMIs: %w", err)
		}

		for _, img := range output.Images {
			imageID := ""
			if img.ImageId != nil {
				imageID = *img.ImageId
			}

			// Name comes directly from Image.Name, not from Tags
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

			r := resource.Resource{
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
				},
				RawStruct: img,
			}

			resources = append(resources, r)
		}

		if output.NextToken == nil {
			break
		}
		nextToken = output.NextToken
	}

	return resources, nil
}
