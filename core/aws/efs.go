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

// efsW1Findings returns the Wave-1 findings; "no mount targets" is declared
// to apply only once the file system is available or in error, because mount
// targets are created after creation and removed before deletion.
func efsW1Findings(lcs efstypes.LifeCycleState, numMT int32, encrypted *bool) ([]domain.Finding, map[domain.FindingCode]domain.AttentionDetail) {
	var posture []domain.Finding
	if numMT == 0 {
		posture = append(posture, wave1Finding(CodeEFSNoMountTargets))
	}
	var details map[domain.FindingCode]domain.AttentionDetail
	if !aws.ToBool(encrypted) {
		posture = append(posture, wave1Finding(CodeEFSUnencrypted))
		details = map[domain.FindingCode]domain.AttentionDetail{
			CodeEFSUnencrypted: {Rows: []domain.DetailRow{
				{Label: "Encryption at rest", Value: "off", Tier: "~"},
			}},
		}
	}
	return withState("efs", string(lcs), posture), details
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

	output, err := api.DescribeFileSystems(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching EFS file systems: %w", err)
	}

	var resources []resource.Resource
	var failures []Failure
	asked := 0
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

		r := resource.Resource{
			ID:       fsID,
			Name:     name,
			Findings: findings,
			Fields: map[string]string{
				"file_system_id":   fsID,
				"name":             name,
				"status":           string(fs.LifeCycleState),
				"performance_mode": performanceMode,
				"throughput_mode":  throughputMode,
				"encrypted":        encrypted,
				"mount_targets":    mountTargets,
			},
			RawStruct:        fs,
			AttentionDetails: attentionDetails,
		}

		if listsAccessPoints && fsID != "" {
			asked++
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
	}, AggregateFailures("efs: DescribeAccessPoints", failures, asked)
}

// efsAccessPointIDs: a walk stopped at PerParentPageCap (100 a page, against a quota of 10,000 access points per
// file system) returns the IDs it read: a lower bound, and no call failed.
func efsAccessPointIDs(ctx context.Context, api EFSDescribeAccessPointsAPI, fsID string) ([]string, error) {
	// DescribeAccessPoints pages by NextToken
	// (https://docs.aws.amazon.com/efs/latest/ug/API_DescribeAccessPoints.html).
	points, _, err := PageAll(ctx, PerParentPageCap, func(ctx context.Context, token *string) ([]efstypes.AccessPointDescription, *string, error) {
		out, err := api.DescribeAccessPoints(ctx, &efs.DescribeAccessPointsInput{FileSystemId: &fsID, NextToken: token})
		if err != nil {
			return nil, nil, err
		}
		return out.AccessPoints, out.NextToken, nil
	})
	var ids []string
	for _, ap := range points {
		if ap.AccessPointId != nil {
			ids = append(ids, *ap.AccessPointId)
		}
	}
	return ids, err
}
