package domain

// FindingCode is a stable identifier for a finding. Never displayed.
// Codes are namespaced by resource short-name (e.g. "ec2.impaired",
// "rds.maint.pending"). Phase 03 introduces these as typed constants
// per enricher; Phase 04 may graduate them to a declarative table.
type FindingCode string

// Finding is the canonical row/menu/status semantics carrier on Resource.
// Drives row coloring, list-view Status display, menu issue badges, and
// the ctrl+z attention filter.
type Finding struct {
	Code     FindingCode
	Phrase   string
	Severity Severity
	Source   string // "wave1" | "wave2:<short>"
}

// AttentionDetail carries the rows shown in the detail-view Attention
// section for a given FindingCode. Consumed only by the detail view's
// Attention section — list views read Finding.Phrase / Finding.Severity.
type AttentionDetail struct {
	Rows []DetailRow
}

// DetailRow is a single label/value pair in an AttentionDetail.
//
// Tier is the optional display tier — "!" for emphasized, "~" for muted,
// "" for default (inherits from the parent Finding.Severity).
type DetailRow struct {
	Label string
	Value string
	Tier  string
}
