package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/docdb"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("docdb-snap", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchDocDBClusterSnapshots(ctx, c.DocDB)
	})
	resource.RegisterFieldKeys("docdb-snap", []string{"snapshot_id", "cluster_id", "status", "engine", "snapshot_type", "snapshot_create_time", "storage_type"})
}

// FetchDocDBClusterSnapshots calls the DocumentDB DescribeDBClusterSnapshots API and converts the
// response into a slice of generic Resource structs.
func FetchDocDBClusterSnapshots(ctx context.Context, api DocDBDescribeDBClusterSnapshotsAPI) ([]resource.Resource, error) {
	output, err := api.DescribeDBClusterSnapshots(ctx, &docdb.DescribeDBClusterSnapshotsInput{})
	if err != nil {
		return nil, err
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

		status := ""
		if snapshot.Status != nil {
			status = *snapshot.Status
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
			snapshotCreateTime = snapshot.SnapshotCreateTime.Format("2006-01-02T15:04:05Z07:00")
		}

		storageType := ""
		if snapshot.StorageType != nil {
			storageType = *snapshot.StorageType
		}

		detail := map[string]string{
			"Snapshot ID": snapshotID,
			"Cluster ID":  clusterID,
			"Status":      status,
			"Engine":      engine,
			"Type":        snapshotType,
			"Created":     snapshotCreateTime,
			"Storage":     storageType,
		}

		if snapshot.DBClusterSnapshotArn != nil {
			detail["ARN"] = *snapshot.DBClusterSnapshotArn
		}
		if snapshot.EngineVersion != nil {
			detail["Engine Version"] = *snapshot.EngineVersion
		}
		if snapshot.MasterUsername != nil {
			detail["Master Username"] = *snapshot.MasterUsername
		}
		if snapshot.VpcId != nil {
			detail["VPC ID"] = *snapshot.VpcId
		}
		if snapshot.Port != nil {
			detail["Port"] = fmt.Sprintf("%d", *snapshot.Port)
		}

		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(snapshot, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
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
			},
			DetailData: detail,
			RawJSON:    rawJSON,
			RawStruct:  snapshot,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
