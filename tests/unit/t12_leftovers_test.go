package unit_test

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbtypes "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
	"github.com/aws/aws-sdk-go-v2/service/glue"
	gluetypes "github.com/aws/aws-sdk-go-v2/service/glue/types"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
	"golang.org/x/tools/go/packages"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/demo"
	demofixtures "github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/runtime"
	"github.com/k2m30/a9s/v3/core/runtime/messages"
	"github.com/k2m30/a9s/v3/core/session"
	"github.com/k2m30/a9s/v3/tests/unit"
)

func t12Raw[T any](v any) (T, bool) {
	switch x := v.(type) {
	case T:
		return x, true
	case *T:
		if x != nil {
			return *x, true
		}
	}
	var zero T
	return zero, false
}

// Every row a demo fetcher returns, and every row a child fetcher returns on
// any screen a drill reaches, carries the type it is registered under.
func TestT12_DemoRowsCarryTheTypeTheyAreRegisteredUnder(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()
	wrong := map[string]string{}
	checked := 0
	var walk func(td resource.ResourceTypeDef, rows []resource.Resource, parentCtx map[string]string, depth int)
	walk = func(td resource.ResourceTypeDef, rows []resource.Resource, parentCtx map[string]string, depth int) {
		for _, r := range rows {
			checked++
			if _, seen := wrong[td.ShortName]; !seen && r.Type != td.ShortName {
				wrong[td.ShortName] = fmt.Sprintf("%s/%s carries Type %q", td.ShortName, r.ID, r.Type)
			}
		}
		if depth > 5 {
			return
		}
		for _, child := range td.Children {
			ctd := resource.GetChildType(child.ChildType)
			if ctd == nil || ctd.ChildFetcher == nil {
				continue
			}
			for i := range rows {
				dr := domain.Resource(rows[i])
				pctx := resource.ResolveChildContext(child, &dr, parentCtx)
				kids, err := unit.CollectAllPages(func(token string) (resource.FetchResult, error) {
					return ctd.ChildFetcher(context.Background(), clients, pctx, token)
				})
				if err == nil && len(kids) > 0 {
					walk(*ctd, kids, pctx, depth+1)
				}
			}
		}
	}
	for _, td := range resource.AllResourceTypes() {
		walk(td, byType[td.ShortName], nil, 0)
	}
	if checked < 1000 {
		t.Fatalf("checked %d rows; the bench lost its rows", checked)
	}
	if len(wrong) > 0 {
		list := slices.Sorted(maps.Values(wrong))
		t.Errorf("%d types return rows without their own type:\n  %s", len(list), strings.Join(list, "\n  "))
	}
}

var t12Doors = []string{"emitWave1", "withWave1", "rebuiltWave1", "addWave1Finding", "addSecretScanFinding", "keepApplicable", "keepApplicableRows"}

func t12LoadCoreRuntime(t *testing.T) []*packages.Package {
	t.Helper()
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo, Dir: filepath.Join("..", "..")}
	pkgs, err := unit.LoadPackagesOnce(cfg, "./core/runtime")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	for _, p := range pkgs {
		for _, e := range p.Errors {
			t.Fatalf("%s: %v", p.PkgPath, e)
		}
	}
	return pkgs
}

// A door reads the type from the row it judges, so none takes the type as an
// argument, and its checks hold in every binary: a door that verifies only
// under testing.Testing() lets a mis-typed row stand unjudged in the shipped
// one.
func TestT12Guard_DoorsTakeNoTypeAndCheckInEveryBinary(t *testing.T) {
	found := map[string]bool{}
	var sites []string
	for _, p := range slices.Concat(t2LoadCoreAWS(t), t12LoadCoreRuntime(t)) {
		for _, f := range p.Syntax {
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Recv != nil || !slices.Contains(t12Doors, fd.Name.Name) {
					continue
				}
				found[fd.Name.Name] = true
				for _, field := range fd.Type.Params.List {
					if id, ok := field.Type.(*ast.Ident); ok && id.Name == "string" {
						names := make([]string, 0, len(field.Names))
						for _, n := range field.Names {
							names = append(names, n.Name)
						}
						sites = append(sites, fmt.Sprintf("%s %s(%s string)", t2Site(p, fd.Pos()), fd.Name.Name, strings.Join(names, ", ")))
					}
				}
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					sel, ok := n.(*ast.SelectorExpr)
					if !ok || sel.Sel.Name != "Testing" {
						return true
					}
					if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "testing" {
						sites = append(sites, fmt.Sprintf("%s %s checks only under testing.Testing()", t2Site(p, sel.Pos()), fd.Name.Name))
					}
					return true
				})
			}
		}
	}
	for _, name := range []string{"emitWave1", "keepApplicable"} {
		if !found[name] {
			t.Errorf("no function %s in core/aws or core/runtime; the scan lost the door", name)
		}
	}
	slices.Sort(sites)
	if len(sites) > 0 {
		t.Errorf("%d door sites take a type or check only under test:\n  %s", len(sites), strings.Join(sites, "\n  "))
	}
}

// democount prints the count the list title's own rule produces; a second
// statement of when a count is a lower bound in the command would drift from
// the title the first time either changes.
func TestT12Guard_DemocountStatesNoLowerBoundRuleOfItsOwn(t *testing.T) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filepath.Join("..", "..", "cmd", "democount", "main.go"), nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse cmd/democount/main.go: %v", err)
	}
	var sites []string
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if x.Sel.Name == "IsTruncated" || x.Sel.Name == "HasPagination" {
				sites = append(sites, fmt.Sprintf("line %d reads %s", fset.Position(x.Pos()).Line, x.Sel.Name))
			}
		case *ast.BasicLit:
			if x.Kind == token.STRING && strings.Contains(x.Value, "+") {
				sites = append(sites, fmt.Sprintf("line %d spells %s", fset.Position(x.Pos()).Line, x.Value))
			}
		}
		return true
	})
	if len(sites) > 0 {
		t.Errorf("cmd/democount decides the lower-bound marker itself:\n  %s", strings.Join(sites, "\n  "))
	}
}

func t12LogGroup(name string, class cwlogstypes.LogGroupClass, retention *int32) cwlogstypes.LogGroup {
	return cwlogstypes.LogGroup{
		LogGroupName:    aws.String(name),
		Arn:             aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + name + ":*"),
		LogGroupArn:     aws.String("arn:aws:logs:us-east-1:123456789012:log-group:" + name),
		CreationTime:    aws.Int64(1735689600000),
		RetentionInDays: retention,
		StoredBytes:     aws.Int64(1_048_576),
		KmsKeyId:        aws.String("arn:aws:kms:us-east-1:123456789012:key/1234abcd-12ab-34cd-56ef-1234567890ab"),
		LogGroupClass:   class,
	}
}

func t12StatesAWSFixedRetention(v string) bool {
	return strings.Contains(v, "AWS") && !strings.Contains(strings.ToLower(v), "never")
}

// AWS keeps the events of a Delivery-class group for a period it sets and the
// customer cannot change, and DescribeLogGroups reports no retention for such
// a group; no other class reads its missing retention that way. The Retention
// value says the period is AWS's, never "never expire".
func TestT12_DeliveryClassRetentionIsFixedByAWS(t *testing.T) {
	fake := &t2LogsFake{groups: []cwlogstypes.LogGroup{
		t12LogGroup("/aws/vendedlogs/acme-delivery", cwlogstypes.LogGroupClassDelivery, nil),
		t12LogGroup("/acme/app/standard", cwlogstypes.LogGroupClassStandard, nil),
		t12LogGroup("/acme/app/unclassed", "", nil),
		t12LogGroup("/acme/app/infrequent", cwlogstypes.LogGroupClassInfrequentAccess, nil),
		t12LogGroup("/acme/app/kept-30", cwlogstypes.LogGroupClassStandard, aws.Int32(30)),
	}}
	page, err := awsclient.FetchCloudWatchLogGroupsPage(context.Background(), fake, "")
	if err != nil {
		t.Fatalf("FetchCloudWatchLogGroupsPage: %v", err)
	}
	got := map[string]string{}
	for _, r := range page.Resources {
		got[r.ID] = r.Fields["retention"]
	}
	if v := got["/aws/vendedlogs/acme-delivery"]; !t12StatesAWSFixedRetention(v) {
		t.Errorf("Delivery group retention = %q, want a value saying AWS fixes it", v)
	}
	for _, id := range []string{"/acme/app/standard", "/acme/app/unclassed", "/acme/app/infrequent"} {
		if got[id] != "never expire" {
			t.Errorf("%s with no retention policy reads %q, want %q", id, got[id], "never expire")
		}
	}
	if got["/acme/app/kept-30"] != "30 days" {
		t.Errorf("a 30-day group reads %q, want %q", got["/acme/app/kept-30"], "30 days")
	}

	byType, _ := buildVisibilityTypeCache(t)
	td := resource.FindResourceType("logs")
	rows := byType["logs"]
	cells := t12ListColumnCells(t, *td, rows, "Retention")
	sawDelivery, sawNeverExpire := false, false
	for _, r := range rows {
		lg, ok := t12Raw[cwlogstypes.LogGroup](r.RawStruct)
		if !ok {
			continue
		}
		switch {
		case lg.LogGroupClass == cwlogstypes.LogGroupClassDelivery:
			sawDelivery = true
			if !t12StatesAWSFixedRetention(cells[r.ID]) {
				t.Errorf("demo Delivery group %s: list Retention cell = %q, want a value saying AWS fixes it", r.ID, cells[r.ID])
			}
			rowsSeen := 0
			for _, f := range t12DetailBody(t, r, "logs").Fields {
				if f.Path == "Attention" || !strings.Contains(strings.ToLower(f.Key), "retention") {
					continue
				}
				rowsSeen++
				if !t12StatesAWSFixedRetention(f.Value) {
					t.Errorf("demo Delivery group %s: detail %s = %q, want a value saying AWS fixes it", r.ID, f.Key, f.Value)
				}
			}
			if rowsSeen == 0 {
				t.Errorf("demo Delivery group %s: the detail shows no Retention row", r.ID)
			}
		case lg.RetentionInDays == nil && lg.LogGroupClass != cwlogstypes.LogGroupClassDelivery:
			sawNeverExpire = true
			if cells[r.ID] != "never expire" {
				t.Errorf("demo %s group %s with no retention policy: list Retention cell = %q, want %q", lg.LogGroupClass, r.ID, cells[r.ID], "never expire")
			}
		}
	}
	if !sawDelivery || !sawNeverExpire {
		t.Errorf("demo bench lacks a witness: Delivery group %v, never-expiring group of another class %v", sawDelivery, sawNeverExpire)
	}
}

func t12ListColumnCells(t *testing.T, td resource.ResourceTypeDef, rows []resource.Resource, title string) map[string]string {
	t.Helper()
	c := newVisibilityListController(t, td.ShortName)
	c.ApplyResourcesLoaded(td.ShortName, rows, nil, false)
	body := c.Snapshot().Body.List
	if body == nil {
		t.Fatalf("%s: no list body", td.ShortName)
	}
	col := slices.IndexFunc(body.Columns, func(cd app.ColumnDef) bool { return cd.Title == title })
	if col < 0 {
		t.Fatalf("%s list has no %q column", td.ShortName, title)
	}
	out := map[string]string{}
	for _, row := range body.Rows {
		if col < len(row.Cells) {
			out[row.ResourceID] = row.Cells[col]
		}
	}
	return out
}

func t12DetailBody(t *testing.T, r resource.Resource, short string) app.DetailBody {
	t.Helper()
	c := newVisibilityDetailController(t)
	c.EnsureDetailState(r, short)
	body := c.Snapshot().Body.Detail
	if body == nil {
		t.Fatalf("%s/%s: no detail body", short, r.ID)
	}
	return *body
}

// A multi-Region replica key takes its rotation from its primary, so the demo
// replica that is still being created is a replica as KMS reports one and
// carries no rotation finding; the detail of every multi-Region key names its
// key type, and a single-Region key's detail names none.
func TestT12_KMSDetailShowsTheMultiRegionKeyType(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	td := resource.FindResourceType("kms")
	rows := mergeWave2Findings(t, *td, byType["kms"], cache, demo.NewServiceClients())
	sawCreatingReplica, sawMulti, sawSingle := false, false, false
	for _, r := range rows {
		km, ok := t12Raw[kmstypes.KeyMetadata](r.RawStruct)
		if !ok {
			continue
		}
		if r.ID == demofixtures.KMSCreatingReplicaKeyID {
			sawCreatingReplica = true
			if km.MultiRegion == nil || !*km.MultiRegion || km.MultiRegionConfiguration == nil || km.MultiRegionConfiguration.MultiRegionKeyType != kmstypes.MultiRegionKeyTypeReplica {
				t.Errorf("demo key %s is described as a replica but KMS would report MultiRegion=%v MultiRegionConfiguration=%+v", r.ID, km.MultiRegion, km.MultiRegionConfiguration)
			}
			if t1aHas(r.Findings, "kms.rotation-disabled") {
				t.Errorf("demo replica %s carries kms.rotation-disabled; a replica rotates with its primary", r.ID)
			}
		}
		var keyTypeRows []string
		for _, f := range t12DetailBody(t, r, "kms").Fields {
			if f.Path == "Attention" {
				continue
			}
			v := strings.TrimSpace(t563ParenAside.ReplaceAllString(f.Value, ""))
			if strings.EqualFold(v, string(kmstypes.MultiRegionKeyTypePrimary)) || strings.EqualFold(v, string(kmstypes.MultiRegionKeyTypeReplica)) {
				keyTypeRows = append(keyTypeRows, f.Key+"="+f.Value)
			}
		}
		if km.MultiRegionConfiguration == nil {
			sawSingle = true
			if len(keyTypeRows) > 0 {
				t.Errorf("single-Region key %s: detail shows a multi-Region key type %v", r.ID, keyTypeRows)
			}
			continue
		}
		sawMulti = true
		want := string(km.MultiRegionConfiguration.MultiRegionKeyType)
		if len(keyTypeRows) != 1 || !strings.Contains(strings.ToUpper(keyTypeRows[0]), want) {
			t.Errorf("multi-Region key %s (%s): detail key-type rows %v, want one naming %s", r.ID, want, keyTypeRows, want)
		}
	}
	if !sawCreatingReplica || !sawMulti || !sawSingle {
		t.Errorf("demo bench lacks a witness: creating replica %v, multi-Region key %v, single-Region key %v", sawCreatingReplica, sawMulti, sawSingle)
	}
}

var t12Stacked = regexp.MustCompile(`^(.*) \(\+(\d+)\)$`)

// t12AttentionEntries returns the phrase of every entry the detail's
// Attention block renders, lower-cased, with the not-inspected entry apart.
// An entry's first line is its severity glyph and its phrase.
func t12AttentionEntries(body *app.DetailBody) (phrases []string, notInspected bool) {
	if body == nil {
		return nil, false
	}
	for _, f := range body.Fields {
		if f.Path != "Attention" || f.IsSection || f.IsSpacer || f.IndentLevel != 1 {
			continue
		}
		rest, ok := strings.CutPrefix(f.Value, "! ")
		if !ok {
			if rest, ok = strings.CutPrefix(f.Value, "~ "); !ok {
				continue
			}
		}
		rest = strings.ToLower(rest)
		if strings.HasPrefix(rest, domain.NotInspectedPhrase) {
			notInspected = true
			continue
		}
		phrases = append(phrases, rest)
	}
	return phrases, notInspected
}

// The list's Status cell and the detail's Attention block state one set: the
// cell leads with a phrase the block lists and counts the rest, a cell with
// no phrase has a block with no finding, and the cell reads "not inspected"
// exactly when the block carries the not-inspected entry. A neutral (dim)
// state names the row's lifecycle and is no Attention entry.
func TestT12_ListCellAndDetailAttentionStateTheSameFindings(t *testing.T) {
	byType, cache := buildVisibilityTypeCache(t)
	clients := demo.NewServiceClients()
	var bad []string
	checked, uninspected := 0, 0
	for _, td := range resource.AllResourceTypes() {
		rows := byType[td.ShortName]
		if len(rows) == 0 {
			continue
		}
		c, core := newTestControllerAndCore(t)
		c.SetViewConfig(config.DefaultConfig())
		c.Apply(app.Action{Kind: app.ActionCommand, Arg: td.ShortName})
		c.ApplyResourcesLoaded(td.ShortName, rows, nil, false)
		core.ObserveRows(td.ShortName, rows, nil, session.OriginFetch, false)
		folded := slices.Clone(rows)
		if _, ok := awsclient.Wave2EnricherFor(td.ShortName); ok {
			res := unit.Wave2Result(t, td.ShortName, clients, rows, cache)
			intents, _ := core.HandleEvent(messages.EnrichmentChecked{
				ResourceType:     td.ShortName,
				Findings:         res.Findings,
				AttentionDetails: res.AttentionDetails,
				FieldUpdates:     res.FieldUpdates,
				TruncatedIDs:     res.TruncatedIDs,
			})
			c.ApplyIntents(intents)
			for i := range folded {
				runtime.ApplyWave2ToRow(&folded[i], td, res.Findings, res.AttentionDetails)
			}
		}
		body := c.Snapshot().Body.List
		if body == nil || body.StatusCol < 0 {
			continue
		}
		cells := map[string]string{}
		for _, row := range body.Rows {
			if _, dup := cells[row.ResourceID]; !dup && body.StatusCol < len(row.Cells) {
				cells[row.ResourceID] = row.Cells[body.StatusCol]
			}
		}
		for _, r := range folded {
			cell, ok := cells[r.ID]
			if !ok {
				continue
			}
			checked++
			c.ApplyIntents([]runtime.UIIntent{runtime.PushScreen{ID: runtime.ScreenDetail, Context: runtime.ScreenContext{ResourceType: td.ShortName, ResourceID: r.ID}}})
			c.EnsureDetailState(r, td.ShortName)
			phrases, marked := t12AttentionEntries(c.Snapshot().Body.Detail)
			c.ApplyIntents([]runtime.UIIntent{runtime.PopScreen{}})

			where := fmt.Sprintf("%s/%s: list %q, detail %v (not inspected %v)", td.ShortName, r.ID, cell, phrases, marked)
			top, rest := cell, 0
			if m := t12Stacked.FindStringSubmatch(cell); m != nil {
				top = m[1]
				rest, _ = strconv.Atoi(m[2])
			}
			if slices.ContainsFunc(r.Findings, func(f domain.Finding) bool { return f.Phrase == top && !f.Severity.IsIssue() }) {
				continue
			}
			top = strings.ToLower(top)
			switch {
			case cell == domain.NotInspectedPhrase:
				uninspected++
				if !marked || len(phrases) > 0 {
					bad = append(bad, where)
				}
			case len(phrases) == 0:
				if marked || slices.ContainsFunc(r.Findings, func(f domain.Finding) bool { return strings.ToLower(f.Phrase) == top }) {
					bad = append(bad, where)
				}
			default:
				if !slices.Contains(phrases, top) || len(phrases) != rest+1 {
					bad = append(bad, where)
				}
			}
		}
	}
	if checked < 1000 || uninspected == 0 {
		t.Fatalf("checked %d rows, %d uninspected; the sweep lost its rows", checked, uninspected)
	}
	if len(bad) > 0 {
		t.Errorf("%d demo rows state different findings in the list and the detail:\n  %s", len(bad), strings.Join(bad, "\n  "))
	}
}

// A stopped task has no health left to judge: the demo's stopped task whose
// last health status was UNHEALTHY shows its stopped state and nothing about
// the check, while a running unhealthy task still reports its health.
func TestT12Demo_StoppedUnhealthyTaskShowsItsStateOnly(t *testing.T) {
	td, rows := t563Merged(t, "ecs-task")
	sawStopped, sawRunning := false, false
	for _, r := range rows {
		task, ok := t12Raw[ecstypes.Task](r.RawStruct)
		if !ok || task.HealthStatus != ecstypes.HealthStatusUnhealthy || task.LastStatus == nil {
			continue
		}
		switch *task.LastStatus {
		case "STOPPED":
			sawStopped = true
			if !t1aHas(r.Findings, awsclient.CodeECSTaskStateStopped) || t1aHas(r.Findings, awsclient.CodeECSTaskHealthUnhealthy) {
				t.Errorf("stopped UNHEALTHY task %s: findings %v, want the stopped state alone", r.ID, t2Codes(r.Findings))
			}
			cell, _ := listStatusCellFor(t, td, rows, r.ID)
			if strings.Contains(cell, catalog.Phrase(awsclient.CodeECSTaskHealthUnhealthy)) {
				t.Errorf("stopped UNHEALTHY task %s: Status cell %q reports the health check", r.ID, cell)
			}
			if top, ok := domain.TopFinding(r.Findings); ok {
				if got, want := td.ResolveColor(r), resource.ColorFromSeverity(top.Severity); got != want {
					t.Errorf("stopped UNHEALTHY task %s: row colour %v, want %v from %s", r.ID, got, want, top.Code)
				}
			}
		case "RUNNING":
			sawRunning = true
			if !t1aHas(r.Findings, awsclient.CodeECSTaskHealthUnhealthy) {
				t.Errorf("running UNHEALTHY task %s carries no %s; findings %v", r.ID, awsclient.CodeECSTaskHealthUnhealthy, t2Codes(r.Findings))
			}
		}
	}
	if !sawStopped {
		t.Errorf("no demo ecs-task row is STOPPED with HealthStatus UNHEALTHY among %d rows", len(rows))
	}
	if !sawRunning {
		t.Errorf("no demo ecs-task row is RUNNING with HealthStatus UNHEALTHY, so the health finding has no row to stand on")
	}
}

func t12ALBLogging(t *testing.T, clients *awsclient.ServiceClients, arn string) (logs string, loggedListener bool) {
	t.Helper()
	attrs, err := clients.ELBv2.DescribeLoadBalancerAttributes(context.Background(), &elbv2.DescribeLoadBalancerAttributesInput{LoadBalancerArn: aws.String(arn)})
	if err != nil {
		return "", false
	}
	for _, a := range attrs.Attributes {
		if a.Key != nil && *a.Key == "access_logs.s3.enabled" && a.Value != nil {
			logs = *a.Value
		}
	}
	ls, err := clients.ELBv2.DescribeListeners(context.Background(), &elbv2.DescribeListenersInput{LoadBalancerArn: aws.String(arn)})
	if err != nil {
		return logs, false
	}
	for _, l := range ls.Listeners {
		if l.Protocol == elbtypes.ProtocolEnumHttp || l.Protocol == elbtypes.ProtocolEnumHttps {
			loggedListener = true
		}
	}
	return logs, loggedListener
}

func t12RowLabels(r resource.Resource, code domain.FindingCode) []string {
	var out []string
	for _, row := range r.AttentionDetails[code].Rows {
		out = append(out, row.Label)
	}
	slices.Sort(out)
	return out
}

// An Application Load Balancer logs the requests its HTTP and HTTPS listeners
// serve, so the demo ALB with access logs off and such a listener shows the
// access-log row, and an ALB writing its logs shows none.
func TestT12Demo_ALBWithLoggedListenerAndLogsOffShowsTheAccessLogRow(t *testing.T) {
	const code domain.FindingCode = "elb.misconfigured"
	td, rows := t563Merged(t, "elb")
	clients := demo.NewServiceClients()
	sawOff, sawOn := false, false
	for _, r := range rows {
		lb, ok := t12Raw[elbtypes.LoadBalancer](r.RawStruct)
		if !ok || lb.Type != elbtypes.LoadBalancerTypeEnumApplication || lb.LoadBalancerArn == nil {
			continue
		}
		logs, logged := t12ALBLogging(t, clients, *lb.LoadBalancerArn)
		hasRow := slices.ContainsFunc(r.AttentionDetails[code].Rows, func(row domain.DetailRow) bool { return row.Label == "Access Logs" })
		switch {
		case logs == "false" && logged:
			sawOff = true
			if !t1aHas(r.Findings, code) || !hasRow {
				t.Errorf("ALB %s with access logs off and a logged listener: %s present %v, Access Logs row %v; rows %v",
					r.ID, code, t1aHas(r.Findings, code), hasRow, t12RowLabels(r, code))
				continue
			}
			t563AssertWitnessRendered(t, td, rows, r)
			if !slices.Contains(detailAttentionValuesFor(t, r, "elb"), "disabled") {
				t.Errorf("ALB %s: the detail Attention block does not show the access logs as disabled", r.ID)
			}
		case logs == "true":
			sawOn = true
			if hasRow {
				t.Errorf("ALB %s writes access logs but shows the Access Logs row", r.ID)
			}
		}
	}
	if !sawOff {
		t.Errorf("no demo ALB has access logs off and an HTTP or HTTPS listener among %d elb rows", len(rows))
	}
	if !sawOn {
		t.Errorf("no demo ALB writes access logs, so the row has no ALB to be absent from")
	}
}

func t12GlueCommand(job gluetypes.Job) string {
	if job.Command == nil || job.Command.Name == nil {
		return ""
	}
	return *job.Command.Name
}

// A Spark ETL job without a security configuration writes its output and logs
// unencrypted, so the demo carries one and it shows the finding.
func TestT12Demo_SparkJobWithoutSecurityConfigurationShowsTheFinding(t *testing.T) {
	td, rows := t563Merged(t, "glue")
	saw := false
	for _, r := range rows {
		job, ok := t12Raw[gluetypes.Job](r.RawStruct)
		if !ok || t12GlueCommand(job) != "glueetl" || job.SecurityConfiguration != nil {
			continue
		}
		saw = true
		if !t1aHas(r.Findings, t2GlueNoSecConfig) {
			t.Errorf("Spark job %s without a security configuration carries no %s; findings %v", r.ID, t2GlueNoSecConfig, t2Codes(r.Findings))
			continue
		}
		t563AssertWitnessRendered(t, td, rows, r)
	}
	if !saw {
		t.Errorf("no demo glue job is a Spark ETL job (glueetl) without a security configuration among %d rows", len(rows))
	}
}

// A Python shell job cannot use job bookmarks, so the demo Python shell job
// whose security configuration leaves encryption off shows a row for each
// of S3 output and CloudWatch logs the configuration leaves off, and no
// bookmarks row.
func TestT12Demo_PythonShellJobWithWeakSecurityConfigurationShowsNoBookmarksRow(t *testing.T) {
	td, rows := t563Merged(t, "glue")
	secCfg, ok := demo.NewServiceClients().Glue.(awsclient.GlueGetSecurityConfigurationAPI)
	if !ok {
		t.Fatal("the demo Glue client answers no GetSecurityConfiguration")
	}
	saw := false
	for _, r := range rows {
		job, ok := t12Raw[gluetypes.Job](r.RawStruct)
		if !ok || t12GlueCommand(job) != "pythonshell" || job.SecurityConfiguration == nil {
			continue
		}
		out, err := secCfg.GetSecurityConfiguration(context.Background(), &glue.GetSecurityConfigurationInput{Name: job.SecurityConfiguration})
		if err != nil || out.SecurityConfiguration == nil || out.SecurityConfiguration.EncryptionConfiguration == nil {
			continue
		}
		ec := out.SecurityConfiguration.EncryptionConfiguration
		var want []string
		if ec.CloudWatchEncryption == nil || ec.CloudWatchEncryption.CloudWatchEncryptionMode == gluetypes.CloudWatchEncryptionModeDisabled {
			want = append(want, "CloudWatch logs")
		}
		s3Off := len(ec.S3Encryption) == 0 || !slices.ContainsFunc(ec.S3Encryption, func(e gluetypes.S3Encryption) bool {
			return e.S3EncryptionMode != "" && e.S3EncryptionMode != gluetypes.S3EncryptionModeDisabled
		})
		if s3Off {
			want = append(want, "S3 output")
		}
		if len(want) == 0 {
			continue
		}
		saw = true
		if !t1aHas(r.Findings, t2GlueSecConfigWeak) {
			t.Errorf("Python shell job %s (%s) carries no %s; findings %v", r.ID, *job.SecurityConfiguration, t2GlueSecConfigWeak, t2Codes(r.Findings))
			continue
		}
		if got := t12RowLabels(r, t2GlueSecConfigWeak); !slices.Equal(got, want) {
			t.Errorf("Python shell job %s: supporting rows %v, want %v", r.ID, got, want)
		}
		t563AssertWitnessRendered(t, td, rows, r)
	}
	if !saw {
		t.Errorf("no demo Python shell job has a security configuration that leaves encryption off among %d glue rows", len(rows))
	}
}

func t12FooterOn(t *testing.T, short string, rows []resource.Resource, id string) []app.KeyHint {
	t.Helper()
	c := newVisibilityListController(t, short)
	c.ApplyResourcesLoaded(short, rows, nil, false)
	body := c.Snapshot().Body.List
	i := slices.IndexFunc(body.Rows, func(r app.ListRow) bool { return r.ResourceID == id })
	if i < 0 {
		t.Fatalf("%s/%s is not in the list body", short, id)
	}
	c.Apply(app.Action{Kind: app.ActionMoveTop})
	for range i {
		c.Apply(app.Action{Kind: app.ActionMoveDown})
	}
	vs := c.Snapshot()
	if got := vs.Body.List.Rows[vs.Body.List.Selected].ResourceID; got != id {
		t.Fatalf("%s: cursor on %s, want %s", short, got, id)
	}
	return vs.Footer
}

// A row whose Enter drill is not offered shows the footer a row of a type
// with no child view shows: no hint for the drill, and the same Enter and d
// hints.
func TestT12_FooterOnARowWithoutItsChildViewIsAPlainRowsFooter(t *testing.T) {
	byType, _ := buildVisibilityTypeCache(t)
	plain := ""
	for _, td := range resource.AllResourceTypes() {
		if len(td.Children) == 0 && len(byType[td.ShortName]) > 0 {
			plain = td.ShortName
			break
		}
	}
	if plain == "" {
		t.Fatal("no demo type declares no child view")
	}
	pick := func(hints []app.KeyHint, keys ...string) []app.KeyHint {
		return slices.DeleteFunc(slices.Clone(hints), func(h app.KeyHint) bool { return !slices.Contains(keys, h.Key) })
	}
	want := pick(t12FooterOn(t, plain, byType[plain], byType[plain][0].ID), "enter", "d")
	checked := 0
	for _, td := range resource.AllResourceTypes() {
		for _, ch := range td.Children {
			if ch.Key != "enter" || ch.DrillCondition == nil {
				continue
			}
			rows := byType[td.ShortName]
			offered := false
			for _, r := range rows {
				if ch.DrillCondition(domain.Resource(r)) {
					if !offered {
						offered = true
						if got := pick(t12FooterOn(t, td.ShortName, rows, r.ID), "enter"); len(got) != 1 {
							t.Errorf("%s/%s offers %s but its footer has Enter hints %v", td.ShortName, r.ID, ch.ChildType, got)
						}
					}
					continue
				}
				checked++
				footer := t12FooterOn(t, td.ShortName, rows, r.ID)
				if got := pick(footer, "enter", "d"); !slices.Equal(got, want) {
					t.Errorf("%s/%s, which does not offer %s: Enter/d hints %v, a %s row shows %v", td.ShortName, r.ID, ch.ChildType, got, plain, want)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no demo row of a top-level type withholds its Enter drill")
	}
}
