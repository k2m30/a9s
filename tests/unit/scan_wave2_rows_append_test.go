package unit

// scan_wave2_rows_append_test.go — a Wave-2 finding raised twice for one
// resource keeps every emission's supporting rows.
//
// setWave2Finding assigns AttentionDetails[id][code] where its Wave-1
// counterpart addWave1Rows appends, so the second emission of a code silently
// replaces the first's rows. The reader is then told about one offending
// stage of two, with nothing to say the other was inspected at all.
//
// API Gateway is where the shape is reachable: the REST enricher walks the
// stages of one API and raises the same three per-stage codes once per
// offending stage, so a two-stage API drives the helper exactly as any other
// enricher raising one code twice for one resource would.

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	apigwtypes "github.com/aws/aws-sdk-go-v2/service/apigateway/types"
	"github.com/k2m30/a9s/v3/core/domain"
)

func TestAPIGWTwoOffendingStages_EveryStageKeepsItsRows(t *testing.T) {
	const apiID = "rst012twostage"

	offending := func(stage, secretKey string) apigwtypes.Stage {
		st := w6aV1Stage(stage)
		st.AccessLogSettings = nil
		st.TracingEnabled = false
		st.Variables = map[string]string{
			"backendStage": stage,
			secretKey:      "hunter2-not-a-real-secret",
		}
		return st
	}

	res := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{
			authorizers: []apigwtypes.Authorizer{{Id: aws.String("auth01")}},
			stages: []apigwtypes.Stage{
				offending("prod", "DB_PASSWORD"),
				offending("staging", "API_KEY"),
			},
		},
		&w6aAPIGWV2Fake{},
		w6aRESTRes(apiID, "acme-two-stage-rest", "REGIONAL", ""),
	)

	for _, code := range []string{w6aAPIGWNoAccessLogs, w6aAPIGWTracingOff, w6aAPIGWStageSecret} {
		t.Run(code, func(t *testing.T) {
			var raised int
			for _, f := range res.Findings[apiID] {
				if string(f.Code) == code {
					raised++
				}
			}
			if raised != 1 {
				t.Errorf("%s is raised %d times on %s, want 1 — one resource states one condition once, however many of its stages trip it", code, raised, apiID)
			}

			var stages []string
			for _, row := range w2Rows(t, res, apiID, code) {
				if row.Label == "Stage" {
					stages = append(stages, row.Value)
				}
			}
			if len(stages) != 2 || stages[0] != "prod" || stages[1] != "staging" {
				t.Errorf("%s names stages %v, want [prod staging] — the second stage's rows must not replace the first's", code, stages)
			}
		})
	}

	// The secret code carries a hit row per stage as well as the Stage row, so
	// a fix that keeps the Stage rows by dropping everything else is caught.
	rows := w2Rows(t, res, apiID, w6aAPIGWStageSecret)
	want := []domain.DetailRow{
		{Label: "Stage", Value: "prod", Tier: "!"},
		{Label: "DB_PASSWORD", Value: "keyword", Tier: "!"},
		{Label: "Stage", Value: "staging", Tier: "!"},
		{Label: "API_KEY", Value: "keyword", Tier: "!"},
	}
	if len(rows) != len(want) {
		t.Fatalf("%s rows = %+v, want %+v", w6aAPIGWStageSecret, rows, want)
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Errorf("%s rows[%d] = %+v, want %+v", w6aAPIGWStageSecret, i, rows[i], want[i])
		}
	}
}

// The negative: one offending stage on a two-stage API still names one stage,
// so the pin above cannot pass by listing every stage whether or not it trips
// the condition.
func TestAPIGWOneOffendingStageOfTwo_NamesOnlyThatStage(t *testing.T) {
	const apiID = "rst013onebad"

	bad := w6aV1Stage("prod")
	bad.TracingEnabled = false

	res := w6aEnrichAPIGW(t,
		&w6aAPIGWV1Fake{
			authorizers: []apigwtypes.Authorizer{{Id: aws.String("auth01")}},
			stages:      []apigwtypes.Stage{bad, w6aV1Stage("staging")},
		},
		&w6aAPIGWV2Fake{},
		w6aRESTRes(apiID, "acme-one-bad-stage-rest", "REGIONAL", ""),
	)

	rows := w2Rows(t, res, apiID, w6aAPIGWTracingOff)
	want := []domain.DetailRow{{Label: "Stage", Value: "prod", Tier: "~"}}
	if len(rows) != 1 || rows[0] != want[0] {
		t.Errorf("%s rows = %+v, want %+v", w6aAPIGWTracingOff, rows, want)
	}
	w2AssertNoCode(t, res.Findings[apiID], w6aAPIGWNoAccessLogs)
}
