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
//
// colorEBSSnap is a bare `colorFromAnyFinding(r) or ColorHealthy` — it does
// not branch on finding Code, only on Severity, Source-prefix ("wave1" or
// "wave2:"), and worst-severity-wins across multiple Findings. Coverage below
// pins: the no-finding Healthy anchor, one wave1 case per severity tier, the
// wave2:ebs-snap Source-prefix acceptance (a bug class where a finding with
// the wrong Source string is silently dropped from color resolution), and
// the multi-finding worst-severity-wins reduction. Per-state wave1-emission
// mapping (which AWS state produces which code) is pinned at the fetcher
// layer, not here.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
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
