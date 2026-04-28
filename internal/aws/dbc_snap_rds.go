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

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// computeRDSDBClusterSnapshotFindings computes the ordered Wave-1 findings
// for an Aurora / Multi-AZ DB cluster snapshot fetched via the RDS SDK.
// Returns nil for a healthy (available) snapshot.
//
// Algorithm mirrors computeDBCSnapFindings — same §4 precedence ladder:
//  1. Broken: Status == "failed" → phrase "failed"
//  2. Broken: strings.HasPrefix(Status, "incompatible-") → phrase verbatim
//  3. Warning: Status == "creating" → phrase "creating"
//  4. Warning: manual snapshot older than 365d → "manual, unused <N>d"
func computeRDSDBClusterSnapshotFindings(snap rdstypes.DBClusterSnapshot) []domain.Finding {
	rawStatus := ""
	if snap.Status != nil {
		rawStatus = *snap.Status
	}

	// Broken checks first (severity wins).
	if rawStatus == "failed" {
		return []domain.Finding{{Code: CodeDBCSnapFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"}}
	}
	if strings.HasPrefix(rawStatus, "incompatible-") {
		return []domain.Finding{{Code: CodeDBCSnapIncompatible, Phrase: rawStatus, Severity: domain.SevBroken, Source: "wave1"}}
	}

	var findings []domain.Finding

	// Warning: creating (transitional). DBClusterSnapshot has no PercentProgress.
	if rawStatus == "creating" {
		findings = append(findings, domain.Finding{Code: CodeDBCSnapCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1"})
	}

	// Warning: manual snapshot unused for > 365 days.
	if snap.SnapshotType != nil && *snap.SnapshotType == "manual" && snap.SnapshotCreateTime != nil {
		ageD := int(time.Since(*snap.SnapshotCreateTime).Hours() / 24)
		if ageD > 365 {
			findings = append(findings, domain.Finding{Code: CodeDBCSnapManualUnused, Phrase: fmt.Sprintf("manual, unused %dd", ageD), Severity: domain.SevWarn, Source: "wave1"})
		}
	}

	return findings
}

// ComputeRDSDBClusterSnapshotStatusAndIssues is the exported compatibility
// wrapper around computeRDSDBClusterSnapshotFindings. The status phrase
// carries the (+N) suffix to match what FetchRDSDBClusterSnapshotsPage writes
// to Fields["status"].
func ComputeRDSDBClusterSnapshotStatusAndIssues(snap rdstypes.DBClusterSnapshot) (string, []string) {
	findings := computeRDSDBClusterSnapshotFindings(snap)
	if len(findings) == 0 {
		return "", nil
	}
	phrases := make([]string, len(findings))
	for i, f := range findings {
		phrases[i] = f.Phrase
	}
	statusPhrase := phrases[0]
	if len(phrases) > 1 {
		statusPhrase = fmt.Sprintf("%s (+%d)", statusPhrase, len(phrases)-1)
	}
	return statusPhrase, phrases
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
		statusPhrase := ""
		if len(findings) > 0 {
			statusPhrase = findings[0].Phrase
			if len(findings) > 1 {
				statusPhrase = fmt.Sprintf("%s (+%d)", statusPhrase, len(findings)-1)
			}
		}

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
			ID:   snapshotID,
			Name: snapshotID,
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
			Findings:  findings,
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
