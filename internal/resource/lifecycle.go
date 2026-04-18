package resource

import "strings"

// StandardLifecycleColor maps a common AWS status string to a Color.
// Only the shapes that genuinely appear across multiple services.
// Opt-in — not a fallback. Types call this explicitly if their
// status vocabulary matches.
func StandardLifecycleColor(status string) Color {
	switch strings.ToLower(status) {
	case "active", "available":
		return ColorHealthy
	case "creating", "updating", "deleting", "modifying":
		return ColorWarning
	case "failed", "error":
		return ColorBroken
	case "deleted":
		return ColorDim
	case "inactive":
		// "inactive" is a steady-state status (ASG scaled to 0, disabled
		// rule). Not actively broken — dim and excluded from issue badges.
		return ColorDim
	}
	return ColorHealthy
}
