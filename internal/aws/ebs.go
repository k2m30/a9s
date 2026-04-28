package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("ebs", []string{"volume_id", "name", "state", "size", "type", "iops", "encrypted", "attached_to", "az", "created"})
	resource.RegisterPaginated("ebs", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEBSVolumesPage(ctx, c.EC2, continuationToken)
	})

	resource.RegisterFieldKeys("ebs-snap", []string{"snapshot_id", "name", "state", "volume_id", "size", "encrypted", "description", "started", "progress"})
	resource.RegisterPaginated("ebs-snap", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEBSSnapshotsPage(ctx, c.EC2, continuationToken)
	})

	resource.RegisterFetchByIDs("ebs-snap", func(ctx context.Context, clients any, ids []string) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchEBSSnapshotsByIDs(ctx, c.EC2, ids)
	})

	resource.RegisterRelated("ebs", []resource.RelatedDef{
		{TargetType: "ec2", DisplayName: "EC2 Instance", Checker: checkEBSEC2, NeedsTargetCache: false},
		{TargetType: "ebs-snap", DisplayName: "EBS Snapshots", Checker: checkEBSSnap, NeedsTargetCache: true},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkEBSKMS, NeedsTargetCache: false},
		{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkEBSAlarm, NeedsTargetCache: true},
		{TargetType: "backup", DisplayName: "Backup", Checker: checkEBSBackup},
		{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkEBSCFN, NeedsTargetCache: true},
	})
	resource.RegisterDefaultNavFields("ebs", []resource.NavigableField{
		{FieldPath: "Attachments.InstanceId", TargetType: "ec2"},
		{FieldPath: "KmsKeyId", TargetType: "kms"},
	})

	resource.RegisterRelated("ebs-snap", []resource.RelatedDef{
		{TargetType: "ami", DisplayName: "AMIs", Checker: checkEBSSnapAMI, NeedsTargetCache: true},
		{TargetType: "ebs", DisplayName: "EBS Volume", Checker: checkEBSSnapEBS, NeedsTargetCache: false},
		{TargetType: "ec2", DisplayName: "EC2 Instance", Checker: checkEBSSnapEC2, NeedsTargetCache: false},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkEBSSnapKMS, NeedsTargetCache: false},
		{TargetType: "backup", DisplayName: "Backup", Checker: checkEBSSnapBackup},
	})
	resource.RegisterDefaultNavFields("ebs-snap", []resource.NavigableField{
		{FieldPath: "VolumeId", TargetType: "ebs"},
		{FieldPath: "KmsKeyId", TargetType: "kms"},
	})
}

// FetchEBSVolumes calls the EC2 DescribeVolumes API and returns all pages
// of volumes. Used by tests; the production path uses the per-page fetcher for pagination.
func FetchEBSVolumes(ctx context.Context, api EC2DescribeVolumesAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchEBSVolumesPage(ctx, api, token)
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

// FetchEBSVolumesPage calls the EC2 DescribeVolumes API and returns a single
// page of volumes. Pass an empty continuationToken for the first page.
func FetchEBSVolumesPage(ctx context.Context, api EC2DescribeVolumesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeVolumesInput{
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeVolumes(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching EBS volumes: %w", err)
	}

	var resources []resource.Resource
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
			ID:   volumeID,
			Name: name,
			// Status: removed — PR-03b migrates fetcher to Findings for lifecycle states.
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

		// Phase 03 PR-03b: emit canonical Findings for non-healthy volume states.
		// in-use and available are healthy (no Finding). deleting is terminal (no Finding).
		// creating → SevWarn. error → SevBroken.
		switch vol.State {
		case ec2types.VolumeStateCreating:
			r.Findings = []domain.Finding{{
				Code: CodeEBSStateCreating, Phrase: "creating",
				Severity: domain.SevWarn, Source: "wave1",
			}}
		case ec2types.VolumeStateError:
			r.Findings = []domain.Finding{{
				Code: CodeEBSStateError, Phrase: "error",
				Severity: domain.SevBroken, Source: "wave1",
			}}
		}

		resources = append(resources, r)
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

// FetchEBSSnapshots calls the EC2 DescribeSnapshots API and returns all pages
// of snapshots. Used by tests; the production path uses the per-page fetcher for pagination.
func FetchEBSSnapshots(ctx context.Context, api EC2DescribeSnapshotsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchEBSSnapshotsPage(ctx, api, token)
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

// FetchEBSSnapshotsPage calls the EC2 DescribeSnapshots API and returns a single
// page of snapshots. Only returns snapshots owned by the caller ("self").
// Pass an empty continuationToken for the first page.
func FetchEBSSnapshotsPage(ctx context.Context, api EC2DescribeSnapshotsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &ec2.DescribeSnapshotsInput{
		OwnerIds:   []string{"self"},
		MaxResults: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.NextToken = &continuationToken
	}

	output, err := api.DescribeSnapshots(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching EBS snapshots: %w", err)
	}

	var resources []resource.Resource
	for _, snap := range output.Snapshots {
		resources = append(resources, snapshotToResource(snap))
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

// FetchEBSSnapshotsByIDs fetches specific EBS snapshots by ID, bypassing the
// OwnerIds=self filter the paginated fetcher applies. Used by the related-panel
// lazy-add path so checkers referencing shared or public snapshots still drill
// into a real entry. DescribeSnapshots accepts SnapshotIds as a batched
// filter, so this is a single API call.
//
// The DescribeSnapshots call is wrapped in RetryOnThrottle. IDs not present in
// the response are collected into a composite error returned alongside the
// partial results.
func FetchEBSSnapshotsByIDs(ctx context.Context, api EC2DescribeSnapshotsAPI, ids []string) ([]resource.Resource, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	filtered := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" {
			filtered = append(filtered, id)
		}
	}
	if len(filtered) == 0 {
		return nil, nil
	}
	out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*ec2.DescribeSnapshotsOutput, error) {
		return api.DescribeSnapshots(ctx, &ec2.DescribeSnapshotsInput{
			SnapshotIds: filtered,
		})
	})
	if err != nil {
		return nil, fmt.Errorf("fetching EBS snapshots by id: %w", err)
	}

	// Build a set of returned IDs to detect which requested IDs are missing.
	returned := make(map[string]struct{}, len(out.Snapshots))
	resources := make([]resource.Resource, 0, len(out.Snapshots))
	for _, snap := range out.Snapshots {
		r := snapshotToResource(snap)
		resources = append(resources, r)
		returned[r.ID] = struct{}{}
	}

	var failures []string
	for _, id := range filtered {
		if _, found := returned[id]; !found {
			failures = append(failures, id)
		}
	}
	return resources, AggregateMissing("ebs-snap FetchByIDs", failures, len(filtered))
}

// snapshotToResource converts an ec2types.Snapshot to our generic Resource.
// Extracted so FetchEBSSnapshotsPage and FetchEBSSnapshotsByIDs agree on
// shape — the lazy-add path must emit the same field keys that the paginated
// path does, otherwise reverse-scan checkers reading Fields["volume_id"]
// miss lazily-added snapshots.
func snapshotToResource(snap ec2types.Snapshot) resource.Resource {
	snapshotID := ""
	if snap.SnapshotId != nil {
		snapshotID = *snap.SnapshotId
	}
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
		ID:   snapshotID,
		Name: name,
		// Status: removed — PR-03b migrates fetcher to Findings for lifecycle states.
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

	// Phase 03 PR-03b: emit canonical Findings for non-healthy snapshot states.
	// completed → healthy (no Finding). pending → SevWarn.
	// error / recoverable / recovering → SevBroken.
	switch snap.State {
	case ec2types.SnapshotStatePending:
		r.Findings = []domain.Finding{{
			Code: CodeEBSSnapStatePending, Phrase: "pending",
			Severity: domain.SevWarn, Source: "wave1",
		}}
	case ec2types.SnapshotStateError, ec2types.SnapshotStateRecoverable, ec2types.SnapshotStateRecovering:
		r.Findings = []domain.Finding{{
			Code: CodeEBSSnapStateError, Phrase: "error",
			Severity: domain.SevBroken, Source: "wave1",
		}}
	}

	return r
}
