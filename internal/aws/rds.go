package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/rds"

	"github.com/k2m30/a9s/internal/resource"
)

func init() {
	resource.Register("dbi", func(ctx context.Context, clients interface{}) ([]resource.Resource, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return nil, fmt.Errorf("AWS clients not initialized")
		}
		return FetchRDSInstances(ctx, c.RDS)
	})
	resource.RegisterFieldKeys("dbi", []string{"db_identifier", "engine", "engine_version", "status", "class", "endpoint", "multi_az"})
}

// FetchRDSInstances calls the RDS DescribeDBInstances API and converts the
// response into a slice of generic Resource structs.
func FetchRDSInstances(ctx context.Context, api RDSDescribeDBInstancesAPI) ([]resource.Resource, error) {
	output, err := api.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{})
	if err != nil {
		return nil, fmt.Errorf("fetching RDS instances: %w", err)
	}

	var resources []resource.Resource

	for _, db := range output.DBInstances {
		dbIdentifier := ""
		if db.DBInstanceIdentifier != nil {
			dbIdentifier = *db.DBInstanceIdentifier
		}

		engine := ""
		if db.Engine != nil {
			engine = *db.Engine
		}

		engineVersion := ""
		if db.EngineVersion != nil {
			engineVersion = *db.EngineVersion
		}

		status := ""
		if db.DBInstanceStatus != nil {
			status = *db.DBInstanceStatus
		}

		class := ""
		if db.DBInstanceClass != nil {
			class = *db.DBInstanceClass
		}

		endpoint := ""
		if db.Endpoint != nil && db.Endpoint.Address != nil {
			endpoint = *db.Endpoint.Address
		}

		multiAZ := "No"
		if db.MultiAZ != nil && *db.MultiAZ {
			multiAZ = "Yes"
		}

		r := resource.Resource{
			ID:     dbIdentifier,
			Name:   dbIdentifier,
			Status: status,
			Fields: map[string]string{
				"db_identifier":  dbIdentifier,
				"engine":         engine,
				"engine_version": engineVersion,
				"status":         status,
				"class":          class,
				"endpoint":       endpoint,
				"multi_az":       multiAZ,
			},
			RawStruct:  db,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
