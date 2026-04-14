package resource

// StandardLifecycleColor maps a common AWS status string to a Color.
// Only the shapes that genuinely appear across multiple services.
// Opt-in — not a fallback. Types call this explicitly if their
// status vocabulary matches.
func StandardLifecycleColor(status string) Color {
	switch status {
	case "active", "available", "ACTIVE", "AVAILABLE":
		return ColorHealthy
	case "creating", "updating", "deleting", "modifying", "CREATING", "UPDATING", "DELETING":
		return ColorWarning
	case "failed", "error", "FAILED", "ERROR":
		return ColorBroken
	case "deleted", "inactive", "DELETED", "INACTIVE":
		return ColorDim
	}
	return ColorHealthy
}
