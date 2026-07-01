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

// colorFallback classifies a resource status string when no per-type Color func
// is set. Matches the helper of the same name in internal/catalog that
// ResolveColor uses — kept in sync because catalog_compute.go's colorEC2 and
// other per-type classifiers call it directly.
func colorFallback(status string) domain.Color {
	switch status {
	case "running", "available", "active", "ACTIVE", "AVAILABLE", "RUNNING",
		"in-service", "healthy":
		return domain.ColorHealthy
	case "stopped", "failed", "error", "impaired", "FAILED", "ERROR",
		"STOPPED":
		return domain.ColorBroken
	case "terminated", "TERMINATED", "shutting-down", "deleted", "DELETED",
		"deregistered", "inactive", "INACTIVE":
		return domain.ColorDim
	}
	lower := strings.ToLower(status)
	switch {
	case strings.HasSuffix(lower, "_failed") || strings.HasSuffix(lower, "-failed"):
		return domain.ColorBroken
	case strings.HasSuffix(lower, "_in_progress") || strings.HasSuffix(lower, "_progress") ||
		strings.HasSuffix(lower, "-in-progress") || status == "pending" ||
		status == "creating" || status == "modifying" || status == "updating" ||
		status == "initializing":
		return domain.ColorWarning
	}
	return domain.ColorHealthy
}

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

// colorWave1OrHealthy classifies r from its first wave1 Finding, defaulting to
// healthy when none is present. Used by child-type catalog entries whose only
// severity signal comes from fetcher-emitted wave1 Findings (cb_builds,
// cfn_resources, glue_runs, log_events, lambda_invocation_logs,
// role_policies).
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

// acmColor classifies an ACM certificate resource.
func acmColor(r domain.Resource) domain.Color {
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

// r53Color classifies a Route53 hosted zone resource.
func r53Color(r domain.Resource) domain.Color {
	s := r.Fields["record_count"]
	if s != "" {
		if n, err := strconv.ParseInt(s, 10, 64); err == nil && n <= 2 {
			return domain.ColorWarning
		}
	}
	return domain.ColorHealthy
}
