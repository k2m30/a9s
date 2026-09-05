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
		return []domain.Finding{{Code: CodeDBCSnapFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"}}
	}
	if strings.HasPrefix(rawStatus, "incompatible-") {
		return []domain.Finding{{Code: CodeDBCSnapIncompatible, Phrase: rawStatus, Severity: domain.SevBroken, Source: "wave1"}}
	}

	var findings []domain.Finding
	switch {
	case rawStatus == "creating":
		findings = append(findings, domain.Finding{Code: CodeDBCSnapCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1"})
	case rawStatus != "" && rawStatus != "available":
		// Any other state AWS reports is one the snapshot cannot be restored
		// from yet — copying, pending, and whatever the API adds next. Passing
		// the keyword through keeps a new state visible instead of silently
		// ready, the way computeDBCFindings does for a cluster.
		findings = append(findings, domain.Finding{Code: CodeDBCSnapTransitional, Phrase: rawStatus, Severity: domain.SevWarn, Source: "wave1"})
	}
	if snap.SnapshotType != nil && *snap.SnapshotType == "manual" && snap.SnapshotCreateTime != nil {
		ageD := int(time.Since(*snap.SnapshotCreateTime).Hours() / 24)
		if ageD > 365 {
			phrase := fmt.Sprintf("manual, unused %dd", ageD)
			findings = append(findings, domain.Finding{Code: CodeDBCSnapManualUnused, Phrase: phrase, Severity: domain.SevWarn, Source: "wave1"})
		}
	}
	if snap.StorageEncrypted != nil && !*snap.StorageEncrypted {
		findings = append(findings, domain.Finding{Code: CodeDBCSnapUnencrypted, Phrase: "unencrypted", Severity: domain.SevWarn, Source: "wave1"})
	}
	return findings
}

// FetchRDSDBClusterSnapshotsPage fetches a single page of Aurora + Multi-AZ DB
// cluster snapshots via the RDS SDK.
//
// Per AWS SDK docstring (rds@v1.116.3/api_op_DescribeDBClusterSnapshots.go:19-25),
// this operation returns Aurora and Multi-AZ cluster snapshots. The docdb-side
// DescribeDBClusterSnapshots is scoped to DocumentDB only
// (docdb@v1.48.12/api_op_DescribeDBClusterSnapshots.go:14).
func FetchRDSDBClusterSnapshotsPage(ctx context.Context, api RDSDescribeDBClusterSnapshotsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &rds.DescribeDBClusterSnapshotsInput{
		MaxRecords: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := RetryOnThrottle(ctx, DefaultRetryConfig(), func() (*rds.DescribeDBClusterSnapshotsOutput, error) {
		return api.DescribeDBClusterSnapshots(ctx, input)
	})
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching RDS cluster snapshots: %w", err)
	}

	var resources []resource.Resource

	for _, snapshot := range output.DBClusterSnapshots {
		snapshotID := ""
		if snapshot.DBClusterSnapshotIdentifier != nil {
			snapshotID = *snapshot.DBClusterSnapshotIdentifier
		}

		clusterID := ""
		if snapshot.DBClusterIdentifier != nil {
			clusterID = *snapshot.DBClusterIdentifier
		}

		findings := computeRDSDBClusterSnapshotFindings(snapshot)
		statusPhrase := domain.StatusPhrase(findings)

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

		r := resource.Resource{
			ID:       snapshotID,
			Name:     snapshotID,
			Findings: findings,
			Fields: map[string]string{
				"snapshot_id":          snapshotID,
				"cluster_id":           clusterID,
				"status":               statusPhrase,
				"engine":               engine,
				"snapshot_type":        snapshotType,
				"snapshot_create_time": snapshotCreateTime,
				"storage_type":         storageType,
				"storage_encrypted":    storageEncrypted,
			},
			RawStruct: snapshot,
		}

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
