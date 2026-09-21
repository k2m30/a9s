package unit_test

// CloudTrail event history is per Region: LookupEvents returns only the events
// recorded in the Region the request is sent to. CloudFront, IAM and STS (and
// Route 53) record their events in us-east-1 whatever Region the operator is
// browsing, and a trail's own management calls land in its home Region. A
// lookup for such a row sent to the session Region answers "no events" for a
// resource that has them.
//
// These tests drive the real controller and executor against real SDK clients
// whose HTTP transport records the host every CloudTrail request goes to, so
// they pin where the lookup lands, not how the Region is threaded through.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/session"
	unit "github.com/k2m30/a9s/v3/tests/unit"
)

// ctRegionTransport answers CloudTrail LookupEvents like the service does:
// the Region named by homeHost holds eventJSON (first page carries a
// NextToken), every other Region holds nothing. Every non-CloudTrail call
// fails fast; the tests only look at where CloudTrail was asked.
type ctRegionTransport struct {
	homeHost  string
	eventJSON map[string]any

	mu    sync.Mutex
	hosts []string
}

func (tr *ctRegionTransport) Do(req *http.Request) (*http.Response, error) {
	if !strings.HasPrefix(req.URL.Host, "cloudtrail.") {
		return nil, errors.New("ctRegionTransport: only CloudTrail is served")
	}
	tr.mu.Lock()
	tr.hosts = append(tr.hosts, req.URL.Host)
	tr.mu.Unlock()

	var in struct{ NextToken string }
	if req.Body != nil {
		raw, _ := io.ReadAll(req.Body) //nolint:errcheck // a short read leaves NextToken empty, which is the first-page answer
		_ = json.Unmarshal(raw, &in)   //nolint:errcheck // same: an unparsable body is answered as a first page
	}
	out := map[string]any{"Events": []any{}}
	if req.URL.Host == tr.homeHost {
		out["Events"] = []any{tr.eventJSON}
		if in.NextToken == "" {
			out["NextToken"] = "page-2"
		}
	}
	body, err := json.Marshal(out)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     http.Header{"Content-Type": []string{"application/x-amz-json-1.1"}, "X-Amzn-Requestid": []string{"req-ct-region-0001"}},
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    req,
	}, nil
}

func (tr *ctRegionTransport) seen() []string {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return slices.Clone(tr.hosts)
}

// ctRegionEvent is a LookupEvents record naming value both as the resource
// and as the caller, so it answers a ResourceName and a Username lookup alike.
func ctRegionEvent(id, value string) map[string]any {
	record, _ := json.Marshal(map[string]any{ //nolint:errcheck // a map of strings always marshals
		"eventVersion":       "1.08",
		"userIdentity":       map[string]any{"type": "IAMUser", "arn": "arn:aws:iam::123456789012:user/" + value, "accountId": "123456789012", "userName": value},
		"eventTime":          "2026-09-01T10:00:00Z",
		"eventSource":        "route53.amazonaws.com",
		"eventName":          "ChangeResourceRecordSets",
		"awsRegion":          "us-east-1",
		"recipientAccountId": "123456789012",
		"eventType":          "AwsApiCall",
		"eventCategory":      "Management",
	})
	return map[string]any{
		"EventId":         id,
		"EventName":       "ChangeResourceRecordSets",
		"EventSource":     "route53.amazonaws.com",
		"EventTime":       1788256800,
		"ReadOnly":        "false",
		"Username":        value,
		"Resources":       []any{map[string]any{"ResourceType": "AWS::Route53::HostedZone", "ResourceName": value}},
		"CloudTrailEvent": string(record),
	}
}

// newCTRegionController builds a session browsing sessionRegion over real SDK
// clients that talk to tr.
func newCTRegionController(t *testing.T, sessionRegion string, tr *ctRegionTransport) (*runtime.Core, *app.Controller) {
	t.Helper()
	t.Setenv("A9S_CONFIG_FOLDER", t.TempDir())
	s := session.New()
	s.Profile = "example-readonly"
	s.Region = sessionRegion
	s.Clients = awsclient.CreateServiceClients(aws.Config{
		Region:      sessionRegion,
		Credentials: credentials.NewStaticCredentialsProvider("AKIAIOSFODNN7EXAMPLE", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", ""),
		HTTPClient:  tr,
		Retryer:     func() aws.Retryer { return aws.NopRetryer{} },
	})
	core := runtime.New(s, nil)
	return core, newBlessedController(t, core)
}

func runCTRegionTasks(t *testing.T, core *runtime.Core, c *app.Controller, tasks []runtime.TaskRequest) {
	t.Helper()
	for _, task := range tasks {
		ev, err := core.ExecuteTask(context.Background(), task)
		if err != nil || ev == nil {
			continue
		}
		c.Handle(ev)
	}
}

func ctRegionDemoRow(t *testing.T, shortName, id string) resource.Resource {
	t.Helper()
	td := resource.FindResourceType(shortName)
	if td == nil {
		t.Fatalf("type %q not registered", shortName)
	}
	rows, _ := unit.DrainFixtures(t, *td, demo.NewServiceClients())
	if len(rows) == 0 {
		t.Fatalf("no demo %s rows", shortName)
	}
	if id == "" {
		return rows[0]
	}
	for _, r := range rows {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("demo %s row %q not found", shortName, id)
	return resource.Resource{}
}

// ctRegionHomeTrailRow is the demo management trail re-homed to eu-central-1,
// in the shape the trail fetcher emits for such a trail.
func ctRegionHomeTrailRow(t *testing.T) resource.Resource {
	t.Helper()
	row := ctRegionDemoRow(t, "trail", "acme-management-trail")
	trail, ok := row.RawStruct.(cloudtrailtypes.Trail)
	if !ok {
		t.Fatalf("demo trail RawStruct is %T, want cloudtrailtypes.Trail", row.RawStruct)
	}
	trail.HomeRegion = aws.String("eu-central-1")
	trail.TrailARN = aws.String("arn:aws:cloudtrail:eu-central-1:123456789012:trail/acme-management-trail")
	row.RawStruct = trail
	fields := maps.Clone(row.Fields)
	fields["home_region"] = "eu-central-1"
	fields["trail_arn"] = aws.ToString(trail.TrailARN)
	row.Fields = fields
	return row
}

func ctHost(region string) string { return "cloudtrail." + region + ".amazonaws.com" }

func assertCTHosts(t *testing.T, what string, got []string, want string) {
	t.Helper()
	if len(got) == 0 {
		t.Fatalf("%s: no CloudTrail request was made", what)
	}
	for _, h := range got {
		if h != want {
			t.Errorf("%s: CloudTrail lookup went to %s, want %s (every lookup: %v)", what, h, want, got)
			return
		}
	}
}

// TestCTRegion_TKeyLooksUpWhereTheRowsEventsAreRecorded pins the `t` hotkey's
// lookup Region per type: a global service's own resources and a trail answer
// from the Region that records their events, a regional resource from the
// session Region. A lookup keyed on the caller rather than on a resource
// answers from the session Region too: a principal's calls are recorded where
// they were made, whatever service the principal belongs to.
func TestCTRegion_TKeyLooksUpWhereTheRowsEventsAreRecorded(t *testing.T) {
	cases := []struct {
		name          string
		shortName     string
		row           func(t *testing.T) resource.Resource
		sessionRegion string
		wantRegion    string
	}{
		{"route53 zone", "r53", func(t *testing.T) resource.Resource { return ctRegionDemoRow(t, "r53", "") }, "eu-west-1", "us-east-1"},
		{"cloudfront distribution", "cf", func(t *testing.T) resource.Resource { return ctRegionDemoRow(t, "cf", "") }, "eu-west-1", "us-east-1"},
		{"iam user", "iam-user", func(t *testing.T) resource.Resource { return ctRegionDemoRow(t, "iam-user", "") }, "eu-west-1", "eu-west-1"},
		{"iam role", "role", func(t *testing.T) resource.Resource { return ctRegionDemoRow(t, "role", "") }, "eu-west-1", "us-east-1"},
		{"iam policy", "policy", func(t *testing.T) resource.Resource { return ctRegionDemoRow(t, "policy", "") }, "eu-west-1", "us-east-1"},
		{"iam group", "iam-group", func(t *testing.T) resource.Resource { return ctRegionDemoRow(t, "iam-group", "") }, "eu-west-1", "us-east-1"},
		{"trail homed in us-east-1", "trail", func(t *testing.T) resource.Resource { return ctRegionDemoRow(t, "trail", "acme-management-trail") }, "eu-west-1", "us-east-1"},
		{"trail homed in eu-central-1", "trail", ctRegionHomeTrailRow, "us-east-1", "eu-central-1"},
		{"regional ec2 instance", "ec2", func(t *testing.T) resource.Resource { return ctRegionDemoRow(t, "ec2", "") }, "eu-west-1", "eu-west-1"},
		{"regional docdb cluster", "dbc", func(t *testing.T) resource.Resource { return ctRegionDemoRow(t, "dbc", "") }, "eu-west-1", "eu-west-1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			row := tc.row(t)
			tr := &ctRegionTransport{homeHost: ctHost(tc.wantRegion), eventJSON: ctRegionEvent("evt-region-0001", row.ID)}
			core, c := newCTRegionController(t, tc.sessionRegion, tr)
			c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
			c.EnsureDetailState(row, tc.shortName)

			_, tasks := c.Apply(app.Action{Kind: app.ActionCloudTrail})
			if len(tasks) == 0 {
				t.Fatalf("`t` on %s %q dispatched no task", tc.shortName, row.ID)
			}
			runCTRegionTasks(t, core, c, tasks)
			assertCTHosts(t, "`t` on "+tc.shortName+" in "+tc.sessionRegion, tr.seen(), ctHost(tc.wantRegion))
		})
	}
}

// TestCTRegion_Route53TKeyListShowsUSEast1EventsAcrossPagesAndRefresh pins
// that a zone's `t` list, opened outside us-east-1, shows its us-east-1
// events, and that load-more and Ctrl+R on that list ask us-east-1 too: every
// page of the row's event list is a lookup for that row.
func TestCTRegion_Route53TKeyListShowsUSEast1EventsAcrossPagesAndRefresh(t *testing.T) {
	zone := ctRegionDemoRow(t, "r53", "")
	tr := &ctRegionTransport{homeHost: ctHost("us-east-1"), eventJSON: ctRegionEvent("evt-r53-change-0001", zone.ID)}
	core, c := newCTRegionController(t, "eu-west-1", tr)
	c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail}})
	c.EnsureDetailState(zone, "r53")

	_, tasks := c.Apply(app.Action{Kind: app.ActionCloudTrail})
	runCTRegionTasks(t, core, c, tasks)
	assertCTHosts(t, "first page", tr.seen(), ctHost("us-east-1"))

	list := c.Snapshot().Body.List
	if list == nil {
		t.Fatal("`t` on the zone opened no list")
	}
	if !slices.ContainsFunc(list.Rows, func(r app.ListRow) bool { return r.ResourceID == "evt-r53-change-0001" }) {
		t.Fatalf("the zone's CloudTrail list does not show its us-east-1 event evt-r53-change-0001; rows: %d", len(list.Rows))
	}

	before := len(tr.seen())
	_, more := c.Apply(app.Action{Kind: app.ActionLoadMore})
	if len(more) == 0 {
		t.Fatal("load-more on the truncated zone event list dispatched no task")
	}
	runCTRegionTasks(t, core, c, more)
	assertCTHosts(t, "load-more page", tr.seen()[before:], ctHost("us-east-1"))

	before = len(tr.seen())
	_, refresh := c.Apply(app.Action{Kind: app.ActionRefresh})
	if len(refresh) == 0 {
		t.Fatal("Ctrl+R on the zone event list dispatched no task")
	}
	runCTRegionTasks(t, core, c, refresh)
	assertCTHosts(t, "refresh", tr.seen()[before:], ctHost("us-east-1"))
}

// TestCTRegion_Route53RelatedRowLooksUpInUSEast1 drives the detail's
// CloudTrail related row the way the operator does: open the zone, let the
// related checks answer, press Enter on the row. Neither the check nor the
// drill may ask the session Region, and the drilled list shows the zone's
// event.
func TestCTRegion_Route53RelatedRowLooksUpInUSEast1(t *testing.T) {
	zone := ctRegionDemoRow(t, "r53", "")
	tr := &ctRegionTransport{homeHost: ctHost("us-east-1"), eventJSON: ctRegionEvent("evt-r53-change-0002", zone.ID)}
	core, c := newCTRegionController(t, "eu-west-1", tr)
	c.Apply(app.Action{Kind: app.ActionCommand, Arg: "r53"})
	c.ApplyResourcesLoaded("r53", []resource.Resource{zone}, nil, false)

	_, openTasks := c.Apply(app.Action{Kind: app.ActionSelect})
	var checks []runtime.TaskRequest
	for _, task := range openTasks {
		if task.Key.Kind == runtime.KindRelatedCheck {
			checks = append(checks, task)
		}
	}
	if len(checks) == 0 {
		t.Fatalf("opening the zone dispatched no related check; tasks: %+v", openTasks)
	}
	runCTRegionTasks(t, core, c, checks)

	detail := c.Snapshot().Body.Detail
	if detail == nil {
		t.Fatal("opening the zone produced no detail body")
	}
	idx := slices.IndexFunc(detail.Related, func(r app.RelatedBlock) bool { return r.Name == "CloudTrail Events" })
	if idx < 0 {
		t.Fatalf("the zone detail has no CloudTrail Events row; related: %+v", detail.Related)
	}
	if got := detail.Related[idx].CountDisplay; got == "(0)" {
		t.Errorf("the zone's CloudTrail row reads %q while us-east-1 holds its event", got)
	}

	c.Apply(app.Action{Kind: app.ActionToggleFocus})
	_, drill := c.Apply(app.Action{Kind: app.ActionRelatedSelect, Arg: strconv.Itoa(idx)})
	runCTRegionTasks(t, core, c, drill)
	assertCTHosts(t, "related check and drill", tr.seen(), ctHost("us-east-1"))

	list := c.Snapshot().Body.List
	if list == nil || !slices.ContainsFunc(list.Rows, func(r app.ListRow) bool { return r.ResourceID == "evt-r53-change-0002" }) {
		t.Errorf("the drilled CloudTrail list does not show the zone's us-east-1 event evt-r53-change-0002")
	}
}
