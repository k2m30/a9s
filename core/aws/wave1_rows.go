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

// wave1Finding builds a Wave-1 Finding, reading the operator sentence from the
// code's registered definition the way setWave2Finding does, so a finding has
// exactly one Detail wherever it is emitted from. It is the one place a Wave-1
// Finding is constructed: a fetcher that returns []domain.Finding rather than
// decorating a Resource calls this, so wave 1 has the seam wave 2 has in
// setWave2Finding.
func wave1Finding(code domain.FindingCode, phrase string, severity domain.Severity) domain.Finding {
	return domain.Finding{
		Code: code, Phrase: phrase, Detail: catalog.Detail(code), Severity: severity, Source: "wave1",
	}
}

// addWave1Finding appends the Wave-1 posture Finding for code to r.
func addWave1Finding(r *resource.Resource, code domain.FindingCode, phrase string, severity domain.Severity) {
	r.Findings = append(r.Findings, wave1Finding(code, phrase, severity))
}

// secretScanRows and secretScanTextRows scan the two shapes a credential
// arrives in — an environment-style map, and free text such as user data or a
// state-machine definition — and return one supporting row per credential
// found. Rows carry Where and Kind only: the value never reaches a rendered
// surface, which is the property worth having in one place rather than
// repeated at each caller. Every hit is a credential in the clear, so every
// row is one tier.
//
// Wave-2 enrichers call these directly and pack the rows into their own
// setWave2Finding call, because what precedes the hit rows differs per
// enricher: a Stage row per stage, a Container row per container, a Version
// row per launch template.
func secretScanRows(kv map[string]string) []domain.DetailRow {
	return secretScanHitRows(secretscan.ScanKV(kv))
}

func secretScanTextRows(text string) []domain.DetailRow {
	return secretScanHitRows(secretscan.ScanText(text))
}

func secretScanHitRows(hits []secretscan.Hit) []domain.DetailRow {
	var rows []domain.DetailRow
	for _, h := range hits {
		rows = append(rows, domain.DetailRow{Label: h.Where, Value: h.Kind, Tier: "!"})
	}
	return rows
}

// addSecretScanFinding scans kv and, on any hit, emits the Wave-1 finding
// plus its supporting rows.
func addSecretScanFinding(r *resource.Resource, code domain.FindingCode, phrase string, kv map[string]string) {
	rows := secretScanRows(kv)
	if len(rows) == 0 {
		return
	}
	addWave1Finding(r, code, phrase, domain.SevBroken)
	addWave1Rows(r, code, rows...)
}
