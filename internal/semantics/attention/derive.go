// Package attention holds the Phase-03 canonical-finding derivation shim.
//
// DeriveFindings is a one-way bridge from the legacy Resource.Status +
// Resource.Issues + Wave-2 EnrichmentFinding model to the new
// Resource.Findings + Resource.AttentionDetails model. Until per-category
// PRs (03b–m) migrate fetchers to write Findings directly, every entry
// point that surfaces a Resource to view code calls DeriveFindings to
// populate the new fields.
//
// The function is deterministic — re-derives every call from inputs, never
// early-returns on len(r.Findings) > 0. The Wave 2 bridge depends on this:
// EnrichmentCheckedMsg arrives after the first derive call has already run,
// and the second call must re-merge the just-populated parallel map.
package attention

import (
	"maps"
	"regexp"
	"strings"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// DeriveFindings populates r.Findings and r.AttentionDetails from r.Status,
// r.Issues, and the supplied enrichmentFindings keyed by resource ID.
// Caller passes m.EnrichmentFindings[resourceType] as the third arg.
//
// Output contract:
//   - r.Findings:        wave1 issue entries only (lifecycle steady-state
//     phrases are filtered), then wave2 entry if
//     enrichmentFindings[r.ID] exists.
//   - r.AttentionDetails: keyed by FindingCode; only contains the wave2 entry
//     when present.
//   - Healthy row (no Status, no Issues, no enrichment): both fields nil.
//   - Replacement, not append: stale prior entries are discarded.
//
// Safe on nil r (no-op).
func DeriveFindings(r *domain.Resource, td resource.ResourceTypeDef, enrichmentFindings map[string]resource.EnrichmentFinding) {
	if r == nil {
		return
	}
	var findings []domain.Finding

	// Wave 1: prefer r.Issues; fall back to r.Status if Issues empty.
	issues := r.Issues
	if len(issues) == 0 && r.Status != "" {
		issues = []string{resource.StripFindingSuffix(r.Status)}
	}
	for _, raw := range issues {
		phrase := resource.StripFindingSuffix(raw)
		if phrase == "" {
			continue
		}
		if isLifecyclePhrase(phrase) {
			continue
		}
		findings = append(findings, domain.Finding{
			Code:     domain.FindingCode(td.ShortName + "." + slug(phrase)),
			Phrase:   phrase,
			Severity: phraseSeverity(phrase),
			Source:   "wave1",
		})
	}

	// Wave 2: at most one finding per resource per type.
	var attn map[domain.FindingCode]domain.AttentionDetail
	if ef, ok := enrichmentFindings[r.ID]; ok && ef.Summary != "" {
		code := domain.FindingCode(td.ShortName + "." + slug(ef.Summary))
		findings = append(findings, domain.Finding{
			Code:     code,
			Phrase:   ef.Summary,
			Severity: severityFromMarker(ef.Severity),
			Source:   "wave2:" + td.ShortName,
		})
		if len(ef.Rows) > 0 {
			rows := make([]domain.DetailRow, 0, len(ef.Rows))
			for _, row := range ef.Rows {
				rows = append(rows, domain.DetailRow{
					Label: row.Label,
					Value: row.Value,
					Tier:  row.Tier,
				})
			}
			attn = map[domain.FindingCode]domain.AttentionDetail{
				code: {Rows: rows},
			}
		}
	}

	r.Findings = findings
	r.AttentionDetails = attn
}

// DeriveWave1Only re-derives only the wave1 portion of r.Findings from r.Status
// and r.Issues, preserving any existing wave2 entries (Source has "wave2:" prefix)
// and their AttentionDetails. Used by non-EnrichmentChecked entry points so that
// wave2 findings written by applyEnrichment (PR-03a-fold) are not wiped when a
// resource is re-derived at a later site (e.g. cache-hit navigation, lazy add).
//
// The EnrichmentChecked handler uses applyEnrichment which calls the full
// DeriveFindings with the new wave2 inputs.
//
// Safe on nil r (no-op).
func DeriveWave1Only(r *domain.Resource, td resource.ResourceTypeDef) {
	if r == nil {
		return
	}
	// Save any existing wave2 entries before the full re-derive wipes them.
	var wave2 []domain.Finding
	for _, f := range r.Findings {
		if strings.HasPrefix(f.Source, "wave2:") {
			wave2 = append(wave2, f)
		}
	}
	var wave2Attn map[domain.FindingCode]domain.AttentionDetail
	if len(wave2) > 0 && r.AttentionDetails != nil {
		wave2Attn = make(map[domain.FindingCode]domain.AttentionDetail, len(wave2))
		for _, f := range wave2 {
			if d, ok := r.AttentionDetails[f.Code]; ok {
				wave2Attn[f.Code] = d
			}
		}
	}
	// Re-derive wave1 only (nil enrichment = no wave2 emitted).
	DeriveFindings(r, td, nil)
	// Re-append the saved wave2 entries.
	r.Findings = append(r.Findings, wave2...)
	if len(wave2Attn) > 0 {
		if r.AttentionDetails == nil {
			r.AttentionDetails = wave2Attn
		} else {
			maps.Copy(r.AttentionDetails, wave2Attn)
		}
	}
}

// severityFromMarker maps an EnrichmentFinding.Severity glyph to a domain.Severity.
//
//	"!" -> SevBroken (degraded/failed)
//	"~" -> SevWarn   (scheduled/informational)
//	any other (incl. "") -> SevDim (unknown)
func severityFromMarker(s string) domain.Severity {
	switch s {
	case "!":
		return domain.SevBroken
	case "~":
		return domain.SevWarn
	default:
		return domain.SevDim
	}
}

// phraseSeverity assigns a default Severity to a wave1 phrase. PR-03a-shim
// uses a coarse heuristic: phrases that look like lifecycle steady-states
// ("running", "available") map to SevOK; phrases containing failure-class
// substrings map to SevBroken; everything else to SevWarn. Per-category PRs
// override this with explicit code-level Severity once they migrate the
// type's Color func.
func phraseSeverity(phrase string) domain.Severity {
	p := strings.ToLower(phrase)
	switch p {
	case "running", "available", "active", "in-service", "healthy":
		return domain.SevOK
	case "terminated", "deleted", "shutting-down", "deregistered":
		return domain.SevDim
	}
	// "inactive" intentionally absent — ECS service/cluster types classify
	// INACTIVE as broken (see internal/resource/types_compute.go:102, 171).
	// Falls through to the default branch (SevWarn) so the shim emits a
	// Finding; per-category PR-03c will assign canonical Severity.
	if strings.Contains(p, "fail") || strings.Contains(p, "impaired") ||
		strings.Contains(p, "error") || strings.Contains(p, "broken") ||
		strings.Contains(p, "stopped") {
		return domain.SevBroken
	}
	return domain.SevWarn
}

func isLifecyclePhrase(phrase string) bool {
	switch strings.ToLower(phrase) {
	case "running", "available", "active", "in-service", "healthy",
		"terminated", "deleted", "shutting-down", "deregistered":
		// "inactive" is intentionally absent from this list. Several resource
		// types (ECS service, ECS cluster) classify INACTIVE as broken rather
		// than lifecycle steady-state. Filtering it here would suppress those
		// Findings. See TestDerive_InactiveIsEmittedAsFinding.
		return true
	default:
		return false
	}
}

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

// slug normalizes a phrase to a stable code suffix. Lowercase; runs of
// non-alphanumerics collapse to a single dot; leading/trailing dots trimmed.
//
//	"system check failed" -> "system.check.failed"
//	"pending maintenance" -> "pending.maintenance"
//
// Per-category PRs (03b–m) replace these synthesized codes with canonical
// constants declared next to each enricher.
func slug(phrase string) string {
	s := strings.ToLower(strings.TrimSpace(phrase))
	s = slugRe.ReplaceAllString(s, ".")
	s = strings.Trim(s, ".")
	return s
}
