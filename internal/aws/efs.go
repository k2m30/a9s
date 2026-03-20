package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/efs"

	"github.com/k2m30/a9s/internal/resource"
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
		return nil, err
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

		arn := ""
		if fs.FileSystemArn != nil {
			arn = *fs.FileSystemArn
		}

		ownerID := ""
		if fs.OwnerId != nil {
			ownerID = *fs.OwnerId
		}

		sizeInBytes := ""
		if fs.SizeInBytes != nil {
			sizeInBytes = fmt.Sprintf("%d", fs.SizeInBytes.Value)
		}

		creationTime := ""
		if fs.CreationTime != nil {
			creationTime = fs.CreationTime.Format("2006-01-02 15:04:05")
		}

		// Build DetailData
		detail := map[string]string{
			"File System ID":   fsID,
			"Name":             name,
			"Life Cycle State": lifeCycleState,
			"Performance Mode": performanceMode,
			"Throughput Mode":  throughputMode,
			"Encrypted":        encrypted,
			"Mount Targets":    mountTargets,
			"ARN":              arn,
			"Owner ID":         ownerID,
			"Size In Bytes":    sizeInBytes,
			"Creation Time":    creationTime,
		}

		// Tags
		for _, tag := range fs.Tags {
			if tag.Key != nil && tag.Value != nil {
				detail["Tag: "+*tag.Key] = *tag.Value
			}
		}

		// Build RawJSON
		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(fs, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
		}

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
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  fs,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
