// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// FetchRDSInstancesPage fetches a single page of RDS instances.
func FetchRDSInstancesPage(ctx context.Context, api RDSDescribeDBInstancesAPI, continuationToken string) (resource.FetchResult, error) {
	return FetchRDSInstancesPageAt(ctx, api, continuationToken, time.Now())
}

// FetchRDSInstancesPageAt is the clock-injectable implementation behind
// FetchRDSInstancesPage. now decides how many days remain on each instance's
// CA certificate, so tests can pin the boundary the way acm.go does.
func FetchRDSInstancesPageAt(ctx context.Context, api RDSDescribeDBInstancesAPI, continuationToken string, now time.Time) (resource.FetchResult, error) {
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

		findings, attentionDetails := computeDBIFindings(db, now)
		statusPhrase := domain.StatusPhrase(findings)
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
			RawStruct:        db,
			AttentionDetails: attentionDetails,
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

// transitionalStatusSet contains RDS instance statuses that indicate a
// transitional (Warning) state. These show a pending modification key suffix when applicable.
var transitionalStatusSet = map[string]struct{}{
	"creating": {}, "modifying": {}, "backing-up": {}, "rebooting": {},
	"renaming": {}, "resetting-master-credentials": {}, "starting": {},
	"stopping": {}, "upgrading": {}, "maintenance": {},
	"configuring-enhanced-monitoring": {}, "configuring-iam-database-auth": {},
	"configuring-log-exports": {}, "converting-to-vpc": {}, "moving-to-vpc": {},
	"storage-optimization": {}, "deleting": {},
}

// computeDBIFindings returns the findings for an RDS DB instance plus the
// supporting AttentionDetail rows keyed by the finding that owns them.
// Broken statuses take priority in the status column; transitional statuses
// are Warn; the security-posture pack (rds_posture.go) is evaluated
// independently and stacks on top of whatever the lifecycle status says,
// except on an instance that is being torn down.
func computeDBIFindings(db rdstypes.DBInstance, now time.Time) ([]domain.Finding, map[domain.FindingCode]domain.AttentionDetail) {
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
	postureFindings, postureDetails := dbiPostureFindings(db, now)
	// An instance on its way out has no posture worth reporting.
	if isTeardownStatus(status) {
		postureFindings, postureDetails = nil, nil
	}

	if code, ok := brokenMap[status]; ok {
		lead := []domain.Finding{wave1Finding(code, catalog.Phrase(code), domain.SevBroken)}
		return append(lead, postureFindings...), postureDetails
	}
	if _, ok := transitionalStatusSet[status]; ok {
		key := firstNonEmptyPendingModifiedValueKey(db.PendingModifiedValues)
		phrase := status
		if key != "" {
			phrase = status + ": " + key
		}
		lead := []domain.Finding{{Code: CodeDBITransitional, Phrase: phrase, Severity: domain.SevWarn, Source: "wave1"}}
		return append(lead, postureFindings...), postureDetails
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
		return append(findings, postureFindings...), postureDetails
	}
	// Unknown status: no lifecycle finding, so the row reads healthy on its
	// lifecycle and the posture pack still decides its colour. Emitting a
	// warning for a status a9s does not recognise would flag every future RDS
	// state AWS adds, most of which are benign.
	return postureFindings, postureDetails
}

// dbiPostureFindings evaluates the shared RDS posture predicates plus the
// CA-certificate countdown against an instance.
func dbiPostureFindings(db rdstypes.DBInstance, now time.Time) ([]domain.Finding, map[domain.FindingCode]domain.AttentionDetail) {
	engine := aws.ToString(db.Engine)
	findings, details := rdsPostureFindings(rdsPosture{
		Engine:                  engine,
		MultiAZ:                 db.MultiAZ,
		IsReadReplica:           db.ReadReplicaSourceDBInstanceIdentifier != nil,
		AutoMinorVersionUpgrade: db.AutoMinorVersionUpgrade,
		IAMAuthEnabled:          db.IAMDatabaseAuthenticationEnabled,
		MasterUsername:          db.MasterUsername,
		// Aurora instances take their AZ topology from the cluster; the
		// per-instance MultiAZ flag is not the operator's lever there.
		SkipSingleAZ: strings.HasPrefix(strings.ToLower(engine), "aurora"),
	}, dbiPostureCodes)

	if db.CertificateDetails != nil {
		caFinding, caRows := rdsCACertFinding(
			aws.ToString(db.CertificateDetails.CAIdentifier),
			db.CertificateDetails.ValidTill, now,
			CodeDBICACertExpiring,
		)
		if caFinding != nil {
			findings = append(findings, *caFinding)
			if details == nil {
				details = map[domain.FindingCode]domain.AttentionDetail{}
			}
			details[CodeDBICACertExpiring] = domain.AttentionDetail{Rows: caRows}
		}
	}
	return findings, details
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
