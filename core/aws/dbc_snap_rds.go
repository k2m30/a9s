// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// computeRDSDBClusterSnapshotFindings returns []domain.Finding for an RDS cluster snapshot.
func computeRDSDBClusterSnapshotFindings(snap rdstypes.DBClusterSnapshot) []domain.Finding {
	rawStatus := aws.ToString(snap.Status)

	if rawStatus == "failed" {
		return []domain.Finding{wave1Finding(CodeDBCSnapFailed)}
	}
	if strings.HasPrefix(rawStatus, "incompatible-") {
		return []domain.Finding{wave1Finding(CodeDBCSnapIncompatible, rawStatus)}
	}

	var findings []domain.Finding
	switch {
	case rawStatus == "creating":
		findings = append(findings, wave1Finding(CodeDBCSnapCreating))
	case rawStatus != "" && rawStatus != "available":
		// Any other state AWS reports is one the snapshot cannot be restored
		// from yet — copying, pending, and whatever the API adds next. Passing
		// the keyword through keeps a new state visible instead of silently
		// ready, the way computeDBCFindings does for a cluster.
		findings = append(findings, wave1Finding(CodeDBCSnapTransitional, rawStatus))
	}
	if snap.SnapshotCreateTime != nil {
		ageD := int(time.Since(*snap.SnapshotCreateTime).Hours() / 24)
		if ageD > dbcSnapManualUnusedAfterDays {
			findings = append(findings, wave1Finding(CodeDBCSnapManualUnused, strconv.Itoa(ageD)))
		}
	}
	if snap.StorageEncrypted != nil && !*snap.StorageEncrypted {
		findings = append(findings, wave1Finding(CodeDBCSnapUnencrypted))
	}
	return findings
}

// FetchRDSDBClusterSnapshotsPage fetches a single page of Aurora + Multi-AZ DB
// cluster snapshots via the RDS SDK.
func FetchRDSDBClusterSnapshotsPage(ctx context.Context, api RDSDescribeDBClusterSnapshotsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &rds.DescribeDBClusterSnapshotsInput{
		MaxRecords: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := api.DescribeDBClusterSnapshots(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching RDS cluster snapshots: %w", err)
	}

	var resources []resource.Resource

	for _, snapshot := range output.DBClusterSnapshots {
		if !isRDSSideDBCEngine(aws.ToString(snapshot.Engine)) {
			continue
		}
		snapshotID := ""
		if snapshot.DBClusterSnapshotIdentifier != nil {
			snapshotID = *snapshot.DBClusterSnapshotIdentifier
		}

		clusterID := ""
		if snapshot.DBClusterIdentifier != nil {
			clusterID = *snapshot.DBClusterIdentifier
		}

		findings := computeRDSDBClusterSnapshotFindings(snapshot)

		engine := ""
		if snapshot.Engine != nil {
			engine = *snapshot.Engine
		}

		snapshotType := ""
		if snapshot.SnapshotType != nil {
			snapshotType = *snapshot.SnapshotType
		}

		snapshotCreateTime := ""
		if snapshot.SnapshotCreateTime != nil {
			snapshotCreateTime = snapshot.SnapshotCreateTime.Format("2006-01-02 15:04")
		}

		storageType := ""
		if snapshot.StorageType != nil {
			storageType = *snapshot.StorageType
		}

		storageEncrypted := "false"
		if snapshot.StorageEncrypted != nil {
			storageEncrypted = strconv.FormatBool(*snapshot.StorageEncrypted)
		}

		r := withWave1(resource.Resource{
			ID:   snapshotID,
			Name: snapshotID,
			Fields: map[string]string{
				"snapshot_id":          snapshotID,
				"cluster_id":           clusterID,
				"engine":               engine,
				"snapshot_type":        snapshotType,
				"snapshot_create_time": snapshotCreateTime,
				"storage_type":         storageType,
				"storage_encrypted":    storageEncrypted,
			},
			RawStruct: snapshot,
		}, findings...)

		resources = append(resources, r)
	}

	nextToken := ""
	isTruncated := false
	if output.Marker != nil {
		nextToken = *output.Marker
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
