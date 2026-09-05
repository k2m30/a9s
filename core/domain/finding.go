// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package domain

import (
	"strconv"
	"strings"
)

// wave2SourcePrefix is the Source prefix stamped on every Wave-2-emitted
// Finding ("wave2:<short>"). Wave-1 findings never carry this prefix — their
// Source is the empty string or a Wave-1-specific value.
const wave2SourcePrefix = "wave2:"

// FindingCode is a stable identifier for a finding. Never displayed.
// Codes are namespaced by resource short-name (e.g. "ec2.impaired",
// "rds.maint.pending"). They are declared as typed constants per enricher.
type FindingCode string

// Finding is the canonical row/menu/status semantics carrier on Resource.
// Drives row coloring, list-view Status display, menu issue badges, and
// the ctrl+z attention filter.
type Finding struct {
	Code   FindingCode
	Phrase string
	// Detail is the S5 "concrete operator sentence" — a full remedy/context
	// sentence for the detail-view Attention section. Empty ⇒ callers fall
	// back to rendering Phrase alone (no stray blank line). Distinct from
	// Phrase (the short S4 cause shown in list/menu surfaces): Phrase is
	// always populated, Detail is optional richer text for S5.
	Detail   string
	Severity Severity
	Source   string // "wave1" | "wave2:<short>"
}

// IsWave2Sourced reports whether f was emitted by a Wave-2 issue enricher
// (Source prefixed "wave2:"), as opposed to a Wave-1 fetcher-written finding.
// This is the single definition of the wave2-source predicate — every caller
// that needs to distinguish carryable Wave-2 findings from Wave-1 findings
// (which never carry across a rows-carrying write, since their absence in a
// fresh fetch means resolved) must use this method rather than re-deriving
// the "wave2:" prefix check.
func (f Finding) IsWave2Sourced() bool {
	return strings.HasPrefix(f.Source, wave2SourcePrefix)
}

// WorstSeverityFinding returns the highest-Severity entry in fs (ties keep
// the first-seen entry). This is the single shared reducer every
// render-boundary consumer that must collapse a resource's independently-
// evaluated Finding slice down to one value (a row glyph, a single-Finding
// detail-apply signature) uses — never a first-seen pick, which silently
// prefers whichever condition an enricher happened to evaluate first over
// the one an operator most needs to see. Callers must ensure len(fs) > 0;
// WorstSeverityFinding does not guard against an empty slice.
func WorstSeverityFinding(fs []Finding) Finding {
	worst := fs[0]
	for _, f := range fs[1:] {
		if f.Severity > worst.Severity {
			worst = f
		}
	}
	return worst
}

// AttentionDetail carries the rows shown in the detail-view Attention
// section for a given FindingCode. Consumed only by the detail view's
// Attention section — list views read Finding.Phrase / Finding.Severity.
type AttentionDetail struct {
	Rows []DetailRow
}

// DetailRow is a single label/value pair in an AttentionDetail.
//
// Tier is the optional display tier — "!" for emphasized, "~" for muted,
// "" for default (inherits from the parent Finding.Severity).
type DetailRow struct {
	Label string
	Value string
	Tier  string
}

// selectionRank orders findings for display, which is not the enum's own
// order: SevOK sits between SevDim and SevWarn, so a plain maximum would put
// "healthy" text on a row the operator is looking at because it is dim.
// Broken beats warn beats dim beats everything else.
func selectionRank(s Severity) int {
	switch s {
	case SevBroken:
		return 3
	case SevWarn:
		return 2
	case SevDim:
		return 1
	default:
		return 0
	}
}

// TopFinding returns the finding that decides the row: the worst by
// selectionRank, slice order among equals. A dim finding is therefore
// selected only when nothing issue-severity is present, and a healthy one
// only when there is nothing else at all. ok is false for an empty slice.
//
// This is the single selection every render surface shares — the Status cell
// phrase and the row colour both resolve through it, so a red row can never
// read as a warning.
func TopFinding(findings []Finding) (Finding, bool) {
	if len(findings) == 0 {
		return Finding{}, false
	}
	top := findings[0]
	for _, f := range findings[1:] {
		if selectionRank(f.Severity) > selectionRank(top.Severity) {
			top = f
		}
	}
	return top, true
}

// StatusPhrase is the list Status cell for a resource's findings: the phrase
// of the finding TopFinding selects, suffixed "(+N)" for the other
// issue-severity findings stacked behind it. Dim findings never count — a
// state is not one of the things wrong with the row. Empty for a resource
// with no findings.
func StatusPhrase(findings []Finding) string {
	top, ok := TopFinding(findings)
	if !ok {
		return ""
	}
	others := 0
	for _, f := range findings {
		if f.Severity.IsIssue() {
			others++
		}
	}
	if top.Severity.IsIssue() {
		others--
	}
	if others <= 0 {
		return top.Phrase
	}
	return top.Phrase + " (+" + strconv.Itoa(others) + ")"
}
