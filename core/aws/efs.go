// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/efs"
	efstypes "github.com/aws/aws-sdk-go-v2/service/efs/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// efsW1Findings returns the active Wave-1 findings for this filesystem in
// precedence order: Broken signals first (error, no mount targets), then
// Warning signals (creating, updating, deleting). The first finding's phrase
// is the "top" displayed in the Status column (plus (+N-1) suffix); the full
// slice feeds Resource.Findings so the detail view can render every signal.
func efsW1Findings(lcs efstypes.LifeCycleState, numMT int32, encrypted *bool) ([]domain.Finding, map[domain.FindingCode]domain.AttentionDetail) {
	var findings []domain.Finding

	switch lcs {
	case efstypes.LifeCycleStateError:
		findings = append(findings, wave1Finding(CodeEFSError))
	case efstypes.LifeCycleStateCreating:
		findings = append(findings, wave1Finding(CodeEFSCreating))
	case efstypes.LifeCycleStateUpdating:
		findings = append(findings, wave1Finding(CodeEFSUpdating))
	case efstypes.LifeCycleStateDeleting:
		findings = append(findings, wave1Finding(CodeEFSDeleting))
	}

	if numMT == 0 && lcs != efstypes.LifeCycleStateDeleted {
		noMTFinding := wave1Finding(CodeEFSNoMountTargets)
		if len(findings) > 0 && findings[0].Code == CodeEFSError {
			findings = append([]domain.Finding{findings[0], noMTFinding}, findings[1:]...)
		} else {
			findings = append([]domain.Finding{noMTFinding}, findings...)
		}
	}

	if isTeardownStatus(string(lcs)) {
		return findings, nil
	}
	if aws.ToBool(encrypted) {
		return findings, nil
	}
	return append(findings, wave1Finding(CodeEFSUnencrypted)), map[domain.FindingCode]domain.AttentionDetail{
		CodeEFSUnencrypted: {Rows: []domain.DetailRow{
			{Label: "Encryption at rest", Value: "off", Tier: "~"},
		}},
	}
}

// FetchEFSFileSystemsPage fetches a single page of EFS file systems. When api
// can also describe access points, each file system carries the IDs of its
// access points (Fields["access_point_ids"]): a Lambda mounts an access point,
// and only the file system's row can say which one is its own.
func FetchEFSFileSystemsPage(ctx context.Context, api EFSDescribeFileSystemsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &efs.DescribeFileSystemsInput{
		MaxItems: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*efs.DescribeFileSystemsOutput, error) {
		return api.DescribeFileSystems(ctx, input)
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching EFS file systems: %w", err)
	}

	var resources []resource.Resource
	var failures []Failure
	apAPI, listsAccessPoints := api.(EFSDescribeAccessPointsAPI)

	for _, fs := range output.FileSystems {
		fsID := ""
		if fs.FileSystemId != nil {
			fsID = *fs.FileSystemId
		}

		name := ""
		if fs.Name != nil {
			name = *fs.Name
		}

		performanceMode := string(fs.PerformanceMode)
		throughputMode := string(fs.ThroughputMode)

		encrypted := "false"
		if fs.Encrypted != nil && *fs.Encrypted {
			encrypted = "true"
		}

		mountTargets := fmt.Sprintf("%d", fs.NumberOfMountTargets)

		findings, attentionDetails := efsW1Findings(fs.LifeCycleState, fs.NumberOfMountTargets, fs.Encrypted)
		statusPhrase := domain.StatusPhrase(findings)

		r := resource.Resource{
			ID:       fsID,
			Name:     name,
			Findings: findings,
			Fields: map[string]string{
				"file_system_id":   fsID,
				"name":             name,
				"status":           statusPhrase,
				"performance_mode": performanceMode,
				"throughput_mode":  throughputMode,
				"encrypted":        encrypted,
				"mount_targets":    mountTargets,
			},
			RawStruct:        fs,
			AttentionDetails: attentionDetails,
		}

		if listsAccessPoints && fsID != "" {
			ids, err := efsAccessPointIDs(ctx, apAPI, fsID)
			if err != nil {
				failures = append(failures, FailedCall(fsID, err))
			}
			r.Fields["access_point_ids"] = strings.Join(ids, ",")
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
	}, AggregateFailures("efs: DescribeAccessPoints", failures, len(output.FileSystems))
}

// efsAccessPointIDs returns the IDs of fsID's access points, every page of
// them.
func efsAccessPointIDs(ctx context.Context, api EFSDescribeAccessPointsAPI, fsID string) ([]string, error) {
	var ids []string
	input := &efs.DescribeAccessPointsInput{FileSystemId: &fsID}
	for {
		out, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*efs.DescribeAccessPointsOutput, error) {
			return api.DescribeAccessPoints(ctx, input)
		})
		if err != nil {
			return ids, err
		}
		for _, ap := range out.AccessPoints {
			if ap.AccessPointId != nil {
				ids = append(ids, *ap.AccessPointId)
			}
		}
		if out.NextToken == nil || *out.NextToken == "" {
			return ids, nil
		}
		input.NextToken = out.NextToken
	}
}
