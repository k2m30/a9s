package unit_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"

	"github.com/k2m30/a9s/v3/core/app"
	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func t9bPathRows(rows []app.FieldRow, path string) []string {
	var out []string
	for _, r := range rows {
		if r.Path == path || strings.HasPrefix(r.Path, path+".") {
			out = append(out, fmt.Sprintf("%s=%s", r.Key, r.Value))
		}
	}
	return out
}

// t9bStructOnly is the row as the SDK struct alone describes it: the detail a
// path-form field shows reads the struct, so dropping Fields must leave every
// path-form row unchanged.
func t9bStructOnly(r resource.Resource) resource.Resource {
	r.Fields = nil
	return r
}

// The Status row of the detail screen is what AWS returned for the resource's
// status field. a9s's finding phrase, with its "(+N)" count, belongs to the
// list's Status cell and the Attention block, not to the AWS field. On
// vpc-peer the field is a struct (Code and Message) and is shown as one.
func TestT9b_Row1_DetailStatusIsTheAWSValue(t *testing.T) {
	types := map[string]bool{
		"redis": true, "dbc": true, "dbi-snap": true, "dbc-snap": true,
		"mwaa": true, "vpc-peer": true, "transfer_agreements": true,
	}
	withFinding := map[string]int{}
	for _, row := range t9bDemoRows(t) {
		if !types[row.typ] || row.res.RawStruct == nil {
			continue
		}
		if len(row.res.Findings) > 0 {
			withFinding[row.typ]++
		}
		got := t9bPathRows(t9bDetailRows(t, row.res, row.typ), "Status")
		want := t9bPathRows(t9bDetailRows(t, t9bStructOnly(row.res), row.typ), "Status")
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Errorf("%s %s (%d findings): Status rows %q, AWS states %q", row.typ, row.res.ID, len(row.res.Findings), got, want)
		}
		for _, g := range got {
			if strings.Contains(g, "(+") {
				t.Errorf("%s %s: Status row %q carries a finding count", row.typ, row.res.ID, g)
			}
		}
	}
	for typ := range types {
		if withFinding[typ] == 0 {
			t.Errorf("%s: no demo row carries a finding and a struct, so the Status row is untested", typ)
		}
	}
}

// DBInstance.Endpoint holds Address, Port and HostedZoneId; the Endpoint row
// shows the struct, so the operator reads the port there. The RDS API
// reference notes the endpoint "might not be shown for instances with the
// status of creating", and such an instance shows no endpoint.
func TestT9b_Row2_DBIEndpointShowsAddressAndPort(t *testing.T) {
	rdsAPI := &t9bRDSAccount{instances: []rdstypes.DBInstance{t9bInstance("acme-orders", "postgres")}}
	creating := t9bInstance("acme-orders-new", "postgres")
	creating.Endpoint = nil
	creating.DBInstanceStatus = aws.String("creating")
	rdsAPI.instances = append(rdsAPI.instances, creating)

	page, err := awsclient.FetchRDSInstancesPage(context.Background(), rdsAPI, "")
	if err != nil || len(page.Resources) != 2 {
		t.Fatalf("FetchRDSInstancesPage: %d rows, err %v", len(page.Resources), err)
	}
	byID := map[string]resource.Resource{}
	for _, r := range page.Resources {
		byID[r.ID] = r
	}

	rows := t9bPathRows(t9bDetailRows(t, byID["acme-orders"], "dbi"), "Endpoint")
	joined := strings.ReplaceAll(strings.Join(rows, "\n"), `"`, "")
	for _, want := range []string{
		"Address: acme-orders.c1a2b3c4d5e6.eu-west-1.rds.amazonaws.com",
		"Port: 5432",
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("dbi acme-orders: Endpoint rows %q do not show %q", rows, want)
		}
	}

	rows = t9bPathRows(t9bDetailRows(t, byID["acme-orders-new"], "dbi"), "Endpoint")
	if strings.Join(rows, "\n") != "Endpoint=-" {
		t.Errorf("dbi acme-orders-new (creating, no endpoint): Endpoint rows %q, want the single absent row \"Endpoint=-\"", rows)
	}
}

// Every default path-form detail field of every type reads the SDK struct the
// fetcher stored: for every demo row its rows are the ones the struct alone
// gives. A value only a fetcher computes is declared key-form in the type's
// default detail instead.
func TestT9b_Guard_PathFormDetailFieldsReadTheStruct(t *testing.T) {
	cfg := config.DefaultConfig()
	violations := map[string]string{}
	rowsPer := map[string]int{}
	checked := 0
	for _, row := range t9bDemoRows(t) {
		if row.res.RawStruct == nil {
			continue
		}
		var paths []string
		for _, df := range config.GetViewDef(cfg, row.typ).Detail {
			if df.Path != "" {
				paths = append(paths, df.Path)
			}
		}
		if len(paths) == 0 {
			continue
		}
		checked++
		got := t9bDetailRows(t, row.res, row.typ)
		want := t9bDetailRows(t, t9bStructOnly(row.res), row.typ)
		for _, p := range paths {
			g, w := t9bPathRows(got, p), t9bPathRows(want, p)
			if strings.Join(g, "\n") == strings.Join(w, "\n") {
				continue
			}
			key := row.typ + " " + p
			rowsPer[key]++
			if _, ok := violations[key]; !ok {
				violations[key] = fmt.Sprintf("%s: e.g. %s shows %q, the struct gives %q", key, row.res.ID, g, w)
			}
		}
	}
	if checked < 1000 {
		t.Fatalf("only %d demo rows with a struct and a default detail were checked", checked)
	}
	var lines []string
	for key, v := range violations {
		lines = append(lines, fmt.Sprintf("%s (%d rows)", v, rowsPer[key]))
	}
	t9bReport(t, "path-form detail fields that do not show the struct's value", lines)
}

// A row with no struct has only Fields, and its detail shows them: a row
// replayed from the disk cache keeps the lifecycle value AWS reported.
func TestT9b_Row4_RowWithoutStructShowsFields(t *testing.T) {
	rows := t9bDemoRows(t)
	cases := []struct {
		typ, id, path, key string
		replay             bool
	}{
		{"redis", "legacy-redis-billing", "Status", "status", true},
		{"dbi", "prod-dbi-1", "Endpoint", "endpoint", true},
	}
	for _, c := range cases {
		row := t9bFindRow(t, rows, c.typ, c.id)
		res := row.res
		if c.replay {
			res.RawStruct = nil
		} else if res.RawStruct != nil {
			t.Fatalf("%s %s: the degraded row carries a struct %T", c.typ, c.id, res.RawStruct)
		}
		want := res.Fields[c.key]
		if want == "" {
			t.Fatalf("%s %s: Fields[%q] is empty, the case cannot tell the lanes apart", c.typ, c.id, c.key)
		}
		got := t9bPathRows(t9bDetailRows(t, res, c.typ), c.path)
		if strings.Join(got, "\n") != c.path+"="+want {
			t.Errorf("%s %s with no struct: %s rows %q, want %q from Fields[%q]", c.typ, c.id, c.path, got, c.path+"="+want, c.key)
		}
	}

	live := t9bFindRow(t, rows, "mwaa", "warn-airflow-maintenance").res
	replayed := live
	replayed.RawStruct = nil
	raw := replayed.Fields["status"]
	if raw == "" {
		t.Fatalf("mwaa warn-airflow-maintenance: Fields[status] is empty; a replayed row keeps the value AWS reported")
	}
	want := t9bPathRows(t9bDetailRows(t, live, "mwaa"), "Status")
	got := t9bPathRows(t9bDetailRows(t, replayed, "mwaa"), "Status")
	if strings.Join(got, "\n") != "Status="+domain.HumanizeStatusPhrase(raw) || strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("mwaa warn-airflow-maintenance replayed: Status rows %q, want %q (AWS reported %q)", got, want, raw)
	}

	// A degraded row has no lifecycle value: its describe failed, so the
	// detail Status reads "-" and the details-denied finding says why.
	degraded := t9bFindRow(t, rows, "mwaa", "warn-airflow-details-denied").res
	if degraded.RawStruct != nil {
		t.Fatalf("mwaa warn-airflow-details-denied: the degraded row carries a struct %T", degraded.RawStruct)
	}
	if got := t9bPathRows(t9bDetailRows(t, degraded, "mwaa"), "Status"); strings.Join(got, "\n") != "Status=-" {
		t.Errorf("mwaa warn-airflow-details-denied: Status rows %q, want [Status=-]", got)
	}
	if !slices.ContainsFunc(degraded.Findings, func(f domain.Finding) bool { return f.Phrase == "details denied" }) {
		t.Errorf("mwaa warn-airflow-details-denied: findings %v carry no details-denied finding", degraded.Findings)
	}
}
