// Per-child-type contract for catalog.ResourceTypeDef.ConsoleURL.
//
// Every child ConsoleURL builder reads only r.ID/r.Fields, never RawStruct, so
// a hand-built Fields map exercises the same code path a real fixture would.
package unit_test

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// childConsoleURLTypeDef returns the installed child-registry entry for
// shortName, failing loudly if the type is not registered.
func childConsoleURLTypeDef(t *testing.T, shortName string) resource.ResourceTypeDef {
	t.Helper()
	td := resource.GetChildType(shortName)
	if td == nil {
		t.Fatalf("resource.GetChildType(%q) returned nil — child type not registered", shortName)
	}
	return *td
}

func TestConsoleURL_RegisteredForEveryChildType(t *testing.T) {
	types := resource.AllChildTypes()
	if len(types) == 0 {
		t.Fatal("resource.AllChildTypes() returned no child types — registry not populated")
	}
	for _, td := range types {
		if td.ConsoleURL == nil {
			t.Errorf("%s: ConsoleURL is nil; every child type must define one", td.ShortName)
		}
	}
}

type childConsoleURLCase struct {
	name      string
	shortName string
	id        string
	fields    map[string]string
	accountID string // passed to ConsoleURL's third arg; "" unless the case needs it
	want      string
}

func TestConsoleURL_ChildPerType(t *testing.T) {
	r := demo.DemoRegion // "us-east-1"

	cases := []childConsoleURLCase{
		// The CW-logs-shaped families share cloudWatchLogStreamConsoleURL under
		// different Fields keys (stream_name, log_stream, log_stream_name); a "/" in
		// group or stream must encode to %2F, or the console's path-segment parsing
		// breaks.
		{
			name: "log_streams", shortName: "log_streams",
			fields: map[string]string{"log_group": "/aws/lambda/acme-api", "stream_name": "2026/07/20/[$LATEST]abc123def456"},
			want:   "https://" + r + ".console.aws.amazon.com/cloudwatch/home?region=" + r + "#logsV2:log-groups/log-group/%2Faws%2Flambda%2Facme-api/log-events/2026%2F07%2F20%2F%5B$LATEST%5Dabc123def456",
		},
		{
			name: "log_events", shortName: "log_events",
			fields: map[string]string{"log_group": "/ecs/acme-web-svc", "log_stream": "ecs/web/9f8e7d6c5b4a"},
			want:   "https://" + r + ".console.aws.amazon.com/cloudwatch/home?region=" + r + "#logsV2:log-groups/log-group/%2Fecs%2Facme-web-svc/log-events/ecs%2Fweb%2F9f8e7d6c5b4a",
		},
		{
			name: "lambda_invocations", shortName: "lambda_invocations",
			fields: map[string]string{"log_group": "/aws/lambda/acme-api", "log_stream": "2026/07/20/[$LATEST]abc123def456"},
			want:   "https://" + r + ".console.aws.amazon.com/cloudwatch/home?region=" + r + "#logsV2:log-groups/log-group/%2Faws%2Flambda%2Facme-api/log-events/2026%2F07%2F20%2F%5B$LATEST%5Dabc123def456",
		},
		{
			name: "lambda_invocation_logs", shortName: "lambda_invocation_logs",
			fields: map[string]string{"log_group": "/aws/lambda/acme-api", "log_stream": "2026/07/20/[$LATEST]abc123def456"},
			want:   "https://" + r + ".console.aws.amazon.com/cloudwatch/home?region=" + r + "#logsV2:log-groups/log-group/%2Faws%2Flambda%2Facme-api/log-events/2026%2F07%2F20%2F%5B$LATEST%5Dabc123def456",
		},
		{
			name: "ecs_svc_logs", shortName: "ecs_svc_logs",
			fields: map[string]string{"log_group": "/ecs/acme-web-svc", "log_stream": "ecs/web/9f8e7d6c5b4a"},
			want:   "https://" + r + ".console.aws.amazon.com/cloudwatch/home?region=" + r + "#logsV2:log-groups/log-group/%2Fecs%2Facme-web-svc/log-events/ecs%2Fweb%2F9f8e7d6c5b4a",
		},
		{
			name: "cb_build_logs", shortName: "cb_build_logs",
			fields: map[string]string{"log_group_name": "/aws/codebuild/acme-api-build", "log_stream_name": "abc123-def456"},
			want:   "https://" + r + ".console.aws.amazon.com/cloudwatch/home?region=" + r + "#logsV2:log-groups/log-group/%2Faws%2Fcodebuild%2Facme-api-build/log-events/abc123-def456",
		},

		{
			name: "asg_activities", shortName: "asg_activities",
			fields: map[string]string{"asg_name": "acme-web-prod-asg"},
			want:   "https://" + r + ".console.aws.amazon.com/ec2/home?region=" + r + "#AutoScalingGroupDetails:id=acme-web-prod-asg;view=activity",
		},

		{
			name: "ecr_images", shortName: "ecr_images",
			fields: map[string]string{"repository_name": "acme/web-app"},
			want:   "https://" + r + ".console.aws.amazon.com/ecr/repositories/acme%2Fweb-app/?region=" + r,
		},

		{
			name: "ecs_tasks", shortName: "ecs_tasks",
			fields: map[string]string{"task_arn": "arn:aws:ecs:us-east-1:123456789012:task/acme-cluster/1a2b3c4d5e6f7g8h"},
			want:   "https://" + r + ".console.aws.amazon.com/ecs/v2/redirect?arn=arn%3Aaws%3Aecs%3Aus-east-1%3A123456789012%3Atask%2Facme-cluster%2F1a2b3c4d5e6f7g8h&region=" + r,
		},

		{
			name: "elb_listeners", shortName: "elb_listeners",
			id:   "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/acme-web-alb/1234567890abcdef/abcdef1234567890",
			want: "https://" + r + ".console.aws.amazon.com/ec2/home?region=" + r + "#ELBListenerV2:listenerArn=arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/acme-web-alb/1234567890abcdef/abcdef1234567890",
		},
		{
			name: "elb_listener_rules", shortName: "elb_listener_rules",
			id:   "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener-rule/app/acme-web-alb/1234567890abcdef/abcdef1234567890/1111222233334444",
			want: "https://" + r + ".console.aws.amazon.com/ec2/home?region=" + r + "#ListenerRuleDetails:ruleArn=arn:aws:elasticloadbalancing:us-east-1:123456789012:listener-rule/app/acme-web-alb/1234567890abcdef/abcdef1234567890/1111222233334444",
		},

		{
			name: "tg_health", shortName: "tg_health",
			fields: map[string]string{"target_group_arn": "arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-web-tg/abcdef1234567890"},
			want:   "https://" + r + ".console.aws.amazon.com/ec2/home?region=" + r + "#TargetGroup:targetGroupArn=arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-web-tg/abcdef1234567890",
		},

		{
			name: "transfer_agreements", shortName: "transfer_agreements",
			fields: map[string]string{"server_id": "s-0a1b2c3d4e5f60001"},
			want:   "https://" + r + ".console.aws.amazon.com/transfer/home#/servers/s-0a1b2c3d4e5f60001",
		},

		{
			name: "dbi_events", shortName: "dbi_events",
			fields: map[string]string{"source_identifier": "acme-prod-db"},
			want:   "https://" + r + ".console.aws.amazon.com/rds/home?region=" + r + "#database:id=acme-prod-db;is-cluster=false",
		},

		{
			name: "alarm_history", shortName: "alarm_history",
			fields: map[string]string{"alarm_name": "acme-cpu-high"},
			want:   "https://" + r + ".console.aws.amazon.com/cloudwatch/home?region=" + r + "#alarmsV2:alarm/acme-cpu-high",
		},

		{
			name: "eb_rule_targets (named bus)", shortName: "eb_rule_targets",
			fields: map[string]string{"rule_name": "acme-order-created-rule", "event_bus": "acme-orders-bus"},
			want:   "https://" + r + ".console.aws.amazon.com/events/home?region=" + r + "#/eventbus/acme-orders-bus/rules/acme-order-created-rule",
		},
		{
			name: "eb_rule_targets (default bus)", shortName: "eb_rule_targets",
			fields: map[string]string{"rule_name": "acme-order-created-rule"}, // event_bus absent → defaults to "default"
			want:   "https://" + r + ".console.aws.amazon.com/events/home?region=" + r + "#/eventbus/default/rules/acme-order-created-rule",
		},

		{
			name: "sfn_executions", shortName: "sfn_executions",
			fields: map[string]string{"execution_arn": "arn:aws:states:us-east-1:123456789012:execution:acme-order-workflow:9f8e7d6c-1234-5678-90ab-cdef12345678"},
			want:   "https://" + r + ".console.aws.amazon.com/states/home?region=" + r + "#/executions/details/arn:aws:states:us-east-1:123456789012:execution:acme-order-workflow:9f8e7d6c-1234-5678-90ab-cdef12345678",
		},
		{
			name: "sfn_execution_history", shortName: "sfn_execution_history",
			fields: map[string]string{"execution_arn": "arn:aws:states:us-east-1:123456789012:execution:acme-order-workflow:9f8e7d6c-1234-5678-90ab-cdef12345678"},
			want:   "https://" + r + ".console.aws.amazon.com/states/home?region=" + r + "#/executions/details/arn:aws:states:us-east-1:123456789012:execution:acme-order-workflow:9f8e7d6c-1234-5678-90ab-cdef12345678",
		},

		{
			name: "sns_subscriptions", shortName: "sns_subscriptions",
			fields: map[string]string{"subscription_arn": "arn:aws:sns:us-east-1:123456789012:acme-orders-topic:1a2b3c4d-5e6f-7g8h-9i0j-k1l2m3n4o5p6"},
			want:   "https://" + r + ".console.aws.amazon.com/sns/v3/home?region=" + r + "#/subscription/arn:aws:sns:us-east-1:123456789012:acme-orders-topic:1a2b3c4d-5e6f-7g8h-9i0j-k1l2m3n4o5p6",
		},

		{
			name: "iam_group_members", shortName: "iam_group_members",
			fields: map[string]string{"user_name": "jane.doe"},
			want:   "https://console.aws.amazon.com/iam/home#/users/details/jane.doe",
		},

		{
			name: "role_policies (policy_arn)", shortName: "role_policies",
			fields: map[string]string{"policy_arn": "arn:aws:iam::123456789012:policy/acme-readonly-policy", "role_name": "acme-ec2-instance-role"},
			want:   "https://console.aws.amazon.com/iam/home#/policies/details/arn%3Aaws%3Aiam%3A%3A123456789012%3Apolicy%2Facme-readonly-policy",
		},
		{
			name: "role_policies (role_name fallback)", shortName: "role_policies",
			fields: map[string]string{"role_name": "acme-ec2-instance-role"}, // policy_arn absent (inline policy)
			want:   "https://console.aws.amazon.com/iam/home#/roles/details/acme-ec2-instance-role",
		},

		{
			name: "r53_records", shortName: "r53_records",
			fields: map[string]string{"zone_id": "/hostedzone/Z1D633PJN98FT9"},
			want:   "https://console.aws.amazon.com/route53/v2/hostedzones#ListRecordSets/Z1D633PJN98FT9",
		},

		{
			name: "cb_builds (account from build_arn)", shortName: "cb_builds",
			fields: map[string]string{
				"build_id":  "acme-api-build:abc123de-4567-8901-fabc-def012345678",
				"build_arn": "arn:aws:codebuild:us-east-1:123456789012:build/acme-api-build:abc123de-4567-8901-fabc-def012345678",
			},
			accountID: "999988887777", // must be ignored — build_arn's account wins
			want:      "https://" + r + ".console.aws.amazon.com/codesuite/codebuild/123456789012/projects/acme-api-build/build/acme-api-build:abc123de-4567-8901-fabc-def012345678",
		},
		{
			name: "cb_builds (account fallback to passed accountID)", shortName: "cb_builds",
			fields:    map[string]string{"build_id": "acme-api-build:abc123de-4567-8901-fabc-def012345678"},
			accountID: "999988887777",
			want:      "https://" + r + ".console.aws.amazon.com/codesuite/codebuild/999988887777/projects/acme-api-build/build/acme-api-build:abc123de-4567-8901-fabc-def012345678",
		},

		{
			name: "pipeline_stages", shortName: "pipeline_stages",
			fields: map[string]string{"pipeline_name": "acme-web-deploy-pipeline"},
			want:   "https://" + r + ".console.aws.amazon.com/codesuite/codepipeline/pipelines/acme-web-deploy-pipeline/view?region=" + r,
		},

		{
			name: "glue_runs", shortName: "glue_runs",
			fields: map[string]string{"job_name": "acme-etl-job"},
			want:   "https://" + r + ".console.aws.amazon.com/gluestudio/home?region=" + r + "#/editor/job/acme-etl-job",
		},

		{
			name: "s3_objects", shortName: "s3_objects",
			fields: map[string]string{"bucket": "acme-prod-data-lake", "key": "raw/2026/07/20/events.parquet"},
			want:   "https://console.aws.amazon.com/s3/object/acme-prod-data-lake?prefix=raw%2F2026%2F07%2F20%2Fevents.parquet",
		},

		{
			name: "cfn_events", shortName: "cfn_events",
			fields: map[string]string{"stack_arn": "arn:aws:cloudformation:us-east-1:123456789012:stack/acme-web-stack/1a2b3c4d-5678-90ab-cdef-111122223333"},
			want:   "https://" + r + ".console.aws.amazon.com/cloudformation/home?region=" + r + "#/stacks/events?stackId=arn:aws:cloudformation:us-east-1:123456789012:stack/acme-web-stack/1a2b3c4d-5678-90ab-cdef-111122223333",
		},
		{
			name: "cfn_resources", shortName: "cfn_resources",
			fields: map[string]string{"stack_arn": "arn:aws:cloudformation:us-east-1:123456789012:stack/acme-web-stack/1a2b3c4d-5678-90ab-cdef-111122223333"},
			want:   "https://" + r + ".console.aws.amazon.com/cloudformation/home?region=" + r + "#/stacks/resources?stackId=arn:aws:cloudformation:us-east-1:123456789012:stack/acme-web-stack/1a2b3c4d-5678-90ab-cdef-111122223333",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			td := childConsoleURLTypeDef(t, c.shortName)
			if td.ConsoleURL == nil {
				t.Fatalf("%s: ConsoleURL is nil", c.shortName)
			}
			row := domain.Resource{ID: c.id, Type: c.shortName, Fields: c.fields}
			got := td.ConsoleURL(row, r, c.accountID)
			if got != c.want {
				t.Errorf("%s: ConsoleURL(%+v) =\n  %q\nwant\n  %q", c.shortName, row, got, c.want)
			}
		})
	}
}

// An incomplete row, missing the parent-context Field its builder needs,
// returns "", never a malformed or partial URL.

func TestConsoleURL_Child_IncompleteRow_ReturnsEmpty(t *testing.T) {
	r := demo.DemoRegion

	cases := []childConsoleURLCase{
		{name: "log_streams: missing log_group", shortName: "log_streams", fields: map[string]string{"stream_name": "s1"}},
		{name: "log_streams: missing stream_name", shortName: "log_streams", fields: map[string]string{"log_group": "/aws/lambda/x"}},
		{name: "log_events: missing log_group", shortName: "log_events", fields: map[string]string{"log_stream": "s1"}},
		{name: "lambda_invocations: missing log_stream", shortName: "lambda_invocations", fields: map[string]string{"log_group": "/aws/lambda/x"}},
		{name: "lambda_invocation_logs: missing log_group", shortName: "lambda_invocation_logs", fields: map[string]string{"log_stream": "s1"}},
		{name: "ecs_svc_logs: missing log_stream", shortName: "ecs_svc_logs", fields: map[string]string{"log_group": "/ecs/x"}},
		{name: "cb_build_logs: missing log_group_name", shortName: "cb_build_logs", fields: map[string]string{"log_stream_name": "s1"}},
		{name: "asg_activities: missing asg_name", shortName: "asg_activities", fields: map[string]string{}},
		{name: "ecr_images: missing repository_name", shortName: "ecr_images", fields: map[string]string{}},
		{name: "ecs_tasks: missing task_arn", shortName: "ecs_tasks", fields: map[string]string{}},
		{name: "ecs_svc_events: missing service_arn", shortName: "ecs_svc_events", fields: map[string]string{}},
		{name: "elb_listeners: missing ID (empty ARN)", shortName: "elb_listeners", id: ""},
		{name: "elb_listener_rules: missing ID (empty ARN)", shortName: "elb_listener_rules", id: ""},
		{name: "tg_health: missing target_group_arn", shortName: "tg_health", fields: map[string]string{}},
		{name: "transfer_agreements: missing server_id", shortName: "transfer_agreements", fields: map[string]string{}},
		{name: "dbi_events: missing source_identifier", shortName: "dbi_events", fields: map[string]string{}},
		{name: "alarm_history: missing alarm_name", shortName: "alarm_history", fields: map[string]string{}},
		{name: "eb_rule_targets: missing rule_name", shortName: "eb_rule_targets", fields: map[string]string{"event_bus": "acme-orders-bus"}},
		{name: "sfn_executions: missing execution_arn", shortName: "sfn_executions", fields: map[string]string{}},
		{name: "sfn_execution_history: missing execution_arn", shortName: "sfn_execution_history", fields: map[string]string{}},
		{name: "sns_subscriptions: missing subscription_arn", shortName: "sns_subscriptions", fields: map[string]string{}},
		{name: "iam_group_members: missing user_name", shortName: "iam_group_members", fields: map[string]string{}},
		{name: "role_policies: missing both policy_arn and role_name", shortName: "role_policies", fields: map[string]string{}},
		{name: "r53_records: missing zone_id", shortName: "r53_records", fields: map[string]string{}},
		{name: "cb_builds: missing build_id", shortName: "cb_builds", fields: map[string]string{}},
		{name: "pipeline_stages: missing pipeline_name", shortName: "pipeline_stages", fields: map[string]string{}},
		{name: "glue_runs: missing job_name", shortName: "glue_runs", fields: map[string]string{}},
		{name: "s3_objects: missing key (bucket present)", shortName: "s3_objects", fields: map[string]string{"bucket": "acme-prod-data-lake"}},
		{name: "s3_objects: missing bucket (key present)", shortName: "s3_objects", fields: map[string]string{"key": "raw/2026/07/20/events.parquet"}},
		{name: "cfn_events: missing stack_arn", shortName: "cfn_events", fields: map[string]string{}},
		{name: "cfn_resources: missing stack_arn", shortName: "cfn_resources", fields: map[string]string{}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			td := childConsoleURLTypeDef(t, c.shortName)
			if td.ConsoleURL == nil {
				t.Fatalf("%s: ConsoleURL is nil", c.shortName)
			}
			row := domain.Resource{ID: c.id, Type: c.shortName, Fields: c.fields}
			got := td.ConsoleURL(row, r, "")
			if got != "" {
				t.Errorf("%s: ConsoleURL on an incomplete row = %q, want \"\" (never a malformed URL)", c.name, got)
			}
		})
	}
}

// TestConsoleURL_CbBuilds_MalformedBuildIDNoColon_ReturnsEmpty pins the
// build_id parse guard directly: build_id is expected as "project:uuid"
// (strings.Cut on ":"); a value with no colon at all must not silently
// resolve to a URL with an empty/garbage build path.
func TestConsoleURL_CbBuilds_MalformedBuildIDNoColon_ReturnsEmpty(t *testing.T) {
	td := childConsoleURLTypeDef(t, "cb_builds")
	row := domain.Resource{Fields: map[string]string{"build_id": "no-colon-here"}}
	if got := td.ConsoleURL(row, demo.DemoRegion, ""); got != "" {
		t.Errorf("cb_builds with a colon-less build_id: ConsoleURL = %q, want \"\"", got)
	}
}
