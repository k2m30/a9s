package catalog

import (
	"strings"
	"time"

	"github.com/k2m30/a9s/v3/internal/domain"
)

func colorDBI(r domain.Resource) domain.Color {
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
	status := r.Status
	if status == "" {
		status = r.Fields["status"]
	}
	stripped := stripFindingSuffix(status)
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
	phrase := stripFindingSuffix(r.Fields["status"])
	if phrase == "" {
		phrase = stripFindingSuffix(r.Status)
	}
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
	status := r.Fields["status"]
	if status == "" {
		status = r.Status
	}
	phrase := stripFindingSuffix(status)
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

var databasesTypes = []ResourceTypeDef{ //nolint:gochecknoglobals // static catalog: intentional package-level var
	{
		Name:          "DB Instances",
		ShortName:     "dbi",
		Aliases:       []string{"dbi", "rds", "databases", "db-instances"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:Fields.arn",
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
	},
	{
		Name:          "ElastiCache Redis",
		ShortName:     "redis",
		Aliases:       []string{"redis", "elasticache"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "cluster_id", Title: "Cluster ID", Width: 28, Sortable: true},
			{Key: "node_type", Title: "Node Type", Width: 18, Sortable: true},
			{Key: "status", Title: "Status", Width: 32, Sortable: true},
			{Key: "nodes", Title: "Nodes", Width: 8, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 40, Sortable: false},
		},
		Color: colorRedis,
	},
	{
		Name:          "DB Clusters",
		ShortName:     "dbc",
		Aliases:       []string{"dbc", "docdb", "clusters", "db-clusters"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:Fields.arn",
		Columns: []domain.Column{
			{Key: "cluster_id", Title: "Cluster ID", Width: 28, Sortable: true},
			{Key: "engine_version", Title: "Version", Width: 10, Sortable: true},
			{Key: "status", Title: "Status", Width: 32, Sortable: true},
			{Key: "instances", Title: "Instances", Width: 10, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 48, Sortable: false},
		},
		Color: colorDBC,
	},
	{
		Name:          "DynamoDB Tables",
		ShortName:     "ddb",
		Aliases:       []string{"ddb", "dynamodb", "dynamo"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "table_name", Title: "Table Name", Width: 36, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "item_count", Title: "Items", Width: 12, Sortable: true},
			{Key: "size_bytes", Title: "Size", Width: 14, Sortable: true},
			{Key: "billing_mode", Title: "Billing", Width: 16, Sortable: true},
		},
		Color: colorDDB,
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
	},
	{
		Name:          "Redshift Clusters",
		ShortName:     "redshift",
		Aliases:       []string{"redshift", "redshift-clusters"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "cluster_id", Title: "Cluster ID", Width: 36, Sortable: true},
			{Key: "status", Title: "Status", Width: 34, Sortable: true},
			{Key: "node_type", Title: "Node Type", Width: 16, Sortable: true},
			{Key: "num_nodes", Title: "Nodes", Width: 7, Sortable: true},
			{Key: "db_name", Title: "Database", Width: 16, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 44, Sortable: false},
		},
		Color: colorRedshift,
	},
	{
		Name:          "EFS File Systems",
		ShortName:     "efs",
		Aliases:       []string{"efs", "file-systems"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "file_system_id", Title: "File System ID", Width: 22, Sortable: true},
			{Key: "status", Title: "Status", Width: 24, Sortable: true},
			{Key: "performance_mode", Title: "Perf Mode", Width: 16, Sortable: true},
			{Key: "encrypted", Title: "Encrypted", Width: 10, Sortable: true},
			{Key: "mount_targets", Title: "Mounts", Width: 8, Sortable: true},
		},
		Color: colorEFS,
	},
	{
		Name:          "DB Instance Snapshots",
		ShortName:     "dbi-snap",
		Aliases:       []string{"dbi-snap", "rds-snapshots", "db-snapshots"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:ID",
		Columns: []domain.Column{
			{Key: "snapshot_id", Title: "Snapshot ID", Width: 36, Sortable: true},
			{Key: "db_instance", Title: "DB Instance", Width: 28, Sortable: true},
			{Key: "status", Title: "Status", Width: 32, Sortable: true},
			{Key: "engine", Title: "Engine", Width: 12, Sortable: true},
			{Key: "snapshot_type", Title: "Type", Width: 12, Sortable: true},
			{Key: "created", Title: "Created", Width: 22, Sortable: true},
		},
		Color: colorDBISnap,
	},
	{
		Name:          "DB Cluster Snapshots",
		ShortName:     "dbc-snap",
		Aliases:       []string{"dbc-snap", "docdb-snapshots", "cluster-snapshots"},
		Category:      "DATABASES & STORAGE",
		CloudTrailKey: "ResourceName:ID",
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
	},
}
