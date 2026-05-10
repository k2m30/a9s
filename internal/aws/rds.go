package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/internal/domain"
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
	resource.RegisterDefaultNavFields("dbi", []resource.NavigableField{
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

		findings := computeDBIFindings(db)
		statusPhrase := phraseFromFindings(findings)
		if statusPhrase == "" {
			// Unknown / undocumented RDS status: keep the raw value visible in
			// the table so colorDBI's legacy classifier (which inspects
			// Fields["status"] and the public/encrypted/deletion-protection
			// overlays) keeps working. "available" with zero warnings legitimately
			// returns "" and is intentionally skipped.
			if raw := aws.ToString(db.DBInstanceStatus); raw != "" && raw != "available" {
				statusPhrase = raw
			}
		}

		r := resource.Resource{
			ID:       dbIdentifier,
			Name:     dbIdentifier,
			Findings: findings,
			Fields: map[string]string{
				"db_identifier":           dbIdentifier,
				"engine":                  engine,
				"engine_version":          engineVersion,
				"status":                  statusPhrase,
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

// phraseFromFindings returns the display phrase for a slice of findings.
// Returns "" for empty, the single phrase, or "top (+N)" for multiple.
func phraseFromFindings(findings []domain.Finding) string {
	if len(findings) == 0 {
		return ""
	}
	if len(findings) == 1 {
		return findings[0].Phrase
	}
	return fmt.Sprintf("%s (+%d)", findings[0].Phrase, len(findings)-1)
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

// computeDBIFindings returns a []domain.Finding for the given RDS DB instance.
// Broken statuses take priority; transitional statuses are Warn; available
// instances accumulate Warn findings.
func computeDBIFindings(db rdstypes.DBInstance) []domain.Finding {
	status := aws.ToString(db.DBInstanceStatus)

	// Broken statuses. `stopped` belongs here per the catalog colorDBI legacy
	// classification (an instance you must restart before it can serve traffic
	// is operationally broken, not transitional).
	brokenMap := map[string]domain.FindingCode{
		"failed":                              CodeDBIFailed,
		"storage-full":                        CodeDBIStorageFull,
		"incompatible-network":                CodeDBIIncompatibleNetwork,
		"incompatible-option-group":           CodeDBIIncompatibleOptionGroup,
		"incompatible-parameters":             CodeDBIIncompatibleParameters,
		"incompatible-restore":                CodeDBIIncompatibleRestore,
		"restore-error":                       CodeDBIRestoreError,
		"inaccessible-encryption-credentials": CodeDBIEncryptionKeyUnavailable,
		"stopped":                             CodeDBIStopped,
	}
	brokenPhraseMap := map[string]string{
		"failed":                              "failed",
		"storage-full":                        "storage-full",
		"incompatible-network":                "incompatible-network",
		"incompatible-option-group":           "incompatible-option-group",
		"incompatible-parameters":             "incompatible-parameters",
		"incompatible-restore":                "incompatible-restore",
		"restore-error":                       "restore-error",
		"inaccessible-encryption-credentials": "encryption key unavailable",
		"stopped":                             "stopped",
	}
	if code, ok := brokenMap[status]; ok {
		return []domain.Finding{{Code: code, Phrase: brokenPhraseMap[status], Severity: domain.SevBroken, Source: "wave1"}}
	}
	if _, ok := transitionalStatusSet[status]; ok {
		key := firstNonEmptyPendingModifiedValueKey(db.PendingModifiedValues)
		phrase := status
		if key != "" {
			phrase = status + ": " + key
		}
		return []domain.Finding{{Code: CodeDBITransitional, Phrase: phrase, Severity: domain.SevWarn, Source: "wave1"}}
	}
	if status == "available" {
		var findings []domain.Finding
		if db.BackupRetentionPeriod != nil && *db.BackupRetentionPeriod == 0 {
			findings = append(findings, domain.Finding{Code: CodeDBINoAutomatedBackups, Phrase: "no automated backups", Severity: domain.SevWarn, Source: "wave1"})
		}
		if db.PubliclyAccessible != nil && *db.PubliclyAccessible {
			findings = append(findings, domain.Finding{Code: CodeDBIPubliclyAccessible, Phrase: "publicly accessible", Severity: domain.SevWarn, Source: "wave1"})
		}
		if db.StorageEncrypted != nil && !*db.StorageEncrypted {
			findings = append(findings, domain.Finding{Code: CodeDBIUnencryptedStorage, Phrase: "unencrypted storage", Severity: domain.SevWarn, Source: "wave1"})
		}
		if db.DeletionProtection != nil && !*db.DeletionProtection {
			findings = append(findings, domain.Finding{Code: CodeDBIDeletionProtectionOff, Phrase: "deletion protection off", Severity: domain.SevWarn, Source: "wave1"})
		}
		return findings
	}
	// Unknown status: do NOT emit a wave1 finding. The fetcher falls back to
	// the raw RDS status string for Fields["status"], and colorDBI's legacy
	// classifier handles severity (preserves the pre-PR-03e overlay semantics
	// for new/unforeseen states such as `incompatible-*` / `inaccessible-*`
	// variants the broken map does not enumerate).
	return nil
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
