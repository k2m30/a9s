// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/consolelink"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func colorDBI(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	status := r.Fields["status"]
	stripped := stripFindingSuffix(status)
	switch stripped {
	case "failed", "storage-full", "restore-error", "stopped",
		"incompatible-network", "incompatible-option-group",
		"incompatible-parameters", "incompatible-restore",
		"encryption key unavailable":
		return domain.ColorBroken
	}
	if strings.HasPrefix(stripped, "incompatible-") || strings.HasPrefix(stripped, "inaccessible-") {
		return domain.ColorBroken
	}
	switch stripped {
	case "no automated backups", "publicly accessible",
		"unencrypted storage", "deletion protection off":
		return domain.ColorWarning
	}
	if stripped != "" && stripped != "available" && stripped != "maintenance scheduled" {
		if strings.Contains(stripped, ":") {
			return domain.ColorWarning
		}
		switch stripped {
		case "creating", "modifying", "backing-up", "rebooting",
			"renaming", "resetting-master-credentials", "starting",
			"stopping", "upgrading", "maintenance",
			"configuring-enhanced-monitoring", "configuring-iam-database-auth",
			"configuring-log-exports", "converting-to-vpc", "moving-to-vpc",
			"storage-optimization", "deleting":
			return domain.ColorWarning
		}
	}
	base := domain.ColorHealthy
	if r.Fields["publicly_accessible"] == "true" {
		if base < domain.ColorWarning {
			base = domain.ColorWarning
		}
	}
	if r.Fields["storage_encrypted"] == "false" {
		if base < domain.ColorWarning {
			base = domain.ColorWarning
		}
	}
	if r.Fields["deletion_protection"] == "false" {
		if base < domain.ColorWarning {
			base = domain.ColorWarning
		}
	}
	if r.Fields["backup_retention_period"] == "0" {
		if base < domain.ColorWarning {
			base = domain.ColorWarning
		}
	}
	return base
}

func colorS3(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return domain.ColorHealthy
}

func colorRedis(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	phrase := stripFindingSuffix(r.Fields["status"])
	switch phrase {
	case "create failed — see events":
		return domain.ColorBroken
	}
	switch phrase {
	case "creating — new group",
		"modifying — config change",
		"snapshotting — backup running",
		"deleting — teardown",
		"multi-AZ without auto-failover":
		return domain.ColorWarning
	}
	if strings.HasPrefix(phrase, "shard ") {
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

func colorDBC(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	phrase := stripFindingSuffix(r.Fields["status"])
	switch phrase {
	case "":
		return domain.ColorHealthy
	case "failed: cluster operation",
		"encryption key unreachable",
		"parameter group incompatible",
		"no writer: reads only":
		return domain.ColorBroken
	case "delete-protection off",
		"not encrypted at rest",
		"no automated backups":
		return domain.ColorWarning
	case "maintenance overdue":
		return domain.ColorHealthy
	}
	if strings.HasSuffix(phrase, ": in progress") {
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

func colorDDB(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	phrase := stripFindingSuffix(r.Fields["status"])
	switch phrase {
	case "":
		return domain.ColorHealthy
	case "creating", "updating", "deleting", "archiving":
		return domain.ColorWarning
	case "kms key inaccessible", "archived: kms key lost":
		return domain.ColorBroken
	case "point-in-time recovery disabled":
		return domain.ColorHealthy
	}
	return domain.ColorHealthy
}

func colorOpenSearch(r domain.Resource) domain.Color {
	if r.Fields["deleted"] == "true" {
		return domain.ColorDim
	}
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	stripped := stripFindingSuffix(r.Fields["status"])
	if strings.HasPrefix(stripped, "isolated:") || r.Fields["domain_processing_status"] == "Isolated" {
		return domain.ColorBroken
	}
	if strings.HasPrefix(stripped, "processing:") ||
		r.Fields["processing"] == "true" ||
		r.Fields["upgrade_processing"] == "true" {
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

func colorRedshift(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	phrase := stripFindingSuffix(r.Fields["status"])
	switch phrase {
	case "unavailable", "failed":
		return domain.ColorBroken
	}
	if len(phrase) >= len("broken:") && phrase[:len("broken:")] == "broken:" {
		return domain.ColorBroken
	}
	var base domain.Color
	switch r.Fields["cluster_status"] {
	case "available":
		base = domain.ColorHealthy
	case "creating", "modifying", "resizing", "rebooting", "renaming", "deleting":
		base = domain.ColorWarning
	case "incompatible-hsm", "incompatible-network", "incompatible-parameters",
		"incompatible-restore", "hardware-failure", "storage-full":
		base = domain.ColorBroken
	default:
		base = domain.ColorHealthy
	}
	if base == domain.ColorBroken {
		return domain.ColorBroken
	}
	switch r.Fields["cluster_availability_status"] {
	case "Unavailable", "Failed":
		return domain.ColorBroken
	case "Maintenance", "Modifying":
		if base == domain.ColorHealthy {
			base = domain.ColorWarning
		}
	}
	if base == domain.ColorBroken {
		return domain.ColorBroken
	}
	switch phrase {
	case "pending change queued", "maintenance deferred",
		"maintenance", "modifying",
		"publicly accessible", "unencrypted at rest":
		if base == domain.ColorHealthy {
			base = domain.ColorWarning
		}
	}
	if r.Fields["publicly_accessible"] == "true" && base == domain.ColorHealthy {
		base = domain.ColorWarning
	}
	if r.Fields["encrypted"] == "false" && base == domain.ColorHealthy {
		base = domain.ColorWarning
	}
	return base
}

func colorEFS(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	phrase := stripFindingSuffix(r.Fields["status"])
	switch phrase {
	case "":
		return domain.ColorHealthy
	case "error", "no mount targets", "mount target down":
		return domain.ColorBroken
	case "creating", "updating", "deleting":
		return domain.ColorWarning
	default:
		return domain.ColorHealthy
	}
}

func colorDBISnap(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	phrase := stripFindingSuffix(r.Fields["status"])
	if phrase == "failed" {
		return domain.ColorBroken
	}
	if strings.HasPrefix(phrase, "incompatible-") {
		return domain.ColorBroken
	}
	if phrase == "" || phrase == "available" {
		if r.Fields["encrypted"] == "false" {
			return domain.ColorWarning
		}
		return domain.ColorHealthy
	}
	return domain.ColorWarning
}

// colorDBCSnap classifies a dbc-snap row. Every Warning/Broken bucket here is
// backed by a domain.Finding — wave1 (failed, incompatible-*, creating,
// manual age > 365d, unencrypted) emitted by computeDBCSnapFindings /
// computeRDSDBClusterSnapshotFindings, or wave2 (orphan, past-retention)
// emitted by enrichDBCSnapCrossRef — colorFromAnyFinding always resolves
// first, so the phrase-parsing fallback below only classifies rows whose
// RawStruct predates a Findings-carrying fetch (e.g. cache replay of an
// older schema).
func colorDBCSnap(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	phrase := stripFindingSuffix(r.Fields["status"])
	if phrase == "failed" {
		return domain.ColorBroken
	}
	if strings.HasPrefix(phrase, "incompatible-") {
		return domain.ColorBroken
	}
	if phrase != "" && phrase != "available" {
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

var databasesTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:          "DB Instances",
		ShortName:     "dbi",
		Aliases:       []string{"dbi", "rds", "databases", "db-instances"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:Fields.arn",
		LifecycleKey:  "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "rds/home?region="+region+"#database:id="+url.PathEscape(r.ID)+";is-cluster=false")
		},
		Columns: []domain.Column{
			{Key: "db_identifier", Title: "DB Identifier", Width: 28, Sortable: true},
			{Key: "engine", Title: "Engine", Width: 12, Sortable: true},
			{Key: "engine_version", Title: "Version", Width: 10, Sortable: true},
			{Key: "status", Title: "Status", Width: 14, Sortable: true},
			{Key: "class", Title: "Class", Width: 16, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 40, Sortable: false},
			{Key: "multi_az", Title: "Multi-AZ", Width: 10, Sortable: true},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "dbi_events",
			Key:            "enter",
			ContextKeys:    map[string]string{"db_identifier": "ID"},
			DisplayNameKey: "db_identifier",
		}},
		Color: colorDBI,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchRDSInstancesPage(ctx, c.RDS, continuationToken)
		}),
		Wave2: IssueEnricher{Fn: EnrichDBIMaintenance, Priority: 10},
		FieldKeys: []string{
			"db_identifier", "engine", "engine_version", "status", "class", "endpoint",
			"multi_az", "arn", "publicly_accessible", "storage_encrypted",
			"deletion_protection", "backup_retention_period",
		},
		Related: []domain.RelatedDef{
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkDbiSG},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkDbiKMS},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkDbiSubnets},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkDbiAlarm, NeedsTargetCache: true, Truncated: true},
			{TargetType: "dbi-snap", DisplayName: "DB Instance Snapshots", Checker: checkDbiDBISnap, NeedsTargetCache: true, Truncated: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkDBILogs, NeedsTargetCache: true, Truncated: true},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkDbiVPC},
			{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkDbiSecrets, NeedsTargetCache: true, Truncated: true},
			{TargetType: "dbc", DisplayName: "RDS Clusters", Checker: checkDbiDBC},
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkDbiRole},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkDbiENI},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkDbiCTEvents, NeedsTargetCache: true},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VpcSecurityGroups.VpcSecurityGroupId", TargetType: "sg"},
			{FieldPath: "DBSubnetGroup.VpcId", TargetType: "vpc"},
			{FieldPath: "DBSubnetGroup.Subnets.SubnetIdentifier", TargetType: "subnet"},
			{FieldPath: "KmsKeyId", TargetType: "kms"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeDBIFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeDBIStorageFull, Phrase: "storage-full", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeDBIIncompatibleNetwork, Phrase: "incompatible-network", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeDBIIncompatibleOptionGroup, Phrase: "incompatible-option-group", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeDBIIncompatibleParameters, Phrase: "incompatible-parameters", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeDBIIncompatibleRestore, Phrase: "incompatible-restore", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeDBIRestoreError, Phrase: "restore-error", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeDBIEncryptionKeyUnavailable, Phrase: "encryption key unavailable", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeDBIStopped, Phrase: "stopped", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeDBITransitional, Phrase: "<status>: <pending field>", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBINoAutomatedBackups, Phrase: "no automated backups", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBIPubliclyAccessible, Phrase: "publicly accessible", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBIUnencryptedStorage, Phrase: "unencrypted storage", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBIDeletionProtectionOff, Phrase: "deletion protection off", Severity: domain.SevWarn, Source: "wave1"},
			{Code: dbiCodePendingMaintenance, Phrase: "maintenance scheduled", Severity: domain.SevWarn, Source: "wave2"},
			{Code: CodeDBISingleAZ, Phrase: "single-AZ", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBIMinorUpgradeOff, Phrase: "auto minor version upgrade off", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBIIAMAuthOff, Phrase: "IAM database authentication off", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBIDefaultMasterUser, Phrase: "default master username", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBICACertExpiring, Phrase: "server certificate expires in <N> days", Severity: domain.SevWarn, Source: "wave1"},
			{Code: dbiCodeEngineDeprecated, Phrase: "engine version deprecated", Severity: domain.SevBroken, Source: "wave2"},
		},
	},
	{
		Name:          "S3 Buckets",
		ShortName:     "s3",
		Aliases:       []string{"s3", "buckets"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Global(region, "s3/buckets/"+url.PathEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Bucket Name", Width: 40, Sortable: true},
			{Key: "creation_date", Title: "Creation Date", Width: 22, Sortable: true},
		},
		Children: []domain.ChildViewDef{{
			ChildType:      "s3_objects",
			Key:            "enter",
			ContextKeys:    map[string]string{"bucket": "ID"},
			DisplayNameKey: "bucket",
		}},
		Color: colorS3,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			// Related-panel contract (docs/resources/s3.md §2): lambda/sns/sqs
			// pivots must resolve non-zero when this bucket has a matching
			// notification target. Those checkers read Fields["notification_*"],
			// which can only be populated by GetBucketNotificationConfiguration
			// — so the list path must run it per-bucket. Accepts N+1 per page
			// (cheap API, typically ≤50 buckets per AWS account) in exchange
			// for having the notification pivots actually work.
			return FetchS3BucketsPageWithNotifications(ctx, c.S3, c.S3, continuationToken)
		}),
		Wave2: IssueEnricher{Fn: EnrichS3Posture, Priority: 100},
		FieldKeys: []string{
			"name",
			"bucket_name",
			"creation_date",
			"notification_lambda",
			"notification_sqs",
			"notification_sns",
		},
		Related: []domain.RelatedDef{
			{TargetType: "trail", DisplayName: "CloudTrail Trails", Checker: checkS3Trail, NeedsTargetCache: true, Truncated: true},
			{TargetType: "cf", DisplayName: "CloudFront", Checker: checkS3CF, NeedsTargetCache: true, Truncated: true},
			{TargetType: "lambda", DisplayName: "Lambda (notifications)", Checker: checkS3Lambda},
			{TargetType: "sns", DisplayName: "SNS (notifications)", Checker: checkS3SNS},
			{TargetType: "sqs", DisplayName: "SQS (notifications)", Checker: checkS3SQS},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkS3CFN, Truncated: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkS3KMS, Truncated: true},
			{TargetType: "s3", DisplayName: "Access Log Bucket", Checker: checkS3Logs, Truncated: true},
			{TargetType: "athena", DisplayName: "Athena WorkGroups", Checker: checkS3Athena, Truncated: true},
			{TargetType: "glue", DisplayName: "Glue Jobs", Checker: checkS3Glue, Truncated: true},
			{TargetType: "backup", DisplayName: "Backup", Checker: checkS3Backup, Truncated: true},
			{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkS3EBRule, Truncated: true},
			{TargetType: "r53", DisplayName: "Route 53", Checker: checkS3R53, Truncated: true},
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkS3Role, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("s3")},
		},
		DetailEnrich:           enrichS3,
		IssueEnricherFieldKeys: []string{"status"},
		Findings: []catalog.FindingDef{
			{Code: s3CodePublicAccessBlockIncomplete, Phrase: "public access block incomplete", Severity: domain.SevWarn, Source: "wave2"},
			{Code: s3CodePublic, Phrase: "publicly accessible", Severity: domain.SevBroken, Source: "wave2"},
			{Code: s3CodeVersioningOff, Phrase: "versioning off", Severity: domain.SevWarn, Source: "wave2"},
			{Code: s3CodeMFADeleteOff, Phrase: "MFA delete off", Severity: domain.SevWarn, Source: "wave2"},
			{Code: s3CodeAccessLoggingOff, Phrase: "access logging off", Severity: domain.SevWarn, Source: "wave2"},
			{Code: s3CodeNoLifecycle, Phrase: "no lifecycle rules", Severity: domain.SevWarn, Source: "wave2"},
			{Code: s3CodeNoObjectLock, Phrase: "object lock off", Severity: domain.SevWarn, Source: "wave2"},
		},
	},
	{
		Name:          "ElastiCache Redis",
		ShortName:     "redis",
		Aliases:       []string{"redis", "elasticache"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "elasticache/home?region="+region+"#/redis/"+url.PathEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "cluster_id", Title: "Cluster ID", Width: 28, Sortable: true},
			{Key: "node_type", Title: "Node Type", Width: 18, Sortable: true},
			{Key: "status", Title: "Status", Width: 32, Sortable: true},
			{Key: "nodes", Title: "Nodes", Width: 8, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 40, Sortable: false},
		},
		Color: colorRedis,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchRedisPage(ctx, c.ElastiCache, continuationToken)
		}),
		FieldKeys: []string{"cluster_id", "node_type", "status", "nodes", "endpoint", "arn"},
		Related: []domain.RelatedDef{
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkRedisAlarms, NeedsTargetCache: true, Truncated: true},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkRedisCFN, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkRedisCtEvents, NeedsTargetCache: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkRedisKMS, NeedsTargetCache: false},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkRedisLogs, NeedsTargetCache: true, Truncated: true},
			{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkRedisSecrets, NeedsTargetCache: true, Truncated: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkRedisSG, NeedsTargetCache: true, Truncated: true},
			{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkRedisSNS, NeedsTargetCache: true, Truncated: true},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkRedisSubnet, NeedsTargetCache: true, Truncated: true},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkRedisVPC, NeedsTargetCache: false},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "KmsKeyId", TargetType: "kms"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeRedisCreateFailed, Phrase: "create failed — see events", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeRedisCreating, Phrase: "creating — new group", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeRedisDeleting, Phrase: "deleting — teardown", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeRedisModifying, Phrase: "modifying — config change", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeRedisSnapshotting, Phrase: "snapshotting — backup running", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeRedisShardIssue, Phrase: "shard <NodeGroupId>: <status>", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeRedisMultiAZWithoutAutoFailover, Phrase: "multi-AZ without auto-failover", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeRedisAtRestOff, Phrase: "encryption at rest off", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeRedisTransitOff, Phrase: "encryption in transit off", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeRedisNoAuth, Phrase: "no authentication token", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeRedisNoBackup, Phrase: "automatic backups off", Severity: domain.SevWarn, Source: "wave1"},
		},
	},
	{
		Name:          "DB Clusters",
		ShortName:     "dbc",
		Aliases:       []string{"dbc", "docdb", "clusters", "db-clusters"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:Fields.arn",
		LifecycleKey:  "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			engine := strings.ToLower(r.Fields["engine"])
			if engine == "" {
				return ""
			}
			switch {
			case strings.HasPrefix(engine, "docdb"):
				return consolelink.Regional(region, "docdb/home?region="+region+"#cluster-details/"+url.PathEscape(r.ID))
			case strings.HasPrefix(engine, "neptune"):
				return consolelink.Regional(region, "neptune/home?region="+region)
			default:
				return consolelink.Regional(region, "rds/home?region="+region+"#database:id="+url.PathEscape(r.ID)+";is-cluster=true")
			}
		},
		Columns: []domain.Column{
			{Key: "cluster_id", Title: "Cluster ID", Width: 28, Sortable: true},
			{Key: "engine_version", Title: "Version", Width: 10, Sortable: true},
			{Key: "status", Title: "Status", Width: 32, Sortable: true},
			{Key: "instances", Title: "Instances", Width: 10, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 48, Sortable: false},
		},
		Color: colorDBC,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			if rdsTok, ok2 := strings.CutPrefix(continuationToken, "rds:"); ok2 {
				result, err := FetchRDSDBClustersPage(ctx, c.RDS, rdsTok)
				if err != nil {
					return resource.FetchResult{}, err
				}
				if result.Pagination != nil && result.Pagination.IsTruncated {
					result.Pagination.NextToken = "rds:" + result.Pagination.NextToken
				}
				return result, nil
			}
			docdbTok, _ := strings.CutPrefix(continuationToken, "docdb:")
			docResult, err := FetchDocDBClustersPage(ctx, c.DocDB, docdbTok)
			if err != nil {
				return resource.FetchResult{}, err
			}
			if docResult.Pagination != nil && docResult.Pagination.IsTruncated {
				docResult.Pagination.NextToken = "docdb:" + docResult.Pagination.NextToken
				return docResult, nil
			}
			rdsResult, rdsErr := FetchRDSDBClustersPage(ctx, c.RDS, "")
			if rdsErr != nil {
				return resource.FetchResult{
					Resources: docResult.Resources,
					Pagination: &resource.PaginationMeta{
						IsTruncated: true,
						NextToken:   "rds:",
						PageSize:    len(docResult.Resources),
						TotalHint:   -1,
					},
				}, fmt.Errorf("dbc: RDS-side cluster fetch failed: %w", rdsErr)
			}
			docResult.Resources = dedupResourcesByID(append(docResult.Resources, rdsResult.Resources...))
			if rdsResult.Pagination != nil && rdsResult.Pagination.IsTruncated {
				return resource.FetchResult{
					Resources: docResult.Resources,
					Pagination: &resource.PaginationMeta{
						IsTruncated: true,
						NextToken:   "rds:" + rdsResult.Pagination.NextToken,
						PageSize:    len(docResult.Resources),
						TotalHint:   -1,
					},
				}, nil
			}
			return resource.FetchResult{
				Resources: docResult.Resources,
				Pagination: &resource.PaginationMeta{
					IsTruncated: false,
					PageSize:    len(docResult.Resources),
					TotalHint:   len(docResult.Resources),
				},
			}, nil
		}),
		Wave2: IssueEnricher{Fn: EnrichDBCMaintenance, Priority: 100},
		FieldKeys: []string{
			"cluster_id", "engine", "engine_version", "status", "instances", "endpoint", "arn",
			"has_writer", "writer_count", "deletion_protection", "storage_encrypted",
			"backup_retention_period",
		},
		Related: []domain.RelatedDef{
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkDbcSG},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkDbcAlarm, NeedsTargetCache: true, Truncated: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkDbcLogs, NeedsTargetCache: true, Truncated: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkDbcKMS},
			{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkDbcSecrets, NeedsTargetCache: true, Truncated: true},
			{TargetType: "dbi", DisplayName: "RDS Instances", Checker: checkDbcDBI, NeedsTargetCache: true, Truncated: true},
			{TargetType: "dbc-snap", DisplayName: "DB Cluster Snapshots", Checker: checkDbcDbcSnap, NeedsTargetCache: true, Truncated: true},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkDbcSubnet},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkDbcVPC},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkDbcCTEvents},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VpcSecurityGroups.VpcSecurityGroupId", TargetType: "sg"},
			{FieldPath: "KmsKeyId", TargetType: "kms"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeDBCFailed, Phrase: "failed: cluster operation", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeDBCEncryptionKeyUnreachable, Phrase: "encryption key unreachable", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeDBCIncompatibleParameters, Phrase: "parameter group incompatible", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeDBCNoWriter, Phrase: "no writer: reads only", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeDBCTransitional, Phrase: "<status>: in progress", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBCDeletionProtectionOff, Phrase: "delete-protection off", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBCNotEncryptedAtRest, Phrase: "not encrypted at rest", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBCNoAutomatedBackups, Phrase: "no automated backups", Severity: domain.SevWarn, Source: "wave1"},
			{Code: dbcCodeMaintenanceOverdue, Phrase: "maintenance overdue", Severity: domain.SevBroken, Source: "wave2"},
			// Both cluster fetchers emit single-AZ and default-master-username.
			// The other two come only from Aurora / Multi-AZ clusters: the
			// DocumentDB SDK's DBCluster carries neither AutoMinorVersionUpgrade
			// nor IAMDatabaseAuthenticationEnabled, so a DocumentDB row is
			// silent on both rather than reporting a setting AWS never sent.
			{Code: CodeDBCSingleAZ, Phrase: "single-AZ", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBCMinorUpgradeOff, Phrase: "auto minor version upgrade off", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBCIAMAuthOff, Phrase: "IAM database authentication off", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBCDefaultMasterUser, Phrase: "default master username", Severity: domain.SevWarn, Source: "wave1"},
		},
	},
	{
		Name:          "DynamoDB Tables",
		ShortName:     "ddb",
		Aliases:       []string{"ddb", "dynamodb", "dynamo"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "dynamodbv2/home?region="+region+"#table?name="+url.QueryEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "table_name", Title: "Table Name", Width: 36, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "item_count", Title: "Items", Width: 12, Sortable: true},
			{Key: "size_bytes", Title: "Size", Width: 14, Sortable: true},
			{Key: "billing_mode", Title: "Billing", Width: 16, Sortable: true},
		},
		Color: colorDDB,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchDynamoDBTablesPage(ctx, c.DynamoDB, c.DynamoDB, continuationToken)
		}),
		Wave2:     IssueEnricher{Fn: EnrichDynamoDBPITR, Priority: 100},
		FieldKeys: []string{"table_name", "status", "item_count", "size_bytes", "billing_mode"},
		Related: []domain.RelatedDef{
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkDdbKMS},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkDdbAlarm, NeedsTargetCache: true, Truncated: true},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkDdbLambda},
			{TargetType: "kinesis", DisplayName: "Kinesis Streams", Checker: checkDdbKinesis},
			{TargetType: "backup", DisplayName: "Backup Plans", Checker: checkDdbBackup, Truncated: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkDdbLogs, NeedsTargetCache: true, Truncated: true},
			{TargetType: "vpce", DisplayName: "VPC Endpoints", Checker: checkDdbVPCE, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("ddb")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "SSEDescription.KMSMasterKeyArn", TargetType: "kms"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeDDBKMSKeyInaccessible, Phrase: "kms key inaccessible", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeDDBArchivedKMSLost, Phrase: "archived: kms key lost", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeDDBCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDDBUpdating, Phrase: "updating", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDDBDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDDBArchiving, Phrase: "archiving", Severity: domain.SevWarn, Source: "wave1"},
			{Code: ddbCodePITROff, Phrase: "point-in-time recovery disabled", Severity: domain.SevWarn, Source: "wave2"},
			{Code: CodeDDBDeletionProtectionOff, Phrase: "deletion protection off", Severity: domain.SevWarn, Source: "wave1"},
			{Code: ddbCodeCrossAccountPolicy, Phrase: "resource policy grants another account", Severity: domain.SevWarn, Source: "wave2"},
			{Code: ddbCodePublicPolicy, Phrase: "resource policy open to anyone", Severity: domain.SevBroken, Source: "wave2"},
			DetailsDeniedFindingDef("ddb"),
			DetailsUnavailableFindingDef("ddb"),
		},
	},
	{
		Name:          "OpenSearch Domains",
		ShortName:     "opensearch",
		Aliases:       []string{"opensearch", "os", "elasticsearch"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "aos/home?region="+region+"#opensearch/domains/"+url.PathEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "domain_name", Title: "Domain Name", Width: 28, Sortable: true},
			{Key: "engine_version", Title: "Engine Version", Width: 16, Sortable: true},
			{Key: "instance_type", Title: "Instance Type", Width: 22, Sortable: true},
			{Key: "instance_count", Title: "Instances", Width: 10, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 48, Sortable: false},
		},
		Color: colorOpenSearch,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			// E5 partial success: degraded name-only rows may arrive alongside
			// a composite error — return both, never drop the rows.
			resources, err := FetchOpenSearchDomains(ctx, c.OpenSearch, c.OpenSearch)
			if err != nil && len(resources) == 0 {
				return resource.FetchResult{}, err
			}
			return resource.FetchResult{
				Resources:  resources,
				Pagination: &resource.PaginationMeta{IsTruncated: false, TotalHint: len(resources), PageSize: len(resources)},
			}, err
		}),
		Wave2: IssueEnricher{Fn: EnrichOpenSearchDomains, Priority: 100},
		FieldKeys: []string{
			"domain_name", "engine_version", "instance_type", "instance_count", "endpoint",
			"status", "domain_processing_status",
			"deleted", "processing", "upgrade_processing",
			"service_software_update_available", "encryption_at_rest_enabled",
			"automated_update_date", "current_version", "new_version",
		},
		Related: []domain.RelatedDef{
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkOpenSearchAlarms, NeedsTargetCache: true, Truncated: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkOpenSearchLogs, NeedsTargetCache: false},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkOpenSearchSG},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkOpenSearchVPC},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkOpenSearchKMS},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkOpenSearchCFN, Truncated: true},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkOpenSearchSubnet},
			{TargetType: "acm", DisplayName: "ACM Certificates", Checker: checkOpenSearchACM, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("opensearch")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "EncryptionAtRestOptions.KmsKeyId", TargetType: "kms"},
			{FieldPath: "VPCOptions.VPCId", TargetType: "vpc"},
			{FieldPath: "VPCOptions.SubnetIds", TargetType: "subnet"},
			{FieldPath: "VPCOptions.SecurityGroupIds", TargetType: "sg"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeOpenSearchDeleting, Phrase: "deleting: removal in progress", Severity: domain.SevDim, Source: "wave1"},
			{Code: CodeOpenSearchIsolated, Phrase: "isolated: quarantined by AWS", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeOpenSearchProcessing, Phrase: "processing: config change in flight", Severity: domain.SevWarn, Source: "wave1"},
			{Code: opensearchCodeUpdateForced, Phrase: "software update forced soon", Severity: domain.SevBroken, Source: "wave2"},
			{Code: opensearchCodeEncryptionOff, Phrase: "encryption at rest off", Severity: domain.SevWarn, Source: "wave2"},
			{Code: opensearchCodePublic, Phrase: "reachable outside a VPC", Severity: domain.SevBroken, Source: "wave2"},
			{Code: opensearchCodeHTTPSNotForced, Phrase: "HTTPS not enforced", Severity: domain.SevWarn, Source: "wave2"},
			{Code: opensearchCodeN2NOff, Phrase: "node-to-node encryption off", Severity: domain.SevWarn, Source: "wave2"},
			DetailsDeniedFindingDef("opensearch"),
			DetailsUnavailableFindingDef("opensearch"),
		},
	},
	{
		Name:          "Redshift Clusters",
		ShortName:     "redshift",
		Aliases:       []string{"redshift", "redshift-clusters"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "redshiftv2/home?region="+region+"#cluster-details:cluster="+url.PathEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "cluster_id", Title: "Cluster ID", Width: 36, Sortable: true},
			{Key: "status", Title: "Status", Width: 34, Sortable: true},
			{Key: "node_type", Title: "Node Type", Width: 16, Sortable: true},
			{Key: "num_nodes", Title: "Nodes", Width: 7, Sortable: true},
			{Key: "db_name", Title: "Database", Width: 16, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 44, Sortable: false},
		},
		Color: colorRedshift,
		Wave2: IssueEnricher{Fn: EnrichRedshiftPosture, Priority: 100},
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchRedshiftClustersPage(ctx, c.Redshift, continuationToken)
		}),
		FieldKeys: []string{
			"cluster_id", "status", "cluster_status", "node_type", "num_nodes",
			"db_name", "endpoint", "publicly_accessible", "encrypted",
			"cluster_availability_status",
		},
		Related: []domain.RelatedDef{
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkRedshiftAlarms, NeedsTargetCache: true, Truncated: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkRedshiftSG},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkRedshiftVPC},
			{TargetType: "role", DisplayName: "IAM Role", Checker: checkRedshiftRole},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkRedshiftKMS},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkRedshiftCFN, NeedsTargetCache: true, Truncated: true},
			{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkRedshiftSecrets, NeedsTargetCache: true, Truncated: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkRedshiftLogs},
			{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkRedshiftS3},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkRedshiftSubnet},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("redshift")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VpcId", TargetType: "vpc"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeRedshiftIncompatibleHSM, Phrase: "incompatible-hsm", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeRedshiftIncompatibleNetwork, Phrase: "incompatible-network", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeRedshiftIncompatibleParameters, Phrase: "incompatible-parameters", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeRedshiftIncompatibleRestore, Phrase: "incompatible-restore", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeRedshiftHardwareFailure, Phrase: "hardware-failure", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeRedshiftStorageFull, Phrase: "storage-full", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeRedshiftUnavailable, Phrase: "unavailable", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeRedshiftFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeRedshiftCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeRedshiftModifying, Phrase: "modifying", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeRedshiftResizing, Phrase: "resizing", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeRedshiftRebooting, Phrase: "rebooting", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeRedshiftRenaming, Phrase: "renaming", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeRedshiftDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeRedshiftMaintenance, Phrase: "maintenance", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeRedshiftAvailabilityModifying, Phrase: "modifying", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeRedshiftPendingChange, Phrase: "pending change queued", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeRedshiftMaintenanceDeferred, Phrase: "maintenance deferred", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeRedshiftPubliclyAccessible, Phrase: "publicly accessible", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeRedshiftUnencryptedAtRest, Phrase: "unencrypted at rest", Severity: domain.SevWarn, Source: "wave1"},
			{Code: redshiftCodeAuditLoggingOff, Phrase: "audit logging off", Severity: domain.SevWarn, Source: "wave2"},
			{Code: redshiftCodeRequireSSLOff, Phrase: "SSL not required", Severity: domain.SevWarn, Source: "wave2"},
		},
	},
	{
		Name:          "EFS File Systems",
		ShortName:     "efs",
		Aliases:       []string{"efs", "file-systems"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "efs/home?region="+region+"#/file-systems/"+r.ID)
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "file_system_id", Title: "File System ID", Width: 22, Sortable: true},
			{Key: "status", Title: "Status", Width: 24, Sortable: true},
			{Key: "performance_mode", Title: "Perf Mode", Width: 16, Sortable: true},
			{Key: "encrypted", Title: "Encrypted", Width: 10, Sortable: true},
			{Key: "mount_targets", Title: "Mounts", Width: 8, Sortable: true},
		},
		Color: colorEFS,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchEFSFileSystemsPage(ctx, c.EFS, continuationToken)
		}),
		Wave2:     IssueEnricher{Fn: EnrichEFSMountTargets, Priority: 100},
		FieldKeys: []string{"file_system_id", "name", "status", "performance_mode", "throughput_mode", "encrypted", "mount_targets"},
		Related: []domain.RelatedDef{
			{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkEFSKMS},
			{TargetType: "cfn", DisplayName: "CloudFormation Stacks", Checker: checkEFSCFN, NeedsTargetCache: true, Truncated: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkEFSSG, NeedsTargetCache: false, Truncated: true},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkEFSSubnet, NeedsTargetCache: false, Truncated: true},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkEFSLambda, NeedsTargetCache: false, Truncated: true},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkEFSAlarm, NeedsTargetCache: true, Truncated: true},
			{TargetType: "backup", DisplayName: "Backup Plans", Checker: checkEFSBackup, NeedsTargetCache: true, Truncated: true},
			// EC2 pivot intentionally removed: EC2→EFS mounting happens at the
			// guest OS level via DNS lookup of mt ENIs. AWS exposes no API edge
			// linking instance → filesystem — mount-target ENIs are
			// RequesterManaged with no Attachment.InstanceId, so a checker can
			// only return zero or heuristic noise. Honest drop beats a registered
			// pivot that always returns Count=0 (U9 violation).
			{TargetType: "ecs-task", DisplayName: "ECS Tasks", Checker: checkEFSECSTask, NeedsTargetCache: true, Truncated: true},
			{TargetType: "eni", DisplayName: "Network Interfaces", Checker: checkEFSENI, NeedsTargetCache: true, Truncated: true},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkEFSVPC, NeedsTargetCache: true, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("efs")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "KmsKeyId", TargetType: "kms"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeEFSError, Phrase: "error", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeEFSNoMountTargets, Phrase: "no mount targets", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeEFSCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeEFSUpdating, Phrase: "updating", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeEFSDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1"},
			{Code: efsCodeMountTargetDown, Phrase: "mount target down", Severity: domain.SevBroken, Source: "wave2"},
			{Code: CodeEFSUnencrypted, Phrase: "not encrypted", Severity: domain.SevWarn, Source: "wave1"},
			{Code: efsCodePublicPolicy, Phrase: "file system policy open to anyone", Severity: domain.SevBroken, Source: "wave2"},
			{Code: efsCodeNoBackupPolicy, Phrase: "automatic backups off", Severity: domain.SevWarn, Source: "wave2"},
		},
	},
	{
		Name:          "DB Instance Snapshots",
		ShortName:     "dbi-snap",
		Aliases:       []string{"dbi-snap", "rds-snapshots", "db-snapshots"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "rds/home?region="+region+"#db-snapshot:id="+url.PathEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "snapshot_id", Title: "Snapshot ID", Width: 36, Sortable: true},
			{Key: "db_instance", Title: "DB Instance", Width: 28, Sortable: true},
			{Key: "status", Title: "Status", Width: 32, Sortable: true},
			{Key: "engine", Title: "Engine", Width: 12, Sortable: true},
			{Key: "snapshot_type", Title: "Type", Width: 12, Sortable: true},
			{Key: "created", Title: "Created", Width: 22, Sortable: true},
		},
		Color: colorDBISnap,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchDBISnapshotsPage(ctx, c.RDS, continuationToken)
		}),
		Wave2:     IssueEnricher{Fn: enrichDBISnapCrossRef, Priority: 100},
		FieldKeys: []string{"snapshot_id", "db_instance", "status", "engine", "snapshot_type", "created", "arn"},
		Related: []domain.RelatedDef{
			{TargetType: "dbi", DisplayName: "DB Instances", Checker: checkDBISnapDBI, NeedsTargetCache: true, Truncated: true},
			{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkDBISnapKMS, NeedsTargetCache: true, Truncated: true},
			{TargetType: "backup", DisplayName: "Backup Plans", Checker: checkDBISnapBackup, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkDBISnapCTEvents, NeedsTargetCache: true},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "DBInstanceIdentifier", TargetType: "dbi"},
			{FieldPath: "KmsKeyId", TargetType: "kms"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeDBISnapFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeDBISnapIncompatible, Phrase: "<incompatible-* status>", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeDBISnapCreating, Phrase: "creating: <pct>%", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBISnapUnencrypted, Phrase: "unencrypted", Severity: domain.SevWarn, Source: "wave1"},
			{Code: dbiSnapOrphanCode, Phrase: "orphan: source DB deleted", Severity: domain.SevBroken, Source: "wave2"},
			{Code: dbiSnapPastRetentionCode, Phrase: "automated, <N>d past retention", Severity: domain.SevBroken, Source: "wave2"},
			{Code: dbiSnapPublicCode, Phrase: "shared with all AWS accounts", Severity: domain.SevBroken, Source: "wave2"},
		},
	},
	{
		Name:          "DB Cluster Snapshots",
		ShortName:     "dbc-snap",
		Aliases:       []string{"dbc-snap", "docdb-snapshots", "cluster-snapshots"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "rds/home?region="+region+"#db-snapshot:id="+url.PathEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "snapshot_id", Title: "Snapshot ID", Width: 36, Sortable: true},
			{Key: "cluster_id", Title: "Cluster ID", Width: 28, Sortable: true},
			{Key: "status", Title: "Status", Width: 32, Sortable: true},
			{Key: "engine", Title: "Engine", Width: 12, Sortable: true},
			{Key: "snapshot_type", Title: "Type", Width: 12, Sortable: true},
			{Key: "snapshot_create_time", Title: "Created", Width: 22, Sortable: true},
			{Key: "storage_type", Title: "Storage", Width: 10, Sortable: true},
		},
		Color: colorDBCSnap,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			if rdsTok, ok2 := strings.CutPrefix(continuationToken, "rds:"); ok2 {
				result, err := FetchRDSDBClusterSnapshotsPage(ctx, c.RDS, rdsTok)
				if err != nil {
					return resource.FetchResult{}, err
				}
				if result.Pagination != nil && result.Pagination.IsTruncated {
					result.Pagination.NextToken = "rds:" + result.Pagination.NextToken
				}
				return result, nil
			}
			docdbTok, _ := strings.CutPrefix(continuationToken, "docdb:")
			docResult, err := FetchDocDBClusterSnapshotsPage(ctx, c.DocDB, docdbTok)
			if err != nil {
				return resource.FetchResult{}, err
			}
			if docResult.Pagination != nil && docResult.Pagination.IsTruncated {
				docResult.Pagination.NextToken = "docdb:" + docResult.Pagination.NextToken
				return docResult, nil
			}
			rdsResult, rdsErr := FetchRDSDBClusterSnapshotsPage(ctx, c.RDS, "")
			if rdsErr != nil {
				return resource.FetchResult{
					Resources: docResult.Resources,
					Pagination: &resource.PaginationMeta{
						IsTruncated: true,
						NextToken:   "rds:",
						PageSize:    len(docResult.Resources),
						TotalHint:   -1,
					},
				}, fmt.Errorf("dbc-snap: RDS-side cluster snapshot fetch failed: %w", rdsErr)
			}
			docResult.Resources = dedupResourcesByID(append(docResult.Resources, rdsResult.Resources...))
			if rdsResult.Pagination != nil && rdsResult.Pagination.IsTruncated {
				return resource.FetchResult{
					Resources: docResult.Resources,
					Pagination: &resource.PaginationMeta{
						IsTruncated: true,
						NextToken:   "rds:" + rdsResult.Pagination.NextToken,
						PageSize:    len(docResult.Resources),
						TotalHint:   -1,
					},
				}, nil
			}
			return resource.FetchResult{
				Resources: docResult.Resources,
				Pagination: &resource.PaginationMeta{
					IsTruncated: false,
					PageSize:    len(docResult.Resources),
					TotalHint:   len(docResult.Resources),
				},
			}, nil
		}),
		Wave2: IssueEnricher{Fn: enrichDBCSnapCrossRef, Priority: 100},
		FieldKeys: []string{
			"snapshot_id", "cluster_id", "status", "engine", "snapshot_type",
			"snapshot_create_time", "storage_type", "storage_encrypted",
		},
		Related: []domain.RelatedDef{
			{TargetType: "dbc", DisplayName: "DocumentDB Cluster", Checker: checkDbcSnapDBC, NeedsTargetCache: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkDbcSnapKMS},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkDbcSnapVPC},
			{TargetType: "backup", DisplayName: "Backup Plans", Checker: checkDbcSnapBackup, Truncated: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkDbcSnapCTEvents},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VpcId", TargetType: "vpc"},
			{FieldPath: "KmsKeyId", TargetType: "kms"},
		},
		Findings: []catalog.FindingDef{
			{Code: CodeDBCSnapFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeDBCSnapIncompatible, Phrase: "<incompatible-* status>", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeDBCSnapCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBCSnapManualUnused, Phrase: "manual, unused <N>d", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBCSnapUnencrypted, Phrase: "unencrypted", Severity: domain.SevWarn, Source: "wave1"},
			{Code: dbcSnapOrphanCode, Phrase: "orphan: source cluster deleted", Severity: domain.SevBroken, Source: "wave2"},
			{Code: dbcSnapPastRetentionCode, Phrase: "automated, <N>d past retention", Severity: domain.SevBroken, Source: "wave2"},
			{Code: dbcSnapPublicCode, Phrase: "shared with all AWS accounts", Severity: domain.SevBroken, Source: "wave2"},
		},
	},
}

var databasesChildTypes = []catalog.ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:      "RDS Events",
		ShortName: "dbi_events",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			db := r.Fields["source_identifier"]
			if db == "" {
				return ""
			}
			return consolelink.Regional(region, "rds/home?region="+region+"#database:id="+url.PathEscape(db)+";is-cluster=false")
		},
		Columns:   resource.DbiEventColumns(),
		CopyField: "message",
		Color:     colorAnyFindingOrHealthy,
		FieldKeys: []string{
			"timestamp", "event_categories", "message",
			"source_identifier", "source_type", "source_arn",
		},
		ChildFetcher: childFetcherWithClients(func(ctx context.Context, c *ServiceClients, parentCtx resource.ParentContext, continuationToken string) (resource.FetchResult, error) {
			return FetchRDSEvents(ctx, c.RDS, parentCtx["db_identifier"], continuationToken)
		}),
		Findings: []catalog.FindingDef{
			{Code: CodeDBIEventFailure, Phrase: "failure", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeDBIEventLowStorage, Phrase: "low storage", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeDBIEventFailover, Phrase: "failover", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBIEventRecovery, Phrase: "recovery", Severity: domain.SevWarn, Source: "wave1"},
		},
	},
}
