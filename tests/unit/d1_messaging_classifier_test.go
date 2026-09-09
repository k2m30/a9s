package unit

import (
	"context"
	"testing"

	awsclient "github.com/k2m30/a9s/v3/core/aws"
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/demo/fakes"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// d1_messaging_classifier_test.go covers the three classifiers that came off
// the raw-enum shape last: eb-rule, kinesis and msk. Two things have to hold.
// A row that carries findings is coloured by the worst of them, not the first
// one in the slice. A row built without findings is coloured by the type's own
// predicate over its raw field, so the bare-Fields path and the fetcher agree
// instead of holding the same mapping twice.

// d1MessagingTypes are the three short names row 8 moved.
var d1MessagingTypes = []string{"eb-rule", "kinesis", "msk"} //nolint:gochecknoglobals // test-only list

// A classifier that returned the first finding instead of the worst would read
// this row as a warning. Order is deliberate: the milder finding comes first,
// so first-wins and worst-wins give different answers.
func TestD1_MessagingWarnThenBrokenReadsAsBroken(t *testing.T) {
	for _, short := range d1MessagingTypes {
		t.Run(short, func(t *testing.T) {
			td := resource.FindResourceType(short)
			if td == nil {
				t.Fatalf("%s not registered", short)
			}
			r := resource.Resource{
				ID: short + "-probe",
				Findings: []domain.Finding{
					{Code: domain.FindingCode(short + ".probe.warn"), Phrase: "warn", Severity: domain.SevWarn, Source: "wave1"},
					{Code: domain.FindingCode(short + ".probe.broken"), Phrase: "broken", Severity: domain.SevBroken, Source: "wave1"},
				},
			}
			if got := td.ResolveColor(r); got != resource.ColorBroken {
				t.Errorf("ResolveColor with a warn and a broken finding = %v, want ColorBroken", got)
			}
		})
	}
}

// The bare-Fields path now runs the type's predicate. Every state the old
// switches enumerated has to land on the colour it landed on before, or the
// removal of those switches changed behaviour instead of removing a duplicate.
//
// eb-rule and kinesis have their own tables (qa_eb_rule_color_test.go,
// qa_kinesis_color_test.go) covering every case theirs enumerated, and both
// pass unchanged. What is left here is msk, which had no table at all, and the
// one eb-rule state its table omits.
func TestD1_MessagingBareFieldsMatchesThePredicate(t *testing.T) {
	cases := []struct {
		short string
		field string
		value string
		want  resource.Color
	}{
		{"eb-rule", "state", "ENABLED_WITH_ALL_CLOUDTRAIL_MANAGEMENT_EVENTS", resource.ColorHealthy},

		{"msk", "state", "ACTIVE", resource.ColorHealthy},
		{"msk", "state", "CREATING", resource.ColorWarning},
		{"msk", "state", "UPDATING", resource.ColorWarning},
		{"msk", "state", "MAINTENANCE", resource.ColorWarning},
		{"msk", "state", "REBOOTING_BROKER", resource.ColorWarning},
		{"msk", "state", "HEALING", resource.ColorWarning},
		{"msk", "state", "FAILED", resource.ColorBroken},
		{"msk", "state", "", resource.ColorHealthy},
		// A cluster being torn down was Healthy under the old switch, which had
		// no DELETING case. The predicate has always reported it, so moving the
		// classifier onto the predicate surfaced a state the switch swallowed.
		{"msk", "state", "DELETING", resource.ColorWarning},
	}

	for _, tc := range cases {
		t.Run(tc.short+"_"+tc.value, func(t *testing.T) {
			td := resource.FindResourceType(tc.short)
			if td == nil {
				t.Fatalf("%s not registered", tc.short)
			}
			got := td.ResolveColor(resource.Resource{Fields: map[string]string{tc.field: tc.value}})
			if got != tc.want {
				t.Errorf("%s %s=%q → %v, want %v", tc.short, tc.field, tc.value, got, tc.want)
			}
		})
	}
}

// d1MessagingBench fetches the demo rows for the three types through their
// catalog fetchers, the way the running app does.
func d1MessagingBench(t *testing.T) map[string][]resource.Resource {
	t.Helper()
	clients := &awsclient.ServiceClients{
		EventBridge: fakes.NewEventBridge(),
		Kinesis:     fakes.NewKinesis(),
		MSK:         fakes.NewMSK(),
	}

	rows := map[string][]resource.Resource{}
	for _, short := range d1MessagingTypes {
		td := catalog.FindAny(short)
		if td == nil || td.Fetcher == nil {
			t.Fatalf("%s has no catalog Fetcher", short)
		}
		page, err := td.Fetcher(context.Background(), clients, "")
		if err != nil && len(page.Resources) == 0 {
			t.Fatalf("demo %s fetch: %v", short, err)
		}
		if len(page.Resources) == 0 {
			t.Fatalf("demo %s returned no rows; the bench would assert nothing", short)
		}
		// A bench where every row is healthy passes the colour checks below
		// without exercising a single one of them.
		flagged := false
		for _, r := range page.Resources {
			if len(r.Findings) > 0 {
				flagged = true
				break
			}
		}
		if !flagged {
			t.Fatalf("no demo %s row carries a finding; the colour checks would be vacuous", short)
		}
		rows[short] = page.Resources
	}
	return rows
}

// Every demo row's colour must be the one its own findings imply. This is the
// check the two-step shape exists to pass: the guard answers for rows that
// carry findings, and the fallback answers identically for rows that do not.
func TestD1_MessagingBenchColoursMatchTheirFindings(t *testing.T) {
	for short, rows := range d1MessagingBench(t) {
		td := resource.FindResourceType(short)
		if td == nil {
			t.Fatalf("%s not registered", short)
		}
		for _, r := range rows {
			if got, want := td.ResolveColor(r), d1ColorOf(r.Findings); got != want {
				t.Errorf("%s row %q colour = %v, want %v from findings %+v", short, r.ID, got, want, r.Findings)
			}
		}
	}
}

// Stripping a fetched row of its findings must not change its colour: the
// predicate the classifier runs over Fields is the same one that produced them.
// A row where the two disagree is a second truth source that has drifted.
func TestD1_MessagingBareFieldsAgreeWithTheFetchedRow(t *testing.T) {
	for short, rows := range d1MessagingBench(t) {
		td := resource.FindResourceType(short)
		if td == nil {
			t.Fatalf("%s not registered", short)
		}
		for _, r := range rows {
			bare := resource.Resource{ID: r.ID, Fields: r.Fields}
			if got, want := td.ResolveColor(bare), td.ResolveColor(r); got != want {
				t.Errorf("%s row %q: without findings %v, with findings %v — the classifier and the predicate disagree; Fields = %v",
					short, r.ID, got, want, r.Fields)
			}
		}
	}
}
