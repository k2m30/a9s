// row_identity_followups_test.go — row identity where the parent is not the
// row's own ID: a target group's registrations, a log line's twin, and the
// two ECS-service pivots that match a service by its bare name.
package unit_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwltypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	unit "github.com/k2m30/a9s/v3/tests/unit"
)

// ---------------------------------------------------------------------------
// Target health: one registration per (id, port, az)
// ---------------------------------------------------------------------------

type rifTargetHealthFake struct {
	descriptions []elbv2types.TargetHealthDescription
}

func (f *rifTargetHealthFake) DescribeTargetHealth(_ context.Context, _ *elbv2.DescribeTargetHealthInput, _ ...func(*elbv2.Options)) (*elbv2.DescribeTargetHealthOutput, error) {
	return &elbv2.DescribeTargetHealthOutput{TargetHealthDescriptions: f.descriptions}, nil
}

func rifRegistration(id string, port int32, az string, state elbv2types.TargetHealthStateEnum) elbv2types.TargetHealthDescription {
	target := &elbv2types.TargetDescription{Id: aws.String(id), Port: aws.Int32(port)}
	if az != "" {
		target.AvailabilityZone = aws.String(az)
	}
	return elbv2types.TargetHealthDescription{
		Target:       target,
		TargetHealth: &elbv2types.TargetHealth{State: state},
	}
}

// TestTargetHealth_OneInstanceOnTwoPortsIsTwoRows: ELB checks each
// registration of a target on its own, so an unhealthy port does not
// disappear behind a healthy one on the same instance.
func TestTargetHealth_OneInstanceOnTwoPortsIsTwoRows(t *testing.T) {
	const instance = "i-0a1b2c3d4e5f60009"
	fake := &rifTargetHealthFake{descriptions: []elbv2types.TargetHealthDescription{
		rifRegistration(instance, 32768, "us-east-1a", elbv2types.TargetHealthStateEnumHealthy),
		rifRegistration(instance, 32771, "us-east-1a", elbv2types.TargetHealthStateEnumUnhealthy),
	}}

	out, err := awsclient.FetchTargetHealth(context.Background(), fake,
		"arn:aws:elasticloadbalancing:us-east-1:123456789012:targetgroup/acme-api/6d0ecf831eec9f09", "")
	if err != nil {
		t.Fatalf("FetchTargetHealth: %v", err)
	}
	if len(out.Resources) != 2 {
		t.Fatalf("fetched %d rows, want 2", len(out.Resources))
	}
	if out.Resources[0].ID == out.Resources[1].ID {
		t.Errorf("both registrations share the ID %q", out.Resources[0].ID)
	}
	byPort := map[string]resource.Resource{}
	for _, r := range out.Resources {
		byPort[r.Fields["port"]] = r
		if r.Name != instance || r.Fields["target_id"] != instance {
			t.Errorf("row %q: Name %q / target_id %q, want the bare %q", r.ID, r.Name, r.Fields["target_id"], instance)
		}
	}
	if got := byPort["32771"].Fields["health"]; got != string(elbv2types.TargetHealthStateEnumUnhealthy) {
		t.Errorf("port 32771 health = %q, want unhealthy", got)
	}
	if got := byPort["32768"].Fields["health"]; got != string(elbv2types.TargetHealthStateEnumHealthy) {
		t.Errorf("port 32768 health = %q, want healthy", got)
	}
}

// TestTargetHealth_DemoHoldsBothRegistrations: the demo's prod-web group
// registers one instance twice, so the two-port case is on a screen.
func TestTargetHealth_DemoHoldsBothRegistrations(t *testing.T) {
	td := resource.GetChildType("tg_health")
	if td == nil || td.ChildFetcher == nil {
		t.Fatal("tg_health has no child fetcher")
	}
	byType, _ := buildVisibilityTypeCache(t)
	arn := ""
	for _, r := range byType["tg"] {
		if strings.Contains(r.Fields["target_group_arn"], "/acme-web-tg/") {
			arn = r.Fields["target_group_arn"]
		}
	}
	if arn == "" {
		t.Fatal("no demo acme-web-tg target group")
	}
	rows, err := unit.CollectAllPages(func(token string) (resource.FetchResult, error) {
		return td.ChildFetcher(context.Background(), demo.NewServiceClients(),
			map[string]string{"target_group_arn": arn}, token)
	})
	if err != nil {
		t.Fatalf("tg_health child fetch: %v", err)
	}
	var ports []string
	for _, r := range rows {
		if r.Fields["target_id"] == fixtures.TGInstanceOnTwoPorts {
			ports = append(ports, r.Fields["port"])
		}
	}
	slices.Sort(ports)
	if !slices.Equal(ports, []string{"32771", "443"}) {
		t.Errorf("%s is registered on ports %v, want both 443 and 32771 as their own rows",
			fixtures.TGInstanceOnTwoPorts, ports)
	}
}

// ---------------------------------------------------------------------------
// Log events: the same line twice in one millisecond
// ---------------------------------------------------------------------------

type rifLogFake struct {
	events []cwltypes.OutputLogEvent
}

func (f *rifLogFake) GetLogEvents(_ context.Context, _ *cloudwatchlogs.GetLogEventsInput, _ ...func(*cloudwatchlogs.Options)) (*cloudwatchlogs.GetLogEventsOutput, error) {
	return &cloudwatchlogs.GetLogEventsOutput{Events: f.events}, nil
}

func rifLine(ms int64, message string) cwltypes.OutputLogEvent {
	return cwltypes.OutputLogEvent{Timestamp: aws.Int64(ms), IngestionTime: aws.Int64(ms + 7), Message: aws.String(message)}
}

// TestLogEvents_TheSameLineTwiceInOneMillisecondIsTwoRows: nothing in either
// event tells the two apart, so what separates them is that one came before
// the other — and the line with no twin keeps the key it had.
func TestLogEvents_TheSameLineTwiceInOneMillisecondIsTwoRows(t *testing.T) {
	const at = int64(1774149432000)
	const repeated = "WARN retrying artifact upload (attempt 2 of 3)"

	fetchers := []struct {
		name  string
		fetch func(api awsclient.CWLogsGetLogEventsAPI) (resource.FetchResult, error)
	}{
		{"log_events", func(api awsclient.CWLogsGetLogEventsAPI) (resource.FetchResult, error) {
			return awsclient.FetchLogEvents(context.Background(), api, "/ecs/acme-api", "api/api/9f1c2b7e4d3a4f0e", "")
		}},
		{"cb_build_logs", func(api awsclient.CWLogsGetLogEventsAPI) (resource.FetchResult, error) {
			return awsclient.FetchCBBuildLogs(context.Background(), api, "/aws/codebuild/acme-api-build", "5b3c9d2e-7f41-4a8b-9c0d-1e2f3a4b5c6d", "")
		}},
	}
	for _, f := range fetchers {
		t.Run(f.name, func(t *testing.T) {
			lone := rifLine(at-1_000, "INFO uploading artifacts")
			out, err := f.fetch(&rifLogFake{events: []cwltypes.OutputLogEvent{
				lone, rifLine(at, repeated), rifLine(at, repeated),
			}})
			if err != nil {
				t.Fatalf("fetch: %v", err)
			}
			if len(out.Resources) != 3 {
				t.Fatalf("fetched %d rows, want 3", len(out.Resources))
			}
			if out.Resources[1].ID == out.Resources[2].ID {
				t.Errorf("the repeated line is one ID twice: %q", out.Resources[1].ID)
			}

			moved, err := f.fetch(&rifLogFake{events: []cwltypes.OutputLogEvent{
				rifLine(at, repeated), rifLine(at, repeated), rifLine(at+50, "INFO upload complete"),
			}})
			if err != nil {
				t.Fatalf("fetch: %v", err)
			}
			for i := range 2 {
				if moved.Resources[i].ID != out.Resources[i+1].ID {
					t.Errorf("repeated line %d has ID %q, then %q", i, out.Resources[i+1].ID, moved.Resources[i].ID)
				}
			}
		})
	}
}

// TestLogEvents_DemoBuildLogHoldsTheRepeatedLine: the demo's build log
// carries the repeated line, so the case is on a screen.
func TestLogEvents_DemoBuildLogHoldsTheRepeatedLine(t *testing.T) {
	td := resource.GetChildType("cb_build_logs")
	if td == nil || td.ChildFetcher == nil {
		t.Fatal("cb_build_logs has no child fetcher")
	}
	rows, err := unit.CollectAllPages(func(token string) (resource.FetchResult, error) {
		return td.ChildFetcher(context.Background(), demo.NewServiceClients(), map[string]string{
			"log_group_name":  "/aws/codebuild/acme-api-build",
			"log_stream_name": "acme-api-build/142",
		}, token)
	})
	if err != nil {
		t.Fatalf("cb_build_logs child fetch: %v", err)
	}
	var ids []string
	for _, r := range rows {
		if strings.Contains(r.Fields["message"], fixtures.BuildLogRepeatedLine) {
			ids = append(ids, r.ID)
		}
	}
	if len(ids) != 2 {
		t.Fatalf("the repeated build-log line is %d rows, want 2", len(ids))
	}
	if ids[0] == ids[1] {
		t.Errorf("both rows carry the ID %q", ids[0])
	}
}

// ---------------------------------------------------------------------------
// The ECS-service pivots that match by the bare service name
// ---------------------------------------------------------------------------

func rifECSSvcRow(t *testing.T, byType map[string][]resource.Resource, id string) resource.Resource {
	t.Helper()
	for _, r := range byType["ecs-svc"] {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("no demo ecs-svc row %q", id)
	return resource.Resource{}
}

// TestECSSvc_TasksAndRulesStayInTheRowsOwnCluster: a task names its service
// by the bare name, and a rule may name a service group, so the two services
// called log-aggregator would otherwise hold each other's rows.
func TestECSSvc_TasksAndRulesStayInTheRowsOwnCluster(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()
	ctx := context.Background()

	batch := rifECSSvcRow(t, byType, "acme-batch/"+fixtures.ECSServiceInTwoClusters)
	staging := rifECSSvcRow(t, byType, "acme-staging/"+fixtures.ECSServiceInTwoClusters)

	for _, pivot := range []struct{ target, cluster string }{
		{"ecs-task", "acme-batch"},
		{"eb-rule", "acme-batch"},
	} {
		def := ridRelatedDef(t, "ecs-svc", pivot.target)
		got := def.Checker(ctx, clients, staging, cache)
		if got.Count() > 0 {
			t.Errorf("acme-staging/%s -> %s: %d rows %v, want none — they belong to %s",
				fixtures.ECSServiceInTwoClusters, def.DisplayName, got.Count(), got.ResourceIDs(), pivot.cluster)
		}
	}

	tasks := ridRelatedDef(t, "ecs-svc", "ecs-task").Checker(ctx, clients, batch, cache)
	if tasks.Count() == 0 {
		t.Errorf("acme-batch/%s -> ECS Tasks: no rows, want its own tasks", fixtures.ECSServiceInTwoClusters)
	}
	rules := ridRelatedDef(t, "ecs-svc", "eb-rule").Checker(ctx, clients, batch, cache)
	if !slices.Contains(rules.ResourceIDs(), "default/"+fixtures.EBRuleForOneClustersService) {
		t.Errorf("acme-batch/%s -> EventBridge Rules: IDs %v, want the rule scoped to its cluster",
			fixtures.ECSServiceInTwoClusters, rules.ResourceIDs())
	}
}

// ---------------------------------------------------------------------------
// The CloudTrail pivot of a parent-scoped type
// ---------------------------------------------------------------------------

// TestCloudTrail_EventsOfANamesakeUnderAnotherParentAreLeftOut: LookupEvents
// filters on one attribute, and a service name is the name of a service in a
// cluster, so the events of the acme-batch log-aggregator are not the
// acme-staging one's. An event naming no cluster answers nothing and stays.
func TestCloudTrail_EventsOfANamesakeUnderAnotherParentAreLeftOut(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()

	for _, c := range []struct {
		rowID   string
		wantOwn bool
	}{
		{"acme-batch/" + fixtures.ECSServiceInTwoClusters, true},
		{"acme-staging/" + fixtures.ECSServiceInTwoClusters, false},
	} {
		row := rifECSSvcRow(t, byType, c.rowID)
		filter := resource.BuildCloudTrailFilter(row, "ecs-svc")
		if filter == nil {
			t.Fatalf("%s: no CloudTrail filter", c.rowID)
		}
		out, err := awsclient.FetchCloudTrailEventsPageFiltered(context.Background(), clients.CloudTrail, filter, "")
		if err != nil {
			t.Fatalf("%s: FetchCloudTrailEventsPageFiltered: %v", c.rowID, err)
		}
		found := false
		for _, r := range out.Resources {
			found = found || r.ID == "evt-ecs-svc-log-aggregator-batch-update-001"
		}
		if found != c.wantOwn {
			t.Errorf("%s: the acme-batch UpdateService event is shown = %v, want %v", c.rowID, found, c.wantOwn)
		}
	}
}

// ---------------------------------------------------------------------------
// A policy attachment names the policy IAM means
// ---------------------------------------------------------------------------

// TestPolicy_AWSManagedAttachmentOpensTheAWSManagedPolicy: an account may own
// a policy whose name AWS also uses — IAM keeps names unique among the
// account's own policies only — and the two differ by ARN alone, so a pivot
// that carries the attachment's ARN opens the policy the principal is
// actually attached to.
func TestPolicy_AWSManagedAttachmentOpensTheAWSManagedPolicy(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()
	ctx := context.Background()

	const awsManaged = "arn:aws:iam::aws:policy/" + fixtures.PolicyNameAlsoAWSManaged
	var ids []string
	for _, r := range byType["policy"] {
		if r.Name == fixtures.PolicyNameAlsoAWSManaged {
			ids = append(ids, r.ID)
		}
	}
	if !slices.Equal(ids, []string{fixtures.LocalPowerUserAccessARN}) {
		t.Fatalf("policy rows named %s: %v, want the account's own keyed by its ARN",
			fixtures.PolicyNameAlsoAWSManaged, ids)
	}

	var role resource.Resource
	for _, r := range byType["role"] {
		if r.ID == fixtures.RolePowerUserAttached {
			role = r
		}
	}
	if role.ID == "" {
		t.Fatalf("no demo role %s", fixtures.RolePowerUserAttached)
	}

	got := ridRelatedDef(t, "role", "policy").Checker(ctx, clients, role, cache)
	if !slices.Contains(got.ResourceIDs(), awsManaged) {
		t.Fatalf("%s -> IAM Policies: %v, want the AWS-managed %s",
			fixtures.RolePowerUserAttached, got.ResourceIDs(), awsManaged)
	}
	if slices.Contains(got.ResourceIDs(), fixtures.LocalPowerUserAccessARN) {
		t.Errorf("%s -> IAM Policies: %v includes the account's own namesake, which nothing attached to it",
			fixtures.RolePowerUserAttached, got.ResourceIDs())
	}

	td := resource.FindResourceType("policy")
	if td.FetchByIDs == nil {
		t.Fatal("policy has no FetchByIDs")
	}
	rows, err := td.FetchByIDs(ctx, clients, []string{awsManaged})
	if err != nil {
		t.Fatalf("policy FetchByIDs: %v", err)
	}
	if len(rows) != 1 || rows[0].ID != awsManaged {
		t.Fatalf("policy FetchByIDs(%s) = %v, want the AWS-managed row", awsManaged, rows)
	}
	if got := rows[0].Fields["policy_type"]; got != "aws-managed" {
		t.Errorf("%s opens a %s policy, want the AWS-managed one", awsManaged, got)
	}

	own, err := td.FetchByIDs(ctx, clients, []string{fixtures.LocalPowerUserAccessARN})
	if err != nil {
		t.Fatalf("policy FetchByIDs: %v", err)
	}
	if len(own) != 1 || own[0].ID != fixtures.LocalPowerUserAccessARN || own[0].Fields["policy_type"] != "managed" {
		t.Errorf("%s opens %v, want the account's own policy",
			fixtures.LocalPowerUserAccessARN, own)
	}
}

// TestPolicy_ARefIsReadAsThePolicyRowItNames: a policy row is keyed by its
// ARN, and an AWS-managed policy's ARN names the account "aws" while being
// this account's policy all the same. Another account's policy is another
// account's, and a bare name is the name of a loaded policy.
func TestPolicy_ARefIsReadAsThePolicyRowItNames(t *testing.T) {
	rc := domain.RefContext{AccountID: "123456789012", Region: "us-east-1"}
	for _, c := range []struct {
		ref, want string
	}{
		{"arn:aws:iam::aws:policy/" + fixtures.PolicyNameAlsoAWSManaged, "arn:aws:iam::aws:policy/" + fixtures.PolicyNameAlsoAWSManaged},
		{fixtures.LocalPowerUserAccessARN, fixtures.LocalPowerUserAccessARN},
		{"arn:aws:iam::210987654321:policy/audit", ""},
		{"arn:aws:sns:us-east-1:123456789012:topic-name", ""},
	} {
		got := resource.NavIDFromValue("policy", c.ref, rc)
		if got != c.want {
			t.Errorf("NavIDFromValue(policy, %q) = %q, want %q", c.ref, got, c.want)
		}
	}
}
