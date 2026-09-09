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

// Every classifier below takes the row's colour from its findings and nothing
// else. Each type's fetcher attaches a Finding for every non-healthy signal it
// can see, and the disk cache round-trips Findings (core/cache: TypeFile
// `findings`), so a row that reaches these with no Finding has no signal to
// report — it is not a row whose rendered Status phrase should be parsed back
// into a colour.

func colorDBI(r domain.Resource) domain.Color {
	return colorAnyFindingOrHealthy(r)
}

func colorS3(r domain.Resource) domain.Color {
	return colorAnyFindingOrHealthy(r)
}

func colorRedis(r domain.Resource) domain.Color {
	return colorAnyFindingOrHealthy(r)
}

func colorDBC(r domain.Resource) domain.Color {
	return colorAnyFindingOrHealthy(r)
}

func colorDDB(r domain.Resource) domain.Color {
	return colorAnyFindingOrHealthy(r)
}

func colorOpenSearch(r domain.Resource) domain.Color {
	return colorAnyFindingOrHealthy(r)
}

func colorRedshift(r domain.Resource) domain.Color {
	return colorAnyFindingOrHealthy(r)
}

func colorEFS(r domain.Resource) domain.Color {
	return colorAnyFindingOrHealthy(r)
}

func colorDBISnap(r domain.Resource) domain.Color {
	return colorAnyFindingOrHealthy(r)
}

func colorDBCSnap(r domain.Resource) domain.Color {
	return colorAnyFindingOrHealthy(r)
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
			{Key: "db_identifier", Title: "DB Identifier", Path: "DBInstanceIdentifier", Width: 28, Sortable: true},
			{Key: "engine", Title: "Engine", Path: "Engine", Width: 12, Sortable: true},
			{Key: "engine_version", Title: "Version", Path: "EngineVersion", Width: 10, Sortable: true},
			{Key: "status", Title: "Status", Width: 28, SortKey: "status_raw", Sortable: true},
			{Key: "class", Title: "Class", Path: "DBInstanceClass", Width: 16, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Path: "Endpoint.Address", Width: 40},
			{Key: "multi_az", Title: "Multi-AZ", Path: "MultiAZ", Width: 10, Sortable: true},
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
			"db_identifier", "engine", "engine_version", "status", "status_raw", "class", "endpoint",
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
			{Code: CodeDBITransitional, Phrase: "<transitional status>", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBINoAutomatedBackups, Phrase: "no automated backups", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBIPubliclyAccessible, Phrase: "publicly accessible", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBIUnencryptedStorage, Phrase: "unencrypted storage", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBIDeletionProtectionOff, Phrase: "deletion protection off", Severity: domain.SevWarn, Source: "wave1"},
			{Code: dbiCodePendingMaintenance, Phrase: "maintenance scheduled", Severity: domain.SevWarn, Source: "wave2", Detail: "AWS has a maintenance action pending for this instance and will apply it in a maintenance window of its choosing once the target date passes; the action, apply method and earliest date are listed below. Apply it yourself in a window that suits you."},
			{Code: CodeDBISingleAZ, Phrase: "single-AZ", Severity: domain.SevWarn, Source: "wave1", Detail: "The instance runs in one Availability Zone, so an AZ failure takes the database down until you restore it. Enable Multi-AZ to keep a synchronous standby in a second AZ."},
			{Code: CodeDBIMinorUpgradeOff, Phrase: "auto minor version upgrade off", Severity: domain.SevWarn, Source: "wave1", Detail: "Minor engine patches — including security fixes — are never applied automatically. Enable auto minor version upgrade, or schedule the patching yourself."},
			{Code: CodeDBINotInBackupPlan, Phrase: "not covered by a backup plan", Severity: domain.SevWarn, Source: "wave2", Detail: "No backup plan selects this database, so its retention is whatever the instance's own automated backups happen to be. Add it to a plan by ARN, or give it a tag one of your plans already selects on."},
			{Code: CodeDBIIAMAuthOff, Phrase: "IAM database authentication off", Severity: domain.SevWarn, Source: "wave1", Detail: "Connections authenticate with long-lived database passwords only. Enable IAM database authentication so credentials become short-lived tokens tied to IAM identities."},
			{Code: CodeDBIDefaultMasterUser, Phrase: "default master username", Severity: domain.SevWarn, Source: "wave1", Detail: "The administrative account uses the vendor default name, so an attacker only has to guess the password. Create a differently-named administrative user and retire this one."},
			{Code: CodeDBICACertExpiring, Phrase: "server certificate expires in <N day(s)>", Severity: domain.SevWarn, Source: "wave1", Detail: "The server certificate expires soon; clients that verify the connection will refuse to talk to it once it does. Rotate the instance onto the current certificate authority during a maintenance window."},
			{Code: CodeDBICACertExpiringUrgent, Phrase: "server certificate expires in <N day(s)>", Severity: domain.SevBroken, Source: "wave1", Detail: "The server certificate expires within a month, and every client that verifies the connection will refuse to talk to the instance the moment it does. Book the maintenance window now and rotate the instance onto the current certificate authority."},
			{Code: dbiCodeEngineDeprecated, Phrase: "engine version deprecated", Severity: domain.SevBroken, Source: "wave2", Detail: "AWS no longer supports this engine version, so it stops receiving security patches and will be force-upgraded on AWS's schedule. Upgrade to a supported version during a maintenance window of your choosing."},
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
			{Key: "name", Title: "Bucket Name", Path: "Name", Width: 36, Sortable: true},
			{Title: "Region", Path: "BucketRegion", Width: 14},
			{Key: "creation_date", Title: "Creation Date", Path: "CreationDate", Width: 22, Sortable: true},
			{Key: "status", Title: "Status", Width: 32},
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
			{Code: s3CodePublicAccessBlockIncomplete, Phrase: "public access block incomplete", Severity: domain.SevWarn, Source: "wave2", Detail: "Bucket-level public access block is missing or partial — account-level PAB may still apply."},
			{Code: s3CodePublic, Phrase: "publicly accessible", Severity: domain.SevBroken, Source: "wave2", Detail: "AWS reports this bucket's policy as public, so anyone on the internet can reach its objects. Remove the wildcard-principal statements from the bucket policy, or block them with a public access block."},
			{Code: s3CodeVersioningOff, Phrase: "versioning off", Severity: domain.SevWarn, Source: "wave2", Detail: "Overwritten and deleted objects are gone for good — there is no previous version to restore. Enable versioning on the bucket."},
			{Code: s3CodeMFADeleteOff, Phrase: "MFA delete off", Severity: domain.SevWarn, Source: "wave2", Detail: "Versioning is on, but anyone holding the delete permission can still remove versions permanently. Enable MFA delete so destroying a version needs a second factor."},
			{Code: s3CodeAccessLoggingOff, Phrase: "access logging off", Severity: domain.SevWarn, Source: "wave2", Detail: "Nothing records who read or wrote objects here, so an incident leaves no trail to follow. Point server access logging at a log destination bucket."},
			{Code: s3CodeNoLifecycle, Phrase: "no lifecycle rules", Severity: domain.SevWarn, Source: "wave2", Detail: "No lifecycle rule expires or transitions objects, so data and cost accumulate indefinitely. Add a lifecycle rule matching the bucket's retention policy."},
			{Code: s3CodeNoObjectLock, Phrase: "object lock off", Severity: domain.SevWarn, Source: "wave2", Detail: "Objects can be overwritten or deleted by anyone with write access — nothing enforces retention. Enable object lock on a new bucket and migrate if the data is compliance-relevant."},
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
			{Key: "cluster_id", Title: "Cluster ID", Path: "ReplicationGroupId", Width: 28, Sortable: true},
			{Key: "node_type", Title: "Node Type", Path: "CacheNodeType", Width: 18, Sortable: true},
			{Key: "status", Title: "Status", Width: 32, SortKey: "status_raw", Sortable: true},
			{Key: "nodes", Title: "Nodes", Width: 8, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Path: "ConfigurationEndpoint.Address", Width: 40},
		},
		Color: colorRedis,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchRedisPage(ctx, c.ElastiCache, continuationToken)
		}),
		FieldKeys: []string{"cluster_id", "node_type", "status", "status_raw", "nodes", "endpoint", "arn"},
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
			{Code: CodeRedisAtRestOff, Phrase: "encryption at rest off", Severity: domain.SevWarn, Source: "wave1", Detail: "Cached data is written to disk and to backups unencrypted. Encryption at rest can only be turned on at creation time — recreate the replication group with it enabled and migrate."},
			{Code: CodeRedisTransitOff, Phrase: "encryption in transit off", Severity: domain.SevWarn, Source: "wave1", Detail: "Client traffic to this group crosses the network in cleartext, so anyone with VPC access can read the cached data. Enable in-transit encryption on the replication group."},
			{Code: CodeRedisNoAuth, Phrase: "no authentication token", Severity: domain.SevBroken, Source: "wave1", Detail: "The group accepts any client that can reach it — encryption in transit is on but no authentication token is required. Set one, so a network-level reachability mistake is not immediately a data breach."},
			{Code: CodeRedisNoBackup, Phrase: "automatic backups off", Severity: domain.SevWarn, Source: "wave1", Detail: "Automatic backups are off, so a failed replication group takes its data with it. Set a snapshot retention limit of at least one day."},
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
			{Key: "cluster_id", Title: "Cluster ID", Path: "DBClusterIdentifier", Width: 28, Sortable: true},
			{Key: "engine_version", Title: "Version", Path: "EngineVersion", Width: 10, Sortable: true},
			{Key: "status", Title: "Status", Width: 32, SortKey: "status_raw", Sortable: true},
			{Key: "instances", Title: "Instances", Path: "DBClusterMembers", Width: 10, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Path: "Endpoint", Width: 48},
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
			"cluster_id", "engine", "engine_version", "status", "status_raw", "instances", "endpoint", "arn",
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
			{Code: CodeDBCSingleAZ, Phrase: "single-AZ", Severity: domain.SevWarn, Source: "wave1", Detail: "The cluster has no instance in a second Availability Zone, so an AZ failure takes it down until you restore it. Add a replica in another AZ."},
			{Code: CodeDBCMinorUpgradeOff, Phrase: "auto minor version upgrade off", Severity: domain.SevWarn, Source: "wave1", Detail: "Minor engine patches — including security fixes — are never applied automatically. Enable auto minor version upgrade, or schedule the patching yourself."},
			{Code: CodeDBCIAMAuthOff, Phrase: "IAM database authentication off", Severity: domain.SevWarn, Source: "wave1", Detail: "Connections authenticate with long-lived database passwords only. Enable IAM database authentication so credentials become short-lived tokens tied to IAM identities."},
			{Code: CodeDBCDefaultMasterUser, Phrase: "default master username", Severity: domain.SevWarn, Source: "wave1", Detail: "The administrative account uses the vendor default name, so an attacker only has to guess the password. Create a differently-named administrative user and retire this one."},
			{Code: CodeDBCNotInBackupPlan, Phrase: "not covered by a backup plan", Severity: domain.SevWarn, Source: "wave2", Detail: "No backup plan selects this cluster, so its retention is whatever the cluster's own automated backups happen to be. Add it to a plan by ARN, or give it a tag one of your plans already selects on."},
		},
	},
	{
		Name:           "DynamoDB Tables",
		ShortName:      "ddb",
		HumanizeFields: []string{"TableStatus"},
		Aliases:        []string{"ddb", "dynamodb", "dynamo"},
		Category:       "DATABASES & STORAGE",
		CloudTrailKey:  "ResourceName:ID",
		LifecycleKey:   "status",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "dynamodbv2/home?region="+region+"#table?name="+url.QueryEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "table_name", Title: "Table Name", Path: "TableName", Width: 36, Sortable: true},
			{Key: "status", Title: "Status", Width: 32, Sortable: true},
			{Key: "item_count", Title: "Items", Path: "ItemCount", Width: 12, Sortable: true},
			{Key: "size_bytes", Title: "Size", Width: 14, SortKey: "size_bytes_raw", Sortable: true},
			{Key: "billing_mode", Title: "Billing", Width: 16, Sortable: true},
		},
		Color: colorDDB,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchDynamoDBTablesPage(ctx, c.DynamoDB, c.DynamoDB, continuationToken)
		}),
		Wave2:     IssueEnricher{Fn: EnrichDynamoDBPITR, Priority: 100},
		FieldKeys: []string{"table_name", "status", "item_count", "size_bytes", "size_bytes_raw", "billing_mode"},
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
			{Code: CodeDDBDeletionProtectionOff, Phrase: "deletion protection off", Severity: domain.SevWarn, Source: "wave1", Detail: "A single delete call (DeleteTable) destroys this table and its data. Turn on deletion protection so removing it takes a deliberate second step."},
			{Code: ddbCodeCrossAccountPolicy, Phrase: "resource policy grants another account", Severity: domain.SevWarn, Source: "wave2", Detail: "The table's resource policy grants access to an AWS account outside this one. Confirm each account belongs to a partner you meant to share with, and remove the rest."},
			{Code: ddbCodePublicPolicy, Phrase: "resource policy open to anyone", Severity: domain.SevBroken, Source: "wave2", Detail: "The table's resource policy allows any AWS principal, so anyone with an AWS account can reach it. Replace the wildcard principal with the specific roles that need access."},
			{Code: CodeDDBNotInBackupPlan, Phrase: "not covered by a backup plan", Severity: domain.SevWarn, Source: "wave2", Detail: "No backup plan selects this table, so nothing is scheduled to copy it and point-in-time recovery alone will not survive the table being deleted. Add it to a plan by ARN, or give it a tag one of your plans already selects on."},
			DetailsDeniedFindingDef("ddb", ""),
			DetailsUnavailableFindingDef("ddb"),
		},
	},
	{
		Name:           "OpenSearch Domains",
		ShortName:      "opensearch",
		HumanizeFields: []string{"domain_processing_status"},
		Aliases:        []string{"opensearch", "os", "elasticsearch"},
		Category:       "DATABASES & STORAGE",
		CloudTrailKey:  "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "aos/home?region="+region+"#opensearch/domains/"+url.PathEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "domain_name", Title: "Domain Name", Path: "DomainName", Width: 28, Sortable: true},
			{Key: "status", Title: "Status", Width: 40},
			{Key: "engine_version", Title: "Engine Version", Path: "EngineVersion", Width: 16, Sortable: true},
			{Key: "instance_type", Title: "Instance Type", Path: "ClusterConfig.InstanceType", Width: 22, Sortable: true},
			{Key: "instance_count", Title: "Instances", Path: "ClusterConfig.InstanceCount", Width: 10, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Path: "Endpoint", Width: 48},
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
			{Code: opensearchCodeUpdateForced, Phrase: "software update forced soon", Severity: domain.SevWarn, Source: "wave1", Detail: "AWS will apply this update automatically once the scheduled date passes; upgrade on your own schedule before then to control the maintenance window."},
			{Code: opensearchCodeEncryptionOff, Phrase: "encryption at rest off", Severity: domain.SevWarn, Source: "wave1", Detail: "Data at rest is stored unencrypted. Enabling encryption at rest requires creating a new domain and migrating data — it cannot be turned on in place."},
			{Code: opensearchCodePublic, Phrase: "reachable outside a VPC", Severity: domain.SevBroken, Source: "wave1", Detail: "The domain sits outside a VPC and its access policy allows any principal, so the search endpoint is reachable from the internet. Move the domain into a VPC, or scope the access policy to named principals."},
			{Code: opensearchCodeHTTPSNotForced, Phrase: "HTTPS not enforced", Severity: domain.SevWarn, Source: "wave1", Detail: "The domain accepts plaintext HTTP, so queries and results can be read off the wire. Turn on Require HTTPS in the domain's endpoint options."},
			{Code: opensearchCodeN2NOff, Phrase: "node-to-node encryption off", Severity: domain.SevWarn, Source: "wave1", Detail: "Traffic between the domain's own nodes is unencrypted. Node-to-node encryption can only be enabled on a domain that already has it configured at creation — recreate the domain if this data is sensitive."},
			DetailsDeniedFindingDefPhraseOnly("opensearch"),
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
			{Key: "cluster_id", Title: "Cluster ID", Path: "ClusterIdentifier", Width: 36, Sortable: true},
			{Key: "status", Title: "Status", Width: 34, SortKey: "cluster_status", Sortable: true},
			{Title: "Pending", Path: "PendingModifiedValues.NodeType", Width: 14},
			{Key: "node_type", Title: "Node Type", Path: "NodeType", Width: 16, Sortable: true},
			{Key: "num_nodes", Title: "Nodes", Path: "NumberOfNodes", Width: 7, Sortable: true},
			{Key: "db_name", Title: "Database", Path: "DBName", Width: 16, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Path: "Endpoint.Address", Width: 44},
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
			{Code: CodeRedshiftIncompatibleHSM, Phrase: "broken: incompatible-hsm", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeRedshiftIncompatibleNetwork, Phrase: "broken: incompatible-network", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeRedshiftIncompatibleParameters, Phrase: "broken: incompatible-parameters", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeRedshiftIncompatibleRestore, Phrase: "broken: incompatible-restore", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeRedshiftHardwareFailure, Phrase: "broken: hardware-failure", Severity: domain.SevBroken, Source: "wave1"},
			{Code: CodeRedshiftStorageFull, Phrase: "broken: storage-full", Severity: domain.SevBroken, Source: "wave1"},
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
			{Code: redshiftCodeAuditLoggingOff, Phrase: "audit logging off", Severity: domain.SevWarn, Source: "wave2", Detail: "Nothing records connections and queries against this cluster, so an incident leaves no trail to follow. Enable audit logging to an S3 bucket or a CloudWatch log group."},
			{Code: redshiftCodeRequireSSLOff, Phrase: "SSL not required", Severity: domain.SevWarn, Source: "wave2", Detail: "The cluster accepts unencrypted client connections, so credentials and query results can be read off the wire. Set the parameter group's require-SSL parameter (require_ssl) to true and reboot."},
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
			{Key: "name", Title: "Name", Path: "Name", Width: 28, Sortable: true},
			{Key: "file_system_id", Title: "File System ID", Path: "FileSystemId", Width: 22, Sortable: true},
			{Key: "status", Title: "Status", Width: 24, Sortable: true},
			{Key: "performance_mode", Title: "Perf Mode", Path: "PerformanceMode", Width: 16, Sortable: true},
			{Key: "encrypted", Title: "Encrypted", Path: "Encrypted", Width: 10, Sortable: true},
			{Key: "mount_targets", Title: "Mounts", Path: "NumberOfMountTargets", Width: 8, Sortable: true},
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
			{Code: CodeEFSUnencrypted, Phrase: "not encrypted", Severity: domain.SevWarn, Source: "wave1", Detail: "File data is stored unencrypted at rest. Encryption can only be set when the file system is created — create an encrypted file system and copy the data across."},
			{Code: efsCodePublicPolicy, Phrase: "file system policy open to anyone", Severity: domain.SevBroken, Source: "wave2", Detail: "The file system policy allows any AWS principal, so anyone who can reach a mount target can read and write the data. Replace the wildcard principal with the specific roles that need access."},
			{Code: efsCodeNoBackupPolicy, Phrase: "automatic backups off", Severity: domain.SevWarn, Source: "wave2", Detail: "AWS Backup is not taking daily backups of this file system, so a deletion or corruption is unrecoverable. Turn the automatic backup policy on."},
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
			{Key: "snapshot_id", Title: "Snapshot ID", Path: "DBSnapshotIdentifier", Width: 36, Sortable: true},
			{Key: "db_instance", Title: "DB Instance", Path: "DBInstanceIdentifier", Width: 28, Sortable: true},
			{Key: "status", Title: "Status", Width: 32, Sortable: true},
			{Key: "engine", Title: "Engine", Path: "Engine", Width: 12, Sortable: true},
			{Key: "snapshot_type", Title: "Type", Path: "SnapshotType", Width: 12, Sortable: true},
			{Key: "created", Title: "Created", Path: "SnapshotCreateTime", Width: 22, Sortable: true},
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
			{Code: CodeDBISnapTransitional, Phrase: "<status>", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBISnapUnencrypted, Phrase: "unencrypted", Severity: domain.SevWarn, Source: "wave1"},
			{Code: dbiSnapOrphanCode, Phrase: "orphan: source DB deleted", Severity: domain.SevBroken, Source: "wave2"},
			{Code: dbiSnapPastRetentionCode, Phrase: "automated, <N>d past retention", Severity: domain.SevBroken, Source: "wave2"},
			{Code: dbiSnapPublicCode, Phrase: "shared with all AWS accounts", Severity: domain.SevBroken, Source: "wave2", Detail: "The snapshot is shared with every AWS account, so anyone can restore it and read the database it came from. Remove `all` from the snapshot's restore attribute."},
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
			{Key: "snapshot_id", Title: "Snapshot ID", Path: "DBClusterSnapshotIdentifier", Width: 36, Sortable: true},
			{Key: "cluster_id", Title: "Cluster ID", Path: "DBClusterIdentifier", Width: 28, Sortable: true},
			{Key: "status", Title: "Status", Width: 32, Sortable: true},
			{Key: "engine", Title: "Engine", Path: "Engine", Width: 12, Sortable: true},
			{Key: "snapshot_type", Title: "Type", Path: "SnapshotType", Width: 12, Sortable: true},
			{Key: "snapshot_create_time", Title: "Created", Path: "SnapshotCreateTime", Width: 22, Sortable: true},
			{Key: "storage_type", Title: "Storage", Path: "StorageType", Width: 10, Sortable: true},
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
			{Code: CodeDBCSnapTransitional, Phrase: "<status>", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBCSnapManualUnused, Phrase: "manual, unused <N>d", Severity: domain.SevWarn, Source: "wave1"},
			{Code: CodeDBCSnapUnencrypted, Phrase: "unencrypted", Severity: domain.SevWarn, Source: "wave1"},
			{Code: dbcSnapOrphanCode, Phrase: "orphan: source cluster deleted", Severity: domain.SevBroken, Source: "wave2"},
			{Code: dbcSnapPastRetentionCode, Phrase: "automated, <N>d past retention", Severity: domain.SevBroken, Source: "wave2"},
			{Code: dbcSnapPublicCode, Phrase: "shared with all AWS accounts", Severity: domain.SevBroken, Source: "wave2", Detail: "The snapshot is shared with every AWS account, so anyone can restore it and read the cluster it came from. Remove `all` from the snapshot's restore attribute."},
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
