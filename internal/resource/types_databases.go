package resource

import "strings"

// rdsInstanceColor maps RDS/DocDB instance and cluster status strings to a Color.
// Used by dbi and dbc types which share the same status vocabulary.
func rdsInstanceColor(status string) Color {
	switch status {
	case "available":
		return ColorHealthy
	case "creating", "modifying", "backing-up", "rebooting", "upgrading",
		"renaming", "resetting-master-credentials", "storage-optimization",
		"starting", "stopping":
		return ColorWarning
	case "stopped", "restore-error", "storage-full", "failed":
		return ColorBroken
	case "deleting":
		return ColorWarning
	}
	// incompatible-* and inaccessible-encryption-credentials patterns
	if strings.HasPrefix(status, "incompatible-") || strings.HasPrefix(status, "inaccessible-") {
		return ColorBroken
	}
	return ColorHealthy
}

func databasesResourceTypes() []ResourceTypeDef {
	return []ResourceTypeDef{
		{
			Name:          "DB Instances",
			ShortName:     "dbi",
			Aliases:       []string{"dbi", "rds", "databases", "db-instances"},
			Category:      "DATABASES & STORAGE",
			CloudTrailKey: "ResourceName:Fields.arn",
			Columns: []Column{
				{Key: "db_identifier", Title: "DB Identifier", Width: 28, Sortable: true},
				{Key: "engine", Title: "Engine", Width: 12, Sortable: true},
				{Key: "engine_version", Title: "Version", Width: 10, Sortable: true},
				{Key: "status", Title: "Status", Width: 14, Sortable: true},
				{Key: "class", Title: "Class", Width: 16, Sortable: true},
				{Key: "endpoint", Title: "Endpoint", Width: 40, Sortable: false},
				{Key: "multi_az", Title: "Multi-AZ", Width: 10, Sortable: true},
			},
			Color: func(r Resource) Color {
				// Prefer "status" (set by fetcher); fall back to legacy "db_instance_status".
				status := r.Fields["status"]
				if status == "" {
					status = r.Fields["db_instance_status"]
				}
				return rdsInstanceColor(status)
			},
			Children: []ChildViewDef{{
				ChildType:      "dbi_events",
				Key:            "enter",
				ContextKeys:    map[string]string{"db_identifier": "ID"},
				DisplayNameKey: "db_identifier",
			}},
		},
		{
			Name:          "S3 Buckets",
			ShortName:     "s3",
			Aliases:       []string{"s3", "buckets"},
			Category:      "DATABASES & STORAGE",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "name", Title: "Bucket Name", Width: 40, Sortable: true},
				{Key: "creation_date", Title: "Creation Date", Width: 22, Sortable: true},
			},
			Color: func(_ Resource) Color { return ColorHealthy },
			Children: []ChildViewDef{{
				ChildType:      "s3_objects",
				Key:            "enter",
				ContextKeys:    map[string]string{"bucket": "ID"},
				DisplayNameKey: "bucket",
			}},
		},
		{
			Name:          "ElastiCache Redis",
			ShortName:     "redis",
			Aliases:       []string{"redis", "elasticache"},
			Category:      "DATABASES & STORAGE",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "cluster_id", Title: "Cluster ID", Width: 28, Sortable: true},
				{Key: "engine_version", Title: "Version", Width: 10, Sortable: true},
				{Key: "node_type", Title: "Node Type", Width: 18, Sortable: true},
				{Key: "status", Title: "Status", Width: 14, Sortable: true},
				{Key: "nodes", Title: "Nodes", Width: 8, Sortable: true},
				{Key: "endpoint", Title: "Endpoint", Width: 40, Sortable: false},
			},
			Color: func(r Resource) Color {
				switch r.Fields["status"] {
				case "available":
					return ColorHealthy
				case "creating", "modifying", "snapshotting", "deleting":
					return ColorWarning
				case "incompatible-network":
					return ColorBroken
				}
				return ColorHealthy
			},
		},
		{
			Name:          "DB Clusters",
			ShortName:     "dbc",
			Aliases:       []string{"dbc", "docdb", "clusters", "db-clusters"},
			Category:      "DATABASES & STORAGE",
			CloudTrailKey: "ResourceName:Fields.arn",
			Columns: []Column{
				{Key: "cluster_id", Title: "Cluster ID", Width: 28, Sortable: true},
				{Key: "engine_version", Title: "Version", Width: 10, Sortable: true},
				{Key: "status", Title: "Status", Width: 14, Sortable: true},
				{Key: "instances", Title: "Instances", Width: 10, Sortable: true},
				{Key: "endpoint", Title: "Endpoint", Width: 48, Sortable: false},
			},
			Color: func(r Resource) Color {
				return rdsInstanceColor(r.Fields["status"])
			},
		},
		{
			Name:          "DynamoDB Tables",
			ShortName:     "ddb",
			Aliases:       []string{"ddb", "dynamodb", "dynamo"},
			Category:      "DATABASES & STORAGE",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "table_name", Title: "Table Name", Width: 36, Sortable: true},
				{Key: "status", Title: "Status", Width: 12, Sortable: true},
				{Key: "item_count", Title: "Items", Width: 12, Sortable: true},
				{Key: "size_bytes", Title: "Size", Width: 14, Sortable: true},
				{Key: "billing_mode", Title: "Billing", Width: 16, Sortable: true},
			},
			Color: func(r Resource) Color {
				// Prefer "status" (set by fetcher); fall back to legacy "table_status".
				status := r.Fields["status"]
				if status == "" {
					status = r.Fields["table_status"]
				}
				switch status {
				case "ACTIVE":
					return ColorHealthy
				case "CREATING", "UPDATING", "DELETING":
					return ColorWarning
				case "INACCESSIBLE_ENCRYPTION_CREDENTIALS", "ARCHIVED", "ARCHIVING":
					return ColorBroken
				}
				return ColorHealthy
			},
		},
		{
			Name:          "OpenSearch Domains",
			ShortName:     "opensearch",
			Aliases:       []string{"opensearch", "os", "elasticsearch"},
			Category:      "DATABASES & STORAGE",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "domain_name", Title: "Domain Name", Width: 28, Sortable: true},
				{Key: "engine_version", Title: "Engine Version", Width: 16, Sortable: true},
				{Key: "instance_type", Title: "Instance Type", Width: 22, Sortable: true},
				{Key: "instance_count", Title: "Instances", Width: 10, Sortable: true},
				{Key: "endpoint", Title: "Endpoint", Width: 48, Sortable: false},
			},
			Color: func(r Resource) Color {
				// OpenSearch DomainStatus: ClusterHealth (Red/Yellow/Green) is the
				// primary health signal; Processing/UpgradeProcessing flag transitions.
				switch r.Fields["cluster_health"] {
				case "Red":
					return ColorBroken
				case "Yellow":
					return ColorWarning
				}
				if r.Fields["deleted"] == "true" {
					return ColorBroken
				}
				if r.Fields["processing"] == "true" || r.Fields["upgrade_processing"] == "true" {
					return ColorWarning
				}
				switch r.Fields["status"] {
				case "failed", "FAILED", "error", "ERROR":
					return ColorBroken
				case "creating", "CREATING", "updating", "UPDATING", "deleting", "DELETING":
					return ColorWarning
				}
				return ColorHealthy
			},
		},
		{
			Name:          "Redshift Clusters",
			ShortName:     "redshift",
			Aliases:       []string{"redshift", "redshift-clusters"},
			Category:      "DATABASES & STORAGE",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "cluster_id", Title: "Cluster ID", Width: 28, Sortable: true},
				{Key: "status", Title: "Status", Width: 14, Sortable: true},
				{Key: "node_type", Title: "Node Type", Width: 16, Sortable: true},
				{Key: "num_nodes", Title: "Nodes", Width: 7, Sortable: true},
				{Key: "db_name", Title: "Database", Width: 16, Sortable: true},
				{Key: "endpoint", Title: "Endpoint", Width: 44, Sortable: false},
			},
			Color: func(r Resource) Color {
				switch r.Fields["status"] {
				case "available":
					return ColorHealthy
				case "creating", "modifying", "deleting", "resizing", "renaming", "rebooting":
					return ColorWarning
				case "incompatible-hsm", "incompatible-network", "incompatible-parameters",
					"incompatible-restore", "hardware-failure", "storage-full":
					return ColorBroken
				}
				return ColorHealthy
			},
		},
		{
			Name:          "EFS File Systems",
			ShortName:     "efs",
			Aliases:       []string{"efs", "file-systems"},
			Category:      "DATABASES & STORAGE",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "name", Title: "Name", Width: 28, Sortable: true},
				{Key: "file_system_id", Title: "File System ID", Width: 22, Sortable: true},
				{Key: "life_cycle_state", Title: "State", Width: 12, Sortable: true},
				{Key: "performance_mode", Title: "Perf Mode", Width: 16, Sortable: true},
				{Key: "encrypted", Title: "Encrypted", Width: 10, Sortable: true},
				{Key: "mount_targets", Title: "Mounts", Width: 8, Sortable: true},
			},
			Color: func(r Resource) Color {
				switch r.Fields["life_cycle_state"] {
				case "available":
					return ColorHealthy
				case "creating", "updating", "deleting":
					return ColorWarning
				case "error":
					return ColorBroken
				}
				return ColorHealthy
			},
		},
		{
			Name:          "RDS Snapshots",
			ShortName:     "rds-snap",
			Aliases:       []string{"rds-snap", "rds-snapshots", "db-snapshots"},
			Category:      "DATABASES & STORAGE",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "snapshot_id", Title: "Snapshot ID", Width: 36, Sortable: true},
				{Key: "db_instance", Title: "DB Instance", Width: 28, Sortable: true},
				{Key: "status", Title: "Status", Width: 12, Sortable: true},
				{Key: "engine", Title: "Engine", Width: 12, Sortable: true},
				{Key: "snapshot_type", Title: "Type", Width: 12, Sortable: true},
				{Key: "created", Title: "Created", Width: 22, Sortable: true},
			},
			Color: func(r Resource) Color {
				switch r.Fields["status"] {
				case "available":
					return ColorHealthy
				case "creating", "copying":
					return ColorWarning
				case "failed":
					return ColorBroken
				}
				return ColorHealthy
			},
		},
		{
			Name:          "DocDB Snapshots",
			ShortName:     "docdb-snap",
			Aliases:       []string{"docdb-snap", "docdb-snapshots", "cluster-snapshots"},
			Category:      "DATABASES & STORAGE",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "snapshot_id", Title: "Snapshot ID", Width: 36, Sortable: true},
				{Key: "cluster_id", Title: "Cluster ID", Width: 28, Sortable: true},
				{Key: "status", Title: "Status", Width: 12, Sortable: true},
				{Key: "engine", Title: "Engine", Width: 12, Sortable: true},
				{Key: "snapshot_type", Title: "Type", Width: 12, Sortable: true},
				{Key: "snapshot_create_time", Title: "Created", Width: 22, Sortable: true},
				{Key: "storage_type", Title: "Storage", Width: 10, Sortable: true},
			},
			Color: func(r Resource) Color {
				switch r.Fields["status"] {
				case "available":
					return ColorHealthy
				case "creating":
					return ColorWarning
				case "failed":
					return ColorBroken
				}
				return ColorHealthy
			},
		},
	}
}
