package unit_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudfront/types"

	"github.com/k2m30/a9s/v3/core/app"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/semantics/projection"

	"github.com/k2m30/a9s/v3/core/consolelink"
	"github.com/k2m30/a9s/v3/core/fieldpath"
)

type t9bDevInner struct {
	Note *string
}

type t9bDevStruct struct {
	Enabled  bool
	Count    int32
	Flag     *bool
	Missing  *string
	Empty    string
	Tags     []string
	When     time.Time
	Inner    t9bDevInner
	Score    float64
	Disabled *bool
}

func TestT9bDev_ToSafeValueKeepsStatedFalseAndZero(t *testing.T) {
	off := false
	got, ok := fieldpath.ToSafeValue(reflect.ValueOf(t9bDevStruct{Flag: &off, Score: 1e10})).(map[string]any)
	if !ok {
		t.Fatalf("ToSafeValue returned %T, want a map", got)
	}
	want := map[string]any{"Enabled": false, "Count": int64(0), "Flag": false, "Score": int64(10000000000)}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ToSafeValue = %#v, want %#v (nil pointers, empty strings, empty slices, a zero time and an all-nil struct dropped)", got, want)
	}
}

func TestT9bDev_ExtractFieldListTypedNilStructReadsFields(t *testing.T) {
	var none *t9bDevStruct
	items := fieldpath.ExtractFieldList(none, map[string]string{"count": "3"}, []string{"Count"}, nil, nil)
	if len(items) != 1 || items[0].Value != "3" {
		t.Errorf("a row whose struct is a nil pointer has no struct: items %+v, want Count=3 from Fields", items)
	}
	items = fieldpath.ExtractFieldList(&t9bDevStruct{Count: 7}, map[string]string{"count": "3"}, []string{"Count"}, nil, nil)
	if len(items) != 1 || items[0].Value != "7" {
		t.Errorf("a row with a struct reads it: items %+v, want Count=7", items)
	}
}

func TestT9bDev_RDSFamilyEngineCaseAndEmpty(t *testing.T) {
	const region = "eu-west-1"
	cases := []struct {
		engine string
		kind   consolelink.RDSKind
		want   string
	}{
		{"DocDB", consolelink.RDSInstance, "https://eu-west-1.console.aws.amazon.com/docdb/home?region=eu-west-1"},
		{"docdb", consolelink.RDSCluster, "https://eu-west-1.console.aws.amazon.com/docdb/home?region=eu-west-1#cluster-details/c%2F1"},
		{"neptune", consolelink.RDSClusterSnapshot, "https://eu-west-1.console.aws.amazon.com/neptune/home?region=eu-west-1"},
		{"aurora-postgresql", consolelink.RDSClusterSnapshot, "https://eu-west-1.console.aws.amazon.com/rds/home?region=eu-west-1#db-snapshot:id=c%2F1"},
		{"mysql", consolelink.RDSInstance, "https://eu-west-1.console.aws.amazon.com/rds/home?region=eu-west-1#database:id=c%2F1;is-cluster=false"},
		{"", consolelink.RDSInstance, "https://eu-west-1.console.aws.amazon.com/rds/home?region=eu-west-1#database:id=c%2F1;is-cluster=false"},
		{"", consolelink.RDSInstanceSnapshot, "https://eu-west-1.console.aws.amazon.com/rds/home?region=eu-west-1#db-snapshot:id=c%2F1"},
		{"", consolelink.RDSCluster, ""},
	}
	for _, c := range cases {
		if got := consolelink.RDSFamily(c.engine, c.kind, "c/1", region); got != c.want {
			t.Errorf("RDSFamily(%q, %d) = %q, want %q", c.engine, c.kind, got, c.want)
		}
	}
}

func TestT9bDev_NoStructLeafPathReadsItsColumnKey(t *testing.T) {
	items := fieldpath.ExtractFieldList(nil, map[string]string{"state": "running"}, []string{"State.Name"}, nil, map[string]string{"State.Name": "state"})
	if len(items) != 1 || items[0].Value != "running" {
		t.Errorf("a leaf path on a row with no struct reads the key its column declares: %+v", items)
	}
	items = fieldpath.ExtractFieldList(nil, map[string]string{"status": "OK", "name": "db-pass"}, []string{"Name"}, nil, map[string]string{"Name": "status"})
	if len(items) != 1 || items[0].Value != "db-pass" {
		t.Errorf("a top-level path reads its own key, not a column key holding another fact: %+v", items)
	}

	rows := t9bDemoRows(t)
	ec2 := t9bFindRow(t, rows, "ec2", "i-0a1b2c3d4e5f60001").res
	ec2.RawStruct = nil
	if got := t9bPathRows(t9bDetailRows(t, ec2, "ec2"), "State.Name"); len(got) != 1 || got[0] != "State=running" {
		t.Errorf("ec2 replayed from the cache: State rows %q, want [State=running]", got)
	}
}

type t9bDevAliases struct {
	Quantity *int32
	Items    []string
}

type t9bDevDoc struct {
	Port     int32
	Ratio    float64
	Small    float32
	Active   bool
	When     time.Time
	Aliases  *t9bDevAliases
	Origins  *t9bDevAliases
	Name     string
	Optional *int64
}

func TestT9bDev_YAMLTypesEachScalar(t *testing.T) {
	zero, two := int32(0), int32(2)
	doc := t9bDevDoc{
		Port: 5432, Ratio: 1234567.5, Small: 0.25, Active: true,
		When:    time.Date(2026, 1, 2, 3, 4, 5, 123000000, time.UTC),
		Aliases: &t9bDevAliases{Quantity: &zero},
		Origins: &t9bDevAliases{Quantity: &two, Items: []string{"a", "b"}},
		Name:    "16",
	}
	r := resource.Resource{ID: "x", RawStruct: doc}
	lines := app.ResourceTextLines(runtime.ScreenYAML, r)
	text := strings.Join(lines, "\n")
	for _, want := range []string{
		"Port: 5432", "Ratio: 1234567.5", "Small: 0.25", "Active: true",
		"When: 2026-01-02T03:04:05.123Z", `Name: "16"`, "Quantity: 2", "Quantity: 0",
	} {
		if !t9bHasLine(lines, want) && !strings.Contains(text, want) {
			t.Errorf("YAML lacks %q:\n%s", want, text)
		}
	}
	for _, absent := range []string{"Optional"} {
		if strings.Contains(text, absent) {
			t.Errorf("YAML shows %s, which AWS left unset:\n%s", absent, text)
		}
	}
}

// One detail screen renders a bool one way: a nested row reads Yes/No like a
// top-level row, and a nested timestamp reads as the day and time of day.
func TestT9bDev_DetailRendersNestedScalarsLikeTopLevel(t *testing.T) {
	nestedBools := 0
	for _, row := range t9bDemoRows(t) {
		if row.typ != "ec2" && row.typ != "ecs-svc" && row.typ != "dbc" {
			continue
		}
		for _, r := range t9bDetailRows(t, row.res, row.typ) {
			v := strings.TrimSpace(r.Value)
			if strings.HasSuffix(v, ": Yes") || strings.HasSuffix(v, ": No") {
				nestedBools++
			}
			if strings.HasSuffix(v, ": true") || strings.HasSuffix(v, ": false") || v == "true" || v == "false" {
				t.Errorf("%s %s detail row %q renders a bool as true/false; the detail screen reads Yes/No", row.typ, row.res.ID, r.Value)
			}
			if _, after, ok := strings.Cut(v, ": "); ok && len(after) >= 20 && after[10] == 'T' && strings.HasSuffix(after, "Z") {
				t.Errorf("%s %s detail row %q renders an RFC 3339 timestamp; the detail screen reads day and time", row.typ, row.res.ID, r.Value)
			}
		}
	}
	if nestedBools == 0 {
		t.Error("no nested bool row on the ec2, ecs-svc and dbc details; the check saw nothing")
	}
}

func TestT9bDev_CloneCopiesParent(t *testing.T) {
	r := resource.Resource{ID: "e", Parent: map[string]string{"engine": "docdb"}}
	c := r.Clone()
	c.Parent["engine"] = "mysql"
	if r.Parent["engine"] != "docdb" {
		t.Errorf("a clone's Parent aliases the original: %v", r.Parent)
	}
}

func TestT9bDev_NoDataTextScreenCopiesNothing(t *testing.T) {
	empty := resource.Resource{ID: "nothing", Type: "ec2"}
	e := newVisibilityDetailController(t)
	e.EnsureDetailState(empty, "ec2")
	e.Apply(app.Action{Kind: app.ActionOpenYAML})
	if got, _ := e.CopyContent(); got != "" {
		t.Errorf("a resource with no data copies %q from its YAML screen; the notice is not data", got)
	}
	if body := e.Snapshot().Body.Text; body == nil || body.Notice != "No YAML data available" || len(body.Lines) != 0 {
		t.Errorf("a resource with no data: text body %+v, want no lines and the YAML notice", body)
	}
}

func TestT9bDev_DetailDropsAZeroCountBesideItsEmptyList(t *testing.T) {
	dcb := &cftypes.DefaultCacheBehavior{
		LambdaFunctionAssociations: &cftypes.LambdaFunctionAssociations{Quantity: aws.Int32(0)},
		TrustedSigners:             &cftypes.TrustedSigners{Enabled: aws.Bool(false), Quantity: aws.Int32(0)},
		MinTTL:                     aws.Int64(0),
		ViewerProtocolPolicy:       cftypes.ViewerProtocolPolicyRedirectToHttps,
	}
	dist := cftypes.DistributionSummary{
		Id:                   aws.String("E1"),
		Aliases:              &cftypes.Aliases{Quantity: aws.Int32(0)},
		DefaultCacheBehavior: dcb,
	}
	text := map[string]string{}
	for _, it := range fieldpath.ExtractFieldList(dist, nil, []string{"Aliases", "DefaultCacheBehavior"}, nil, nil) {
		top, _, _ := strings.Cut(it.Path, ".")
		text[top] += it.Value + "\n"
	}
	if v := strings.TrimSpace(text["Aliases"]); v != "" && v != "-" {
		t.Errorf("Aliases with no alias shows %q; a zero count beside its empty list is the SDK's not set", v)
	}
	got := text["DefaultCacheBehavior"]
	if strings.Contains(got, "LambdaFunctionAssociations") {
		t.Errorf("DefaultCacheBehavior shows an empty association list as its zero count:\n%s", got)
	}
	for _, want := range []string{"MinTTL: 0", "Enabled: No", "ViewerProtocolPolicy: redirect-to-https"} {
		if !strings.Contains(got, want) {
			t.Errorf("DefaultCacheBehavior lacks %q, a zero or value AWS states:\n%s", want, got)
		}
	}
	if strings.Contains(got, "Quantity") {
		t.Errorf("DefaultCacheBehavior shows a zero count beside an empty list:\n%s", got)
	}

	yaml := strings.Join(app.ResourceTextLines(runtime.ScreenYAML, resource.Resource{ID: "E1", RawStruct: dist}), "\n")
	if !strings.Contains(yaml, "LambdaFunctionAssociations:") || !strings.Contains(yaml, "Quantity: 0") {
		t.Errorf("the YAML view names every key the JSON view shows, Quantity: 0 included:\n%s", yaml)
	}
}

func TestT9bDev_NoDataNoticeIsForTheYAMLAndJSONScreensOnly(t *testing.T) {
	for id, want := range map[runtime.ScreenID]string{
		runtime.ScreenYAML:     "No YAML data available",
		runtime.ScreenJSON:     "No JSON data available",
		runtime.ScreenErrorLog: "",
	} {
		if got := app.NoDataNotice(id); got != want {
			t.Errorf("an empty %s screen says %q, want %q", id, got, want)
		}
	}
}

type t9bDevPayload struct {
	Payload any
}

func TestT9bDev_OpeningADetailLeavesTheStructAsItWas(t *testing.T) {
	doc := t9bDevPayload{Payload: map[string]any{
		"Nested": map[string]any{"Quantity": int64(0)},
		"Flag":   true,
	}}
	fieldpath.ExtractFieldList(doc, nil, []string{"Payload"}, nil, nil)
	yaml := strings.Join(app.ResourceTextLines(runtime.ScreenYAML, resource.Resource{ID: "p", RawStruct: doc}), "\n")
	for _, want := range []string{"Quantity: 0", "Flag: true"} {
		if !strings.Contains(yaml, want) {
			t.Errorf("after the detail read Payload, the YAML lacks %q:\n%s", want, yaml)
		}
	}
}

func TestT9bDev_AStatedZeroCodeSizeIsShown(t *testing.T) {
	fn := t9bFindRow(t, t9bDemoRows(t), "lambda", "api-service-runner").res
	for _, r := range t9bDetailRows(t, fn, "lambda") {
		if r.Key == "CodeSize" && r.Value != "0 B" {
			t.Errorf("an image-packaged function's CodeSize reads %q; AWS states 0 bytes", r.Value)
		}
	}
}

func TestT9bDev_DemoNeptuneInstanceOpensTheNeptuneConsole(t *testing.T) {
	row := t9bFindRow(t, t9bDemoRows(t), "dbi", fixtures.DBINeptuneMember).res
	u, ok := consolelink.Resolve(*catalog.FindAny("dbi"), row, "us-east-1", t9bAccount)
	if want := "https://us-east-1.console.aws.amazon.com/neptune/home?region=us-east-1"; !ok || u != want {
		t.Errorf("the demo Neptune instance opens %q, want %q", u, want)
	}
	if row.Fields["engine"] != "neptune" {
		t.Errorf("the demo Neptune instance lists engine %q", row.Fields["engine"])
	}
}

func t9bDevStatusRow(t *testing.T, res resource.Resource, typ string) (string, bool) {
	t.Helper()
	for _, r := range t9bDetailRows(t, res, typ) {
		if r.Key == "Status" && !r.IsHeader && r.Path != "Attention" {
			return r.Value, true
		}
	}
	return "", false
}

func TestT9bDev_ReplayedRowWithAFindingShowsTheAWSStatus(t *testing.T) {
	types := map[string]bool{"redis": true, "dbc": true, "mwaa": true, "vpc-peer": true, "acm": true}
	checked := map[string]int{}
	for _, row := range t9bDemoRows(t) {
		if !types[row.typ] || row.res.RawStruct == nil || len(row.res.Findings) == 0 {
			continue
		}
		want, ok := t9bDevStatusRow(t, row.res, row.typ)
		if !ok {
			t.Errorf("%s %s: the detail has no Status row", row.typ, row.res.ID)
			continue
		}
		replayed := row.res
		replayed.RawStruct = nil
		got, _ := t9bDevStatusRow(t, replayed, row.typ)
		if got != want {
			top, _ := domain.TopFinding(row.res.Findings)
			t.Errorf("%s %s replayed from the cache: Status %q, the AWS value is %q (top finding %q)", row.typ, row.res.ID, got, want, top.Phrase)
		}
		checked[row.typ]++
	}
	for typ := range types {
		if checked[typ] == 0 {
			t.Errorf("%s: no demo row with a finding and a struct, so the replayed Status row is untested", typ)
		}
	}
}

type t9bDevJSONHolder struct {
	Rules    string
	Empty    string
	List     string
	Nested   t9bDevJSONInner
	Document any
}

type t9bDevJSONInner struct {
	Doc string
}

func TestT9bDev_AJSONDocumentKeepsItsEmptyContainers(t *testing.T) {
	doc := t9bDevJSONHolder{
		Rules:    `{"rules":[]}`,
		Empty:    `{}`,
		List:     `[]`,
		Nested:   t9bDevJSONInner{Doc: `{"Quantity":0,"tags":{}}`},
		Document: map[string]any{"Statement": []any{}, "Quantity": int64(0)},
	}
	detail := map[string]string{}
	for _, it := range fieldpath.ExtractFieldList(doc, nil, []string{"Rules", "Empty", "List", "Nested", "Document"}, nil, nil) {
		top, _, _ := strings.Cut(it.Path, ".")
		detail[top] += strings.TrimSpace(it.Value) + "\n"
	}
	for path, want := range map[string][]string{
		"Rules":    {"rules: []"},
		"Empty":    {"{}"},
		"List":     {"[]"},
		"Nested":   {"Quantity: 0", "tags: {}"},
		"Document": {"Statement: []", "Quantity: 0"},
	} {
		for _, w := range want {
			if !strings.Contains(detail[path], w) {
				t.Errorf("detail %s lacks %q; a JSON document is shown as written:\n%s", path, w, detail[path])
			}
		}
	}
	yaml := strings.Join(app.ResourceTextLines(runtime.ScreenYAML, resource.Resource{ID: "j", RawStruct: doc}), "\n")
	for _, w := range []string{"rules: []", "Empty: {}", "List: []", "tags: {}", "Statement: []"} {
		if !strings.Contains(yaml, w) {
			t.Errorf("YAML lacks %q:\n%s", w, yaml)
		}
	}
}

func TestT9bDev_KeyFormJSONExpandsLikePathForm(t *testing.T) {
	cfg := &config.ViewsConfig{Views: map[string]config.ViewDef{
		"t9bdevkeys": {Detail: []config.DetailField{{Key: "Policy"}, {Key: "name"}}},
	}}
	r := domain.Resource{ID: "k", Type: "t9bdevkeys", Fields: map[string]string{
		"Policy": `{"Version":"2012-10-17","Statement":[{"Effect":"Allow"}]}`,
		"name":   "acme",
	}}
	var got []string
	for _, sec := range projection.GenericWithConfigAndNavProvider(cfg, nil)(r) {
		for _, it := range sec.Items {
			got = append(got, fmt.Sprintf("%d|%s|%s", it.Kind, strings.TrimSpace(it.Label), strings.TrimSpace(it.Value)))
		}
	}
	joined := strings.Join(got, "\n")
	if len(got) < 4 || got[0] != fmt.Sprintf("%d|Policy|", domain.ItemHeader) {
		t.Fatalf("a key-form JSON object opens with its header row:\n%s", joined)
	}
	for _, want := range []string{"Version: \"2012-10-17\"", "Effect: Allow"} {
		if !strings.Contains(joined, want) {
			t.Errorf("the key-form Policy lacks the nested row %q:\n%s", want, joined)
		}
	}
	if last := got[len(got)-1]; last != fmt.Sprintf("%d|name|acme", domain.ItemField) {
		t.Errorf("the scalar key-form field keeps its row and its declared place last, got %q:\n%s", last, joined)
	}
}
