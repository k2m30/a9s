package aws

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/k2m30/a9s/v3/internal/catalog"
	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func colorDBI(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
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

func colorS3(_ domain.Resource) domain.Color { return domain.ColorHealthy }

func colorRedis(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
		return c
	}
	phrase := stripFindingSuffix(r.Fields["status"])
	if phrase == "deleted" {
		return domain.ColorDim
	}
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
	if c, ok := colorFromWave1(r); ok {
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
	if c, ok := colorFromWave1(r); ok {
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
	case "PITR off":
		return domain.ColorHealthy
	}
	return domain.ColorHealthy
}

func colorOpenSearch(r domain.Resource) domain.Color {
	if r.Fields["deleted"] == "true" {
		return domain.ColorDim
	}
	if c, ok := colorFromWave1(r); ok {
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
	if c, ok := colorFromWave1(r); ok {
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
	if c, ok := colorFromWave1(r); ok {
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
	if c, ok := colorFromWave1(r); ok {
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

func colorDBCSnap(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
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
	if r.Fields["storage_encrypted"] == "false" {
		return domain.ColorWarning
	}
	if r.Fields["snapshot_type"] == "manual" {
		if ts, err := time.Parse("2006-01-02 15:04", r.Fields["snapshot_create_time"]); err == nil {
			if time.Since(ts) > 365*24*time.Hour {
				return domain.ColorWarning
			}
		}
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
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchRDSInstancesPage(ctx, c.RDS, continuationToken)
		},
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
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkDbiAlarm, NeedsTargetCache: true},
			{TargetType: "dbi-snap", DisplayName: "DB Instance Snapshots", Checker: checkDbiDBISnap, NeedsTargetCache: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkDBILogs, NeedsTargetCache: true},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkDbiVPC},
			{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkDbiSecrets, NeedsTargetCache: true},
			{TargetType: "dbc", DisplayName: "RDS Clusters", Checker: checkDbiDBC, NeedsTargetCache: true},
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
	},
	{
		Name:          "S3 Buckets",
		ShortName:     "s3",
		Aliases:       []string{"s3", "buckets"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:ID",
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
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			// Related-panel contract (docs/resources/s3.md §2): lambda/sns/sqs
			// pivots must resolve non-zero when this bucket has a matching
			// notification target. Those checkers read Fields["notification_*"],
			// which can only be populated by GetBucketNotificationConfiguration
			// — so the list path must run it per-bucket. Accepts N+1 per page
			// (cheap API, typically ≤50 buckets per AWS account) in exchange
			// for having the notification pivots actually work.
			return FetchS3BucketsPageWithNotifications(ctx, c.S3, c.S3, continuationToken)
		},
		Wave2: IssueEnricher{Fn: EnrichS3PublicAccessBlock, Priority: 100},
		FieldKeys: []string{
			"name",
			"bucket_name",
			"creation_date",
			"notification_lambda",
			"notification_sqs",
			"notification_sns",
		},
		Related: []domain.RelatedDef{
			{TargetType: "trail", DisplayName: "CloudTrail Trails", Checker: checkS3Trail, NeedsTargetCache: true},
			{TargetType: "cf", DisplayName: "CloudFront", Checker: checkS3CF, NeedsTargetCache: true},
			{TargetType: "lambda", DisplayName: "Lambda (notifications)", Checker: checkS3Lambda},
			{TargetType: "sns", DisplayName: "SNS (notifications)", Checker: checkS3SNS},
			{TargetType: "sqs", DisplayName: "SQS (notifications)", Checker: checkS3SQS},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkS3CFN},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkS3KMS},
			{TargetType: "s3", DisplayName: "Access Log Bucket", Checker: checkS3Logs},
			{TargetType: "athena", DisplayName: "Athena WorkGroups", Checker: checkS3Athena},
			{TargetType: "glue", DisplayName: "Glue Jobs", Checker: checkS3Glue},
			{TargetType: "backup", DisplayName: "Backup", Checker: checkS3Backup},
			{TargetType: "eb-rule", DisplayName: "EventBridge Rules", Checker: checkS3EBRule},
			{TargetType: "r53", DisplayName: "Route 53", Checker: checkS3R53},
			{TargetType: "role", DisplayName: "IAM Roles", Checker: checkS3Role},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("s3")},
		},
		IssueEnricherFieldKeys: []string{"status"},
	},
	{
		Name:          "ElastiCache Redis",
		ShortName:     "redis",
		Aliases:       []string{"redis", "elasticache"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "cluster_id", Title: "Cluster ID", Width: 28, Sortable: true},
			{Key: "node_type", Title: "Node Type", Width: 18, Sortable: true},
			{Key: "status", Title: "Status", Width: 32, Sortable: true},
			{Key: "nodes", Title: "Nodes", Width: 8, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 40, Sortable: false},
		},
		Color: colorRedis,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchRedisPage(ctx, c.ElastiCache, continuationToken)
		},
		FieldKeys: []string{"cluster_id", "node_type", "status", "nodes", "endpoint", "arn"},
		Related: []domain.RelatedDef{
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkRedisAlarms, NeedsTargetCache: true},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkRedisCFN, NeedsTargetCache: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkRedisCtEvents, NeedsTargetCache: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkRedisKMS, NeedsTargetCache: false},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkRedisLogs, NeedsTargetCache: true},
			{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkRedisSecrets, NeedsTargetCache: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkRedisSG, NeedsTargetCache: true},
			{TargetType: "sns", DisplayName: "SNS Topics", Checker: checkRedisSNS, NeedsTargetCache: true},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkRedisSubnet, NeedsTargetCache: true},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkRedisVPC, NeedsTargetCache: false},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "KmsKeyId", TargetType: "kms"},
		},
	},
	{
		Name:          "DB Clusters",
		ShortName:     "dbc",
		Aliases:       []string{"dbc", "docdb", "clusters", "db-clusters"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:Fields.arn",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "cluster_id", Title: "Cluster ID", Width: 28, Sortable: true},
			{Key: "engine_version", Title: "Version", Width: 10, Sortable: true},
			{Key: "status", Title: "Status", Width: 32, Sortable: true},
			{Key: "instances", Title: "Instances", Width: 10, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 48, Sortable: false},
		},
		Color: colorDBC,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
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
		},
		Wave2: IssueEnricher{Fn: EnrichDBCMaintenance, Priority: 100},
		FieldKeys: []string{
			"cluster_id", "engine_version", "status", "instances", "endpoint", "arn",
			"has_writer", "writer_count", "deletion_protection", "storage_encrypted",
			"backup_retention_period",
		},
		Related: []domain.RelatedDef{
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkDbcSG},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkDbcAlarm, NeedsTargetCache: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkDbcLogs, NeedsTargetCache: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkDbcKMS},
			{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkDbcSecrets, NeedsTargetCache: true},
			{TargetType: "dbi", DisplayName: "RDS Instances", Checker: checkDbcDBI, NeedsTargetCache: true},
			{TargetType: "dbc-snap", DisplayName: "DB Cluster Snapshots", Checker: checkDbcDbcSnap, NeedsTargetCache: true},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkDbcSubnet},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkDbcVPC},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkDbcCTEvents},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VpcSecurityGroups.VpcSecurityGroupId", TargetType: "sg"},
			{FieldPath: "KmsKeyId", TargetType: "kms"},
		},
	},
	{
		Name:          "DynamoDB Tables",
		ShortName:     "ddb",
		Aliases:       []string{"ddb", "dynamodb", "dynamo"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "table_name", Title: "Table Name", Width: 36, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "item_count", Title: "Items", Width: 12, Sortable: true},
			{Key: "size_bytes", Title: "Size", Width: 14, Sortable: true},
			{Key: "billing_mode", Title: "Billing", Width: 16, Sortable: true},
		},
		Color: colorDDB,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchDynamoDBTablesPage(ctx, c.DynamoDB, c.DynamoDB, continuationToken)
		},
		Wave2:     IssueEnricher{Fn: EnrichDynamoDBPITR, Priority: 100},
		FieldKeys: []string{"table_name", "status", "item_count", "size_bytes", "billing_mode"},
		Related: []domain.RelatedDef{
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkDdbKMS},
			{TargetType: "alarm", DisplayName: "CloudWatch Alarms", Checker: checkDdbAlarm, NeedsTargetCache: true},
			{TargetType: "lambda", DisplayName: "Lambda Functions", Checker: checkDdbLambda},
			{TargetType: "kinesis", DisplayName: "Kinesis Streams", Checker: checkDdbKinesis},
			{TargetType: "backup", DisplayName: "Backup Plans", Checker: checkDdbBackup},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkDdbLogs, NeedsTargetCache: true},
			{TargetType: "vpce", DisplayName: "VPC Endpoints", Checker: checkDdbVPCE, NeedsTargetCache: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("ddb")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "SSEDescription.KMSMasterKeyArn", TargetType: "kms"},
		},
	},
	{
		Name:          "OpenSearch Domains",
		ShortName:     "opensearch",
		Aliases:       []string{"opensearch", "os", "elasticsearch"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "domain_name", Title: "Domain Name", Width: 28, Sortable: true},
			{Key: "engine_version", Title: "Engine Version", Width: 16, Sortable: true},
			{Key: "instance_type", Title: "Instance Type", Width: 22, Sortable: true},
			{Key: "instance_count", Title: "Instances", Width: 10, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 48, Sortable: false},
		},
		Color: colorOpenSearch,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			resources, err := FetchOpenSearchDomains(ctx, c.OpenSearch, c.OpenSearch)
			if err != nil {
				return resource.FetchResult{}, err
			}
			return resource.FetchResult{
				Resources:  resources,
				Pagination: &resource.PaginationMeta{IsTruncated: false, TotalHint: len(resources), PageSize: len(resources)},
			}, nil
		},
		Wave2: IssueEnricher{Fn: EnrichOpenSearchDomains, Priority: 100},
		FieldKeys: []string{
			"domain_name", "engine_version", "instance_type", "instance_count", "endpoint",
			"status", "domain_processing_status",
			"deleted", "processing", "upgrade_processing",
			"service_software_update_available", "encryption_at_rest_enabled",
			"automated_update_date", "current_version", "new_version",
		},
		Related: []domain.RelatedDef{
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkOpenSearchAlarms, NeedsTargetCache: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkOpenSearchLogs, NeedsTargetCache: false},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkOpenSearchSG},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkOpenSearchVPC},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkOpenSearchKMS},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkOpenSearchCFN},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkOpenSearchSubnet},
			{TargetType: "acm", DisplayName: "ACM Certificates", Checker: checkOpenSearchACM, NeedsTargetCache: true},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("opensearch")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "EncryptionAtRestOptions.KmsKeyId", TargetType: "kms"},
			{FieldPath: "VPCOptions.VPCId", TargetType: "vpc"},
			{FieldPath: "VPCOptions.SubnetIds", TargetType: "subnet"},
			{FieldPath: "VPCOptions.SecurityGroupIds", TargetType: "sg"},
		},
	},
	{
		Name:          "Redshift Clusters",
		ShortName:     "redshift",
		Aliases:       []string{"redshift", "redshift-clusters"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "cluster_id", Title: "Cluster ID", Width: 36, Sortable: true},
			{Key: "status", Title: "Status", Width: 34, Sortable: true},
			{Key: "node_type", Title: "Node Type", Width: 16, Sortable: true},
			{Key: "num_nodes", Title: "Nodes", Width: 7, Sortable: true},
			{Key: "db_name", Title: "Database", Width: 16, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 44, Sortable: false},
		},
		Color: colorRedshift,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchRedshiftClustersPage(ctx, c.Redshift, continuationToken)
		},
		FieldKeys: []string{
			"cluster_id", "status", "cluster_status", "node_type", "num_nodes",
			"db_name", "endpoint", "publicly_accessible", "encrypted",
			"cluster_availability_status",
		},
		Related: []domain.RelatedDef{
			{TargetType: "alarm", DisplayName: "CW Alarms", Checker: checkRedshiftAlarms, NeedsTargetCache: true},
			{TargetType: "sg", DisplayName: "Security Groups", Checker: checkRedshiftSG},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkRedshiftVPC},
			{TargetType: "role", DisplayName: "IAM Role", Checker: checkRedshiftRole},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkRedshiftKMS},
			{TargetType: "cfn", DisplayName: "CloudFormation", Checker: checkRedshiftCFN, NeedsTargetCache: true},
			{TargetType: "secrets", DisplayName: "Secrets Manager", Checker: checkRedshiftSecrets, NeedsTargetCache: true},
			{TargetType: "logs", DisplayName: "Log Groups", Checker: checkRedshiftLogs},
			{TargetType: "s3", DisplayName: "S3 Buckets", Checker: checkRedshiftS3},
			{TargetType: "subnet", DisplayName: "Subnets", Checker: checkRedshiftSubnet},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: ctEventsCheckerFor("redshift")},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VpcId", TargetType: "vpc"},
		},
	},
	{
		Name:          "EFS File Systems",
		ShortName:     "efs",
		Aliases:       []string{"efs", "file-systems"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "file_system_id", Title: "File System ID", Width: 22, Sortable: true},
			{Key: "status", Title: "Status", Width: 24, Sortable: true},
			{Key: "performance_mode", Title: "Perf Mode", Width: 16, Sortable: true},
			{Key: "encrypted", Title: "Encrypted", Width: 10, Sortable: true},
			{Key: "mount_targets", Title: "Mounts", Width: 8, Sortable: true},
		},
		Color: colorEFS,
		Wave2: IssueEnricher{Fn: EnrichEFSMountTargets, Priority: 100},
	},
	{
		Name:          "DB Instance Snapshots",
		ShortName:     "dbi-snap",
		Aliases:       []string{"dbi-snap", "rds-snapshots", "db-snapshots"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
		Columns: []domain.Column{
			{Key: "snapshot_id", Title: "Snapshot ID", Width: 36, Sortable: true},
			{Key: "db_instance", Title: "DB Instance", Width: 28, Sortable: true},
			{Key: "status", Title: "Status", Width: 32, Sortable: true},
			{Key: "engine", Title: "Engine", Width: 12, Sortable: true},
			{Key: "snapshot_type", Title: "Type", Width: 12, Sortable: true},
			{Key: "created", Title: "Created", Width: 22, Sortable: true},
		},
		Color: colorDBISnap,
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
			return FetchDBISnapshotsPage(ctx, c.RDS, continuationToken)
		},
		Wave2:     IssueEnricher{Fn: enrichDBISnapCrossRef, Priority: 100},
		FieldKeys: []string{"snapshot_id", "db_instance", "status", "engine", "snapshot_type", "created", "arn"},
		Related: []domain.RelatedDef{
			{TargetType: "dbi", DisplayName: "DB Instances", Checker: checkDBISnapDBI, NeedsTargetCache: true},
			{TargetType: "kms", DisplayName: "KMS Keys", Checker: checkDBISnapKMS, NeedsTargetCache: true},
			{TargetType: "backup", DisplayName: "Backup Plans", Checker: checkDBISnapBackup},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkDBISnapCTEvents, NeedsTargetCache: true},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "DBInstanceIdentifier", TargetType: "dbi"},
			{FieldPath: "KmsKeyId", TargetType: "kms"},
		},
	},
	{
		Name:          "DB Cluster Snapshots",
		ShortName:     "dbc-snap",
		Aliases:       []string{"dbc-snap", "docdb-snapshots", "cluster-snapshots"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:ID",
		LifecycleKey:  "status",
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
		Fetcher: func(ctx context.Context, clients any, continuationToken string) (resource.FetchResult, error) {
			c, ok := clients.(*ServiceClients)
			if !ok || c == nil {
				return resource.FetchResult{}, fmt.Errorf("AWS clients not initialized")
			}
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
		},
		Wave2: IssueEnricher{Fn: enrichDBCSnapCrossRef, Priority: 100},
		FieldKeys: []string{
			"snapshot_id", "cluster_id", "status", "engine", "snapshot_type",
			"snapshot_create_time", "storage_type", "storage_encrypted",
		},
		Related: []domain.RelatedDef{
			{TargetType: "dbc", DisplayName: "DocumentDB Cluster", Checker: checkDbcSnapDBC, NeedsTargetCache: true},
			{TargetType: "kms", DisplayName: "KMS Key", Checker: checkDbcSnapKMS},
			{TargetType: "vpc", DisplayName: "VPC", Checker: checkDbcSnapVPC},
			{TargetType: "backup", DisplayName: "Backup Plans", Checker: checkDbcSnapBackup},
			{TargetType: "ct-events", DisplayName: "CloudTrail Events", Checker: checkDbcSnapCTEvents},
		},
		Navigable: []domain.NavigableField{
			{FieldPath: "VpcId", TargetType: "vpc"},
			{FieldPath: "KmsKeyId", TargetType: "kms"},
		},
	},
}
