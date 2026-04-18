package resource

// FindingRow is a single labeled data point within an EnrichmentFinding.
// Tier controls per-row coloring; empty tier falls back to the finding's Severity.
type FindingRow struct {
	Label string
	Value string
	Tier  string // "!", "~", or "" (neutral context)
}

// EnrichmentFinding is a per-resource finding produced by Wave 2 background
// enrichment. It lives in internal/resource/ (not aws/ or tui/) to be
// importable by both enricher implementations and view code without creating
// an import cycle.
type EnrichmentFinding struct {
	// Severity is "!" for broken/degraded resources (contributes to menu badge)
	// or "~" for scheduled/informational findings (excluded from menu badge).
	Severity string
	// Summary is a short human-readable description of the finding, e.g.
	// "pending maintenance: system-update" or "latest build FAILED (2026-04-13)".
	Summary string
	// Rows contains structured labeled data for per-type detail-view injection.
	Rows []FindingRow
}
