// row_identity_parent_scoped_test.go — row identity for names AWS scopes to a
// parent (ECS service in a cluster, CodeArtifact repository in a domain,
// EventBridge rule on a bus, inline policy in a group).
//
// Every list is one flat set keyed by Resource.ID, and every related pivot
// opens its target by that ID. A bare name as the ID makes the second
// same-named resource either vanish or resolve to the first parent's row, so
// the ID must carry the parent while Name keeps the bare name the columns show.
package unit_test

import (
	"context"
	"sort"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/codeartifact"
	codeartifacttypes "github.com/aws/aws-sdk-go-v2/service/codeartifact/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/session"
)

const ridAccount = "123456789012"

// ridOne returns the single row whose Name is name and whose parent field
// holds parent; zero or two such rows fail the test.
func ridOne(t *testing.T, rows []resource.Resource, name, parentKey, parent string) resource.Resource {
	t.Helper()
	var hits []resource.Resource
	for _, r := range rows {
		if r.Name == name && r.Fields[parentKey] == parent {
			hits = append(hits, r)
		}
	}
	if len(hits) != 1 {
		t.Fatalf("want exactly one row named %q with %s=%q, got %d (rows: %v)", name, parentKey, parent, len(hits), ridIDs(rows))
	}
	return hits[0]
}

func ridIDs(rows []resource.Resource) []string {
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	return ids
}

// ridAssertDistinctIDs fails when two rows share an ID.
func ridAssertDistinctIDs(t *testing.T, rows []resource.Resource) {
	t.Helper()
	seen := map[string]bool{}
	for _, r := range rows {
		if seen[r.ID] {
			t.Errorf("two rows share the ID %q (all IDs: %v)", r.ID, ridIDs(rows))
		}
		seen[r.ID] = true
	}
}

// ridRelatedDef returns the one related def on source that targets target.
func ridRelatedDef(t *testing.T, source, target string) resource.RelatedDef {
	t.Helper()
	td := resource.FindResourceType(source)
	if td == nil {
		t.Fatalf("%s is not a registered type", source)
	}
	var found []resource.RelatedDef
	for _, d := range td.Related {
		if d.TargetType == target {
			found = append(found, d)
		}
	}
	if len(found) != 1 {
		t.Fatalf("%s: want one related def targeting %s, got %d", source, target, len(found))
	}
	return found[0]
}

// ridOpensServices asserts that the IDs a related row carries open exactly the want
// rows of the target list, one row per ID: an ID matching nothing opens
// nothing, an ID matching two rows opens the wrong parent's resource too.
func ridOpensServices(t *testing.T, label string, ids []string, targetRows []resource.Resource, want ...resource.Resource) {
	t.Helper()
	if len(ids) != len(want) {
		t.Fatalf("%s: related row carries %d IDs %v, want %d", label, len(ids), ids, len(want))
	}
	var opened []string
	for _, id := range ids {
		n := 0
		for _, r := range targetRows {
			if r.ID == id {
				n++
				opened = append(opened, r.Fields["cluster"]+"/"+r.Name)
			}
		}
		if n != 1 {
			t.Errorf("%s: related ID %q matches %d target rows, want exactly 1", label, id, n)
		}
	}
	var wantOpened []string
	for _, w := range want {
		wantOpened = append(wantOpened, w.Fields["cluster"]+"/"+w.Name)
	}
	sort.Strings(opened)
	sort.Strings(wantOpened)
	if len(opened) != len(wantOpened) {
		t.Fatalf("%s: opens %v, want %v", label, opened, wantOpened)
	}
	for i := range opened {
		if opened[i] != wantOpened[i] {
			t.Errorf("%s: opens %v, want %v", label, opened, wantOpened)
			return
		}
	}
}

// ---------------------------------------------------------------------------
// ECS services
// ---------------------------------------------------------------------------

type ridECSFake struct {
	clusterArns []string
	services    map[string][]ecstypes.Service
}

func (f *ridECSFake) ListClusters(_ context.Context, _ *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	return &ecs.ListClustersOutput{ClusterArns: f.clusterArns}, nil
}

func (f *ridECSFake) ListServices(_ context.Context, in *ecs.ListServicesInput, _ ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	var arns []string
	for _, s := range f.services[aws.ToString(in.Cluster)] {
		arns = append(arns, aws.ToString(s.ServiceArn))
	}
	return &ecs.ListServicesOutput{ServiceArns: arns}, nil
}

func (f *ridECSFake) DescribeServices(_ context.Context, in *ecs.DescribeServicesInput, _ ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error) {
	want := map[string]bool{}
	for _, a := range in.Services {
		want[a] = true
	}
	var out []ecstypes.Service
	for _, s := range f.services[aws.ToString(in.Cluster)] {
		if want[aws.ToString(s.ServiceArn)] {
			out = append(out, s)
		}
	}
	return &ecs.DescribeServicesOutput{Services: out}, nil
}

func ridClusterArn(cluster string) string {
	return "arn:aws:ecs:us-east-1:" + ridAccount + ":cluster/" + cluster
}

func ridTGArn(name, suffix string) string {
	return "arn:aws:elasticloadbalancing:us-east-1:" + ridAccount + ":targetgroup/" + name + "/" + suffix
}

func ridService(cluster, name, tgArn string) ecstypes.Service {
	svc := ecstypes.Service{
		ServiceName:    aws.String(name),
		ServiceArn:     aws.String("arn:aws:ecs:us-east-1:" + ridAccount + ":service/" + cluster + "/" + name),
		ClusterArn:     aws.String(ridClusterArn(cluster)),
		Status:         aws.String("ACTIVE"),
		DesiredCount:   2,
		RunningCount:   2,
		LaunchType:     ecstypes.LaunchTypeFargate,
		TaskDefinition: aws.String("arn:aws:ecs:us-east-1:" + ridAccount + ":task-definition/" + cluster + "-" + name + ":14"),
		CreatedAt:      aws.Time(time.Date(2026, 3, 2, 9, 30, 0, 0, time.UTC)),
	}
	if tgArn != "" {
		svc.LoadBalancers = []ecstypes.LoadBalancer{{
			TargetGroupArn: aws.String(tgArn),
			ContainerName:  aws.String(name),
			ContainerPort:  aws.Int32(8080),
		}}
	}
	return svc
}

// ridECSServiceRows fetches the "blue" and "green" clusters, each running a
// service named "api" behind its own target group, plus "worker" in green.
func ridECSServiceRows(t *testing.T) []resource.Resource {
	t.Helper()
	blue, green := ridClusterArn("blue"), ridClusterArn("green")
	fake := &ridECSFake{
		clusterArns: []string{blue, green},
		services: map[string][]ecstypes.Service{
			blue:  {ridService("blue", "api", ridTGArn("api-blue", "6d0ecf831eec9f09"))},
			green: {ridService("green", "api", ridTGArn("api-green", "0c1f2d3e4a5b6c7d")), ridService("green", "worker", "")},
		},
	}
	out, err := awsclient.FetchECSServicesPage(context.Background(), fake, fake, fake, "")
	if err != nil {
		t.Fatalf("FetchECSServicesPage: %v", err)
	}
	return out.Resources
}

// TestRowIdentity_ECSSvc_SameNameInTwoClustersAreTwoRows: two services called
// "api" in two clusters are two rows, each still named "api".
func TestRowIdentity_ECSSvc_SameNameInTwoClustersAreTwoRows(t *testing.T) {
	rows := ridECSServiceRows(t)
	if len(rows) != 3 {
		t.Fatalf("fetched %d rows, want 3", len(rows))
	}
	ridAssertDistinctIDs(t, rows)

	blue := ridOne(t, rows, "api", "cluster", "blue")
	green := ridOne(t, rows, "api", "cluster", "green")
	if blue.ID == green.ID {
		t.Errorf("api in blue and api in green share the ID %q", blue.ID)
	}
	for _, r := range []resource.Resource{blue, green} {
		if got := r.Fields["service_name"]; got != "api" {
			t.Errorf("row %q: Fields[service_name] = %q, want %q", r.ID, got, "api")
		}
	}
}

// TestRowIdentity_ECSSvc_TargetGroupPivotOpensItsOwnService pins that a
// target group's ECS Services row opens the service behind that target group,
// not its same-named twin in the other cluster.
func TestRowIdentity_ECSSvc_TargetGroupPivotOpensItsOwnService(t *testing.T) {
	rows := ridECSServiceRows(t)
	blue := ridOne(t, rows, "api", "cluster", "blue")

	tgArn := ridTGArn("api-blue", "6d0ecf831eec9f09")
	tg := resource.Resource{
		ID:     "api-blue",
		Name:   "api-blue",
		Fields: map[string]string{"target_group_arn": tgArn, "target_group_name": "api-blue"},
		RawStruct: elbv2types.TargetGroup{
			TargetGroupArn:  aws.String(tgArn),
			TargetGroupName: aws.String("api-blue"),
			Protocol:        elbv2types.ProtocolEnumHttp,
			Port:            aws.Int32(8080),
			TargetType:      elbv2types.TargetTypeEnumIp,
		},
	}
	cache := resource.ResourceCache{"ecs-svc": {Resources: rows}}

	def := ridRelatedDef(t, "tg", "ecs-svc")
	res := def.Checker(context.Background(), &awsclient.ServiceClients{Region: "us-east-1"}, tg, cache)
	ridOpensServices(t, "tg api-blue -> ECS Services", res.ResourceIDs(), rows, blue)
}

// TestRowIdentity_ECSSvc_TaskPivotOpensItsOwnService: a task started by the
// blue cluster's "api" service opens that service. The task only records
// "service:api" in Group, so the cluster has to come from the task itself.
func TestRowIdentity_ECSSvc_TaskPivotOpensItsOwnService(t *testing.T) {
	rows := ridECSServiceRows(t)
	blue := ridOne(t, rows, "api", "cluster", "blue")

	taskArn := "arn:aws:ecs:us-east-1:" + ridAccount + ":task/blue/9f1c2b7e4d3a4f0e8b6c5d4e3f2a1b0c"
	task := resource.Resource{
		ID:   taskArn,
		Name: "9f1c2b7e4d3a4f0e8b6c5d4e3f2a1b0c",
		Fields: map[string]string{
			"task_id":     "9f1c2b7e4d3a4f0e8b6c5d4e3f2a1b0c",
			"cluster":     ridClusterArn("blue"),
			"last_status": "RUNNING",
		},
		RawStruct: ecstypes.Task{
			TaskArn:           aws.String(taskArn),
			ClusterArn:        aws.String(ridClusterArn("blue")),
			Group:             aws.String("service:api"),
			LastStatus:        aws.String("RUNNING"),
			DesiredStatus:     aws.String("RUNNING"),
			LaunchType:        ecstypes.LaunchTypeFargate,
			TaskDefinitionArn: aws.String("arn:aws:ecs:us-east-1:" + ridAccount + ":task-definition/blue-api:14"),
		},
	}
	cache := resource.ResourceCache{"ecs-svc": {Resources: rows}}

	def := ridRelatedDef(t, "ecs-task", "ecs-svc")
	res := def.Checker(context.Background(), &awsclient.ServiceClients{Region: "us-east-1"}, task, cache)
	ridOpensServices(t, "ecs-task (blue, service:api) -> ECS Services", res.ResourceIDs(), rows, blue)
}

// TestRowIdentity_ECSSvc_ClusterPivotOpensOnlyItsServices: the blue cluster's
// ECS Services row opens blue's "api" and nothing from green.
func TestRowIdentity_ECSSvc_ClusterPivotOpensOnlyItsServices(t *testing.T) {
	rows := ridECSServiceRows(t)
	blue := ridOne(t, rows, "api", "cluster", "blue")

	cluster := resource.Resource{
		ID:     "blue",
		Name:   "blue",
		Fields: map[string]string{"cluster_name": "blue", "status": "ACTIVE"},
		RawStruct: ecstypes.Cluster{
			ClusterArn:          aws.String(ridClusterArn("blue")),
			ClusterName:         aws.String("blue"),
			Status:              aws.String("ACTIVE"),
			ActiveServicesCount: 1,
		},
	}
	cache := resource.ResourceCache{"ecs-svc": {Resources: rows}}

	def := ridRelatedDef(t, "ecs", "ecs-svc")
	res := def.Checker(context.Background(), &awsclient.ServiceClients{Region: "us-east-1"}, cluster, cache)
	ridOpensServices(t, "ecs blue -> ECS Services", res.ResourceIDs(), rows, blue)
}

// ---------------------------------------------------------------------------
// CodeArtifact repositories
// ---------------------------------------------------------------------------

type ridCodeArtifactFake struct {
	repos []codeartifacttypes.RepositorySummary
}

func (f *ridCodeArtifactFake) ListRepositories(_ context.Context, _ *codeartifact.ListRepositoriesInput, _ ...func(*codeartifact.Options)) (*codeartifact.ListRepositoriesOutput, error) {
	return &codeartifact.ListRepositoriesOutput{Repositories: f.repos}, nil
}

func ridRepo(domain, name string) codeartifacttypes.RepositorySummary {
	return codeartifacttypes.RepositorySummary{
		Name:                 aws.String(name),
		DomainName:           aws.String(domain),
		DomainOwner:          aws.String(ridAccount),
		AdministratorAccount: aws.String(ridAccount),
		Arn:                  aws.String("arn:aws:codeartifact:us-east-1:" + ridAccount + ":repository/" + domain + "/" + name),
		Description:          aws.String("npm mirror for " + domain),
		CreatedTime:          aws.Time(time.Date(2025, 11, 4, 14, 2, 0, 0, time.UTC)),
	}
}

// TestRowIdentity_CodeArtifact_SameNameInTwoDomainsAreTwoRows: ListRepositories
// spans every domain in the account, and a repository name is only unique
// inside its domain.
func TestRowIdentity_CodeArtifact_SameNameInTwoDomainsAreTwoRows(t *testing.T) {
	fake := &ridCodeArtifactFake{repos: []codeartifacttypes.RepositorySummary{
		ridRepo("acme-platform", "npm-store"),
		ridRepo("acme-data", "npm-store"),
		ridRepo("acme-data", "pypi-store"),
	}}
	out, err := awsclient.FetchCodeArtifactReposPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchCodeArtifactReposPage: %v", err)
	}
	rows := out.Resources
	if len(rows) != 3 {
		t.Fatalf("fetched %d rows, want 3", len(rows))
	}
	ridAssertDistinctIDs(t, rows)

	platform := ridOne(t, rows, "npm-store", "domain_name", "acme-platform")
	data := ridOne(t, rows, "npm-store", "domain_name", "acme-data")
	if platform.ID == data.ID {
		t.Errorf("npm-store in acme-platform and in acme-data share the ID %q", platform.ID)
	}
	for _, r := range []resource.Resource{platform, data} {
		if got := r.Fields["repo_name"]; got != "npm-store" {
			t.Errorf("row %q: Fields[repo_name] = %q, want %q", r.ID, got, "npm-store")
		}
	}
}

// ---------------------------------------------------------------------------
// EventBridge rules
// ---------------------------------------------------------------------------

type ridEventBridgeFake struct {
	rules []eventbridgetypes.Rule
}

func (f *ridEventBridgeFake) ListRules(_ context.Context, _ *eventbridge.ListRulesInput, _ ...func(*eventbridge.Options)) (*eventbridge.ListRulesOutput, error) {
	return &eventbridge.ListRulesOutput{Rules: f.rules}, nil
}

func ridRule(bus, name string) eventbridgetypes.Rule {
	arn := "arn:aws:events:us-east-1:" + ridAccount + ":rule/" + name
	if bus != "default" {
		arn = "arn:aws:events:us-east-1:" + ridAccount + ":rule/" + bus + "/" + name
	}
	return eventbridgetypes.Rule{
		Name:         aws.String(name),
		Arn:          aws.String(arn),
		EventBusName: aws.String(bus),
		State:        eventbridgetypes.RuleStateEnabled,
		EventPattern: aws.String(`{"source":["acme.orders"],"detail-type":["OrderPlaced"]}`),
		Description:  aws.String("archive order events"),
	}
}

// TestRowIdentity_EBRule_SameNameOnTwoBusesAreTwoRows: a rule name is unique
// only on its event bus.
func TestRowIdentity_EBRule_SameNameOnTwoBusesAreTwoRows(t *testing.T) {
	fake := &ridEventBridgeFake{rules: []eventbridgetypes.Rule{
		ridRule("default", "archive-orders"),
		ridRule("orders", "archive-orders"),
		ridRule("orders", "notify-fulfilment"),
	}}
	out, err := awsclient.FetchEventBridgeRulesPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchEventBridgeRulesPage: %v", err)
	}
	rows := out.Resources
	if len(rows) != 3 {
		t.Fatalf("fetched %d rows, want 3", len(rows))
	}
	ridAssertDistinctIDs(t, rows)

	onDefault := ridOne(t, rows, "archive-orders", "event_bus", "default")
	onOrders := ridOne(t, rows, "archive-orders", "event_bus", "orders")
	if onDefault.ID == onOrders.ID {
		t.Errorf("archive-orders on default and on orders share the ID %q", onDefault.ID)
	}
	for _, r := range []resource.Resource{onDefault, onOrders} {
		if got := r.Fields["name"]; got != "archive-orders" {
			t.Errorf("row %q: Fields[name] = %q, want %q", r.ID, got, "archive-orders")
		}
	}
}

// ---------------------------------------------------------------------------
// Inline IAM group policies
// ---------------------------------------------------------------------------

type ridIAMFake struct {
	awsclient.IAMAPI

	managed []iamtypes.Policy
	groups  []string
	inline  map[string][]string
}

func (f *ridIAMFake) ListPolicies(_ context.Context, _ *iam.ListPoliciesInput, _ ...func(*iam.Options)) (*iam.ListPoliciesOutput, error) {
	return &iam.ListPoliciesOutput{Policies: f.managed}, nil
}

func (f *ridIAMFake) ListGroups(_ context.Context, _ *iam.ListGroupsInput, _ ...func(*iam.Options)) (*iam.ListGroupsOutput, error) {
	out := &iam.ListGroupsOutput{}
	for _, g := range f.groups {
		out.Groups = append(out.Groups, iamtypes.Group{
			GroupName:  aws.String(g),
			GroupId:    aws.String("AGPA" + g + "EXAMPLE"),
			Arn:        aws.String("arn:aws:iam::" + ridAccount + ":group/" + g),
			Path:       aws.String("/"),
			CreateDate: aws.Time(time.Date(2024, 6, 1, 8, 0, 0, 0, time.UTC)),
		})
	}
	return out, nil
}

func (f *ridIAMFake) ListGroupPolicies(_ context.Context, in *iam.ListGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListGroupPoliciesOutput, error) {
	return &iam.ListGroupPoliciesOutput{PolicyNames: f.inline[aws.ToString(in.GroupName)]}, nil
}

func (f *ridIAMFake) ListAttachedGroupPolicies(_ context.Context, _ *iam.ListAttachedGroupPoliciesInput, _ ...func(*iam.Options)) (*iam.ListAttachedGroupPoliciesOutput, error) {
	return &iam.ListAttachedGroupPoliciesOutput{}, nil
}

func (f *ridIAMFake) GetPolicy(_ context.Context, in *iam.GetPolicyInput, _ ...func(*iam.Options)) (*iam.GetPolicyOutput, error) {
	return nil, &iamtypes.NoSuchEntityException{Message: aws.String("Policy " + aws.ToString(in.PolicyArn) + " was not found.")}
}

// TestRowIdentity_InlinePolicy_SameNameInTwoGroupsAndAManagedPolicy: groups
// "dev" and "ops" each carry an inline policy "s3-read", and the account has a
// customer-managed policy "s3-read" too. Each group's IAM Policies row must
// open its own inline policy, and the managed policy must stay reachable
// under its own ID.
func TestRowIdentity_InlinePolicy_SameNameInTwoGroupsAndAManagedPolicy(t *testing.T) {
	fake := &ridIAMFake{
		managed: []iamtypes.Policy{{
			PolicyName:       aws.String("s3-read"),
			PolicyId:         aws.String("ANPAS3READEXAMPLE0001"),
			Arn:              aws.String("arn:aws:iam::" + ridAccount + ":policy/s3-read"),
			Path:             aws.String("/"),
			DefaultVersionId: aws.String("v3"),
			AttachmentCount:  aws.Int32(2),
			IsAttachable:     true,
			CreateDate:       aws.Time(time.Date(2024, 2, 12, 10, 0, 0, 0, time.UTC)),
		}},
		groups: []string{"dev", "ops"},
		inline: map[string][]string{"dev": {"s3-read"}, "ops": {"s3-read"}},
	}
	ctx := context.Background()

	managedPage, err := awsclient.FetchIAMPoliciesPage(ctx, fake, "")
	if err != nil {
		t.Fatalf("FetchIAMPoliciesPage: %v", err)
	}
	if len(managedPage.Resources) != 1 {
		t.Fatalf("managed page has %d rows, want 1", len(managedPage.Resources))
	}
	managedID := managedPage.Resources[0].ID

	clients := &awsclient.ServiceClients{IAM: fake, Region: "us-east-1"}
	clients.SetIAMPolicies(session.NewPolicyStore())

	def := ridRelatedDef(t, "iam-group", "policy")
	groupIDs := map[string][]string{}
	for _, g := range []string{"dev", "ops"} {
		row := resource.Resource{ID: g, Name: g, Fields: map[string]string{"group_name": g}}
		res := def.Checker(ctx, clients, row, nil)
		if len(res.ResourceIDs()) != 1 {
			t.Fatalf("group %s: IAM Policies row carries %v, want one inline policy", g, res.ResourceIDs())
		}
		groupIDs[g] = res.ResourceIDs()
	}
	devID, opsID := groupIDs["dev"][0], groupIDs["ops"][0]
	if devID == opsID || devID == managedID || opsID == managedID {
		t.Fatalf("policy IDs collide: managed %q, dev inline %q, ops inline %q", managedID, devID, opsID)
	}

	policyTD := resource.FindResourceType("policy")
	if policyTD == nil || policyTD.FetchByIDs == nil {
		t.Fatal("the policy type has no FetchByIDs resolver")
	}
	resolved, err := policyTD.FetchByIDs(ctx, clients, []string{managedID, devID, opsID})
	if err != nil {
		t.Fatalf("policy FetchByIDs: %v", err)
	}
	byID := map[string]resource.Resource{}
	for _, r := range resolved {
		byID[r.ID] = r
	}
	checks := []struct {
		id, policyType, path string
	}{
		{managedID, "managed", "/"},
		{devID, "inline", "inline/dev"},
		{opsID, "inline", "inline/ops"},
	}
	for _, c := range checks {
		r, ok := byID[c.id]
		if !ok {
			t.Errorf("ID %q resolves to no policy (resolved IDs: %v)", c.id, ridIDs(resolved))
			continue
		}
		if r.Name != "s3-read" {
			t.Errorf("ID %q: Name = %q, want %q", c.id, r.Name, "s3-read")
		}
		if got := r.Fields["policy_type"]; got != c.policyType {
			t.Errorf("ID %q: Fields[policy_type] = %q, want %q", c.id, got, c.policyType)
		}
		if got := r.Fields["path"]; got != c.path {
			t.Errorf("ID %q: Fields[path] = %q, want %q", c.id, got, c.path)
		}
	}
}
