package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// dbiTransitionalStatuses is the Warning-bucket transitional set from
// docs/resources/dbi.md §3.1.
var dbiTransitionalStatuses = map[string]struct{}{
	"creating": {}, "modifying": {}, "backing-up": {}, "rebooting": {},
	"renaming": {}, "resetting-master-credentials": {}, "starting": {},
	"stopping": {}, "upgrading": {}, "maintenance": {},
	"configuring-enhanced-monitoring":  {},
	"configuring-iam-database-auth":    {},
	"configuring-log-exports":          {},
	"converting-to-vpc":                {},
	"moving-to-vpc":                    {},
	"storage-optimization":             {},
}

// dbiBrokenStatuses is the Broken-bucket set from docs/resources/dbi.md §3.1.
var dbiBrokenStatuses = map[string]struct{}{
	"failed":                              {},
	"storage-full":                        {},
	"incompatible-network":                {},
	"incompatible-option-group":           {},
	"incompatible-parameters":             {},
	"incompatible-restore":                {},
	"inaccessible-encryption-credentials": {},
	"restore-error":                       {},
}

// dbiPendingModifiedValuesFirstKey returns the name of the first non-empty
// field in PendingModifiedValues using the deterministic priority order from
// docs/resources/dbi.md §4 / impl-plan §3.5.
func dbiPendingModifiedValuesFirstKey(pmv *rdstypes.PendingModifiedValues) string {
	if pmv == nil {
		return ""
	}
	if pmv.DBInstanceClass != nil && *pmv.DBInstanceClass != "" {
		return "DBInstanceClass"
	}
	if pmv.AllocatedStorage != nil {
		return "AllocatedStorage"
	}
	if pmv.MasterUserPassword != nil && *pmv.MasterUserPassword != "" {
		return "MasterUserPassword"
	}
	if pmv.Port != nil {
		return "Port"
	}
	if pmv.BackupRetentionPeriod != nil {
		return "BackupRetentionPeriod"
	}
	if pmv.MultiAZ != nil {
		return "MultiAZ"
	}
	if pmv.EngineVersion != nil && *pmv.EngineVersion != "" {
		return "EngineVersion"
	}
	if pmv.LicenseModel != nil && *pmv.LicenseModel != "" {
		return "LicenseModel"
	}
	if pmv.Iops != nil {
		return "Iops"
	}
	if pmv.DBInstanceIdentifier != nil && *pmv.DBInstanceIdentifier != "" {
		return "DBInstanceIdentifier"
	}
	if pmv.StorageType != nil && *pmv.StorageType != "" {
		return "StorageType"
	}
	if pmv.CACertificateIdentifier != nil && *pmv.CACertificateIdentifier != "" {
		return "CACertificateIdentifier"
	}
	if pmv.DBSubnetGroupName != nil && *pmv.DBSubnetGroupName != "" {
		return "DBSubnetGroupName"
	}
	if pmv.PendingCloudwatchLogsExports != nil {
		return "PendingCloudwatchLogsExports"
	}
	if len(pmv.ProcessorFeatures) > 0 {
		return "ProcessorFeatures"
	}
	if pmv.IAMDatabaseAuthenticationEnabled != nil {
		return "IAMDatabaseAuthenticationEnabled"
	}
	if pmv.AutomationMode != "" {
		return "AutomationMode"
	}
	if pmv.ResumeFullAutomationModeTime != nil {
		return "ResumeFullAutomationModeTime"
	}
	if pmv.StorageThroughput != nil {
		return "StorageThroughput"
	}
	if pmv.Engine != nil && *pmv.Engine != "" {
		return "Engine"
	}
	if pmv.DedicatedLogVolume != nil {
		return "DedicatedLogVolume"
	}
	if pmv.MultiTenant != nil {
		return "MultiTenant"
	}
	return ""
}

// deriveDbiStatus returns the Status string for a DB instance per
// docs/resources/dbi.md §4 — Issue Visualization. Healthy rows get blank;
// Warning/Broken keywords pass through (with one rewrite for the encryption
// case); transitional statuses append the first non-empty PendingModifiedValues
// key; configuration-policy misses render a short cause string by precedence.
// This centralizes the §4 List text column for every Wave 1 signal.
//
// Precedence (highest first):
//  1. inaccessible-encryption-credentials → "encryption key unavailable"
//  2. broken status set → status verbatim
//  3. transitional status set → "<status>: <first PMV key>" or bare "<status>"
//  4. available + config warning → warning string by precedence
//  5. available + no warning → "" (silence)
//  6. any other non-empty status → pass through defensively
func deriveDbiStatus(db rdstypes.DBInstance) string {
	status := ""
	if db.DBInstanceStatus != nil {
		status = *db.DBInstanceStatus
	}

	// 1. Special broken case — rewrite to human-readable text.
	if status == "inaccessible-encryption-credentials" {
		return "encryption key unavailable"
	}

	// 2. Broken statuses — pass through keyword verbatim.
	if _, ok := dbiBrokenStatuses[status]; ok {
		return status
	}

	// 3. Transitional statuses — append first non-empty PendingModifiedValues key.
	if _, ok := dbiTransitionalStatuses[status]; ok {
		if key := dbiPendingModifiedValuesFirstKey(db.PendingModifiedValues); key != "" {
			return status + ": " + key
		}
		return status
	}

	// 4. Any other non-available keyword — pass through defensively (AWS may
	// add new statuses we have not yet classified).
	if status != "" && status != "available" {
		return status
	}

	// 5. status == "available": check configuration-policy warnings in precedence order.
	if db.BackupRetentionPeriod != nil && *db.BackupRetentionPeriod == 0 {
		return "no automated backups"
	}
	if db.PubliclyAccessible != nil && *db.PubliclyAccessible {
		return "publicly accessible"
	}
	if db.StorageEncrypted != nil && !*db.StorageEncrypted {
		return "unencrypted storage"
	}
	if db.DeletionProtection != nil && !*db.DeletionProtection {
		return "deletion protection off"
	}

	// 6. Healthy — silence is the UX.
	return ""
}

func init() {
	resource.RegisterFieldKeys("dbi", []string{"db_identifier", "engine", "engine_version", "status", "class", "endpoint", "multi_az", "arn", "publicly_accessible", "storage_encrypted", "deletion_protection", "backup_retention_period", "cis_flags"})

	resource.RegisterRelated("dbi", []resource.RelatedDef{
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkDbiSG},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkDbiKMS},
		{TargetType: "subnet", DisplayName: "Subnets", Checker: checkDbiSubnets},
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkDbiAlarm, NeedsTargetCache: true},
		{TargetType: "rds-snap", DisplayName: "RDS Snapshots", Checker: checkDbiRDSSnap, NeedsTargetCache: true},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkDBILogs, NeedsTargetCache: true},
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkDbiVPC},
		{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkDbiSecrets, NeedsTargetCache: true},
		{TargetType: "dbc", DisplayName: "RDS Clusters", Checker: checkDbiDBC, NeedsTargetCache: true},
		{TargetType: "role", DisplayName: "IAM Roles", Checker: checkDbiRole},
		{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkDbiENI},
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

		// Compute CIS compliance flags.
		var cisFlags []string
		if publiclyAccessible == "true" {
			cisFlags = append(cisFlags, "PUB")
		}
		if storageEncrypted == "false" {
			cisFlags = append(cisFlags, "UNENC")
		}
		if backupRetentionPeriod == "0" {
			cisFlags = append(cisFlags, "NOBKP")
		}
		if deletionProtection == "false" {
			cisFlags = append(cisFlags, "NOPROT")
		}
		cisFlagsVal := strings.Join(cisFlags, "|")

		r := resource.Resource{
			ID:     dbIdentifier,
			Name:   dbIdentifier,
			Status: deriveDbiStatus(db),
			Fields: map[string]string{
				"db_identifier":           dbIdentifier,
				"engine":                  engine,
				"engine_version":          engineVersion,
				"status":                  status,
				"class":                   class,
				"endpoint":                endpoint,
				"multi_az":                multiAZ,
				"arn":                     aws.ToString(db.DBInstanceArn),
				"publicly_accessible":     publiclyAccessible,
				"storage_encrypted":       storageEncrypted,
				"deletion_protection":     deletionProtection,
				"backup_retention_period": backupRetentionPeriod,
				"cis_flags":               cisFlagsVal,
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
