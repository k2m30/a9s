package resource

import (
	"strings"
	"time"
)

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
				status := r.Fields["status"]
				// Strip the universal-rule-7 (+N) suffix before matching:
				//   "publicly accessible (+1)" → "publicly accessible"
				//   "no automated backups (+2)" → "no automated backups"
				// The suffix only records hidden-finding count for the operator;
				// color precedence is driven by the TOP (shown) phrase.
				stripped := StripFindingSuffix(status)
				// Broken statuses (including remapped "encryption key unavailable").
				switch stripped {
				case "failed", "storage-full", "restore-error", "stopped",
					"incompatible-network", "incompatible-option-group",
					"incompatible-parameters", "incompatible-restore",
					"encryption key unavailable":
					return ColorBroken
				}
				if strings.HasPrefix(stripped, "incompatible-") || strings.HasPrefix(stripped, "inaccessible-") {
					return ColorBroken
				}
				// Config-warning phrases encoded by the new fetcher → Warning.
				// NOTE: "maintenance scheduled" is NOT here — it is the Wave-2 `~`
				// text rendered on a Healthy (green) row per spec §4 rule 3.
				switch stripped {
				case "no automated backups", "publicly accessible",
					"unencrypted storage", "deletion protection off":
					return ColorWarning
				}
				// Transitional statuses → Warning (including "keyword: pending-key" suffix form).
				if stripped != "" && stripped != "available" && stripped != "maintenance scheduled" {
					if strings.Contains(stripped, ":") {
						return ColorWarning
					}
					switch stripped {
					case "creating", "modifying", "backing-up", "rebooting",
						"renaming", "resetting-master-credentials", "starting",
						"stopping", "upgrading", "maintenance",
						"configuring-enhanced-monitoring", "configuring-iam-database-auth",
						"configuring-log-exports", "converting-to-vpc", "moving-to-vpc",
						"storage-optimization", "deleting":
						return ColorWarning
					}
				}
				// status == "" (healthy silence from new fetcher), "available"
				// (legacy / backward-compat), or "maintenance scheduled" (Wave-2
				// on Healthy row). All are base-healthy, but individual
				// field-level checks may upgrade to Warning.
				base := ColorHealthy
				// CIS RDS.2: publicly accessible → Warning.
				if r.Fields["publicly_accessible"] == "true" {
					if base < ColorWarning {
						base = ColorWarning
					}
				}
				// CIS RDS.3: unencrypted storage → Warning.
				if r.Fields["storage_encrypted"] == "false" {
					if base < ColorWarning {
						base = ColorWarning
					}
				}
				// No deletion protection → Warning.
				if r.Fields["deletion_protection"] == "false" {
					if base < ColorWarning {
						base = ColorWarning
					}
				}
				// No automated backups (BackupRetentionPeriod == 0) → Warning.
				if r.Fields["backup_retention_period"] == "0" {
					if base < ColorWarning {
						base = ColorWarning
					}
				}
				return base
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
				{Key: "node_type", Title: "Node Type", Width: 18, Sortable: true},
				{Key: "status", Title: "Status", Width: 32, Sortable: true},
				{Key: "nodes", Title: "Nodes", Width: 8, Sortable: true},
				{Key: "endpoint", Title: "Endpoint", Width: 40, Sortable: false},
			},
			Color: func(r Resource) Color {
				// Strip the universal-rule-7 (+N) suffix so both raw AWS keywords
				// (stored by DescribeReplicationGroups) and §4 phrases (constructed
				// by computeRedisIssues) classify correctly.
				// Examples:
				//   "creating — new group" (§4)        → ColorWarning
				//   "create failed — see events" (§4)  → ColorBroken
				//   "shard 0001: modifying" (§4)       → ColorWarning
				//   "deleted" (terminal)                → ColorDim
				//   "" (healthy silence)                → ColorHealthy
				phrase := StripFindingSuffix(r.Fields["status"])
				// Terminal state — dim, not broken.
				if phrase == "deleted" {
					return ColorDim
				}
				// Broken: §4 phrases.
				switch phrase {
				case "create failed \u2014 see events":
					return ColorBroken
				}
				// Warning: §4 phrases.
				switch phrase {
				case "creating \u2014 new group",
					"modifying \u2014 config change",
					"snapshotting \u2014 backup running",
					"deleting \u2014 teardown",
					"multi-AZ without auto-failover":
					return ColorWarning
				}
				// Multi-shard shard-level phrases: "shard <id>: <state>" — all Warning.
				if strings.HasPrefix(phrase, "shard ") {
					return ColorWarning
				}
				// "" (healthy silence) → Healthy.
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
				{Key: "status", Title: "Status", Width: 32, Sortable: true},
				{Key: "instances", Title: "Instances", Width: 10, Sortable: true},
				{Key: "endpoint", Title: "Endpoint", Width: 48, Sortable: false},
			},
			Color: func(r Resource) Color {
				// Strip (+N) suffix so phrase matching works regardless of stacking.
				phrase := StripFindingSuffix(r.Fields["status"])
				switch phrase {
				case "":
					return ColorHealthy
				// Broken phrases (§4 "List text" column).
				case "failed: cluster operation",
					"encryption key unreachable",
					"parameter group incompatible",
					"no writer: reads only":
					return ColorBroken
				// Warning phrases (§4 "List text" column).
				case "delete-protection off",
					"not encrypted at rest",
					"no automated backups":
					return ColorWarning
				// Wave 2 phrase on a Healthy row — stays green so the `!` glyph renders.
				case "maintenance overdue":
					return ColorHealthy
				}
				// Transitional — format is "<status>: in progress".
				if strings.HasSuffix(phrase, ": in progress") {
					return ColorWarning
				}
				// Unknown phrase → treat as Healthy (future-proof for new AWS statuses).
				return ColorHealthy
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
				// Strip the universal-rule-7 (+N) suffix before matching so that
				// "archived: kms key lost (+1)" still maps to ColorBroken.
				phrase := StripFindingSuffix(r.Fields["status"])
				switch phrase {
				case "":
					return ColorHealthy
				case "creating", "updating", "deleting", "archiving":
					return ColorWarning
				case "kms key inaccessible", "archived: kms key lost":
					return ColorBroken
				case "PITR off":
					// Wave-2 ~ finding on a Healthy row — the `~` glyph does the
					// signaling; the row color stays green so the glyph renders.
					return ColorHealthy
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
			// OpenSearch DomainStatus per docs/attention-signals.md.
			// Precedence: Deleted → Dim; Isolated → Broken; Processing → Warning;
			// background-check findings (! / ~) stay green — the glyph handles those.
			// Field contract:
			//   - deleted: "true" when Deleted==true
			//   - domain_processing_status: string form of DomainProcessingStatusType
			//     (fetcher always emits at least "Active" so the Isolated branch is deterministic)
			//   - processing / upgrade_processing: "true"/"false" from DomainStatus
			//   - status: top §4 phrase with optional (+N) suffix; stripped before matching
			//   - cluster_health: Red/Yellow/Green from CloudWatch (Wave 3, not yet
			//     implemented — branch kept for forward-compatibility, currently never fires)
			Color: func(r Resource) Color {
				// Deleted → Dim (highest precedence, no further checks needed).
				if r.Fields["deleted"] == "true" {
					return ColorDim
				}

				// Strip (+N) suffix before pattern matching.
				status := r.Status
				if status == "" {
					status = r.Fields["status"]
				}
				stripped := StripFindingSuffix(status)

				// Isolated → Broken.
				if strings.HasPrefix(stripped, "isolated:") || r.Fields["domain_processing_status"] == "Isolated" {
					return ColorBroken
				}

				// Processing → Warning.
				if strings.HasPrefix(stripped, "processing:") ||
					r.Fields["processing"] == "true" ||
					r.Fields["upgrade_processing"] == "true" {
					return ColorWarning
				}

				// Background-check signals (! / ~) stay green — the glyph signals
				// the operator; the row color remains Healthy so the glyph renders.
				// NOTE: do NOT add service_software_update_available or
				// encryption_at_rest_enabled → Warning branches here.

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
				// Base color from ClusterStatus.
				var base Color
				switch r.Fields["status"] {
				case "available":
					base = ColorHealthy
				case "creating", "modifying", "resizing", "rebooting", "renaming", "deleting":
					base = ColorWarning
				case "incompatible-hsm", "incompatible-network", "incompatible-parameters",
					"incompatible-restore", "hardware-failure", "storage-full":
					base = ColorBroken
				default:
					base = ColorHealthy
				}
				// Do not downgrade Broken.
				if base == ColorBroken {
					return ColorBroken
				}
				// ClusterAvailabilityStatus upgrades.
				switch r.Fields["cluster_availability_status"] {
				case "Unavailable", "Failed":
					return ColorBroken
				case "Maintenance", "Modifying":
					if base == ColorHealthy {
						base = ColorWarning
					}
				}
				// Re-check after availability upgrade.
				if base == ColorBroken {
					return ColorBroken
				}
				// Publicly accessible → upgrade to Warning.
				if r.Fields["publicly_accessible"] == "true" && base == ColorHealthy {
					base = ColorWarning
				}
				// Unencrypted → upgrade to Warning.
				if r.Fields["encrypted"] == "false" && base == ColorHealthy {
					base = ColorWarning
				}
				return base
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
				base := ColorHealthy
				switch r.Fields["life_cycle_state"] {
				case "available":
					base = ColorHealthy
				case "creating", "updating", "deleting":
					base = ColorWarning
				case "error":
					base = ColorBroken
				}
				// Unreachable FS: no mount targets → upgrade to Broken.
				if r.Fields["mount_targets"] == "0" {
					base = ColorBroken
				}
				return base
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
				status := r.Fields["status"]
				c := ColorHealthy
				switch {
				case status == "failed":
					c = ColorBroken
				case strings.HasPrefix(status, "incompatible-"):
					c = ColorBroken
				case status == "creating" || status == "copying":
					c = ColorWarning
				}
				// CIS RDS.4: unencrypted snapshot → warning (Broken takes precedence)
				if c != ColorBroken && r.Fields["encrypted"] == "false" {
					c = ColorWarning
				}
				return c
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
				status := r.Fields["status"]
				c := ColorHealthy
				switch status {
				case "failed":
					c = ColorBroken
				case "creating":
					c = ColorWarning
				}
				// Unencrypted snapshot → warning
				if c != ColorBroken && r.Fields["storage_encrypted"] == "false" {
					c = ColorWarning
				}
				// Long-lived manual snapshot (>365 days) → warning (cost signal)
				if c != ColorBroken && r.Fields["snapshot_type"] == "manual" {
					if ts, err := time.Parse("2006-01-02 15:04", r.Fields["snapshot_create_time"]); err == nil {
						if time.Since(ts) > 365*24*time.Hour {
							c = ColorWarning
						}
					}
				}
				return c
			},
		},
	}
}
