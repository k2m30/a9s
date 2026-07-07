package aws

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/k2m30/a9s/v3/internal/domain"
)

// Shared helpers used by the per-category catalog data files
// (catalog_<cat>.go) for color classification and status-phrase parsing.
//
// These helpers live here (not internal/catalog) so the catalog data slices
// can live in the same package as the fetchers and transports they describe.
// The intrinsic ResolveColor fallback used by
// catalog.ResourceTypeDef.ResolveColor stays in internal/catalog.

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

// colorFromWave1 returns the Color implied by the first wave1 Finding on r and
// ok=true. ok=false signals no wave1 Finding — the caller should fall through
// to its structural classifier.
func colorFromWave1(r domain.Resource) (domain.Color, bool) {
	for i := range r.Findings {
		if r.Findings[i].Source == "wave1" {
			return colorFromSeverity(r.Findings[i].Severity), true
		}
	}
	return domain.ColorHealthy, false
}

// colorFromAnyFinding returns the Color for the worst-severity Finding on r
// (wave1 or wave2, source prefix "wave2:") and ok=true. ok=false signals no
// Finding at all — the caller should fall through to its structural
// classifier. Unlike colorFromWave1, this also surfaces wave2-only findings
// (e.g. elb's deletion-protection check, vpc's flow-logs check, tgw's
// attachment-health check) that a wave1-only lookup silently drops.
func colorFromAnyFinding(r domain.Resource) (domain.Color, bool) {
	found := false
	worst := domain.SevDim
	for i := range r.Findings {
		s := r.Findings[i].Source
		if s != "wave1" && !strings.HasPrefix(s, "wave2:") {
			continue
		}
		sev := r.Findings[i].Severity
		if !found || sev > worst {
			worst = sev
			found = true
		}
	}
	if !found {
		return domain.ColorHealthy, false
	}
	return colorFromSeverity(worst), true
}

// colorWave1OrHealthy classifies r from its first wave1 Finding, defaulting to
// healthy when none is present. Used by child-type catalog entries whose only
// severity signal comes from fetcher-emitted wave1 Findings (cb_builds,
// cfn_resources, glue_runs, log_events, lambda_invocation_logs,
// role_policies, ecr_images, ecs_svc_logs, dbi_events, eb_rule_targets,
// sns_subscriptions, elb_listeners).
func colorWave1OrHealthy(r domain.Resource) domain.Color {
	if c, ok := colorFromWave1(r); ok {
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
	switch r.Fields["status"] {
	case "ISSUED":
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
	case "PENDING_VALIDATION":
		return domain.ColorWarning
	case "EXPIRED", "REVOKED", "FAILED", "VALIDATION_TIMED_OUT":
		return domain.ColorBroken
	case "INACTIVE":
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
