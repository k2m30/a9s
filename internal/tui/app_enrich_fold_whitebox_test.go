// SPDX-License-Identifier: GPL-3.0-or-later

package tui

// app_enrich_fold_whitebox_test.go — Model.applyEnrichment is the TUI's
// per-type Wave-2 clear, run by handleRefresh before a re-fetch. It is
// unexported and its only production caller passes no data, so nothing outside
// this package could reach it; the clear ran on every refresh with no test
// asserting what it keeps.
//
// What it must keep is the part that cannot be recomputed: wave-1 supporting
// rows come from data only the fetcher held, so a clear that drops them leaves
// the row permanently missing detail that no later fold restores.

import (
	"testing"

	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
)

func TestApplyEnrichment_ClearsWave2AndKeepsWave1SupportingRows(t *testing.T) {
	m := New("demo", "us-east-1")

	const (
		wave1Code domain.FindingCode = "ec2.imdsv1"
		wave2Code domain.FindingCode = "ec2.internet-exposed"
	)
	row := resource.Resource{
		ID:   "i-0a1b2c3d4e5f60001",
		Name: "acme-web-01",
		Type: "ec2",
		Findings: []domain.Finding{
			{Code: wave1Code, Phrase: "IMDSv1 allowed", Severity: domain.SevWarn, Source: "wave1"},
			{Code: wave2Code, Phrase: "port 22 reachable from the internet", Severity: domain.SevBroken, Source: "wave2:ec2"},
		},
		AttentionDetails: map[domain.FindingCode]domain.AttentionDetail{
			wave1Code: {Rows: []domain.DetailRow{{Label: "Metadata service", Value: "v1 and v2"}}},
			wave2Code: {Rows: []domain.DetailRow{{Label: "Open port", Value: "22"}}},
		},
	}
	m.core.SetResourceCache("ec2", &domain.ListViewCacheEntry{Resources: []domain.Resource{row}})

	m.applyEnrichment("ec2")

	entry, ok := m.core.AnyOriginResourceCache("ec2")
	if !ok || len(entry.Resources) != 1 {
		t.Fatalf("cached ec2 rows after the clear = %+v, want the one seeded row", entry)
	}
	got := entry.Resources[0]

	var codes []domain.FindingCode
	for _, f := range got.Findings {
		codes = append(codes, f.Code)
	}
	if len(codes) != 1 || codes[0] != wave1Code {
		t.Errorf("findings after the clear = %v, want only the wave-1 finding %q", codes, wave1Code)
	}

	if _, kept := got.AttentionDetails[wave1Code]; !kept {
		t.Errorf("the wave-1 supporting rows were dropped; only the fetcher holds that data, so nothing restores them: %+v",
			got.AttentionDetails)
	}
	if _, stale := got.AttentionDetails[wave2Code]; stale {
		t.Errorf("the wave-2 AttentionDetail survived the clear, so the next fold appends onto stale rows: %+v",
			got.AttentionDetails)
	}
	if rows := got.AttentionDetails[wave1Code].Rows; len(rows) != 1 || rows[0].Label != "Metadata service" || rows[0].Value != "v1 and v2" {
		t.Errorf("wave-1 rows after the clear = %+v, want the seeded Metadata service row unchanged", rows)
	}
}

// TestApplyEnrichment_LeavesOtherTypesAlone pins the scope: the clear is
// per-type, so refreshing one list must not strip the wave-2 findings another
// list is currently rendering.
func TestApplyEnrichment_LeavesOtherTypesAlone(t *testing.T) {
	m := New("demo", "us-east-1")

	const wave2Code domain.FindingCode = "s3.public"
	s3Row := resource.Resource{
		ID:   "acme-public-bucket",
		Type: "s3",
		Findings: []domain.Finding{
			{Code: wave2Code, Phrase: "readable by anyone", Severity: domain.SevBroken, Source: "wave2:s3"},
		},
	}
	m.core.SetResourceCache("s3", &domain.ListViewCacheEntry{Resources: []domain.Resource{s3Row}})
	m.core.SetResourceCache("ec2", &domain.ListViewCacheEntry{Resources: []domain.Resource{{ID: "i-0", Type: "ec2"}}})

	m.applyEnrichment("ec2")

	entry, ok := m.core.AnyOriginResourceCache("s3")
	if !ok || len(entry.Resources) != 1 {
		t.Fatalf("cached s3 rows = %+v, want the one seeded row", entry)
	}
	if len(entry.Resources[0].Findings) != 1 || entry.Resources[0].Findings[0].Code != wave2Code {
		t.Errorf("refreshing ec2 stripped s3's wave-2 finding: %+v", entry.Resources[0].Findings)
	}
}
