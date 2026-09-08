package unit

// qa_logs_color_test.go — Behavioral tests for the logs (CloudWatch Log Groups) Color function.
//
// Contract assertions:
//   - retention_days set, stored_bytes>0, recent creation, kms_key_id set → ColorHealthy.
//   - retention_days empty, with no `retention` word → ColorHealthy.
//   - kms_key_id empty alone (retention set, not orphan) → ColorHealthy per
//     docs/attention-signals.md (KMS issue only triggers when key is PendingDeletion, a
//     cross-ref check, not "missing"). Changed from ColorWarning per CodeRabbit PR-273 finding.
//   - stored_bytes=0 with old creation_time (>90d orphan) → ColorWarning.
//   - Empty fields → ColorHealthy: an absent word is unknown, not bad.

// The raw-field branch this table was written against is gone twice over. w6a
// removed it, and w29 gave the classifier the type's own predicate over Fields
// instead — so the states below are reported again, but from the words the
// fetcher derives (`retention`, `encryption`) rather than the raw keys the old
// branch read.
//
// The rows naming raw keys therefore want Healthy: a row carrying
// `retention_days` and no `retention` word says nothing, which is what proves
// the classifier stopped reading it. The row naming the word wants the colour
// the word earns.
//
// `retention_days` is a key no fetcher writes anymore — aws5 row 1 collapsed
// the log group's retention into one field, `retention`, carrying words. It
// stays in this table on purpose: a key colorLogs does not know must still
// leave the row healthy, and this is the only place that is pinned.

import (
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/core/resource"
)

func TestLogsColor(t *testing.T) {
	td := resource.FindResourceType("logs")
	if td == nil {
		t.Fatal("logs not registered")
	}

	now := time.Now()
	recentCreation := now.Add(-24 * time.Hour).Format("2006-01-02 15:04")
	oldCreation := now.Add(-100 * 24 * time.Hour).Format("2006-01-02 15:04")

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		{
			name: "healthy",
			fields: map[string]string{
				"retention_days": "30",
				"stored_bytes":   "1024",
				"creation_time":  recentCreation,
				"kms_key_id":     "arn:aws:kms:us-east-1:123456789012:key/aaaabbbb-1111-2222-3333-444455556666",
			},
			want: resource.ColorHealthy,
		},
		{
			// The raw key alone, with no derived word beside it: nothing to
			// report, which is how this table catches the branch coming back.
			name: "no_retention_raw_key_only",
			fields: map[string]string{
				"retention_days": "",
				"stored_bytes":   "1024",
				"kms_key_id":     "arn:aws:kms:us-east-1:123456789012:key/aaaabbbb-1111-2222-3333-444455556666",
			},
			want: resource.ColorHealthy,
		},
		{
			// The same state in the vocabulary the classifier reads now.
			name:   "retention_never_expires",
			fields: map[string]string{"retention": "never expire"},
			want:   resource.ColorWarning,
		},
		{
			// CodeRabbit PR-273 finding: core/resource/types_monitoring.go:68-69
			// currently returns ColorWarning when kms_key_id is empty, but
			// docs/attention-signals.md only raises a KMS issue when the referenced
			// key is PendingDeletion (a cross-ref check). Missing KMS alone is not
			// enough to warn. This test will FAIL until the production colorer is fixed.
			name: "no_kms",
			fields: map[string]string{
				"retention_days": "30",
				"stored_bytes":   "1024",
				"kms_key_id":     "",
			},
			want: resource.ColorHealthy,
		},
		{
			// Explicit regression: a log group with retention set, data stored, and
			// no KMS key must not be flagged as Warning (kms_key_id alone is not an
			// actionable signal per docs/attention-signals.md).
			name: "kms_alone_should_not_warn",
			fields: map[string]string{
				"retention_days": "30",
				"stored_bytes":   "1024",
				"kms_key_id":     "",
			},
			want: resource.ColorHealthy,
		},
		{
			name: "orphan",
			fields: map[string]string{
				"retention_days": "30",
				"stored_bytes":   "0 B",
				"creation_time":  oldCreation,
				"kms_key_id":     "arn:aws:kms:us-east-1:123456789012:key/aaaabbbb-1111-2222-3333-444455556666",
			},
			want: resource.ColorWarning,
		},
		{
			name:   "empty",
			fields: map[string]string{},
			want:   resource.ColorHealthy,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Each case asserts its own want again. w6a made the driver demand
			// Healthy for all of them, on the reading that a colour with no
			// finding behind it is a colour nobody can explain. w29 converted
			// this classifier: the fields reach the type's own predicate, which
			// produces the finding, so the colour the table always named is the
			// one the row now carries for a reason the detail view shows.
			got := td.Color(resource.Resource{Fields: tc.fields})
			if got != tc.want {
				t.Errorf("Color(%v) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}
