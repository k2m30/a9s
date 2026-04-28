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
				if c, ok := ColorFromWave1(r); ok {
					return c
				}
				// Phrase-based structural fallback (for fixture/test data where
				// Findings are not populated). Fields["status"] carries the §4
				// phrase written by the fetcher.
				phrase := StripFindingSuffix(r.Fields["status"])
				switch phrase {
				case "failed", "storage-full", "encryption key unavailable", "restore-error":
					return ColorBroken
				}
				if strings.HasPrefix(phrase, "incompatible-") {
					return ColorBroken
				}
				switch phrase {
				case "creating", "modifying", "backing-up", "rebooting",
					"upgrading", "stopping", "stopped", "starting", "deleting", "renaming":
					return ColorWarning
				}
				// Field-level checks for structurally healthy instances.
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
				if c, ok := ColorFromWave1(r); ok {
					return c
				}
				// Phrase-based structural fallback (for fixture/test data where
				// Findings are not populated). Fields["status"] carries the §4
				// phrase written by the fetcher.
				phrase := StripFindingSuffix(r.Fields["status"])
				if phrase == "deleted" {
					return ColorDim
				}
				if phrase == "create failed — see events" {
					return ColorBroken
				}
				// Shard-level phrases (cluster-mode multi-shard RGs).
				if strings.HasPrefix(phrase, "shard ") {
					return ColorWarning
				}
				// §4 warning phrases — em-dash variants set by the fetcher.
				if strings.HasPrefix(phrase, "creating —") ||
					strings.HasPrefix(phrase, "modifying —") ||
					strings.HasPrefix(phrase, "snapshotting —") ||
					strings.HasPrefix(phrase, "deleting —") ||
					phrase == "multi-AZ without auto-failover" {
					return ColorWarning
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
				{Key: "status", Title: "Status", Width: 32, Sortable: true},
				{Key: "instances", Title: "Instances", Width: 10, Sortable: true},
				{Key: "endpoint", Title: "Endpoint", Width: 48, Sortable: false},
			},
			Color: func(r Resource) Color {
				if c, ok := ColorFromWave1(r); ok {
					return c
				}
				// Phrase-based structural fallback (for fixture/test data where
				// Findings are not populated). Fields["status"] carries the §4
				// phrase written by the fetcher; strip any (+N) suffix first.
				phrase := StripFindingSuffix(r.Fields["status"])
				switch phrase {
				case "":
					return ColorHealthy
				// Wave 2 phrase on a Healthy row — stays green so the `!` glyph renders.
				case "maintenance overdue":
					return ColorHealthy
				// §4 Broken phrases.
				case "failed: cluster operation", "encryption key unreachable",
					"parameter group incompatible", "no writer: reads only":
					return ColorBroken
				// §4 Warning phrases — structural config signals.
				case "delete-protection off", "not encrypted at rest", "no automated backups":
					return ColorWarning
				}
				// Transitional — format is "<status>: in progress".
				if strings.HasSuffix(phrase, ": in progress") {
					return ColorWarning
				}
				// Unknown phrase → treat as Healthy (future-proof).
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
				if c, ok := ColorFromWave1(r); ok {
					return c
				}
				// Phrase-based structural fallback (for fixture/test data where
				// Findings are not populated). Fields["status"] carries the §4
				// phrase written by the fetcher.
				phrase := StripFindingSuffix(r.Fields["status"])
				switch phrase {
				case "kms key inaccessible", "archived: kms key lost":
					return ColorBroken
				case "creating", "updating", "deleting", "archiving":
					return ColorWarning
				case "":
					return ColorHealthy
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
			Color: func(r Resource) Color {
				if c, ok := ColorFromWave1(r); ok {
					return c
				}
				// Structural fallback: read raw fields for legacy/fixture data.
				if r.Fields["deleted"] == "true" {
					return ColorDim
				}
				if r.Fields["domain_processing_status"] == "Isolated" {
					return ColorBroken
				}
				if r.Fields["processing"] == "true" || r.Fields["upgrade_processing"] == "true" {
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
				// Widths match defaults_databases.go so fallback paths (tests,
				// environments without a loaded view config) don't truncate the
				// longest phrase ("broken: incompatible-parameters" = 31 chars).
				{Key: "cluster_id", Title: "Cluster ID", Width: 36, Sortable: true},
				{Key: "status", Title: "Status", Width: 34, Sortable: true},
				{Key: "node_type", Title: "Node Type", Width: 16, Sortable: true},
				{Key: "num_nodes", Title: "Nodes", Width: 7, Sortable: true},
				{Key: "db_name", Title: "Database", Width: 16, Sortable: true},
				{Key: "endpoint", Title: "Endpoint", Width: 44, Sortable: false},
			},
			Color: func(r Resource) Color {
				if c, ok := ColorFromWave1(r); ok {
					return c
				}
				// Phrase-based structural fallback (for fixture/test data where
				// Findings are not populated). Fields["status"] carries the §4
				// phrase written by the fetcher; strip any (+N) suffix first.
				phrase := StripFindingSuffix(r.Fields["status"])
				if phrase != "" {
					switch phrase {
					case "unavailable", "failed":
						return ColorBroken
					case "pending change queued", "maintenance deferred",
						"publicly accessible", "unencrypted at rest":
						return ColorWarning
					}
					if strings.HasPrefix(phrase, "broken: ") {
						return ColorBroken
					}
				}
				// Structural fallback for raw ClusterStatus / ClusterAvailabilityStatus
				// values (e.g. from raw fixtures or legacy cache entries).
				var base Color
				switch r.Fields["cluster_status"] {
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
				if base == ColorBroken {
					return ColorBroken
				}
				switch r.Fields["cluster_availability_status"] {
				case "Unavailable", "Failed":
					return ColorBroken
				case "Maintenance", "Modifying":
					if base == ColorHealthy {
						base = ColorWarning
					}
				}
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
				{Key: "status", Title: "Status", Width: 24, Sortable: true},
				{Key: "performance_mode", Title: "Perf Mode", Width: 16, Sortable: true},
				{Key: "encrypted", Title: "Encrypted", Width: 10, Sortable: true},
				{Key: "mount_targets", Title: "Mounts", Width: 8, Sortable: true},
			},
			Color: func(r Resource) Color {
				if c, ok := ColorFromWave1(r); ok {
					return c
				}
				// Structural fallback — checks both the canonical phrase key
				// ("status") and the raw life-cycle-state key ("life_cycle_state")
				// so that test data injected via either field is classified correctly.
				// For the canonical path, strip any (+N) suffix before matching.
				phrase := StripFindingSuffix(r.Fields["status"])
				if phrase == "" {
					// Fallback to raw enum for resources whose Findings are not
					// populated (e.g. test probes via the typeContracts table).
					phrase = r.Fields["life_cycle_state"]
				}
				switch phrase {
				case "error", "no mount targets", "mount target down":
					return ColorBroken
				case "creating", "updating", "deleting":
					return ColorWarning
				case "available", "":
					return ColorHealthy
				default:
					return ColorHealthy
				}
			},
		},
		{
			Name:          "DB Instance Snapshots",
			ShortName:     "dbi-snap",
			Aliases:       []string{"dbi-snap", "rds-snapshots", "db-snapshots"},
			Category:      "DATABASES & STORAGE",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "snapshot_id", Title: "Snapshot ID", Width: 36, Sortable: true},
				{Key: "db_instance", Title: "DB Instance", Width: 28, Sortable: true},
				{Key: "status", Title: "Status", Width: 32, Sortable: true},
				{Key: "engine", Title: "Engine", Width: 12, Sortable: true},
				{Key: "snapshot_type", Title: "Type", Width: 12, Sortable: true},
				{Key: "created", Title: "Created", Width: 22, Sortable: true},
			},
			Color: func(r Resource) Color {
				if c, ok := ColorFromWave1(r); ok {
					return c
				}
				// Phrase-based structural fallback (for fixture/test data where
				// Findings are not populated). Fields["status"] carries the §4
				// phrase written by the fetcher.
				phrase := StripFindingSuffix(r.Fields["status"])
				if phrase == "failed" {
					return ColorBroken
				}
				if strings.HasPrefix(phrase, "incompatible-") {
					return ColorBroken
				}
				if phrase == "creating" || phrase == "copying" {
					return ColorWarning
				}
				// Structural fallback: unencrypted snapshot (CIS RDS.4).
				if r.Fields["encrypted"] == "false" {
					return ColorWarning
				}
				return ColorHealthy
			},
		},
		{
			Name:          "DB Cluster Snapshots",
			ShortName:     "dbc-snap",
			Aliases:       []string{"dbc-snap", "docdb-snapshots", "cluster-snapshots"},
			Category:      "DATABASES & STORAGE",
			CloudTrailKey: "ResourceName:ID",
			Columns: []Column{
				{Key: "snapshot_id", Title: "Snapshot ID", Width: 36, Sortable: true},
				{Key: "cluster_id", Title: "Cluster ID", Width: 28, Sortable: true},
				// Status column reads from Fields["status"] (set by the
				// fetcher to the phrase, then overwritten by the cross-ref
				// enricher's FieldUpdates). Width 32 matches dbi-snap so
				// "automated, Nd past retention" fits without truncation.
				{Key: "status", Title: "Status", Width: 32, Sortable: true},
				{Key: "engine", Title: "Engine", Width: 12, Sortable: true},
				{Key: "snapshot_type", Title: "Type", Width: 12, Sortable: true},
				{Key: "snapshot_create_time", Title: "Created", Width: 22, Sortable: true},
				{Key: "storage_type", Title: "Storage", Width: 10, Sortable: true},
			},
			Color: func(r Resource) Color {
				if c, ok := ColorFromWave1(r); ok {
					return c
				}
				// Phrase-based structural fallback (for fixture/test data where
				// Findings are not populated). Fields["status"] carries the §4
				// phrase written by the fetcher.
				phrase := StripFindingSuffix(r.Fields["status"])
				if phrase == "failed" {
					return ColorBroken
				}
				if strings.HasPrefix(phrase, "incompatible-") {
					return ColorBroken
				}
				// "creating" and progress variants (e.g. "creating: 47%") → Warning.
				if strings.HasPrefix(phrase, "creating") {
					return ColorWarning
				}
				// Structural fallback: unencrypted snapshot → warning.
				if r.Fields["storage_encrypted"] == "false" {
					return ColorWarning
				}
				// Long-lived manual snapshot (>365 days) → warning (cost signal).
				if r.Fields["snapshot_type"] == "manual" {
					if ts, err := time.Parse("2006-01-02 15:04", r.Fields["snapshot_create_time"]); err == nil {
						if time.Since(ts) > 365*24*time.Hour {
							return ColorWarning
						}
					}
				}
				return ColorHealthy
			},
		},
	}
}
