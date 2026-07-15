package unit

// qa_lambda_color_test.go — Color contract pin for Lambda functions.
//
// Since the color-findings-conformance wave (qa_color_findings_conformance_test.go),
// colorLambda is colorFromAnyFinding-only (core/aws/catalog_compute.go) —
// it has NO raw-field fallback at all. Every non-healthy case here attaches a
// Finding shaped exactly like the real fetcher (core/aws/lambda.go, wave1
// Findings, codes in lambda_codes.go), whose switch fires exactly ONE Finding
// in precedence order: last-update failure, then deprecated runtime, then
// lifecycle state, then no-DLQ fallback. Fields are kept for realism/context
// only — they are no longer read by Color.
//
// colorLambda is a bare `colorFromAnyFinding(r) or ColorHealthy` — it does
// not branch on finding Code, only on Severity, Source-prefix, and
// worst-severity-wins across multiple Findings. One representative case per
// severity tier (plus the no-finding Healthy anchor) exercises every branch
// colorLambda can take, and the multi-finding case pins the worst-wins
// reduction. Which structural signal (last-update failure, deprecated
// runtime, lifecycle state, missing DLQ) produces which code/severity is
// pinned at the fetcher layer, not here.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestLambdaColor_StateAndOverrides(t *testing.T) {
	td := resource.FindResourceType("lambda")
	if td == nil {
		t.Fatal("lambda not registered")
	}

	cases := []struct {
		name     string
		fields   map[string]string
		findings []domain.Finding
		want     resource.Color
	}{
		{
			name:   "state=Active",
			fields: map[string]string{"state": "Active", "dlq_target_arn": "arn:aws:sqs:us-east-1:123456789012:my-queue"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "state=Pending",
			fields: map[string]string{"state": "Pending", "dlq_target_arn": "arn:aws:sqs:us-east-1:123456789012:my-queue"},
			findings: []domain.Finding{
				{Code: "lambda.state.pending", Phrase: "pending", Severity: domain.SevWarn, Source: "wave1"},
			},
			want: resource.ColorWarning,
		},
		{
			name:   "state=Inactive",
			fields: map[string]string{"state": "Inactive", "dlq_target_arn": "arn:aws:sqs:us-east-1:123456789012:my-queue"},
			findings: []domain.Finding{
				{
					Code: "lambda.state.inactive", Phrase: "inactive, evicted after extended idle time",
					Severity: domain.SevDim, Source: "wave1",
				},
			},
			want: resource.ColorDim,
		},
		{
			name:   "state=Failed",
			fields: map[string]string{"state": "Failed", "dlq_target_arn": "arn:aws:sqs:us-east-1:123456789012:my-queue"},
			findings: []domain.Finding{
				{Code: "lambda.state.failed", Phrase: "failed", Severity: domain.SevBroken, Source: "wave1"},
			},
			want: resource.ColorBroken,
		},
		{
			// Synthetic multi-finding case (the real fetcher's switch only
			// ever emits ONE Finding, last-update-failed taking top
			// precedence — see lambda.go) exercising colorFromAnyFinding's
			// max-severity-wins reduction directly: SevBroken must win even
			// when a SevWarn finding (no-DLQ) is also present.
			name: "all_signals_broken_wins_over_warning",
			fields: map[string]string{
				"state":              "Active",
				"last_update_status": "Failed",
				"runtime":            "python3.7",
				"dlq_target_arn":     "",
			},
			findings: []domain.Finding{
				{
					Code: "lambda.last-update.failed", Phrase: "last update failed to apply",
					Severity: domain.SevBroken, Source: "wave1",
				},
				{Code: "lambda.dlq.missing", Phrase: "no dead-letter queue configured", Severity: domain.SevWarn, Source: "wave1"},
			},
			want: resource.ColorBroken,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := td.Color(resource.Resource{Fields: tc.fields, Findings: tc.findings})
			if got != tc.want {
				t.Errorf("Color(%v, findings=%v) = %v, want %v", tc.fields, tc.findings, got, tc.want)
			}
		})
	}
}
