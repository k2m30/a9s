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

	"github.com/k2m30/a9s/v3/internal/resource"
)

// ComputeRDSDBClusterSnapshotStatusAndIssues computes the §4 status phrase and
// ordered issues slice for an Aurora / Multi-AZ DB cluster snapshot fetched via
// the RDS SDK. Returns ("", nil) for a healthy (available) snapshot.
//
// Algorithm mirrors ComputeDBCSnapStatusAndIssues — same §4 precedence ladder:
//  1. Broken: Status == "failed" → phrase "failed"
//  2. Broken: strings.HasPrefix(Status, "incompatible-") → phrase verbatim
//  3. Warning: Status == "creating" → phrase "creating"
//  4. Warning: manual snapshot older than 365d → "manual, unused <N>d"
func ComputeRDSDBClusterSnapshotStatusAndIssues(snap rdstypes.DBClusterSnapshot) (string, []string) {
	rawStatus := ""
	if snap.Status != nil {
		rawStatus = *snap.Status
	}

	var issues []string

	// Broken checks first (severity wins).
	if rawStatus == "failed" {
		issues = append(issues, "failed")
		return buildStatusFromIssues(issues), issues
	}
	if strings.HasPrefix(rawStatus, "incompatible-") {
		issues = append(issues, rawStatus)
		return buildStatusFromIssues(issues), issues
	}

	// Warning: creating (transitional). DBClusterSnapshot has no PercentProgress.
	if rawStatus == "creating" {
		issues = append(issues, "creating")
	}

	// Warning: manual snapshot unused for > 365 days.
	if snap.SnapshotType != nil && *snap.SnapshotType == "manual" && snap.SnapshotCreateTime != nil {
		ageD := int(time.Since(*snap.SnapshotCreateTime).Hours() / 24)
		if ageD > 365 {
			issues = append(issues, fmt.Sprintf("manual, unused %dd", ageD))
		}
	}

	return buildStatusFromIssues(issues), issues
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

		computedStatus, allIssues := ComputeRDSDBClusterSnapshotStatusAndIssues(snapshot)

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
			ID:     snapshotID,
			Name:   snapshotID,
			Status: computedStatus,
			Issues: allIssues,
			Fields: map[string]string{
				"snapshot_id":          snapshotID,
				"cluster_id":           clusterID,
				"status":               computedStatus,
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
