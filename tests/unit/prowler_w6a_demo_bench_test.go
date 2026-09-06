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
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/demo"
	"github.com/k2m30/a9s/v3/core/demo/fixtures"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

// w6aBatchTypes are the seven resource types this batch touches.
var w6aBatchTypes = []string{"trail", "logs", "alarm", "r53", "cf", "acm", "apigw"} //nolint:gochecknoglobals // test-only list

// w6aOffTheSharedFallback are the batch's types whose classifier still picks a
// colour of its own after the findings lookup, per
// qa_classifier_fields_source_gate_test.go's burn-down list. The other three
// hand their raw fields to the type's own findings predicate, which is the
// sanctioned shared fallback and not a second opinion.
var w6aOffTheSharedFallback = map[string]bool{ //nolint:gochecknoglobals // test-only lookup
	"logs": true, // colorLogs
	"cf":   true, // colorCF
	"acm":  true, // acmColor
	"r53":  true, // r53Color
}

// TestW6AColorDerivesFromFindings pins the classifier ruling for the batch:
// a type's colour is whatever its findings say, and nothing else.
//
// Two probes, because they fail on different defects. The warn-then-broken
// pair catches a classifier that stops at the first finding rather than the
// worst, and applies to every type. The findings-free probe catches a
// classifier that interprets a raw field itself and returns a colour no
// finding backs, so the row is coloured for a reason the detail view never
// names and the issue badge never counts; it applies only to the classifiers
// still off the shared fallback.
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
		"record_count":                "2",
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

			if !w6aOffTheSharedFallback[short] {
				return
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

// w6aWitness maps every code the batch emits to the fixture constant naming
// the one demo resource that must show it. Contract rule 8: the constant's
// value is the witness, so this table is the batch's bench contract.
var w6aWitness = map[string]struct {
	short   string
	witness string
}{ //nolint:gochecknoglobals // test-only table
	"trail.no-cloudwatch-logs":           {"trail", fixtures.TrailNoCWLogs},
	"trail.no-kms":                       {"trail", fixtures.TrailNoKMS},
	"trail.log-bucket-public":            {"trail", fixtures.TrailLogBucketPublic},
	"trail.log-bucket-no-access-logging": {"trail", fixtures.TrailLogBucketNoLogging},
	"logs.no-kms":                        {"logs", fixtures.LogGroupNoKMS},
	"alarm.actions-disabled":             {"alarm", fixtures.AlarmActionsDisabled},
	"r53.query-logging-off":              {"r53", fixtures.R53QueryLoggingOff},
	"r53.dangling-record":                {"r53", fixtures.R53DanglingA},
	"cf.origin-bucket-missing":           {"cf", fixtures.CFOriginBucketMissing},
	"cf.deprecated-tls":                  {"cf", fixtures.CFDeprecatedTLS},
	"cf.logging-off":                     {"cf", fixtures.CFLoggingOff},
	"cf.no-default-root-object":          {"cf", fixtures.CFNoRootObject},
	"cf.s3-origin-no-oac":                {"cf", fixtures.CFS3OriginNoOAC},
	"cf.default-certificate":             {"cf", fixtures.CFDefaultCert},
	"cf.no-geo-restriction":              {"cf", fixtures.CFNoGeoRestriction},
	"acm.weak-key":                       {"acm", fixtures.ACMWeakKey},
	"apigw.no-authorizer-public":         {"apigw", fixtures.APIGWRESTNoAuthorizer},
	"apigw.no-authorizer":                {"apigw", fixtures.APIGWHTTPNoAuthorizer},
	"apigw.no-access-logs":               {"apigw", fixtures.APIGWRESTNoAccessLogs},
	"apigw.tracing-off":                  {"apigw", fixtures.APIGWRESTTracingOff},
	"apigw.stage-variable-secret":        {"apigw", fixtures.APIGWRESTStageSecret},
}

// TestW6AEveryFindingFiresOnItsNamedWitnessOnly is the batch's bench gate,
// keyed by witness name rather than by code.
//
// The code-fires-somewhere gate it replaces asks only whether a finding
// appears on any demo row, which four of this batch's constants passed while
// naming a resource no fixture built: the finding rode on a pre-existing row
// and the constant pointed at nothing. It is also silent when a row fires on a
// third of a list — a bench where 35 of 40 log groups carry the same warning
// cannot show an operator which row the signal was built for.
//
// So this asserts both halves of contract rule 8 at once: exactly one demo row
// carries the code, and that row is the one the constant names.
func TestW6AEveryFindingFiresOnItsNamedWitnessOnly(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	for code, want := range w6aWitness {
		t.Run(code, func(t *testing.T) {
			td := resource.FindResourceType(want.short)
			if td == nil {
				t.Fatalf("%s not registered", want.short)
			}
			var carriers []string
			for _, res := range mergeWave2Findings(t, *td, byType[want.short], cache, clients) {
				for _, f := range res.Findings {
					if string(f.Code) != code {
						continue
					}
					// The witness usually names the row itself. r53's dangling
					// record is the exception the acceptance round accepted: the
					// finding lands on the zone that holds the record, which is
					// the only shape a per-zone row allows, so the record name
					// is matched where it actually renders — the finding's own
					// supporting rows.
					named := res.ID + " / " + res.Name
					for _, row := range res.AttentionDetails[f.Code].Rows {
						named += " / " + row.Value
					}
					carriers = append(carriers, named)
				}
			}
			switch {
			case len(carriers) == 0:
				t.Errorf("no demo row carries %q; its witness %q builds no row", code, want.witness)
			case len(carriers) > 1:
				t.Errorf("%d demo rows carry %q, want exactly the witness %q: %v",
					len(carriers), code, want.witness, carriers)
			case !strings.Contains(carriers[0], want.witness):
				t.Errorf("%q fires on %q, want the named witness %q", code, carriers[0], want.witness)
			}
		})
	}
}

// TestW6ADemoRowsRenderTheMappedWords pins rows 22 and 23 where they are read
// rather than where they are computed: on the demo row the renderer consumes.
//
// The per-row tests drive enrichers over hand-built input, so they pass on a
// value the demo never produces. These two rows exist because an SDK enum and
// an empty field each reached a rendered surface once already.
func TestW6ADemoRowsRenderTheMappedWords(t *testing.T) {
	clients := demo.NewServiceClients()
	byType, cache := buildVisibilityTypeCache(t)

	for _, tc := range []struct {
		short, witness, code, label, value string
	}{
		{"cf", fixtures.CFDeprecatedTLS, "cf.deprecated-tls", "Minimum TLS version", "TLS 1.0 (2016)"},
		{"apigw", fixtures.APIGWRESTNoAuthorizer, "apigw.no-authorizer-public", "Endpoint", "edge"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			td := resource.FindResourceType(tc.short)
			if td == nil {
				t.Fatalf("%s not registered", tc.short)
			}
			for _, res := range mergeWave2Findings(t, *td, byType[tc.short], cache, clients) {
				if res.ID != tc.witness && res.Name != tc.witness {
					continue
				}
				for _, row := range res.AttentionDetails[domain.FindingCode(tc.code)].Rows {
					if row.Label != tc.label {
						continue
					}
					if row.Value != tc.value {
						t.Errorf("%s row %q = %q, want %q", tc.code, tc.label, row.Value, tc.value)
					}
					return
				}
				t.Fatalf("%s on %s carries no %q row", tc.code, tc.witness, tc.label)
			}
			t.Fatalf("witness %q not found among the %s demo rows", tc.witness, tc.short)
		})
	}
}
