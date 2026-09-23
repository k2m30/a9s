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

		r := resource.Resource{
			ID:       dbIdentifier,
			Name:     dbIdentifier,
			Findings: findings,
			Fields: map[string]string{
				"db_identifier":           dbIdentifier,
				"engine":                  engine,
				"engine_version":          engineVersion,
				"status":                  statusPhrase,
				"status_raw":              aws.ToString(db.DBInstanceStatus),
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

// computeDBIFindings returns the findings for an RDS DB instance plus the
// supporting AttentionDetail rows keyed by the finding that owns them.
// The lifecycle status leads; the security-posture pack (rds_posture.go) is
// evaluated independently and stacks on top of whatever the lifecycle status
// says, except on an instance that is being torn down.
func computeDBIFindings(db rdstypes.DBInstance, now time.Time) ([]domain.Finding, map[domain.FindingCode]domain.AttentionDetail) {
	status := aws.ToString(db.DBInstanceStatus)
	postureFindings, postureDetails := dbiPostureFindings(db, now)
	// An instance on its way out has no posture worth reporting.
	if isTeardownStatus(status) {
		postureFindings, postureDetails = nil, nil
	}

	transitionalPhrase := status
	if key := firstNonEmptyPendingModifiedValueKey(db.PendingModifiedValues); key != "" {
		transitionalPhrase = status + ": " + key
	}
	if lead, ok := dbiLifecycle.finding(status, transitionalPhrase); ok {
		return append([]domain.Finding{lead}, postureFindings...), postureDetails
	}
	if status != "available" {
		return postureFindings, postureDetails
	}

	var findings []domain.Finding
	if db.BackupRetentionPeriod != nil && *db.BackupRetentionPeriod == 0 {
		findings = append(findings, wave1Finding(CodeDBINoAutomatedBackups))
	}
	if db.PubliclyAccessible != nil && *db.PubliclyAccessible {
		findings = append(findings, wave1Finding(CodeDBIPubliclyAccessible))
	}
	if db.StorageEncrypted != nil && !*db.StorageEncrypted {
		findings = append(findings, wave1Finding(CodeDBIUnencryptedStorage))
	}
	if db.DeletionProtection != nil && !*db.DeletionProtection {
		findings = append(findings, wave1Finding(CodeDBIDeletionProtectionOff))
	}
	return append(findings, postureFindings...), postureDetails
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
		)
		if caFinding != nil {
			findings = append(findings, *caFinding)
			if details == nil {
				details = map[domain.FindingCode]domain.AttentionDetail{}
			}
			details[caFinding.Code] = domain.AttentionDetail{Rows: caRows}
		}
	}
	return findings, details
}

// firstNonEmptyPendingModifiedValueKey inspects PendingModifiedValues fields in a fixed order
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
