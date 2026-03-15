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
	"rds":     {"db_identifier", "engine", "engine_version", "status", "class", "endpoint", "multi_az"},
	"redis":   {"cluster_id", "engine_version", "node_type", "status", "nodes", "endpoint"},
	"docdb":   {"cluster_id", "engine_version", "status", "instances", "endpoint"},
	"eks":     {"cluster_name", "version", "status", "endpoint", "platform_version"},
	"secrets": {"secret_name", "description", "last_accessed", "last_changed", "rotation_enabled"},
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
