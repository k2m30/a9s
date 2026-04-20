package unit

// Tests for internal/resource enrichment types
//
// Note: the four tests that previously appeared here
// (TestEnrichmentFinding_FieldValues, TestEnrichmentFinding_ZeroValue,
// TestEnrichmentFinding_UsableAsMapValue, TestEnrichmentFinding_SeverityValues)
// were removed as busywork per CLAUDE.md audit rule: they only assigned struct
// fields and asserted they retained those values — pure Go language semantics,
// not application behavior.
//
// Behavioral tests for enrichers that produce EnrichmentFinding values live in
// enrichment_pipeline_findings_test.go, enrichment_sfn_findings_test.go, etc.
