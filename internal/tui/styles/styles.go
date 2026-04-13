package styles

import (
	"os"
	"strings"

	lipgloss "charm.land/lipgloss/v2"
)

// Composed styles built from the Tokyo Night Dark palette.
var (
	HeaderStyle   lipgloss.Style
	TableHeader   lipgloss.Style
	RowSelected   lipgloss.Style
	RowNormal     lipgloss.Style
	RowAlt        lipgloss.Style
	BorderNormal  lipgloss.Style
	BorderFocused lipgloss.Style
	DetailKey     lipgloss.Style
	DetailVal     lipgloss.Style
	DetailSection lipgloss.Style
	FlashSuccess  lipgloss.Style
	FlashError    lipgloss.Style
	FilterActive  lipgloss.Style
	DimText           lipgloss.Style
	SpinnerStyle      lipgloss.Style
	NavigableField    lipgloss.Style
	ColSepDim         lipgloss.Style // │ separator when left column is focused
	ColSepAccent      lipgloss.Style // │ separator when right column is focused

	StatusCheckFailed lipgloss.Style // "!" glyph — RED bold (impaired)
	StatusCheckWarn   lipgloss.Style // "~" glyph — YELLOW (initializing)
	StatusCheckOk     lipgloss.Style // GREEN (ok values in detail view)

	HelpCatStyle         lipgloss.Style
	HelpKeyStyle         lipgloss.Style
	HelpDescStyle        lipgloss.Style
	IdentitySectionStyle lipgloss.Style
	IdentityLabelStyle   lipgloss.Style
	IdentityValueStyle   lipgloss.Style
	YAMLKeyStyle         lipgloss.Style
	YAMLStrStyle         lipgloss.Style
	YAMLNumStyle         lipgloss.Style
	YAMLBoolStyle        lipgloss.Style
	YAMLNullStyle        lipgloss.Style
	SearchCurrentStyle   lipgloss.Style
	SearchOtherStyle     lipgloss.Style
)

// NoColorActive reports whether NO_COLOR is set in the environment.
func NoColorActive() bool {
	_, ok := os.LookupEnv("NO_COLOR")
	return ok
}

// rowColorCache maps lowercase status strings to pre-built styles.
var rowColorCache map[string]lipgloss.Style

// RowColorStyle returns a style for a full row based on resource status.
// Uses a pre-built cache to avoid allocating new styles on every call.
func RowColorStyle(status string) lipgloss.Style {
	if NoColorActive() {
		return lipgloss.NewStyle()
	}
	lower := strings.ToLower(status)
	if s, ok := rowColorCache[lower]; ok {
		return s
	}
	// CloudFormation pattern-based matching: check suffixes.
	// Order matters: _in_progress before _complete because
	// UPDATE_COMPLETE_CLEANUP_IN_PROGRESS should match yellow, not green.
	switch {
	case strings.HasSuffix(lower, "_in_progress"):
		return lipgloss.NewStyle().Foreground(ColPending)
	case strings.HasSuffix(lower, "_failed"):
		return lipgloss.NewStyle().Foreground(ColStopped)
	case strings.HasSuffix(lower, "_complete"):
		return lipgloss.NewStyle().Foreground(ColRunning)
	}
	return lipgloss.NewStyle().Foreground(ColHeaderFg)
}

// IsDimRowColor reports whether RowColorStyle for the given status produces
// a dim or neutral foreground (i.e., the row has no severity signal worth
// the user's attention). Used by the global ctrl+z "show interesting only"
// filter. See docs/design/ct-event-list-v2.md §7.1.
func IsDimRowColor(status string) bool {
	// NO_COLOR disables the filter entirely — treat everything as non-dim
	// so attentionOnly mode does not blank the screen.
	if NoColorActive() {
		return false
	}
	lower := strings.ToLower(status)
	s, ok := rowColorCache[lower]
	if !ok {
		// Suffix branches mirror RowColorStyle.
		switch {
		case strings.HasSuffix(lower, "_in_progress"),
			strings.HasSuffix(lower, "_failed"),
			strings.HasSuffix(lower, "_complete"):
			return false
		}
		return true // default ColHeaderFg → dim
	}
	// Compare by constructing reference styles.
	dimRef := lipgloss.NewStyle().Foreground(ColTerminated)
	neutralRef := lipgloss.NewStyle().Foreground(ColHeaderFg)
	fg := s.GetForeground()
	return fg == dimRef.GetForeground() || fg == neutralRef.GetForeground()
}

func init() {
	applyPalette(DefaultTheme())
	initStyles()
}

// Reinit re-initializes all composed styles. Useful for tests that toggle NO_COLOR.
func Reinit() {
	applyPalette(ActiveTheme())
	initStyles()
}

func initStyles() {
	// Reset all styles to zero values first.
	HeaderStyle = lipgloss.Style{}
	TableHeader = lipgloss.Style{}
	RowSelected = lipgloss.Style{}
	RowNormal = lipgloss.Style{}
	RowAlt = lipgloss.Style{}
	BorderNormal = lipgloss.Style{}
	BorderFocused = lipgloss.Style{}
	DetailKey = lipgloss.Style{}
	DetailVal = lipgloss.Style{}
	DetailSection = lipgloss.Style{}
	FlashSuccess = lipgloss.Style{}
	FlashError = lipgloss.Style{}
	FilterActive = lipgloss.Style{}
	DimText = lipgloss.Style{}
	SpinnerStyle = lipgloss.Style{}
	NavigableField = lipgloss.Style{}
	ColSepDim = lipgloss.Style{}
	ColSepAccent = lipgloss.Style{}
	StatusCheckFailed = lipgloss.Style{}
	StatusCheckWarn = lipgloss.Style{}
	StatusCheckOk = lipgloss.Style{}
	HelpCatStyle = lipgloss.Style{}
	HelpKeyStyle = lipgloss.Style{}
	HelpDescStyle = lipgloss.Style{}
	IdentitySectionStyle = lipgloss.Style{}
	IdentityLabelStyle = lipgloss.Style{}
	IdentityValueStyle = lipgloss.Style{}
	YAMLKeyStyle = lipgloss.Style{}
	YAMLStrStyle = lipgloss.Style{}
	YAMLNumStyle = lipgloss.Style{}
	YAMLBoolStyle = lipgloss.Style{}
	YAMLNullStyle = lipgloss.Style{}
	SearchCurrentStyle = lipgloss.Style{}
	SearchOtherStyle = lipgloss.Style{}
	rowColorCache = nil

	// These 13 styles were previously package-level vars in view files,
	// initialized once at load time and unaffected by NO_COLOR / Reinit().
	// They are always initialized regardless of NO_COLOR to preserve that behavior.
	HelpCatStyle = lipgloss.NewStyle().Foreground(ColHelpCat).Bold(true)
	HelpKeyStyle = lipgloss.NewStyle().Foreground(ColHelpKey).Bold(true)
	HelpDescStyle = lipgloss.NewStyle().Foreground(ColDetailVal)
	IdentitySectionStyle = lipgloss.NewStyle().Foreground(ColDetailSec).Bold(true)
	IdentityLabelStyle = lipgloss.NewStyle().Foreground(ColDim)
	IdentityValueStyle = lipgloss.NewStyle().Foreground(ColDetailVal)
	YAMLKeyStyle = lipgloss.NewStyle().Foreground(ColYAMLKey)
	YAMLStrStyle = lipgloss.NewStyle().Foreground(ColYAMLStr)
	YAMLNumStyle = lipgloss.NewStyle().Foreground(ColYAMLNum)
	YAMLBoolStyle = lipgloss.NewStyle().Foreground(ColYAMLBool)
	YAMLNullStyle = lipgloss.NewStyle().Foreground(ColYAMLNull)
	SearchCurrentStyle = lipgloss.NewStyle().Foreground(ActiveTheme().SearchHighlightFg).Background(ActiveTheme().SearchHighlightBg)
	SearchOtherStyle = lipgloss.NewStyle().Underline(true).Foreground(ActiveTheme().SearchHighlightBg)

	if NoColorActive() {
		RowSelected = lipgloss.NewStyle().Reverse(true)
		return
	}

	// Pre-build row color styles by status string.
	rowColorCache = map[string]lipgloss.Style{
		"running":      lipgloss.NewStyle().Foreground(ColRunning),
		"available":    lipgloss.NewStyle().Foreground(ColRunning),
		"active":       lipgloss.NewStyle().Foreground(ColRunning),
		"in-use":       lipgloss.NewStyle().Foreground(ColRunning),
		"stopped":      lipgloss.NewStyle().Foreground(ColStopped),
		"failed":       lipgloss.NewStyle().Foreground(ColStopped),
		"error":        lipgloss.NewStyle().Foreground(ColStopped),
		"deleting":     lipgloss.NewStyle().Foreground(ColStopped),
		"deleted":      lipgloss.NewStyle().Foreground(ColStopped),
		"pending":      lipgloss.NewStyle().Foreground(ColPending),
		"stopping":     lipgloss.NewStyle().Foreground(ColPending), // ec2 lifecycle — transitioning
		"creating":     lipgloss.NewStyle().Foreground(ColPending),
		"modifying":    lipgloss.NewStyle().Foreground(ColPending),
		"updating":     lipgloss.NewStyle().Foreground(ColPending),
		"terminated":      lipgloss.NewStyle().Foreground(ColTerminated),
		"shutting-down":   lipgloss.NewStyle().Foreground(ColTerminated),
		"succeeded":       lipgloss.NewStyle().Foreground(ColRunning),
		"timed_out":       lipgloss.NewStyle().Foreground(ColStopped),
		"aborted":         lipgloss.NewStyle().Foreground(ColTerminated),
		"pending_redrive": lipgloss.NewStyle().Foreground(ColPending),

		// --- Green (ColRunning) ---
		"healthy":   lipgloss.NewStyle().Foreground(ColRunning), // TG Health
		"ok":        lipgloss.NewStyle().Foreground(ColRunning), // CloudWatch Alarms
		"issued":    lipgloss.NewStyle().Foreground(ColRunning), // ACM
		"deployed":  lipgloss.NewStyle().Foreground(ColRunning), // CloudFront
		"enabled":   lipgloss.NewStyle().Foreground(ColRunning), // EventBridge, KMS, Athena
		"green":     lipgloss.NewStyle().Foreground(ColRunning), // EB Health
		"success":   lipgloss.NewStyle().Foreground(ColRunning), // SES
		"completed": lipgloss.NewStyle().Foreground(ColRunning), // EBS Snapshot

		// --- Red (ColStopped) ---
		"unhealthy":                  lipgloss.NewStyle().Foreground(ColStopped), // TG Health
		"unavailable":                lipgloss.NewStyle().Foreground(ColStopped), // TG Health
		"alarm":                      lipgloss.NewStyle().Foreground(ColStopped), // CloudWatch
		"expired":                    lipgloss.NewStyle().Foreground(ColStopped), // ACM, VPC Endpoints
		"revoked":                    lipgloss.NewStyle().Foreground(ColStopped), // ACM
		"rejected":                   lipgloss.NewStyle().Foreground(ColStopped), // VPC Endpoints
		"pendingdeletion":            lipgloss.NewStyle().Foreground(ColStopped), // KMS
		"rollback_complete":          lipgloss.NewStyle().Foreground(ColStopped), // CFN: rollback = original op failed
		"import_rollback_complete":   lipgloss.NewStyle().Foreground(ColStopped), // CFN: import rollback = failure
		"red":                        lipgloss.NewStyle().Foreground(ColStopped), // EB Health
		"deregistered":               lipgloss.NewStyle().Foreground(ColStopped), // AMI
		"impaired":                   lipgloss.NewStyle().Foreground(ColStopped).Bold(true), // EC2 Status Checks

		// --- Yellow (ColPending) ---
		"draining":           lipgloss.NewStyle().Foreground(ColPending), // TG Health
		"initial":            lipgloss.NewStyle().Foreground(ColPending), // TG Health
		"insufficient_data":  lipgloss.NewStyle().Foreground(ColPending), // CloudWatch
		"pending_validation": lipgloss.NewStyle().Foreground(ColPending), // ACM
		"inprogress":         lipgloss.NewStyle().Foreground(ColPending), // CloudFront
		"healing":            lipgloss.NewStyle().Foreground(ColPending), // MSK
		"rebooting_broker":   lipgloss.NewStyle().Foreground(ColPending), // MSK
		"maintenance":        lipgloss.NewStyle().Foreground(ColPending), // MSK
		"rebooting":          lipgloss.NewStyle().Foreground(ColPending), // Redshift
		"resizing":           lipgloss.NewStyle().Foreground(ColPending), // Redshift
		"pendingimport":      lipgloss.NewStyle().Foreground(ColPending), // KMS
		"pendingacceptance":  lipgloss.NewStyle().Foreground(ColPending), // VPC Endpoints
		"yellow":             lipgloss.NewStyle().Foreground(ColPending), // EB Health
		"temporary_failure":  lipgloss.NewStyle().Foreground(ColPending), // SES
		"recovering":         lipgloss.NewStyle().Foreground(ColPending), // EBS Snapshot
		"recoverable":        lipgloss.NewStyle().Foreground(ColPending), // EBS Snapshot
		"initializing":       lipgloss.NewStyle().Foreground(ColPending), // EC2 Status Checks

		// --- Dim (ColTerminated) ---
		"unused":      lipgloss.NewStyle().Foreground(ColTerminated), // TG Health
		"disabled":    lipgloss.NewStyle().Foreground(ColTerminated), // EventBridge, KMS, Athena, CloudFront
		"inactive":    lipgloss.NewStyle().Foreground(ColTerminated), // ACM
		"grey":        lipgloss.NewStyle().Foreground(ColTerminated), // EB Health
		"not_started": lipgloss.NewStyle().Foreground(ColTerminated), // SES
		"paused":      lipgloss.NewStyle().Foreground(ColTerminated), // Redshift

		// --- CloudTrail severity ladder (§1.1 of ct-event-list-v2.md) ---
		"ct-info":      lipgloss.NewStyle().Foreground(ColTerminated), // routine reads — dim
		"ct-attention": lipgloss.NewStyle().Foreground(ColPending),    // writes / ROOT / sensitive / cross-account — yellow
		"ct-danger":    lipgloss.NewStyle().Foreground(ColStopped),    // destructive / failed — red
	}

	HeaderStyle = lipgloss.NewStyle().Padding(0, 1)
	TableHeader = lipgloss.NewStyle().Foreground(ColAccent).Bold(true)
	RowSelected = lipgloss.NewStyle().Background(ColRowSelectedBg).Foreground(ColRowSelectedFg).Bold(true)
	RowNormal = lipgloss.NewStyle().Foreground(ColHeaderFg)
	RowAlt = lipgloss.NewStyle().Foreground(ColHeaderFg).Background(ColRowAltBg)
	BorderNormal = lipgloss.NewStyle().Foreground(ColBorder)
	BorderFocused = lipgloss.NewStyle().Foreground(ColAccent)
	DetailKey = lipgloss.NewStyle().Foreground(ColDetailKey)
	DetailVal = lipgloss.NewStyle().Foreground(ColDetailVal)
	DetailSection = lipgloss.NewStyle().Foreground(ColDetailSec).Bold(true)
	FlashSuccess = lipgloss.NewStyle().Foreground(ColSuccess).Bold(true)
	FlashError = lipgloss.NewStyle().Foreground(ColError).Bold(true)
	FilterActive = lipgloss.NewStyle().Foreground(ColFilter).Bold(true)
	DimText = lipgloss.NewStyle().Foreground(ColDim)
	SpinnerStyle = lipgloss.NewStyle().Foreground(ColSpinner)
	NavigableField = lipgloss.NewStyle().Foreground(ColAccent).Underline(true)
	ColSepDim = lipgloss.NewStyle().Foreground(ColBorder)
	ColSepAccent = lipgloss.NewStyle().Foreground(ColAccent)
	StatusCheckFailed = lipgloss.NewStyle().Foreground(ColStopped).Bold(true)
	StatusCheckWarn = lipgloss.NewStyle().Foreground(ColPending)
	StatusCheckOk = lipgloss.NewStyle().Foreground(ColRunning)
}
