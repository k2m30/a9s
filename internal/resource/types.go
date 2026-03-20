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
		Name:      "DB Instances",
		ShortName: "dbi",
		Aliases:   []string{"dbi", "rds", "databases", "db-instances"},
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
		Name:      "DB Clusters",
		ShortName: "dbc",
		Aliases:   []string{"dbc", "docdb", "clusters", "db-clusters"},
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
	{
		Name:      "VPCs",
		ShortName: "vpc",
		Aliases:   []string{"vpc", "vpcs"},
		Columns: []Column{
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			{Key: "name", Title: "Name", Width: 24, Sortable: true},
			{Key: "cidr_block", Title: "CIDR Block", Width: 18, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "is_default", Title: "Default", Width: 9, Sortable: true},
		},
	},
	{
		Name:      "Security Groups",
		ShortName: "sg",
		Aliases:   []string{"sg", "securitygroups", "security-groups"},
		Columns: []Column{
			{Key: "group_id", Title: "Group ID", Width: 24, Sortable: true},
			{Key: "group_name", Title: "Group Name", Width: 28, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			{Key: "description", Title: "Description", Width: 36, Sortable: false},
		},
	},
	{
		Name:      "EKS Node Groups",
		ShortName: "ng",
		Aliases:   []string{"ng", "nodegroups", "node-groups"},
		Columns: []Column{
			{Key: "nodegroup_name", Title: "Node Group", Width: 28, Sortable: true},
			{Key: "cluster_name", Title: "Cluster", Width: 24, Sortable: true},
			{Key: "status", Title: "Status", Width: 14, Sortable: true},
			{Key: "instance_types", Title: "Instance Types", Width: 20, Sortable: false},
			{Key: "desired_size", Title: "Desired", Width: 9, Sortable: true},
		},
	},
	{
		Name:      "Subnets",
		ShortName: "subnet",
		Aliases:   []string{"subnet", "subnets"},
		Columns: []Column{
			{Key: "subnet_id", Title: "Subnet ID", Width: 26, Sortable: true},
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			{Key: "cidr_block", Title: "CIDR Block", Width: 18, Sortable: true},
			{Key: "availability_zone", Title: "AZ", Width: 14, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "available_ips", Title: "Available IPs", Width: 14, Sortable: true},
		},
	},
	{
		Name:      "Route Tables",
		ShortName: "rtb",
		Aliases:   []string{"rtb", "routetables", "route-tables"},
		Columns: []Column{
			{Key: "route_table_id", Title: "Route Table ID", Width: 26, Sortable: true},
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			{Key: "routes_count", Title: "Routes", Width: 8, Sortable: true},
			{Key: "associations_count", Title: "Assoc.", Width: 8, Sortable: true},
		},
	},
	{
		Name:      "NAT Gateways",
		ShortName: "nat",
		Aliases:   []string{"nat", "natgateways", "nat-gateways"},
		Columns: []Column{
			{Key: "nat_gateway_id", Title: "NAT Gateway ID", Width: 26, Sortable: true},
			{Key: "name", Title: "Name", Width: 24, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			{Key: "subnet_id", Title: "Subnet ID", Width: 26, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "public_ip", Title: "Public IP", Width: 16, Sortable: false},
		},
	},
	{
		Name:      "Internet Gateways",
		ShortName: "igw",
		Aliases:   []string{"igw", "internetgateways", "internet-gateways"},
		Columns: []Column{
			{Key: "igw_id", Title: "IGW ID", Width: 26, Sortable: true},
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
		},
	},
	{
		Name:      "Lambda Functions",
		ShortName: "lambda",
		Aliases:   []string{"lambda", "functions"},
		Columns: []Column{
			{Key: "function_name", Title: "Function Name", Width: 36, Sortable: true},
			{Key: "runtime", Title: "Runtime", Width: 16, Sortable: true},
			{Key: "memory", Title: "Memory", Width: 8, Sortable: true},
			{Key: "timeout", Title: "Timeout", Width: 8, Sortable: true},
			{Key: "handler", Title: "Handler", Width: 30, Sortable: false},
			{Key: "last_modified", Title: "Last Modified", Width: 22, Sortable: true},
		},
	},
	{
		Name:      "CloudWatch Alarms",
		ShortName: "alarm",
		Aliases:   []string{"alarm", "alarms", "cloudwatch"},
		Columns: []Column{
			{Key: "alarm_name", Title: "Alarm Name", Width: 36, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "metric_name", Title: "Metric", Width: 24, Sortable: true},
			{Key: "namespace", Title: "Namespace", Width: 24, Sortable: true},
			{Key: "threshold", Title: "Threshold", Width: 12, Sortable: true},
		},
	},
	{
		Name:      "SNS Topics",
		ShortName: "sns",
		Aliases:   []string{"sns", "topics"},
		Columns: []Column{
			{Key: "display_name", Title: "Topic Name", Width: 40, Sortable: true},
			{Key: "topic_arn", Title: "Topic ARN", Width: 60, Sortable: true},
		},
	},
	{
		Name:      "SQS Queues",
		ShortName: "sqs",
		Aliases:   []string{"sqs", "queues"},
		Columns: []Column{
			{Key: "queue_name", Title: "Queue Name", Width: 36, Sortable: true},
			{Key: "approx_messages", Title: "Messages", Width: 10, Sortable: true},
			{Key: "approx_not_visible", Title: "In Flight", Width: 10, Sortable: true},
			{Key: "delay_seconds", Title: "Delay", Width: 8, Sortable: true},
			{Key: "queue_url", Title: "Queue URL", Width: 50, Sortable: false},
		},
	},
	{
		Name:      "Load Balancers",
		ShortName: "elb",
		Aliases:   []string{"elb", "alb", "nlb", "loadbalancers", "load-balancers"},
		Columns: []Column{
			{Key: "name", Title: "Name", Width: 32, Sortable: true},
			{Key: "dns_name", Title: "DNS Name", Width: 48, Sortable: false},
			{Key: "type", Title: "Type", Width: 12, Sortable: true},
			{Key: "scheme", Title: "Scheme", Width: 14, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
		},
	},
	{
		Name:      "Target Groups",
		ShortName: "tg",
		Aliases:   []string{"tg", "targetgroups", "target-groups"},
		Columns: []Column{
			{Key: "target_group_name", Title: "Target Group", Width: 32, Sortable: true},
			{Key: "port", Title: "Port", Width: 8, Sortable: true},
			{Key: "protocol", Title: "Protocol", Width: 10, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			{Key: "target_type", Title: "Target Type", Width: 12, Sortable: true},
			{Key: "health_check_path", Title: "Health Check", Width: 24, Sortable: false},
		},
	},
	{
		Name:      "ECS Clusters",
		ShortName: "ecs",
		Aliases:   []string{"ecs", "ecs-clusters"},
		Columns: []Column{
			{Key: "cluster_name", Title: "Cluster Name", Width: 32, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "running_tasks", Title: "Running", Width: 9, Sortable: true},
			{Key: "pending_tasks", Title: "Pending", Width: 9, Sortable: true},
			{Key: "services_count", Title: "Services", Width: 10, Sortable: true},
		},
	},
	{
		Name:      "ECS Services",
		ShortName: "ecs-svc",
		Aliases:   []string{"ecs-svc", "ecs-services"},
		Columns: []Column{
			{Key: "service_name", Title: "Service Name", Width: 32, Sortable: true},
			{Key: "cluster", Title: "Cluster", Width: 24, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "desired_count", Title: "Desired", Width: 9, Sortable: true},
			{Key: "running_count", Title: "Running", Width: 9, Sortable: true},
			{Key: "launch_type", Title: "Launch Type", Width: 12, Sortable: true},
		},
	},
	{
		Name:      "CloudFormation Stacks",
		ShortName: "cfn",
		Aliases:   []string{"cfn", "cloudformation", "stacks"},
		Columns: []Column{
			{Key: "stack_name", Title: "Stack Name", Width: 36, Sortable: true},
			{Key: "status", Title: "Status", Width: 24, Sortable: true},
			{Key: "creation_time", Title: "Created", Width: 22, Sortable: true},
			{Key: "last_updated", Title: "Updated", Width: 22, Sortable: true},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
		},
	},
	{
		Name:      "IAM Roles",
		ShortName: "role",
		Aliases:   []string{"role", "roles", "iam-roles"},
		Columns: []Column{
			{Key: "role_name", Title: "Role Name", Width: 36, Sortable: true},
			{Key: "role_id", Title: "Role ID", Width: 22, Sortable: true},
			{Key: "path", Title: "Path", Width: 20, Sortable: true},
			{Key: "create_date", Title: "Created", Width: 22, Sortable: true},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
		},
	},
	{
		Name:      "CloudWatch Log Groups",
		ShortName: "logs",
		Aliases:   []string{"logs", "loggroups", "log-groups", "cwlogs"},
		Columns: []Column{
			{Key: "log_group_name", Title: "Log Group Name", Width: 48, Sortable: true},
			{Key: "stored_bytes", Title: "Size (bytes)", Width: 14, Sortable: true},
			{Key: "retention_days", Title: "Retention", Width: 10, Sortable: true},
			{Key: "creation_time", Title: "Created", Width: 16, Sortable: true},
		},
	},
	{
		Name:      "SSM Parameters",
		ShortName: "ssm",
		Aliases:   []string{"ssm", "parameters", "parameter-store"},
		Columns: []Column{
			{Key: "name", Title: "Name", Width: 40, Sortable: true},
			{Key: "type", Title: "Type", Width: 14, Sortable: true},
			{Key: "version", Title: "Version", Width: 8, Sortable: true},
			{Key: "last_modified", Title: "Last Modified", Width: 22, Sortable: true},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
		},
	},
	{
		Name:      "DynamoDB Tables",
		ShortName: "ddb",
		Aliases:   []string{"ddb", "dynamodb", "dynamo"},
		Columns: []Column{
			{Key: "table_name", Title: "Table Name", Width: 36, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "item_count", Title: "Items", Width: 12, Sortable: true},
			{Key: "size_bytes", Title: "Size (bytes)", Width: 14, Sortable: true},
			{Key: "billing_mode", Title: "Billing", Width: 16, Sortable: true},
		},
	},
	{
		Name:      "Elastic IPs",
		ShortName: "eip",
		Aliases:   []string{"eip", "elastic-ips", "elasticips"},
		Columns: []Column{
			{Key: "allocation_id", Title: "Allocation ID", Width: 26, Sortable: true},
			{Key: "name", Title: "Name", Width: 24, Sortable: true},
			{Key: "public_ip", Title: "Public IP", Width: 16, Sortable: true},
			{Key: "association_id", Title: "Association", Width: 26, Sortable: true},
			{Key: "instance_id", Title: "Instance", Width: 20, Sortable: true},
			{Key: "domain", Title: "Domain", Width: 8, Sortable: true},
		},
	},
	{
		Name:      "ACM Certificates",
		ShortName: "acm",
		Aliases:   []string{"acm", "certificates", "certs"},
		Columns: []Column{
			{Key: "domain_name", Title: "Domain Name", Width: 40, Sortable: true},
			{Key: "status", Title: "Status", Width: 14, Sortable: true},
			{Key: "type", Title: "Type", Width: 14, Sortable: true},
			{Key: "not_after", Title: "Expires", Width: 22, Sortable: true},
			{Key: "in_use", Title: "In Use", Width: 8, Sortable: true},
		},
	},
	{
		Name:      "Auto Scaling Groups",
		ShortName: "asg",
		Aliases:   []string{"asg", "autoscaling", "auto-scaling"},
		Columns: []Column{
			{Key: "asg_name", Title: "ASG Name", Width: 36, Sortable: true},
			{Key: "min_size", Title: "Min", Width: 6, Sortable: true},
			{Key: "max_size", Title: "Max", Width: 6, Sortable: true},
			{Key: "desired", Title: "Desired", Width: 8, Sortable: true},
			{Key: "instances", Title: "Instances", Width: 10, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
		},
	},
	{
		Name:      "ECS Tasks",
		ShortName: "ecs-task",
		Aliases:   []string{"ecs-task", "ecs-tasks", "tasks"},
		Columns: []Column{
			{Key: "task_id", Title: "Task ID", Width: 38, Sortable: true},
			{Key: "cluster", Title: "Cluster", Width: 24, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "task_definition", Title: "Task Definition", Width: 30, Sortable: true},
			{Key: "launch_type", Title: "Launch", Width: 10, Sortable: true},
			{Key: "cpu", Title: "CPU", Width: 6, Sortable: true},
			{Key: "memory", Title: "Memory", Width: 8, Sortable: true},
		},
	},
	{
		Name:      "IAM Policies",
		ShortName: "policy",
		Aliases:   []string{"policy", "policies", "iam-policies"},
		Columns: []Column{
			{Key: "policy_name", Title: "Policy Name", Width: 36, Sortable: true},
			{Key: "policy_id", Title: "Policy ID", Width: 22, Sortable: true},
			{Key: "attachment_count", Title: "Attached", Width: 10, Sortable: true},
			{Key: "path", Title: "Path", Width: 20, Sortable: true},
			{Key: "create_date", Title: "Created", Width: 22, Sortable: true},
		},
	},
	{
		Name:      "RDS Snapshots",
		ShortName: "rds-snap",
		Aliases:   []string{"rds-snap", "rds-snapshots", "db-snapshots"},
		Columns: []Column{
			{Key: "snapshot_id", Title: "Snapshot ID", Width: 36, Sortable: true},
			{Key: "db_instance", Title: "DB Instance", Width: 28, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "engine", Title: "Engine", Width: 12, Sortable: true},
			{Key: "snapshot_type", Title: "Type", Width: 12, Sortable: true},
			{Key: "created", Title: "Created", Width: 22, Sortable: true},
		},
	},
	{
		Name:      "Transit Gateways",
		ShortName: "tgw",
		Aliases:   []string{"tgw", "transit-gateways", "transitgateways"},
		Columns: []Column{
			{Key: "tgw_id", Title: "TGW ID", Width: 26, Sortable: true},
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "owner_id", Title: "Owner", Width: 14, Sortable: true},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
		},
	},
	{
		Name:      "VPC Endpoints",
		ShortName: "vpce",
		Aliases:   []string{"vpce", "vpc-endpoints", "vpcendpoints"},
		Columns: []Column{
			{Key: "vpce_id", Title: "Endpoint ID", Width: 26, Sortable: true},
			{Key: "service_name", Title: "Service Name", Width: 40, Sortable: true},
			{Key: "type", Title: "Type", Width: 12, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
		},
	},
	{
		Name:      "Network Interfaces",
		ShortName: "eni",
		Aliases:   []string{"eni", "network-interfaces", "nis"},
		Columns: []Column{
			{Key: "eni_id", Title: "ENI ID", Width: 26, Sortable: true},
			{Key: "name", Title: "Name", Width: 24, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "type", Title: "Type", Width: 14, Sortable: true},
			{Key: "vpc_id", Title: "VPC ID", Width: 24, Sortable: true},
			{Key: "private_ip", Title: "Private IP", Width: 16, Sortable: false},
		},
	},
	{
		Name:      "SNS Subscriptions",
		ShortName: "sns-sub",
		Aliases:   []string{"sns-sub", "sns-subscriptions", "subscriptions"},
		Columns: []Column{
			{Key: "topic_arn", Title: "Topic ARN", Width: 48, Sortable: true},
			{Key: "protocol", Title: "Protocol", Width: 10, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 48, Sortable: false},
			{Key: "subscription_arn", Title: "Subscription ARN", Width: 60, Sortable: false},
		},
	},
	{
		Name:      "IAM Users",
		ShortName: "iam-user",
		Aliases:   []string{"iam-user", "iam-users", "users"},
		Columns: []Column{
			{Key: "user_name", Title: "User Name", Width: 32, Sortable: true},
			{Key: "user_id", Title: "User ID", Width: 22, Sortable: true},
			{Key: "path", Title: "Path", Width: 20, Sortable: true},
			{Key: "create_date", Title: "Created", Width: 22, Sortable: true},
			{Key: "password_last_used", Title: "Password Last Used", Width: 22, Sortable: true},
		},
	},
	{
		Name:      "IAM Groups",
		ShortName: "iam-group",
		Aliases:   []string{"iam-group", "iam-groups", "groups"},
		Columns: []Column{
			{Key: "group_name", Title: "Group Name", Width: 32, Sortable: true},
			{Key: "group_id", Title: "Group ID", Width: 22, Sortable: true},
			{Key: "path", Title: "Path", Width: 20, Sortable: true},
			{Key: "create_date", Title: "Created", Width: 22, Sortable: true},
			{Key: "arn", Title: "ARN", Width: 60, Sortable: true},
		},
	},
	{
		Name:      "DocDB Snapshots",
		ShortName: "docdb-snap",
		Aliases:   []string{"docdb-snap", "docdb-snapshots", "cluster-snapshots"},
		Columns: []Column{
			{Key: "snapshot_id", Title: "Snapshot ID", Width: 36, Sortable: true},
			{Key: "cluster_id", Title: "Cluster ID", Width: 28, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "engine", Title: "Engine", Width: 12, Sortable: true},
			{Key: "snapshot_type", Title: "Type", Width: 12, Sortable: true},
			{Key: "snapshot_create_time", Title: "Created", Width: 22, Sortable: true},
			{Key: "storage_type", Title: "Storage", Width: 10, Sortable: true},
		},
	},
	// --- Batch 2a: CloudFront, Route 53, API Gateway, ECR, EFS, EventBridge, Step Functions, CodePipeline ---
	{
		Name:      "CloudFront Distributions",
		ShortName: "cf",
		Aliases:   []string{"cf", "cloudfront", "cdn"},
		Columns: []Column{
			{Key: "distribution_id", Title: "Distribution ID", Width: 16, Sortable: true},
			{Key: "domain_name", Title: "Domain Name", Width: 40, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "enabled", Title: "Enabled", Width: 9, Sortable: true},
			{Key: "aliases", Title: "Aliases", Width: 30, Sortable: false},
			{Key: "price_class", Title: "Price Class", Width: 16, Sortable: true},
		},
	},
	{
		Name:      "Route 53 Hosted Zones",
		ShortName: "r53",
		Aliases:   []string{"r53", "route53", "dns", "hosted-zones"},
		Columns: []Column{
			{Key: "zone_id", Title: "Zone ID", Width: 30, Sortable: true},
			{Key: "name", Title: "Name", Width: 36, Sortable: true},
			{Key: "record_count", Title: "Records", Width: 9, Sortable: true},
			{Key: "private_zone", Title: "Private", Width: 9, Sortable: true},
			{Key: "comment", Title: "Comment", Width: 30, Sortable: false},
		},
	},
	{
		Name:      "API Gateways",
		ShortName: "apigw",
		Aliases:   []string{"apigw", "apigateway", "api-gateway"},
		Columns: []Column{
			{Key: "api_id", Title: "API ID", Width: 14, Sortable: true},
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "protocol", Title: "Protocol", Width: 12, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 50, Sortable: false},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
		},
	},
	{
		Name:      "ECR Repositories",
		ShortName: "ecr",
		Aliases:   []string{"ecr", "container-registry"},
		Columns: []Column{
			{Key: "repository_name", Title: "Repository", Width: 36, Sortable: true},
			{Key: "uri", Title: "URI", Width: 60, Sortable: false},
			{Key: "tag_mutability", Title: "Tag Mutability", Width: 16, Sortable: true},
			{Key: "scan_on_push", Title: "Scan", Width: 6, Sortable: true},
			{Key: "created_at", Title: "Created", Width: 22, Sortable: true},
		},
	},
	{
		Name:      "EFS File Systems",
		ShortName: "efs",
		Aliases:   []string{"efs", "file-systems"},
		Columns: []Column{
			{Key: "file_system_id", Title: "File System ID", Width: 22, Sortable: true},
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "life_cycle_state", Title: "State", Width: 12, Sortable: true},
			{Key: "performance_mode", Title: "Perf Mode", Width: 16, Sortable: true},
			{Key: "encrypted", Title: "Encrypted", Width: 10, Sortable: true},
			{Key: "mount_targets", Title: "Mounts", Width: 8, Sortable: true},
		},
	},
	{
		Name:      "EventBridge Rules",
		ShortName: "eb-rule",
		Aliases:   []string{"eb-rule", "eventbridge", "events"},
		Columns: []Column{
			{Key: "name", Title: "Rule Name", Width: 28, Sortable: true},
			{Key: "state", Title: "State", Width: 10, Sortable: true},
			{Key: "event_bus", Title: "Event Bus", Width: 18, Sortable: true},
			{Key: "schedule", Title: "Schedule", Width: 24, Sortable: false},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
		},
	},
	{
		Name:      "Step Functions",
		ShortName: "sfn",
		Aliases:   []string{"sfn", "stepfunctions", "state-machines"},
		Columns: []Column{
			{Key: "name", Title: "Name", Width: 36, Sortable: true},
			{Key: "type", Title: "Type", Width: 10, Sortable: true},
			{Key: "arn", Title: "ARN", Width: 60, Sortable: false},
			{Key: "creation_date", Title: "Created", Width: 22, Sortable: true},
		},
	},
	{
		Name:      "CodePipelines",
		ShortName: "pipeline",
		Aliases:   []string{"pipeline", "codepipeline", "pipelines"},
		Columns: []Column{
			{Key: "name", Title: "Pipeline Name", Width: 30, Sortable: true},
			{Key: "pipeline_type", Title: "Type", Width: 6, Sortable: true},
			{Key: "version", Title: "Version", Width: 9, Sortable: true},
			{Key: "created", Title: "Created", Width: 22, Sortable: true},
			{Key: "updated", Title: "Updated", Width: 22, Sortable: true},
		},
	},
	// --- Batch 2b: Kinesis, WAF, Glue, Elastic Beanstalk, SES, Redshift, CloudTrail, Athena, CodeArtifact ---
	{
		Name:      "Kinesis Streams",
		ShortName: "kinesis",
		Aliases:   []string{"kinesis", "streams"},
		Columns: []Column{
			{Key: "stream_name", Title: "Stream Name", Width: 36, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "stream_mode", Title: "Mode", Width: 14, Sortable: true},
			{Key: "creation_time", Title: "Created", Width: 22, Sortable: true},
		},
	},
	{
		Name:      "WAF Web ACLs",
		ShortName: "waf",
		Aliases:   []string{"waf", "webacl", "web-acl"},
		Columns: []Column{
			{Key: "name", Title: "Name", Width: 28, Sortable: true},
			{Key: "id", Title: "ID", Width: 38, Sortable: true},
			{Key: "description", Title: "Description", Width: 36, Sortable: false},
		},
	},
	{
		Name:      "Glue Jobs",
		ShortName: "glue",
		Aliases:   []string{"glue", "glue-jobs"},
		Columns: []Column{
			{Key: "job_name", Title: "Job Name", Width: 32, Sortable: true},
			{Key: "glue_version", Title: "Version", Width: 10, Sortable: true},
			{Key: "worker_type", Title: "Worker Type", Width: 14, Sortable: true},
			{Key: "num_workers", Title: "Workers", Width: 9, Sortable: true},
			{Key: "last_modified", Title: "Last Modified", Width: 22, Sortable: true},
		},
	},
	{
		Name:      "Elastic Beanstalk",
		ShortName: "eb",
		Aliases:   []string{"eb", "beanstalk", "elastic-beanstalk"},
		Columns: []Column{
			{Key: "environment_name", Title: "Environment", Width: 28, Sortable: true},
			{Key: "application_name", Title: "Application", Width: 24, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "health", Title: "Health", Width: 10, Sortable: true},
			{Key: "version_label", Title: "Version", Width: 16, Sortable: true},
		},
	},
	{
		Name:      "SES Identities",
		ShortName: "ses",
		Aliases:   []string{"ses", "email", "ses-identities"},
		Columns: []Column{
			{Key: "identity_name", Title: "Identity", Width: 36, Sortable: true},
			{Key: "identity_type", Title: "Type", Width: 16, Sortable: true},
			{Key: "verification_status", Title: "Verification", Width: 16, Sortable: true},
			{Key: "sending_enabled", Title: "Sending", Width: 10, Sortable: true},
		},
	},
	{
		Name:      "Redshift Clusters",
		ShortName: "redshift",
		Aliases:   []string{"redshift", "redshift-clusters"},
		Columns: []Column{
			{Key: "cluster_id", Title: "Cluster ID", Width: 28, Sortable: true},
			{Key: "status", Title: "Status", Width: 14, Sortable: true},
			{Key: "node_type", Title: "Node Type", Width: 16, Sortable: true},
			{Key: "num_nodes", Title: "Nodes", Width: 7, Sortable: true},
			{Key: "db_name", Title: "Database", Width: 16, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 44, Sortable: false},
		},
	},
	{
		Name:      "CloudTrail Trails",
		ShortName: "trail",
		Aliases:   []string{"trail", "cloudtrail", "trails"},
		Columns: []Column{
			{Key: "trail_name", Title: "Trail Name", Width: 28, Sortable: true},
			{Key: "s3_bucket", Title: "S3 Bucket", Width: 28, Sortable: true},
			{Key: "home_region", Title: "Home Region", Width: 16, Sortable: true},
			{Key: "multi_region", Title: "Multi-Region", Width: 14, Sortable: true},
		},
	},
	{
		Name:      "Athena Workgroups",
		ShortName: "athena",
		Aliases:   []string{"athena", "workgroups"},
		Columns: []Column{
			{Key: "workgroup_name", Title: "Workgroup", Width: 28, Sortable: true},
			{Key: "state", Title: "State", Width: 12, Sortable: true},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
			{Key: "engine_version", Title: "Engine", Width: 28, Sortable: true},
		},
	},
	{
		Name:      "CodeArtifact Repos",
		ShortName: "codeartifact",
		Aliases:   []string{"codeartifact", "artifact", "ca"},
		Columns: []Column{
			{Key: "repo_name", Title: "Repository", Width: 28, Sortable: true},
			{Key: "domain_name", Title: "Domain", Width: 24, Sortable: true},
			{Key: "description", Title: "Description", Width: 30, Sortable: false},
			{Key: "domain_owner", Title: "Owner", Width: 14, Sortable: true},
		},
	},
	// --- Batch 3: CodeBuild, OpenSearch, KMS, MSK, Backup ---
	{
		Name:      "CodeBuild Projects",
		ShortName: "cb",
		Aliases:   []string{"cb", "codebuild"},
		Columns: []Column{
			{Key: "name", Title: "Project Name", Width: 32, Sortable: true},
			{Key: "source_type", Title: "Source Type", Width: 14, Sortable: true},
			{Key: "description", Title: "Description", Width: 36, Sortable: false},
			{Key: "last_modified", Title: "Last Modified", Width: 22, Sortable: true},
		},
	},
	{
		Name:      "OpenSearch Domains",
		ShortName: "opensearch",
		Aliases:   []string{"opensearch", "os", "elasticsearch"},
		Columns: []Column{
			{Key: "domain_name", Title: "Domain Name", Width: 28, Sortable: true},
			{Key: "engine_version", Title: "Engine Version", Width: 16, Sortable: true},
			{Key: "instance_type", Title: "Instance Type", Width: 22, Sortable: true},
			{Key: "instance_count", Title: "Instances", Width: 10, Sortable: true},
			{Key: "endpoint", Title: "Endpoint", Width: 48, Sortable: false},
		},
	},
	{
		Name:      "KMS Keys",
		ShortName: "kms",
		Aliases:   []string{"kms", "keys"},
		Columns: []Column{
			{Key: "alias", Title: "Alias", Width: 32, Sortable: true},
			{Key: "key_id", Title: "Key ID", Width: 38, Sortable: true},
			{Key: "status", Title: "Status", Width: 12, Sortable: true},
			{Key: "description", Title: "Description", Width: 36, Sortable: false},
		},
	},
	{
		Name:      "MSK Clusters",
		ShortName: "msk",
		Aliases:   []string{"msk", "kafka"},
		Columns: []Column{
			{Key: "cluster_name", Title: "Cluster Name", Width: 28, Sortable: true},
			{Key: "cluster_type", Title: "Type", Width: 14, Sortable: true},
			{Key: "state", Title: "State", Width: 14, Sortable: true},
			{Key: "version", Title: "Version", Width: 14, Sortable: true},
		},
	},
	{
		Name:      "Backup Plans",
		ShortName: "backup",
		Aliases:   []string{"backup", "backup-plans"},
		Columns: []Column{
			{Key: "plan_name", Title: "Plan Name", Width: 32, Sortable: true},
			{Key: "plan_id", Title: "Plan ID", Width: 38, Sortable: true},
			{Key: "creation_date", Title: "Created", Width: 22, Sortable: true},
			{Key: "last_execution", Title: "Last Execution", Width: 22, Sortable: true},
		},
	},
}

// AllResourceTypes returns the definitions for all supported resource types.
func AllResourceTypes() []ResourceTypeDef {
	result := make([]ResourceTypeDef, len(resourceTypes))
	copy(result, resourceTypes)
	return result
}

// AllShortNames returns the ShortName of every registered resource type.
func AllShortNames() []string {
	names := make([]string, len(resourceTypes))
	for i, rt := range resourceTypes {
		names[i] = rt.ShortName
	}
	return names
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
