// ami_issue_enrichment.go — Wave 2 = None for the ami resource type.
//
// Retained as a NoOp registration so TestConformance_EveryResourceTypeHasWave2Registration
// and TestAttentionSignalsDoc see an explicit registry entry. AS-795n deletes
// both this file and the IssueEnricherRegistry it populates once consumers
// (BuildEnrichQueue, runtime_adapter_navigate, probe_adapter) move to
// catalog.AllByWave2().
package aws

func init() {
	registerIssueEnricher("ami", NoOpIssueEnricher, 100)
}
