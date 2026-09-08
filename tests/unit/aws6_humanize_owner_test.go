package unit_test

// aws6_humanize_owner_test.go — one owner of the "this field carries an AWS
// constant an operator should not have to read" declaration.
//
// The opt-in says something about the FACT, not about the list: a field that
// needs readable wording in a column needs it in the detail too, and a field
// that no column happens to show needs it just the same. While the flag lives
// on a column, a fact with no column has no way to ask, and the detail shows
// the SDK constant verbatim.
//
// These tests are written against the rendered value only. Whatever shape the
// declaration takes, the demo detail must read the same words a person would.

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// humanizedDetailWitness names one demo detail row that must read as words.
// The raw value is recorded so the failure says what is on screen today.
var humanizedDetailWitnesses = []struct {
	shortName  string
	resourceID string
	path       string
	raw        string
	want       string
	// statusColumnShowsAFinding marks a witness whose type spells its Status
	// COLUMN from findings rather than from this field, so the column and the
	// detail are showing two different facts and comparing them says nothing.
	statusColumnShowsAFinding bool
}{
	// The field row 4 is about: mwaa declares no column for it, so the
	// column-owned opt-in cannot reach it at all.
	{"mwaa", "prod-airflow-etl", "endpoint_management", "SERVICE", "service", false},
	{"mwaa", "prod-airflow-etl", "last_update_status", "SUCCESS", "success", false},

	// The three other demo detail screens showing a raw constant.
	{"apigw", "efg567hij8", "protocol", "WEBSOCKET", "websocket", false},
	{"ecs-svc", "api-gateway", "launch_type", "FARGATE", "fargate", false},
	{"ecs-svc", "api-gateway", "status", "ACTIVE", "active", true},
	{"ng", "acme-prod-degraded-pool", "status", "DEGRADED", "degraded", false},
}

// TestDemoDetailShowsWordsNotConstants pins each named field on the demo
// detail, through the real detail build.
func TestDemoDetailShowsWordsNotConstants(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	for _, w := range humanizedDetailWitnesses {
		t.Run(w.shortName+"/"+w.path, func(t *testing.T) {
			td := resource.FindResourceType(w.shortName)
			if td == nil {
				t.Fatalf("no resource type %q", w.shortName)
			}
			rows := mergeWave2Findings(t, *td, byType[w.shortName], cache, clients)
			var row resource.Resource
			found := false
			for _, r := range rows {
				if r.ID == w.resourceID {
					row, found = r, true
					break
				}
			}
			if !found {
				t.Fatalf("no demo %s row %q — the witness this field is pinned on is gone", w.shortName, w.resourceID)
			}

			got, ok := aws6DetailValueAtPath(t, row, w.shortName, w.path)
			if !ok {
				t.Fatalf("%s %s: no detail row at path %q", w.shortName, w.resourceID, w.path)
			}
			if got == w.raw {
				t.Errorf("%s %s: %s renders the SDK constant %q; want %q",
					w.shortName, w.resourceID, w.path, got, w.want)
				return
			}
			if got != w.want {
				t.Errorf("%s %s: %s = %q, want %q", w.shortName, w.resourceID, w.path, got, w.want)
			}
		})
	}
}

// TestListColumnAndDetailAgreeOnWording pins the second half of row 4: one
// declaration read by both surfaces. A field that reads as words on the detail
// and as a constant in its column (or the reverse) is the fact declared twice
// and disagreeing, which is the shape this change removes.
func TestListColumnAndDetailAgreeOnWording(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	for _, w := range humanizedDetailWitnesses {
		td := resource.FindResourceType(w.shortName)
		if td == nil {
			t.Fatalf("no resource type %q", w.shortName)
		}
		if w.statusColumnShowsAFinding {
			continue
		}
		colIndex := -1
		for i, col := range td.Columns {
			if col.Key == w.path {
				colIndex = i
				break
			}
		}
		if colIndex < 0 {
			// No column shows this field; row 4 is exactly about that case, and
			// the detail pin above already covers it.
			continue
		}

		rows := mergeWave2Findings(t, *td, byType[w.shortName], cache, clients)
		cell, ok := aws6ListCell(t, *td, rows, w.resourceID, w.path)
		if !ok {
			t.Errorf("%s: no %q cell for row %q", w.shortName, w.path, w.resourceID)
			continue
		}
		if cell != w.want {
			t.Errorf("%s %s: list cell %q = %q, want %q — the column and the detail read one declaration",
				w.shortName, w.resourceID, w.path, cell, w.want)
		}
	}
}

// rawEnumCellPattern matches a whole value shaped like an UPPER_SNAKE_CASE AWS
// constant ("SERVICE", "CREATE_FAILED"). Anchored end to end, so prose that
// merely contains a capitalized word is never touched.
var rawEnumCellPattern = regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)*$`)

// rawCamelEnumCellPattern matches a whole value shaped like a CamelCase AWS
// constant ("PendingDeletion", "InProgress"): two or more capital-initial words
// run together with no separator.
var rawCamelEnumCellPattern = regexp.MustCompile(`^([A-Z][a-z0-9]+){2,}$`)

// verbatimDetailPaths names a (type, detail path) pair whose value must reach
// the screen exactly as AWS wrote it, with the reason.
//
// Every entry is a value a person types back or searches for — an access key,
// a role id, an API operation name, an error code — so rewording it would make
// it wrong. That is the only reason an entry is allowed here: this is not a
// waiting list, and a field that merely has not been got to yet does not
// belong in it.
//
// A field that SHOULD read as words is declared on its type
// (ResourceTypeDef.HumanizeFields) and then renders as words, so the sweep
// below never reaches it. Those declarations are held to it by the ratchet in
// aws6_humanize_declared_fields_test.go, which fails the day one of them
// starts rendering a constant again.
var verbatimDetailPaths = map[string]string{
	"alarm/metric_name":                 "identifier: the CloudWatch metric's own name",
	"alarm/namespace":                   "identifier: the CloudWatch namespace",
	"cf/distribution_id":                "identifier: the CloudFront distribution id",
	"ct-events/ACTION.Insight type":     "identifier: the CloudTrail insight type name",
	"ct-events/ACTOR.Access key":        "identifier: the access key id",
	"ct-events/ENVELOPE.EventName":      "identifier: the AWS API operation name",
	"ct-events/ENVELOPE.Username":       "identifier: the calling principal's name",
	"ct-events/ERROR.errorCode":         "identifier: AWS's own error code, quoted verbatim so it can be searched",
	"ct-events/REQUEST.rotationType":    "verbatim: the raw request block reproduces what the event carried",
	"ct-events/RAW EVENT.errorCode":     "verbatim: the raw event view reproduces the event as AWS sent it",
	"ct-events/RAW EVENT.eventCategory": "verbatim: the raw event view reproduces the event as AWS sent it",
	"ct-events/RAW EVENT.eventName":     "verbatim: the raw event view reproduces the event as AWS sent it",
	"ct-events/RAW EVENT.eventType":     "verbatim: the raw event view reproduces the event as AWS sent it",
	"ecs-task/Attention":                "identifier by design: the stop code IS the thing to search for (core/aws/ecs_task_issue_enrichment.go)",
	"ecs-task/stop_code":                "identifier by design: the stop code IS the thing to search for",
	"iam-group/Attention":               "identifier: the attached policy's own name",
	"iam-group/group_id":                "identifier: the IAM group id",
	"iam-user/Attention":                "identifier: the attached policy's own name",
	"iam-user/user_id":                  "identifier: the IAM user id",
	"msk/version":                       "identifier: the MSK configuration revision",
	"policy/policy_name":                "identifier: the policy's own name",
	"role/Attention":                    "identifier: the attached policy's own name",
	"role/role_id":                      "identifier: the IAM role id",
	"role/role_name":                    "identifier: the role's own name",
}

// TestNoDemoDetailRowIsARawConstant sweeps every demo detail screen for a row
// whose whole value is an SDK constant. It is the standing half of rows 4 to 6:
// the four fields fixed here stay fixed, and a new one has to be argued for in
// the allowlist above rather than shipped silently.
func TestNoDemoDetailRowIsARawConstant(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	seen := map[string]bool{}
	var offenders []string
	for _, td := range resource.AllResourceTypes() {
		rows := mergeWave2Findings(t, td, byType[td.ShortName], cache, clients)
		for _, r := range rows {
			for _, f := range aws6DetailRows(t, r, td.ShortName) {
				if f.Value == "" {
					continue
				}
				if !rawEnumCellPattern.MatchString(f.Value) && !rawCamelEnumCellPattern.MatchString(f.Value) {
					continue
				}
				key := td.ShortName + "/" + f.Path
				if seen[key] {
					continue
				}
				seen[key] = true
				if _, ok := verbatimDetailPaths[key]; ok {
					continue
				}
				offenders = append(offenders, fmt.Sprintf("%s (row %s) = %q", key, r.ID, f.Value))
			}
		}
	}
	sort.Strings(offenders)
	for _, o := range offenders {
		t.Errorf("demo detail row shows a raw SDK constant: %s — declare the field's readable wording on its type, "+
			"or add the pair to verbatimDetailPaths with the reason it must stay verbatim", o)
	}
}

// logEventStatusClasses names each class classifyLogEventStatus recognizes and
// the words its status must read as. The demo account must contain a witness
// for every one: a class with no witness is a classification nobody can see.
var logEventStatusClasses = []struct {
	class string
	want  string
}{
	{"ERROR", "error"},
	{"WARN", "warn"},
	{"REPORT", "report"},
	{"META", "meta"},
}

// TestLogEventStatusReadsAsWords pins row 6 on the demo bench. The status a log
// event carries is classified by a9s, not returned by AWS, so a raw token there
// is a9s writing an SDK-shaped constant of its own.
func TestLogEventStatusReadsAsWords(t *testing.T) {
	events := demoLogEvents(t, demo.NewServiceClients())
	if len(events) == 0 {
		t.Fatal("no demo log events")
	}

	byStatus := map[string][]string{}
	for _, e := range events {
		s := e.Fields["status"]
		if s == "" {
			continue
		}
		byStatus[s] = append(byStatus[s], e.ID)
	}

	for _, c := range logEventStatusClasses {
		if ids, raw := byStatus[c.class]; raw {
			t.Errorf("demo log events %v carry the raw status token %q; want %q — "+
				"the classification is a9s's own, so it is written in the words it renders in",
				ids, c.class, c.want)
			continue
		}
		if len(byStatus[c.want]) == 0 {
			t.Errorf("no demo log event carries status %q — the %s class has no witness, "+
				"so nothing on the demo bench shows what it looks like", c.want, c.class)
		}
	}

	// The negative case: an event that matches no class carries no status at
	// all, rather than a word that reads like one.
	for s := range byStatus {
		known := false
		for _, c := range logEventStatusClasses {
			if s == c.want || s == c.class {
				known = true
			}
		}
		if !known {
			t.Errorf("demo log events carry status %q, which is not one of the classified classes %v", s, byStatus[s])
		}
	}
}

// detailRowsFor drives the real detail build and returns every rendered field
// row for a resource.
func aws6DetailRows(t *testing.T, res resource.Resource, shortName string) []app.FieldRow {
	t.Helper()
	c := newVisibilityDetailController(t)
	c.EnsureDetailState(res, shortName)
	body := c.Snapshot().Body.Detail
	if body == nil {
		return nil
	}
	return body.Fields
}

// detailValueAtPath returns the rendered value of the detail row at path.
func aws6DetailValueAtPath(t *testing.T, res resource.Resource, shortName, path string) (string, bool) {
	t.Helper()
	for _, f := range aws6DetailRows(t, res, shortName) {
		if strings.EqualFold(f.Path, path) {
			return f.Value, true
		}
	}
	return "", false
}

// listCellFor drives the real list body build and returns the cell under the
// column whose key is colKey for the row with the given id.
func aws6ListCell(t *testing.T, td resource.ResourceTypeDef, rows []resource.Resource, resourceID, colKey string) (string, bool) {
	t.Helper()
	c := newVisibilityListController(t, td.ShortName)
	c.ApplyResourcesLoaded(td.ShortName, rows, nil, false)
	body := c.Snapshot().Body.List
	if body == nil {
		return "", false
	}
	col := -1
	for i, h := range body.Columns {
		if h.Key == colKey {
			col = i
			break
		}
	}
	if col < 0 {
		return "", false
	}
	for _, row := range body.Rows {
		if row.ResourceID != resourceID {
			continue
		}
		if col >= len(row.Cells) {
			return "", false
		}
		return row.Cells[col], true
	}
	return "", false
}

// demoLogEvents drains the log-events child view for every demo log stream.
func demoLogEvents(t *testing.T, clients *awsclient.ServiceClients) []resource.Resource {
	t.Helper()
	groups := resource.FindResourceType("logs")
	if groups == nil {
		t.Fatal("no logs resource type registered")
	}
	groupRows, ok := drainVisibilityFixtures(t, *groups, clients)
	if !ok {
		t.Fatal("logs has no fetcher")
	}

	var out []resource.Resource
	var walk func(td resource.ResourceTypeDef, rows []resource.Resource, parentCtx map[string]string, depth int)
	walk = func(td resource.ResourceTypeDef, rows []resource.Resource, parentCtx map[string]string, depth int) {
		if depth > 2 {
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
				res, err := ctd.ChildFetcher(context.Background(), clients, pctx, "")
				if err != nil {
					continue
				}
				if child.ChildType == "log_events" {
					out = append(out, res.Resources...)
					continue
				}
				walk(*ctd, res.Resources, pctx, depth+1)
			}
		}
	}
	walk(*groups, groupRows, nil, 0)
	return out
}
