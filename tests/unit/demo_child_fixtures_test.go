package unit

import (
	"testing"

	demo "github.com/k2m30/a9s/v3/internal/demo"
)

// ═══════════════════════════════════════════════════════════════════════════
// Demo child fixture coverage tests — exercise every child demo fixture
// generator so they appear in code coverage. Each child type is registered
// via RegisterChildDemo() in the fixtures_*.go category files.
// ═══════════════════════════════════════════════════════════════════════════

// ---------------------------------------------------------------------------
// 1. TestDemo_AllChildFixturesReturnData
// ---------------------------------------------------------------------------

func TestDemo_AllChildFixturesReturnData(t *testing.T) {
	childTypes := []string{
		"asg_activities",
		"ecs_svc_events",
		"ecs_tasks",
		"ecs_svc_logs",
		"tg_health",
		"log_streams",
		"log_events",
		"alarm_history",
		"lambda_invocations",
		"lambda_invocation_logs",
		"elb_listeners",
		"elb_listener_rules",
		"role_policies",
		"iam_group_members",
		"cfn_events",
		"cfn_resources",
		"ecr_images",
		"sfn_executions",
		"sfn_execution_history",
		"cb_builds",
		"cb_build_logs",
		"pipeline_stages",
	}

	parentCtx := map[string]string{"ID": "test", "Name": "test"}

	for _, childType := range childTypes {
		t.Run(childType, func(t *testing.T) {
			resources, ok := demo.GetChildResources(childType, parentCtx)
			if !ok {
				t.Fatalf("GetChildResources(%q, parentCtx) returned ok=false; expected true", childType)
			}
			if len(resources) == 0 {
				t.Fatalf("GetChildResources(%q, parentCtx) returned empty slice; expected non-empty", childType)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// 2. TestDemo_AllS3BucketObjectFixturesReturnData
// ---------------------------------------------------------------------------

func TestDemo_AllS3BucketObjectFixturesReturnData(t *testing.T) {
	buckets := []string{
		"webapp-assets-prod",
		"ml-training-data",
		"terraform-state-prod",
		"cloudtrail-audit-logs",
		"backup-db-snapshots",
	}

	for _, bucket := range buckets {
		t.Run(bucket, func(t *testing.T) {
			resources, ok := demo.GetS3Objects(bucket, "")
			if !ok {
				t.Fatalf("GetS3Objects(%q, \"\") returned ok=false; expected true", bucket)
			}
			if len(resources) == 0 {
				t.Fatalf("GetS3Objects(%q, \"\") returned empty slice; expected non-empty", bucket)
			}
		})
	}
}
