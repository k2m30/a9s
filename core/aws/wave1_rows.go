// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// wave1_rows.go — the Wave-1 counterpart of setWave2Finding's AttentionDetail
// packing. Fetchers that emit a Finding with supporting rows attach them here
// so the map keying (Resource.ID → FindingCode → rows) matches what
// ApplyWave2ToRow and buildAttentionEntries already read.
package aws

import (
	"strings"

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
	ad.Rows = capRows(ad.Rows, rows)
	r.AttentionDetails[code] = ad
}

// wave1Finding builds a Wave-1 Finding. It is the one place a Wave-1 Finding
// is constructed, and the phrase it carries is the one the code's
// catalog.FindingDef declares, so a wording exists in exactly one place and
// the rendered row, the detail view and the generated signals page cannot
// drift apart.
//
// values fill the declared phrase's "<…>" slots left to right — a code whose
// wording carries a measurement ("expires in <N> days") passes the
// measurement, never a sentence it assembled itself.
func wave1Finding(code domain.FindingCode, severity domain.Severity, values ...string) domain.Finding {
	return domain.Finding{
		Code: code, Phrase: fillPhrase(catalog.Phrase(code), values...),
		Detail: catalog.Detail(code), Severity: severity, Source: "wave1",
	}
}

// stateFinding is one row of a lifecycle lookup table: the code a state maps
// to and the severity it carries. The wording is not here — wave1Finding
// reads it from the code's declaration, so a table cannot become a second
// phrase list.
type stateFinding struct {
	code     domain.FindingCode
	severity domain.Severity
}

// fillPhrase substitutes values into the "<…>" slots of a declared phrase, in
// order. A value with no slot left to fill is dropped rather than appended:
// the declared wording is the shape of the sentence, and a caller passing more
// values than the declaration has slots is caught by the phrase gate, not
// papered over with text nobody registered.
func fillPhrase(phrase string, values ...string) string {
	// done is where the last value ended: scanning resumes past it, because a
	// value AWS supplied may itself contain a "<" and must not be re-read as
	// the next slot's opening bracket.
	done := 0
	for _, v := range values {
		open := strings.Index(phrase[done:], "<")
		if open < 0 {
			break
		}
		open += done
		closeAt := strings.Index(phrase[open:], ">")
		if closeAt < 0 {
			break
		}
		phrase = phrase[:open] + v + phrase[open+closeAt+1:]
		done = open + len(v)
	}
	return phrase
}

// addWave1Finding appends the Wave-1 posture Finding for code to r.
func addWave1Finding(r *resource.Resource, code domain.FindingCode, severity domain.Severity, values ...string) {
	r.Findings = append(r.Findings, wave1Finding(code, severity, values...))
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
func addSecretScanFinding(r *resource.Resource, code domain.FindingCode, kv map[string]string) {
	rows := secretScanRows(kv)
	if len(rows) == 0 {
		return
	}
	addWave1Finding(r, code, domain.SevBroken)
	addWave1Rows(r, code, rows...)
}
