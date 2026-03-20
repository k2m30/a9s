package aws

import (
	"context"
	"encoding/json"
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
		return nil, err
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

		detail := map[string]string{
			"Snapshot ID": snapshotID,
			"DB Instance": dbInstance,
			"Status":      status,
			"Engine":      engine,
			"Type":        snapshotType,
			"Created":     created,
		}

		if snap.DBSnapshotArn != nil {
			detail["ARN"] = *snap.DBSnapshotArn
		}
		if snap.EngineVersion != nil {
			detail["Engine Version"] = *snap.EngineVersion
		}
		if snap.StorageType != nil {
			detail["Storage Type"] = *snap.StorageType
		}
		if snap.AvailabilityZone != nil {
			detail["Availability Zone"] = *snap.AvailabilityZone
		}

		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(snap, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
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
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  snap,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
