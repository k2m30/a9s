package unit_test

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/fieldpath"
	"github.com/k2m30/a9s/v3/core/resource"
	unit "github.com/k2m30/a9s/v3/tests/unit"
)

func t9bDevTreeRows(t *testing.T) []t9bRow {
	t.Helper()
	rows, _, _ := t9bDevTreeRowsEnriched(t)
	return rows
}

// The detail screen shows the row its type's detail enricher returns once the
// read lands, so the walk checks that row too.
func t9bDevTreeRowsEnriched(t *testing.T) ([]t9bRow, map[int]bool, map[string]int) {
	t.Helper()
	enrichedRows := map[string]int{}
	enrichedIdx := map[int]bool{}
	dctx := &awsclient.DetailEnrichmentCtx{Clients: demo.NewServiceClients(), PolicyDocs: &awsclient.PolicyDocumentCache{}, DetailDocs: &awsclient.DetailDocCache{}}
	clients := demo.NewServiceClients()
	const perTypeCap = 300
	var out []t9bRow
	count := map[string]int{}
	var walk func(td catalog.ResourceTypeDef, rows []resource.Resource, parentCtx map[string]string, depth int)
	walk = func(td catalog.ResourceTypeDef, rows []resource.Resource, parentCtx map[string]string, depth int) {
		for i := range rows {
			if count[td.ShortName] >= perTypeCap {
				break
			}
			count[td.ShortName]++
			out = append(out, t9bRow{typ: td.ShortName, res: rows[i]})
			if td.DetailEnrich != nil {
				enriched, err := td.DetailEnrich(context.Background(), dctx, rows[i])
				if err == nil || errors.As(err, new(domain.PartialDetailErr)) {
					enrichedIdx[len(out)] = true
					out = append(out, t9bRow{typ: td.ShortName, res: enriched})
					enrichedRows[td.ShortName]++
				}
			}
			if depth >= 4 {
				continue
			}
			for _, child := range td.Children {
				cd := catalog.FindAny(child.ChildType)
				if cd == nil || cd.ChildFetcher == nil {
					continue
				}
				pctx := resource.ResolveChildContext(child, &rows[i], parentCtx)
				got, _ := cd.ChildFetcher(context.Background(), clients, pctx, "") //nolint:errcheck // a partial page still carries the rows the drill shows
				walk(*cd, got.Resources, pctx, depth+1)
			}
		}
	}
	for _, td := range resource.AllResourceTypes() {
		rows, ok := unit.DrainFixtures(t, td, clients)
		if !ok {
			continue
		}
		walk(td, rows, nil, 0)
	}
	return out, enrichedIdx, enrichedRows
}

// t9bDevStructRight names the path-form fields whose struct value is the one
// the detail shows although the fetcher also stores a different Fields value
// under the matching key: the Fields copy is a list cell (a joined list, one
// sub-field, a trimmed line) and the struct carries what AWS returned.
var t9bDevStructRight = map[string]string{
	"asg SuspendedProcesses":               "each suspended process with its reason",
	"athena EngineVersion":                 "selected and effective engine version",
	"cf Aliases.Items":                     "the alias list; Fields joins it into one cell",
	"dbi Endpoint":                         "address and port; Fields holds the address",
	"dbi_events EventCategories":           "a list; Fields joins it",
	"eb-rule EventPattern":                 "the pattern document; Fields holds the JSON string",
	"ecr_images ImageTags":                 "a list; Fields joins it",
	"elb State":                            "code and reason; Fields holds the code",
	"glue Command":                         "name, script location and runtime; Fields holds the name",
	"ng InstanceTypes":                     "a list; Fields joins it",
	"redshift Endpoint":                    "address and port; Fields holds the address",
	"role AssumeRolePolicyDocument":        "the policy document; Fields holds the JSON string",
	"lambda_invocation_logs message":       "the event message; Fields trims it to one line",
	"log_events Message":                   "the event message; Fields trims it to one line",
	"cb_build_logs Message":                "the event message; Fields trims it to one line",
	"ecs_svc_logs Message":                 "the event message; Fields trims it to one line",
	"sfn_execution_history EventTimestamp": "the instant; Fields holds it relative",
	"transfer_agreements Status":           "Fields[status] is the list cell phrase; the type declares no state table",
}

func t9bDevFieldsKey(fields map[string]string, path string) string {
	for k := range fields {
		if strings.EqualFold(k, path) {
			return k
		}
	}
	if _, ok := fields[fieldpath.ToSnakeCase(path)]; ok {
		return fieldpath.ToSnakeCase(path)
	}
	return ""
}

func t9bDevResolvesOnType(raw any, path string) bool {
	t := reflect.TypeOf(raw)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t.Kind() != reflect.Struct {
		return true
	}
	first, _, _ := strings.Cut(path, ".")
	if _, ok := t.FieldByName(first); ok {
		return true
	}
	for i := 0; i < t.NumField(); i++ {
		if strings.EqualFold(t.Field(i).Name, first) {
			return true
		}
	}
	return false
}

func TestT9bDev_DetailPathsShowTheStructAtEveryLevel(t *testing.T) {
	cfg := config.DefaultConfig()
	rows, enrichedIdx, enriched := t9bDevTreeRowsEnriched(t)
	for _, want := range []string{"policy", "role_policies", "sfn", "cfn", "s3", "sns", "transfer_agreements", "ec2"} {
		if enriched[want] == 0 {
			t.Errorf("the walk checked no %s row as its detail enricher returns it", want)
		}
	}
	levels := map[string]bool{}
	var problems []string
	seen := map[string]bool{}
	report := func(key, msg string) {
		if !seen[key] {
			seen[key] = true
			problems = append(problems, msg)
		}
	}
	checked := 0
	resolves := map[string]bool{}
	computedShown := map[string]bool{}
	for i, row := range rows {
		if row.res.RawStruct == nil {
			continue
		}
		vd := config.GetViewDef(cfg, row.typ)
		td := catalog.FindAny(row.typ)
		if td == nil || td.Project != nil {
			continue
		}
		humanized := td.HumanizedFields()
		computed := map[string]bool{}
		for _, p := range td.ComputedDetailPaths {
			computed[p] = true
		}
		levels[row.typ] = true
		shown := t9bDetailRows(t, row.res, row.typ)
		for _, df := range vd.Detail {
			if df.Path == "" || (computed[df.Path] && !enrichedIdx[i]) {
				continue
			}
			checked++
			key := row.typ + " " + df.Path
			if computed[df.Path] {
				if _, ok := computedShown[key]; !ok {
					computedShown[key] = false
				}
				for _, g := range t9bPathRows(shown, df.Path) {
					if !strings.HasSuffix(g, "=-") {
						computedShown[key] = true
					}
				}
			}
			if _, ok := resolves[key]; !ok {
				resolves[key] = false
			}
			if !t9bDevResolvesOnType(row.res.RawStruct, df.Path) {
				continue
			}
			resolves[key] = true
			got := t9bPathRows(shown, df.Path)
			sub := fieldpath.ExtractSubtree(row.res.RawStruct, df.Path)
			if len(got) == 1 && !strings.Contains(sub, "\n") && !strings.HasSuffix(got[0], "=") {
				want := sub
				if want == "" {
					want = "-"
				} else if catalog.Humanizes(humanized, df.Path, df.DisplayLabel()) {
					want = domain.HumanizeStatusPhrase(want)
				}
				want = config.CanonicalValue(want)
				if _, value, _ := strings.Cut(got[0], "="); value != want && !strings.ContainsAny(value, "*…") {
					report(key+" struct", fmt.Sprintf("%s %s: shows %q, the struct holds %q", key, row.res.ID, value, want))
				}
			}
			if _, ok := t9bDevStructRight[key]; ok {
				continue
			}
			fk := t9bDevFieldsKey(row.res.Fields, df.Path)
			if fk == "" || row.res.Fields[fk] == "" || row.res.Fields[fk] == "-" || (len(got) == 1 && strings.ContainsAny(got[0], "*…")) {
				continue
			}
			readable := row.res.Fields[fk]
			if catalog.Humanizes(humanized, df.Path, df.DisplayLabel()) {
				readable = domain.HumanizeStatusPhrase(readable)
			}
			readable = config.CanonicalValue(readable)
			if len(got) != 1 || !strings.HasSuffix(got[0], "="+readable) {
				report(key+" fields", fmt.Sprintf("%s %s: shows %q, the fetcher stores %q under Fields[%q]; declare it key-form or show the struct readably", key, row.res.ID, got, readable, fk))
			}
		}
	}
	for key, ok := range computedShown {
		if !ok {
			report(key+" computed", fmt.Sprintf("%s: no demo row shows a value once its detail enricher returns it; a value left only in Fields needs a key-form field", key))
		}
	}
	for key, ok := range resolves {
		if !ok {
			report(key+" type", fmt.Sprintf("%s: the path names no field of the struct on any demo row (a typo, or a computed path not in ComputedDetailPaths)", key))
		}
	}
	if checked < 2000 {
		t.Fatalf("only %d detail fields checked", checked)
	}
	for _, want := range []string{"log_events", "cb_build_logs", "lambda_invocation_logs", "log_streams"} {
		if !levels[want] {
			t.Errorf("the drill walk never reached %s", want)
		}
	}
	t9bReport(t, "detail path problems", problems)
}

func t9bDevRenderedLabel(s string) string {
	s = strings.TrimPrefix(strings.TrimSpace(s), "- ")
	label, _, _ := strings.Cut(s, ": ")
	return strings.TrimSuffix(label, ":")
}

func TestT9bDev_DetailRowsRenderOnceInDeclaredOrder(t *testing.T) {
	cfg := config.DefaultConfig()
	var problems []string
	seen := map[string]bool{}
	report := func(key, msg string) {
		if !seen[key] {
			seen[key] = true
			problems = append(problems, msg)
		}
	}
	for _, row := range t9bDevTreeRows(t) {
		shown := t9bDetailRows(t, row.res, row.typ)
		for _, r := range shown {
			if r.IsSection || r.IsSpacer || r.IsHeader {
				continue
			}
			twice := false
			switch {
			case r.IsSubField:
				if r.Key != "" && r.Key != r.Value && !(r.Path == "Attention" && r.IndentLevel == 1) {
					label := t9bDevRenderedLabel(r.Key)
					twice = strings.Contains(r.Key, ": ") && label != "" && t9bDevRenderedLabel(r.Value) == label
				}
			default:
				twice = r.Key != "" && strings.HasPrefix(strings.TrimSpace(r.Value), r.Key+": ")
			}
			if twice {
				report(row.typ+" "+r.Path+" twice", fmt.Sprintf("%s %s: row %q shows %q, two renderings of one field", row.typ, row.res.ID, r.Key, r.Value))
			}
		}

		td := catalog.FindAny(row.typ)
		vd := config.GetViewDef(cfg, row.typ)
		if td == nil || td.Project != nil || len(vd.Detail) == 0 {
			continue
		}
		var want []string
		declared := map[string]bool{}
		for _, df := range vd.Detail {
			id := df.Path
			if id == "" {
				id = df.Key
			}
			want = append(want, id)
			declared[id] = true
		}
		var got []string
		for _, r := range shown {
			if r.IsSection || r.IsSpacer || r.IsSubField || r.Path == "Attention" {
				continue
			}
			got = append(got, r.Path)
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			report(row.typ+" order", fmt.Sprintf("%s %s: rows %v, declared %v", row.typ, row.res.ID, got, want))
		}
	}
	t9bReport(t, "detail rows not rendered once in the declared order", problems)
}
