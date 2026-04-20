package unit

// qa_logs_color_test.go — Behavioral tests for the logs (CloudWatch Log Groups) Color function.
//
// Contract assertions:
//   - retention_days set, stored_bytes>0, recent creation, kms_key_id set → ColorHealthy.
//   - retention_days empty (no retention policy) → ColorWarning.
//   - kms_key_id empty alone (retention set, not orphan) → ColorHealthy per
//     docs/attention-signals.md (KMS issue only triggers when key is PendingDeletion, a
//     cross-ref check, not "missing"). Changed from ColorWarning per CodeRabbit PR-273 finding.
//   - stored_bytes=0 with old creation_time (>90d orphan) → ColorWarning.
//   - Empty fields → ColorWarning (multiple defaults trigger Warning).

import (
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/internal/resource"
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
			name: "no_retention",
			fields: map[string]string{
				"retention_days": "",
				"stored_bytes":   "1024",
				"kms_key_id":     "arn:aws:kms:us-east-1:123456789012:key/aaaabbbb-1111-2222-3333-444455556666",
			},
			want: resource.ColorWarning,
		},
		{
			// CodeRabbit PR-273 finding: internal/resource/types_monitoring.go:68-69
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
			want:   resource.ColorWarning,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := td.Color(resource.Resource{Fields: tc.fields})
			if got != tc.want {
				t.Errorf("Color(%v) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}
