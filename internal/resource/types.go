package resource

import "strings"

// Column defines a column in a resource table view.
type Column struct {
	// Key is the field key used to extract the value from Resource.Fields.
	Key string
	// Title is the column header display text.
	Title string
	// Width is the fixed column width; 0 means flexible.
	Width int
	// Sortable indicates whether this column supports sorting.
	Sortable bool
}

// ResourceTypeDef defines a category of AWS resources the app can browse.
type ResourceTypeDef struct {
	// Name is the display name (e.g., "EC2 Instances").
	Name string
	// ShortName is the colon-command alias (e.g., "ec2").
	ShortName string
	// Aliases are alternative command names for this resource type.
	Aliases []string
	// Columns are the table columns for list view.
	Columns []Column
}

var resourceTypes = []ResourceTypeDef{
	{
		Name:      "S3 Buckets",
		ShortName: "s3",
		Aliases:   []string{"s3", "buckets"},
		Columns: []Column{
			{Key: "name", Title: "Bucket Name", Width: 40, Sortable: true},
			{Key: "creation_date", Title: "Creation Date", Width: 22, Sortable: true},
		},
	},
	{
		Name:      "EC2 Instances",
		ShortName: "ec2",
		Aliases:   []string{"ec2", "instances"},
		Columns: []Column{
			{Key: "instance_id", Title: "Instance ID", Width: 20, Sortable: true},
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "type", Title: "Type", Width: 14, Sortable: true},
			{Key: "private_ip", Title: "Private IP", Width: 16, Sortable: false},
			{Key: "public_ip", Title: "Public IP", Width: 16, Sortable: false},
			{Key: "launch_time", Title: "Launch Time", Width: 22, Sortable: true},
		},
	},
	{
		Name:      "RDS Instances",
		ShortName: "rds",
		Aliases:   []string{"rds", "databases"},
		Columns: []Column{
			{Key: "db_identifier", Title: "DB Identifier", Width: 28, Sortable: true},
			{Key: "engine", Title: "Engine", Width: 12, Sortable: true},
			{Key: "engine_version", Title: "Version", Width: 10, Sortable: true},
			{Key: "status", Title: "Status", Width: 14, Sortable: true},
			{Key: "class", Title: "Class", Width: 16, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 40, Sortable: false},
			{Key: "multi_az", Title: "Multi-AZ", Width: 10, Sortable: true},
		},
	},
	{
		Name:      "ElastiCache Redis",
		ShortName: "redis",
		Aliases:   []string{"redis", "elasticache"},
		Columns: []Column{
			{Key: "cluster_id", Title: "Cluster ID", Width: 28, Sortable: true},
			{Key: "engine_version", Title: "Version", Width: 10, Sortable: true},
			{Key: "node_type", Title: "Node Type", Width: 18, Sortable: true},
			{Key: "status", Title: "Status", Width: 14, Sortable: true},
			{Key: "nodes", Title: "Nodes", Width: 8, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 40, Sortable: false},
		},
	},
	{
		Name:      "DocumentDB Clusters",
		ShortName: "docdb",
		Aliases:   []string{"docdb", "documentdb"},
		Columns: []Column{
			{Key: "cluster_id", Title: "Cluster ID", Width: 28, Sortable: true},
			{Key: "engine_version", Title: "Version", Width: 10, Sortable: true},
			{Key: "status", Title: "Status", Width: 14, Sortable: true},
			{Key: "instances", Title: "Instances", Width: 10, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 48, Sortable: false},
		},
	},
	{
		Name:      "EKS Clusters",
		ShortName: "eks",
		Aliases:   []string{"eks", "kubernetes", "k8s"},
		Columns: []Column{
			{Key: "cluster_name", Title: "Cluster Name", Width: 28, Sortable: true},
			{Key: "version", Title: "Version", Width: 10, Sortable: true},
			{Key: "status", Title: "Status", Width: 14, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 48, Sortable: false},
			{Key: "platform_version", Title: "Platform Version", Width: 18, Sortable: true},
		},
	},
	{
		Name:      "Secrets Manager",
		ShortName: "secrets",
		Aliases:   []string{"secrets", "secretsmanager", "sm"},
		Columns: []Column{
			{Key: "secret_name", Title: "Secret Name", Width: 36, Sortable: true},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
			{Key: "last_accessed", Title: "Last Accessed", Width: 18, Sortable: true},
			{Key: "last_changed", Title: "Last Changed", Width: 18, Sortable: true},
			{Key: "rotation_enabled", Title: "Rotation", Width: 10, Sortable: true},
		},
	},
}

// AllResourceTypes returns the definitions for all supported resource types.
func AllResourceTypes() []ResourceTypeDef {
	result := make([]ResourceTypeDef, len(resourceTypes))
	copy(result, resourceTypes)
	return result
}

// S3ObjectColumns returns the column definitions used when browsing inside
// an S3 bucket (objects/prefixes), as opposed to the bucket list columns.
func S3ObjectColumns() []Column {
	return []Column{
		{Key: "key", Title: "Key", Width: 50, Sortable: true},
		{Key: "size", Title: "Size", Width: 12, Sortable: true},
		{Key: "last_modified", Title: "Last Modified", Width: 22, Sortable: true},
		{Key: "storage_class", Title: "Storage Class", Width: 16, Sortable: true},
	}
}

// FindResourceType looks up a resource type by its ShortName or any of its Aliases.
// Returns nil if no match is found.
func FindResourceType(name string) *ResourceTypeDef {
	lower := strings.ToLower(name)
	for i := range resourceTypes {
		if strings.ToLower(resourceTypes[i].ShortName) == lower {
			return &resourceTypes[i]
		}
		for _, alias := range resourceTypes[i].Aliases {
			if strings.ToLower(alias) == lower {
				return &resourceTypes[i]
			}
		}
	}
	return nil
}
