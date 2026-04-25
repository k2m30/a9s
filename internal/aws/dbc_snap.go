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

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("dbc-snap", []string{"snapshot_id", "cluster_id", "status", "engine", "snapshot_type", "snapshot_create_time", "storage_type", "storage_encrypted"})

	// dbc-snap fetcher merges results from two separate SDK calls:
	//   c.DocDB.DescribeDBClusterSnapshots — DocumentDB cluster snapshots only
	//     (docdb@v1.48.12/api_op_DescribeDBClusterSnapshots.go:14)
	//   c.RDS.DescribeDBClusterSnapshots  — Aurora + Multi-AZ cluster snapshots
	//     (rds@v1.116.3/api_op_DescribeDBClusterSnapshots.go:19-25)
	// The two SDKs are NOT interchangeable — each scopes its response to its own
	// engine family. Token format: "" or "docdb:<tok>" for DocDB pages, then
	// "rds:" (sentinel) or "rds:<tok>" for RDS pages.
	resource.RegisterPaginated("dbc-snap", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}

		// RDS phase: continuation already transitioned to RDS side.
		// Partial DocDB rows were already returned in the prior page; this is a
		// single-side fetch with no prior-state append — an RDS error here returns
		// an empty result with the error so the operator can retry by re-opening the list.
		if rdsTok, ok2 := strings.CutPrefix(continuationToken, "rds:"); ok2 {
			result, err := FetchRDSDBClusterSnapshotsPage(ctx, c.RDS, rdsTok)
			if err != nil {
				return resource.FetchResult{}, err
			}
			if result.Pagination != nil && result.Pagination.IsTruncated {
				result.Pagination.NextToken = "rds:" + result.Pagination.NextToken
			}
			return result, nil
		}

		// DocDB phase (continuationToken == "" or "docdb:<tok>").
		docdbTok, _ := strings.CutPrefix(continuationToken, "docdb:")
		docResult, err := FetchDocDBClusterSnapshotsPage(ctx, c.DocDB, docdbTok)
		if err != nil {
			return resource.FetchResult{}, err
		}
		if docResult.Pagination != nil && docResult.Pagination.IsTruncated {
			// DocDB still has more pages — return with docdb: prefix.
			docResult.Pagination.NextToken = "docdb:" + docResult.Pagination.NextToken
			return docResult, nil
		}

		// Rule E5: preserve partial DocDB rows on RDS failure — return what we have
		// with IsTruncated=true so the operator sees the DocDB rows and a composite
		// error rather than an empty result with a silent discard.
		rdsResult, rdsErr := FetchRDSDBClusterSnapshotsPage(ctx, c.RDS, "")
		if rdsErr != nil {
			return resource.FetchResult{
				Resources: docResult.Resources,
				Pagination: &resource.PaginationMeta{
					IsTruncated: true,
					NextToken:   "rds:",
					PageSize:    len(docResult.Resources),
					TotalHint:   -1,
				},
			}, fmt.Errorf("dbc-snap: RDS-side cluster snapshot fetch failed: %w", rdsErr)
		}
		// Combined success: DocDB page + RDS page concatenated. Page size may exceed DefaultPageSize when both SDKs return full pages on the same fetch tick — this is a deliberate trade so the operator sees a unified list rather than waiting for a second tick. Pagination tokens stay correct (docdb: vs rds: prefix tracks side authoritatively).
		docResult.Resources = append(docResult.Resources, rdsResult.Resources...)
		if rdsResult.Pagination != nil && rdsResult.Pagination.IsTruncated {
			return resource.FetchResult{
				Resources: docResult.Resources,
				Pagination: &resource.PaginationMeta{
					IsTruncated: true,
					NextToken:   "rds:" + rdsResult.Pagination.NextToken,
					PageSize:    len(docResult.Resources),
					TotalHint:   -1,
				},
			}, nil
		}
		return resource.FetchResult{
			Resources: docResult.Resources,
			Pagination: &resource.PaginationMeta{
				IsTruncated: false,
				PageSize:    len(docResult.Resources),
				TotalHint:   len(docResult.Resources),
			},
		}, nil
	})

	resource.RegisterRelated("dbc-snap", []resource.RelatedDef{
		{TargetType: "dbc", DisplayName: "DocumentDB Cluster", Checker: checkDbcSnapDBC, NeedsTargetCache: true},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkDbcSnapKMS},
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkDbcSnapVPC},
		{TargetType: "backup", DisplayName: "Backup Plans", Checker: checkDbcSnapBackup},
		{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkDbcSnapCTEvents},
	})

	// docdbtypes.DBClusterSnapshot: VpcId, KmsKeyId
	resource.RegisterNavigableFields("dbc-snap", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
		{FieldPath: "KmsKeyId", TargetType: "kms"},
	})
}

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
		// raw AWS state. Healthy snapshots render BLANK. Wave-1 fetcher-local
		// signals (failed, incompatible-*, creating, manual-unused) are computed
		// by ComputeDBCSnapStatusAndIssues and stored in Issues. Cross-ref signals
		// (orphan, past-retention) come from enrichDBCSnapCrossRef and overwrite
		// via FieldUpdates / merge with computeMergedStatus.
		computedStatus, allIssues := ComputeDBCSnapStatusAndIssues(snapshot)

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
