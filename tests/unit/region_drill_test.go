package unit_test

// A related count found in another Region has to drill into that Region. The
// panel that renders "Log Groups (1)" and the list Enter opens read the same
// AWS, or the operator is told a resource exists and then shown an empty
// screen indistinguishable from a real zero.
//
// These tests drive the real checkers and the real executor against SDK
// clients whose transport records the Region every request was signed for.

import (
	"context"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/config"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
)

// rdDrill runs the checker for shortName → target on src, then drives the
// controller the way an operator does — open the detail, press Enter on the
// related row the panel rendered — and executes the fetch tasks that Enter
// produced. Nothing about the Region is set by hand: it has to survive the
// checker, the row, the event and the payload on its own.
func rdDrill(t *testing.T, w *rwWorld, sessionRegion, shortName, target string, src resource.Resource, cache resource.ResourceCache) (resource.RelatedCheckResult, []resource.Resource) {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "example-readonly"
	s.Region = sessionRegion
	s.Clients = w.clients(sessionRegion)

	got := rwChecker(t, shortName, target)(context.Background(), s.Clients, src, cache)

	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: shortName})
	c.ApplyResourcesLoaded(shortName, []resource.Resource{src}, nil, false)
	c.Apply(app.Action{Kind: app.ActionSelect})

	def, idx := rdRelatedDef(t, shortName, target)
	errText := ""
	if err := got.Err(); err != nil {
		errText = err.Error()
	}
	c.ApplyDetailRelatedResultForResource(shortName, src.ID, def.DisplayName, def.TargetType,
		got.EffectiveState(), got.Count(), false, errText, got.Truncated(), got.ResourceIDs(),
		got.FetchFilter(), got.Region())

	w.resetCalls()
	_, tasks := c.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: strconv.Itoa(idx)})
	if len(tasks) == 0 {
		t.Fatalf("Enter on the %s → %s row produced no fetch", shortName, target)
	}
	var rows []resource.Resource
	for _, task := range tasks {
		ev, err := core.ExecuteTask(context.Background(), task)
		if err != nil {
			t.Fatalf("%s → %s drill task %v: %v", shortName, target, task.Key.Kind, err)
		}
		if loaded, ok := ev.(messages.ResourcesLoaded); ok {
			rows = append(rows, loaded.Resources...)
		}
	}
	return got, rows
}

// rdRelatedDef returns the RelatedDef of shortName targeting target and its
// index in the registered order, which is the order the related panel seeds
// its rows in and so the index Enter addresses.
func rdRelatedDef(t *testing.T, shortName, target string) (resource.RelatedDef, int) {
	t.Helper()
	for i, d := range resource.GetRelated(shortName) {
		if d.TargetType == target {
			return d, i
		}
	}
	t.Fatalf("%s has no related def targeting %q", shortName, target)
	return resource.RelatedDef{}, -1
}

func rdIDs(rows []resource.Resource) []string {
	ids := make([]string, 0, len(rows))
	for _, r := range rows {
		ids = append(ids, r.ID)
	}
	slices.Sort(ids)
	return ids
}

// A trail homed in us-west-2, browsed from eu-west-1: the panel resolves its
// log group in us-west-2, and Enter has to list us-west-2's log groups.
func TestRegionDrill_TrailLogGroupIsListedInTheTrailsOwnRegion(t *testing.T) {
	w := newRegionWorld()
	clients := w.clients("eu-west-1")
	org := rwByID(t, rwFetch(t, clients, "trail"), rwOrgTrail)
	cache := resource.ResourceCache{"logs": {Resources: rwFetch(t, clients, "logs")}}

	got, rows := rdDrill(t, w, "eu-west-1", "trail", "logs", org, cache)
	if got.Region() != "us-west-2" {
		t.Fatalf("checker result Region = %q, want us-west-2", got.Region())
	}
	if ids := rdIDs(rows); !slices.Contains(ids, rwOrgTrailLogGroup) {
		t.Errorf("the drilled list holds %v, want the group the panel counted, %s", ids, rwOrgTrailLogGroup)
	}
	for _, c := range w.callsFor("logs", "") {
		if c.Region != "us-west-2" {
			t.Errorf("the drill issued logs %s against %s; the row it counted is in us-west-2", c.Op, c.Region)
		}
	}
}

// The keyboard Enter on a focused related row builds the same navigation the
// click does — a second construction site, and so a second chance to drop the
// Region the count was read in.
func TestRegionDrill_KeyboardEnterCarriesTheRowsRegionToo(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	w := newRegionWorld()
	s := session.New()
	s.Profile = "example-readonly"
	s.Region = "eu-west-1"
	s.Clients = w.clients("eu-west-1")
	org := rwByID(t, rwFetch(t, s.Clients, "trail"), rwOrgTrail)
	cache := resource.ResourceCache{"logs": {Resources: rwFetch(t, s.Clients, "logs")}}
	got := rwChecker(t, "trail", "logs")(context.Background(), s.Clients, org, cache)

	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "trail"})
	c.ApplyResourcesLoaded("trail", []resource.Resource{org}, nil, false)
	c.Apply(app.Action{Kind: app.ActionSelect})

	def, idx := rdRelatedDef(t, "trail", "logs")
	c.ApplyDetailRelatedResultForResource("trail", org.ID, def.DisplayName, def.TargetType,
		got.EffectiveState(), got.Count(), false, "", got.Truncated(), got.ResourceIDs(), nil, got.Region())

	// ToggleFocus moves the cursor into the related panel, where Select is
	// the keyboard Enter the click path never runs through.
	c.Apply(app.Action{Kind: app.ActionToggleFocus})
	for i := 0; i < idx; i++ {
		c.Apply(app.Action{Kind: app.ActionMoveDown})
	}
	row, ok := c.SelectedRelatedRow()
	if !ok || row.TargetType != "logs" {
		t.Fatalf("the focused related row is %+v (ok=%v), want the logs row", row, ok)
	}
	if row.Region != "us-west-2" {
		t.Fatalf("the rendered row carries Region %q, want us-west-2", row.Region)
	}

	_, tasks := c.Apply(app.Action{Kind: app.ActionSelect})
	if len(tasks) == 0 {
		t.Fatal("keyboard Enter on the related row produced no fetch")
	}
	for _, task := range tasks {
		p, ok := task.Payload.(runtime.FetchResourcesPayload)
		if !ok {
			continue
		}
		if p.Region != "us-west-2" {
			t.Errorf("keyboard Enter fetches Region %q, want us-west-2", p.Region)
		}
	}
}

// A row of the same ID in the session's own Region is a different resource.
// The drill must not be satisfied by it — every cache short-circuit on the way
// to the fetch has to know the count came from somewhere else.
func TestRegionDrill_ASameNamedRowInTheSessionRegionDoesNotAnswer(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	w := newRegionWorld()
	s := session.New()
	s.Profile = "example-readonly"
	s.Region = "eu-west-1"
	s.Clients = w.clients("eu-west-1")
	org := rwByID(t, rwFetch(t, s.Clients, "trail"), rwOrgTrail)
	cache := resource.ResourceCache{"logs": {Resources: rwFetch(t, s.Clients, "logs")}}
	got := rwChecker(t, "trail", "logs")(context.Background(), s.Clients, org, cache)
	if got.Region() != "us-west-2" || !slices.Equal(got.ResourceIDs(), []string{rwOrgTrailLogGroup}) {
		t.Fatalf("precondition: checker gave Region %q ids %v", got.Region(), got.ResourceIDs())
	}

	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	// The session's own eu-west-1 list already holds a group of that name.
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "logs"})
	c.ApplyResourcesLoaded("logs", []resource.Resource{
		{ID: rwOrgTrailLogGroup, Name: rwOrgTrailLogGroup, Type: "logs",
			Fields: map[string]string{"log_group_name": rwOrgTrailLogGroup, "retention": "1 day"}},
	}, nil, false)

	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "trail"})
	c.ApplyResourcesLoaded("trail", []resource.Resource{org}, nil, false)
	c.Apply(app.Action{Kind: app.ActionSelect})
	def, idx := rdRelatedDef(t, "trail", "logs")
	c.ApplyDetailRelatedResultForResource("trail", org.ID, def.DisplayName, def.TargetType,
		got.EffectiveState(), got.Count(), false, "", got.Truncated(), got.ResourceIDs(), nil, got.Region())

	w.resetCalls()
	_, tasks := c.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: strconv.Itoa(idx)})
	if len(tasks) == 0 {
		t.Fatal("the drill was answered from the session's own rows; the group it counted is in us-west-2")
	}
	for _, task := range tasks {
		if _, err := core.ExecuteTask(context.Background(), task); err != nil {
			t.Fatalf("%v: %v", task.Key.Kind, err)
		}
	}
	for _, call := range w.callsFor("logs", "") {
		if call.Region != "us-west-2" {
			t.Errorf("the drill issued logs %s against %s", call.Op, call.Region)
		}
	}
}

// The click path onto a navigable field reads the same reference the keyboard
// does: an ARN of another Region names a row the session's own does not hold,
// and the fetch behind the click has to go where the ARN points.
func TestRegionDrill_FieldSelectCarriesTheReferencesRegion(t *testing.T) {
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	const homed = "us-west-2"
	s := session.New()
	s.Profile = "example-readonly"
	s.Region = "eu-west-1"
	core := runtime.New(s, nil)
	c := newBlessedController(t, core)
	c.SetViewConfig(config.DefaultConfig())

	org := resource.Resource{
		ID: rwOrgTrail, Type: "trail", Name: rwOrgTrail,
		Fields: map[string]string{"name": rwOrgTrail, "home_region": homed},
		RawStruct: cloudtrailtypes.Trail{
			Name:                      aws.String(rwOrgTrail),
			HomeRegion:                aws.String(homed),
			CloudWatchLogsLogGroupArn: aws.String("arn:aws:logs:" + homed + ":123456789012:log-group:" + rwOrgTrailLogGroup + ":*"),
		},
	}
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "trail"})
	c.ApplyResourcesLoaded("trail", []resource.Resource{org}, nil, false)
	c.Apply(app.Action{Kind: app.ActionSelect})

	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatal("the trail detail did not open")
	}
	idx := -1
	for i, f := range body.Fields {
		if f.IsNavigable && f.TargetType == "logs" {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatal("the log-group field is not navigable; a reference into another Region still names a row")
	}

	_, tasks := c.Apply(app.Action{Kind: app.ActionFieldSelect, Arg: strconv.Itoa(idx)})
	if len(tasks) == 0 {
		t.Fatal("selecting the log-group field produced no fetch")
	}
	for _, task := range tasks {
		switch p := task.Payload.(type) {
		case runtime.FetchResourcesPayload:
			if p.Region != homed {
				t.Errorf("the field drill fetches Region %q, want %q", p.Region, homed)
			}
		case runtime.FetchByIDDetailPayload:
			if p.Region != homed {
				t.Errorf("the field drill looks the row up in Region %q, want %q", p.Region, homed)
			}
		}
	}
}

// A Region the drill cannot reach is reported, never rendered as a list that
// happens to be empty: a refusal and a genuine zero are different answers.
func TestRegionDrill_ARefusedRegionIsReportedNotRenderedEmpty(t *testing.T) {
	w := newRegionWorld()
	s := session.New()
	s.Region = "eu-west-1"
	s.Clients = w.clients("eu-west-1")
	org := rwByID(t, rwFetch(t, s.Clients, "trail"), rwOrgTrail)
	cache := resource.ResourceCache{"logs": {Resources: rwFetch(t, s.Clients, "logs")}}
	got := rwChecker(t, "trail", "logs")(context.Background(), s.Clients, org, cache)
	w.denyAccess("logs", "us-west-2")

	core := runtime.New(s, nil)
	_, tasks := core.HandleRelatedNavigate(runtime.RelatedNavigateEvent{
		TargetType: "logs", SourceType: "trail", SourceResource: org,
		RelatedIDs: got.ResourceIDs(), Region: got.Region(),
	})
	if len(tasks) == 0 {
		t.Fatal("a drill into another Region issued no fetch")
	}
	for _, task := range tasks {
		ev, err := core.ExecuteTask(context.Background(), task)
		if err != nil {
			t.Fatalf("%v: %v", task.Key.Kind, err)
		}
		if _, ok := ev.(messages.APIError); !ok {
			t.Errorf("%v returned %T for a Region that refused the read, want the refusal reported", task.Key.Kind, ev)
		}
	}
}

// A drill dispatched before the session has clients has nothing to read any
// Region with, and says so rather than taking the process down.
func TestRegionDrill_NoClientsIsReportedNotAPanic(t *testing.T) {
	s := session.New()
	s.Region = "eu-west-1"
	core := runtime.New(s, nil)
	_, tasks := core.HandleRelatedNavigate(runtime.RelatedNavigateEvent{
		TargetType: "logs", SourceType: "trail",
		RelatedIDs: []string{rwOrgTrailLogGroup}, Region: "us-west-2",
	})
	for _, task := range tasks {
		ev, err := core.ExecuteTask(context.Background(), task)
		if err != nil {
			t.Fatalf("%v: %v", task.Key.Kind, err)
		}
		if _, ok := ev.(messages.APIError); !ok {
			t.Errorf("%v returned %T with no clients, want the failure reported", task.Key.Kind, ev)
		}
	}
}

// A trail homed in the session's own Region keeps the session's clients and
// the session's cache: nothing is routed anywhere.
func TestRegionDrill_SessionRegionRowCarriesNoRegion(t *testing.T) {
	w := newRegionWorld()
	clients := w.clients("eu-west-1")
	eu := rwByID(t, rwFetch(t, clients, "trail"), rwEUTrail)
	cache := resource.ResourceCache{"logs": {Resources: rwFetch(t, clients, "logs")}}

	got := rwChecker(t, "trail", "logs")(context.Background(), clients, eu, cache)
	if got.Region() != "" {
		t.Errorf("Region = %q for a trail homed in the session Region, want empty", got.Region())
	}
	if ids := got.ResourceIDs(); !slices.Equal(ids, []string{rwEUTrailLogGroup}) {
		t.Errorf("ids = %v, want [%s]", ids, rwEUTrailLogGroup)
	}
}

// The other three cross-Region pivots carry their Region the same way.
func TestRegionDrill_EveryCrossRegionPivotCarriesItsRegion(t *testing.T) {
	w := newRegionWorld()
	clients := w.clients("eu-west-1")
	ctx := context.Background()
	edge, _ := rwWAFResources(t, w, clients)

	cases := []struct {
		name         string
		shortName    string
		target       string
		src          resource.Resource
		cache        resource.ResourceCache
		wantRegion   string
		wantResource string
	}{
		{"cloudfront certificate", "cf", "acm", rwDistribution(),
			resource.ResourceCache{"acm": {Resources: rwFetch(t, clients, "acm")}}, "us-east-1", rwEdgeCertArn},
		{"route 53 query log group", "r53", "logs",
			resource.Resource{ID: rwZoneID, Name: "acme-example.com.", Fields: map[string]string{"private_zone": "false"}},
			resource.ResourceCache{"logs": {Resources: rwFetch(t, clients, "logs")}}, "us-east-1", rwZoneLogGroup},
		{"cloudfront ACL log group", "waf", "logs", edge, resource.ResourceCache{}, "us-east-1", "aws-waf-logs-acme-edge"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rwChecker(t, tc.shortName, tc.target)(ctx, clients, tc.src, tc.cache)
			if got.Region() != tc.wantRegion {
				t.Errorf("Region = %q, want %q", got.Region(), tc.wantRegion)
			}
			if ids := got.ResourceIDs(); !slices.Contains(ids, tc.wantResource) {
				t.Errorf("ids = %v, want to hold %s", ids, tc.wantResource)
			}
		})
	}
}

// Every wafv2 call about a CloudFront-scope ACL is served by us-east-1, so
// its events and its metrics are both recorded there.
func TestRegionRow_CloudFrontACLLooksUpEventsAndAlarmsInUSEast1(t *testing.T) {
	w := newRegionWorld()
	clients := w.clients("eu-west-1")
	edge, api := rwWAFResources(t, w, clients)

	filter := resource.BuildCloudTrailFilter(edge, "waf")
	if got := filter[resource.CTRegionFilterKey]; got != "us-east-1" {
		t.Errorf("CloudFront ACL event lookup Region = %q, want us-east-1", got)
	}
	if got, ok := resource.BuildCloudTrailFilter(api, "waf")[resource.CTRegionFilterKey]; ok {
		t.Errorf("regional ACL event lookup pinned to %q; it is recorded in the session Region", got)
	}

	alarms := resource.ResourceCache{"alarm": {Resources: rwFetch(t, clients, "alarm")}}
	w.resetCalls()
	rwChecker(t, "waf", "alarm")(context.Background(), clients, edge, alarms)
	asked := 0
	for _, c := range w.callsFor("monitoring", "") {
		asked++
		if c.Region != "us-east-1" {
			t.Errorf("alarm lookup for a CloudFront ACL issued against %s", c.Region)
		}
	}
	if asked == 0 {
		t.Error("the CloudFront ACL's alarms came from the session's own list; its metrics are published in us-east-1")
	}

	w.resetCalls()
	rwChecker(t, "waf", "alarm")(context.Background(), clients, api, alarms)
	for _, c := range w.callsFor("monitoring", "") {
		t.Errorf("regional ACL alarm lookup called %s@%s; its metrics are in the session's own list", c.Op, c.Region)
	}
}

// A bucket-Region lookup that failed is no answer: the next read asks again.
func TestRegionRow_FailedBucketLocationLookupIsAskedAgain(t *testing.T) {
	w := newRegionWorld()
	clients := w.clients("us-east-1")
	assets := resource.Resource{ID: rwAssetsBucket, Name: rwAssetsBucket, Fields: map[string]string{"name": rwAssetsBucket}}
	ctx := context.Background()

	w.denyAccess("s3", "us-east-1")
	if got := rwChecker(t, "s3", "s3")(ctx, clients, assets, resource.ResourceCache{}); got.EffectiveState() == domain.RelatedResolved {
		t.Fatalf("state = %v while the session endpoint refused every call, want an unresolved answer", got.EffectiveState())
	}

	w.mu.Lock()
	delete(w.deny, "s3@us-east-1")
	w.mu.Unlock()
	got := rwChecker(t, "s3", "s3")(ctx, clients, assets, resource.ResourceCache{})
	rwRequireResolved(t, got, "acme-access-logs-eu")
}

// A session has one set of capability stores however many Regions it reads:
// a rewire on the profile-switch rollback path has to reach the client sets
// a cross-Region read left behind.
func TestRegionRow_DerivedClientSetSharesTheSessionStores(t *testing.T) {
	w := newRegionWorld()
	clients := w.clients("eu-west-1")
	elsewhere := clients.InRegion("us-east-1")
	if elsewhere == clients {
		t.Fatal("InRegion returned the receiver for another Region")
	}

	store := session.NewIdentityStore()
	store.Set("123456789012", nil)
	clients.SetIdentityStore(store)
	if got := elsewhere.IdentityStore().AccountID(); got != "123456789012" {
		t.Errorf("the us-east-1 clients read account %q; a store set on the session reaches every Region it reads", got)
	}

	rewired := session.NewIdentityStore()
	rewired.Set("210987654321", nil)
	clients.SetIdentityStore(rewired)
	if got := elsewhere.IdentityStore().AccountID(); got != "210987654321" {
		t.Errorf("after a rewire the us-east-1 clients still read account %q, the pre-rewire one", got)
	}
}

// The demo has to exercise the path production takes for a CloudFront-scope
// ACL: AWS writes "global/webacl/" into its ARN, and reports the
// distributions it protects through CloudFront alone — its reference directs
// CloudFront callers away from wafv2:ListResourcesForWebACL, whose resource
// types are the regional ones.
func TestRegionRow_DemoCloudFrontACLIsAssociatedThroughCloudFront(t *testing.T) {
	clients := demo.NewServiceClients()
	rows := rwFetch(t, clients, "waf")

	var edge resource.Resource
	for _, r := range rows {
		if r.Fields["scope"] == "CLOUDFRONT" {
			edge = r
		}
	}
	if edge.ID == "" {
		t.Fatal("the demo lists no CLOUDFRONT-scope web ACL")
	}
	if !strings.Contains(edge.Fields["arn"], ":global/webacl/") {
		t.Errorf("CLOUDFRONT-scope ACL ARN = %q, want the global/webacl/ segment AWS writes for that scope", edge.Fields["arn"])
	}

	for aclArn, resources := range fixtures.NewWAFFixtures().ResourcesByWebACL {
		for _, r := range resources {
			if strings.HasPrefix(r, "arn:aws:cloudfront:") {
				t.Errorf("ListResourcesForWebACL answers %s for %s; a distribution is reported by ListDistributionsByWebACLId", r, aclArn)
			}
		}
	}

	got := rwChecker(t, "waf", "cf")(context.Background(), clients, edge, resource.ResourceCache{})
	if got.EffectiveState() != domain.RelatedResolved || got.Count() != 1 {
		t.Errorf("the CloudFront ACL protects state=%v count=%d, want the one distribution that carries it", got.EffectiveState(), got.Count())
	}
	for _, f := range rwCodes(rwFindingsOf(t, clients, rows, edge.ID)) {
		if f == "waf.orphan" {
			t.Error("the CloudFront ACL reads as attached to nothing while a distribution carries it")
		}
	}
}

// rwFindingsOf returns the wave-2 findings the WAF enricher raises for id.
func rwFindingsOf(t *testing.T, clients *awsclient.ServiceClients, rows []resource.Resource, id string) []domain.Finding {
	t.Helper()
	res, err := awsclient.EnrichWAFLogging(context.Background(), clients, rows, resource.ResourceCache{})
	if err != nil {
		t.Fatalf("EnrichWAFLogging: %v", err)
	}
	return res.Findings[id]
}
