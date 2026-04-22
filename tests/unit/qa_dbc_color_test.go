package unit

import (
	"testing"

	"github.com/k2m30/a9s/v3/internal/resource"
)

// TestDbcColor tests the Color function for DB Clusters (dbc).
//
// The production Color func reads "status" (the phrase-based key populated by
// the fetcher after Wave-1 enrichment), strips any (+N) suffix via
// StripFindingSuffix, then matches against explicit Broken/Warning/Healthy phrases.
// Transitional statuses carry the suffix ": in progress".
//
// Coverage targets every branch of the dbc Color switch:
//   - Empty status → Healthy
//   - Broken phrases: "failed: cluster operation", "encryption key unreachable",
//     "parameter group incompatible", "no writer: reads only"
//   - Warning phrases: "delete-protection off", "not encrypted at rest",
//     "no automated backups"
//   - Wave-2 phrase: "maintenance overdue" → Healthy (green so "!" glyph renders)
//   - Transitional "* : in progress" suffix → Warning
//   - Stacked (+N) suffix on any phrase → stripped before matching
//   - Unknown phrase → Healthy (future-proof)
func TestDbcColor(t *testing.T) {
	td := resource.FindResourceType("dbc")
	if td == nil {
		t.Fatal("dbc not registered")
	}

	cases := []struct {
		name   string
		fields map[string]string
		want   resource.Color
	}{
		// ── ColorHealthy ────────────────────────────────────────────────────────
		{
			name:   "empty_status",
			fields: map[string]string{"status": ""},
			want:   resource.ColorHealthy,
		},
		{
			name:   "nil_fields",
			fields: nil,
			want:   resource.ColorHealthy,
		},
		{
			name:   "unknown_status_future_proof",
			fields: map[string]string{"status": "some-future-aws-status"},
			want:   resource.ColorHealthy,
		},
		// Wave-2 phrase "maintenance overdue" stays green so the "!" glyph renders.
		{
			name:   "maintenance_overdue",
			fields: map[string]string{"status": "maintenance overdue"},
			want:   resource.ColorHealthy,
		},

		// ── ColorBroken ──────────────────────────────────────────────────────────
		{
			name:   "failed_cluster_operation",
			fields: map[string]string{"status": "failed: cluster operation"},
			want:   resource.ColorBroken,
		},
		{
			name:   "encryption_key_unreachable",
			fields: map[string]string{"status": "encryption key unreachable"},
			want:   resource.ColorBroken,
		},
		{
			name:   "parameter_group_incompatible",
			fields: map[string]string{"status": "parameter group incompatible"},
			want:   resource.ColorBroken,
		},
		{
			name:   "no_writer_reads_only",
			fields: map[string]string{"status": "no writer: reads only"},
			want:   resource.ColorBroken,
		},

		// ── ColorWarning ─────────────────────────────────────────────────────────
		{
			name:   "delete_protection_off",
			fields: map[string]string{"status": "delete-protection off"},
			want:   resource.ColorWarning,
		},
		{
			name:   "not_encrypted_at_rest",
			fields: map[string]string{"status": "not encrypted at rest"},
			want:   resource.ColorWarning,
		},
		{
			name:   "no_automated_backups",
			fields: map[string]string{"status": "no automated backups"},
			want:   resource.ColorWarning,
		},
		// Transitional statuses carry the ": in progress" suffix.
		{
			name:   "creating_in_progress",
			fields: map[string]string{"status": "creating: in progress"},
			want:   resource.ColorWarning,
		},
		{
			name:   "modifying_in_progress",
			fields: map[string]string{"status": "modifying: in progress"},
			want:   resource.ColorWarning,
		},
		{
			name:   "deleting_in_progress",
			fields: map[string]string{"status": "deleting: in progress"},
			want:   resource.ColorWarning,
		},

		// ── (+N) suffix stripping ────────────────────────────────────────────────
		// Stacked findings append "(+N)" — must be stripped before phrase matching.
		{
			name:   "broken_with_stacked_suffix",
			fields: map[string]string{"status": "failed: cluster operation (+1)"},
			want:   resource.ColorBroken,
		},
		{
			name:   "warning_with_stacked_suffix",
			fields: map[string]string{"status": "no automated backups (+2)"},
			want:   resource.ColorWarning,
		},
		{
			name:   "maintenance_overdue_with_stacked_suffix",
			fields: map[string]string{"status": "maintenance overdue (+1)"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "delete_protection_off_with_suffix",
			fields: map[string]string{"status": "delete-protection off (+3)"},
			want:   resource.ColorWarning,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := td.Color(resource.Resource{Fields: tc.fields})
			if got != tc.want {
				t.Errorf("Color(%v) = %v, want %v", tc.fields, got, tc.want)
			}
		})
	}
}
