package catalog

import (
	"strings"

	"github.com/k2m30/a9s/v3/internal/domain"
)

// colorFallback classifies a resource status string when no per-type Color func
// is set. Covers the common AWS vocabulary so ad-hoc ResourceTypeDef instances
// (which omit Color) behave sensibly without requiring every type to set up Color.
//
// Used by ResolveColor in types.go; per-type classifiers (e.g. colorEC2) and
// other shared helpers (colorFromSeverity, stripFindingSuffix, cfnStackColor,
// acmColor, r53Color) live alongside the catalog data in
// internal/aws/catalog_color_helpers.go.
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
