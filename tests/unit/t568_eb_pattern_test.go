package unit_test

// An EventBridge rule relates to a resource when its event pattern can match
// an event about that resource, read the way EventBridge reads a pattern:
//
//   - "Event patterns have the same structure as the events they match. An
//     event pattern either matches an event or it doesn't."
//     https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-event-patterns.html
//   - Values in a list match any one of them. The content filters are
//     "prefix", "suffix", "anything-but" (with a list, "prefix", "suffix",
//     "wildcard" or "equals-ignore-case"), "equals-ignore-case" (alone or
//     inside "prefix"/"suffix"), "exists", "numeric", "cidr", "wildcard",
//     "" for an empty value, null, and "$or" across fields.
//     https://docs.aws.amazon.com/eventbridge/latest/userguide/eb-create-pattern-operators.html
//
// The fields that name the resource are the ones each service writes:
//   - ECR: detail."repository-name", and resources [repository ARN] on scan,
//     pull-through-cache and replication events.
//     https://docs.aws.amazon.com/AmazonECR/latest/userguide/ecr-eventbridge.html
//   - S3: detail.bucket.name, and resources ["arn:aws:s3:::<bucket>"].
//     https://docs.aws.amazon.com/AmazonS3/latest/userguide/ev-events.html
//   - ECS: resources [service ARN] and detail.clusterArn on "ECS Service
//     Action" events; detail.group "service:<name>" on task state changes.
//     https://docs.aws.amazon.com/AmazonECS/latest/developerguide/ecs_service_events.html
//
// A field the pattern constrains that says nothing about which resource the
// event is about (detail-type, action-type, an object key) leaves the rule
// matching some of the resource's events. A pattern the evaluator cannot
// read — an operator AWS does not document, or a filter of the wrong shape —
// answers unknown for that rule. A pattern that constrains no field naming a
// resource — the source alone, or with a detail-type — matches every resource
// of the type alike and is no count on any one of them.

import (
	"context"
	"slices"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	ecrtypes "github.com/aws/aws-sdk-go-v2/service/ecr/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// t568Rule is one rule on the default bus with the targets it delivers to.
type t568Rule struct {
	name, pattern, schedule string
	targets                 []eventbridgetypes.Target
}

// t568EB answers the EventBridge reads as the service does, for the rules
// of one account's default bus.
type t568EB struct {
	awsclient.EventBridgeAPI
	rules []t568Rule
}

func (f *t568EB) ListEventBuses(_ context.Context, _ *eventbridge.ListEventBusesInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListEventBusesOutput, error) {
	return &eventbridge.ListEventBusesOutput{EventBuses: []eventbridgetypes.EventBus{{
		Name: aws.String("default"), Arn: aws.String("arn:aws:events:us-east-1:123456789012:event-bus/default"),
	}}}, nil
}

func (f *t568EB) ListRules(_ context.Context, _ *eventbridge.ListRulesInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListRulesOutput, error) {
	out := &eventbridge.ListRulesOutput{Rules: []eventbridgetypes.Rule{}}
	for _, r := range f.rules {
		rule := eventbridgetypes.Rule{
			Name:         aws.String(r.name),
			Arn:          aws.String("arn:aws:events:us-east-1:123456789012:rule/" + r.name),
			EventBusName: aws.String("default"),
			State:        eventbridgetypes.RuleStateEnabled,
		}
		if r.pattern != "" {
			rule.EventPattern = aws.String(r.pattern)
		}
		if r.schedule != "" {
			rule.ScheduleExpression = aws.String(r.schedule)
		}
		out.Rules = append(out.Rules, rule)
	}
	return out, nil
}

func (f *t568EB) ListTargetsByRule(_ context.Context, in *eventbridge.ListTargetsByRuleInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListTargetsByRuleOutput, error) {
	for _, r := range f.rules {
		if r.name == aws.ToString(in.Rule) {
			return &eventbridge.ListTargetsByRuleOutput{Targets: r.targets}, nil
		}
	}
	return nil, &eventbridgetypes.ResourceNotFoundException{Message: aws.String("Rule " + aws.ToString(in.Rule) + " does not exist on EventBus default.")}
}

func (f *t568EB) ListRuleNamesByTarget(_ context.Context, in *eventbridge.ListRuleNamesByTargetInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListRuleNamesByTargetOutput, error) {
	out := &eventbridge.ListRuleNamesByTargetOutput{RuleNames: []string{}}
	for _, r := range f.rules {
		if slices.ContainsFunc(r.targets, func(t eventbridgetypes.Target) bool { return aws.ToString(t.Arn) == aws.ToString(in.TargetArn) }) {
			out.RuleNames = append(out.RuleNames, r.name)
		}
	}
	return out, nil
}

// t568RuleWorld is the rules as their list fetcher emits them, the loaded
// eb-rule list, and clients that answer for the same rules.
func t568RuleWorld(t *testing.T, rules ...t568Rule) (resource.ResourceCache, *awsclient.ServiceClients, map[string]string) {
	t.Helper()
	eb := &t568EB{rules: rules}
	page, err := awsclient.FetchEventBridgeRulesPage(context.Background(), eb, "")
	if err != nil {
		t.Fatalf("listing the rules: %v", err)
	}
	ids := make(map[string]string, len(page.Resources))
	for _, r := range page.Resources {
		ids[r.Name] = r.ID
	}
	return resource.ResourceCache{"eb-rule": {Resources: page.Resources}}, &awsclient.ServiceClients{EventBridge: eb, Region: "us-east-1"}, ids
}

type t568PatternCase struct {
	name, pattern string
	want          string // "match", "no", "unknown", "not-a-count" or "not-a-match"
}

// t568RequireRuleAnswer holds r, built over one rule, to what the case says
// the pattern means for the resource.
func t568RequireRuleAnswer(t *testing.T, pivot string, tc t568PatternCase, r resource.RelatedCheckResult, ruleID string) {
	t.Helper()
	exact := r.EffectiveState() == domain.RelatedResolved && r.Coverage() == domain.CoverageComplete
	counted := slices.Contains(r.ResourceIDs(), ruleID)
	switch tc.want {
	case "match":
		if !exact || !slices.Equal(sortedIDs(r), []string{ruleID}) {
			t.Errorf("%s for %s = %v (state %v, coverage %v), want exactly [%s]", pivot, tc.pattern, r.ResourceIDs(), r.EffectiveState(), r.Coverage(), ruleID)
		}
	case "no":
		if !exact || len(r.ResourceIDs()) != 0 {
			t.Errorf("%s for %s = %v (state %v, coverage %v), want a proven 0: the pattern cannot match this resource's events", pivot, tc.pattern, r.ResourceIDs(), r.EffectiveState(), r.Coverage())
		}
	case "unknown":
		if counted || exact {
			t.Errorf("%s for %s = %v (state %v, coverage %v), want unknown for that rule: neither a match nor a proven non-match", pivot, tc.pattern, r.ResourceIDs(), r.EffectiveState(), r.Coverage())
		}
	case "not-a-count":
		if exact {
			t.Errorf("%s for %s = %v (state %v, coverage %v): a pattern naming no resource of the type matches every one alike, so it is neither a count on this one nor a proven 0", pivot, tc.pattern, r.ResourceIDs(), r.EffectiveState(), r.Coverage())
		}
	case "not-a-match":
		if exact && counted {
			t.Errorf("%s for %s counts %s: the pattern constrains a field the service's events do not carry", pivot, tc.pattern, ruleID)
		}
	}
	if r.Err() != nil {
		t.Errorf("%s for %s: error %v", pivot, tc.pattern, r.Err())
	}
}

func TestEBRulePattern_ECRRepository(t *testing.T) {
	const repoARN = "arn:aws:ecr:us-east-1:123456789012:repository/acme-api"
	repo := resource.Resource{ID: "acme-api", Name: "acme-api", Type: "ecr", RawStruct: ecrtypes.Repository{
		RepositoryName: aws.String("acme-api"), RepositoryArn: aws.String(repoARN),
		RegistryId: aws.String("123456789012"), RepositoryUri: aws.String("123456789012.dkr.ecr.us-east-1.amazonaws.com/acme-api"),
	}}
	cases := []t568PatternCase{
		{"literal list naming the repository", `{"source":["aws.ecr"],"detail-type":["ECR Image Action"],"detail":{"repository-name":["acme-web","acme-api"],"action-type":["PUSH"],"result":["SUCCESS"]}}`, "match"},
		{"literal naming another repository", `{"source":["aws.ecr"],"detail":{"repository-name":["acme-api-worker"]}}`, "no"},
		{"prefix the name starts with", `{"source":["aws.ecr"],"detail":{"repository-name":[{"prefix":"acme-"}]}}`, "match"},
		{"prefix the name does not start with", `{"source":["aws.ecr"],"detail":{"repository-name":[{"prefix":"prod-"}]}}`, "no"},
		{"anything-but another name", `{"source":["aws.ecr"],"detail":{"repository-name":[{"anything-but":["acme-web"]}]}}`, "match"},
		{"anything-but this name", `{"source":["aws.ecr"],"detail":{"repository-name":[{"anything-but":"acme-api"}]}}`, "no"},
		{"anything-but a prefix of the name", `{"source":["aws.ecr"],"detail":{"repository-name":[{"anything-but":{"prefix":"acme-"}}]}}`, "no"},
		{"exists on the repository name", `{"source":["aws.ecr"],"detail":{"repository-name":[{"exists":true}]}}`, "match"},
		{"resources naming the repository ARN", `{"source":["aws.ecr"],"detail-type":["ECR Image Scan"],"resources":["` + repoARN + `"]}`, "match"},
		{"resources naming another repository ARN", `{"source":["aws.ecr"],"resources":["arn:aws:ecr:us-east-1:123456789012:repository/acme-api-worker"]}`, "no"},
		{"numeric filter on a scan count beside the name", `{"source":["aws.ecr"],"detail-type":["ECR Image Scan"],"detail":{"repository-name":["acme-api"],"finding-severity-counts":{"CRITICAL":[{"numeric":[">",0]}]}}}`, "match"},
		{"another service's source", `{"source":["aws.ecs"],"detail":{"repository-name":["acme-api"]}}`, "no"},
		{"source alone", `{"source":["aws.ecr"]}`, "not-a-count"},
		{"source and detail-type alone", `{"source":["aws.ecr"],"detail-type":["ECR Image Scan"]}`, "not-a-count"},
		{"a key ECR events do not carry", `{"source":["aws.ecr"],"detail":{"repositoryName":["acme-api"]}}`, "not-a-match"},
		{"suffix the name ends with", `{"source":["aws.ecr"],"detail":{"repository-name":[{"suffix":"-api"}]}}`, "match"},
		{"suffix the name does not end with", `{"source":["aws.ecr"],"detail":{"repository-name":[{"suffix":"-worker"}]}}`, "no"},
		{"equals-ignore-case", `{"source":["aws.ecr"],"detail":{"repository-name":[{"equals-ignore-case":"ACME-API"}]}}`, "match"},
		{"equals-ignore-case another name", `{"source":["aws.ecr"],"detail":{"repository-name":[{"equals-ignore-case":"ACME-API-WORKER"}]}}`, "no"},
		{"prefix ignoring case", `{"source":["aws.ecr"],"detail":{"repository-name":[{"prefix":{"equals-ignore-case":"ACME-"}}]}}`, "match"},
		{"suffix ignoring case", `{"source":["aws.ecr"],"detail":{"repository-name":[{"suffix":{"equals-ignore-case":"-API"}}]}}`, "match"},
		{"wildcard the name fits", `{"source":["aws.ecr"],"detail":{"repository-name":[{"wildcard":"acme-*"}]}}`, "match"},
		{"wildcard the name does not fit", `{"source":["aws.ecr"],"detail":{"repository-name":[{"wildcard":"acme-*-worker"}]}}`, "no"},
		{"anything-but a suffix of the name", `{"source":["aws.ecr"],"detail":{"repository-name":[{"anything-but":{"suffix":"-api"}}]}}`, "no"},
		{"anything-but ignoring case", `{"source":["aws.ecr"],"detail":{"repository-name":[{"anything-but":{"equals-ignore-case":["ACME-API"]}}]}}`, "no"},
		{"anything-but a wildcard the name fits", `{"source":["aws.ecr"],"detail":{"repository-name":[{"anything-but":{"wildcard":"acme-*"}}]}}`, "no"},
		{"anything-but a wildcard the name does not fit", `{"source":["aws.ecr"],"detail":{"repository-name":[{"anything-but":{"wildcard":"prod-*"}}]}}`, "match"},
		{"empty name", `{"source":["aws.ecr"],"detail":{"repository-name":[""]}}`, "no"},
		{"null name", `{"source":["aws.ecr"],"detail":{"repository-name":[null]}}`, "no"},
		{"$or with a branch naming the repository", `{"source":["aws.ecr"],"detail":{"$or":[{"repository-name":["acme-web"]},{"repository-name":[{"prefix":"acme-a"}]}]}}`, "match"},
		{"$or with no branch naming the repository", `{"source":["aws.ecr"],"detail":{"$or":[{"repository-name":["acme-web"]},{"repository-name":[{"suffix":"-worker"}]}]}}`, "no"},
		{"an operator AWS does not document", `{"source":["aws.ecr"],"detail":{"repository-name":[{"regex":"^acme-"}]}}`, "unknown"},
		{"a prefix that is not a string", `{"source":["aws.ecr"],"detail":{"repository-name":[{"prefix":123}]}}`, "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cache, clients, ids := t568RuleWorld(t, t568Rule{name: "acme-ecr-rule", pattern: tc.pattern})
			got := refChecker(t, "ecr", "eb-rule")(context.Background(), clients, repo, cache)
			t568RequireRuleAnswer(t, "ecr → eb-rule", tc, got, ids["acme-ecr-rule"])
		})
	}
}

func TestEBRulePattern_S3Bucket(t *testing.T) {
	bucket := resource.Resource{ID: "acme-assets", Name: "acme-assets", Type: "s3", Fields: map[string]string{"name": "acme-assets", "region": "us-east-1"}}
	cases := []t568PatternCase{
		{"bucket name literal", `{"source":["aws.s3"],"detail-type":["Object Created"],"detail":{"bucket":{"name":["acme-assets"]}}}`, "match"},
		{"another bucket's name", `{"source":["aws.s3"],"detail":{"bucket":{"name":["acme-assets-logs"]}}}`, "no"},
		{"another bucket, this name as an object key prefix", `{"source":["aws.s3"],"detail":{"bucket":{"name":["acme-web"]},"object":{"key":[{"prefix":"acme-assets"}]}}}`, "no"},
		{"prefix the bucket name starts with", `{"source":["aws.s3"],"detail":{"bucket":{"name":[{"prefix":"acme-"}]}}}`, "match"},
		{"object key prefix beside the bucket name", `{"source":["aws.s3"],"detail":{"bucket":{"name":["acme-assets"]},"object":{"key":[{"prefix":"uploads/"}]}}}`, "match"},
		{"resources naming the bucket ARN", `{"source":["aws.s3"],"resources":["arn:aws:s3:::acme-assets"]}`, "match"},
		{"anything-but this bucket", `{"source":["aws.s3"],"detail":{"bucket":{"name":[{"anything-but":["acme-assets"]}]}}}`, "no"},
		{"source alone", `{"source":["aws.s3"],"detail-type":["Object Created"]}`, "not-a-count"},
		{"wildcard the bucket name fits", `{"source":["aws.s3"],"detail":{"bucket":{"name":[{"wildcard":"acme-*"}]}}}`, "match"},
		{"cidr on the requester's address beside the bucket name", `{"source":["aws.s3"],"detail":{"bucket":{"name":["acme-assets"]},"source-ip-address":[{"cidr":"192.0.2.0/24"}]}}`, "match"},
		{"cidr on the bucket name", `{"source":["aws.s3"],"detail":{"bucket":{"name":[{"cidr":"10.0.0.0/8"}]}}}`, "no"},
		{"$or naming the bucket in one branch", `{"source":["aws.s3"],"$or":[{"detail":{"bucket":{"name":["acme-assets"]}}},{"resources":["arn:aws:s3:::acme-web"]}]}`, "match"},
		{"an operator AWS does not document", `{"source":["aws.s3"],"detail":{"bucket":{"name":[{"contains":"assets"}]}}}`, "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cache, clients, ids := t568RuleWorld(t, t568Rule{name: "acme-s3-rule", pattern: tc.pattern})
			got := refChecker(t, "s3", "eb-rule")(context.Background(), clients, bucket, cache)
			t568RequireRuleAnswer(t, "s3 → eb-rule", tc, got, ids["acme-s3-rule"])
		})
	}
}

const (
	t568Cluster     = "arn:aws:ecs:us-east-1:123456789012:cluster/prod"
	t568ServiceARN  = "arn:aws:ecs:us-east-1:123456789012:service/prod/api"
	t568APITaskDef  = "arn:aws:ecs:us-east-1:123456789012:task-definition/api:42"
	t568WorkTaskDef = "arn:aws:ecs:us-east-1:123456789012:task-definition/api-worker:7"
)

// t568ECSService is service api in cluster prod as the ecs-svc fetcher
// emits it; api-worker is another service in the same cluster.
func t568ECSService() resource.Resource {
	return resource.Resource{
		ID: "prod/api", Name: "api", Type: "ecs-svc",
		Fields: map[string]string{"service_name": "api", "cluster": "prod", "task_definition": t568APITaskDef},
		RawStruct: ecstypes.Service{
			ServiceName: aws.String("api"), ServiceArn: aws.String(t568ServiceARN), ClusterArn: aws.String(t568Cluster),
			TaskDefinition: aws.String(t568APITaskDef), Status: aws.String("ACTIVE"), DesiredCount: 2, RunningCount: 2,
		},
	}
}

func TestEBRulePattern_ECSService(t *testing.T) {
	svc := t568ECSService()
	cases := []t568PatternCase{
		{"task group naming the service", `{"source":["aws.ecs"],"detail-type":["ECS Task State Change"],"detail":{"group":["service:api"],"lastStatus":["STOPPED"]}}`, "match"},
		{"task group naming another service", `{"source":["aws.ecs"],"detail-type":["ECS Task State Change"],"detail":{"group":["service:api-worker"]}}`, "no"},
		{"group prefix the service's group starts with", `{"source":["aws.ecs"],"detail":{"group":[{"prefix":"service:api"}]}}`, "match"},
		{"cluster and group naming the service", `{"source":["aws.ecs"],"detail":{"clusterArn":["` + t568Cluster + `"],"group":["service:api"]}}`, "match"},
		{"another cluster", `{"source":["aws.ecs"],"detail":{"clusterArn":["arn:aws:ecs:us-east-1:123456789012:cluster/staging"],"group":["service:api"]}}`, "no"},
		{"resources naming the service ARN", `{"source":["aws.ecs"],"detail-type":["ECS Service Action"],"resources":["` + t568ServiceARN + `"]}`, "match"},
		{"resources naming another service ARN", `{"source":["aws.ecs"],"detail-type":["ECS Service Action"],"resources":["arn:aws:ecs:us-east-1:123456789012:service/prod/api-worker"]}`, "no"},
		{"a key ECS events do not carry", `{"source":["aws.ecs"],"detail":{"serviceName":["api"]}}`, "not-a-match"},
		{"source alone", `{"source":["aws.ecs"]}`, "not-a-count"},
		{"the service's cluster alone", `{"source":["aws.ecs"],"detail":{"clusterArn":["` + t568Cluster + `"]}}`, "match"},
		{"wildcard the service's group fits", `{"source":["aws.ecs"],"detail":{"group":[{"wildcard":"service:a*i"}]}}`, "match"},
		{"suffix of the service ARN", `{"source":["aws.ecs"],"resources":[{"suffix":"service/prod/api"}]}`, "match"},
		{"an operator AWS does not document", `{"source":["aws.ecs"],"detail":{"group":[{"startsWith":"service:"}]}}`, "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cache, clients, ids := t568RuleWorld(t, t568Rule{name: "acme-ecs-rule", pattern: tc.pattern})
			got := refChecker(t, "ecs-svc", "eb-rule")(context.Background(), clients, svc, cache)
			t568RequireRuleAnswer(t, "ecs-svc → eb-rule", tc, got, ids["acme-ecs-rule"])
		})
	}
}

// A scheduled task is a rule with no pattern whose target is the cluster
// (ListRuleNamesByTarget answers by the target's ARN) running the service's
// task definition: the rule counts on the service whatever its pattern. The
// same schedule running another task definition is another workload.
func TestEBRuleTarget_ScheduleRunningTheServiceTaskDefinitionCounts(t *testing.T) {
	svc := t568ECSService()
	target := func(taskDef string) []eventbridgetypes.Target {
		return []eventbridgetypes.Target{{
			Id: aws.String("run-task"), Arn: aws.String(t568Cluster),
			RoleArn: aws.String("arn:aws:iam::123456789012:role/acme-events-ecs"),
			EcsParameters: &eventbridgetypes.EcsParameters{
				TaskDefinitionArn: aws.String(taskDef), TaskCount: aws.Int32(1), LaunchType: eventbridgetypes.LaunchTypeFargate,
			},
		}}
	}
	cache, clients, ids := t568RuleWorld(t,
		t568Rule{name: "api-nightly", schedule: "cron(0 3 * * ? *)", targets: target(t568APITaskDef)},
		t568Rule{name: "worker-nightly", schedule: "cron(0 4 * * ? *)", targets: target(t568WorkTaskDef)},
	)
	got := refChecker(t, "ecs-svc", "eb-rule")(context.Background(), clients, svc, cache)
	if !slices.Contains(got.ResourceIDs(), ids["api-nightly"]) {
		t.Errorf("ecs-svc → eb-rule = %v (state %v): the schedule that runs the service's task definition on its cluster is missing", got.ResourceIDs(), got.EffectiveState())
	}
	if slices.Contains(got.ResourceIDs(), ids["worker-nightly"]) {
		t.Errorf("ecs-svc → eb-rule counts %s, a schedule running another task definition", ids["worker-nightly"])
	}
}
