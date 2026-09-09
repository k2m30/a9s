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
			{Key: "db_identifier", Title: "DB Identifier", Path: "DBInstanceIdentifier", Width: 28},
			{Key: "engine", Title: "Engine", Path: "Engine", Width: 12},
			{Key: "engine_version", Title: "Version", Path: "EngineVersion", Width: 10},
			{Key: "status", Title: "Status", Width: 28, SortKey: "status_raw"},
			{Key: "class", Title: "Class", Path: "DBInstanceClass", Width: 16},
			{Key: "endpoint", Title: "Endpoint", Path: "Endpoint.Address", Width: 40},
			{Key: "multi_az", Title: "Multi-AZ", Path: "MultiAZ", Width: 10},
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
		Wave2: IssueEnricher{Fn: EnrichDBIMaintenance, Priority: 10, Reads: []string{"backup"}},
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
			{Code: CodeDBIFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1", Detail: "The instance is not serving connections and AWS could not bring it back on its own. Check its recent events, and plan a restore from the latest automated backup or snapshot — an instance in this state rarely recovers in place."},
			{Code: CodeDBIStorageFull, Phrase: "storage-full", Severity: domain.SevBroken, Source: "wave1", Detail: "The instance has run out of disk, so writes are rejected and the database is effectively read-only until space is freed. Raise the allocated storage now, then turn on storage autoscaling so the next growth spurt does not repeat this."},
			{Code: CodeDBIIncompatibleNetwork, Phrase: "incompatible-network", Severity: domain.SevBroken, Source: "wave1", Detail: "The instance cannot start because its subnet group no longer gives it what it needs, usually free addresses or the Availability Zones it was created in. Fix the subnet group's subnets and free capacity, then reboot the instance."},
			{Code: CodeDBIIncompatibleOptionGroup, Phrase: "incompatible-option-group", Severity: domain.SevBroken, Source: "wave1", Detail: "The option group attached to this instance does not work with the engine version it is running, so it cannot come up. Move it to an option group built for that version. An engine version cannot be rolled back in place, so returning to the older one means restoring a snapshot taken before the upgrade into a new instance."},
			{Code: CodeDBIIncompatibleParameters, Phrase: "incompatible-parameters", Severity: domain.SevBroken, Source: "wave1", Detail: "A parameter in this instance's parameter group is rejected by the engine, most often a memory setting larger than the instance class allows, so it will not start. Correct the offending parameter and reboot the instance to apply it."},
			{Code: CodeDBIIncompatibleRestore, Phrase: "incompatible-restore", Severity: domain.SevBroken, Source: "wave1", Detail: "The restore into this instance could not complete, so it holds no usable database. Check that the snapshot's engine version and options match the target, then start the restore again into a fresh instance."},
			{Code: CodeDBIRestoreError, Phrase: "restore-error", Severity: domain.SevBroken, Source: "wave1", Detail: "The instance failed while restoring from backup, so the recovery you were counting on did not land. Read its events for the failing step and restore again, choosing a different snapshot or point in time if one snapshot is the problem."},
			{Code: CodeDBIEncryptionKeyUnavailable, Phrase: "encryption key unavailable", Severity: domain.SevBroken, Source: "wave1", Detail: "The KMS key that encrypts this instance's storage cannot be used, so the database is inaccessible and stays that way until the key is usable again. Check whether the key was disabled, scheduled for deletion, or has a policy that no longer grants the database service access."},
			{Code: CodeDBIStopped, Phrase: "stopped (storage still billed)", Severity: domain.SevBroken, Source: "wave1", Detail: "The database accepts no connections, and a stopped instance is restarted automatically after seven days, so this is not a way to keep it switched off. Start it if applications need it, or take a final snapshot and delete it — its storage and any provisioned IOPS are billed while it sits here."},
			{Code: CodeDBITransitional, Phrase: "<transitional status>", Severity: domain.SevWarn, Source: "wave1", Detail: "The instance is mid-change, so it may fail over, drop connections, or run with reduced performance until it settles. Wait for it to return to available before starting another modification or judging its performance."},
			{Code: CodeDBINoAutomatedBackups, Phrase: "no automated backups", Severity: domain.SevWarn, Source: "wave1", Detail: "Backup retention is zero, so there are no automated backups and no point-in-time recovery: a bad deployment or a dropped table can only be undone from a manual snapshot. Set a retention period of at least one day, and longer for anything that matters."},
			{Code: CodeDBIPubliclyAccessible, Phrase: "public endpoint", Severity: domain.SevWarn, Source: "wave1", Detail: "The instance resolves to a routable address from outside the VPC, so only its security groups stand between the database and the internet. Turn public accessibility off and reach it over a private link or a bastion host unless an external system genuinely requires it."},
			{Code: CodeDBIUnencryptedStorage, Phrase: "unencrypted storage", Severity: domain.SevWarn, Source: "wave1", Detail: "The instance's storage, its snapshots and its automated backups are all written unencrypted, and encryption cannot be turned on in place. Snapshot the instance, copy the snapshot with encryption enabled, and restore into a new encrypted instance when you can take the cutover."},
			{Code: CodeDBIDeletionProtectionOff, Phrase: "deletion protection off", Severity: domain.SevWarn, Source: "wave1", Detail: "One delete call or one console click can delete this database and its automated backups together. Turn deletion protection on so removing it takes a deliberate second step."},
			{Code: dbiCodePendingMaintenance, Phrase: "maintenance scheduled", Severity: domain.SevWarn, Source: "wave2", Detail: "AWS has a maintenance action pending for this instance and will apply it in a maintenance window of its choosing once the target date passes; the action, apply method and earliest date are listed below. Apply it yourself in a window that suits you."},
			{Code: CodeDBISingleAZ, Phrase: "single-AZ", Severity: domain.SevWarn, Source: "wave1", Detail: "The instance runs in one Availability Zone, so an AZ failure takes the database down until you restore it. Enable Multi-AZ to keep a synchronous standby in a second AZ."},
			{Code: CodeDBIMinorUpgradeOff, Phrase: "auto minor version upgrade off", Severity: domain.SevWarn, Source: "wave1", Detail: "Minor engine patches — including security fixes — are never applied automatically. Enable auto minor version upgrade, or schedule the patching yourself."},
			{Code: CodeDBINotInBackupPlan, Phrase: "not covered by a backup plan", Severity: domain.SevWarn, Source: "wave2", Detail: "No backup plan selects this database, so its retention is whatever the instance's own automated backups happen to be. Add it to a plan by ARN, or give it a tag one of your plans already selects on."},
			{Code: CodeDBIIAMAuthOff, Phrase: "IAM database authentication off", Severity: domain.SevWarn, Source: "wave1", Detail: "Connections authenticate with long-lived database passwords only. Enable IAM database authentication so credentials become short-lived tokens tied to IAM identities."},
			{Code: CodeDBIDefaultMasterUser, Phrase: "default master username", Severity: domain.SevWarn, Source: "wave1", Detail: "The administrative account uses the vendor default name, so an attacker only has to guess the password. Create a differently-named administrative user and retire this one."},
			{Code: CodeDBICACertExpiring, Phrase: "server certificate expires in <N day(s)>", Severity: domain.SevWarn, Source: "wave1", Detail: "The server certificate expires soon; clients that verify the connection will refuse to talk to it once it does. Rotate the instance onto the current certificate authority during a maintenance window."},
			{Code: CodeDBICACertExpiringUrgent, Phrase: "server certificate expires in <N day(s)> — rotate now", Severity: domain.SevBroken, Source: "wave1", Detail: "The server certificate expires within a month, and every client that verifies the connection will refuse to talk to the instance the moment it does. Book the maintenance window now and rotate the instance onto the current certificate authority."},
			{Code: dbiCodeEngineDeprecated, Phrase: "engine version deprecated", Severity: domain.SevBroken, Source: "wave2", Detail: "AWS no longer supports this engine version, so it stops receiving security patches and will be force-upgraded on AWS's schedule. Upgrade to a supported version during a maintenance window of your choosing."},
		},
	},
	{
		Name:          "S3 Buckets",
		ShortName:     "s3",
		LifecycleKey:  "status",
		Aliases:       []string{"s3", "buckets"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Global(region, "s3/buckets/"+url.PathEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "name", Title: "Bucket Name", Path: "Name", Width: 36},
			{Title: "Region", Path: "BucketRegion", Width: 14},
			{Key: "creation_date", Title: "Creation Date", Path: "CreationDate", Width: 22},
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
			{Code: s3CodePublicAccessBlockIncomplete, Phrase: "public access block incomplete", Severity: domain.SevWarn, Source: "wave2", Detail: "This bucket does not set all four public-access settings, so it relies on the account-level block to stop a future policy or ACL from making it public, and that block may not be set either. Turn on all four settings on the bucket itself so it is safe regardless of the account."},
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
			{Key: "cluster_id", Title: "Cluster ID", Path: "ReplicationGroupId", Width: 28},
			{Key: "node_type", Title: "Node Type", Path: "CacheNodeType", Width: 18},
			{Key: "status", Title: "Status", Width: 32, SortKey: "status_raw"},
			{Key: "nodes", Title: "Nodes", Width: 8},
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
			{Code: CodeRedisCreateFailed, Phrase: "create failed — see events", Severity: domain.SevBroken, Source: "wave1", Detail: "The replication group never came up, so nothing can connect to it. Read the group's events for the failing step — subnet capacity, the parameter group, or the node type in that Availability Zone — then delete it and create it again once fixed."},
			{Code: CodeRedisCreating, Phrase: "creating — new group", Severity: domain.SevWarn, Source: "wave1", Detail: "Nodes are still being provisioned, so the endpoint is not ready to take connections. Wait for the group to become available before pointing an application at it."},
			{Code: CodeRedisDeleting, Phrase: "deleting — teardown", Severity: domain.SevWarn, Source: "wave1", Detail: "The group's nodes are being removed and its endpoint stops answering shortly. Confirm nothing still connects to it — a cache that disappears usually shows up as latency on the database behind it."},
			{Code: CodeRedisModifying, Phrase: "modifying — config change", Severity: domain.SevWarn, Source: "wave1", Detail: "A configuration change is being applied, and depending on the change the group may fail over or restart nodes while it runs. Expect brief connection resets, and hold off on further changes until it settles."},
			{Code: CodeRedisSnapshotting, Phrase: "snapshotting — backup running", Severity: domain.SevWarn, Source: "wave1", Detail: "A backup is being taken, which uses memory and I/O on the node doing it and can slow responses on a busy group. Nothing to fix; move the backup window outside peak hours if this keeps appearing during traffic."},
			{Code: CodeRedisShardIssue, Phrase: "shard <shard id>: <status>", Severity: domain.SevWarn, Source: "wave1", Detail: "One shard of this group is not in a normal state, so the keys that hash to it may be unavailable or served without a replica. Check that shard's nodes and its failover history before treating the whole group as healthy."},
			{Code: CodeRedisMultiAZWithoutAutoFailover, Phrase: "multi-AZ without auto-failover", Severity: domain.SevWarn, Source: "wave1", Detail: "The group has replicas in more than one Availability Zone but will not promote them by itself, so losing the primary means downtime until somebody fails it over by hand. Turn automatic failover on — the replicas are already being paid for."},
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
			{Key: "cluster_id", Title: "Cluster ID", Path: "DBClusterIdentifier", Width: 28},
			{Key: "engine_version", Title: "Version", Path: "EngineVersion", Width: 10},
			{Key: "status", Title: "Status", Width: 32, SortKey: "status_raw"},
			{Key: "instances", Title: "Instances", Path: "DBClusterMembers", Width: 10},
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
		Wave2: IssueEnricher{Fn: EnrichDBCMaintenance, Priority: 100, Reads: []string{"backup"}},
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
			{Code: CodeDBCFailed, Phrase: "failed: cluster operation", Severity: domain.SevBroken, Source: "wave1", Detail: "The cluster's last operation failed and it is not serving, so both its writer and its readers are unavailable. Read the cluster events for the failing step, and plan a snapshot restore — a cluster in this state rarely returns on its own."},
			{Code: CodeDBCEncryptionKeyUnreachable, Phrase: "encryption key unreachable", Severity: domain.SevBroken, Source: "wave1", Detail: "The KMS key protecting this cluster's volume cannot be used, so no node can read the data and the cluster will not start. Check whether the key is disabled, pending deletion, or whether its policy still allows the database service to use it."},
			{Code: CodeDBCIncompatibleParameters, Phrase: "parameter group incompatible", Severity: domain.SevBroken, Source: "wave1", Detail: "The cluster parameter group holds a setting the engine rejects, so the cluster will not come up with it applied. Correct the parameter the events list names. A dynamic parameter takes effect at once; a static one needs the affected instances rebooted individually, which is the only reboot DocumentDB offers."},
			{Code: CodeDBCNoWriter, Phrase: "no writer: reads only", Severity: domain.SevBroken, Source: "wave1", Detail: "The cluster has no writer instance, so every write fails while reads may still succeed and hide the outage from a shallow health check. Check whether a failover is stuck or the writer was deleted, then promote a reader or add an instance."},
			{Code: CodeDBCTransitional, Phrase: "<status>: in progress", Severity: domain.SevWarn, Source: "wave1", Detail: "The cluster is mid-operation, so it may fail over or drop connections before it settles. Wait for it to return to available rather than starting another change on top of this one."},
			{Code: CodeDBCDeletionProtectionOff, Phrase: "delete-protection off", Severity: domain.SevWarn, Source: "wave1", Detail: "Nothing stands between this cluster and a delete call. Aurora and DocumentDB still make you remove the member instances before the cluster volume goes, so it is not one click, but it is also nothing anybody has to think twice about. Turn deletion protection on so removing it takes a deliberate second step."},
			{Code: CodeDBCNotEncryptedAtRest, Phrase: "not encrypted at rest", Severity: domain.SevWarn, Source: "wave1", Detail: "The cluster's volume, its snapshots and its backups are stored unencrypted, and that cannot be changed in place. Snapshot it, copy the snapshot with a KMS key, and restore into a new encrypted cluster at the next opportunity for a cutover."},
			{Code: CodeDBCNoAutomatedBackups, Phrase: "no automated backups", Severity: domain.SevWarn, Source: "wave1", Detail: "Backup retention is zero, so there is no point-in-time recovery for this cluster and a bad migration can only be undone from a manual snapshot. Set a retention period that matches how much data loss you could actually accept."},
			{Code: dbcCodeMaintenanceOverdue, Phrase: "maintenance overdue", Severity: domain.SevBroken, Source: "wave2", Detail: "A pending maintenance action on this cluster is past the date AWS will apply it by, so AWS takes the outage at a time of its choosing rather than yours. Apply it in your own maintenance window now."},
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
			{Key: "table_name", Title: "Table Name", Path: "TableName", Width: 36},
			{Key: "status", Title: "Status", Width: 32},
			{Key: "item_count", Title: "Items", Path: "ItemCount", Width: 12},
			{Key: "size_bytes", Title: "Size", Width: 14, SortKey: "size_bytes_raw"},
			{Key: "billing_mode", Title: "Billing", Width: 16},
		},
		Color: colorDDB,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchDynamoDBTablesPage(ctx, c.DynamoDB, c.DynamoDB, continuationToken)
		}),
		Wave2:     IssueEnricher{Fn: EnrichDynamoDBPITR, Priority: 100, Reads: []string{"backup"}},
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
			{Code: CodeDDBKMSKeyInaccessible, Phrase: "kms key inaccessible", Severity: domain.SevBroken, Source: "wave1", Detail: "The KMS key encrypting this table cannot be used, so reads and writes fail until it is available again. Check whether the key was disabled or scheduled for deletion, and whether its policy still grants DynamoDB access."},
			{Code: CodeDDBArchivedKMSLost, Phrase: "archived: kms key lost", Severity: domain.SevBroken, Source: "wave1", Detail: "The table was archived because its encryption key became unusable, and it stays archived until the key is restored — the data cannot be read in this state. Recover the key if it is only pending deletion; once the key is gone the table's data is unrecoverable."},
			{Code: CodeDDBCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1", Detail: "The table is still being created and does not accept requests yet. Wait for it to become active before pointing an application at it or attaching a stream consumer."},
			{Code: CodeDDBUpdating, Phrase: "updating", Severity: domain.SevWarn, Source: "wave1", Detail: "A change is being applied — capacity mode, an index, a replica — and some operations are restricted while it runs. Wait for it to finish before starting another table change."},
			{Code: CodeDDBDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1", Detail: "The table and its indexes are being removed, and the data is not recoverable unless a backup exists. If this was not intended, check right now whether point-in-time recovery covers the window you need."},
			{Code: CodeDDBArchiving, Phrase: "archiving", Severity: domain.SevWarn, Source: "wave1", Detail: "The table is being moved to an archived state and is becoming unavailable for reads and writes. Nothing to do while it runs; restoring it later needs the encryption key it used."},
			{Code: ddbCodePITROff, Phrase: "point-in-time recovery disabled", Severity: domain.SevWarn, Source: "wave2", Detail: "Point-in-time recovery is off, so there is no way to roll this table back to a moment before a bad write and only explicit backups exist. Turn it on — it keeps a rolling 35-day window with no change to the table."},
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
		LifecycleKey:   "status",
		HumanizeFields: []string{"domain_processing_status"},
		Aliases:        []string{"opensearch", "os", "elasticsearch"},
		Category:       "DATABASES & STORAGE",
		CloudTrailKey:  "ResourceName:ID",
		ConsoleURL: func(r domain.Resource, region, _ string) string {
			return consolelink.Regional(region, "aos/home?region="+region+"#opensearch/domains/"+url.PathEscape(r.ID))
		},
		Columns: []domain.Column{
			{Key: "domain_name", Title: "Domain Name", Path: "DomainName", Width: 28},
			{Key: "status", Title: "Status", Width: 40},
			{Key: "engine_version", Title: "Engine Version", Path: "EngineVersion", Width: 16},
			{Key: "instance_type", Title: "Instance Type", Path: "ClusterConfig.InstanceType", Width: 22},
			{Key: "instance_count", Title: "Instances", Path: "ClusterConfig.InstanceCount", Width: 10},
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
			{Code: CodeOpenSearchIsolated, Phrase: "isolated: quarantined by AWS", Severity: domain.SevBroken, Source: "wave1", Detail: "AWS has isolated this domain, which it does when a cluster is unstable or out of disk, and a domain in that state serves no requests. Check its storage and shard health, free space or scale up, then work with AWS support to bring it back."},
			{Code: CodeOpenSearchProcessing, Phrase: "processing: config change in flight", Severity: domain.SevWarn, Source: "wave1", Detail: "A configuration change is being applied to the domain, which moves shards between nodes and can slow queries or briefly reject them. Wait for it to complete before making another change or judging the domain's performance."},
			{Code: opensearchCodeUpdateForced, Phrase: "software update forced soon", Severity: domain.SevWarn, Source: "wave1", Detail: "AWS will apply this service software update itself once the scheduled date passes, taking whatever maintenance window it chooses. Apply it yourself before that date so the blue/green deployment lands at a time you picked."},
			{Code: opensearchCodeEncryptionOff, Phrase: "encryption at rest off", Severity: domain.SevWarn, Source: "wave1", Detail: "Data at rest is stored unencrypted. Enabling encryption at rest requires creating a new domain and migrating data — it cannot be turned on in place."},
			{Code: opensearchCodePublic, Phrase: "reachable outside a VPC", Severity: domain.SevBroken, Source: "wave1", Detail: "The domain sits outside a VPC and its access policy allows any principal, so the search endpoint is reachable from the internet. Move the domain into a VPC, or scope the access policy to named principals."},
			{Code: opensearchCodeHTTPSNotForced, Phrase: "HTTPS not enforced", Severity: domain.SevWarn, Source: "wave1", Detail: "The domain accepts plaintext HTTP, so queries and results can be read off the wire. Turn on Require HTTPS in the domain's endpoint options."},
			{Code: opensearchCodeN2NOff, Phrase: "node-to-node encryption off", Severity: domain.SevWarn, Source: "wave1", Detail: "Traffic between the domain's own nodes is unencrypted. Node-to-node encryption can only be enabled on a domain that already has it configured at creation — recreate the domain if this data is sensitive."},
			DetailsDeniedFindingDef("opensearch", ""),
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
			{Key: "cluster_id", Title: "Cluster ID", Path: "ClusterIdentifier", Width: 36},
			{Key: "status", Title: "Status", Width: 34, SortKey: "cluster_status"},
			{Title: "Pending", Path: "PendingModifiedValues.NodeType", Width: 14},
			{Key: "node_type", Title: "Node Type", Path: "NodeType", Width: 16},
			{Key: "num_nodes", Title: "Nodes", Path: "NumberOfNodes", Width: 7},
			{Key: "db_name", Title: "Database", Path: "DBName", Width: 16},
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
			{Code: CodeRedshiftIncompatibleHSM, Phrase: "encryption key store unreachable", Severity: domain.SevBroken, Source: "wave1", Detail: "The cluster cannot reach the hardware security module holding its encryption key, so it will not come up. Check the client certificate for that module and the network path to it, then restore the connection."},
			{Code: CodeRedshiftIncompatibleNetwork, Phrase: "subnet group cannot host the cluster", Severity: domain.SevBroken, Source: "wave1", Detail: "The cluster's subnet group no longer provides what it needs — free addresses, or the Availability Zone it was created in — so it cannot start. Fix the subnet group, then restore the cluster."},
			{Code: CodeRedshiftIncompatibleParameters, Phrase: "parameter group rejected", Severity: domain.SevBroken, Source: "wave1", Detail: "A value in this cluster's parameter group is rejected, so the cluster will not come up with it applied. Correct the parameter group and reboot the cluster."},
			{Code: CodeRedshiftIncompatibleRestore, Phrase: "restore did not complete", Severity: domain.SevBroken, Source: "wave1", Detail: "The restore from snapshot failed, so this cluster holds no usable data. Check the snapshot's node type and encryption against the target, then restore again."},
			{Code: CodeRedshiftHardwareFailure, Phrase: "node hardware failed", Severity: domain.SevBroken, Source: "wave1", Detail: "A node's underlying hardware failed. Redshift replaces the node itself, but the cluster is degraded or unavailable until it does; watch the events and restore from the latest snapshot if it does not recover."},
			{Code: CodeRedshiftStorageFull, Phrase: "out of storage", Severity: domain.SevBroken, Source: "wave1", Detail: "The cluster has no disk left, so queries that need to spill fail and loads are rejected. Delete or unload cold tables, vacuum to reclaim space, then resize to more storage."},
			{Code: CodeRedshiftUnavailable, Phrase: "unavailable", Severity: domain.SevBroken, Source: "wave1", Detail: "The cluster is not answering queries, so every dashboard and job behind it is failing. Check the cluster events for the cause, and its most recent snapshot, before deciding between waiting and restoring."},
			{Code: CodeRedshiftFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1", Detail: "The cluster is in a failed state and will not serve queries again in place. Restore the most recent snapshot into a new cluster and repoint the applications at it."},
			{Code: CodeRedshiftCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1", Detail: "The cluster is still being provisioned and cannot take connections yet. Wait for it to become available before loading data or pointing tools at the endpoint."},
			{Code: CodeRedshiftModifying, Phrase: "modifying — cluster settings", Severity: domain.SevWarn, Source: "wave1", Detail: "A configuration change is being applied to the cluster, and it may reboot or run with reduced capacity before it settles. Wait for it to finish before starting another change or judging query times."},
			{Code: CodeRedshiftResizing, Phrase: "resizing", Severity: domain.SevWarn, Source: "wave1", Detail: "The cluster is changing node count or node type; depending on the resize type it is read-only or unavailable for part of it. Hold off on loads until it is done, and expect query plans to change afterwards."},
			{Code: CodeRedshiftRebooting, Phrase: "rebooting", Severity: domain.SevWarn, Source: "wave1", Detail: "The cluster is restarting, so open connections are dropped and queries in flight are lost. Wait for it to come back; applications should reconnect on their own."},
			{Code: CodeRedshiftRenaming, Phrase: "renaming", Severity: domain.SevWarn, Source: "wave1", Detail: "The cluster identifier is changing, which changes its endpoint address, so anything holding the old name stops connecting. Update the connection strings and any DNS record pointing at the old endpoint."},
			{Code: CodeRedshiftDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1", Detail: "The cluster is being removed, and unless a final snapshot was requested its data goes with it. If this was not intended, check now whether a snapshot exists to restore from."},
			{Code: CodeRedshiftMaintenance, Phrase: "maintenance", Severity: domain.SevWarn, Source: "wave1", Detail: "The cluster is in its maintenance window and AWS is applying updates, so it can be briefly unavailable. Nothing to do beyond expecting the interruption; move the window if it clashes with your load schedule."},
			{Code: CodeRedshiftAvailabilityModifying, Phrase: "modifying — availability affected", Severity: domain.SevWarn, Source: "wave1", Detail: "Redshift reports the cluster's availability as changing rather than steady, so queries may be refused or slow while the change lands. Wait for it to report available again before treating a query failure as an application fault."},
			{Code: CodeRedshiftPendingChange, Phrase: "pending change queued", Severity: domain.SevWarn, Source: "wave1", Detail: "A modification is queued for the next maintenance window, so the cluster's live configuration is not the one shown as desired. Check what is pending and, if it needs an outage, apply it at a time you choose."},
			{Code: CodeRedshiftMaintenanceDeferred, Phrase: "maintenance deferred", Severity: domain.SevWarn, Source: "wave1", Detail: "Maintenance on this cluster has been postponed, so it runs without updates AWS has scheduled, and the deferral has an end date. Plan a window before that date, or AWS will pick one for you."},
			{Code: CodeRedshiftPubliclyAccessible, Phrase: "public endpoint", Severity: domain.SevWarn, Source: "wave1", Detail: "The cluster answers on a routable address outside the VPC, so only its security groups separate the warehouse from the internet. Turn public access off and reach it over a private link unless an outside system genuinely needs it."},
			{Code: CodeRedshiftUnencryptedAtRest, Phrase: "unencrypted at rest", Severity: domain.SevWarn, Source: "wave1", Detail: "The cluster's blocks and its snapshots are stored unencrypted. Turning encryption on requires a cluster migration, so plan it into a maintenance window rather than leaving it indefinitely."},
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
			{Key: "name", Title: "Name", Path: "Name", Width: 28},
			{Key: "file_system_id", Title: "File System ID", Path: "FileSystemId", Width: 22},
			{Key: "status", Title: "Status", Width: 24},
			{Key: "performance_mode", Title: "Perf Mode", Path: "PerformanceMode", Width: 16},
			{Key: "encrypted", Title: "Encrypted", Path: "Encrypted", Width: 10},
			{Key: "mount_targets", Title: "Mounts", Path: "NumberOfMountTargets", Width: 8},
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
			{Code: CodeEFSError, Phrase: "error", Severity: domain.SevBroken, Source: "wave1", Detail: "The filesystem is in an error state, so mounts fail and anything depending on it is stalled. Check its events and its KMS key, and restore from a backup if it does not recover."},
			{Code: CodeEFSNoMountTargets, Phrase: "no mount targets", Severity: domain.SevBroken, Source: "wave1", Detail: "The filesystem has no mount target in any subnet, so nothing in the VPC can mount it however correct the client configuration is. Create a mount target in each Availability Zone that runs clients."},
			{Code: CodeEFSCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1", Detail: "The filesystem is still being created and cannot be mounted yet. Wait for it to become available, then add its mount targets."},
			{Code: CodeEFSUpdating, Phrase: "updating", Severity: domain.SevWarn, Source: "wave1", Detail: "A change to the filesystem is being applied, such as its throughput mode or lifecycle policy. Existing mounts keep working; expect throughput to change once it lands."},
			{Code: CodeEFSDeleting, Phrase: "deleting", Severity: domain.SevWarn, Source: "wave1", Detail: "The filesystem is being removed and its data goes with it; mounts still open start failing. If this was not intended, check whether an AWS Backup recovery point exists before it completes."},
			{Code: efsCodeMountTargetDown, Phrase: "mount target down", Severity: domain.SevBroken, Source: "wave2", Detail: "A mount target for this filesystem is unavailable, so clients in that Availability Zone cannot mount it while clients elsewhere still can — which presents as a partial, confusing outage. Check the mount target's subnet and security groups, and whether the subnet still has free addresses."},
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
			{Key: "snapshot_id", Title: "Snapshot ID", Path: "DBSnapshotIdentifier", Width: 36},
			{Key: "db_instance", Title: "DB Instance", Path: "DBInstanceIdentifier", Width: 28},
			{Key: "status", Title: "Status", Width: 32},
			{Key: "engine", Title: "Engine", Path: "Engine", Width: 12},
			{Key: "snapshot_type", Title: "Type", Path: "SnapshotType", Width: 12},
			{Key: "created", Title: "Created", Path: "SnapshotCreateTime", Width: 22},
		},
		Color: colorDBISnap,
		Fetcher: fetcherWithClients(func(ctx context.Context, c *ServiceClients, continuationToken string) (resource.FetchResult, error) {
			return FetchDBISnapshotsPage(ctx, c.RDS, continuationToken)
		}),
		Wave2:     IssueEnricher{Fn: enrichDBISnapCrossRef, Priority: 100, Reads: []string{"dbi"}},
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
			{Code: CodeDBISnapFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1", Detail: "This snapshot did not complete, so it holds no restorable copy and any recovery plan naming it has a gap. Take a new snapshot of the instance and delete this one."},
			{Code: CodeDBISnapIncompatible, Phrase: "<incompatible-* status>", Severity: domain.SevBroken, Source: "wave1", Detail: "The snapshot cannot be restored as it stands, usually because its engine version or options are no longer offered in this account or region. Copy it and upgrade the copy's engine version, or restore into a configuration matching what it was taken from."},
			{Code: CodeDBISnapCreating, Phrase: "creating: <pct>%", Severity: domain.SevWarn, Source: "wave1", Detail: "The snapshot is still being written and cannot be restored from or copied yet. Wait for it to complete before counting it as this window's recovery point."},
			{Code: CodeDBISnapTransitional, Phrase: "<status>", Severity: domain.SevWarn, Source: "wave1", Detail: "The snapshot is being modified or copied and is not usable for a restore until it settles. Wait for it to reach an available state."},
			{Code: CodeDBISnapUnencrypted, Phrase: "unencrypted", Severity: domain.SevWarn, Source: "wave1", Detail: "The snapshot's contents are stored unencrypted, and any instance restored from it starts unencrypted too. Copy it with a KMS key and restore from the copy. An automated snapshot cannot be deleted on its own, so remove the original only if it is a manual one and otherwise let the retention period expire it."},
			{Code: dbiSnapOrphanCode, Phrase: "orphan: source DB deleted", Severity: domain.SevBroken, Source: "wave2", Detail: "The instance this snapshot was taken from no longer exists, so nothing is producing newer recovery points from it. Check what else covers that data before you decide: this may be the last copy or one of several. Give it a retention decision and an owner, or delete it, because it is billed for its stored size either way."},
			{Code: dbiSnapPastRetentionCode, Phrase: "automated, <N>d past retention", Severity: domain.SevBroken, Source: "wave2", Detail: "This automated snapshot has outlived the instance's retention period, so something other than the backup policy is keeping it and nobody is managing its lifecycle. An automated snapshot cannot be renamed into a managed manual one, so copy it if the data is worth keeping deliberately, and otherwise adjust the retention setting or remove the retained automated backup that holds it."},
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
			{Key: "snapshot_id", Title: "Snapshot ID", Path: "DBClusterSnapshotIdentifier", Width: 36},
			{Key: "cluster_id", Title: "Cluster ID", Path: "DBClusterIdentifier", Width: 28},
			{Key: "status", Title: "Status", Width: 32},
			{Key: "engine", Title: "Engine", Path: "Engine", Width: 12},
			{Key: "snapshot_type", Title: "Type", Path: "SnapshotType", Width: 12},
			{Key: "snapshot_create_time", Title: "Created", Path: "SnapshotCreateTime", Width: 22},
			{Key: "storage_type", Title: "Storage", Path: "StorageType", Width: 10},
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
		Wave2: IssueEnricher{Fn: enrichDBCSnapCrossRef, Priority: 100, Reads: []string{"dbc"}},
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
			{Code: CodeDBCSnapFailed, Phrase: "failed", Severity: domain.SevBroken, Source: "wave1", Detail: "The cluster snapshot did not complete, so it cannot be restored and the recovery point you think you have for that time does not exist. Take a new snapshot and delete this one."},
			{Code: CodeDBCSnapIncompatible, Phrase: "<incompatible-* status>", Severity: domain.SevBroken, Source: "wave1", Detail: "This snapshot cannot be restored in its current form, usually because its engine version or options are no longer offered. Copy it and upgrade the copy, or restore into a configuration matching the cluster it came from."},
			{Code: CodeDBCSnapCreating, Phrase: "creating", Severity: domain.SevWarn, Source: "wave1", Detail: "The snapshot is still being written and is not restorable yet. Wait for it to complete before treating it as a recovery point."},
			{Code: CodeDBCSnapTransitional, Phrase: "<status>", Severity: domain.SevWarn, Source: "wave1", Detail: "The snapshot is being copied or modified and cannot be restored from while that runs. Wait for it to become available."},
			{Code: CodeDBCSnapManualUnused, Phrase: "manual, unused <N>d", Severity: domain.SevWarn, Source: "wave1", Detail: "This manual snapshot has sat unused for a long time, and manual snapshots are never removed automatically, so it is billed indefinitely until somebody deletes it. Confirm whether it is a deliberate archive, and delete it if it is not."},
			{Code: CodeDBCSnapUnencrypted, Phrase: "unencrypted", Severity: domain.SevWarn, Source: "wave1", Detail: "The snapshot is stored unencrypted, and any cluster restored from it inherits that. Copy it with a KMS key and restore from the copy. Delete the original only if it is a manual snapshot; an automated one goes when its retention period expires."},
			{Code: dbcSnapOrphanCode, Phrase: "orphan: source cluster deleted", Severity: domain.SevBroken, Source: "wave2", Detail: "The cluster this snapshot came from is gone, so nothing is producing newer recovery points from it. Check the other snapshots and backups of that cluster before deciding whether this is the last copy. Give it a retention decision and an owner, or delete it."},
			{Code: dbcSnapPastRetentionCode, Phrase: "automated, <N>d past retention", Severity: domain.SevBroken, Source: "wave2", Detail: "This automated cluster snapshot is older than the retention period that should have removed it, so it is outliving the policy meant to manage it. Copy it to a manual snapshot if the data is worth keeping deliberately; otherwise adjust the retention setting or remove the retained automated backup that holds it."},
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
			{Code: CodeDBIEventFailure, Phrase: "failure", Severity: domain.SevBroken, Source: "wave1", Detail: "A failure event was recorded against this instance, so something the service tried to do on your behalf did not work. Read the message — failover, backup and storage failures all land here — and act on the specific one before it repeats."},
			{Code: CodeDBIEventLowStorage, Phrase: "low storage", Severity: domain.SevBroken, Source: "wave1", Detail: "The service is warning that this instance is close to running out of disk, and writes stop when it does. Raise the allocated storage now and turn on storage autoscaling so it does not come back."},
			{Code: CodeDBIEventFailover, Phrase: "failover", Severity: domain.SevWarn, Source: "wave1", Detail: "The instance failed over to its standby, so connections were dropped and the writer is now in the other Availability Zone. Check what triggered it — hardware, patching or a manual reboot — and confirm applications reconnected."},
			{Code: CodeDBIEventRecovery, Phrase: "recovery", Severity: domain.SevWarn, Source: "wave1", Detail: "A recovery action was performed on this instance, which usually means it restarted the database after a crash or a stall. Look at the events just before it for the cause, and check the application for errors during that window."},
		},
	},
}
