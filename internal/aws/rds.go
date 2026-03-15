package aws

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/service/rds"

	"github.com/k2m30/a9s/internal/resource"
)

// FetchRDSInstances calls the RDS DescribeDBInstances API and converts the
// response into a slice of generic Resource structs.
func FetchRDSInstances(ctx context.Context, api RDSDescribeDBInstancesAPI) ([]resource.Resource, error) {
	output, err := api.DescribeDBInstances(ctx, &rds.DescribeDBInstancesInput{})
	if err != nil {
		return nil, err
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

		// Build DetailData
		detail := map[string]string{
			"DB Identifier":  dbIdentifier,
			"Engine":         engine,
			"Engine Version": engineVersion,
			"Status":         status,
			"Class":          class,
			"Endpoint":       endpoint,
			"Multi-AZ":       multiAZ,
		}

		// Port
		port := ""
		if db.Endpoint != nil && db.Endpoint.Port != nil {
			port = fmt.Sprintf("%d", *db.Endpoint.Port)
		}
		detail["Port"] = port

		// DB Name
		dbName := ""
		if db.DBName != nil {
			dbName = *db.DBName
		}
		detail["DB Name"] = dbName

		// Storage Type
		storageType := ""
		if db.StorageType != nil {
			storageType = *db.StorageType
		}
		detail["Storage Type"] = storageType

		// Allocated Storage
		if db.AllocatedStorage != nil {
			detail["Allocated Storage"] = fmt.Sprintf("%d GB", *db.AllocatedStorage)
		} else {
			detail["Allocated Storage"] = ""
		}

		// VPC
		vpc := ""
		if db.DBSubnetGroup != nil && db.DBSubnetGroup.VpcId != nil {
			vpc = *db.DBSubnetGroup.VpcId
		}
		detail["VPC"] = vpc

		// Availability Zone
		az := ""
		if db.AvailabilityZone != nil {
			az = *db.AvailabilityZone
		}
		detail["Availability Zone"] = az

		// Build RawJSON
		rawJSON := ""
		if jsonBytes, err := json.MarshalIndent(db, "", "  "); err == nil {
			rawJSON = string(jsonBytes)
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
			DetailData: detail,
			RawJSON:    rawJSON,
		}

		resources = append(resources, r)
	}

	return resources, nil
}
