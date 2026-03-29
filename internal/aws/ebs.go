package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/service/ec2"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ebs", []string{"volume_id", "name", "state", "size", "type", "iops", "encrypted", "attached_to", "az", "created"})
	resource.Register("ebs", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEBSVolumes(ctx, c.EC2)
	})

	resource.RegisterFieldKeys("ebs-snap", []string{"snapshot_id", "name", "state", "volume_id", "size", "encrypted", "description", "started", "progress"})
	resource.Register("ebs-snap", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEBSSnapshots(ctx, c.EC2)
	})
}

// FetchEBSVolumes calls the EC2 DescribeVolumes API and returns a slice of
// generic Resource structs.
func FetchEBSVolumes(ctx context.Context, api EC2DescribeVolumesAPI) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	for {
		output, err := api.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("fetching EBS volumes: %w", err)
		}

		for _, vol := range output.Volumes {
			volumeID := ""
			if vol.VolumeId != nil {
				volumeID = *vol.VolumeId
			}

			// Extract Name from Tags
			name := ""
			for _, tag := range vol.Tags {
				if tag.Key != nil && *tag.Key == "Name" {
					if tag.Value != nil {
						name = *tag.Value
					}
					break
				}
			}

			state := string(vol.State)

			size := ""
			if vol.Size != nil {
				size = strconv.Itoa(int(*vol.Size))
			}

			volType := string(vol.VolumeType)

			iops := ""
			if vol.Iops != nil {
				iops = strconv.Itoa(int(*vol.Iops))
			}

			encrypted := "false"
			if vol.Encrypted != nil && *vol.Encrypted {
				encrypted = "true"
			}

			attachedTo := ""
			if len(vol.Attachments) > 0 && vol.Attachments[0].InstanceId != nil {
				attachedTo = *vol.Attachments[0].InstanceId
			}

			az := ""
			if vol.AvailabilityZone != nil {
				az = *vol.AvailabilityZone
			}

			created := ""
			if vol.CreateTime != nil {
				created = vol.CreateTime.Format("2006-01-02 15:04")
			}

			r := resource.Resource{
				ID:     volumeID,
				Name:   name,
				Status: state,
				Fields: map[string]string{
					"volume_id":   volumeID,
					"name":        name,
					"state":       state,
					"size":        size,
					"type":        volType,
					"iops":        iops,
					"encrypted":   encrypted,
					"attached_to": attachedTo,
					"az":          az,
					"created":     created,
				},
				RawStruct: vol,
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

// FetchEBSSnapshots calls the EC2 DescribeSnapshots API and returns a slice of
// generic Resource structs. Only returns snapshots owned by the caller ("self").
func FetchEBSSnapshots(ctx context.Context, api EC2DescribeSnapshotsAPI) ([]resource.Resource, error) {
	var resources []resource.Resource
	var nextToken *string

	for {
		output, err := api.DescribeSnapshots(ctx, &ec2.DescribeSnapshotsInput{
			OwnerIds:  []string{"self"},
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("fetching EBS snapshots: %w", err)
		}

		for _, snap := range output.Snapshots {
			snapshotID := ""
			if snap.SnapshotId != nil {
				snapshotID = *snap.SnapshotId
			}

			// Extract Name from Tags
			name := ""
			for _, tag := range snap.Tags {
				if tag.Key != nil && *tag.Key == "Name" {
					if tag.Value != nil {
						name = *tag.Value
					}
					break
				}
			}

			state := string(snap.State)

			volumeID := ""
			if snap.VolumeId != nil {
				volumeID = *snap.VolumeId
			}

			size := ""
			if snap.VolumeSize != nil {
				size = strconv.Itoa(int(*snap.VolumeSize))
			}

			encrypted := "false"
			if snap.Encrypted != nil && *snap.Encrypted {
				encrypted = "true"
			}

			description := ""
			if snap.Description != nil {
				description = *snap.Description
			}

			started := ""
			if snap.StartTime != nil {
				started = snap.StartTime.Format("2006-01-02 15:04")
			}

			progress := ""
			if snap.Progress != nil {
				progress = *snap.Progress
			}

			r := resource.Resource{
				ID:     snapshotID,
				Name:   name,
				Status: state,
				Fields: map[string]string{
					"snapshot_id": snapshotID,
					"name":        name,
					"state":       state,
					"volume_id":   volumeID,
					"size":        size,
					"encrypted":   encrypted,
					"description": description,
					"started":     started,
					"progress":    progress,
				},
				RawStruct: snap,
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
