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
//
// Contract — Summary vs Rows (enforced by the Attention section renderer and
// required by the a9s-implement-resource skill):
//
//   - Summary is the short S5 phrase — ideally a §4-style lowercase phrase
//     like "pending maintenance", "latest build failed", "unhealthy targets
//     2/5". It is what renders beside the glyph on the Attention primary
//     entry row, and it is also what Wave 2 may promote to the S4 Status
//     column via FieldUpdates["status"] (so it must fit that width).
//
//   - Rows are the structured facts that SUPPORT Summary (the specific
//     Action, Description, Earliest Target, failing target names, failure
//     timestamp, etc). They render beneath the primary entry as indented
//     `Label: Value` pairs.
//
//   - Summary must NEVER embed Row content. Every fact lives in exactly one
//     place: either Summary or Rows, never both. The Attention section is
//     not allowed to produce duplication; if you find yourself writing
//     `fmt.Sprintf("… %s (%s)", action, description)` as a Summary while
//     also emitting Action and Description as Rows, that is the bug this
//     contract exists to prevent.
type EnrichmentFinding struct {
	// Severity is "!" for broken/degraded resources (contributes to menu badge)
	// or "~" for scheduled/informational findings (excluded from menu badge).
	Severity string
	// Summary is the short S5 phrase. See the contract on EnrichmentFinding.
	Summary string
	// Rows are the structured facts that support Summary. See the contract
	// on EnrichmentFinding — content here must not also appear in Summary.
	Rows []FindingRow
}
