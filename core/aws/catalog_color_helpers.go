// SPDX-License-Identifier: GPL-3.0-or-later OR LicenseRef-Commercial

package aws

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

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

// findingSuffixRe strips the trailing " (+N)" suffix from a status phrase.
var findingSuffixRe = regexp.MustCompile(` \(\+\d+\)$`)

// stripFindingSuffix removes any trailing " (+N)" from a Status phrase.
func stripFindingSuffix(s string) string {
	return findingSuffixRe.ReplaceAllString(s, "")
}

// cfnStackColor maps CloudFormation stack status strings to a Color.
func cfnStackColor(status string) domain.Color {
	switch status {
	case "CREATE_COMPLETE", "UPDATE_COMPLETE", "IMPORT_COMPLETE":
		return domain.ColorHealthy
	case "DELETE_COMPLETE":
		return domain.ColorDim
	case "ROLLBACK_COMPLETE", "ROLLBACK_FAILED",
		"UPDATE_ROLLBACK_COMPLETE", "UPDATE_ROLLBACK_FAILED",
		"IMPORT_ROLLBACK_COMPLETE", "IMPORT_ROLLBACK_FAILED":
		return domain.ColorBroken
	}
	if strings.HasSuffix(status, "_IN_PROGRESS") {
		return domain.ColorWarning
	}
	if strings.HasSuffix(status, "_FAILED") {
		return domain.ColorBroken
	}
	return domain.ColorHealthy
}

// acmColor classifies an ACM certificate resource. Prefers colorFromAnyFinding
// so real fetched resources (Findings populated by acmStatusFindings, Source:
// "wave1", and EnrichACMCertificate, Source: "wave2:acm") color from their own
// Finding; the raw-field switch below is the identical-precedence fallback
// for callers that construct a Resource with only Fields set (e.g.
// qa_acm_color_test.go, qa_acm_validation_timed_out_test.go).
func acmColor(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	switch domain.HumanizeStatusPhrase(r.Fields["status"]) {
	case "issued":
		dl := r.Fields["days_left"]
		if dl == "expired" {
			return domain.ColorBroken
		}
		if dl != "" {
			var n int
			if _, err := fmt.Sscanf(dl, "%d days", &n); err == nil {
				if n < 7 {
					return domain.ColorBroken
				}
				if n < 30 {
					return domain.ColorWarning
				}
			}
		}
		if r.Fields["in_use"] == "false" {
			return domain.ColorWarning
		}
		return domain.ColorHealthy
	case "pending validation":
		return domain.ColorWarning
	case "expired", "revoked", "failed", "validation timed out":
		return domain.ColorBroken
	case "inactive":
		return domain.ColorDim
	}
	return domain.ColorHealthy
}

// r53Color classifies a Route53 hosted zone resource. Prefers
// colorFromAnyFinding so real fetched resources (Findings populated by
// r53CodeUnusedZone, Source: "wave1", and r53CodeOrphanPrivateZone, Source:
// "wave2:r53") color from their own Finding; the raw-field check below is the
// identical-precedence fallback for callers that construct a Resource with
// only Fields set (e.g. qa_r53_color_test.go).
func r53Color(r domain.Resource) domain.Color {
	if c, ok := colorFromAnyFinding(r); ok {
		return c
	}
	s := r.Fields["record_count"]
	if s != "" {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil && n <= 2 {
			return domain.ColorWarning
		}
	}
	return domain.ColorHealthy
}
