// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// wave1_rows.go — the Wave-1 counterpart of setWave2Finding's AttentionDetail
// packing. Fetchers that emit a Finding with supporting rows attach them here
// so the map keying (Resource.ID → FindingCode → rows) matches what
// ApplyWave2ToRow and buildAttentionEntries already read.
package aws

import (
	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/secretscan"
)

// addWave1Rows attaches supporting rows to a Wave-1 Finding already appended
// to r.Findings, keyed by that Finding's own Code so two independently
// evaluated conditions on one resource never crowd out each other's rows.
func addWave1Rows(r *resource.Resource, code domain.FindingCode, rows ...domain.DetailRow) {
	if len(rows) == 0 {
		return
	}
	if r.AttentionDetails == nil {
		r.AttentionDetails = make(map[domain.FindingCode]domain.AttentionDetail, 1)
	}
	ad := r.AttentionDetails[code]
	ad.Rows = append(ad.Rows, rows...)
	r.AttentionDetails[code] = ad
}

// addWave1Finding appends a Wave-1 posture Finding, reading the operator
// sentence from the code's registered definition the way setWave2Finding does,
// so a finding has exactly one Detail wherever it is emitted from.
func addWave1Finding(r *resource.Resource, code domain.FindingCode, phrase string, severity domain.Severity) {
	r.Findings = append(r.Findings, domain.Finding{
		Code: code, Phrase: phrase, Detail: catalog.Detail(code), Severity: severity, Source: "wave1",
	})
}

// addSecretScanFinding scans kv and, on any hit, emits the finding plus one
// supporting row per hit. Rows carry Where and Kind only — the value never
// reaches a rendered surface, which is the property worth having in one place
// rather than repeated at each caller.
func addSecretScanFinding(r *resource.Resource, code domain.FindingCode, phrase string, kv map[string]string) {
	hits := secretscan.ScanKV(kv)
	if len(hits) == 0 {
		return
	}
	addWave1Finding(r, code, phrase, domain.SevBroken)
	for _, h := range hits {
		addWave1Rows(r, code, domain.DetailRow{Label: h.Where, Value: h.Kind, Tier: "!"})
	}
}
