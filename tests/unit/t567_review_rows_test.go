package unit_test

// Completeness of a stored target list is decided once, from what the fetch
// returned, whichever writer stored it; a lazily added handful of rows is not
// a list; and a pivot that could read nothing says so the same way on every
// pivot.

import (
	"context"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/backup"
	"github.com/aws/aws-sdk-go-v2/service/codepipeline"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/elasticbeanstalk"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	eventbridgetypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
	lambdapkg "github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"github.com/aws/smithy-go"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// t567Core is a demo-mode Core whose session clients are clients.
func t567Core(t *testing.T, clients *awsclient.ServiceClients) *runtime.Core {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	c := runtime.Bootstrap(demo.DemoProfile, demo.DemoRegion, catalog.All())
	c.SetNoCache(true)
	c.SetIsDemo(true)
	c.SetPreSuppliedClients(clients)
	c.HandleClientsReady(runtime.ClientsReadyEvent{Gen: c.ConnectGen(), StackDepth: 1})
	return c
}

// t567NGClusterDeniedFake lists acme-prod and acme-staging and refuses
// ListNodegroups on acme-staging, so the node-group page misses every group
// of that cluster.
type t567NGClusterDeniedFake struct {
	t567NGEKSFake
}

func (f *t567NGClusterDeniedFake) ListClusters(context.Context, *eks.ListClustersInput, ...func(*eks.Options)) (*eks.ListClustersOutput, error) {
	return &eks.ListClustersOutput{Clusters: []string{"acme-prod", "acme-staging"}}, nil
}

func (f *t567NGClusterDeniedFake) ListNodegroups(ctx context.Context, in *eks.ListNodegroupsInput, opts ...func(*eks.Options)) (*eks.ListNodegroupsOutput, error) {
	if aws.ToString(in.ClusterName) == "acme-staging" {
		return nil, t567Denied("eks:ListNodegroups on resource: arn:aws:eks:us-east-1:123456789012:cluster/acme-staging")
	}
	return f.t567NGEKSFake.ListNodegroups(ctx, in, opts...)
}

func t567NGDeniedClients() *awsclient.ServiceClients {
	clients := refClients()
	clients.EKS = &t567NGClusterDeniedFake{t567NGEKSFake{EKSAPI: clients.EKS}}
	lt := &t567LTEC2Fake{EC2API: clients.EC2, data: map[string]ec2types.ResponseLaunchTemplateData{
		"lt-0a1b2c3d4e5f60001": {ImageId: aws.String("ami-0eks111111111111a")},
		"lt-0a1b2c3d4e5f60002": {ImageId: aws.String("ami-0eks111111111111a")},
	}}
	clients.EC2 = lt
	return clients
}

// TestT567CacheWriters_DroppedRowsStoreALowerBound: a node-group page that
// lost a cluster's groups to a refused ListNodegroups is a subset, whichever
// path stored it. Read from the cache it answers what the live read answers.
func TestT567CacheWriters_DroppedRowsStoreALowerBound(t *testing.T) {
	b := newRefBench(t)
	staging := b.row(t, "eks", "acme-staging")
	lt := resource.Resource{ID: "lt-0a1b2c3d4e5f60009", Name: "acme-batch-workers", Type: "lt"}
	eksNG := refChecker(t, "eks", "ng")
	ltNG := refChecker(t, "lt", "ng")

	live := eksNG(context.Background(), t567NGDeniedClients(), staging, resource.ResourceCache{})
	if t567Badge(live) != "(0+)" {
		t.Fatalf("live eks acme-staging → Node Groups = %q, want (0+) with its ListNodegroups refused", t567Badge(live))
	}

	writers := map[string]func(t *testing.T) resource.ResourceCache{
		"canonical list load": func(t *testing.T) resource.ResourceCache {
			clients := t567NGDeniedClients()
			core := t567Core(t, clients)
			result, err := resource.GetPaginatedFetcher("ng")(context.Background(), clients, "")
			if err == nil || len(result.Resources) == 0 {
				t.Fatalf("ng fetch = %d rows, err %v; want rows beside the refused cluster's error", len(result.Resources), err)
			}
			ctrl := newBlessedController(t, core)
			ctrl.Apply(app.Action{Kind: app.ActionCommand, Arg: "ng"})
			ctrl.Handle(messages.ResourcesLoaded{
				ResourceType: "ng",
				Resources:    result.Resources,
				Pagination:   result.Pagination,
				Err:          err,
				Provenance:   messages.FetchProvenanceCanonicalList,
			})
			return core.BuildResourceCacheSnapshot()
		},
		"availability probe": func(t *testing.T) resource.ResourceCache {
			core := t567Core(t, t567NGDeniedClients())
			ev, err := core.ExecuteTask(context.Background(), runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.TaskKindProbeAvailability, Scope: "ng"}})
			if err != nil || ev == nil {
				t.Fatalf("probe ng = %v, %v", ev, err)
			}
			core.HandleEvent(ev)
			return core.BuildResourceCacheSnapshot()
		},
		"demo prefetch": func(t *testing.T) resource.ResourceCache {
			core := t567Core(t, t567NGDeniedClients())
			ev, err := core.ExecuteTask(context.Background(), runtime.TaskRequest{Key: runtime.TaskKey{Kind: runtime.TaskKindDemoPrefetchCounts}})
			if err != nil || ev == nil {
				t.Fatalf("demo prefetch = %v, %v", ev, err)
			}
			core.HandleEvent(ev)
			return core.BuildResourceCacheSnapshot()
		},
	}
	for name, write := range writers {
		t.Run(name, func(t *testing.T) {
			cache := write(t)
			if len(cache["ng"].Resources) == 0 {
				t.Fatalf("%s stored no ng rows", name)
			}
			if got := eksNG(context.Background(), t567NGDeniedClients(), staging, cache); t567Badge(got) != t567Badge(live) {
				t.Errorf("eks acme-staging → Node Groups from the stored list = %q, want %q as the live read", t567Badge(got), t567Badge(live))
			}
			if got := ltNG(context.Background(), t567NGDeniedClients(), lt, cache); t567Badge(got) != "(0+)" {
				t.Errorf("lt %s → EKS Node Groups from the stored list = %q, want (0+): acme-staging's groups were never listed", lt.ID, t567Badge(got))
			}
		})
	}
}

// t567RelatedRun runs one registered pivot the way the executor does, over
// the core's current cache.
func t567RelatedRun(t *testing.T, core *runtime.Core, clients *awsclient.ServiceClients, src resource.Resource, target string) messages.RelatedCheckResult {
	t.Helper()
	keys := map[string]struct{}{}
	for _, k := range core.FetchOriginCacheKeys() {
		keys[k] = struct{}{}
	}
	op := runtime.DetailOperation{ID: 1, ResourceType: src.Type, Resource: src, Clients: clients}
	return runtime.RunRelatedDef(context.Background(), op, core.BuildResourceCacheSnapshot(), keys, refRelatedDef(t, src.Type, target))
}

// t567LazyAdd opens src's pivot to target and stores what it lazily added,
// as the executor's result handler does.
func t567LazyAdd(t *testing.T, core *runtime.Core, clients *awsclient.ServiceClients, src resource.Resource, target string) {
	t.Helper()
	msg := t567RelatedRun(t, core, clients, src, target)
	if len(msg.LazyAddedResources[target]) == 0 {
		t.Fatalf("%s %s → %s lazily added no %s rows", src.Type, src.ID, target, target)
	}
	core.HandleEvent(msg)
}

func t567Typed(r resource.Resource, typ string) resource.Resource {
	r.Type = typ
	return r
}

// TestT567Prefetch_FreshPageStandsOverALazyEntry: a first page read whole is
// whole; the handful of rows a detail lazily added before it says nothing
// about that page.
func TestT567Prefetch_FreshPageStandsOverALazyEntry(t *testing.T) {
	b := newRefBench(t)
	clients := refClients()
	eni := t567Typed(b.byType["eni"][slices.IndexFunc(b.byType["eni"], func(r resource.Resource) bool {
		return refChecker(t, "eni", "ec2")(context.Background(), clients, t567Typed(r, "eni"), resource.ResourceCache{}).Count() > 0
	})], "eni")
	sg := t567Typed(b.row(t, "sg", "sg-0aaa111111111111a"), "sg")
	subnet := t567Typed(b.row(t, "subnet", "subnet-0aaa111111111111a"), "subnet")

	cold := t567Core(t, clients)
	wantSG := t567RelatedRun(t, cold, clients, sg, "ec2").Result
	if wantSG.Truncated() || wantSG.State() != domain.RelatedResolved {
		t.Fatalf("sg → ec2 on a cold session = %q, want an exact count", t567Badge(wantSG))
	}
	wantSubnet := t567RelatedRun(t, cold, clients, subnet, "ec2").Result

	core := t567Core(t, clients)
	t567LazyAdd(t, core, clients, eni, "ec2")
	msg := t567RelatedRun(t, core, clients, sg, "ec2")
	if t567Badge(msg.Result) != t567Badge(wantSG) {
		t.Errorf("sg → EC2 Instances after an eni lazily added an instance = %q, want %q as on a cold session", t567Badge(msg.Result), t567Badge(wantSG))
	}
	core.HandleEvent(msg)
	if got := t567RelatedRun(t, core, clients, subnet, "ec2").Result; t567Badge(got) != t567Badge(wantSubnet) {
		t.Errorf("subnet → EC2 Instances over the page the sg pivot stored = %q, want %q as on a cold session", t567Badge(got), t567Badge(wantSubnet))
	}
}

// TestT567LazyRows_AreNotTheList: rows added one by one for a detail are not
// the type's list; a pivot over the list reads its first page.
func TestT567LazyRows_AreNotTheList(t *testing.T) {
	b := newRefBench(t)
	clients := refClients()
	eni := t567Typed(b.byType["eni"][slices.IndexFunc(b.byType["eni"], func(r resource.Resource) bool {
		return refChecker(t, "eni", "ec2")(context.Background(), clients, t567Typed(r, "eni"), resource.ResourceCache{}).Count() > 0
	})], "eni")
	vpc := t567Typed(b.row(t, "vpc", "vpc-0abc123def456789a"), "vpc")
	check := refChecker(t, "vpc", "ec2")

	want := check(context.Background(), clients, vpc, resource.ResourceCache{})
	if want.Truncated() || want.Count() < 2 {
		t.Fatalf("vpc → ec2 over a fresh read = %q, want an exact count above the one lazy row", t567Badge(want))
	}

	core := t567Core(t, clients)
	t567LazyAdd(t, core, clients, eni, "ec2")
	if got := check(context.Background(), clients, vpc, core.BuildResourceCacheSnapshot()); t567Badge(got) != t567Badge(want) || !slices.Equal(sortedIDs(got), sortedIDs(want)) {
		t.Errorf("vpc → EC2 Instances with only lazily added ec2 rows cached = %q %v, want %q %v as over the first page", t567Badge(got), sortedIDs(got), t567Badge(want), sortedIDs(want))
	}
	if got := t567RelatedRun(t, core, clients, vpc, "ec2").Result; t567Badge(got) != t567Badge(want) {
		t.Errorf("vpc → EC2 Instances through the executor with only lazily added ec2 rows cached = %q, want %q", t567Badge(got), t567Badge(want))
	}
}

type t567DDBRefuse struct{ awsclient.DynamoDBAPI }

func (t567DDBRefuse) DescribeKinesisStreamingDestination(context.Context, *dynamodb.DescribeKinesisStreamingDestinationInput, ...func(*dynamodb.Options)) (*dynamodb.DescribeKinesisStreamingDestinationOutput, error) {
	return nil, t567Denied("dynamodb:DescribeKinesisStreamingDestination")
}

type t567SFNRefuse struct{ awsclient.SFNAPI }

func (t567SFNRefuse) DescribeStateMachine(context.Context, *sfn.DescribeStateMachineInput, ...func(*sfn.Options)) (*sfn.DescribeStateMachineOutput, error) {
	return nil, t567Denied("states:DescribeStateMachine")
}

type t567EBRefuse struct{ awsclient.ElasticBeanstalkAPI }

func (t567EBRefuse) DescribeConfigurationSettings(context.Context, *elasticbeanstalk.DescribeConfigurationSettingsInput, ...func(*elasticbeanstalk.Options)) (*elasticbeanstalk.DescribeConfigurationSettingsOutput, error) {
	return nil, t567Denied("elasticbeanstalk:DescribeConfigurationSettings")
}

type t567BackupRefuse struct{ awsclient.BackupAPI }

func (t567BackupRefuse) GetBackupVaultNotifications(context.Context, *backup.GetBackupVaultNotificationsInput, ...func(*backup.Options)) (*backup.GetBackupVaultNotificationsOutput, error) {
	return nil, t567Denied("backup:GetBackupVaultNotifications")
}

type t567EventsRefuse struct{ awsclient.EventBridgeAPI }

func (t567EventsRefuse) ListTargetsByRule(context.Context, *eventbridge.ListTargetsByRuleInput, ...func(*eventbridge.Options)) (*eventbridge.ListTargetsByRuleOutput, error) {
	return nil, t567Denied("events:ListTargetsByRule")
}

type t567PipelineRefuse struct{ awsclient.CodePipelineAPI }

func (t567PipelineRefuse) GetPipeline(context.Context, *codepipeline.GetPipelineInput, ...func(*codepipeline.Options)) (*codepipeline.GetPipelineOutput, error) {
	return nil, t567Denied("codepipeline:GetPipeline")
}

type t567LambdaRefuse struct{ awsclient.LambdaAPI }

func (t567LambdaRefuse) GetFunction(context.Context, *lambdapkg.GetFunctionInput, ...func(*lambdapkg.Options)) (*lambdapkg.GetFunctionOutput, error) {
	return nil, t567Denied("lambda:GetFunction")
}

// TestT567ReverseScans_NothingReadIsUnknownEverywhere: a scan whose every
// per-row read was refused established nothing about any row. Every such
// pivot reads unknown, and the refusal reaches the flash.
func TestT567ReverseScans_NothingReadIsUnknownEverywhere(t *testing.T) {
	b := newRefBench(t)
	// first is the first row of typ whose pivot to target is answered when
	// every read succeeds.
	first := func(typ, target string) resource.Resource {
		check := refChecker(t, typ, target)
		for _, r := range b.byType[typ] {
			if check(context.Background(), refClients(), r, b.cache).State() != domain.RelatedUnknown {
				return r
			}
		}
		t.Fatalf("no demo %s row answers its %s pivot", typ, target)
		return resource.Resource{}
	}
	sfnRows := append(slices.Clone(b.byType["sfn"]), awsclient.DegradedDetails("sfn", "acme-nightly-export", t567Denied("states:DescribeStateMachine")))
	cases := []struct {
		source, target string
		src            resource.Resource
		refuse         func(c *awsclient.ServiceClients)
		cache          resource.ResourceCache
	}{
		{"kinesis", "ddb", first("kinesis", "ddb"), func(c *awsclient.ServiceClients) { c.DynamoDB = t567DDBRefuse{c.DynamoDB} }, nil},
		{"ecs-svc", "sfn", b.row(t, "ecs-svc", "acme-services/api-gateway"), func(c *awsclient.ServiceClients) { c.SFN = t567SFNRefuse{c.SFN} },
			t567CacheWith(b.cache, "sfn", resource.ResourceCacheEntry{Resources: sfnRows})},
		{"secrets", "eb", b.row(t, "secrets", "prod/database/primary"), func(c *awsclient.ServiceClients) { c.ElasticBeanstalk = t567EBRefuse{c.ElasticBeanstalk} }, nil},
		{"backup", "sns", first("backup", "sns"), func(c *awsclient.ServiceClients) { c.Backup = t567BackupRefuse{c.Backup} }, nil},
		{"lambda", "eb-rule", first("lambda", "eb-rule"), func(c *awsclient.ServiceClients) { c.EventBridge = t567EventsRefuse{c.EventBridge} }, nil},
		{"cb", "pipeline", first("cb", "pipeline"), func(c *awsclient.ServiceClients) { c.CodePipeline = t567PipelineRefuse{c.CodePipeline} }, nil},
		{"ecr", "lambda", first("ecr", "lambda"), func(c *awsclient.ServiceClients) { c.Lambda = t567LambdaRefuse{c.Lambda} }, nil},
		{"ecr", "pipeline", first("ecr", "pipeline"), func(c *awsclient.ServiceClients) { c.CodePipeline = t567PipelineRefuse{c.CodePipeline} }, nil},
	}
	for _, tc := range cases {
		t.Run(tc.source+"→"+tc.target, func(t *testing.T) {
			cache := tc.cache
			if cache == nil {
				cache = b.cache
			}
			def := refRelatedDef(t, tc.source, tc.target)
			if healthy := def.Checker(context.Background(), refClients(), tc.src, cache); healthy.State() == domain.RelatedUnknown {
				t.Fatalf("%s %s → %s reads unknown with every read answered; the case proves nothing", tc.source, tc.src.ID, tc.target)
			}
			clients := refClients()
			tc.refuse(clients)
			got := def.Checker(context.Background(), clients, tc.src, cache)
			if got.State() != domain.RelatedUnknown {
				t.Errorf("%s %s → %s with every read refused = %q (state %s), want unknown", tc.source, tc.src.ID, tc.target, t567Badge(got), got.State())
			}
			intents, _ := runtime.New(session.New(), nil).HandleRelatedCheckResult(runtime.RelatedCheckResultEvent{
				ResourceType: tc.source, SourceResourceID: tc.src.ID, DefDisplayName: def.DisplayName, Result: got,
			})
			if !slices.ContainsFunc(intents, func(i runtime.UIIntent) bool { f, ok := i.(runtime.FlashIntent); return ok && f.IsError }) {
				t.Errorf("%s → %s with every read refused raised no error flash", tc.source, tc.target)
			}
		})
	}
}

// t567ALBRulesFake answers acme-prod-web's listener the way an Elastic
// Beanstalk environment with extra processes is wired: the default action
// forwards to the default process, listener rules forward to the others.
type t567ALBRulesFake struct {
	awsclient.ELBv2API
	defaultTG string
	ruleTGs   []string
}

const t567ListenerARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:listener/app/acme-prod-web/1234567890abcdef/f1e2d3c4b5a69788"

func t567Forward(tgARN string) []elbv2types.Action {
	return []elbv2types.Action{{Type: elbv2types.ActionTypeEnumForward, Order: aws.Int32(1), TargetGroupArn: aws.String(tgARN),
		ForwardConfig: &elbv2types.ForwardActionConfig{TargetGroups: []elbv2types.TargetGroupTuple{{TargetGroupArn: aws.String(tgARN), Weight: aws.Int32(1)}}}}}
}

func (f *t567ALBRulesFake) DescribeListeners(context.Context, *elbv2.DescribeListenersInput, ...func(*elbv2.Options)) (*elbv2.DescribeListenersOutput, error) {
	return &elbv2.DescribeListenersOutput{Listeners: []elbv2types.Listener{{
		ListenerArn:     aws.String(t567ListenerARN),
		LoadBalancerArn: aws.String("arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-prod-web/1234567890abcdef"),
		Port:            aws.Int32(80),
		Protocol:        elbv2types.ProtocolEnumHttp,
		DefaultActions:  t567Forward(f.defaultTG),
	}}}, nil
}

func (f *t567ALBRulesFake) DescribeRules(context.Context, *elbv2.DescribeRulesInput, ...func(*elbv2.Options)) (*elbv2.DescribeRulesOutput, error) {
	out := &elbv2.DescribeRulesOutput{Rules: []elbv2types.Rule{{
		RuleArn: aws.String(t567ListenerARN + "/0000000000000000"), Priority: aws.String("default"), IsDefault: aws.Bool(true), Actions: t567Forward(f.defaultTG),
	}}}
	for i, tg := range f.ruleTGs {
		out.Rules = append(out.Rules, elbv2types.Rule{
			RuleArn:    aws.String(t567ListenerARN + "/1a2b3c4d5e6f708" + string(rune('1'+i))),
			Priority:   aws.String(string(rune('1' + i))),
			Conditions: []elbv2types.RuleCondition{{Field: aws.String("path-pattern"), PathPatternConfig: &elbv2types.PathPatternConditionConfig{Values: []string{"/process-" + string(rune('a'+i)) + "/*"}}}},
			Actions:    t567Forward(tg),
		})
	}
	return out, nil
}

// TestT567EbTG_RuleForwardedTargetGroupsCount: an environment's load balancer
// forwards to every target group that names it in LoadBalancerArns, through
// its listener rules as well as its default action.
func TestT567EbTG_RuleForwardedTargetGroupsCount(t *testing.T) {
	b := newRefBench(t)
	const webARN = "arn:aws:elasticloadbalancing:us-east-1:123456789012:loadbalancer/app/acme-prod-web/1234567890abcdef"
	arnOf := map[string]string{}
	var want []string
	for _, tg := range b.byType["tg"] {
		raw, ok := tg.RawStruct.(elbv2types.TargetGroup)
		if p, isPtr := tg.RawStruct.(*elbv2types.TargetGroup); isPtr {
			raw, ok = *p, true
		}
		if ok && slices.Contains(raw.LoadBalancerArns, webARN) {
			want = append(want, tg.ID)
			arnOf[tg.ID] = aws.ToString(raw.TargetGroupArn)
		}
	}
	slices.Sort(want)
	if len(want) < 2 {
		t.Fatalf("demo has %d target groups on acme-prod-web, the witness needs two or more", len(want))
	}
	clients := refClients()
	var rules []string
	for _, id := range want[1:] {
		rules = append(rules, arnOf[id])
	}
	clients.ELBv2 = &t567ALBRulesFake{ELBv2API: clients.ELBv2, defaultTG: arnOf[want[0]], ruleTGs: rules}

	got := refChecker(t, "eb", "tg")(context.Background(), clients, b.row(t, "eb", "e-acmeprodapi"), b.cache)
	switch {
	case got.State() != domain.RelatedResolved:
		t.Errorf("eb → Target Groups = state %s, want the target groups counted", got.State())
	case !got.Truncated() && !slices.Equal(sortedIDs(got), want):
		t.Errorf("eb → Target Groups = %s %v, want (%d) %v: rule-forwarded groups are the environment's too", t567Badge(got), sortedIDs(got), len(want), want)
	case got.Truncated() && !slices.ContainsFunc(sortedIDs(got), func(id string) bool { return slices.Contains(want, id) }):
		t.Errorf("eb → Target Groups = %s %v, want a lower bound over %v", t567Badge(got), sortedIDs(got), want)
	}
}

// t567BusesFake has a default bus that answers and a custom bus that refuses.
type t567BusesFake struct {
	awsclient.EventBridgeAPI
}

func (t567BusesFake) ListEventBuses(context.Context, *eventbridge.ListEventBusesInput, ...func(*eventbridge.Options)) (*eventbridge.ListEventBusesOutput, error) {
	return &eventbridge.ListEventBusesOutput{EventBuses: []eventbridgetypes.EventBus{
		{Name: aws.String("default"), Arn: aws.String("arn:aws:events:us-east-1:123456789012:event-bus/default")},
		{Name: aws.String("acme-orders"), Arn: aws.String("arn:aws:events:us-east-1:123456789012:event-bus/acme-orders")},
	}}, nil
}

func (f t567BusesFake) ListRuleNamesByTarget(ctx context.Context, in *eventbridge.ListRuleNamesByTargetInput, opts ...func(*eventbridge.Options)) (*eventbridge.ListRuleNamesByTargetOutput, error) {
	if aws.ToString(in.EventBusName) == "acme-orders" {
		return nil, t567Denied("events:ListRuleNamesByTarget on resource: arn:aws:events:us-east-1:123456789012:event-bus/acme-orders")
	}
	return f.EventBridgeAPI.ListRuleNamesByTarget(ctx, in, opts...)
}

// TestT567EbRules_RefusedBusKeepsWhatOtherBusesFound: rules found on a bus
// that answered are rules of the resource; a bus that refused may hold more.
func TestT567EbRules_RefusedBusKeepsWhatOtherBusesFound(t *testing.T) {
	b := newRefBench(t)
	check := refChecker(t, "pipeline", "eb-rule")
	pipeline := b.row(t, "pipeline", "acme-api-deploy")
	healthy := check(context.Background(), refClients(), pipeline, b.cache)
	if healthy.Count() == 0 {
		t.Fatalf("demo pipeline acme-api-deploy → EventBridge Rules = %q, want a witness", t567Badge(healthy))
	}
	clients := refClients()
	clients.EventBridge = t567BusesFake{clients.EventBridge}
	got := check(context.Background(), clients, pipeline, b.cache)
	if !got.Truncated() || !slices.Equal(sortedIDs(got), sortedIDs(healthy)) {
		t.Errorf("pipeline → EventBridge Rules with bus acme-orders refused = %q %v (state %s), want %v as a lower bound", t567Badge(got), sortedIDs(got), got.State(), sortedIDs(healthy))
	}
}

type t567R53Refuse struct{ awsclient.Route53API }

func (t567R53Refuse) ListHostedZones(context.Context, *route53.ListHostedZonesInput, ...func(*route53.Options)) (*route53.ListHostedZonesOutput, error) {
	return nil, &smithy.GenericAPIError{
		Code:    "AccessDenied",
		Message: "User: arn:aws:sts::123456789012:assumed-role/example-readonly/session is not authorized to perform: route53:ListHostedZones",
	}
}

// TestT567ACMR53_ZoneListFailureIsAnError: a zone list that could not be read
// is a failed read, reported as every sibling pivot reports it.
func TestT567ACMR53_ZoneListFailureIsAnError(t *testing.T) {
	b := newRefBench(t)
	check := refChecker(t, "acm", "r53")
	i := slices.IndexFunc(b.byType["acm"], func(r resource.Resource) bool {
		return check(context.Background(), refClients(), r, b.cache).Count() > 0
	})
	if i < 0 {
		t.Fatal("no demo certificate validates in a demo zone")
	}
	cert := b.byType["acm"][i]
	clients := refClients()
	clients.Route53 = t567R53Refuse{clients.Route53}
	noZones := maps.Clone(b.cache)
	delete(noZones, "r53")
	got := check(context.Background(), clients, cert, noZones)
	if got.State() != domain.RelatedError || got.Err() == nil || !strings.Contains(got.Err().Error(), "ListHostedZones") {
		t.Errorf("acm %s → Route 53 Zones with ListHostedZones refused = state %s err %v, want the refusal as an error", cert.ID, got.State(), got.Err())
	}
}
