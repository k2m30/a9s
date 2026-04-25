package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/docdb"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("dbc-snap", []string{"snapshot_id", "cluster_id", "status", "engine", "snapshot_type", "snapshot_create_time", "storage_type", "storage_encrypted"})

	resource.RegisterPaginated("dbc-snap", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchDocDBClusterSnapshotsPage(ctx, c.DocDB, continuationToken)
	})

	resource.RegisterRelated("dbc-snap", []resource.RelatedDef{
		{TargetType: "dbc", DisplayName: "DocumentDB Cluster", Checker: checkDbcSnapDBC},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkDbcSnapKMS},
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkDbcSnapVPC},
		{TargetType: "backup", DisplayName: "Backup Plans", Checker: checkDbcSnapBackup},
	})

	// docdbtypes.DBClusterSnapshot: VpcId, KmsKeyId
	resource.RegisterNavigableFields("dbc-snap", []resource.NavigableField{
		{FieldPath: "VpcId", TargetType: "vpc"},
		{FieldPath: "KmsKeyId", TargetType: "kms"},
	})
}

// computeDBCSnapPhrase returns the §4 status phrase for a DocDB / Aurora
// cluster snapshot given the raw AWS Status keyword. Healthy = "available"
// → blank. Transitional and Broken states carry their keyword verbatim
// (no per-row failure-reason field exists on DBClusterSnapshot).
func computeDBCSnapPhrase(rawStatus string) string {
	switch rawStatus {
	case "available", "":
		return ""
	default:
		return rawStatus
	}
}

// FetchDocDBClusterSnapshots calls the DocumentDB DescribeDBClusterSnapshots API and converts the
// response into a slice of generic Resource structs.
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
		// signals (creating, failed) carry the §4 phrase. Cross-ref signals
		// (orphan, past-retention) come from the issue enricher and overwrite
		// via FieldUpdates.
		rawStatus := ""
		if snapshot.Status != nil {
			rawStatus = *snapshot.Status
		}
		status := computeDBCSnapPhrase(rawStatus)

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
			Status: status,
			Fields: map[string]string{
				"snapshot_id":          snapshotID,
				"cluster_id":           clusterID,
				"status":               status,
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
