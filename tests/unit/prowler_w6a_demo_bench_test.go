package unit_test

// prowler_w6a_demo_bench_test.go — the two pins that apply to batch w6a as a
// whole rather than to one row: colour derives from findings for every type
// in the batch, and no supporting row restates the phrase it sits under.
//
// Both run over the demo fixtures through the same path the app uses, so they
// catch what a per-row test cannot: a row that is individually correct but
// says nothing new, and a classifier that still decides the colour from a raw
// field once real fixture data reaches it.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// w6aBatchTypes are the seven resource types this batch touches.
var w6aBatchTypes = []string{"trail", "logs", "alarm", "r53", "cf", "acm", "apigw"} //nolint:gochecknoglobals // test-only list

// TestW6AColorDerivesFromFindings pins the classifier ruling for the batch:
// a type's colour is whatever its findings say, and nothing else.
//
// Two probes, because they fail on different defects. The warn-then-broken
// pair catches a classifier that stops at the first finding rather than the
// worst. The findings-free probe catches the one this ruling is actually
// about: a classifier that interprets a raw field itself and returns a colour
// no finding backs, so the row is coloured for a reason the detail view never
// names and the issue badge never counts.
func TestW6AColorDerivesFromFindings(t *testing.T) {
	// Values the raw-field branches read as unhealthy: an unset retention, a
	// zero size on an old group, a trail that is not logging. A classifier
	// still consulting them colours this resource; one that does not, cannot.
	unhealthyLookingFields := map[string]string{
		"stored_bytes":                "0 B",
		"creation_time":               "2020-01-01 00:00",
		"is_logging":                  "false",
		"latest_delivery_error":       "AccessDenied",
		"log_file_validation_enabled": "false",
		"actions_count":               "0",
		"state":                       "INSUFFICIENT_DATA",
		"status":                      "EXPIRED",
		"enabled":                     "false",
	}

	for _, short := range w6aBatchTypes {
		t.Run(short, func(t *testing.T) {
			td := resource.FindResourceType(short)
			if td == nil {
				t.Fatalf("%s not registered", short)
			}

			worst := resource.Resource{
				ID: short + "-probe",
				Findings: []domain.Finding{
					{Code: domain.FindingCode(short + ".probe.warn"), Phrase: "warn", Severity: domain.SevWarn, Source: "wave1"},
					{Code: domain.FindingCode(short + ".probe.broken"), Phrase: "broken", Severity: domain.SevBroken, Source: "wave1"},
				},
			}
			if got := td.ResolveColor(worst); got != resource.ColorBroken {
				t.Errorf("ResolveColor with a warn and a broken finding = %v, want ColorBroken", got)
			}

			bare := resource.Resource{ID: short + "-bare", Fields: unhealthyLookingFields}
			if got := td.ResolveColor(bare); got != resource.ColorHealthy {
				t.Errorf("ResolveColor with no findings = %v, want ColorHealthy: the classifier still reads a raw field", got)
			}
		})
	}
}

// TestW6ADetailAttentionNeverRepeatsItself pins U11 for the batch: a
// supporting row must add something its own finding's phrase does not already
// say. The row and the phrase render one line apart, so "Query logging: off"
// under "query logging off" prints one fact twice. Per-row tests assert a row
// exists; only this one catches a row that merely restates its phrase.
func TestW6ADetailAttentionNeverRepeatsItself(t *testing.T) {
	batch := make(map[string]bool, len(w6aBatchTypes))
	for _, s := range w6aBatchTypes {
		batch[s] = true
	}

	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	for _, td := range resource.AllResourceTypes() {
		if !batch[td.ShortName] {
			continue
		}
		rows := byType[td.ShortName]
		if len(rows) == 0 {
			continue
		}
		for _, res := range mergeWave2Findings(t, td, rows, cache, clients) {
			for _, f := range res.Findings {
				ad, ok := res.AttentionDetails[f.Code]
				if !ok {
					continue
				}
				phrase := normalizeRowText(f.Phrase)
				for _, row := range ad.Rows {
					switch {
					case normalizeRowText(row.Label+" "+row.Value) == phrase,
						normalizeRowText(row.Value) == phrase:
						t.Errorf("%s/%s %s: row %q: %q adds nothing to the phrase %q",
							td.ShortName, res.ID, f.Code, row.Label, row.Value, f.Phrase)
					}
				}
			}
		}
	}
}
