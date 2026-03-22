package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/efs"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("efs", []string{"file_system_id", "name", "life_cycle_state", "performance_mode", "encrypted", "mount_targets"})
	resource.Register("efs", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEFSFileSystems(ctx, c.EFS)
	})
}

// FetchEFSFileSystems calls the EFS DescribeFileSystems API and converts
// the response into a slice of generic Resource structs.
func FetchEFSFileSystems(ctx context.Context, api EFSDescribeFileSystemsAPI) ([]resource.Resource, error) {
	output, err := api.DescribeFileSystems(ctx, &efs.DescribeFileSystemsInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching EFS file systems: %w", err)
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
			RawStruct:  fs,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
