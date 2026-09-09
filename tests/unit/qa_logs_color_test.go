package unit

// qa_logs_color_test.go — the logs (CloudWatch Log Groups) Color function.
//
// The classifier runs the type's own predicate over Fields, so the states
// below are reported from the words the fetcher derives (`retention`,
// `encryption`) rather than raw keys.
//
// The rows naming raw keys therefore want Healthy: a row carrying
// `retention_days` and no `retention` word says nothing, which is what proves
// the classifier does not read it. The row naming the word wants the colour
// the word earns. kms_key_id empty alone is Healthy per
// docs/attention-signals.md (the KMS issue only triggers when the key is
// PendingDeletion, a cross-ref check, not "missing").
//
// `retention_days` is a key no fetcher writes — the log group's retention is
// one field, `retention`, carrying words. It stays in this table on purpose:
// a key colorLogs does not know must still leave the row healthy, and this
// is the only place that is pinned.

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
			// docs/attention-signals.md only raises a KMS issue when the
			// referenced key is PendingDeletion (a cross-ref check). Missing KMS
			// alone is not enough to warn.
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
			// The fields reach the type\'s own predicate, which produces the
			// finding, so the colour the table names is one the row carries for a
			// reason the detail view shows.
			got := td.Color(resource.Resource{Fields: tc.fields})
			if got != tc.want {
				t.Errorf("Color(%v) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}
