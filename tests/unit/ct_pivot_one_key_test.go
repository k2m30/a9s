package unit_test

// A resource's CloudTrail events are reached three ways: the `t` hotkey
// (resource.BuildCloudTrailFilter, the one call every `t` site makes), the
// detail's "CloudTrail Events" related row, and the TARGET column an operator
// reads on the event list. All three must name the same events, and the name
// they agree on must be the one CloudTrail records: a LookupEvents ResourceName
// (or Username) lookup returns only events whose recorded value equals the
// attribute value, case-sensitively, never a prefix or a suffix
// (docs.aws.amazon.com/awscloudtrail/latest/userguide/view-cloudtrail-events-cli.html).

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cloudtrailtypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	unit "github.com/k2m30/a9s/v3/tests/unit"
)

func ctPivotDef(shortName string) (resource.RelatedDef, bool) {
	if shortName == "ct-events" {
		return resource.RelatedDef{}, false
	}
	for _, d := range resource.GetRelated(shortName) {
		if d.TargetType == "ct-events" {
			return d, true
		}
	}
	return resource.RelatedDef{}, false
}

// ctRecorded reports whether CloudTrail would return ev for filter: every
// LookupEvents attribute equals the recorded value exactly. Keys that are not
// a lookup attribute are ignored, except the qualifier: a lookup attribute
// names a resource whose name AWS scopes to a parent, and the event of a
// namesake under another parent is another row's.
func ctRecorded(ev resource.Resource, filter map[string]string) bool {
	raw, _ := ev.RawStruct.(cloudtrailtypes.Event)
	matchedAny := false
	for k, v := range filter {
		switch {
		case k == "ResourceName":
			if !slices.ContainsFunc(raw.Resources, func(r cloudtrailtypes.Resource) bool { return aws.ToString(r.ResourceName) == v }) {
				return false
			}
		case k == "Username":
			if aws.ToString(raw.Username) != v {
				return false
			}
		default:
			continue
		}
		matchedAny = true
	}
	return matchedAny && ctOfParent(raw, filter)
}

// ctOfParent is the client-side half of the pivot: the parent the row
// belongs to against the parent the event's body names.
func ctOfParent(raw cloudtrailtypes.Event, filter map[string]string) bool {
	parent := filter[resource.CTQualifierFilterKey]
	paths := filter[resource.CTQualifierPathsKey]
	if parent == "" || paths == "" {
		return true
	}
	var parsed map[string]any
	if err := json.Unmarshal([]byte(aws.ToString(raw.CloudTrailEvent)), &parsed); err != nil {
		return true
	}
	named := false
	for _, path := range strings.Split(paths, ",") {
		node := any(parsed)
		for _, key := range strings.Split(path, ".") {
			m, ok := node.(map[string]any)
			if !ok {
				node = nil
				break
			}
			node = m[key]
		}
		value, _ := node.(string)
		if value == "" {
			continue
		}
		named = true
		if value[strings.LastIndex(value, "/")+1:] == parent {
			return true
		}
	}
	return !named
}

func ctRecordedIDs(events []resource.Resource, filter map[string]string) []string {
	var ids []string
	for _, ev := range events {
		if ctRecorded(ev, filter) {
			ids = append(ids, ev.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

func sortedCopy(ids []string) []string {
	out := slices.Clone(ids)
	sort.Strings(out)
	return out
}

type ctBench struct {
	clients *awsclient.ServiceClients
	byType  map[string][]resource.Resource
	cache   resource.ResourceCache
	events  []resource.Resource
}

func newCTBench(t *testing.T) ctBench {
	t.Helper()
	byType, cache := buildVisibilityTypeCache(t)
	events := byType["ct-events"]
	if len(events) == 0 {
		t.Fatal("demo bench has no ct-events rows")
	}
	return ctBench{clients: demo.NewServiceClients(), byType: byType, cache: cache, events: events}
}

func ctPivotTypeNames() []string {
	var names []string
	for _, td := range resource.AllResourceTypes() {
		if _, ok := ctPivotDef(td.ShortName); ok {
			names = append(names, td.ShortName)
		}
	}
	sort.Strings(names)
	return names
}

// TestCTPivot_DBC_RelatedRowUsesTheTKeyFilter pins that a DocumentDB
// cluster's CloudTrail related row looks events up with the filter its `t`
// hotkey uses, not an identifier of its own.
func TestCTPivot_DBC_RelatedRowUsesTheTKeyFilter(t *testing.T) {
	b := newCTBench(t)
	def, ok := ctPivotDef("dbc")
	if !ok {
		t.Fatal("dbc has no CloudTrail Events related row")
	}
	idx := slices.IndexFunc(b.byType["dbc"], func(r resource.Resource) bool { return r.ID == fixtures.ProdDbcID })
	if idx < 0 {
		t.Fatalf("demo dbc row %q not found", fixtures.ProdDbcID)
	}
	row := b.byType["dbc"][idx]

	tFilter := resource.BuildCloudTrailFilter(row, "dbc")
	got := def.Checker(context.Background(), b.clients, row, b.cache)
	if !maps.Equal(got.FetchFilter(), tFilter) {
		t.Errorf("dbc %s: CloudTrail related row looks up %v, `t` looks up %v", row.ID, got.FetchFilter(), tFilter)
	}
}

// TestCTPivot_EveryPivotAnswersLikeTheSharedChecker is the class guard over
// the catalog: every type with a CloudTrail related row has a CloudTrailKey,
// its row drills with exactly the `t` filter, a counted row counts exactly the
// cached events CloudTrail would return for that filter, and every type
// answers the same cached page the same way.
func TestCTPivot_EveryPivotAnswersLikeTheSharedChecker(t *testing.T) {
	b := newCTBench(t)
	names := ctPivotTypeNames()
	if len(names) < 40 {
		t.Fatalf("only %d types carry a CloudTrail related row; the catalog registers ~60", len(names))
	}

	statesByType := map[domain.RelatedRowState][]string{}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			td := resource.FindResourceType(name)
			if td.CloudTrailKey == "" {
				t.Errorf("%s has a CloudTrail related row but no CloudTrailKey", name)
			}
			def, _ := ctPivotDef(name)
			rows := b.byType[name]
			if len(rows) == 0 {
				t.Skipf("%s has no demo rows", name)
			}
			var state domain.RelatedRowState
			stateSet := false
			for _, row := range rows {
				tFilter := resource.BuildCloudTrailFilter(row, name)
				got := def.Checker(context.Background(), b.clients, row, b.cache)
				if !maps.Equal(got.FetchFilter(), tFilter) {
					t.Errorf("%s %s: related row looks up %v, `t` looks up %v", name, row.ID, got.FetchFilter(), tFilter)
				}
				if tFilter == nil {
					continue
				}
				if got.State() == domain.RelatedResolved {
					want := ctRecordedIDs(b.events, tFilter)
					if have := sortedCopy(got.ResourceIDs()); !slices.Equal(have, want) {
						t.Errorf("%s %s: related row counts %v, CloudTrail records %v for %v", name, row.ID, have, want, tFilter)
					}
				}
				if !stateSet {
					state, stateSet = got.State(), true
				}
			}
			if stateSet {
				statesByType[state] = append(statesByType[state], name)
			}
		})
	}
	if len(statesByType) > 1 {
		var parts []string
		for st, types := range statesByType {
			parts = append(parts, fmt.Sprintf("%v: %v", st, types))
		}
		sort.Strings(parts)
		t.Errorf("CloudTrail related rows answer the same cached page in %d different ways, so they are not one checker:\n  %s",
			len(statesByType), strings.Join(parts, "\n  "))
	}
}

// TestCTPivot_DemoTKeyShowsExactlyWhatCloudTrailRecords pins the demo half of
// the verification: `t` in --demo lists exactly the events a real CloudTrail
// lookup with the same filter would return, so the demo cannot show events the
// related row does not count (or hide ones it does).
func TestCTPivot_DemoTKeyShowsExactlyWhatCloudTrailRecords(t *testing.T) {
	b := newCTBench(t)
	ct := resource.FindResourceType("ct-events")
	if ct == nil || ct.FilteredFetcher == nil {
		t.Fatal("ct-events has no filtered fetcher")
	}
	for _, name := range ctPivotTypeNames() {
		for _, row := range b.byType[name] {
			tFilter := resource.BuildCloudTrailFilter(row, name)
			if tFilter == nil {
				continue
			}
			shown := unit.DrainPages(t, "ct-events", func(token string) (resource.FetchResult, error) {
				return ct.FilteredFetcher(context.Background(), b.clients, tFilter, token)
			})
			var have []string
			for _, ev := range shown {
				have = append(have, ev.ID)
			}
			have = sortedCopy(have)
			if want := ctRecordedIDs(b.events, tFilter); !slices.Equal(have, want) {
				t.Errorf("%s %s: demo `t` (%v) lists %v, CloudTrail records %v", name, row.ID, tFilter, have, want)
			}
		}
	}
}

// ctRowIdentities are the spellings an event uses for row: its ID, its ARN.
func ctRowIdentities(row resource.Resource) []string {
	ids := []string{row.ID}
	if a := row.Fields["arn"]; a != "" && a != row.ID {
		ids = append(ids, a)
	}
	return ids
}

// ctNames reports whether s, a recorded resource name or a TARGET cell, names
// row: its ID, its ARN, the resource part of its ARN (what TARGET shows for a
// same-account ARN), or a full ARN whose last segment is its ID. Only a full
// ARN is split, so a slash-named secret never names a row called by its last
// path segment.
func ctNames(s string, row resource.Resource) bool {
	if s == "" {
		return false
	}
	if s == row.ID {
		return true
	}
	if a := row.Fields["arn"]; a != "" {
		if s == a {
			return true
		}
		if parts := strings.SplitN(a, ":", 6); len(parts) == 6 && s == parts[5] {
			return true
		}
	}
	if !strings.HasPrefix(s, "arn:") {
		return false
	}
	i := strings.LastIndexAny(s, "/:")
	return s[i+1:] == row.ID
}

type ctPivotRow struct {
	typ string
	row resource.Resource
}

// TestCTPivot_DemoEventsNamingARowAreFoundByItsKey pins that the name the
// TARGET column shows, and the resource name CloudTrail records, reach the
// named row's CloudTrail lookup: when an event names exactly one
// ResourceName-keyed demo row, that row's `t` filter returns the event and a
// counted related row counts it.
//
// A Username-keyed type is judged by the caller the event names, which is a
// different fact from the resource it acted on.
func TestCTPivot_DemoEventsNamingARowAreFoundByItsKey(t *testing.T) {
	b := newCTBench(t)

	var rows []ctPivotRow
	owners := map[string]int{}
	for _, name := range ctPivotTypeNames() {
		if !strings.HasPrefix(resource.FindResourceType(name).CloudTrailKey, "ResourceName:") {
			continue
		}
		for _, row := range b.byType[name] {
			rows = append(rows, ctPivotRow{typ: name, row: row})
			for _, id := range ctRowIdentities(row) {
				owners[id]++
			}
		}
	}
	unique := func(row resource.Resource) bool {
		for _, id := range ctRowIdentities(row) {
			if owners[id] > 1 {
				return false
			}
		}
		return true
	}

	c := newVisibilityListController(t, "ct-events")
	c.ApplyResourcesLoaded("ct-events", b.events, nil, false)
	body := c.Snapshot().Body.List
	if body == nil {
		t.Fatal("ct-events list rendered no body")
	}
	targetCol := slices.IndexFunc(body.Columns, func(col app.ColumnDef) bool { return col.Title == "TARGET" })
	if targetCol < 0 {
		t.Fatal("ct-events list has no TARGET column")
	}
	targetOf := map[string]string{}
	for _, r := range body.Rows {
		if targetCol < len(r.Cells) {
			targetOf[r.ResourceID] = r.Cells[targetCol]
		}
	}

	dbcWitness := false
	for _, ev := range b.events {
		raw, _ := ev.RawStruct.(cloudtrailtypes.Event)
		spellings := []string{targetOf[ev.ID]}
		for _, r := range raw.Resources {
			spellings = append(spellings, aws.ToString(r.ResourceName))
		}
		for _, pr := range rows {
			if !unique(pr.row) || !slices.ContainsFunc(spellings, func(s string) bool { return ctNames(s, pr.row) }) {
				continue
			}
			if pr.typ == "dbc" && pr.row.ID == fixtures.ProdDbcID && ev.ID == "evt-docdb-prod-modify-001" {
				dbcWitness = true
			}
			tFilter := resource.BuildCloudTrailFilter(pr.row, pr.typ)
			if !ctRecorded(ev, tFilter) {
				t.Errorf("event %s (TARGET %q, recorded %v) names %s %s, but its `t` filter %v does not return it",
					ev.ID, targetOf[ev.ID], spellings[1:], pr.typ, pr.row.ID, tFilter)
				continue
			}
			def, _ := ctPivotDef(pr.typ)
			got := def.Checker(context.Background(), b.clients, pr.row, b.cache)
			if got.State() == domain.RelatedResolved && !slices.Contains(got.ResourceIDs(), ev.ID) {
				t.Errorf("event %s names %s %s and its `t` returns it, but the CloudTrail related row counts %v",
					ev.ID, pr.typ, pr.row.ID, got.ResourceIDs())
			}
		}
	}
	if !dbcWitness {
		t.Errorf("the naming rule did not find demo event evt-docdb-prod-modify-001 naming dbc %s; the guard is not looking at the TARGET column", fixtures.ProdDbcID)
	}
}

// ctPrefixFake serves two ECS clusters whose names share a prefix, and the
// CloudTrail events recorded for each under both spellings CloudTrail uses
// (the cluster ARN and the bare name). LookupEvents filters like the service:
// exact, case-sensitive attribute equality.
type ctPrefixFake struct {
	events []cloudtrailtypes.Event
}

const ctPrefixARN = "arn:aws:ecs:us-east-1:123456789012:cluster/"

func (ctPrefixFake) ListClusters(_ context.Context, _ *ecs.ListClustersInput, _ ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	return &ecs.ListClustersOutput{ClusterArns: []string{ctPrefixARN + "prod", ctPrefixARN + "prod-batch"}}, nil
}

func (ctPrefixFake) DescribeClusters(_ context.Context, in *ecs.DescribeClustersInput, _ ...func(*ecs.Options)) (*ecs.DescribeClustersOutput, error) {
	out := &ecs.DescribeClustersOutput{}
	for _, arn := range in.Clusters {
		out.Clusters = append(out.Clusters, ecstypes.Cluster{
			ClusterArn:          aws.String(arn),
			ClusterName:         aws.String(strings.TrimPrefix(arn, ctPrefixARN)),
			Status:              aws.String("ACTIVE"),
			RunningTasksCount:   3,
			ActiveServicesCount: 2,
		})
	}
	return out, nil
}

func (f ctPrefixFake) LookupEvents(_ context.Context, in *cloudtrail.LookupEventsInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error) {
	out := &cloudtrail.LookupEventsOutput{}
	for _, ev := range f.events {
		keep := true
		for _, a := range in.LookupAttributes {
			v := aws.ToString(a.AttributeValue)
			switch a.AttributeKey {
			case cloudtrailtypes.LookupAttributeKeyResourceName:
				keep = keep && slices.ContainsFunc(ev.Resources, func(r cloudtrailtypes.Resource) bool { return aws.ToString(r.ResourceName) == v })
			case cloudtrailtypes.LookupAttributeKeyUsername:
				keep = keep && aws.ToString(ev.Username) == v
			}
		}
		if keep {
			out.Events = append(out.Events, ev)
		}
	}
	return out, nil
}

func (ctPrefixFake) DescribeTrails(_ context.Context, _ *cloudtrail.DescribeTrailsInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.DescribeTrailsOutput, error) {
	return &cloudtrail.DescribeTrailsOutput{}, nil
}

func (ctPrefixFake) GetTrailStatus(_ context.Context, _ *cloudtrail.GetTrailStatusInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.GetTrailStatusOutput, error) {
	return &cloudtrail.GetTrailStatusOutput{}, nil
}

func ctPrefixEvent(id, cluster, recorded string) cloudtrailtypes.Event {
	return cloudtrailtypes.Event{
		EventId:     aws.String(id),
		EventName:   aws.String("UpdateService"),
		EventSource: aws.String("ecs.amazonaws.com"),
		Username:    aws.String("ci-service-account"),
		ReadOnly:    aws.String("false"),
		Resources:   []cloudtrailtypes.Resource{{ResourceType: aws.String("AWS::ECS::Cluster"), ResourceName: aws.String(recorded)}},
		CloudTrailEvent: aws.String(`{"eventVersion":"1.08","userIdentity":{"type":"IAMUser","arn":"arn:aws:iam::123456789012:user/ci-service-account","accountId":"123456789012","userName":"ci-service-account"},` +
			`"eventSource":"ecs.amazonaws.com","eventName":"UpdateService","awsRegion":"us-east-1","recipientAccountId":"123456789012","eventType":"AwsApiCall","eventCategory":"Management",` +
			`"requestParameters":{"cluster":"` + cluster + `","service":"api","desiredCount":2}}`),
	}
}

// TestCTPivot_ECS_ClusterDoesNotCountItsPrefixSibling pins that cluster
// `prod` never counts the events of `prod-batch` (nor the reverse): whichever
// spelling the key picks, the `t` lookup and the related row reach only the
// cluster's own events.
func TestCTPivot_ECS_ClusterDoesNotCountItsPrefixSibling(t *testing.T) {
	fake := ctPrefixFake{events: []cloudtrailtypes.Event{
		ctPrefixEvent("evt-prod-arn", "prod", ctPrefixARN+"prod"),
		ctPrefixEvent("evt-prod-name", "prod", "prod"),
		ctPrefixEvent("evt-batch-arn", "prod-batch", ctPrefixARN+"prod-batch"),
		ctPrefixEvent("evt-batch-name", "prod-batch", "prod-batch"),
	}}
	ctx := context.Background()
	clusters, err := awsclient.FetchECSClustersPage(ctx, fake, fake, "")
	if err != nil || len(clusters.Resources) != 2 {
		t.Fatalf("fetch clusters: %v (%d rows)", err, len(clusters.Resources))
	}
	page, err := awsclient.FetchCloudTrailEventsPage(ctx, fake, "")
	if err != nil {
		t.Fatalf("fetch events: %v", err)
	}
	clients := &awsclient.ServiceClients{Region: "us-east-1", CloudTrail: fake}
	cache := resource.ResourceCache{
		"ecs":       {Resources: clusters.Resources},
		"ct-events": {Resources: page.Resources},
	}
	def, ok := ctPivotDef("ecs")
	if !ok {
		t.Fatal("ecs has no CloudTrail Events related row")
	}
	ct := resource.FindResourceType("ct-events")

	own := map[string][]string{
		"prod":       {"evt-prod-arn", "evt-prod-name"},
		"prod-batch": {"evt-batch-arn", "evt-batch-name"},
	}
	for _, row := range clusters.Resources {
		t.Run(row.ID, func(t *testing.T) {
			tFilter := resource.BuildCloudTrailFilter(row, "ecs")
			if v := tFilter["ResourceName"]; v != row.ID && v != ctPrefixARN+row.ID {
				t.Errorf("`t` looks up ResourceName %q, which is neither spelling CloudTrail records for cluster %s", v, row.ID)
			}

			shown, err := ct.FilteredFetcher(ctx, clients, tFilter, "")
			if err != nil {
				t.Fatalf("`t` lookup: %v", err)
			}
			var tIDs []string
			for _, ev := range shown.Resources {
				tIDs = append(tIDs, ev.ID)
			}
			if len(tIDs) == 0 {
				t.Errorf("`t` on %s finds none of its own events %v", row.ID, own[row.ID])
			}
			for _, id := range tIDs {
				if !slices.Contains(own[row.ID], id) {
					t.Errorf("`t` on %s lists %s, an event of another cluster", row.ID, id)
				}
			}

			got := def.Checker(ctx, clients, row, cache)
			if !maps.Equal(got.FetchFilter(), tFilter) {
				t.Errorf("related row looks up %v, `t` looks up %v", got.FetchFilter(), tFilter)
			}
			for _, id := range got.ResourceIDs() {
				if !slices.Contains(own[row.ID], id) {
					t.Errorf("related row of %s counts %s, an event of another cluster (counted %v)", row.ID, id, got.ResourceIDs())
				}
			}
			if got.State() == domain.RelatedResolved {
				if want := ctRecordedIDs(page.Resources, tFilter); !slices.Equal(sortedCopy(got.ResourceIDs()), want) {
					t.Errorf("related row of %s counts %v, CloudTrail records %v for %v", row.ID, sortedCopy(got.ResourceIDs()), want, tFilter)
				}
			}
		})
	}
}
