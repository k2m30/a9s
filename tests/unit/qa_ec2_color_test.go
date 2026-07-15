package unit

// qa_ec2_color_test.go — Color contract pin for EC2 Instances.
//
// Since the color-findings-conformance wave (qa_color_findings_conformance_test.go),
// colorEC2 is colorFromAnyFinding-only (internal/aws/catalog_compute.go) — it
// has NO raw-field fallback at all. Color is entirely derived from Findings,
// so every non-healthy case here must attach a Finding shaped exactly like
// the real fetcher (internal/aws/ec2.go, wave1 lifecycle Findings) or the
// real Wave-2 enricher (internal/aws/ec2_issue_enrichment.go, Source
// "wave2:ec2") would produce. Fields are kept for realism/context only —
// they are no longer read by Color.
//
// colorEC2 is a bare `colorFromAnyFinding(r) or ColorHealthy` — it does not
// branch on finding Code, only on Severity and Source-prefix ("wave1" or
// "wave2:"). One representative case per severity tier (plus the no-finding
// Healthy anchor) exercises every branch colorEC2 can take, and one wave2:ec2
// case pins the Source-prefix acceptance (a bug class where a finding with
// the wrong Source string is silently dropped from color resolution).
// Per-state wave1-emission mapping (which AWS state produces which code) is
// pinned at the fetcher layer, not here.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestEc2Color(t *testing.T) {
	td := resource.FindResourceType("ec2")
	if td == nil {
		t.Fatal("ec2 not registered")
	}

	cases := []struct {
		name     string
		fields   map[string]string
		findings []domain.Finding
		want     resource.Color
	}{
		{
			// Healthy running instance with ok status checks — no Finding at all.
			name:   "running",
			fields: map[string]string{"state": "running", "system_status": "ok", "instance_status": "ok"},
			want:   resource.ColorHealthy,
		},
		{
			// Pending instance — transitioning state. Wave-1 finding per ec2.go.
			name:   "pending",
			fields: map[string]string{"state": "pending"},
			findings: []domain.Finding{
				{Code: "ec2.state.pending", Phrase: "pending", Severity: domain.SevWarn, Source: "wave1"},
			},
			want: resource.ColorWarning,
		},
		{
			// Stopped instance with Server.* reason code — AWS forced the stop
			// (capacity issue). This is unexpected and warrants Broken.
			name: "stopped_capacity",
			fields: map[string]string{
				"state":             "stopped",
				"state_reason_code": "Server.InsufficientInstanceCapacity",
			},
			findings: []domain.Finding{
				{Code: "ec2.state.stopped.server", Phrase: "stopped", Severity: domain.SevBroken, Source: "wave1"},
			},
			want: resource.ColorBroken,
		},
		{
			// Terminated instance — end-of-life state. Now emits a SevDim Finding
			// (wave #42): terminated is no longer a silent "no finding" state.
			name:   "terminated",
			fields: map[string]string{"state": "terminated"},
			findings: []domain.Finding{
				{Code: "ec2.state.terminated", Phrase: "terminated", Severity: domain.SevDim, Source: "wave1"},
			},
			want: resource.ColorDim,
		},
		{
			// Running instance with impaired instance status check (Wave 2
			// enricher EnrichEC2InstanceStatus, Source "wave2:ec2"). Must
			// override the (findings-absent) healthy state color.
			name: "impaired_via_enricher",
			fields: map[string]string{
				"state":           "running",
				"instance_status": "impaired",
				"system_status":   "ok",
			},
			findings: []domain.Finding{
				{
					Code: "ec2.instance-status-impaired", Phrase: "impaired: system checks failing",
					Severity: domain.SevBroken, Source: "wave2:ec2",
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
