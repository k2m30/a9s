package aws

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("rds-snap", []string{"snapshot_id", "db_instance", "status", "engine", "snapshot_type", "created", "encrypted"})

	resource.RegisterRelated("rds-snap", []resource.RelatedDef{
		{TargetType: "dbi", DisplayName: "DB Instances", Checker: checkRDSSnapDBI, NeedsTargetCache: true},
		{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkRDSSnapKMS, NeedsTargetCache: true},
		{TargetType: "backup", DisplayName: "AWS Backups", Checker: checkRDSSnapBackup},
		{TargetType: "dbc", DisplayName: "DB Clusters", Checker: checkRDSSnapDBC},
	})

	resource.RegisterNavigableFields("rds-snap", []resource.NavigableField{
		{FieldPath: "DBInstanceIdentifier", TargetType: "dbi"},
		{FieldPath: "KmsKeyId", TargetType: "kms"},
		{FieldPath: "VpcId", TargetType: "vpc"},
	})

	resource.RegisterPaginated("rds-snap", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchRDSSnapshotsPage(ctx, c.RDS, continuationToken)
	})
}

// FetchRDSSnapshots calls the RDS DescribeDBSnapshots API and converts the
// response into a slice of generic Resource structs.
func FetchRDSSnapshots(ctx context.Context, api RDSDescribeDBSnapshotsAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchRDSSnapshotsPage(ctx, api, token)
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

// FetchRDSSnapshotsPage fetches a single page of RDS snapshots.
func FetchRDSSnapshotsPage(ctx context.Context, api RDSDescribeDBSnapshotsAPI, continuationToken string) (resource.FetchResult, error) {
	input := &rds.DescribeDBSnapshotsInput{
		MaxRecords: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := api.DescribeDBSnapshots(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching RDS snapshots: %w", err)
	}

	var resources []resource.Resource

	for _, snap := range output.DBSnapshots {
		snapshotID := ""
		if snap.DBSnapshotIdentifier != nil {
			snapshotID = *snap.DBSnapshotIdentifier
		}

		dbInstance := ""
		if snap.DBInstanceIdentifier != nil {
			dbInstance = *snap.DBInstanceIdentifier
		}

		status := ""
		if snap.Status != nil {
			status = *snap.Status
		}

		engine := ""
		if snap.Engine != nil {
			engine = *snap.Engine
		}

		snapshotType := ""
		if snap.SnapshotType != nil {
			snapshotType = *snap.SnapshotType
		}

		created := ""
		if snap.SnapshotCreateTime != nil {
			created = snap.SnapshotCreateTime.Format("2006-01-02 15:04")
		}

		encrypted := "false"
		if snap.Encrypted != nil {
			encrypted = strconv.FormatBool(*snap.Encrypted)
		}

		r := resource.Resource{
			ID:     snapshotID,
			Name:   snapshotID,
			Status: status,
			Fields: map[string]string{
				"snapshot_id":   snapshotID,
				"db_instance":   dbInstance,
				"status":        status,
				"engine":        engine,
				"snapshot_type": snapshotType,
				"created":       created,
				"encrypted":     encrypted,
			},
			RawStruct: snap,
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
