package unit

// qa_tier_a_coverage_test.go — Group 5: column-key cross-check sweep.
//
// Verifies that DefaultViewDef for every resource type that the 017-issue-counts
// branch targets actually contains a List column whose Key (or Path for path-backed
// columns) matches the intended attention-column field.
//
// This test will FAIL until the coder adds all 23 attention columns to the
// internal/config/defaults_*.go files.

import (
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/internal/config"
)

// attentionColumnCase describes one expected attention column in a resource list view.
type attentionColumnCase struct {
	// resourceType is the short name used in DefaultViewDef (e.g. "tg", "vpc").
	resourceType string
	// key is the ListColumn.Key that must exist (mutually exclusive with path).
	key string
	// path is the ListColumn.Path substring that must exist (used when a path-backed
	// column is used instead of a key-backed FieldUpdates column).
	path string
	// description is a human-readable description used in t.Errorf messages.
	description string
}

// allAttentionColumns lists all 23 attention columns introduced in 017-issue-counts.
var allAttentionColumns = []attentionColumnCase{
	// Group 1: Wave-2 enricher FieldUpdates
	{resourceType: "tg", key: "health_summary", description: "TG health summary"},
	{resourceType: "vpc", key: "flow_logs", description: "VPC flow logs"},
	{resourceType: "tgw", key: "att_status", description: "TGW attachment issues"},
	{resourceType: "sqs", key: "dlq", description: "SQS DLQ present"},
	{resourceType: "sns", key: "subs_count", description: "SNS subscription count"},
	{resourceType: "sfn", key: "last_run", description: "Step Functions last run"},
	{resourceType: "policy", key: "risk", description: "IAM policy risk"},
	{resourceType: "waf", key: "rules_summary", description: "WAF rules summary"},
	{resourceType: "apigw", key: "stages_count", description: "API Gateway stages count"},
	{resourceType: "pipeline", key: "last_status", description: "CodePipeline last status"},
	{resourceType: "cb", key: "last_build", description: "CodeBuild last build"},
	{resourceType: "codeartifact", key: "package_count", description: "CodeArtifact package count"},
	{resourceType: "glue", key: "last_run", description: "Glue job last run"},
	{resourceType: "backup", key: "last_status", description: "Backup plan last status"},

	// Group 2: Wave-1 fetcher-stored fields referenced by Key in list view
	{resourceType: "ec2", key: "instance_status", description: "EC2 instance health status"},

	// Redshift pending: path-based column (PendingModifiedValues.NodeType), not key.
	{resourceType: "redshift", path: "PendingModifiedValues", description: "Redshift pending modifications column"},

	// Group 3: Fetcher-computed Wave-1 field additions
	{resourceType: "rtb", key: "blackhole_routes_count", description: "Route table blackhole count"},
	{resourceType: "eip", key: "status", description: "EIP attachment status"},
	{resourceType: "dbi", key: "status", description: "derived status phrase"},
	{resourceType: "dbc", key: "status", description: "DocumentDB cluster derived status phrase (docs/resources/dbc.md §4)"},
	{resourceType: "secrets", key: "status", description: "Secrets Manager OVERDUE status"},
	{resourceType: "ssm", key: "risk", description: "SSM parameter staleness risk"},

	// Group 4: Cosmetic format fields
	{resourceType: "acm", key: "days_left", description: "ACM certificate days left"},
	{resourceType: "ami", key: "deprecated", description: "AMI deprecation status"},
}

// TestTierA_AllAttentionColumnsHaveBackingField is a sweep test that verifies every
// attention column in the 017-issue-counts scope appears in the DefaultViewDef for
// its resource type. For Key-backed columns, checks ListColumn.Key. For Path-backed
// columns, checks ListColumn.Path contains the path prefix.
func TestTierA_AllAttentionColumnsHaveBackingField(t *testing.T) {
	for _, tc := range allAttentionColumns {
		tc := tc // capture range var
		t.Run(tc.resourceType+"/"+tc.description, func(t *testing.T) {
			viewDef := config.DefaultViewDef(tc.resourceType)
			if len(viewDef.List) == 0 {
				t.Fatalf("DefaultViewDef(%q) returned empty List — resource type not registered or coder has not added the view yet", tc.resourceType)
			}

			if tc.key != "" {
				// Key-backed column: look for a ListColumn with matching Key.
				found := false
				for _, col := range viewDef.List {
					if col.Key == tc.key {
						found = true
						break
					}
				}
				if !found {
					keys := make([]string, 0, len(viewDef.List))
					for _, col := range viewDef.List {
						if col.Key != "" {
							keys = append(keys, col.Key)
						}
					}
					t.Errorf(
						"%s: no column with Key=%q found in DefaultViewDef(%q) List\n  existing keys: %v\n  coder must add {Key: %q} column to defaults_%s.go",
						tc.description, tc.key, tc.resourceType, keys, tc.key, tc.resourceType,
					)
				}
			} else if tc.path != "" {
				// Path-backed column: look for a ListColumn whose Path contains the expected substring.
				found := false
				for _, col := range viewDef.List {
					if strings.Contains(col.Path, tc.path) {
						found = true
						break
					}
				}
				if !found {
					paths := make([]string, 0, len(viewDef.List))
					for _, col := range viewDef.List {
						if col.Path != "" {
							paths = append(paths, col.Path)
						}
					}
					t.Errorf(
						"%s: no column with Path containing %q found in DefaultViewDef(%q) List\n  existing paths: %v\n  coder must add a column with Path containing %q to defaults_%s.go",
						tc.description, tc.path, tc.resourceType, paths, tc.path, tc.resourceType,
					)
				}
			}
		})
	}
}
