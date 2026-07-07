package unit

// qa_ebs_snap_color_test.go — Color contract pin for EBS Snapshots.
//
// Since the color-findings-conformance wave (qa_color_findings_conformance_test.go),
// colorEBSSnap is colorFromAnyFinding-only (internal/aws/catalog_compute.go) —
// it has NO raw-field fallback at all. Every non-healthy case here attaches a
// Finding shaped exactly like the real fetcher (internal/aws/ebs.go, wave1
// state Findings + ebsSnapStructuralFindings) or the real Wave-2 cross-ref
// enricher (internal/aws/ebs_snap_issue_enrichment.go, Source
// "wave2:ebs-snap"). Fields are kept for realism/context only — they are no
// longer read by Color.

import (
	"testing"
	"time"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

func TestEbsSnapColor(t *testing.T) {
	td := resource.FindResourceType("ebs-snap")
	if td == nil {
		t.Fatal("ebs-snap not registered")
	}

	cases := []struct {
		name     string
		fields   map[string]string
		findings []domain.Finding
		want     resource.Color
	}{
		{
			name:   "state_completed",
			fields: map[string]string{"state": "completed"},
			want:   resource.ColorHealthy,
		},
		{
			name:   "state_pending",
			fields: map[string]string{"state": "pending"},
			findings: []domain.Finding{
				{Code: "ebs-snap.state.pending", Phrase: "pending", Severity: domain.SevWarn, Source: "wave1"},
			},
			want: resource.ColorWarning,
		},
		{
			name:   "state_error",
			fields: map[string]string{"state": "error"},
			findings: []domain.Finding{
				{Code: "ebs-snap.state.error", Phrase: "error", Severity: domain.SevBroken, Source: "wave1"},
			},
			want: resource.ColorBroken,
		},
		{
			name:   "state_recoverable",
			fields: map[string]string{"state": "recoverable"},
			findings: []domain.Finding{
				{Code: "ebs-snap.state.error", Phrase: "error", Severity: domain.SevBroken, Source: "wave1"},
			},
			want: resource.ColorBroken,
		},
		{
			name:   "state_recovering",
			fields: map[string]string{"state": "recovering"},
			findings: []domain.Finding{
				{Code: "ebs-snap.state.error", Phrase: "error", Severity: domain.SevBroken, Source: "wave1"},
			},
			want: resource.ColorBroken,
		},
		{
			name:   "encrypted_false",
			fields: map[string]string{"state": "completed", "encrypted": "false"},
			findings: []domain.Finding{
				{
					Code: "ebs-snap.encryption.disabled", Phrase: "unencrypted",
					Detail:   "Snapshot is not encrypted at rest — re-create from an encrypted volume.",
					Severity: domain.SevWarn, Source: "wave1",
				},
			},
			want: resource.ColorWarning,
		},
		{
			name:   "encrypted_true",
			fields: map[string]string{"state": "completed", "encrypted": "true"},
			want:   resource.ColorHealthy,
		},
		{
			name: "automated_old",
			fields: map[string]string{
				"state":       "completed",
				"description": "Created by CreateImage(i-abc)",
				"started":     time.Now().AddDate(-2, 0, 0).Format(time.RFC3339),
			},
			findings: []domain.Finding{
				{
					Code: "ebs-snap.aged-automated", Phrase: "automated, 730d old",
					Detail:   "Automated snapshot is 730 days old with no retention policy pruning it — billed indefinitely.",
					Severity: domain.SevWarn, Source: "wave1",
				},
			},
			want: resource.ColorWarning,
		},
		{
			name: "automated_recent",
			fields: map[string]string{
				"state":       "completed",
				"description": "Created by CreateImage(i-abc)",
				"started":     time.Now().Format(time.RFC3339),
			},
			want: resource.ColorHealthy,
		},
		{
			name: "manual_old",
			fields: map[string]string{
				"state":       "completed",
				"description": "manual snap",
				"started":     time.Now().AddDate(-2, 0, 0).Format(time.RFC3339),
			},
			want: resource.ColorHealthy,
		},
		{
			name: "orphan_flag",
			fields: map[string]string{
				"state":         "completed",
				"volume_id":     "vol-deleted",
				"volume_orphan": "true",
			},
			findings: []domain.Finding{
				{
					Code: "ebs-snap.orphan", Phrase: "orphan: source volume deleted",
					Severity: domain.SevWarn, Source: "wave2:ebs-snap",
				},
			},
			want: resource.ColorWarning,
		},
		{
			// Synthetic multi-finding case (the real fetcher never emits both
			// state.error and encryption.disabled at once — see ebs.go's
			// switch/ebsSnapStructuralFindings guard) exercising
			// colorFromAnyFinding's max-severity-wins reduction directly:
			// SevBroken must win even when a SevWarn finding is also present.
			name: "broken_overrides_warning",
			fields: map[string]string{
				"state":     "error",
				"encrypted": "false",
			},
			findings: []domain.Finding{
				{Code: "ebs-snap.state.error", Phrase: "error", Severity: domain.SevBroken, Source: "wave1"},
				{
					Code: "ebs-snap.encryption.disabled", Phrase: "unencrypted",
					Severity: domain.SevWarn, Source: "wave1",
				},
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
