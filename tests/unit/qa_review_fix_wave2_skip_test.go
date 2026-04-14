package unit

import (
	"testing"

	awsclient "github.com/k2m30/a9s/v3/internal/aws"
)

// TestEnricherRegistryPopulated verifies that all expected Wave 2 enricher keys
// are present in the registry. A missing key means that resource type will never
// get enrichment, silently skipping issue detection.
// This is the fix from issue #196 bug 4: Wave 2 must be started in live sessions.
func TestEnricherRegistryPopulated(t *testing.T) {
	// These are the resource types that have Wave 2 enrichers as of issue #196.
	expected := []string{
		"rds",  // RDS/DocDB pending maintenance (batch)
		"dbi",  // RDS instance maintenance (batch, shared enricher with rds)
		"ec2",  // EC2 status checks (batch)
		"ebs",  // EBS volume status (batch)
		"cb",   // CodeBuild project status (batch)
		"tg",   // Target group health (per-resource, capped)
		"pipe", // CodePipeline status (per-resource, capped)
		"ddb",  // DynamoDB table status (per-resource, capped)
		"sfn",  // Step Functions execution status (per-resource, capped)
		"glue", // Glue job status (per-resource, capped)
	}

	for _, key := range expected {
		t.Run(key, func(t *testing.T) {
			fn, ok := awsclient.EnricherRegistry[key]
			if !ok {
				t.Errorf("EnricherRegistry missing key %q — Wave 2 enrichment will be silently skipped for this resource type", key)
				return
			}
			if fn == nil {
				t.Errorf("EnricherRegistry[%q] is nil — enricher registered but not implemented", key)
			}
		})
	}
}

// TestEnricherRegistryHasNoNilEntries verifies that every entry in EnricherRegistry
// is a non-nil function. A nil entry is a registration bug — it passes the "key
// present" check but panics at call time.
func TestEnricherRegistryHasNoNilEntries(t *testing.T) {
	for key, fn := range awsclient.EnricherRegistry {
		if fn == nil {
			t.Errorf("EnricherRegistry[%q] is nil — must be a non-nil EnricherFunc", key)
		}
	}
}

// TestEnricherRegistryIsNotEmpty verifies the registry is not accidentally cleared.
// If it is empty, Wave 2 will silently produce no results regardless of live/demo mode.
func TestEnricherRegistryIsNotEmpty(t *testing.T) {
	if len(awsclient.EnricherRegistry) == 0 {
		t.Fatal("EnricherRegistry is empty — no Wave 2 enrichers are registered")
	}
}
