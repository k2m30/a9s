// Package attention holds the Phase-03 canonical-finding derivation shim.
//
// DeriveFindings is a Wave-1-only one-way bridge from the legacy
// Resource.Status + Resource.Issues model to the new Resource.Findings model.
// Wave-2 entries are appended downstream by runtime.Core.applyEnrichment using
// the typed Finding + AttentionDetail emitted by each enricher (post-AS-1395
// type swap). Until per-category PRs (03b–m) migrate fetchers to write
// Findings directly, every entry point that surfaces a Resource to view code
// calls DeriveFindings to populate the new fields.
//
// The function is deterministic — re-derives every call from inputs, never
// early-returns on len(r.Findings) > 0. The Wave 2 bridge depends on this:
// applyEnrichment re-runs derive before each Wave-2 append so stale Wave-2
// entries do not accumulate across reruns.
package attention

import (
	"maps"
	"regexp"
	"strings"

	"github.com/k2m30/a9s/v3/internal/domain"
	"github.com/k2m30/a9s/v3/internal/resource"
)

// DeriveFindings populates r.Findings from r.Status + r.Issues (Wave-1 only).
// Replaces — never appends to — r.Findings and r.AttentionDetails so stale
// entries from prior runs are discarded.
//
// Output contract:
//   - r.Findings:         wave1 issue entries only (lifecycle steady-state
//     phrases are filtered).
//   - r.AttentionDetails: cleared (nil).  Wave-2 callers (applyEnrichment)
//     are responsible for appending the wave2 Finding and writing the
//     companion AttentionDetail keyed by FindingCode after this call.
//   - Healthy row (no Status, no Issues): both fields nil.
//
// Safe on nil r (no-op).
func DeriveFindings(r *domain.Resource, td resource.ResourceTypeDef) {
	if r == nil {
		return
	}
	var findings []domain.Finding

	// Detect whether this row was written by a migrated fetcher (PR-03b+).
	// Migrated fetchers write neither r.Status nor r.Issues — they emit
	// Findings directly. When both legacy fields are empty, preserve any
	// existing wave1 entries rather than re-deriving (and wiping) them.
	migrated := r.Status == "" && len(r.Issues) == 0

	if migrated {
		// Preserve fetcher-emitted wave1 entries verbatim.
		for _, f := range r.Findings {
			if f.Source == "wave1" {
				findings = append(findings, f)
			}
		}
	} else {
		// Legacy path: derive wave1 from r.Status / r.Issues.
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
	}

	r.Findings = findings
	r.AttentionDetails = nil
}

// DeriveWave1Only re-derives only the wave1 portion of r.Findings from r.Status
// and r.Issues, preserving any existing wave2 entries (Source has "wave2:" prefix)
// and their AttentionDetails. Used by non-EnrichmentChecked entry points so that
// wave2 findings written by applyEnrichment (PR-03a-fold) are not wiped when a
// resource is re-derived at a later site (e.g. cache-hit navigation, lazy add).
//
// The EnrichmentChecked handler uses applyEnrichment which re-derives wave1
// and then appends the new wave2 entry directly.
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
	DeriveFindings(r, td)
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
