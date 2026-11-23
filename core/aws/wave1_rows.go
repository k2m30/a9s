// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

// wave1_rows.go — the Wave-1 counterpart of setWave2Finding's AttentionDetail
// packing. Fetchers that emit a Finding with supporting rows attach them here
// so the map keying (Resource.ID → FindingCode → rows) matches what
// ApplyWave2ToRow and buildAttentionEntries already read.
package aws

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/k2m30/a9s/v3/core/catalog"
	"github.com/k2m30/a9s/v3/core/config"
	"github.com/k2m30/a9s/v3/core/domain"
	"github.com/k2m30/a9s/v3/core/resource"
	"github.com/k2m30/a9s/v3/core/secretscan"
)

// addWave1Rows attaches supporting rows to a Wave-1 Finding already appended
// to r.Findings, keyed by that Finding's own Code so two independently
// evaluated conditions on one resource never crowd out each other's rows. A
// finding the emit door withheld takes no rows.
func addWave1Rows(r *resource.Resource, code domain.FindingCode, rows ...domain.DetailRow) {
	if len(rows) == 0 || !slices.ContainsFunc(r.Findings, func(f domain.Finding) bool { return f.Code == code }) {
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
// wording carries a measurement ("expires in <N day(s)>") passes the
// measurement, never a sentence it assembled itself. The severity is the
// code's own too: an emitter that needs two tiers needs two codes, so that a
// row's colour and the tier the generated signals page prints have one owner.
func wave1Finding(code domain.FindingCode, values ...string) domain.Finding {
	return domain.Finding{
		Code: code, Phrase: fillPhrase(catalog.Phrase(code), values...),
		Detail: catalog.Detail(code), Severity: catalog.Severity(code), Source: "wave1",
	}
}

// tierOf is the detail-row glyph for a finding's supporting rows: "!" when
// the code is declared Broken, "~" otherwise. The rows wear the tier the code
// declares, so a row cannot claim an urgency the finding does not have.
func tierOf(code domain.FindingCode) string {
	if catalog.Severity(code) == domain.SevBroken {
		return "!"
	}
	return "~"
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
		filled := fillSlot(phrase[open+1:open+closeAt], v)
		phrase = phrase[:open] + filled + phrase[open+closeAt+1:]
		done = open + len(filled)
	}
	return phrase
}

// pluralMarker marks a noun inside a slot that agrees with the slot's value.
// The tokens say where the value lands and what it counts: slotCountTokens are
// numbers, slotListToken is a comma-separated list whose length is the count.
// The count tokens are the two the declared wordings already spell, in
// "<N> of <M>".
const (
	pluralMarker  = "(s)"
	slotListToken = "LIST"
)

var slotCountTokens = []string{"N", "M"}

// countTokenIn returns the count token slot names, or "" when it names none.
func countTokenIn(slot string) string {
	for _, tok := range slotCountTokens {
		if strings.Contains(slot, tok) {
			return tok
		}
	}
	return ""
}

// fillSlot renders one "<…>" slot's content with value.
//
// A slot carrying no pluralMarker is a placeholder and nothing more: the value
// replaces it whole, which is every declaration that names a status, an error
// or an identifier. A slot carrying one is a shape — "<N day(s)>",
// "<port(s) LIST>" — and number agreement is read off it here rather than at
// the emit site, so a wording and its singular live in the same declaration
// and an emitter passes the number or the list and nothing else.
func fillSlot(slot, value string) string {
	// A slot value becomes cell text, so it reads the way every other cell
	// reads: config.CanonicalValue is the one place those conventions live.
	value = config.CanonicalValue(value)
	if value == "" {
		if testing.Testing() {
			panic(fmt.Sprintf("finding phrase slot %q filled with an empty value — "+
				"the emitter must pass a word or a declared fallback, not a hole in the sentence", slot))
		}
		// Only reachable in a release build: the panic above is where an
		// emitter that forgot a value is caught.
		value = "unknown"
	}
	if !strings.Contains(slot, pluralMarker) {
		return value
	}
	var singular bool
	switch tok := countTokenIn(slot); {
	case strings.Contains(slot, slotListToken):
		singular = !strings.Contains(value, ",")
		slot = strings.Replace(slot, slotListToken, value, 1)
	case tok != "":
		singular = value == "1"
		slot = strings.Replace(slot, tok, value, 1)
	default:
		// The declaration marked a noun for agreement and named no token, so
		// there is nowhere to put the value. The value is what the reader is
		// being shown; the declaration is what is wrong, and the gate over
		// the registered phrases is where that is caught.
		return value
	}
	if singular {
		return strings.ReplaceAll(slot, pluralMarker, "")
	}
	return strings.ReplaceAll(slot, pluralMarker, "s")
}

// addWave1Finding appends the Wave-1 posture Finding for code to r, through
// emitWave1.
func addWave1Finding(rowType string, r *resource.Resource, code domain.FindingCode, values ...string) {
	emitWave1(rowType, r, wave1Finding(code, values...))
}

// withWave1 is emitWave1 for a row built as one literal.
func withWave1(rowType string, r resource.Resource, fs ...domain.Finding) resource.Resource {
	emitWave1(rowType, &r, fs...)
	return r
}

// rebuiltWave1 is withWave1 for a copy of a row whose findings are rebuilt
// whole: the copy's slice shares its backing array with the original, so it
// is replaced rather than appended to.
func rebuiltWave1(rowType string, r resource.Resource, fs ...domain.Finding) resource.Resource {
	r.Findings = nil
	return withWave1(rowType, r, fs...)
}

// emitWave1 is the Wave 1 emit door, the one writer of a row's Findings in
// core/aws: each finding lands on r, a row of the type rowType, only where
// catalog.AppliesToRow allows it for the row's lifecycle and kind, so a
// fetcher evaluates its predicates and never decides applicability. r carries
// its Fields and RawStruct before the call, because the kind is read from
// them. The supporting rows of a finding the door withholds go with it.
func emitWave1(rowType string, r *resource.Resource, fs ...domain.Finding) {
	for _, f := range fs {
		// A code rowType does not declare stands unjudged, so a call naming
		// the wrong type would pass every finding through.
		if testing.Testing() && !catalog.Declares(rowType, f.Code) {
			panic(fmt.Sprintf("emitWave1: %s emits %s, which it does not declare", rowType, f.Code))
		}
		if catalog.AppliesToRow(rowType, f.Code, *r) {
			r.Findings = append(r.Findings, f)
			continue
		}
		if !slices.ContainsFunc(r.Findings, func(g domain.Finding) bool { return g.Code == f.Code }) {
			delete(r.AttentionDetails, f.Code)
		}
	}
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
func addSecretScanFinding(rowType string, r *resource.Resource, code domain.FindingCode, kv map[string]string) {
	rows := secretScanRows(kv)
	if len(rows) == 0 {
		return
	}
	addWave1Finding(rowType, r, code)
	addWave1Rows(r, code, rows...)
}
