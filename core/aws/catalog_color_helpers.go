// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"slices"
	"time"

	"github.com/k2m30/a9s/v3/core/domain"
)

// Shared helpers used by the per-category catalog data files
// (catalog_<cat>.go) for color classification and status-phrase parsing.
//
// These helpers live here (not core/catalog) so the catalog data slices
// can live in the same package as the fetchers and transports they describe.
// The intrinsic ResolveColor fallback used by
// catalog.ResourceTypeDef.ResolveColor stays in core/catalog.

// colorFromSeverity maps a domain.Severity to the corresponding display Color.
func colorFromSeverity(sev domain.Severity) domain.Color {
	switch sev {
	case domain.SevBroken:
		return domain.ColorBroken
	case domain.SevWarn:
		return domain.ColorWarning
	case domain.SevDim:
		return domain.ColorDim
	default:
		return domain.ColorHealthy
	}
}

// colorFromAnyFinding returns the Color for the worst-severity Finding on r
// (wave1 or wave2, source prefix "wave2:") and ok=true. ok=false signals no
// Finding at all — the caller should fall through to its structural
// classifier. It surfaces wave2-only findings
// (e.g. elb's deletion-protection check, vpc's flow-logs check, tgw's
// attachment-health check) that a wave1-only lookup silently drops.
func colorFromAnyFinding(r domain.Resource) (domain.Color, bool) {
	top, ok := domain.TopFinding(r.Findings)
	if !ok {
		return domain.ColorHealthy, false
	}
	return colorFromSeverity(top.Severity), true
}

// hasFinding reports whether findings carries code. A fetcher attaching the
// supporting rows for a posture finding asks its own predicate's answer rather
// than re-evaluating the condition the predicate just decided.
func hasFinding(findings []domain.Finding, code domain.FindingCode) bool {
	return slices.ContainsFunc(findings, func(f domain.Finding) bool { return f.Code == code })
}

// colorFromFindings is the bare-Fields path every classifier ends with: a
// Resource built outside the fetcher (a constructed row in a contract test, a
// cached row from before findings existed) carries Fields but no Findings, so
// the classifier runs the type's OWN findings predicate over those Fields
// rather than reading the same raw fields a second way.
func colorFromFindings(findings []domain.Finding) domain.Color {
	return colorAnyFindingOrHealthy(domain.Resource{Findings: findings})
}

// colorAnyFindingOrHealthy classifies r from colorFromAnyFinding, defaulting
// to healthy when no Finding is present. Shared by the mwaa/transfer/vpc-peer/
// lt catalog entries: every signal on these types is color-bearing — no
// glyph-on-green case exists (docs/resources/mwaa.md §4, transfer.md §4,
// vpc-peer.md §4, lt.md §4) — and real fetched resources always carry a
// Finding when off-Healthy, so there is no raw-field fallback to keep.
func colorAnyFindingOrHealthy(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return domain.ColorHealthy
}

// acmColor derives the row colour from the certificate's findings, and for a
// row that carries none from the same predicate the fetcher used, over the
// values it wrote into Fields.
func acmColor(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return colorFromFindings(acmFindings(
		r.Fields["status"], r.Fields["not_after"], r.Fields["in_use"],
		r.Fields["key_algorithm"], time.Now()))
}

// r53Color derives the row colour from the zone's findings, and for a row that
// carries none from the same predicate the fetcher used, over the record count
// it wrote into Fields. The orphan-private-zone signal is wave 2 and no
// fallback recovers it.
func r53Color(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	return colorFromFindings(r53ZoneFindings(r.Fields["record_count"]))
}
