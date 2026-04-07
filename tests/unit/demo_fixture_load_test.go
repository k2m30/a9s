package unit

// demo_fixture_load_test.go — smoke tests for demo fixture loading.
//
// Two goals:
//   1. TestDemo_AllFixturesLoadWithoutPanic — every registered demo fetcher
//      (main, child, and related) must invoke without panicking.
//   2. TestDemo_ParseTime_BadLiteral — demo.ParseTime must return an error
//      rather than panic when given a malformed RFC3339 string.
//      This test FAILS (compile error) until the coder converts mustParseTime
//      from panic → (time.Time, error) and exposes ParseTime in the demo package.

import (
	"fmt"
	"testing"

	_ "github.com/k2m30/a9s/v3/internal/aws" // registers all resource types via init()
	"github.com/k2m30/a9s/v3/internal/demo"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// ---------------------------------------------------------------------------
// 1. TestDemo_AllFixturesLoadWithoutPanic
// ---------------------------------------------------------------------------

// TestDemo_AllFixturesLoadWithoutPanic iterates every registered demo fetcher
// and asserts that invoking it does not panic.  It covers three registries:
//
//   - demoData (main resource fixtures) — iterated via resource.AllResourceTypes()
//   - childDemoData (child-view fixtures) — iterated by known child type keys
//   - relatedDemoRegistry (related-panel checkers) — iterated via resource.AllResourceTypes()
func TestDemo_AllFixturesLoadWithoutPanic(t *testing.T) {
	// ---- Part A: main demoData fixtures ----
	t.Run("main_fixtures", func(t *testing.T) {
		for _, rt := range resource.AllResourceTypes() {
			rt := rt // capture
			t.Run(rt.ShortName, func(t *testing.T) {
				didPanic := catchPanic(func() {
					demo.GetResources(rt.ShortName)
				})
				if didPanic != "" {
					t.Errorf("GetResources(%q) panicked: %s", rt.ShortName, didPanic)
				}
			})
		}
	})

	// ---- Part B: child demo fixtures ----
	// Child types are registered via RegisterChildDemo.  We enumerate all known
	// child type keys that are wired up in the fixtures_*.go category files.
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
		"cb_builds",
		"pipeline_stages",
		"sfn_executions",
		"sfn_execution_history",
		"eb_rule_targets",
		"sns_subscriptions",
		"dbi_events",
		"glue_runs",
		"s3_objects",
		"r53_records",
	}
	t.Run("child_fixtures", func(t *testing.T) {
		// Use a minimal parentCtx that satisfies the most common key patterns.
		parentCtx := map[string]string{
			"asg_name":       "demo-asg",
			"service_name":   "demo-svc",
			"cluster_name":   "demo-cluster",
			"log_group_name": "demo-log-group",
			"alarm_name":     "demo-alarm",
			"function_name":  "demo-fn",
			"role_name":      "demo-role",
			"group_name":     "demo-group",
			"stack_name":     "demo-stack",
			"project_name":   "demo-project",
			"pipeline_name":  "demo-pipeline",
			"db_identifier":  "demo-db",
			"job_name":       "demo-job",
			"bucket":         "data-pipeline-logs",
			"prefix":         "",
			"zone_id":        "Z0DEMO0HOSTED0ZONE",
			"topic_arn":      "arn:aws:sns:us-east-1:123456789012:demo-topic",
			"state_machine_arn": "arn:aws:states:us-east-1:123456789012:stateMachine:demo",
			"repository_name": "demo-repo",
			"listener_arn":   "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/demo",
			"load_balancer_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/demo",
			"target_group_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/demo/abc",
		}
		for _, childType := range childTypes {
			childType := childType // capture
			t.Run(childType, func(t *testing.T) {
				didPanic := catchPanic(func() {
					demo.GetChildResources(childType, parentCtx)
				})
				if didPanic != "" {
					t.Errorf("GetChildResources(%q) panicked: %s", childType, didPanic)
				}
			})
		}
	})

	// ---- Part C: related demo checkers ----
	t.Run("related_checkers", func(t *testing.T) {
		dummyRes := resource.Resource{
			ID:   "demo-id",
			Name: "demo-name",
		}
		for _, rt := range resource.AllResourceTypes() {
			rt := rt // capture
			checker := resource.GetRelatedDemo(rt.ShortName)
			if checker == nil {
				// Not every type has a related checker — skip silently.
				continue
			}
			t.Run(rt.ShortName, func(t *testing.T) {
				didPanic := catchPanic(func() {
					checker(dummyRes)
				})
				if didPanic != "" {
					t.Errorf("related checker for %q panicked: %s", rt.ShortName, didPanic)
				}
			})
		}
	})
}

// ---------------------------------------------------------------------------
// 2. TestDemo_ParseTime_BadLiteral
// ---------------------------------------------------------------------------

// TestDemo_ParseTime_BadLiteral verifies that demo.ParseTime returns an error
// (not a panic) when given a string that is not valid RFC3339.
//
// This test FAILS to compile until the coder:
//   - renames mustParseTime → exports ParseTime(s string) (time.Time, error)
//   - removes the panic branch from fixtures.go:130
//
// Once the coder does that, the test compiles and passes.
func TestDemo_ParseTime_BadLiteral(t *testing.T) {
	badInputs := []string{
		"not-a-time",
		"2026-13-01T00:00:00Z",  // month 13
		"2026-04-07",            // date only, not RFC3339
		"",
		"9999-99-99T99:99:99Z",
	}

	for _, s := range badInputs {
		s := s // capture
		t.Run(fmt.Sprintf("input=%q", s), func(t *testing.T) {
			var panicked string
			func() {
				defer func() {
					if r := recover(); r != nil {
						panicked = fmt.Sprintf("%v", r)
					}
				}()
				_, err := demo.ParseTime(s)
				if err == nil {
					t.Errorf("ParseTime(%q): expected non-nil error for malformed input, got nil", s)
				}
			}()
			if panicked != "" {
				t.Errorf("ParseTime(%q) panicked instead of returning error: %s", s, panicked)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// catchPanic calls f and returns a non-empty string if f panicked.
func catchPanic(f func()) (panicMsg string) {
	defer func() {
		if r := recover(); r != nil {
			panicMsg = fmt.Sprintf("%v", r)
		}
	}()
	f()
	return ""
}
