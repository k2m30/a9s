package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/efs"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("efs", []string{"file_system_id", "name", "life_cycle_state", "performance_mode", "encrypted", "mount_targets"})

	resource.RegisterPaginated("efs", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEFSFileSystemsPage(ctx, c.EFS, continuationToken)
	})
}

// FetchEFSFileSystems calls the EFS DescribeFileSystems API and converts
// the response into a slice of generic Resource structs.
func FetchEFSFileSystems(ctx context.Context, api EFSDescribeFileSystemsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchEFSFileSystemsPage(ctx, api, token)
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

// FetchEFSFileSystemsPage fetches a single page of EFS file systems.
func FetchEFSFileSystemsPage(ctx context.Context, api EFSDescribeFileSystemsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &efs.DescribeFileSystemsInput{}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := api.DescribeFileSystems(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching EFS file systems: %w", err)
	}

	var resources []resource.Resource

	for _, fs := range output.FileSystems {
		fsID := ""
		if fs.FileSystemId != nil {
			fsID = *fs.FileSystemId
		}

		name := ""
		if fs.Name != nil {
			name = *fs.Name
		}

		lifeCycleState := string(fs.LifeCycleState)
		performanceMode := string(fs.PerformanceMode)
		throughputMode := string(fs.ThroughputMode)

		encrypted := "false"
		if fs.Encrypted != nil && *fs.Encrypted {
			encrypted = "true"
		}

		mountTargets := fmt.Sprintf("%d", fs.NumberOfMountTargets)

		r := resource.Resource{
			ID:     fsID,
			Name:   name,
			Status: lifeCycleState,
			Fields: map[string]string{
				"file_system_id":   fsID,
				"name":             name,
				"life_cycle_state": lifeCycleState,
				"performance_mode": performanceMode,
				"throughput_mode":  throughputMode,
				"encrypted":        encrypted,
				"mount_targets":    mountTargets,
			},
			RawStruct: fs,
		}

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.NextMarker != nil {
		nextToken = *output.NextMarker
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
