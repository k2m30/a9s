package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/internal/resource"
)

func init() {
	resource.RegisterFieldKeys("dbi", []string{"db_identifier", "engine", "engine_version", "status", "class", "endpoint", "multi_az", "arn", "publicly_accessible", "storage_encrypted", "deletion_protection", "backup_retention_period"})

	resource.RegisterRelated("dbi", []resource.RelatedDef{
		{TargetType: "sg", DisplayName: "Security Groups", Checker: checkDbiSG},
		{TargetType: "kms", DisplayName: "KMS Key", Checker: checkDbiKMS},
		{TargetType: "subnet", DisplayName: "Subnets", Checker: checkDbiSubnets},
		{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkDbiAlarm, NeedsTargetCache: true},
		{TargetType: "dbi-snap", DisplayName: "DB Instance Snapshots", Checker: checkDbiDBISnap, NeedsTargetCache: true},
		{TargetType: "logs", DisplayName: "Log Groups", Checker: checkDBILogs, NeedsTargetCache: true},
		{TargetType: "vpc", DisplayName: "VPC", Checker: checkDbiVPC},
		{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkDbiSecrets, NeedsTargetCache: true},
		{TargetType: "dbc", DisplayName: "RDS Clusters", Checker: checkDbiDBC, NeedsTargetCache: true},
		{TargetType: "role", DisplayName: "IAM Roles", Checker: checkDbiRole},
		{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkDbiENI},
		{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkDbiCTEvents, NeedsTargetCache: true},
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
//
// Engine coverage: per the AWS SDK Go v2 docstring on
// rds.DescribeDBInstances ("Describes provisioned RDS instances. ... This
// operation can also return information for Amazon Neptune DB instances and
// Amazon DocumentDB instances."), the rds-side call returns RDS + Neptune +
// DocDB instances. No companion docdb-side fetcher is needed for the dbi
// resource type — single source covers all engines. See
// docs/resources/dbi.md §1 Coverage for the user-facing claim.
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

		computedStatus, computedIssues := computeDBIStatusAndIssues(db)

		r := resource.Resource{
			ID:     dbIdentifier,
			Name:   dbIdentifier,
			Status: computedStatus,
			Issues: computedIssues,
			Fields: map[string]string{
				"db_identifier":           dbIdentifier,
				"engine":                  engine,
				"engine_version":          engineVersion,
				"status":                  computedStatus,
				"class":                   class,
				"endpoint":                endpoint,
				"multi_az":                multiAZ,
				"arn":                     aws.ToString(db.DBInstanceArn),
				"publicly_accessible":     publiclyAccessible,
				"storage_encrypted":       storageEncrypted,
				"deletion_protection":     deletionProtection,
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

// brokenStatusPhrase maps RDS instance statuses that represent hard failures to
// their display phrases. "inaccessible-encryption-credentials" is remapped per spec §4.
var brokenStatusPhrase = map[string]string{
	"failed":                              "failed",
	"storage-full":                        "storage-full",
	"incompatible-network":                "incompatible-network",
	"incompatible-option-group":           "incompatible-option-group",
	"incompatible-parameters":             "incompatible-parameters",
	"incompatible-restore":                "incompatible-restore",
	"restore-error":                       "restore-error",
	"inaccessible-encryption-credentials": "encryption key unavailable",
}

// transitionalStatusSet contains RDS instance statuses that indicate a
// transitional (Warning) state. These show a pending modification key suffix when applicable.
var transitionalStatusSet = map[string]struct{}{
	"creating": {}, "modifying": {}, "backing-up": {}, "rebooting": {},
	"renaming": {}, "resetting-master-credentials": {}, "starting": {},
	"stopping": {}, "upgrading": {}, "maintenance": {},
	"configuring-enhanced-monitoring": {}, "configuring-iam-database-auth": {},
	"configuring-log-exports": {}, "converting-to-vpc": {}, "moving-to-vpc": {},
	"storage-optimization": {},
}

// computeDBIStatusAndIssues returns the top S4 phrase (with `(+N)` suffix when
// multiple warnings stack) AND the full ordered list of every active issue
// phrase. The second return feeds Resource.Issues so the detail view can
// render all active warnings individually (spec rule 7: "every finding
// individually visible"). Broken / transitional / healthy states each produce
// at most one phrase.
func computeDBIStatusAndIssues(db rdstypes.DBInstance) (string, []string) {
	status := aws.ToString(db.DBInstanceStatus)
	if phrase, ok := brokenStatusPhrase[status]; ok {
		return phrase, []string{phrase}
	}
	if _, ok := transitionalStatusSet[status]; ok {
		key := firstNonEmptyPendingModifiedValueKey(db.PendingModifiedValues)
		if key == "" {
			return status, []string{status}
		}
		phrase := status + ": " + key
		return phrase, []string{phrase}
	}
	if status == "available" {
		var warnings []string
		if db.BackupRetentionPeriod != nil && *db.BackupRetentionPeriod == 0 {
			warnings = append(warnings, "no automated backups")
		}
		if db.PubliclyAccessible != nil && *db.PubliclyAccessible {
			warnings = append(warnings, "publicly accessible")
		}
		if db.StorageEncrypted != nil && !*db.StorageEncrypted {
			warnings = append(warnings, "unencrypted storage")
		}
		if db.DeletionProtection != nil && !*db.DeletionProtection {
			warnings = append(warnings, "deletion protection off")
		}
		switch len(warnings) {
		case 0:
			return "", nil
		case 1:
			return warnings[0], warnings
		default:
			return fmt.Sprintf("%s (+%d)", warnings[0], len(warnings)-1), warnings
		}
	}
	return status, []string{status} // unknown status — pass through
}

// firstNonEmptyPendingModifiedValueKey inspects PendingModifiedValues fields in spec-defined order
// and returns the name of the first non-nil/non-empty field.
func firstNonEmptyPendingModifiedValueKey(pmv *rdstypes.PendingModifiedValues) string {
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
