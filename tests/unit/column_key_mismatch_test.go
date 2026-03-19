package unit

import (
	"testing"

	"github.com/k2m30/a9s/internal/resource"
)

// Test that every column Key in every ResourceTypeDef has a corresponding
// Fields key in the expected fetcher output. This catches mismatches between
// types.go column definitions and aws/*.go fetcher Fields keys.
//
// Known valid Fields keys per resource type (from aws/*.go fetchers):
var expectedFieldKeys = map[string][]string{
	"s3":      {"name", "bucket_name", "creation_date"},
	"ec2":     {"instance_id", "name", "state", "type", "private_ip", "public_ip", "launch_time"},
	"dbi":     {"db_identifier", "engine", "engine_version", "status", "class", "endpoint", "multi_az"},
	"redis":   {"cluster_id", "engine_version", "node_type", "status", "nodes", "endpoint"},
	"dbc":     {"cluster_id", "engine_version", "status", "instances", "endpoint"},
	"eks":     {"cluster_name", "version", "status", "endpoint", "platform_version"},
	"secrets": {"secret_name", "description", "last_accessed", "last_changed", "rotation_enabled"},
	"vpc":     {"vpc_id", "name", "cidr_block", "state", "is_default"},
	"sg":      {"group_id", "group_name", "vpc_id", "description"},
	"ng":      {"nodegroup_name", "cluster_name", "status", "instance_types", "desired_size"},
	"subnet":  {"subnet_id", "name", "vpc_id", "cidr_block", "availability_zone", "state", "available_ips"},
	"rtb":     {"route_table_id", "name", "vpc_id", "routes_count", "associations_count"},
	"nat":     {"nat_gateway_id", "name", "vpc_id", "subnet_id", "state", "public_ip"},
	"igw":     {"igw_id", "name", "vpc_id", "state"},
	"lambda":  {"function_name", "runtime", "memory", "timeout", "handler", "last_modified", "code_size"},
	"alarm":   {"alarm_name", "state", "metric_name", "namespace", "threshold"},
	"sns":     {"topic_arn", "display_name"},
	"sqs":     {"queue_name", "queue_url", "approx_messages", "approx_not_visible", "delay_seconds"},
	"elb":     {"name", "dns_name", "type", "scheme", "state", "vpc_id"},
	"tg":      {"target_group_name", "port", "protocol", "vpc_id", "target_type", "health_check_path"},
	"ecs":     {"cluster_name", "status", "running_tasks", "pending_tasks", "services_count"},
	"ecs-svc": {"service_name", "cluster", "status", "desired_count", "running_count", "launch_type"},
	"cfn":     {"stack_name", "status", "creation_time", "last_updated", "description"},
	"role":    {"role_name", "role_id", "path", "create_date", "description"},
	"logs":    {"log_group_name", "stored_bytes", "retention_days", "creation_time"},
	"ssm":     {"name", "type", "version", "last_modified", "description"},
	"ddb":     {"table_name", "status", "item_count", "size_bytes", "billing_mode"},
	"eip":     {"allocation_id", "name", "public_ip", "association_id", "instance_id", "domain"},
	"acm":     {"domain_name", "status", "type", "not_after", "in_use"},
	"asg":     {"asg_name", "min_size", "max_size", "desired", "instances", "status"},
}

func TestColumnKeys_MatchFetcherFieldKeys(t *testing.T) {
	for _, rt := range resource.AllResourceTypes() {
		validKeys, ok := expectedFieldKeys[rt.ShortName]
		if !ok {
			t.Errorf("no expected field keys defined for resource type %q", rt.ShortName)
			continue
		}

		validSet := make(map[string]bool)
		for _, k := range validKeys {
			validSet[k] = true
		}

		for _, col := range rt.Columns {
			if !validSet[col.Key] {
				t.Errorf("resource type %q: column Key %q does not match any fetcher Fields key. Valid keys: %v",
					rt.ShortName, col.Key, validKeys)
			}
		}
	}
}
