package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("dbi", []string{"db_identifier", "engine", "engine_version", "status", "class", "endpoint", "multi_az", "arn", "publicly_accessible", "storage_encrypted", "deletion_protection", "backup_retention_period"})

	resource.RegisterRelated("dbi", []resource.RelatedDef{
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkDbiSG},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkDbiKMS},
		{TargetType: "subnet", DisplayName: "Subnets", Checker: checkDbiSubnets},
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkDbiAlarm, NeedsTargetCache: true},
		{TargetType: "rds-snap", DisplayName: "RDS Snapshots", Checker: checkDbiRDSSnap, NeedsTargetCache: true},
		{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkDbiSecrets},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkDBILogs, NeedsTargetCache: true},
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkDbiVPC},
	})

	// rdstypes.DBInstance: VpcSecurityGroups[].VpcSecurityGroupId, DBSubnetGroup.VpcId,
	// DBSubnetGroup.Subnets[].SubnetIdentifier, KmsKeyId
	resource.RegisterNavigableFields("dbi", []resource.NavigableField{
		{FieldPath: "VpcSecurityGroups.VpcSecurityGroupId", TargetType: "sg"},
		{FieldPath: "DBSubnetGroup.VpcId", TargetType: "vpc"},
		{FieldPath: "DBSubnetGroup.Subnets.SubnetIdentifier", TargetType: "subnet"},
		{FieldPath: "KmsKeyId", TargetType: "kms"},
	})

	resource.RegisterPaginated("dbi", func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
		c, ok := clients.(*ServiceClients)
		if !ok || c == nil {
			return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
		}
		return FetchRDSInstancesPage(ctx, c.RDS, continuationToken)
	})
}

// FetchRDSInstances calls the RDS DescribeDBInstances API and converts the
// response into a slice of generic Resource structs.
func FetchRDSInstances(ctx context.Context, api RDSDescribeDBInstancesAPI) ([]resource.Resource, error) {
	var all []resource.Resource
	token := ""
	for {
		result, err := FetchRDSInstancesPage(ctx, api, token)
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

// FetchRDSInstancesPage fetches a single page of RDS instances.
func FetchRDSInstancesPage(ctx context.Context, api RDSDescribeDBInstancesAPI, continuationToken string) (resource.FetchResult, error) {
	input := &rds.DescribeDBInstancesInput{
		MaxRecords: aws.Int32(DefaultPageSize),
	}
	if continuationToken != "" {
		input.Marker = &continuationToken
	}

	output, err := api.DescribeDBInstances(ctx, input)
	if err != nil {
		return resource.FetchResult{}, fmt.Errorf("fetching RDS instances: %w", err)
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

		publiclyAccessible := "false"
		if db.PubliclyAccessible != nil && *db.PubliclyAccessible {
			publiclyAccessible = "true"
		}

		storageEncrypted := "true"
		if db.StorageEncrypted != nil && !*db.StorageEncrypted {
			storageEncrypted = "false"
		}

		deletionProtection := "true"
		if db.DeletionProtection != nil && !*db.DeletionProtection {
			deletionProtection = "false"
		}

		backupRetentionPeriod := "0"
		if db.BackupRetentionPeriod != nil {
			backupRetentionPeriod = fmt.Sprintf("%d", *db.BackupRetentionPeriod)
		}

		r := resource.Resource{
			ID:     dbIdentifier,
			Name:   dbIdentifier,
			Status: status,
			Fields: map[string]string{
				"db_identifier":          dbIdentifier,
				"engine":                 engine,
				"engine_version":         engineVersion,
				"status":                 status,
				"class":                  class,
				"endpoint":               endpoint,
				"multi_az":               multiAZ,
				"arn":                    aws.ToString(db.DBInstanceArn),
				"publicly_accessible":    publiclyAccessible,
				"storage_encrypted":      storageEncrypted,
				"deletion_protection":    deletionProtection,
				"backup_retention_period": backupRetentionPeriod,
			},
			RawStruct: db,
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
