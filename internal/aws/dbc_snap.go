package aws

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"
	docdbtypes "github.com/aws/aws-sdk-go-v2/service/docdb/types"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ComputeDBCSnapStatusAndIssues computes the §4 status phrase and ordered
// issues slice for a DocDB / Aurora cluster snapshot. Returns ("", nil) for a
// healthy (available) snapshot. The top phrase becomes Resource.Status; the
// full slice becomes Resource.Issues.
//
// §0.1 / §3.1 precedence ladder (Broken > Warning, table order within severity):
//  1. Broken: Status == "failed" → phrase "failed"
//  2. Broken: strings.HasPrefix(Status, "incompatible-") → phrase verbatim
//  3. Warning: Status == "creating" → phrase "creating" (DBClusterSnapshot
//     has no PercentProgress field — §4 table omits it)
//  4. Warning: manual snapshot older than 365d → "manual, unused <N>d" where
//     N = int(time.Since(SnapshotCreateTime).Hours()/24).
//     Gate: SnapshotType == "manual" AND SnapshotCreateTime != nil AND age > 365.
//
// Cross-ref signals (orphan, past-retention) are added by the Wave-1 issue
// enricher (ComputeDBCSnapStatusAndIssues) via FieldUpdates, never here.
func ComputeDBCSnapStatusAndIssues(snap docdbtypes.DBClusterSnapshot) (string, []string) {
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

// computeDBCSnapFindings returns []domain.Finding for a DocDB cluster snapshot.
func computeDBCSnapFindings(snap docdbtypes.DBClusterSnapshot) []domain.Finding {
	rawStatus := aws.ToString(snap.Status)

	if rawStatus == "failed" {
		return []domain.Finding{{Code: CodeDBCSnapFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"}}
	}
	if strings.HasPrefix(rawStatus, "incompatible-") {
		return []domain.Finding{{Code: CodeDBCSnapIncompatible, Phrase: rawStatus, Severity: domain.SevBroken, Source: "wave1"}}
	}

	var findings []domain.Finding
	if rawStatus == "creating" {
		findings = append(findings, domain.Finding{Code: CodeDBCSnapCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1"})
	}
	if snap.SnapshotType != nil && *snap.SnapshotType == "manual" && snap.SnapshotCreateTime != nil {
		ageD := int(time.Since(*snap.SnapshotCreateTime).Hours() / 24)
		if ageD > 365 {
			phrase := fmt.Sprintf("manual, unused %dd", ageD)
			findings = append(findings, domain.Finding{Code: CodeDBCSnapManualUnused, Phrase: phrase, Severity: domain.SevWarn, Source: "wave1"})
		}
	}
	return findings
}

// FetchDocDBClusterSnapshots calls the DocumentDB DescribeDBClusterSnapshots API and converts the
// response into a slice of generic Resource structs. This covers DocumentDB cluster
// snapshots only (docdb@v1.48.12/api_op_DescribeDBClusterSnapshots.go:14).
// Aurora + Multi-AZ snapshots are fetched separately via FetchRDSDBClusterSnapshotsPage.
func FetchDocDBClusterSnapshots(ctx context.Context, api DocDBDescribeDBClusterSnapshotsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchDocDBClusterSnapshotsPage(ctx, api, token)
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

// FetchDocDBClusterSnapshotsPage fetches a single page of DocumentDB cluster snapshots.
func FetchDocDBClusterSnapshotsPage(ctx context.Context, api DocDBDescribeDBClusterSnapshotsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &docdb.DescribeDBClusterSnapshotsInput{
		MaxRecords: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := api.DescribeDBClusterSnapshots(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching DocumentDB cluster snapshots: %w", err)
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

		// Per spec §4 (docs/resources/dbc-snap.md), Status is the §4 phrase, not
		// raw AWS state. Healthy snapshots render BLANK.
		findings := computeDBCSnapFindings(snapshot)
		statusPhrase := phraseFromFindings(findings)

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
