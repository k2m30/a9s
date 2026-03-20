package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/rds"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("rds-snap", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchRDSSnapshots(ctx, c.RDS)
	})
	resource.RegisterFieldKeys("rds-snap", []string{"snapshot_id", "db_instance", "status", "engine", "snapshot_type", "created"})
}

// FetchRDSSnapshots calls the RDS DescribeDBSnapshots API and converts the
// response into a slice of generic Resource structs.
func FetchRDSSnapshots(ctx context.Context, api RDSDescribeDBSnapshotsAPI) ([]resource.Resource, error) {
	output, err := api.DescribeDBSnapshots(ctx, &rds.DescribeDBSnapshotsInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching RDS snapshots: %w", err)
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
			created = snap.SnapshotCreateTime.Format("2006-01-02T15:04:05Z07:00")
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
			},
			RawStruct:  snap,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
