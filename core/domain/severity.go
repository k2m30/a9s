package domain

// Severity classifies a resource or finding by its health level.
type Severity int

const (
	SevDim    Severity = iota // pending / transitional
	SevOK                     // healthy
	SevWarn                   // degraded
	SevBroken                 // failed / unreachable
)

// IsIssue reports whether this severity level contributes to attention filtering
// and issue badges.
func (s Severity) IsIssue() bool { return s == SevWarn || s == SevBroken }
